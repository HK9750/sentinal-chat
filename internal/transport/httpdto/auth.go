package httpdto

import (
	"sentinal-chat/internal/domain/user"
	"time"
)

// RegisterRequest is used for POST /auth/register
type RegisterRequest struct {
	Email       string `json:"email" binding:"required"`
	Username    string `json:"username" binding:"required"`
	PhoneNumber string `json:"phone_number,omitempty"`
	Password    string `json:"password" binding:"required"`
	DisplayName string `json:"display_name,omitempty"`
	DeviceID    string `json:"device_id" binding:"required"`
	DeviceName  string `json:"device_name,omitempty"`
	DeviceType  string `json:"device_type,omitempty"`
}

// RegisterResponse is returned after successful registration
type RegisterResponse struct {
	UserID       string `json:"user_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	SessionID    string `json:"session_id"`
	ExpiresAt    string `json:"expires_at"`
}

// LoginRequest is used for POST /auth/login
type LoginRequest struct {
	Identity   string `json:"identity" binding:"required"` // email, username, or phone
	Password   string `json:"password" binding:"required"`
	DeviceID   string `json:"device_id" binding:"required"`
	DeviceName string `json:"device_name,omitempty"`
	DeviceType string `json:"device_type,omitempty"`
}

// LoginResponse is returned after successful login
type LoginResponse struct {
	UserID       string `json:"user_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	SessionID    string `json:"session_id"`
	ExpiresAt    string `json:"expires_at"`
}

// AuthResponse represents token-based auth responses.
type AuthResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token,omitempty"`
	ExpiresIn    int64       `json:"expires_in"`
	SessionID    string      `json:"session_id"`
	User         AuthUserDTO `json:"user"`
}

// AuthUserDTO represents the authenticated user in auth responses.
type AuthUserDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username,omitempty"`
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
}

// RefreshRequest is used for POST /auth/refresh
type RefreshRequest struct {
	SessionID    string `json:"session_id" binding:"required"`
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshResponse is returned after successful token refresh
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
}

// LogoutRequest is used for POST /auth/logout
type LogoutRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

// PasswordForgotRequest is used for POST /auth/password/forgot
type PasswordForgotRequest struct {
	Identity string `json:"identity" binding:"required"`
}

// PasswordResetRequest is used for POST /auth/password/reset
type PasswordResetRequest struct {
	Identity    string `json:"identity" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// SessionsResponse is returned when listing sessions
type SessionsResponse struct {
	Sessions []SessionDTO `json:"sessions"`
}

// SessionDTO represents a user session in API responses
type SessionDTO struct {
	ID         string `json:"id"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name,omitempty"`
	DeviceType string `json:"device_type,omitempty"`
	IPAddress  string `json:"ip_address,omitempty"`
	LastActive string `json:"last_active"`
	CreatedAt  string `json:"created_at"`
	IsCurrent  bool   `json:"is_current,omitempty"`
}

// FromUserSession converts a domain user session to SessionDTO
func FromUserSession(s user.UserSession) SessionDTO {
	dto := SessionDTO{
		ID:        s.ID.String(),
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
	}
	if s.DeviceID.Valid {
		dto.DeviceID = s.DeviceID.UUID.String()
	}
	return dto
}

// FromUserSessionSlice converts a slice of domain user sessions to SessionDTO slice
func FromUserSessionSlice(sessions []user.UserSession) []SessionDTO {
	dtos := make([]SessionDTO, len(sessions))
	for i, s := range sessions {
		dtos[i] = FromUserSession(s)
	}
	return dtos
}
