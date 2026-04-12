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

type NotificationHandler struct {
	service *services.NotificationService
	logger  *logger.Logger
}

func NewNotificationHandler(service *services.NotificationService, l *logger.Logger) *NotificationHandler {
	return &NotificationHandler{service: service, logger: l.WithComponent("notification_handler")}
}

func (h *NotificationHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/notifications", h.List)
	router.POST("/notifications/:id/read", h.MarkRead)
	router.POST("/notifications/read-all", h.MarkAllRead)

	router.GET("/users/notification-settings", h.GetSettings)
	router.PATCH("/users/notification-settings", h.UpdateSettings)
}

func (h *NotificationHandler) List(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}

	var query httpdto.NotificationQuery
	_ = c.ShouldBindQuery(&query)
	items, total, err := h.service.List(c.Request.Context(), userID, query.Page, query.Limit, query.UnreadOnly)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, httpdto.ListPayload[services.NotificationView]{Items: items, Total: total})
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}
	notificationID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, err)
		return
	}

	if err := h.service.MarkRead(c.Request.Context(), userID, notificationID); err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, httpdto.MarkNotificationReadPayload{Read: true})
}

func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}
	updated, err := h.service.MarkAllRead(c.Request.Context(), userID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, httpdto.MarkAllNotificationsReadPayload{Updated: updated})
}

func (h *NotificationHandler) GetSettings(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}
	settings, err := h.service.GetSettings(c.Request.Context(), userID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, settings)
}

func (h *NotificationHandler) UpdateSettings(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return
	}

	var req httpdto.UpdateNotificationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	settings, err := h.service.UpdateSettings(c.Request.Context(), userID, services.UpdateNotificationSettingsInput{
		InAppEnabled:       req.InAppEnabled,
		SoundEnabled:       req.SoundEnabled,
		ShowMessagePreview: req.ShowMessagePreview,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpdto.WriteSuccess(c, http.StatusOK, settings)
}

func (h *NotificationHandler) writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	message := "internal server error"

	switch {
	case errors.Is(err, sentinal_errors.ErrInvalidInput):
		status = http.StatusBadRequest
		code = "NOTIFICATION_INVALID_INPUT"
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
		code = "NOTIFICATION_NOT_FOUND"
		message = "not found"
	case errors.Is(err, sentinal_errors.ErrConflict):
		status = http.StatusConflict
		code = "NOTIFICATION_CONFLICT"
		message = "conflict"
	case errors.Is(err, sentinal_errors.ErrServiceUnavailable):
		status = http.StatusServiceUnavailable
		code = "SERVICE_UNAVAILABLE"
		message = "service unavailable"
	}

	httpdto.WriteError(c, status, message, code)
}
