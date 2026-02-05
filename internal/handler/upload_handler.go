package handler

import (
	"net/http"
	"strconv"
	"time"

	"sentinal-chat/internal/domain/upload"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UploadHandler struct {
	service *services.UploadService
}

func NewUploadHandler(service *services.UploadService) *UploadHandler {
	return &UploadHandler{service: service}
}

func (h *UploadHandler) Create(c *gin.Context) {
	var req upload.UploadSession
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.Create(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *UploadHandler) GetByID(c *gin.Context) {
	uploadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid upload id", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.GetByID(c.Request.Context(), uploadID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(item))
}

func (h *UploadHandler) Update(c *gin.Context) {
	var req upload.UploadSession
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.Update(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *UploadHandler) Delete(c *gin.Context) {
	uploadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid upload id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.Delete(c.Request.Context(), uploadID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *UploadHandler) ListUser(c *gin.Context) {
	uploaderID, err := uuid.Parse(c.Query("uploader_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid uploader_id", "INVALID_REQUEST"))
		return
	}
	items, err := h.service.GetUserUploadSessions(c.Request.Context(), uploaderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"uploads": items}))
}

func (h *UploadHandler) ListCompleted(c *gin.Context) {
	uploaderID, err := uuid.Parse(c.Query("uploader_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid uploader_id", "INVALID_REQUEST"))
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, total, err := h.service.GetCompletedUploads(c.Request.Context(), uploaderID, page, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"uploads": items, "total": total}))
}

func (h *UploadHandler) UpdateProgress(c *gin.Context) {
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
	if err := h.service.UpdateProgress(c.Request.Context(), sessionID, req.UploadedBytes); err != nil {
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
	if err := h.service.MarkCompleted(c.Request.Context(), sessionID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *UploadHandler) MarkFailed(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid upload id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.MarkFailed(c.Request.Context(), sessionID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *UploadHandler) ListInProgress(c *gin.Context) {
	uploaderID, err := uuid.Parse(c.Query("uploader_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid uploader_id", "INVALID_REQUEST"))
		return
	}
	items, err := h.service.GetInProgressUploads(c.Request.Context(), uploaderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"uploads": items}))
}

func (h *UploadHandler) ListStale(c *gin.Context) {
	olderThanSec, _ := strconv.Atoi(c.Query("older_than_sec"))
	items, err := h.service.GetStaleUploads(c.Request.Context(), time.Duration(olderThanSec)*time.Second)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"uploads": items}))
}

func (h *UploadHandler) DeleteStale(c *gin.Context) {
	olderThanSec, _ := strconv.Atoi(c.Query("older_than_sec"))
	count, err := h.service.DeleteStaleUploads(c.Request.Context(), time.Duration(olderThanSec)*time.Second)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"deleted": count}))
}
