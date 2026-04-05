package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(level string) (*zap.Logger, error) {
	lvl := zap.NewAtomicLevel()
	if err := lvl.UnmarshalText([]byte(strings.ToLower(level))); err != nil {
		return nil, err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = lvl
	cfg.Encoding = "json"
	cfg.EncoderConfig.TimeKey = "time"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return cfg.Build()
}
