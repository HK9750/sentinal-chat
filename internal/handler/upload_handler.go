package handler

import (
	"net/http"
	"strconv"
	"time"

	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UploadHandler struct {
	s3Service *services.UploadS3Service
}

func NewUploadHandler(s3Service *services.UploadS3Service) *UploadHandler {
	return &UploadHandler{s3Service: s3Service}
}

func (h *UploadHandler) Create(c *gin.Context) {
	var req httpdto.CreateUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	uploaderID, err := uuid.Parse(req.UploaderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid uploader_id", "INVALID_REQUEST"))
		return
	}

	if h.s3Service == nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("s3 uploads not configured", "REQUEST_FAILED"))
		return
	}
	result, err := h.s3Service.CreatePresignedUpload(c.Request.Context(), services.PresignInput{
		UploaderID:  uploaderID,
		FileName:    req.FileName,
		ContentType: req.ContentType,
		FileSize:    req.FileSize,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.CreateUploadResponse{
		ID:          result.Session.ID.String(),
		FileName:    result.Session.Filename,
		FileSize:    result.Session.SizeBytes,
		ContentType: result.Session.MimeType,
		UploaderID:  result.Session.UploaderID.String(),
		Status:      result.Session.Status,
		UploadURL:   result.UploadURL,
		UploadKey:   result.UploadKey,
		Headers:     result.Headers,
		CreatedAt:   result.Session.CreatedAt.Format(time.RFC3339),
	}))
}

func (h *UploadHandler) GetByID(c *gin.Context) {
	if h.s3Service == nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("s3 uploads not configured", "REQUEST_FAILED"))
		return
	}
	uploadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid upload id", "INVALID_REQUEST"))
		return
	}
	item, err := h.s3Service.GetByID(c.Request.Context(), uploadID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromUploadSession(item)))
}

func (h *UploadHandler) Update(c *gin.Context) {
	if h.s3Service == nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("s3 uploads not configured", "REQUEST_FAILED"))
		return
	}
	var req httpdto.UpdateUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	uploadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid upload id", "INVALID_REQUEST"))
		return
	}

	item, err := h.s3Service.GetByID(c.Request.Context(), uploadID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if req.FileName != "" {
		item.Filename = req.FileName
	}
	if req.ContentType != "" {
		item.MimeType = req.ContentType
	}

	if err := h.s3Service.Update(c.Request.Context(), item); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromUploadSession(item)))
}

func (h *UploadHandler) Delete(c *gin.Context) {
	if h.s3Service == nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("s3 uploads not configured", "REQUEST_FAILED"))
		return
	}
	uploadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid upload id", "INVALID_REQUEST"))
		return
	}
	if err := h.s3Service.Delete(c.Request.Context(), uploadID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *UploadHandler) ListUser(c *gin.Context) {
	if h.s3Service == nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("s3 uploads not configured", "REQUEST_FAILED"))
		return
	}
	uploaderID, err := uuid.Parse(c.Query("uploader_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid uploader_id", "INVALID_REQUEST"))
		return
	}
	items, err := h.s3Service.GetUserUploadSessions(c.Request.Context(), uploaderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ListUploadsResponse{
		Uploads: httpdto.FromUploadSessionSlice(items),
	}))
}

func (h *UploadHandler) ListCompleted(c *gin.Context) {
	if h.s3Service == nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("s3 uploads not configured", "REQUEST_FAILED"))
		return
	}
	uploaderID, err := uuid.Parse(c.Query("uploader_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid uploader_id", "INVALID_REQUEST"))
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, total, err := h.s3Service.GetCompletedUploads(c.Request.Context(), uploaderID, page, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ListUploadsResponse{
		Uploads: httpdto.FromUploadSessionSlice(items),
		Total:   total,
	}))
}

func (h *UploadHandler) UpdateProgress(c *gin.Context) {
	if h.s3Service == nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("s3 uploads not configured", "REQUEST_FAILED"))
		return
	}
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid upload id", "INVALID_REQUEST"))
		return
	}
	var req httpdto.UpdateProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.s3Service.UpdateProgress(c.Request.Context(), sessionID, req.UploadedBytes); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *UploadHandler) MarkCompleted(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid upload id", "INVALID_REQUEST"))
		return
	}
	if h.s3Service == nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("s3 uploads not configured", "REQUEST_FAILED"))
		return
	}
	session, err := h.s3Service.MarkCompletedWithS3(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromUploadSession(session)))
}

func (h *UploadHandler) MarkFailed(c *gin.Context) {
	if h.s3Service == nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("s3 uploads not configured", "REQUEST_FAILED"))
		return
	}
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid upload id", "INVALID_REQUEST"))
		return
	}
	if err := h.s3Service.MarkFailed(c.Request.Context(), sessionID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *UploadHandler) ListInProgress(c *gin.Context) {
	if h.s3Service == nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("s3 uploads not configured", "REQUEST_FAILED"))
		return
	}
	uploaderID, err := uuid.Parse(c.Query("uploader_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid uploader_id", "INVALID_REQUEST"))
		return
	}
	items, err := h.s3Service.GetInProgressUploads(c.Request.Context(), uploaderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ListUploadsResponse{
		Uploads: httpdto.FromUploadSessionSlice(items),
	}))
}

func (h *UploadHandler) ListStale(c *gin.Context) {
	if h.s3Service == nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("s3 uploads not configured", "REQUEST_FAILED"))
		return
	}
	olderThanSec, _ := strconv.Atoi(c.Query("older_than_sec"))
	items, err := h.s3Service.GetStaleUploads(c.Request.Context(), time.Duration(olderThanSec)*time.Second)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ListUploadsResponse{
		Uploads: httpdto.FromUploadSessionSlice(items),
	}))
}

func (h *UploadHandler) DeleteStale(c *gin.Context) {
	if h.s3Service == nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("s3 uploads not configured", "REQUEST_FAILED"))
		return
	}
	olderThanSec, _ := strconv.Atoi(c.Query("older_than_sec"))
	count, err := h.s3Service.DeleteStaleUploads(c.Request.Context(), time.Duration(olderThanSec)*time.Second)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.DeleteStaleUploadsResponse{Deleted: count}))
}
