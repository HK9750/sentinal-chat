package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"
	sentinal_errors "sentinal-chat/pkg/errors"
	"sentinal-chat/pkg/logger"
)

type ConversationHandler struct {
	service *services.ConversationService
	logger  *logger.Logger
}

func NewConversationHandler(service *services.ConversationService, l *logger.Logger) *ConversationHandler {
	return &ConversationHandler{service: service, logger: l}
}

func (h *ConversationHandler) RegisterRoutes(router gin.IRouter) {
	router.POST("/conversations", h.Create)
	router.GET("/conversations", h.List)
	router.GET("/conversations/:id", h.Get)
	router.POST("/conversations/:id/participants", h.AddParticipant)
	router.DELETE("/conversations/:id/participants/:user_id", h.RemoveParticipant)
	router.GET("/conversations/:id/participants", h.Participants)
	router.POST("/conversations/:id/clear", h.Clear)
}

func (h *ConversationHandler) Create(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}
	var req httpdto.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}
	participantIDs, err := parseUUIDList(req.ParticipantIDs)
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}
	conv, err := h.service.Create(c.Request.Context(), services.CreateConversationInput{
		CreatorID:        userID,
		Type:             req.Type,
		Subject:          req.Subject,
		Description:      req.Description,
		AvatarURL:        req.AvatarURL,
		ParticipantIDs:   participantIDs,
		DisappearingMode: req.DisappearingMode,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusCreated, conv)
}

func (h *ConversationHandler) List(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}
	var query httpdto.ConversationQuery
	_ = c.ShouldBindQuery(&query)
	items, total, err := h.service.List(c.Request.Context(), userID, query.Page, query.Limit)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, httpdto.ListPayload[services.ConversationView]{Items: items, Total: total})
}

func (h *ConversationHandler) Get(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}
	conversationID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}
	conv, err := h.service.Get(c.Request.Context(), conversationID, userID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, conv)
}

func (h *ConversationHandler) AddParticipant(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}
	conversationID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}
	var req httpdto.AddParticipantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}
	targetID, err := uuid.Parse(strings.TrimSpace(req.UserID))
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}
	conv, err := h.service.AddParticipant(c.Request.Context(), services.AddParticipantInput{ConversationID: conversationID, ActorID: userID, UserID: targetID, Role: req.Role})
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, conv)
}

func (h *ConversationHandler) RemoveParticipant(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}
	conversationID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}
	targetID, err := parseUUIDParam(c, "user_id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}
	conv, err := h.service.RemoveParticipant(c.Request.Context(), services.RemoveParticipantInput{ConversationID: conversationID, ActorID: userID, UserID: targetID})
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, conv)
}

func (h *ConversationHandler) Participants(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}
	conversationID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}
	conv, err := h.service.Get(c.Request.Context(), conversationID, userID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, httpdto.ItemsPayload[services.ParticipantView]{Items: conv.Participants})
}

func (h *ConversationHandler) Clear(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}
	conversationID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}
	if err := h.service.Clear(c.Request.Context(), services.ClearConversationInput{ConversationID: conversationID, ActorID: userID}); err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, httpdto.ClearConversationPayload{Cleared: true})
}

func (h *ConversationHandler) writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	message := "internal server error"
	switch {
	case errors.Is(err, sentinal_errors.ErrInvalidInput):
		status = http.StatusBadRequest
		code = "CONVERSATION_INVALID_INPUT"
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
		code = "CONVERSATION_NOT_FOUND"
		message = "not found"
	case errors.Is(err, sentinal_errors.ErrAlreadyExists), errors.Is(err, sentinal_errors.ErrConflict):
		status = http.StatusConflict
		code = "CONVERSATION_CONFLICT"
		message = "conflict"
	case errors.Is(err, sentinal_errors.ErrServiceUnavailable):
		status = http.StatusServiceUnavailable
		code = "SERVICE_UNAVAILABLE"
		message = "service unavailable"
	}
	if status >= http.StatusInternalServerError && h.logger != nil {
		h.logger.Errorf("conversation handler error: %v", err)
	}
	httpdto.WriteError(c, status, message, code)
}

func parseUUIDList(items []string) ([]uuid.UUID, error) {
	parsed := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		id, err := uuid.Parse(strings.TrimSpace(item))
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, id)
	}
	return parsed, nil
}
