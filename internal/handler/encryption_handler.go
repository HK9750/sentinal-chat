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
	resp := req
	resp.PublicKey = nil
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(resp))
}

func (h *EncryptionHandler) GetIdentityKey(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	deviceID, err := uuid.Parse(c.Query("device_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid device_id", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.GetIdentityKey(c.Request.Context(), userID, deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	item.PublicKey = nil
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
	resp := req
	resp.PublicKey = nil
	resp.Signature = nil
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(resp))
}

func (h *EncryptionHandler) GetSignedPreKey(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	deviceID, err := uuid.Parse(c.Query("device_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid device_id", "INVALID_REQUEST"))
		return
	}
	keyID, _ := strconv.Atoi(c.Query("key_id"))
	item, err := h.service.GetSignedPreKey(c.Request.Context(), userID, deviceID, keyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	item.PublicKey = nil
	item.Signature = nil
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(item))
}

func (h *EncryptionHandler) GetActiveSignedPreKey(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	deviceID, err := uuid.Parse(c.Query("device_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid device_id", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.GetActiveSignedPreKey(c.Request.Context(), userID, deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	item.PublicKey = nil
	item.Signature = nil
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
	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid device_id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.RotateSignedPreKey(c.Request.Context(), userID, deviceID, &req.Key); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	resp := req.Key
	resp.PublicKey = nil
	resp.Signature = nil
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(resp))
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
	deviceID, err := uuid.Parse(c.Query("device_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid device_id", "INVALID_REQUEST"))
		return
	}
	consumedDeviceID, err := uuid.Parse(c.Query("consumed_device_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid consumed_device_id", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.ConsumeOneTimePreKey(c.Request.Context(), userID, deviceID, consumedBy, consumedDeviceID)
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
	deviceID, err := uuid.Parse(c.Query("device_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid device_id", "INVALID_REQUEST"))
		return
	}
	count, err := h.service.GetAvailablePreKeyCount(c.Request.Context(), userID, deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"count": count}))
}

func (h *EncryptionHandler) CreateSession(c *gin.Context) {
	c.JSON(http.StatusNotFound, httpdto.NewErrorResponse("route disabled", "NOT_FOUND"))
}

func (h *EncryptionHandler) GetSession(c *gin.Context) {
	c.JSON(http.StatusNotFound, httpdto.NewErrorResponse("route disabled", "NOT_FOUND"))
}

func (h *EncryptionHandler) UpdateSession(c *gin.Context) {
	c.JSON(http.StatusNotFound, httpdto.NewErrorResponse("route disabled", "NOT_FOUND"))
}

func (h *EncryptionHandler) DeleteSession(c *gin.Context) {
	c.JSON(http.StatusNotFound, httpdto.NewErrorResponse("route disabled", "NOT_FOUND"))
}

func (h *EncryptionHandler) UpsertKeyBundle(c *gin.Context) {
	c.JSON(http.StatusNotFound, httpdto.NewErrorResponse("route disabled", "NOT_FOUND"))
}

func (h *EncryptionHandler) GetKeyBundle(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	deviceID, err := uuid.Parse(c.Query("device_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid device_id", "INVALID_REQUEST"))
		return
	}
	consumerID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	if consumerID == userID {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("cannot fetch bundle for self", "REQUEST_FAILED"))
		return
	}
	consumerDeviceID, err := uuid.Parse(c.Query("consumer_device_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid consumer_device_id", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.GetKeyBundle(c.Request.Context(), userID, deviceID, consumerID, consumerDeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(item))
}

func (h *EncryptionHandler) GetUserKeyBundles(c *gin.Context) {
	c.JSON(http.StatusNotFound, httpdto.NewErrorResponse("route disabled", "NOT_FOUND"))
}

func (h *EncryptionHandler) DeleteKeyBundle(c *gin.Context) {
	c.JSON(http.StatusNotFound, httpdto.NewErrorResponse("route disabled", "NOT_FOUND"))
}

func (h *EncryptionHandler) HasActiveKeys(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	deviceID, err := uuid.Parse(c.Query("device_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid device_id", "INVALID_REQUEST"))
		return
	}
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
