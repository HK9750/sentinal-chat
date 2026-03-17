package handler

import (
	"database/sql"
	"errors"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"sentinal-chat/internal/domain/message"
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
	router.POST("/uploads", h.UploadFile)
	router.POST("/uploads/bulk", h.UploadFiles)

	router.POST("/attachments", h.CreateAttachment)
	router.GET("/attachments/:id", h.GetAttachment)
	router.POST("/attachments/:id/viewed", h.MarkAttachmentViewed)
	router.GET("/messages/:id/attachments", h.GetMessageAttachments)
}

func (h *UploadHandler) UploadFile(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	result, err := h.uploadMultipartFile(c, userID, fileHeader)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusCreated, toUploadedFilePayload(result))
}

func (h *UploadHandler) UploadFiles(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		if single, singleErr := c.FormFile("file"); singleErr == nil {
			files = []*multipart.FileHeader{single}
		}
	}
	if len(files) == 0 {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}
	if len(files) > 20 {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	inputs := make([]services.UploadFileInput, 0, len(files))
	closers := make([]multipart.File, 0, len(files))
	defer func() {
		for _, closer := range closers {
			_ = closer.Close()
		}
	}()

	for _, fileHeader := range files {
		file, openErr := fileHeader.Open()
		if openErr != nil {
			h.writeError(c, openErr)
			return
		}
		closers = append(closers, file)

		mimeType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
		if mimeType == "" {
			mimeType = mimeFromFilename(fileHeader.Filename)
		}

		inputs = append(inputs, services.UploadFileInput{
			Filename:  fileHeader.Filename,
			MimeType:  mimeType,
			SizeBytes: fileHeader.Size,
			Body:      file,
		})
	}

	items, err := h.service.UploadFiles(c.Request.Context(), userID, inputs)
	if err != nil {
		h.writeError(c, err)
		return
	}

	payload := make([]httpdto.UploadFilePayload, 0, len(items))
	for _, item := range items {
		payload = append(payload, toUploadedFilePayload(item))
	}

	httpdto.WriteSuccess(c, http.StatusCreated, httpdto.UploadFilesPayload{
		Items: payload,
	})
}

func (h *UploadHandler) uploadMultipartFile(c *gin.Context, userID uuid.UUID, fileHeader *multipart.FileHeader) (services.UploadedFile, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return services.UploadedFile{}, err
	}
	defer file.Close()

	mimeType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = mimeFromFilename(fileHeader.Filename)
	}

	return h.service.UploadFile(c.Request.Context(), services.UploadFileInput{
		UploaderID: userID,
		Filename:   fileHeader.Filename,
		MimeType:   mimeType,
		SizeBytes:  fileHeader.Size,
		Body:       file,
	})
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
		MessageID:       messageID,
		FileURL:         req.FileURL,
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

func toUploadedFilePayload(item services.UploadedFile) httpdto.UploadFilePayload {
	return httpdto.UploadFilePayload{
		Filename:  item.Filename,
		MimeType:  item.MimeType,
		SizeBytes: item.SizeBytes,
		ObjectKey: item.ObjectKey,
		FileURL:   item.FileURL,
	}
}

func toAttachmentPayload(attachment message.Attachment) httpdto.AttachmentPayload {
	return httpdto.AttachmentPayload{
		ID:              attachment.ID.String(),
		UploaderID:      nullUUIDPtr(attachment.UploaderID),
		FileURL:         attachment.FileURL,
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

func mimeFromFilename(filename string) string {
	mimeType := strings.TrimSpace(mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))))
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}
