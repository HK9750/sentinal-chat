package handler

import (
	"encoding/base64"
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
	var req httpdto.UploadIdentityKeyRequest
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
	publicKey, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid public_key", "INVALID_REQUEST"))
		return
	}
	item := &encryption.IdentityKey{
		UserID:    userID,
		DeviceID:  deviceID,
		PublicKey: publicKey,
		IsActive:  true,
	}
	if err := h.service.CreateIdentityKey(c.Request.Context(), item); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	identity := httpdto.FromIdentityKey(*item)
	identity.PublicKey = ""
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(identity))
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
	identity := httpdto.FromIdentityKey(item)
	identity.PublicKey = ""
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(identity))
}

func (h *EncryptionHandler) UploadSignedPreKey(c *gin.Context) {
	var req httpdto.UploadSignedPreKeyRequest
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
	publicKey, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid public_key", "INVALID_REQUEST"))
		return
	}
	signature, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid signature", "INVALID_REQUEST"))
		return
	}
	item := &encryption.SignedPreKey{
		UserID:    userID,
		DeviceID:  deviceID,
		KeyID:     req.KeyID,
		PublicKey: publicKey,
		Signature: signature,
		IsActive:  true,
	}
	if err := h.service.CreateSignedPreKey(c.Request.Context(), item); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	signed := httpdto.FromSignedPreKey(*item)
	signed.PublicKey = ""
	signed.Signature = ""
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(signed))
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
	signed := httpdto.FromSignedPreKey(item)
	signed.PublicKey = ""
	signed.Signature = ""
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(signed))
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
	signed := httpdto.FromSignedPreKey(item)
	signed.PublicKey = ""
	signed.Signature = ""
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(signed))
}

func (h *EncryptionHandler) RotateSignedPreKey(c *gin.Context) {
	var req httpdto.RotateSignedPreKeyRequest
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
	publicKey, err := base64.StdEncoding.DecodeString(req.Key.PublicKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid public_key", "INVALID_REQUEST"))
		return
	}
	signature, err := base64.StdEncoding.DecodeString(req.Key.Signature)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid signature", "INVALID_REQUEST"))
		return
	}
	key := encryption.SignedPreKey{
		UserID:    userID,
		DeviceID:  deviceID,
		KeyID:     req.Key.KeyID,
		PublicKey: publicKey,
		Signature: signature,
		IsActive:  true,
	}
	if err := h.service.RotateSignedPreKey(c.Request.Context(), userID, deviceID, &key); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	signed := httpdto.FromSignedPreKey(key)
	signed.PublicKey = ""
	signed.Signature = ""
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(signed))
}

func (h *EncryptionHandler) UploadOneTimePreKeys(c *gin.Context) {
	var req httpdto.UploadOneTimePreKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	keys := make([]encryption.OneTimePreKey, 0, len(req.Keys))
	for _, k := range req.Keys {
		userID, err := uuid.Parse(k.UserID)
		if err != nil {
			c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
			return
		}
		deviceID, err := uuid.Parse(k.DeviceID)
		if err != nil {
			c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid device_id", "INVALID_REQUEST"))
			return
		}
		publicKey, err := base64.StdEncoding.DecodeString(k.PublicKey)
		if err != nil {
			c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid public_key", "INVALID_REQUEST"))
			return
		}
		keys = append(keys, encryption.OneTimePreKey{
			UserID:    userID,
			DeviceID:  deviceID,
			KeyID:     k.KeyID,
			PublicKey: publicKey,
		})
	}
	if err := h.service.UploadOneTimePreKeys(c.Request.Context(), keys); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.UploadedKeysCountResponse{Uploaded: len(req.Keys)}))
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
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromOneTimePreKey(item)))
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
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.PreKeyCountResponse{Count: int(count)}))
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
	bundle := httpdto.KeyBundleDTO{
		IdentityKey: httpdto.IdentityKeyDTO{
			UserID:    item.UserID.String(),
			DeviceID:  item.DeviceID.String(),
			PublicKey: base64.StdEncoding.EncodeToString(item.IdentityKey),
		},
		SignedPreKey: httpdto.SignedPreKeyDTO{
			UserID:    item.UserID.String(),
			DeviceID:  item.DeviceID.String(),
			KeyID:     item.SignedPreKeyID,
			PublicKey: base64.StdEncoding.EncodeToString(item.SignedPreKey),
			Signature: base64.StdEncoding.EncodeToString(item.SignedPreKeySignature),
		},
	}
	if item.OneTimePreKeyID != nil {
		bundle.OneTimePreKey = httpdto.OneTimePreKeyDTO{
			UserID:    item.UserID.String(),
			DeviceID:  item.DeviceID.String(),
			KeyID:     *item.OneTimePreKeyID,
			PublicKey: base64.StdEncoding.EncodeToString(item.OneTimePreKey),
		}
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(bundle))
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
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.HasActiveKeysResponse{HasActiveKeys: ok}))
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
