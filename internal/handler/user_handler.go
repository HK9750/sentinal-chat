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

type UserHandler struct {
	service *services.UserService
	logger  *logger.Logger
}

func NewUserHandler(service *services.UserService, l *logger.Logger) *UserHandler {
	return &UserHandler{service: service, logger: l}
}

func (h *UserHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/users/search", h.Search)
	router.GET("/users/contacts", h.ListContacts)
	router.POST("/users/contacts", h.AddContact)
	router.DELETE("/users/contacts/:contact_user_id", h.RemoveContact)
}

func (h *UserHandler) Search(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}

	var query httpdto.UserSearchQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	items, total, err := h.service.SearchUsers(c.Request.Context(), userID, query.Query, query.Page, query.Limit)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusOK, httpdto.ListPayload[services.UserSearchView]{Items: items, Total: total})
}

func (h *UserHandler) ListContacts(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}

	items, err := h.service.ListContacts(c.Request.Context(), userID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusOK, httpdto.ItemsPayload[services.ContactView]{Items: items})
}

func (h *UserHandler) AddContact(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}

	var req httpdto.AddContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	contactUserID, err := parseUUIDParamFromValue(req.ContactUserID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	contact, err := h.service.AddContact(c.Request.Context(), services.AddContactInput{
		UserID:        userID,
		ContactUserID: contactUserID,
		Nickname:      req.Nickname,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusCreated, contact)
}

func (h *UserHandler) RemoveContact(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}

	contactUserID, err := parseUUIDParam(c, "contact_user_id")
	if err != nil {
		h.writeError(c, err)
		return
	}

	if err := h.service.RemoveContact(c.Request.Context(), userID, contactUserID); err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusOK, httpdto.RemoveContactPayload{Removed: true})
}

func (h *UserHandler) writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	message := "internal server error"

	switch {
	case errors.Is(err, sentinal_errors.ErrInvalidInput):
		status = http.StatusBadRequest
		code = "USER_INVALID_INPUT"
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
		code = "USER_NOT_FOUND"
		message = "not found"
	case errors.Is(err, sentinal_errors.ErrAlreadyExists), errors.Is(err, sentinal_errors.ErrConflict):
		status = http.StatusConflict
		code = "USER_CONFLICT"
		message = "conflict"
	case errors.Is(err, sentinal_errors.ErrServiceUnavailable):
		status = http.StatusServiceUnavailable
		code = "SERVICE_UNAVAILABLE"
		message = "service unavailable"
	}

	if status >= http.StatusInternalServerError && h.logger != nil {
		h.logger.Errorf("user handler error: %v", err)
	}

	httpdto.WriteError(c, status, message, code)
}

func parseUUIDParamFromValue(value string) (uuid.UUID, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return uuid.Nil, sentinal_errors.ErrInvalidInput
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return uuid.Nil, sentinal_errors.ErrInvalidInput
	}
	return parsed, nil
}
