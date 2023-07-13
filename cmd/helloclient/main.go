package main

import (
	"flag"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tckz/healthcheck-old-grpc/api"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	defaultName = "world"
)

func main() {
	optTimeout := flag.Duration("timeout", 3*time.Second, "Seconds to timeout")
	optServer := flag.String("server", "127.0.0.1:3000", "Server addr:port")
	optInsecure := flag.Bool("insecure", false, "Use http instead of https")
	flag.Parse()

	logrus.SetFormatter(&logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
	})
	logrus.Infof("Server: %s", *optServer)

	var grpcOpts []grpc.DialOption
	if *optInsecure {
		grpcOpts = append(grpcOpts, grpc.WithInsecure())
	} else {
		grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")))
	}
	conn, err := grpc.Dial(*optServer, grpcOpts...)
	if err != nil {
		logrus.Fatalf("*** Failed to Dial %s: %v", *optServer, err)
	}
	defer conn.Close()
	client := api.NewGreeterClient(conn)

	name := defaultName
	if flag.NArg() >= 1 {
		name = flag.Arg(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *optTimeout)
	defer cancel()
	r, err := client.SayHello(ctx, &api.HelloRequest{Name: name})
	if err != nil {
		logrus.Fatalf("*** Failed to SayHello: %v", err)
	}

	now := time.Unix(r.Now.Seconds, int64(r.Now.Nanos))
	logrus.Printf("Response: Message=%s, Now=%s", r.Message, now)
}
