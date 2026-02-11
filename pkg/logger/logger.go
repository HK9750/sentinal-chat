package logger

import (
	"context"
	"sync"

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

// Singleton instance variables
var (
	globalLogger      *Logger
	loggerOnce        sync.Once
	defaultLoggerMode = DevelopmentMode
)

// Init initializes the global logger singleton with the specified mode.
// This function is safe to call multiple times - only the first call will create the logger.
// Mode can be "production" or "development".
func Init(mode string) {
	loggerOnce.Do(func() {
		defaultLoggerMode = mode
		globalLogger = New(mode)
	})
}

// InitWithConfig initializes the global logger with a custom zap configuration.
// This function is safe to call multiple times - only the first call will create the logger.
func InitWithConfig(config zap.Config) {
	loggerOnce.Do(func() {
		zapLogger, err := config.Build(zap.AddCallerSkip(1))
		if err != nil {
			panic(err)
		}
		globalLogger = &Logger{Logger: zapLogger}
	})
}

// IsInitialized returns true if the global logger has been initialized
func IsInitialized() bool {
	return globalLogger != nil
}

// New creates a new Logger instance (not singleton - use for custom loggers).
// For the global singleton logger, use Init() and GetGlobalLogger() instead.
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

// SetGlobalLogger sets a custom logger as the global instance.
// Use this if you need custom configuration. For standard initialization, use Init().
func SetGlobalLogger(l *Logger) {
	globalLogger = l
}

// GetGlobalLogger returns the global logger instance.
// If Init() has not been called, it will auto-initialize with Development mode.
// This ensures the logger is always available, but explicit initialization is recommended.
func GetGlobalLogger() *Logger {
	if globalLogger == nil {
		Init(defaultLoggerMode)
	}
	return globalLogger
}

func (l *Logger) Infof(template string, args ...interface{}) {
	l.Logger.Sugar().Infof(template, args...)
}

func (l *Logger) Errorf(template string, args ...interface{}) {
	l.Logger.Sugar().Errorf(template, args...)
}
