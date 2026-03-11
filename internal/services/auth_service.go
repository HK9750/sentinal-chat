package services

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"sentinal-chat/internal/domain/user"
	"sentinal-chat/internal/middleware"
	"sentinal-chat/internal/repository"
	"sentinal-chat/pkg/database"
	sentinal_errors "sentinal-chat/pkg/errors"
)

var (
	ErrInvalidCredentials       = errors.New("invalid credentials")
	ErrUnsupportedOAuthProvider = errors.New("unsupported oauth provider")
	ErrOAuthEmailUnverified     = errors.New("oauth email is not verified")
)

type AuthService struct {
	users   repository.UserRepository
	oauth   repository.OAuthIdentityRepository
	tokens  *TokenService
	clients map[AuthProvider]OAuthProviderClient
}

func NewAuthService(
	users repository.UserRepository,
	oauth repository.OAuthIdentityRepository,
	tokens *TokenService,
	clients map[AuthProvider]OAuthProviderClient,
) (*AuthService, error) {
	if users == nil || oauth == nil || tokens == nil {
		return nil, sentinal_errors.ErrServiceUnavailable
	}

	if clients == nil {
		clients = map[AuthProvider]OAuthProviderClient{}
	}

	return &AuthService{
		users:   users,
		oauth:   oauth,
		tokens:  tokens,
		clients: clients,
	}, nil
}

func (s *AuthService) ParseAccessToken(token string) (*middleware.TokenClaims, error) {
	return s.tokens.ParseAccessToken(token)
}

func (s *AuthService) ValidateAccessSession(ctx context.Context, claims *middleware.TokenClaims) error {
	if claims == nil {
		return sentinal_errors.ErrUnauthorized
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return sentinal_errors.ErrUnauthorized
	}

	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return sentinal_errors.ErrUnauthorized
	}

	session, err := s.users.GetSessionByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sentinal_errors.ErrNotFound) {
			return sentinal_errors.ErrUnauthorized
		}
		return err
	}
	if session.UserID != userID {
		return sentinal_errors.ErrUnauthorized
	}

	if strings.TrimSpace(claims.DeviceID) == "" {
		return nil
	}
	if session.DeviceID == nil {
		return sentinal_errors.ErrUnauthorized
	}

	device, err := s.users.GetDeviceByID(ctx, *session.DeviceID)
	if err != nil {
		if errors.Is(err, sentinal_errors.ErrNotFound) {
			return sentinal_errors.ErrUnauthorized
		}
		return err
	}

	if !device.IsActive || device.UserID != userID || device.DeviceID != claims.DeviceID {
		return sentinal_errors.ErrUnauthorized
	}

	if err := s.users.UpdateDeviceLastSeen(ctx, device.ID); err != nil && !errors.Is(err, sentinal_errors.ErrNotFound) {
		return err
	}

	return nil
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	now := time.Now().UTC()
	displayName := strings.TrimSpace(input.DisplayName)
	password := strings.TrimSpace(input.Password)
	email := normalizeEmail(input.Email)
	username := normalizeUsername(input.Username)
	phone := normalizePhone(input.PhoneNumber)

	if displayName == "" || len(password) < 8 {
		return AuthResult{}, sentinal_errors.ErrInvalidInput
	}
	if invalidDeviceInput(input.Device) {
		return AuthResult{}, sentinal_errors.ErrInvalidInput
	}
	if email == "" && username == "" && phone == "" {
		return AuthResult{}, sentinal_errors.ErrInvalidInput
	}

	passwordHash, err := database.HashPassword(password)
	if err != nil {
		return AuthResult{}, err
	}

	u := user.User{
		ID:           uuid.New(),
		DisplayName:  displayName,
		Email:        toNullString(email),
		Username:     toNullString(username),
		PhoneNumber:  toNullString(phone),
		PasswordHash: passwordHash,
		IsActive:     true,
		IsVerified:   false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, &u); err != nil {
		if errors.Is(err, sentinal_errors.ErrAlreadyExists) {
			return AuthResult{}, sentinal_errors.ErrAlreadyExists
		}
		return AuthResult{}, err
	}

	return s.issueAuthForUser(ctx, u, input.Device, AuthProviderPassword, false)
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	identifier := strings.TrimSpace(input.Identifier)
	password := strings.TrimSpace(input.Password)
	if identifier == "" || password == "" {
		return AuthResult{}, sentinal_errors.ErrInvalidInput
	}
	if invalidDeviceInput(input.Device) {
		return AuthResult{}, sentinal_errors.ErrInvalidInput
	}

	u, err := s.findUserByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, sentinal_errors.ErrNotFound) {
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, err
	}

	if !u.IsActive || !database.CheckPasswordHash(password, u.PasswordHash) {
		return AuthResult{}, ErrInvalidCredentials
	}

	if err := s.users.UpdateOnlineStatus(ctx, u.ID, true); err != nil && !errors.Is(err, sentinal_errors.ErrNotFound) {
		return AuthResult{}, err
	}

	return s.issueAuthForUser(ctx, u, input.Device, AuthProviderPassword, false)
}

