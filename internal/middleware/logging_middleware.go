package middleware

import (
	"strings"
	"time"

	"sentinal-chat/pkg/logger"

	"github.com/gin-gonic/gin"
)

func LoggingMiddleware(l *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		log := l
		if log == nil {
			log = logger.GetGlobalLogger()
		}
		if log != nil {
			requestID := strings.TrimSpace(c.Writer.Header().Get("X-Request-Id"))
			userID := ""
			if userValue, exists := c.Get("user_id"); exists {
				if parsedUserID, ok := userValue.(interface{ String() string }); ok {
					userID = parsedUserID.String()
				}
			}
			log.Infow("http_request",
				"method", method,
				"path", path,
				"status", status,
				"latency_ms", latency.Milliseconds(),
				"request_id", requestID,
				"user_id", userID,
			)
		}
	}
}
