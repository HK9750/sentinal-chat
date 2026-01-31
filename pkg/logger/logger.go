package logger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	Logger *zap.Logger
}

var (
	ProductionMode  = "production"
	DevelopmentMode = "development"
)

func New(mode string) *Logger {
	var config zap.Config
	if mode == ProductionMode {
		config = zap.NewProductionConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	zapLogger, err := config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}
	return &Logger{Logger: zapLogger}
}

type ctxKey string

var RequestIdKey ctxKey = "request_id"
var UserIdKey ctxKey = "user_id"

func (l *Logger) withContext(ctx context.Context) *zap.Logger {
	var fields []zap.Field
	if ctx != nil {
		if requestId, ok := ctx.Value(RequestIdKey).(string); ok {
			fields = append(fields, zap.String(string(RequestIdKey), requestId))
		}
		if userId, ok := ctx.Value(UserIdKey).(string); ok {
			fields = append(fields, zap.String(string(UserIdKey), userId))
		}
	}
	return l.Logger.With(fields...)
}

var logger *Logger

func SetGlobalLogger(l *Logger) {
	logger = l
}

func GetGlobalLogger() *Logger {
	return logger
}

func (l *Logger) Infof(template string, args ...interface{}) {
	l.Logger.Sugar().Infof(template, args...)
}

func (l *Logger) Errorf(template string, args ...interface{}) {
	l.Logger.Sugar().Errorf(template, args...)
}