func (s *AuthService) AuthorizeOAuth(input OAuthAuthorizeInput) (OAuthAuthorizeResult, error) {
	provider := normalizeProvider(input.Provider)
	if provider == "" || provider == AuthProviderPassword {
		return OAuthAuthorizeResult{}, ErrUnsupportedOAuthProvider
	}
	if strings.TrimSpace(input.CodeChallenge) == "" {
		return OAuthAuthorizeResult{}, sentinal_errors.ErrInvalidInput
	}

	client, err := s.oauthClient(provider)
	if err != nil {
		return OAuthAuthorizeResult{}, err
	}

	input.Provider = provider
	input.State = strings.TrimSpace(input.State)
	if input.State == "" {
		generatedState, tokenErr := database.GenerateSecureToken(16)
		if tokenErr != nil {
			return OAuthAuthorizeResult{}, tokenErr
		}
		input.State = generatedState
	}

	result, err := client.AuthorizationURL(input)
	if err != nil {
		return OAuthAuthorizeResult{}, err
	}
	if strings.TrimSpace(result.State) == "" {
		result.State = input.State
	}
	if strings.TrimSpace(result.RedirectURI) == "" {
		result.RedirectURI = strings.TrimSpace(input.RedirectURI)
	}
	result.Provider = provider

	return result, nil
}

func (s *AuthService) Refresh(ctx context.Context, input RefreshInput) (AuthResult, error) {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken == "" {
		return AuthResult{}, sentinal_errors.ErrInvalidInput
	}

	hash := HashRefreshToken(refreshToken)
	session, err := s.users.GetSessionByRefreshTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sentinal_errors.ErrNotFound) {
			return AuthResult{}, sentinal_errors.ErrUnauthorized
		}
		return AuthResult{}, err
	}

	u, err := s.users.GetUserByID(ctx, session.UserID)
	if err != nil {
		return AuthResult{}, err
	}
	if !u.IsActive {
		return AuthResult{}, sentinal_errors.ErrUnauthorized
	}

	device, deviceExternalID, err := s.resolveSessionDevice(ctx, session)
	if err != nil {
		return AuthResult{}, err
	}

	pair, err := s.tokens.GenerateTokenPair(u.ID, session.ID, deviceExternalID)
	if err != nil {
		return AuthResult{}, err
	}

	session.RefreshTokenHash = pair.RefreshTokenHash
	session.ExpiresAt = pair.RefreshTokenExpiresAt
	if err := s.users.UpdateSession(ctx, session); err != nil {
		return AuthResult{}, err
	}

	provider := normalizeSessionProvider(session.AuthProvider)
	return buildAuthResult(u, session, device, pair, provider, false), nil
}

