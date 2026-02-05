package middleware

import (
	"time"

	"sentinal-chat/pkg/logger"

	"github.com/gin-gonic/gin"
)

// The purpose of this middleware is to log details about each HTTP request processed by the server, including the method, path, status code, and latency.
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
			log.Infof("%s %s %d %s", method, path, status, latency.String())
		}
	}
}
