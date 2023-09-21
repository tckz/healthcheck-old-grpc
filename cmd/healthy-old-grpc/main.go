package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/samber/lo"
	"github.com/tckz/healthcheck-old-grpc"
	"github.com/tckz/healthcheck-old-grpc/api"
	"github.com/tckz/healthcheck-old-grpc/log"
	"go.uber.org/zap"
	goji "goji.io"
	"goji.io/pat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var myName = filepath.Base(os.Args[0])
var logger *zap.SugaredLogger

var (
	optRandomPanic = flag.Bool("random-panic", false, "Random panic")
)

func init() {
	logger = lo.Must(log.New()).With(zap.String("app", myName))
}

type helloServer struct {
}

var hostName = lo.Must(os.Hostname())

func (s *helloServer) SayHello(ctx context.Context, req *api.HelloRequest) (*api.HelloReply, error) {

	if *optRandomPanic && rand.Intn(100) < 10 {
		panic("Random panic!!")
	}

	delay := rand.Intn(300)
	time.Sleep(time.Duration(delay) * time.Millisecond)

	res := &api.HelloReply{
		Message: fmt.Sprintf("Hello %s, from %s", req.Name, hostName),
		Now:     TimestampPB(time.Now()),
	}
	return res, nil
}

func (s *helloServer) SayMorning(ctx context.Context, req *api.MorningRequest) (*api.MorningReply, error) {
	res := &api.MorningReply{
		Message: fmt.Sprintf("Morning %s, from %s", req.Name, hostName),
		Now:     TimestampPB(time.Now()),
	}
	return res, nil
}

func setupHealthCheckGateway(ctx context.Context, bindHealthCheck *string) *http.Server {
	logger := logger.With(zap.String("type", "hc"))
	mux := goji.NewMux()
	mux.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(ow http.ResponseWriter, r *http.Request) {
			begin := time.Now()
			w := healthcheck.NewResponseWriterWrapper(ow)
			defer func() {
				logger := logger
				if r := recover(); r != nil {
					var err error
					if e, ok := r.(error); !ok {
						err = fmt.Errorf("panic: %v", e)
					}
					logger = logger.With(zap.Stack("stack"), zap.Error(err))
					w.WriteHeader(http.StatusInternalServerError)
				}

				dur := time.Since(begin)
				ms := float64(dur) / float64(time.Millisecond)
				logger.With(zap.Int("status", w.StatusCode),
					zap.String("method", r.Method),
					zap.String("uri", r.RequestURI),
					zap.String("remote", r.RemoteAddr),
					zap.Float64("msec", ms),
				).
					Infof("done: %s", dur)
			}()

			h.ServeHTTP(w, r)
		})
	})

	mux.HandleFunc(pat.Get("/healthz"), func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-ctx.Done():
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			fmt.Fprintf(w, "!\n")
		}
	})
	server := &http.Server{
		Addr:    *bindHealthCheck,
		Handler: mux,
	}

	return server

}

func main() {
	delay := flag.Duration("delay", 0*time.Second, "Wait duration before shutdown")
	bind := flag.String("bind", ":3000", "addr:port")
	bindHealthCheck := flag.String("health-check", ":3001", "addr:port")

	flag.Parse()

	gs := grpc.NewServer(
		grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
			logger := logger
			ctx = log.ToContext(ctx, logger)
			begin := time.Now()
			defer func() {
				logger.Infof("dur=%s, err=%v", time.Since(begin), err)
			}()
			defer func() {
				if v := recover(); v != nil {
					if e, ok := v.(error); ok {
						err = e
					} else {
						err = fmt.Errorf("panic: %v", v)
					}
					logger = logger.With(zap.Stack("stack"))
				}
			}()

			return handler(ctx, req)
		}),
	)
	api.RegisterGreeterServer(gs, &helloServer{})
	reflection.Register(gs)

	lis := lo.Must(net.Listen("tcp", *bind))
	logger.Infof("Start to Serve: %s, %+v", lis.Addr(), gs.GetServiceInfo())
	go func() {
		if err := gs.Serve(lis); err != nil {
			logger.Fatalf("*** Failed to Serve(): %v", err)
		}
	}()

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	server := setupHealthCheckGateway(ctx, bindHealthCheck)
	logger.Infof("Start to serve HealthCheck gateway: %s", server.Addr)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("*** Failed to ListenAndServe(): %v", err)
		}
	}()

	<-ctx.Done()
	logger.Infof("Caught signal, Wait %s before shutdown", *delay)
	cancel()
	time.Sleep(*delay)

	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
		gs.GracefulStop()
	}

	logger.Info("exit")
}
