package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sentinal_errors "sentinal-chat/pkg/errors"
)

func mustUserID(c *gin.Context) (uuid.UUID, bool) {
	value, ok := c.Get("user_id")
	if !ok {
		return uuid.Nil, false
	}
	userID, ok := value.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return uuid.Nil, false
	}
	return userID, true
}

func parseUUIDParam(c *gin.Context, paramName string) (uuid.UUID, error) {
	value := strings.TrimSpace(c.Param(paramName))
	if value == "" {
		return uuid.Nil, sentinal_errors.ErrInvalidInput
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, sentinal_errors.ErrInvalidInput
	}
	return id, nil
}
