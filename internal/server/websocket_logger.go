package server

import (
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// WebSocketLogger provides structured logging for WebSocket events
type WebSocketLogger struct {
	logger *zap.Logger
}

// NewWebSocketLogger creates a new WebSocket logger
func NewWebSocketLogger() *WebSocketLogger {
	return &WebSocketLogger{
		logger: zap.L().With(zap.String("component", "websocket")),
	}
}

// Info logs info level event
func (l *WebSocketLogger) Info(event string, userID uuid.UUID, clientID string, fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.String("event", event),
		zap.String("user_id", userID.String()),
		zap.String("client_id", clientID),
	}, fields...)
	l.logger.Info("websocket_event", allFields...)
}

// Error logs error level event
func (l *WebSocketLogger) Error(event string, userID uuid.UUID, clientID string, err error, fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.String("event", event),
		zap.String("user_id", userID.String()),
		zap.String("client_id", clientID),
		zap.Error(err),
	}, fields...)
	l.logger.Error("websocket_error", allFields...)
}

// Warn logs warning level event
func (l *WebSocketLogger) Warn(event string, userID uuid.UUID, clientID string, fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.String("event", event),
		zap.String("user_id", userID.String()),
		zap.String("client_id", clientID),
	}, fields...)
	l.logger.Warn("websocket_warning", allFields...)
}
