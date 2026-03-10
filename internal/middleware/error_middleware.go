package middleware

import (
	"sentinal-chat/pkg/logger"

	"github.com/gin-gonic/gin"
)

// ErrorHandler catches any gin errors registered during request processing
// and logs them before returning a generic error response.
func ErrorHandler(l *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err
		if l != nil {
			l.Errorf("request error: %s", err.Error())
		}
		c.JSON(c.Writer.Status(), gin.H{
			"success": false,
			"error":   err.Error(),
			"code":    "INTERNAL_ERROR",
		})
	}
}
