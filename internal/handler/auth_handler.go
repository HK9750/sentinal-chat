package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"sentinal-chat/config"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"
	sentinal_errors "sentinal-chat/pkg/errors"
	"sentinal-chat/pkg/logger"
)

const refreshCookieName = "refresh_token"
const accessCookieName = "access_token"

type AuthHandler struct {
	service *services.AuthService
	cfg     *config.Config
	logger  *logger.Logger
}

func NewAuthHandler(service *services.AuthService, cfg *config.Config, l *logger.Logger) *AuthHandler {
	return &AuthHandler{service: service, cfg: cfg, logger: l}
}

func (h *AuthHandler) RegisterPublicRoutes(router gin.IRouter) {
	router.POST("/auth/register", h.Register)
	router.POST("/auth/login", h.Login)
	router.POST("/auth/refresh", h.Refresh)
	router.GET("/auth/oauth/:provider/url", h.OAuthAuthorizeURL)
	router.POST("/auth/oauth/:provider/exchange", h.OAuthExchange)
}

func (h *AuthHandler) RegisterProtectedRoutes(router gin.IRouter) {
	router.POST("/auth/logout", h.Logout)
	router.POST("/auth/logout-all", h.LogoutAll)
	router.GET("/auth/sessions", h.Sessions)
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req httpdto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	result, err := h.service.Register(c.Request.Context(), services.RegisterInput{
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Username:    req.Username,
		PhoneNumber: req.PhoneNumber,
		Password:    req.Password,
		Device:      toServiceDeviceInput(req.Device),
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	h.setRefreshCookie(c, result.Tokens.RefreshToken)
	h.setAccessCookie(c, result.Tokens.AccessToken, result.Tokens.ExpiresIn)
	result.Tokens.RefreshToken = nil
	httpdto.WriteSuccess(c, http.StatusCreated, toAuthPayload(result))
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req httpdto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	result, err := h.service.Login(c.Request.Context(), services.LoginInput{
		Identifier: req.Identifier,
		Password:   req.Password,
		Device:     toServiceDeviceInput(req.Device),
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	h.setRefreshCookie(c, result.Tokens.RefreshToken)
	h.setAccessCookie(c, result.Tokens.AccessToken, result.Tokens.ExpiresIn)
	result.Tokens.RefreshToken = nil
	httpdto.WriteSuccess(c, http.StatusOK, toAuthPayload(result))
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req httpdto.RefreshRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			h.writeError(c, sentinal_errors.ErrInvalidInput)
			return
		}
	}

	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		cookieValue, err := c.Cookie(refreshCookieName)
		if err == nil {
			refreshToken = strings.TrimSpace(cookieValue)
		}
	}

	result, err := h.service.Refresh(c.Request.Context(), services.RefreshInput{RefreshToken: refreshToken})
	if err != nil {
		h.writeError(c, err)
		return
	}

	h.setRefreshCookie(c, result.Tokens.RefreshToken)
	h.setAccessCookie(c, result.Tokens.AccessToken, result.Tokens.ExpiresIn)
	result.Tokens.RefreshToken = nil
	httpdto.WriteSuccess(c, http.StatusOK, toAuthPayload(result))
}

func (h *AuthHandler) OAuthExchange(c *gin.Context) {
	provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))

	var req httpdto.OAuthExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	result, err := h.service.ExchangeOAuth(c.Request.Context(), services.OAuthExchangeInput{
		Provider:     services.AuthProvider(provider),
		Code:         req.Code,
		CodeVerifier: req.CodeVerifier,
		RedirectURI:  req.RedirectURI,
		Device:       toServiceDeviceInput(req.Device),
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	h.setRefreshCookie(c, result.Tokens.RefreshToken)
	h.setAccessCookie(c, result.Tokens.AccessToken, result.Tokens.ExpiresIn)
	result.Tokens.RefreshToken = nil
	httpdto.WriteSuccess(c, http.StatusOK, toAuthPayload(result))
}

func (h *AuthHandler) OAuthAuthorizeURL(c *gin.Context) {
	provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))

	var query httpdto.OAuthAuthorizeQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		h.writeError(c, sentinal_errors.ErrInvalidInput)
		return
	}

	result, err := h.service.AuthorizeOAuth(services.OAuthAuthorizeInput{
		Provider:      services.AuthProvider(provider),
		RedirectURI:   query.RedirectURI,
		CodeChallenge: query.CodeChallenge,
		State:         query.State,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpdto.WriteSuccess(c, http.StatusOK, httpdto.OAuthAuthorizePayload{
		Provider:         string(result.Provider),
		AuthorizationURL: result.AuthorizationURL,
		RedirectURI:      result.RedirectURI,
		State:            result.State,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	userID, sessionID, ok := h.mustAuthContext(c)
	if !ok {
		return
	}

	var req httpdto.LogoutRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			h.writeError(c, sentinal_errors.ErrInvalidInput)
			return
		}
	}

	var target *uuid.UUID
	if req.SessionID != nil {
		id, err := uuid.Parse(strings.TrimSpace(*req.SessionID))
		if err != nil {
			h.writeError(c, sentinal_errors.ErrInvalidInput)
			return
		}
		target = &id
	}

	revokedID, err := h.service.Logout(c.Request.Context(), userID, sessionID, target)
	if err != nil {
		h.writeError(c, err)
		return
	}

	if revokedID == sessionID {
		h.clearRefreshCookie(c)
		h.clearAccessCookie(c)
	}

	httpdto.WriteSuccess(c, http.StatusOK, httpdto.LogoutPayload{RevokedSessionID: revokedID.String()})
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
	userID, _, ok := h.mustAuthContext(c)
	if !ok {
		return
	}

	if err := h.service.LogoutAll(c.Request.Context(), userID); err != nil {
		h.writeError(c, err)
		return
	}

	h.clearRefreshCookie(c)
	h.clearAccessCookie(c)
	httpdto.WriteSuccess(c, http.StatusOK, httpdto.LogoutAllPayload{RevokedAll: true})
}

func (h *AuthHandler) Sessions(c *gin.Context) {
	userID, sessionID, ok := h.mustAuthContext(c)
	if !ok {
		return
	}

	items, err := h.service.ListSessions(c.Request.Context(), userID, sessionID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	payloadItems := make([]httpdto.AuthSessionPayload, 0, len(items))
	for _, item := range items {
		payloadItems = append(payloadItems, httpdto.AuthSessionPayload{
			ID:           item.ID,
			UserID:       item.UserID,
			Device:       toSessionDevicePayload(item.Device),
			CreatedAt:    item.CreatedAt,
			ExpiresAt:    item.ExpiresAt,
			AuthProvider: string(item.AuthProvider),
			IsCurrent:    item.IsCurrent,
		})
	}

	httpdto.WriteSuccess(c, http.StatusOK, httpdto.SessionsPayload{Items: payloadItems})
}

func (h *AuthHandler) mustAuthContext(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	userValue, hasUser := c.Get("user_id")
	if !hasUser {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return uuid.Nil, uuid.Nil, false
	}
	userID, ok := userValue.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return uuid.Nil, uuid.Nil, false
	}

	sessionValue, hasSession := c.Get("session_id")
	if !hasSession {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return uuid.Nil, uuid.Nil, false
	}

	sessionStr, ok := sessionValue.(string)
	if !ok {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return uuid.Nil, uuid.Nil, false
	}

	sessionID, err := uuid.Parse(strings.TrimSpace(sessionStr))
	if err != nil {
		h.writeError(c, sentinal_errors.ErrUnauthorized)
		return uuid.Nil, uuid.Nil, false
	}

	return userID, sessionID, true
}

func (h *AuthHandler) setRefreshCookie(c *gin.Context, refreshToken *string) {
	if refreshToken == nil || strings.TrimSpace(*refreshToken) == "" {
		return
	}

	maxAge := h.cfg.RefreshExpiry * 24 * 60 * 60
	if maxAge <= 0 {
		maxAge = 14 * 24 * 60 * 60
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, strings.TrimSpace(*refreshToken), maxAge, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, true)
}

func (h *AuthHandler) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, "", -1, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, true)
}

