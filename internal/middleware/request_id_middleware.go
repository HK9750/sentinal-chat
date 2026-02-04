package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"sentinal-chat/pkg/logger"

	"github.com/gin-gonic/gin"
)

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Writer.Header().Set("X-Request-Id", requestID)
		ctx := context.WithValue(c.Request.Context(), logger.RequestIdKey, requestID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func newRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}
