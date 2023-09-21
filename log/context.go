package log

import (
	"context"

	"go.uber.org/zap"
)

type loggerKey struct{}

var nop = zap.NewNop().Sugar()

func Extract(ctx context.Context) *zap.SugaredLogger {
	if v, ok := ctx.Value(loggerKey{}).(*zap.SugaredLogger); ok {
		return v
	}
	return nop
}

func ToContext(ctx context.Context, logger *zap.SugaredLogger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}