func (h *AuthHandler) setAccessCookie(c *gin.Context, accessToken string, expiresInSeconds int) {
	token := strings.TrimSpace(accessToken)
	if token == "" {
		return
	}

	maxAge := expiresInSeconds
	if maxAge <= 0 {
		maxAge = h.cfg.JWTExpiryHours * 60 * 60
	}
	if maxAge <= 0 {
		maxAge = 60 * 60
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(accessCookieName, token, maxAge, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, true)
}

func (h *AuthHandler) clearAccessCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(accessCookieName, "", -1, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, true)
}

func (h *AuthHandler) writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	message := "internal server error"

	switch {
	case errors.Is(err, sentinal_errors.ErrInvalidInput):
		status = http.StatusBadRequest
		code = "AUTH_INVALID_INPUT"
		message = "invalid input"
	case errors.Is(err, services.ErrInvalidCredentials):
		status = http.StatusUnauthorized
		code = "AUTH_INVALID_CREDENTIALS"
		message = "invalid credentials"
	case errors.Is(err, sentinal_errors.ErrServiceUnavailable):
		status = http.StatusServiceUnavailable
		code = "SERVICE_UNAVAILABLE"
		message = "service unavailable"
	case errors.Is(err, sentinal_errors.ErrUnauthorized):
		status = http.StatusUnauthorized
		code = "AUTH_UNAUTHORIZED"
		message = "unauthorized"
	case errors.Is(err, sentinal_errors.ErrForbidden):
		status = http.StatusForbidden
		code = "AUTH_FORBIDDEN"
		message = "forbidden"
	case errors.Is(err, sentinal_errors.ErrNotFound):
		status = http.StatusNotFound
		code = "AUTH_NOT_FOUND"
		message = "not found"
	case errors.Is(err, sentinal_errors.ErrAlreadyExists):
		status = http.StatusConflict
		code = "AUTH_IDENTIFIER_TAKEN"
		message = "identifier already exists"
	case errors.Is(err, sentinal_errors.ErrConflict):
		status = http.StatusConflict
		code = "AUTH_CONFLICT"
		message = "conflict"
	case errors.Is(err, services.ErrUnsupportedOAuthProvider):
		status = http.StatusBadRequest
		code = "AUTH_OAUTH_PROVIDER_UNSUPPORTED"
		message = "unsupported oauth provider"
	case errors.Is(err, services.ErrOAuthEmailUnverified):
		status = http.StatusForbidden
		code = "AUTH_OAUTH_EMAIL_UNVERIFIED"
		message = "oauth email is not verified"
	}

	if status >= http.StatusInternalServerError && h.logger != nil {
		h.logger.Errorf("auth handler error: %v", err)
	}

	if status == http.StatusUnauthorized {
		h.clearRefreshCookie(c)
		h.clearAccessCookie(c)
	}

	httpdto.WriteError(c, status, message, code)
}

