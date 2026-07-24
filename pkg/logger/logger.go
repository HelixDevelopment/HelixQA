package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(level string, format string) (*zap.Logger, error) {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}

	var cfg zap.Config
	if format == "console" {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		cfg.EncoderConfig.EncodeDuration = zapcore.MillisDurationEncoder
	} else {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		cfg.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	}

	cfg.Level = zap.NewAtomicLevelAt(lvl)
	if format == "console" {
		cfg.OutputPaths = []string{"stderr"}
		cfg.ErrorOutputPaths = []string{"stderr"}
	}

	return cfg.Build(zap.AddCallerSkip(0))
}

func FromContext(logger *zap.Logger, fields ...zap.Field) *zap.Logger {
	return logger.With(fields...)
}