func (s *AuthService) ExchangeOAuth(ctx context.Context, input OAuthExchangeInput) (AuthResult, error) {
	provider := normalizeProvider(input.Provider)
	if provider == "" || provider == AuthProviderPassword {
		return AuthResult{}, ErrUnsupportedOAuthProvider
	}
	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.CodeVerifier) == "" || strings.TrimSpace(input.RedirectURI) == "" {
		return AuthResult{}, sentinal_errors.ErrInvalidInput
	}
	if invalidDeviceInput(input.Device) {
		return AuthResult{}, sentinal_errors.ErrInvalidInput
	}

	client, err := s.oauthClient(provider)
	if err != nil {
		return AuthResult{}, err
	}

	identity, err := client.ExchangeCode(ctx, input)
	if err != nil {
		return AuthResult{}, err
	}

	identity.Provider = provider
	u, isNewUser, err := s.resolveOAuthUser(ctx, identity)
	if err != nil {
		return AuthResult{}, err
	}

	return s.issueAuthForUser(ctx, u, input.Device, provider, isNewUser)
}

func (s *AuthService) Logout(ctx context.Context, userID, currentSessionID uuid.UUID, targetSessionID *uuid.UUID) (uuid.UUID, error) {
	revokeID := currentSessionID
	if targetSessionID != nil {
		targetSession, err := s.users.GetSessionByID(ctx, *targetSessionID)
		if err != nil {
			if errors.Is(err, sentinal_errors.ErrNotFound) {
				return uuid.Nil, sentinal_errors.ErrNotFound
			}
			return uuid.Nil, err
		}
		if targetSession.UserID != userID {
			return uuid.Nil, sentinal_errors.ErrForbidden
		}
		revokeID = targetSession.ID
	}

	if err := s.users.RevokeSession(ctx, revokeID); err != nil {
		return uuid.Nil, err
	}

	return revokeID, nil
}

func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.users.RevokeAllUserSessions(ctx, userID)
}

func (s *AuthService) ListSessions(ctx context.Context, userID, currentSessionID uuid.UUID) ([]SessionItem, error) {
	sessions, err := s.users.GetUserSessions(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]SessionItem, 0, len(sessions))
	for _, row := range sessions {
		items = append(items, SessionItem{
			ID:           row.ID.String(),
			UserID:       row.UserID.String(),
			Device:       toSessionDeviceView(row.Device),
			CreatedAt:    row.CreatedAt,
			ExpiresAt:    row.ExpiresAt,
			AuthProvider: normalizeSessionProvider(row.AuthProvider),
			IsCurrent:    row.ID == currentSessionID,
		})
	}

	return items, nil
}

func (s *AuthService) issueAuthForUser(ctx context.Context, u user.User, deviceInput DeviceInput, provider AuthProvider, isNewUser bool) (AuthResult, error) {
	now := time.Now().UTC()

	device, deviceExternalID, err := s.upsertDevice(ctx, u.ID, deviceInput, now)
	if err != nil {
		return AuthResult{}, err
	}

	session := user.UserSession{
		ID:           uuid.New(),
		UserID:       u.ID,
		ExpiresAt:    now,
		IsRevoked:    false,
		AuthProvider: string(normalizeSessionProvider(string(provider))),
		CreatedAt:    now,
	}
	if device != nil {
		session.DeviceID = &device.ID
	}

	pair, err := s.tokens.GenerateTokenPair(u.ID, session.ID, deviceExternalID)
	if err != nil {
		return AuthResult{}, err
	}

	session.RefreshTokenHash = pair.RefreshTokenHash
	session.ExpiresAt = pair.RefreshTokenExpiresAt

	if err := s.users.CreateSession(ctx, &session); err != nil {
		return AuthResult{}, err
	}

	return buildAuthResult(u, session, device, pair, provider, isNewUser), nil
}

func (s *AuthService) oauthClient(provider AuthProvider) (OAuthProviderClient, error) {
	client, ok := s.clients[provider]
	if !ok || client == nil {
		return nil, sentinal_errors.ErrServiceUnavailable
	}
	return client, nil
}

