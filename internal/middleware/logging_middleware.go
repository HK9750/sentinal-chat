package middleware

import (
	"strings"
	"time"

	"sentinal-chat/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// LoggingMiddleware creates a comprehensive logging middleware
func LoggingMiddleware(l *logger.Logger) gin.HandlerFunc {
	if l == nil {
		l = logger.GetGlobalLogger()
	}
	log := l.WithComponent("http")

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		method := c.Request.Method
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()
		requestID := c.Writer.Header().Get("X-Request-Id")

		// Log request start (debug level)
		log.Debug("request.started",
			zap.String("method", method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("client_ip", clientIP),
			zap.String("request_id", requestID),
		)

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)
		status := c.Writer.Status()
		responseSize := c.Writer.Size()

		// Extract user ID if set
		userID := ""
		if userValue, exists := c.Get("user_id"); exists {
			if parsedUserID, ok := userValue.(uuid.UUID); ok {
				userID = parsedUserID.String()
			} else if parsedUserID, ok := userValue.(interface{ String() string }); ok {
				userID = parsedUserID.String()
			}
		}

		// Build common fields
		fields := []zap.Field{
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.Int64("latency_ms", latency.Milliseconds()),
			zap.Int("response_size", responseSize),
			zap.String("client_ip", clientIP),
			zap.String("request_id", requestID),
		}

		if userID != "" {
			fields = append(fields, zap.String("user_id", userID))
		}

		if query != "" {
			fields = append(fields, zap.String("query", query))
		}

		// Add error information if present
		if len(c.Errors) > 0 {
			errorMessages := make([]string, len(c.Errors))
			for i, err := range c.Errors {
				errorMessages[i] = err.Error()
			}
			fields = append(fields, zap.Strings("errors", errorMessages))
		}

		// Log based on status code
		if status >= 500 {
			fields = append(fields, zap.String("user_agent", userAgent))
			log.Error("request.completed", fields...)
		} else if status >= 400 {
			log.Warn("request.completed", fields...)
		} else {
			log.Info("request.completed", fields...)
		}
	}
}

// WebSocketLoggingMiddleware creates logging middleware for WebSocket connections
func WebSocketLoggingMiddleware(l *logger.Logger) gin.HandlerFunc {
	if l == nil {
		l = logger.GetGlobalLogger()
	}
	log := l.WithComponent("websocket")

	return func(c *gin.Context) {
		start := time.Now()
		requestID := c.Writer.Header().Get("X-Request-Id")
		clientIP := c.ClientIP()

		// Extract user ID
		userID := ""
		if userValue, exists := c.Get("user_id"); exists {
			if parsedUserID, ok := userValue.(uuid.UUID); ok {
				userID = parsedUserID.String()
			}
		}

		log.Info("connection.initiated",
			zap.String("client_ip", clientIP),
			zap.String("request_id", requestID),
			zap.String("user_id", userID),
		)

		c.Next()

		duration := time.Since(start)
		log.Info("connection.closed",
			zap.String("request_id", requestID),
			zap.String("user_id", userID),
			zap.Duration("duration", duration),
		)
	}
}

// SensitivePathFilter checks if a path contains sensitive data that shouldn't be logged
func SensitivePathFilter(path string) bool {
	sensitivePatterns := []string{
		"/auth/login",
		"/auth/register",
		"/auth/refresh",
		"/auth/password",
	}
	for _, pattern := range sensitivePatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// RecoveryMiddleware creates a panic recovery middleware with logging
func RecoveryMiddleware(l *logger.Logger) gin.HandlerFunc {
	if l == nil {
		l = logger.GetGlobalLogger()
	}
	log := l.WithComponent("recovery")

	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				requestID := c.Writer.Header().Get("X-Request-Id")

				log.LogPanic(c.Request.Context(), recovered)
				log.Error("panic.recovery",
					zap.Any("panic", recovered),
					zap.String("request_id", requestID),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)

				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}
