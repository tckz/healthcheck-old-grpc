package log

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New() (*zap.SugaredLogger, error) {

	encConfig := zap.NewProductionEncoderConfig()
	encConfig.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(t.Format("2006-01-02T15:04:05.000Z07:00"))
	}

	zc := zap.Config{
		DisableCaller:    true,
		Level:            zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Development:      false,
		Sampling:         nil,
		Encoding:         "json",
		EncoderConfig:    encConfig,
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	zl, err := zc.Build()
	if err != nil {
		return nil, err
	}

	return zl.Sugar(), nil
}