func (s *AuthService) resolveOAuthUser(ctx context.Context, identity OAuthIdentity) (user.User, bool, error) {
	oauthIdentity, err := s.oauth.GetByProviderSubject(ctx, string(identity.Provider), identity.ProviderUserID)
	if err == nil {
		u, getErr := s.users.GetUserByID(ctx, oauthIdentity.UserID)
		if getErr != nil {
			return user.User{}, false, getErr
		}
		return u, false, nil
	}
	if err != nil && !errors.Is(err, sentinal_errors.ErrNotFound) {
		return user.User{}, false, err
	}

	if !identity.EmailVerified || strings.TrimSpace(identity.Email) == "" {
		return user.User{}, false, ErrOAuthEmailUnverified
	}

	normalizedEmail := normalizeEmail(identity.Email)
	u, err := s.users.GetUserByEmail(ctx, normalizedEmail)
	isNewUser := false
	now := time.Now().UTC()
	if err != nil {
		if !errors.Is(err, sentinal_errors.ErrNotFound) {
			return user.User{}, false, err
		}

		passwordSeed, tokenErr := database.GenerateSecureToken(16)
		if tokenErr != nil {
			return user.User{}, false, tokenErr
		}
		passwordHash, hashErr := database.HashPassword(passwordSeed)
		if hashErr != nil {
			return user.User{}, false, hashErr
		}

		displayName := strings.TrimSpace(identity.DisplayName)
		if displayName == "" {
			displayName = fallbackDisplayName(normalizedEmail)
		}

		u = user.User{
			ID:           uuid.New(),
			DisplayName:  displayName,
			Email:        toNullString(normalizedEmail),
			PasswordHash: passwordHash,
			AvatarURL:    strings.TrimSpace(identity.AvatarURL),
			IsActive:     true,
			IsVerified:   true,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if createErr := s.users.Create(ctx, &u); createErr != nil {
			if errors.Is(createErr, sentinal_errors.ErrAlreadyExists) {
				u, createErr = s.users.GetUserByEmail(ctx, normalizedEmail)
				if createErr != nil {
					return user.User{}, false, createErr
				}
			} else {
				return user.User{}, false, createErr
			}
		}
		isNewUser = true
	}

	link := repository.OAuthIdentity{
		ID:             uuid.New(),
		UserID:         u.ID,
		Provider:       string(identity.Provider),
		ProviderUserID: strings.TrimSpace(identity.ProviderUserID),
		ProviderEmail:  normalizedEmail,
		EmailVerified:  identity.EmailVerified,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.oauth.Create(ctx, &link); err != nil {
		if !errors.Is(err, sentinal_errors.ErrAlreadyExists) {
			return user.User{}, false, err
		}

		existing, getErr := s.oauth.GetByProviderSubject(ctx, link.Provider, link.ProviderUserID)
		if getErr != nil {
			return user.User{}, false, getErr
		}
		if existing.UserID != u.ID {
			return user.User{}, false, sentinal_errors.ErrConflict
		}
	}

	return u, isNewUser, nil
}

func (s *AuthService) findUserByIdentifier(ctx context.Context, identifier string) (user.User, error) {
	identifier = strings.TrimSpace(identifier)
	if strings.Contains(identifier, "@") {
		return s.users.GetUserByEmail(ctx, normalizeEmail(identifier))
	}

	u, err := s.users.GetUserByUsername(ctx, normalizeUsername(identifier))
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, sentinal_errors.ErrNotFound) {
		return user.User{}, err
	}

	return s.users.GetUserByPhoneNumber(ctx, normalizePhone(identifier))
}

func (s *AuthService) upsertDevice(ctx context.Context, userID uuid.UUID, input DeviceInput, now time.Time) (*user.Device, string, error) {
	deviceID := strings.TrimSpace(input.DeviceID)
	if deviceID == "" {
		return nil, "", nil
	}

	device := &user.Device{
		ID:           uuid.New(),
		UserID:       userID,
		DeviceID:     deviceID,
		DeviceName:   strings.TrimSpace(input.DeviceName),
		DeviceType:   strings.TrimSpace(input.DeviceType),
		IsActive:     true,
		RegisteredAt: now,
		LastSeenAt:   sql.NullTime{Time: now, Valid: true},
	}

	if err := s.users.UpsertDevice(ctx, device); err != nil {
		return nil, "", err
	}

	return device, device.DeviceID, nil
}

func (s *AuthService) resolveSessionDevice(ctx context.Context, session user.UserSession) (*user.Device, string, error) {
	if session.DeviceID == nil {
		return nil, "", nil
	}

	device, err := s.users.GetDeviceByID(ctx, *session.DeviceID)
	if err != nil {
		return nil, "", err
	}

	return &device, device.DeviceID, nil
}

func buildAuthResult(u user.User, session user.UserSession, device *user.Device, pair TokenPair, provider AuthProvider, isNewUser bool) AuthResult {
	refresh := pair.RefreshToken
	return AuthResult{
		User:         toAuthUserView(u),
		Session:      toAuthSessionView(session, device),
		AuthProvider: provider,
		IsNewUser:    isNewUser,
		Tokens: AuthTokensView{
			AccessToken:           pair.AccessToken,
			TokenType:             "Bearer",
			ExpiresIn:             pair.AccessExpiresIn,
			ExpiresAt:             pair.AccessExpiresAt,
			RefreshToken:          &refresh,
			RefreshTokenExpiresAt: pair.RefreshTokenExpiresAt,
			RefreshTokenSet:       true,
		},
	}
}

func toAuthUserView(u user.User) AuthUserView {
	return AuthUserView{
		ID:          u.ID.String(),
		DisplayName: u.DisplayName,
		Email:       nullStringPtr(u.Email),
		Username:    nullStringPtr(u.Username),
		PhoneNumber: nullStringPtr(u.PhoneNumber),
		AvatarURL:   optionalStringPtr(u.AvatarURL),
		IsVerified:  u.IsVerified,
	}
}

func toAuthSessionView(session user.UserSession, device *user.Device) AuthSessionView {
	return AuthSessionView{
		ID:           session.ID.String(),
		UserID:       session.UserID.String(),
		Device:       toSessionDeviceView(device),
		CreatedAt:    session.CreatedAt,
		ExpiresAt:    session.ExpiresAt,
		AuthProvider: normalizeSessionProvider(session.AuthProvider),
		IsCurrent:    true,
	}
}

func toSessionDeviceView(device *user.Device) SessionDeviceView {
	if device == nil {
		return SessionDeviceView{}
	}
	id := device.ID.String()
	deviceID := strings.TrimSpace(device.DeviceID)
	deviceName := strings.TrimSpace(device.DeviceName)
	deviceType := strings.TrimSpace(device.DeviceType)

	out := SessionDeviceView{ID: &id}
	if deviceID != "" {
		out.DeviceID = &deviceID
	}
	if deviceName != "" {
		out.DeviceName = &deviceName
	}
	if deviceType != "" {
		out.DeviceType = &deviceType
	}
	return out
}

func normalizeProvider(provider AuthProvider) AuthProvider {
	switch strings.ToLower(strings.TrimSpace(string(provider))) {
	case string(AuthProviderPassword):
		return AuthProviderPassword
	case string(AuthProviderGoogle):
		return AuthProviderGoogle
	case string(AuthProviderGitHub):
		return AuthProviderGitHub
	default:
		return ""
	}
}

func normalizeSessionProvider(provider string) AuthProvider {
	normalized := normalizeProvider(AuthProvider(provider))
	if normalized == "" {
		return AuthProviderPassword
	}
	return normalized
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeUsername(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizePhone(value string) string {
	return strings.TrimSpace(value)
}

func invalidDeviceInput(input DeviceInput) bool {
	deviceID := strings.TrimSpace(input.DeviceID)
	if deviceID != "" {
		return false
	}
	return strings.TrimSpace(input.DeviceName) != "" || strings.TrimSpace(input.DeviceType) != ""
}

func fallbackDisplayName(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "User"
	}
	return parts[0]
}

func toNullString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	copy := value.String
	return &copy
}

func optionalStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}
