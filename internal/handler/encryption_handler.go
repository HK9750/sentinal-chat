package handler

import (
	"net/http"
	"strconv"

	"sentinal-chat/internal/domain/encryption"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type EncryptionHandler struct {
	service *services.EncryptionService
}

func NewEncryptionHandler(service *services.EncryptionService) *EncryptionHandler {
	return &EncryptionHandler{service: service}
}

func (h *EncryptionHandler) UploadIdentityKey(c *gin.Context) {
	var req encryption.IdentityKey
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.CreateIdentityKey(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *EncryptionHandler) GetIdentityKey(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	deviceID := c.Query("device_id")
	item, err := h.service.GetIdentityKey(c.Request.Context(), userID, deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(item))
}

func (h *EncryptionHandler) UploadSignedPreKey(c *gin.Context) {
	var req encryption.SignedPreKey
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.CreateSignedPreKey(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *EncryptionHandler) GetSignedPreKey(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	deviceID := c.Query("device_id")
	keyID, _ := strconv.Atoi(c.Query("key_id"))
	item, err := h.service.GetSignedPreKey(c.Request.Context(), userID, deviceID, keyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(item))
}

func (h *EncryptionHandler) GetActiveSignedPreKey(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	deviceID := c.Query("device_id")
	item, err := h.service.GetActiveSignedPreKey(c.Request.Context(), userID, deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(item))
}

func (h *EncryptionHandler) RotateSignedPreKey(c *gin.Context) {
	var req struct {
		UserID   string                  `json:"user_id"`
		DeviceID string                  `json:"device_id"`
		Key      encryption.SignedPreKey `json:"key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.RotateSignedPreKey(c.Request.Context(), userID, req.DeviceID, &req.Key); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req.Key))
}

func (h *EncryptionHandler) UploadOneTimePreKeys(c *gin.Context) {
	var req struct {
		Keys []encryption.OneTimePreKey `json:"keys"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.UploadOneTimePreKeys(c.Request.Context(), req.Keys); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"uploaded": len(req.Keys)}))
}

func (h *EncryptionHandler) ConsumeOneTimePreKey(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	consumedBy, err := uuid.Parse(c.Query("consumed_by"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid consumed_by", "INVALID_REQUEST"))
		return
	}
	deviceID := c.Query("device_id")
	item, err := h.service.ConsumeOneTimePreKey(c.Request.Context(), userID, deviceID, consumedBy)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(item))
}

func (h *EncryptionHandler) GetPreKeyCount(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	deviceID := c.Query("device_id")
	count, err := h.service.GetAvailablePreKeyCount(c.Request.Context(), userID, deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"count": count}))
}

func (h *EncryptionHandler) CreateSession(c *gin.Context) {
	var req encryption.EncryptedSession
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.CreateSession(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *EncryptionHandler) GetSession(c *gin.Context) {
	localUserID, err := uuid.Parse(c.Query("local_user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid local_user_id", "INVALID_REQUEST"))
		return
	}
	remoteUserID, err := uuid.Parse(c.Query("remote_user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid remote_user_id", "INVALID_REQUEST"))
		return
	}
	localDeviceID := c.Query("local_device_id")
	remoteDeviceID := c.Query("remote_device_id")
	item, err := h.service.GetSession(c.Request.Context(), localUserID, localDeviceID, remoteUserID, remoteDeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(item))
}

func (h *EncryptionHandler) UpdateSession(c *gin.Context) {
	var req encryption.EncryptedSession
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.UpdateSession(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *EncryptionHandler) DeleteSession(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid session id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.DeleteSession(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *EncryptionHandler) UpsertKeyBundle(c *gin.Context) {
	var req encryption.KeyBundle
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.UpsertKeyBundle(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *EncryptionHandler) GetKeyBundle(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	deviceID := c.Query("device_id")
	item, err := h.service.GetKeyBundle(c.Request.Context(), userID, deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(item))
}

func (h *EncryptionHandler) GetUserKeyBundles(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	items, err := h.service.GetUserKeyBundles(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"bundles": items}))
}

func (h *EncryptionHandler) DeleteKeyBundle(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	deviceID := c.Query("device_id")
	if err := h.service.DeleteKeyBundle(c.Request.Context(), userID, deviceID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *EncryptionHandler) HasActiveKeys(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	deviceID := c.Query("device_id")
	ok, err := h.service.HasActiveKeys(c.Request.Context(), userID, deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"has_active_keys": ok}))
}

func (h *EncryptionHandler) DeactivateIdentityKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid key id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.DeactivateIdentityKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *EncryptionHandler) DeleteIdentityKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid key id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.DeleteIdentityKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *EncryptionHandler) DeactivateSignedPreKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid key id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.DeactivateSignedPreKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}
