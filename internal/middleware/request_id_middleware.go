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
		// set up a new context with the request ID
		ctx := context.WithValue(c.Request.Context(), logger.RequestIdKey, requestID)
		// update the request with the new context
		c.Request = c.Request.WithContext(ctx)
		// proceed to the next middleware/handler
		c.Next()
	}
}

func newRequestID() string {
	buf := make([]byte, 16)
	/// if i do make([]byte, 16), then buf will be a byte slice of length 16 initialized to all zeros: [0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]

	// rand.read(buf) will look like [0x9f, 0x2b, 0x7c, 0x6e, 0x1a, 0x4d, 0x8f, 0x3c, 0x5e, 0x0a, 0x9b, 0x2d, 0x7c, 0x6f, 0x4e, 0x1a]
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	// hex.EncodeToString(buf) will look like "9f2b7c6e1a4d8f3c5e0a9b2d7c6f4e1a"
	// why is this method used instead of just using uuid.New()? Because uuid.New() generates a UUID with hyphens, and we want a compact representation without hyphens.
	return hex.EncodeToString(buf)
}
