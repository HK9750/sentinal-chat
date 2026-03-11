package services

import (
	"context"
	"time"
)

type AuthProvider string

const (
	AuthProviderPassword AuthProvider = "password"
	AuthProviderGoogle   AuthProvider = "google"
	AuthProviderGitHub   AuthProvider = "github"
)

type DeviceInput struct {
	DeviceID   string
	DeviceName string
	DeviceType string
}

type RegisterInput struct {
	DisplayName string
	Email       string
	Username    string
	PhoneNumber string
	Password    string
	Device      DeviceInput
}

type LoginInput struct {
	Identifier string
	Password   string
	Device     DeviceInput
}

type RefreshInput struct {
	RefreshToken string
}

type OAuthExchangeInput struct {
	Provider     AuthProvider
	Code         string
	CodeVerifier string
	RedirectURI  string
	Device       DeviceInput
}

type OAuthAuthorizeInput struct {
	Provider      AuthProvider
	RedirectURI   string
	CodeChallenge string
	State         string
}

type OAuthAuthorizeResult struct {
	Provider         AuthProvider
	AuthorizationURL string
	RedirectURI      string
	State            string
}

type OAuthIdentity struct {
	Provider       AuthProvider
	ProviderUserID string
	Email          string
	EmailVerified  bool
	DisplayName    string
	AvatarURL      string
}

type AuthUserView struct {
	ID          string
	DisplayName string
	Email       *string
	Username    *string
	PhoneNumber *string
	AvatarURL   *string
	IsVerified  bool
}

type SessionDeviceView struct {
	ID         *string
	DeviceID   *string
	DeviceName *string
	DeviceType *string
}

type AuthSessionView struct {
	ID           string
	UserID       string
	Device       SessionDeviceView
	CreatedAt    time.Time
	ExpiresAt    time.Time
	AuthProvider AuthProvider
	IsCurrent    bool
}

type AuthTokensView struct {
	AccessToken           string
	TokenType             string
	ExpiresIn             int
	ExpiresAt             time.Time
	RefreshToken          *string
	RefreshTokenExpiresAt time.Time
	RefreshTokenSet       bool
}

type AuthResult struct {
	User         AuthUserView
	Session      AuthSessionView
	Tokens       AuthTokensView
	AuthProvider AuthProvider
	IsNewUser    bool
}

type SessionItem struct {
	ID           string
	UserID       string
	Device       SessionDeviceView
	CreatedAt    time.Time
	ExpiresAt    time.Time
	AuthProvider AuthProvider
	IsCurrent    bool
}

type OAuthProviderClient interface {
	AuthorizationURL(input OAuthAuthorizeInput) (OAuthAuthorizeResult, error)
	ExchangeCode(ctx context.Context, input OAuthExchangeInput) (OAuthIdentity, error)
}
