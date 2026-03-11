package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/domain/upload"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"
	sentinal_errors "sentinal-chat/pkg/errors"
	"sentinal-chat/pkg/logger"
)

type UploadHandler struct {
	service *services.UploadService
	logger  *logger.Logger
}

func NewUploadHandler(service *services.UploadService, l *logger.Logger) *UploadHandler {
	return &UploadHandler{service: service, logger: l}
}

func (h *UploadHandler) RegisterRoutes(router gin.IRouter) {
	router.POST("/uploads", h.CreateUploadSession)
	router.GET("/uploads/:id", h.GetUploadSession)
	router.GET("/uploads", h.ListUploads)
	router.PATCH("/uploads/:id/progress", h.UpdateUploadProgress)
	router.POST("/uploads/:id/complete", h.CompleteUpload)
	router.POST("/uploads/:id/fail", h.FailUpload)

	router.POST("/attachments", h.CreateAttachment)
	router.GET("/attachments/:id", h.GetAttachment)
	router.POST("/attachments/:id/viewed", h.MarkAttachmentViewed)
	router.GET("/messages/:id/attachments", h.GetMessageAttachments)
}

func (h *UploadHandler) CreateUploadSession(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	var req httpdto.CreateUploadSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	out, err := h.service.CreateUploadSession(c.Request.Context(), services.CreateUploadSessionInput{
		UploaderID: userID,
		Filename:   req.Filename,
		MimeType:   req.MimeType,
		SizeBytes:  req.SizeBytes,
		ChunkSize:  req.ChunkSize,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusCreated, httpdto.CreateUploadSessionPayload{
		Upload: toUploadPayload(out.Session),
		UploadTarget: httpdto.UploadTargetPayload{
			URL:     out.URL,
			Headers: out.Headers,
		},
	})
}

func (h *UploadHandler) GetUploadSession(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	sessionID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	session, err := h.service.GetUploadSession(c.Request.Context(), userID, sessionID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusOK, toUploadPayload(session))
}

func (h *UploadHandler) ListUploads(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	page := parsePositiveIntQuery(c, "page", 1)
	limit := parsePositiveIntQuery(c, "limit", 20)
	status := strings.ToUpper(strings.TrimSpace(c.Query("status")))

	items, total, pageOut, limitOut, err := h.service.ListUploadSessions(c.Request.Context(), services.ListUploadSessionsInput{
		UploaderID: userID,
		Status:     status,
		Page:       page,
		Limit:      limit,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	payloadItems := make([]httpdto.UploadSessionPayload, 0, len(items))
	for _, item := range items {
		payloadItems = append(payloadItems, toUploadPayload(item))
	}

	httpdto.WriteSuccess(c, http.StatusOK, httpdto.ListUploadSessionsPayload{
		Items:  payloadItems,
		Page:   pageOut,
		Limit:  limitOut,
		Total:  total,
		Status: status,
	})
}

func (h *UploadHandler) UpdateUploadProgress(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	sessionID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	var req httpdto.UpdateUploadProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	session, err := h.service.UpdateUploadProgress(c.Request.Context(), userID, sessionID, req.UploadedBytes)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusOK, toUploadPayload(session))
}

func (h *UploadHandler) CompleteUpload(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	sessionID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	session, err := h.service.CompleteUploadSession(c.Request.Context(), userID, sessionID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusOK, toUploadPayload(session))
}

func (h *UploadHandler) FailUpload(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	sessionID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	session, err := h.service.FailUploadSession(c.Request.Context(), userID, sessionID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusOK, toUploadPayload(session))
}

func (h *UploadHandler) CreateAttachment(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	var req httpdto.CreateAttachmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	uploadSessionID, err := uuid.Parse(strings.TrimSpace(req.UploadSessionID))
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	var messageID *uuid.UUID
	if req.MessageID != nil && strings.TrimSpace(*req.MessageID) != "" {
		parsedMessageID, parseErr := uuid.Parse(strings.TrimSpace(*req.MessageID))
		if parseErr != nil {
			h.writeError(c, sentinal_errors.ErrInvalidInput)
			return
		}
		messageID = &parsedMessageID
	}

	attachment, err := h.service.CreateAttachment(c.Request.Context(), services.CreateAttachmentInput{
		UploaderID:      userID,
		UploadSessionID: uploadSessionID,
		MessageID:       messageID,
		EncryptedURL:    req.EncryptedURL,
		Filename:        req.Filename,
		MimeType:        req.MimeType,
		SizeBytes:       req.SizeBytes,
		ViewOnce:        req.ViewOnce,
		ThumbnailURL:    req.ThumbnailURL,
		Width:           req.Width,
		Height:          req.Height,
		DurationSeconds: req.DurationSeconds,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusCreated, toAttachmentPayload(attachment))
}

func (h *UploadHandler) GetAttachment(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	attachmentID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	attachment, err := h.service.GetAttachment(c.Request.Context(), userID, attachmentID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusOK, toAttachmentPayload(attachment))
}

func (h *UploadHandler) MarkAttachmentViewed(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	attachmentID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	attachment, err := h.service.MarkAttachmentViewed(c.Request.Context(), userID, attachmentID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	viewedAt := time.Now().UTC()
	if attachment.ViewedAt.Valid {
		viewedAt = attachment.ViewedAt.Time
	}

	httpdto.WriteSuccess(c, http.StatusOK, httpdto.AttachmentViewedPayload{
		AttachmentID: attachmentID.String(),
		Viewed:       attachment.ViewedAt.Valid,
		ViewedAt:     viewedAt,
	})
}

func (h *UploadHandler) GetMessageAttachments(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	messageID, err := parseUUIDParam(c, "id")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	attachments, err := h.service.GetMessageAttachments(c.Request.Context(), userID, messageID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	items := make([]httpdto.AttachmentPayload, 0, len(attachments))
	for _, item := range attachments {
		items = append(items, toAttachmentPayload(item))
	}

	httpdto.WriteSuccess(c, http.StatusOK, httpdto.MessageAttachmentsPayload{
		MessageID:   messageID.String(),
		Attachments: items,
	})
}

func (h *UploadHandler) mustUserID(c *gin.Context) (uuid.UUID, bool) {
	value, exists := c.Get("user_id")
	if !exists {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return uuid.Nil, false
	}

	userID, ok := value.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return uuid.Nil, false
	}

	return userID, true
}

func (h *UploadHandler) writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	message := "internal server error"

	switch {
	case errors.Is(err, sentinal_errors.ErrInvalidInput):
		status = http.StatusBadRequest
		code = "INVALID_INPUT"
		message = "invalid input"
	case errors.Is(err, sentinal_errors.ErrTooLarge):
		status = http.StatusRequestEntityTooLarge
		code = "TOO_LARGE"
		message = "file too large"
	case errors.Is(err, sentinal_errors.ErrNotUploaded):
		status = http.StatusConflict
		code = "UPLOAD_NOT_COMPLETED"
		message = "upload is not completed"
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
		code = "NOT_FOUND"
		message = "resource not found"
	case errors.Is(err, sentinal_errors.ErrConflict), errors.Is(err, sentinal_errors.ErrAlreadyExists), errors.Is(err, sentinal_errors.ErrInvalidTransition):
		status = http.StatusConflict
		code = "CONFLICT"
		message = "conflict"
	case errors.Is(err, sentinal_errors.ErrServiceUnavailable):
		status = http.StatusServiceUnavailable
		code = "SERVICE_UNAVAILABLE"
		message = "service unavailable"
	}

	if status >= http.StatusInternalServerError && h.logger != nil {
		h.logger.Errorf("upload handler error: %v", err)
	}

	httpdto.WriteError(c, status, message, code)
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

func parsePositiveIntQuery(c *gin.Context, key string, fallback int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func toUploadPayload(session upload.UploadSession) httpdto.UploadSessionPayload {
	return httpdto.UploadSessionPayload{
		ID:            session.ID.String(),
		UploaderID:    session.UploaderID.String(),
		Filename:      session.Filename,
		MimeType:      session.MimeType,
		SizeBytes:     session.SizeBytes,
		ChunkSize:     session.ChunkSize,
		UploadedBytes: session.UploadedBytes,
		Status:        session.Status,
		ObjectKey:     session.ObjectKey,
		FileURL:       nullStringPtr(session.FileURL),
		CompletedAt:   nullTimePtr(session.CompletedAt),
		CreatedAt:     session.CreatedAt,
		UpdatedAt:     session.UpdatedAt,
	}
}

func toAttachmentPayload(attachment message.Attachment) httpdto.AttachmentPayload {
	return httpdto.AttachmentPayload{
		ID:              attachment.ID.String(),
		UploaderID:      nullUUIDPtr(attachment.UploaderID),
		EncryptedURL:    attachment.EncryptedURL,
		Filename:        nullStringPtr(attachment.Filename),
		MimeType:        attachment.MimeType,
		SizeBytes:       attachment.SizeBytes,
		ViewOnce:        attachment.ViewOnce,
		ViewedAt:        nullTimePtr(attachment.ViewedAt),
		ThumbnailURL:    nullStringPtr(attachment.ThumbnailURL),
		Width:           nullInt32Ptr(attachment.Width),
		Height:          nullInt32Ptr(attachment.Height),
		DurationSeconds: nullInt32Ptr(attachment.DurationSeconds),
		CreatedAt:       attachment.CreatedAt,
	}
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	copy := value.String
	return &copy
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	copy := value.Time
	return &copy
}

func nullInt32Ptr(value sql.NullInt32) *int32 {
	if !value.Valid {
		return nil
	}
	copy := value.Int32
	return &copy
}

func nullUUIDPtr(value uuid.NullUUID) *string {
	if !value.Valid {
		return nil
	}
	copy := value.UUID.String()
	return &copy
}
