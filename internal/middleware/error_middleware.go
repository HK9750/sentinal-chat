package middleware

import (
	"sentinal-chat/internal/transport/httpdto"
	"sentinal-chat/pkg/logger"

	"github.com/gin-gonic/gin"
)

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
		c.JSON(c.Writer.Status(), httpdto.NewErrorResponse(err.Error(), "INTERNAL_ERROR"))
	}
}
