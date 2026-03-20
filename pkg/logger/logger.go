package logger

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	Logger    *zap.Logger
	component string
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

// Context keys for structured logging
type ctxKey string

const (
	RequestIdKey    ctxKey = "request_id"
	UserIdKey       ctxKey = "user_id"
	ConversationKey ctxKey = "conversation_id"
	SessionKey      ctxKey = "session_id"
	DeviceKey       ctxKey = "device_id"
	TraceKey        ctxKey = "trace_id"
	SpanKey         ctxKey = "span_id"
	OperationKey    ctxKey = "operation"
)

// LogLevel represents log severity
type LogLevel string

const (
	LevelDebug LogLevel = "DEBUG"
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
	LevelFatal LogLevel = "FATAL"
)

// Init initializes the global logger singleton with the specified mode.
func Init(mode string) {
	loggerOnce.Do(func() {
		defaultLoggerMode = mode
		globalLogger = New(mode)
	})
}

// InitWithConfig initializes the global logger with a custom zap configuration.
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

// New creates a new Logger instance
func New(mode string) *Logger {
	var config zap.Config
	if mode == ProductionMode {
		config = zap.NewProductionConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.LevelKey = "level"
		config.EncoderConfig.MessageKey = "message"
		config.EncoderConfig.CallerKey = "caller"
		config.EncoderConfig.StacktraceKey = "stacktrace"
		config.OutputPaths = []string{"stdout"}
		config.ErrorOutputPaths = []string{"stderr"}
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000")
	}

	zapLogger, err := config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}
	return &Logger{Logger: zapLogger}
}

// WithComponent creates a child logger with a component name
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger:    l.Logger.With(zap.String("component", component)),
		component: component,
	}
}

// WithFields creates a child logger with additional fields
func (l *Logger) WithFields(fields ...zap.Field) *Logger {
	return &Logger{
		Logger:    l.Logger.With(fields...),
		component: l.component,
	}
}

// withContext extracts context values and adds them as fields
func (l *Logger) withContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return l.Logger
	}

	var fields []zap.Field

	if requestId, ok := ctx.Value(RequestIdKey).(string); ok && requestId != "" {
		fields = append(fields, zap.String("request_id", requestId))
	}
	if userId, ok := ctx.Value(UserIdKey).(string); ok && userId != "" {
		fields = append(fields, zap.String("user_id", userId))
	}
	if conversationId, ok := ctx.Value(ConversationKey).(string); ok && conversationId != "" {
		fields = append(fields, zap.String("conversation_id", conversationId))
	}
	if sessionId, ok := ctx.Value(SessionKey).(string); ok && sessionId != "" {
		fields = append(fields, zap.String("session_id", sessionId))
	}
	if deviceId, ok := ctx.Value(DeviceKey).(string); ok && deviceId != "" {
		fields = append(fields, zap.String("device_id", deviceId))
	}
	if traceId, ok := ctx.Value(TraceKey).(string); ok && traceId != "" {
		fields = append(fields, zap.String("trace_id", traceId))
	}
	if spanId, ok := ctx.Value(SpanKey).(string); ok && spanId != "" {
		fields = append(fields, zap.String("span_id", spanId))
	}
	if operation, ok := ctx.Value(OperationKey).(string); ok && operation != "" {
		fields = append(fields, zap.String("operation", operation))
	}

	if len(fields) == 0 {
		return l.Logger
	}
	return l.Logger.With(fields...)
}

// SetGlobalLogger sets a custom logger as the global instance.
func SetGlobalLogger(l *Logger) {
	globalLogger = l
}

// GetGlobalLogger returns the global logger instance.
func GetGlobalLogger() *Logger {
	if globalLogger == nil {
		Init(defaultLoggerMode)
	}
	return globalLogger
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// ---------- Basic Logging Methods ----------

func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, fields...)
}

func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(msg, fields...)
}

// ---------- Formatted Logging Methods ----------

func (l *Logger) Debugf(template string, args ...interface{}) {
	l.Logger.Sugar().Debugf(template, args...)
}

func (l *Logger) Infof(template string, args ...interface{}) {
	l.Logger.Sugar().Infof(template, args...)
}

