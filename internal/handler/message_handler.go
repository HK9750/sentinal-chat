package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"
	sentinal_errors "sentinal-chat/pkg/errors"
	"sentinal-chat/pkg/logger"
)

type MessageHandler struct {
	service *services.MessageService
	logger  *logger.Logger
}

func NewMessageHandler(service *services.MessageService, l *logger.Logger) *MessageHandler {
	return &MessageHandler{service: service, logger: l}
}

func (h *MessageHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/conversations/:id/messages", h.History)
	router.GET("/messages/:id", h.Get)
}

func (h *MessageHandler) History(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}
	conversationID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, err)
		return
	}
	var query httpdto.MessageHistoryQuery
	_ = c.ShouldBindQuery(&query)
	items, err := h.service.History(c.Request.Context(), conversationID, userID, query.BeforeSeq, query.Limit)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, gin.H{"items": items})
}

func (h *MessageHandler) Get(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}
	messageID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, err)
		return
	}
	item, err := h.service.GetByID(c.Request.Context(), messageID, userID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, item)
}

func (h *MessageHandler) writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	message := "internal server error"
	switch {
	case errors.Is(err, sentinal_errors.ErrInvalidInput):
		status = http.StatusBadRequest
		code = "MESSAGE_INVALID_INPUT"
		message = "invalid input"
	case errors.Is(err, sentinal_errors.ErrUnauthorized):
		status = http.StatusUnauthorized
		code = "UNAUTHORIZED"
		message = "unauthorized"
	case errors.Is(err, sentinal_errors.ErrForbidden):
		status = http.StatusForbidden
		code = "FORBIDDEN"
		message = "forbidden"
	case errors.Is(err, sentinal_errors.ErrNotFound):
		status = http.StatusNotFound
		code = "MESSAGE_NOT_FOUND"
		message = "not found"
	case errors.Is(err, sentinal_errors.ErrConflict):
		status = http.StatusConflict
		code = "MESSAGE_CONFLICT"
		message = "conflict"
	case errors.Is(err, sentinal_errors.ErrServiceUnavailable):
		status = http.StatusServiceUnavailable
		code = "SERVICE_UNAVAILABLE"
		message = "service unavailable"
	}
	httpdto.WriteError(c, status, message, code)
}
