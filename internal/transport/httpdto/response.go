package httpdto

import "github.com/gin-gonic/gin"

type Response[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
}

func WriteSuccess[T any](c *gin.Context, status int, data T) {
	c.JSON(status, Response[T]{
		Success: true,
		Data:    data,
	})
}

func WriteError(c *gin.Context, status int, message, code string) {
	c.JSON(status, Response[any]{
		Success: false,
		Error:   message,
		Code:    code,
	})
}