func (l *Logger) Warnf(template string, args ...interface{}) {
	l.Logger.Sugar().Warnf(template, args...)
}

func (l *Logger) Errorf(template string, args ...interface{}) {
	l.Logger.Sugar().Errorf(template, args...)
}

func (l *Logger) Fatalf(template string, args ...interface{}) {
	l.Logger.Sugar().Fatalf(template, args...)
}

// ---------- Structured Logging Methods ----------

func (l *Logger) Debugw(message string, keysAndValues ...interface{}) {
	l.Logger.Sugar().Debugw(message, keysAndValues...)
}

func (l *Logger) Infow(message string, keysAndValues ...interface{}) {
	l.Logger.Sugar().Infow(message, keysAndValues...)
}

func (l *Logger) Warnw(message string, keysAndValues ...interface{}) {
	l.Logger.Sugar().Warnw(message, keysAndValues...)
}

func (l *Logger) Errorw(message string, keysAndValues ...interface{}) {
	l.Logger.Sugar().Errorw(message, keysAndValues...)
}

func (l *Logger) Fatalw(message string, keysAndValues ...interface{}) {
	l.Logger.Sugar().Fatalw(message, keysAndValues...)
}

// ---------- Context-Aware Logging Methods ----------

func (l *Logger) DebugCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.withContext(ctx).Debug(msg, fields...)
}

func (l *Logger) InfoCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.withContext(ctx).Info(msg, fields...)
}

func (l *Logger) WarnCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.withContext(ctx).Warn(msg, fields...)
}

func (l *Logger) ErrorCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.withContext(ctx).Error(msg, fields...)
}

func (l *Logger) InfofCtx(ctx context.Context, template string, args ...interface{}) {
	l.withContext(ctx).Sugar().Infof(template, args...)
}

func (l *Logger) ErrorfCtx(ctx context.Context, template string, args ...interface{}) {
	l.withContext(ctx).Sugar().Errorf(template, args...)
}

func (l *Logger) WarnfCtx(ctx context.Context, template string, args ...interface{}) {
	l.withContext(ctx).Sugar().Warnf(template, args...)
}

func (l *Logger) DebugfCtx(ctx context.Context, template string, args ...interface{}) {
	l.withContext(ctx).Sugar().Debugf(template, args...)
}

func (l *Logger) InfowCtx(ctx context.Context, message string, keysAndValues ...interface{}) {
	l.withContext(ctx).Sugar().Infow(message, keysAndValues...)
}

func (l *Logger) ErrorwCtx(ctx context.Context, message string, keysAndValues ...interface{}) {
	l.withContext(ctx).Sugar().Errorw(message, keysAndValues...)
}

func (l *Logger) WarnwCtx(ctx context.Context, message string, keysAndValues ...interface{}) {
	l.withContext(ctx).Sugar().Warnw(message, keysAndValues...)
}

func (l *Logger) DebugwCtx(ctx context.Context, message string, keysAndValues ...interface{}) {
	l.withContext(ctx).Sugar().Debugw(message, keysAndValues...)
}

// ---------- Operation Logging Helpers ----------

// LogOperation logs the start of an operation with timing
func (l *Logger) LogOperation(ctx context.Context, operation string, fn func() error) error {
	start := time.Now()
	logger := l.withContext(ctx).With(zap.String("operation", operation))

	logger.Info("operation.started")
	err := fn()
	duration := time.Since(start)

	if err != nil {
		logger.Error("operation.failed",
			zap.Duration("duration_ms", duration),
			zap.Error(err),
		)
		return err
	}

	logger.Info("operation.completed",
		zap.Duration("duration_ms", duration),
	)
	return nil
}

// LogWebSocketEvent logs WebSocket-related events
func (l *Logger) LogWebSocketEvent(ctx context.Context, event string, data map[string]interface{}) {
	fields := []interface{}{"event", event}
	for k, v := range data {
		fields = append(fields, k, v)
	}
	l.withContext(ctx).Sugar().Infow("websocket.event", fields...)
}

