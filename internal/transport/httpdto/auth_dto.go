package httpdto

import "time"

type DeviceInput struct {
	DeviceID   string `json:"device_id" binding:"omitempty,max=255"`
	DeviceName string `json:"device_name" binding:"omitempty,max=255"`
	DeviceType string `json:"device_type" binding:"omitempty,max=64"`
}

type RegisterRequest struct {
	DisplayName string       `json:"display_name" binding:"required,max=255"`
	Email       string       `json:"email" binding:"omitempty,email,max=255"`
	Username    string       `json:"username" binding:"omitempty,max=64"`
	PhoneNumber string       `json:"phone_number" binding:"omitempty,max=32"`
	Password    string       `json:"password" binding:"required,min=8,max=128"`
	Device      *DeviceInput `json:"device,omitempty"`
}

type LoginRequest struct {
	Identifier string       `json:"identifier" binding:"required,max=255"`
	Password   string       `json:"password" binding:"required,min=8,max=128"`
	Device     *DeviceInput `json:"device,omitempty"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"omitempty,min=10,max=512"`
}

type OAuthExchangeRequest struct {
	Code         string       `json:"code" binding:"required,max=2048"`
	CodeVerifier string       `json:"code_verifier" binding:"required,max=512"`
	RedirectURI  string       `json:"redirect_uri" binding:"required,url,max=2048"`
	Device       *DeviceInput `json:"device,omitempty"`
}

type OAuthAuthorizeQuery struct {
	RedirectURI   string `form:"redirect_uri" binding:"omitempty,url,max=2048"`
	CodeChallenge string `form:"code_challenge" binding:"required,max=512"`
	State         string `form:"state" binding:"omitempty,max=512"`
}

type LogoutRequest struct {
	SessionID *string `json:"session_id" binding:"omitempty,uuid"`
}

type AuthUserPayload struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	Email       *string `json:"email,omitempty"`
	Username    *string `json:"username,omitempty"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	IsVerified  bool    `json:"is_verified"`
}

type SessionDevicePayload struct {
	ID         *string `json:"id,omitempty"`
	DeviceID   *string `json:"device_id,omitempty"`
	DeviceName *string `json:"device_name,omitempty"`
	DeviceType *string `json:"device_type,omitempty"`
}

type AuthSessionPayload struct {
	ID           string               `json:"id"`
	UserID       string               `json:"user_id"`
	Device       SessionDevicePayload `json:"device"`
	CreatedAt    time.Time            `json:"created_at"`
	ExpiresAt    time.Time            `json:"expires_at"`
	AuthProvider string               `json:"auth_provider"`
	IsCurrent    bool                 `json:"is_current"`
}

type AuthTokensPayload struct {
	AccessToken           string    `json:"access_token"`
	TokenType             string    `json:"token_type"`
	ExpiresIn             int       `json:"expires_in"`
	ExpiresAt             time.Time `json:"expires_at"`
	RefreshToken          *string   `json:"refresh_token,omitempty"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
	RefreshTokenSet       bool      `json:"refresh_token_set"`
}

type AuthPayload struct {
	User         AuthUserPayload    `json:"user"`
	Session      AuthSessionPayload `json:"session"`
	Tokens       AuthTokensPayload  `json:"tokens"`
	AuthProvider string             `json:"auth_provider"`
	IsNewUser    bool               `json:"is_new_user,omitempty"`
}

type LogoutPayload struct {
	RevokedSessionID string `json:"revoked_session_id"`
}

type LogoutAllPayload struct {
	RevokedAll bool `json:"revoked_all"`
}

type SessionsPayload struct {
	Items []AuthSessionPayload `json:"items"`
}

type OAuthAuthorizePayload struct {
	Provider         string `json:"provider"`
	AuthorizationURL string `json:"authorization_url"`
	RedirectURI      string `json:"redirect_uri"`
	State            string `json:"state"`
}
