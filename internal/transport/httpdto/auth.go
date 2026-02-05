package httpdto

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