// LogDatabaseQuery logs database query execution
func (l *Logger) LogDatabaseQuery(ctx context.Context, query string, duration time.Duration, rowsAffected int64, err error) {
	// Truncate long queries for logging
	if len(query) > 200 {
		query = query[:200] + "..."
	}

	if err != nil {
		l.withContext(ctx).Sugar().Errorw("database.query.failed",
			"query", query,
			"duration_ms", duration.Milliseconds(),
			"error", err.Error(),
		)
		return
	}

	l.withContext(ctx).Sugar().Debugw("database.query.executed",
		"query", query,
		"duration_ms", duration.Milliseconds(),
		"rows_affected", rowsAffected,
	)
}

// LogRedisOperation logs Redis operations
func (l *Logger) LogRedisOperation(ctx context.Context, operation string, key string, duration time.Duration, err error) {
	if err != nil {
		l.withContext(ctx).Sugar().Errorw("redis.operation.failed",
			"operation", operation,
			"key", key,
			"duration_ms", duration.Milliseconds(),
			"error", err.Error(),
		)
		return
	}

	l.withContext(ctx).Sugar().Debugw("redis.operation.completed",
		"operation", operation,
		"key", key,
		"duration_ms", duration.Milliseconds(),
	)
}

// LogHTTPRequest logs HTTP request details
func (l *Logger) LogHTTPRequest(ctx context.Context, method, path string, status int, latency time.Duration, userID, requestID string) {
	level := zapcore.InfoLevel
	if status >= 500 {
		level = zapcore.ErrorLevel
	} else if status >= 400 {
		level = zapcore.WarnLevel
	}

	fields := []zap.Field{
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", status),
		zap.Duration("latency_ms", latency),
	}

	if requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}
	if userID != "" {
		fields = append(fields, zap.String("user_id", userID))
	}

	l.Logger.Check(level, "http.request").Write(fields...)
}

// LogError logs an error with stack trace
func (l *Logger) LogError(ctx context.Context, operation string, err error, additionalFields ...zap.Field) {
	fields := []zap.Field{
		zap.String("operation", operation),
		zap.Error(err),
		zap.String("stack", getStackTrace()),
	}
	fields = append(fields, additionalFields...)
	l.withContext(ctx).Error("error.occurred", fields...)
}

// LogPanic logs panic recovery
func (l *Logger) LogPanic(ctx context.Context, recovered interface{}) {
	l.withContext(ctx).Error("panic.recovered",
		zap.Any("panic", recovered),
		zap.String("stack", getStackTrace()),
	)
}

// LogAudit logs audit events for security/compliance
func (l *Logger) LogAudit(ctx context.Context, action string, resourceType string, resourceID string, details map[string]interface{}) {
	fields := []interface{}{
		"action", action,
		"resource_type", resourceType,
		"resource_id", resourceID,
		"timestamp", time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range details {
		fields = append(fields, k, v)
	}
	l.withContext(ctx).Sugar().Infow("audit.event", fields...)
}

// ---------- Helper Functions ----------

// getStackTrace returns a formatted stack trace
func getStackTrace() string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var sb strings.Builder
	for {
		frame, more := frames.Next()
		// Skip runtime and testing frames
		if strings.Contains(frame.File, "runtime/") {
			if !more {
				break
			}
			continue
		}
		fmt.Fprintf(&sb, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	return sb.String()
}

// ContextWithRequestID adds a request ID to context
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIdKey, requestID)
}

// ContextWithUserID adds a user ID to context
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIdKey, userID)
}

// ContextWithConversation adds a conversation ID to context
func ContextWithConversation(ctx context.Context, conversationID string) context.Context {
	return context.WithValue(ctx, ConversationKey, conversationID)
}

// ContextWithOperation adds an operation name to context
func ContextWithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, OperationKey, operation)
}

// ContextWithTracing adds trace and span IDs to context
func ContextWithTracing(ctx context.Context, traceID, spanID string) context.Context {
	ctx = context.WithValue(ctx, TraceKey, traceID)
	ctx = context.WithValue(ctx, SpanKey, spanID)
	return ctx
}

// GetRequestID extracts request ID from context
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIdKey).(string); ok {
		return requestID
	}
	return ""
}

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIdKey).(string); ok {
		return userID
	}
	return ""
}

// Environment helpers
func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
