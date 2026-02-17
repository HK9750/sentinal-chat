// Package handler provides HTTP handlers for API endpoints.
package handler

import (
	"net/http"
	"time"

	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication HTTP endpoints.
type AuthHandler struct {
	service *services.AuthService
}

// NewAuthHandler creates an auth handler.
func NewAuthHandler(service *services.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

// Register handles user registration.
func (h *AuthHandler) Register(c *gin.Context) {
	var req httpdto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}

	res, err := h.service.Register(c.Request.Context(), services.RegisterInput{
		Email:       req.Email,
		Username:    req.Username,
		PhoneNumber: req.PhoneNumber,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		DeviceID:    req.DeviceID,
		DeviceName:  req.DeviceName,
		DeviceType:  req.DeviceType,
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.AuthResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresIn:    res.ExpiresIn,
		SessionID:    res.SessionID,
		User: httpdto.AuthUserDTO{
			ID:          res.User.ID,
			DisplayName: res.User.DisplayName,
			Username:    res.User.Username,
			Email:       res.User.Email,
			PhoneNumber: res.User.PhoneNumber,
		},
	}))
}

// Login handles user authentication.
func (h *AuthHandler) Login(c *gin.Context) {
	var req httpdto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}

	res, err := h.service.Login(c.Request.Context(), services.LoginInput{
		Identity:   req.Identity,
		Password:   req.Password,
		DeviceID:   req.DeviceID,
		DeviceName: req.DeviceName,
		DeviceType: req.DeviceType,
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.AuthResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresIn:    res.ExpiresIn,
		SessionID:    res.SessionID,
		User: httpdto.AuthUserDTO{
			ID:          res.User.ID,
			DisplayName: res.User.DisplayName,
			Username:    res.User.Username,
			Email:       res.User.Email,
			PhoneNumber: res.User.PhoneNumber,
		},
	}))
}

// Refresh handles token refresh.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req httpdto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}

	res, err := h.service.Refresh(c.Request.Context(), services.RefreshInput{
		SessionID:    req.SessionID,
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.AuthResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresIn:    res.ExpiresIn,
		SessionID:    res.SessionID,
		User: httpdto.AuthUserDTO{
			ID:          res.User.ID,
			DisplayName: res.User.DisplayName,
			Username:    res.User.Username,
			Email:       res.User.Email,
			PhoneNumber: res.User.PhoneNumber,
		},
	}))
}

// Logout handles user logout.
func (h *AuthHandler) Logout(c *gin.Context) {
	var req httpdto.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}

	if err := h.service.Logout(c.Request.Context(), req.SessionID); err != nil {
		writeAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

// LogoutAll handles logout from all sessions.
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	if err := h.service.LogoutAll(c.Request.Context(), userID); err != nil {
		writeAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

// Sessions lists all active user sessions.
func (h *AuthHandler) Sessions(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	sessions, err := h.service.Sessions(c.Request.Context(), userID)
	if err != nil {
		writeAuthError(c, err)
		return
	}

	// Convert service SessionInfo to DTO SessionDTO
	sessionDTOs := make([]httpdto.SessionDTO, len(sessions))
	for i, s := range sessions {
		sessionDTOs[i] = httpdto.SessionDTO{
			ID:         s.ID,
			DeviceID:   s.DeviceID,
			DeviceName: s.DeviceName,
			DeviceType: s.DeviceType,
			CreatedAt:  s.CreatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.SessionsResponse{Sessions: sessionDTOs}))
}

func (h *AuthHandler) PasswordForgot(c *gin.Context) {
	var req httpdto.PasswordForgotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}

	if err := h.service.PasswordForgot(c.Request.Context(), req.Identity); err != nil {
		writeAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *AuthHandler) PasswordReset(c *gin.Context) {
	var req httpdto.PasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}

	if err := h.service.PasswordReset(c.Request.Context(), services.ResetInput{
		Identity:    req.Identity,
		NewPassword: req.NewPassword,
	}); err != nil {
		writeAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func writeAuthError(c *gin.Context, err error) {
	status := services.HTTPStatus(err)
	c.JSON(status, httpdto.NewErrorResponse(err.Error(), errorCode(status)))
}

func errorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "INVALID_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	default:
		return "INTERNAL_ERROR"
	}
}