func toServiceDeviceInput(in *httpdto.DeviceInput) services.DeviceInput {
	if in == nil {
		return services.DeviceInput{}
	}

	return services.DeviceInput{
		DeviceID:   in.DeviceID,
		DeviceName: in.DeviceName,
		DeviceType: in.DeviceType,
	}
}

func toAuthPayload(result services.AuthResult) httpdto.AuthPayload {
	return httpdto.AuthPayload{
		User: httpdto.AuthUserPayload{
			ID:          result.User.ID,
			DisplayName: result.User.DisplayName,
			Email:       result.User.Email,
			Username:    result.User.Username,
			PhoneNumber: result.User.PhoneNumber,
			AvatarURL:   result.User.AvatarURL,
			IsVerified:  result.User.IsVerified,
		},
		Session: httpdto.AuthSessionPayload{
			ID:           result.Session.ID,
			UserID:       result.Session.UserID,
			Device:       toSessionDevicePayload(result.Session.Device),
			CreatedAt:    result.Session.CreatedAt,
			ExpiresAt:    result.Session.ExpiresAt,
			AuthProvider: string(result.Session.AuthProvider),
			IsCurrent:    result.Session.IsCurrent,
		},
		Tokens: httpdto.AuthTokensPayload{
			AccessToken:           result.Tokens.AccessToken,
			TokenType:             result.Tokens.TokenType,
			ExpiresIn:             result.Tokens.ExpiresIn,
			ExpiresAt:             result.Tokens.ExpiresAt,
			RefreshToken:          result.Tokens.RefreshToken,
			RefreshTokenExpiresAt: result.Tokens.RefreshTokenExpiresAt,
			RefreshTokenSet:       result.Tokens.RefreshTokenSet,
		},
		AuthProvider: string(result.AuthProvider),
		IsNewUser:    result.IsNewUser,
	}
}

func toSessionDevicePayload(device services.SessionDeviceView) httpdto.SessionDevicePayload {
	return httpdto.SessionDevicePayload{
		ID:         device.ID,
		DeviceID:   device.DeviceID,
		DeviceName: device.DeviceName,
		DeviceType: device.DeviceType,
	}
}
