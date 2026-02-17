package handler

import (
	"net/http"
	"strconv"

	"sentinal-chat/internal/domain/user"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserHandler struct {
	service *services.UserService
}

func NewUserHandler(service *services.UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	search := c.Query("search")

	items, total, err := h.service.List(c.Request.Context(), page, limit, search)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ListUsersResponse{
		Users: httpdto.FromUserSlice(items),
		Total: total,
	}))
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	userInfo, err := h.service.GetByID(c.Request.Context(), userID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromUser(userInfo)))
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	var req httpdto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}

	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	// Get current user data
	currentUser, err := h.service.GetByID(c.Request.Context(), userID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	// Update fields from request
	if req.DisplayName != "" {
		currentUser.DisplayName = req.DisplayName
	}
	if req.AvatarURL != "" {
		currentUser.AvatarURL = req.AvatarURL
	}
	if req.Bio != "" {
		currentUser.Bio = req.Bio
	}

	updated, err := h.service.UpdateProfile(c.Request.Context(), userID, currentUser)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromUser(updated)))
}

func (h *UserHandler) DeleteProfile(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	if err := h.service.Delete(c.Request.Context(), userID, userID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *UserHandler) GetSettings(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	settings, err := h.service.GetSettings(c.Request.Context(), userID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromUserSettings(settings)))
}

func (h *UserHandler) UpdateSettings(c *gin.Context) {
	var req httpdto.UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}

	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	// Get current settings
	currentSettings, err := h.service.GetSettings(c.Request.Context(), userID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	// Update fields from request
	currentSettings.NotificationsEnabled = req.NotificationsEnabled
	if req.Theme != "" {
		currentSettings.Theme = req.Theme
	}
	if req.Language != "" {
		currentSettings.Language = req.Language
	}

	updated, err := h.service.UpdateSettings(c.Request.Context(), userID, currentSettings)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromUserSettings(updated)))
}

func (h *UserHandler) ListContacts(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	items, err := h.service.GetContacts(c.Request.Context(), userID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ContactsResponse{
		Contacts: httpdto.FromUserContactSlice(items),
	}))
}

func (h *UserHandler) AddContact(c *gin.Context) {
	var req httpdto.AddContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}

	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	contactID, err := uuid.Parse(req.ContactUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid contact_user_id", "INVALID_REQUEST"))
		return
	}

	contact := user.UserContact{UserID: userID, ContactUserID: contactID}
	if err := h.service.AddContact(c.Request.Context(), userID, contact); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.AddContactResponse{Success: true}))
}

func (h *UserHandler) RemoveContact(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	contactID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid contact id", "INVALID_REQUEST"))
		return
	}

	if err := h.service.RemoveContact(c.Request.Context(), userID, userID, contactID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *UserHandler) BlockContact(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	contactID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid contact id", "INVALID_REQUEST"))
		return
	}

	if err := h.service.BlockContact(c.Request.Context(), userID, userID, contactID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *UserHandler) UnblockContact(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	contactID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid contact id", "INVALID_REQUEST"))
		return
	}

	if err := h.service.UnblockContact(c.Request.Context(), userID, userID, contactID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *UserHandler) BlockedContacts(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	items, err := h.service.GetBlockedContacts(c.Request.Context(), userID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ContactsResponse{
		Contacts: httpdto.FromUserContactSlice(items),
	}))
}

func (h *UserHandler) GetDevice(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	deviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid device id", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.GetDeviceByID(c.Request.Context(), userID, userID, deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromDevice(item)))
}

func (h *UserHandler) ListDevices(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	items, err := h.service.GetDevices(c.Request.Context(), userID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.DevicesResponse{
		Devices: httpdto.FromDeviceSlice(items),
	}))
}

func (h *UserHandler) DeactivateDevice(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	deviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid device id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.DeactivateDevice(c.Request.Context(), userID, userID, deviceID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *UserHandler) ListPushTokens(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	items, err := h.service.GetPushTokens(c.Request.Context(), userID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.PushTokensResponse{
		Tokens: httpdto.FromPushTokenSlice(items),
	}))
}

func (h *UserHandler) RevokeSession(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid session id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.RevokeSession(c.Request.Context(), userID, userID, sessionID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *UserHandler) RevokeAllSessions(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	if err := h.service.RevokeAllSessions(c.Request.Context(), userID, userID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}
