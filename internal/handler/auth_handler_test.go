package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"sentinal-chat/config"
	"sentinal-chat/internal/domain/user"
	"sentinal-chat/internal/repository"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"
	sentinal_errors "sentinal-chat/pkg/errors"
)

func TestAuthHandlerOAuthAuthorizeURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, oauthClient, _, _ := newTestAuthRouter(t)
	oauthClient.authorizeFn = func(input services.OAuthAuthorizeInput) (services.OAuthAuthorizeResult, error) {
		return services.OAuthAuthorizeResult{
			Provider:         input.Provider,
			AuthorizationURL: "https://accounts.example.com/oauth?client_id=test-client&state=" + input.State,
			RedirectURI:      input.RedirectURI,
			State:            input.State,
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/oauth/google/url?redirect_uri=http://localhost:3000/auth/callback/google&code_challenge=test-challenge&state=test-state", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response httpdto.Response[httpdto.OAuthAuthorizePayload]
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !response.Success {
		t.Fatalf("expected success response, got %+v", response)
	}
	if response.Data.Provider != "google" {
		t.Fatalf("expected provider google, got %q", response.Data.Provider)
	}
	if response.Data.RedirectURI != "http://localhost:3000/auth/callback/google" {
		t.Fatalf("unexpected redirect uri: %q", response.Data.RedirectURI)
	}
	if response.Data.State != "test-state" {
		t.Fatalf("unexpected state: %q", response.Data.State)
	}
	if !strings.Contains(response.Data.AuthorizationURL, "state=test-state") {
		t.Fatalf("expected authorization url to include state, got %q", response.Data.AuthorizationURL)
	}

	if oauthClient.lastAuthorizeInput.Provider != services.AuthProviderGoogle {
		t.Fatalf("expected provider google, got %q", oauthClient.lastAuthorizeInput.Provider)
	}
	if oauthClient.lastAuthorizeInput.CodeChallenge != "test-challenge" {
		t.Fatalf("unexpected code challenge: %q", oauthClient.lastAuthorizeInput.CodeChallenge)
	}
}

func TestAuthHandlerOAuthExchange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, oauthClient, users, oauthRepo := newTestAuthRouter(t)
	oauthClient.exchangeFn = func(_ context.Context, input services.OAuthExchangeInput) (services.OAuthIdentity, error) {
		return services.OAuthIdentity{
			Provider:       input.Provider,
			ProviderUserID: "google-user-123",
			Email:          "oauth-user@example.com",
			EmailVerified:  true,
			DisplayName:    "OAuth User",
			AvatarURL:      "https://cdn.example.com/avatar.png",
		}, nil
	}

	body := map[string]any{
		"code":          "oauth-code-123",
		"code_verifier": "verifier-123",
		"redirect_uri":  "http://localhost:3000/auth/callback/google",
		"device": map[string]any{
			"device_id":   "web-test-device",
			"device_name": "Postman Desktop",
			"device_type": "web",
		},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/oauth/google/exchange", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response httpdto.Response[httpdto.AuthPayload]
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !response.Success {
		t.Fatalf("expected success response, got %+v", response)
	}
	if response.Data.AuthProvider != "google" {
		t.Fatalf("expected auth provider google, got %q", response.Data.AuthProvider)
	}
	if response.Data.Session.AuthProvider != "google" {
		t.Fatalf("expected session auth provider google, got %q", response.Data.Session.AuthProvider)
	}
	if response.Data.User.Email == nil || *response.Data.User.Email != "oauth-user@example.com" {
		t.Fatalf("unexpected user email: %+v", response.Data.User.Email)
	}
	if !response.Data.IsNewUser {
		t.Fatalf("expected oauth exchange to create a new user")
	}
	if response.Data.Tokens.AccessToken == "" {
		t.Fatalf("expected access token in response")
	}
	if response.Data.Tokens.RefreshToken != nil {
		t.Fatalf("expected refresh token to be omitted from response body")
	}

	cookieHeader := rec.Header().Get("Set-Cookie")
	if !strings.Contains(cookieHeader, "refresh_token=") {
		t.Fatalf("expected refresh_token cookie, got %q", cookieHeader)
	}

	if oauthClient.lastExchangeInput.Provider != services.AuthProviderGoogle {
		t.Fatalf("expected provider google, got %q", oauthClient.lastExchangeInput.Provider)
	}
	if oauthClient.lastExchangeInput.Code != "oauth-code-123" {
		t.Fatalf("unexpected code: %q", oauthClient.lastExchangeInput.Code)
	}
	if oauthClient.lastExchangeInput.CodeVerifier != "verifier-123" {
		t.Fatalf("unexpected code verifier: %q", oauthClient.lastExchangeInput.CodeVerifier)
	}
	if len(users.sessions) != 1 {
		t.Fatalf("expected one session to be stored, got %d", len(users.sessions))
	}
	if len(oauthRepo.identities) != 1 {
		t.Fatalf("expected one oauth identity to be stored, got %d", len(oauthRepo.identities))
	}
}

func newTestAuthRouter(t *testing.T) (*gin.Engine, *stubOAuthProviderClient, *stubUserRepository, *stubOAuthIdentityRepository) {
	t.Helper()

	tokens, err := services.NewTokenService("test-secret", time.Hour, 24*time.Hour, "sentinal-chat-test")
	if err != nil {
		t.Fatalf("create token service: %v", err)
	}

	users := newStubUserRepository()
	oauthRepo := newStubOAuthIdentityRepository()
	oauthClient := &stubOAuthProviderClient{}

	authService, err := services.NewAuthService(users, oauthRepo, tokens, map[services.AuthProvider]services.OAuthProviderClient{
		services.AuthProviderGoogle: oauthClient,
	})
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}

	handler := NewAuthHandler(authService, &config.Config{RefreshExpiry: 14, CookieSecure: false}, nil)
	router := gin.New()
	v1 := router.Group("/v1")
	handler.RegisterPublicRoutes(v1)

	return router, oauthClient, users, oauthRepo
}

type stubOAuthProviderClient struct {
	authorizeFn        func(input services.OAuthAuthorizeInput) (services.OAuthAuthorizeResult, error)
	exchangeFn         func(ctx context.Context, input services.OAuthExchangeInput) (services.OAuthIdentity, error)
	lastAuthorizeInput services.OAuthAuthorizeInput
	lastExchangeInput  services.OAuthExchangeInput
}

func (s *stubOAuthProviderClient) AuthorizationURL(input services.OAuthAuthorizeInput) (services.OAuthAuthorizeResult, error) {
	s.lastAuthorizeInput = input
	if s.authorizeFn != nil {
		return s.authorizeFn(input)
	}
	return services.OAuthAuthorizeResult{}, nil
}

func (s *stubOAuthProviderClient) ExchangeCode(ctx context.Context, input services.OAuthExchangeInput) (services.OAuthIdentity, error) {
	s.lastExchangeInput = input
	if s.exchangeFn != nil {
		return s.exchangeFn(ctx, input)
	}
	return services.OAuthIdentity{}, nil
}

type stubOAuthIdentityRepository struct {
	identities map[string]repository.OAuthIdentity
}

func newStubOAuthIdentityRepository() *stubOAuthIdentityRepository {
	return &stubOAuthIdentityRepository{identities: make(map[string]repository.OAuthIdentity)}
}

func (s *stubOAuthIdentityRepository) Create(_ context.Context, identity *repository.OAuthIdentity) error {
	if identity == nil {
		return sentinal_errors.ErrInvalidInput
	}
	key := oauthIdentityKey(identity.Provider, identity.ProviderUserID)
	if _, exists := s.identities[key]; exists {
		return sentinal_errors.ErrAlreadyExists
	}
	s.identities[key] = *identity
	return nil
}

func (s *stubOAuthIdentityRepository) GetByProviderSubject(_ context.Context, provider, providerUserID string) (repository.OAuthIdentity, error) {
	identity, ok := s.identities[oauthIdentityKey(provider, providerUserID)]
	if !ok {
		return repository.OAuthIdentity{}, sentinal_errors.ErrNotFound
	}
	return identity, nil
}

func oauthIdentityKey(provider, providerUserID string) string {
	return strings.ToLower(strings.TrimSpace(provider)) + ":" + strings.TrimSpace(providerUserID)
}

type stubUserRepository struct {
	usersByID          map[uuid.UUID]user.User
	usersByEmail       map[string]uuid.UUID
	usersByUsername    map[string]uuid.UUID
	usersByPhone       map[string]uuid.UUID
	devicesByID        map[uuid.UUID]user.Device
	devicesByLookup    map[string]uuid.UUID
	sessions           map[uuid.UUID]user.UserSession
	sessionsByHash     map[string]uuid.UUID
	userSessionIndexes map[uuid.UUID][]uuid.UUID
}

func newStubUserRepository() *stubUserRepository {
	return &stubUserRepository{
		usersByID:          make(map[uuid.UUID]user.User),
		usersByEmail:       make(map[string]uuid.UUID),
		usersByUsername:    make(map[string]uuid.UUID),
		usersByPhone:       make(map[string]uuid.UUID),
		devicesByID:        make(map[uuid.UUID]user.Device),
		devicesByLookup:    make(map[string]uuid.UUID),
		sessions:           make(map[uuid.UUID]user.UserSession),
		sessionsByHash:     make(map[string]uuid.UUID),
		userSessionIndexes: make(map[uuid.UUID][]uuid.UUID),
	}
}

func (s *stubUserRepository) Create(_ context.Context, u *user.User) error {
	if u == nil {
		return sentinal_errors.ErrInvalidInput
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = u.CreatedAt
	}
	s.usersByID[u.ID] = *u
	if u.Email.Valid {
		s.usersByEmail[strings.ToLower(strings.TrimSpace(u.Email.String))] = u.ID
	}
	if u.Username.Valid {
		s.usersByUsername[strings.ToLower(strings.TrimSpace(u.Username.String))] = u.ID
	}
	if u.PhoneNumber.Valid {
		s.usersByPhone[strings.TrimSpace(u.PhoneNumber.String)] = u.ID
	}
	return nil
}

func (s *stubUserRepository) GetAllUsers(context.Context, int, int) ([]user.User, int64, error) {
	return nil, 0, nil
}

func (s *stubUserRepository) GetUserByID(_ context.Context, id uuid.UUID) (user.User, error) {
	u, ok := s.usersByID[id]
	if !ok {
		return user.User{}, sentinal_errors.ErrNotFound
	}
	return u, nil
}

func (s *stubUserRepository) UpdateUser(_ context.Context, u user.User) error {
	if _, ok := s.usersByID[u.ID]; !ok {
		return sentinal_errors.ErrNotFound
	}
	s.usersByID[u.ID] = u
	return nil
}

func (s *stubUserRepository) DeleteUser(_ context.Context, id uuid.UUID) error {
	delete(s.usersByID, id)
	return nil
}

func (s *stubUserRepository) GetUserByEmail(_ context.Context, email string) (user.User, error) {
	id, ok := s.usersByEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return user.User{}, sentinal_errors.ErrNotFound
	}
	return s.usersByID[id], nil
}

func (s *stubUserRepository) GetUserByUsername(_ context.Context, username string) (user.User, error) {
	id, ok := s.usersByUsername[strings.ToLower(strings.TrimSpace(username))]
	if !ok {
		return user.User{}, sentinal_errors.ErrNotFound
	}
	return s.usersByID[id], nil
}

func (s *stubUserRepository) GetUserByPhoneNumber(_ context.Context, phone string) (user.User, error) {
	id, ok := s.usersByPhone[strings.TrimSpace(phone)]
	if !ok {
		return user.User{}, sentinal_errors.ErrNotFound
	}
	return s.usersByID[id], nil
}

func (s *stubUserRepository) SearchUsers(context.Context, string, int, int) ([]user.User, int64, error) {
	return nil, 0, nil
}

func (s *stubUserRepository) UpdateOnlineStatus(_ context.Context, userID uuid.UUID, isOnline bool) error {
	u, ok := s.usersByID[userID]
	if !ok {
		return sentinal_errors.ErrNotFound
	}
	u.IsOnline = isOnline
	s.usersByID[userID] = u
	return nil
}

func (s *stubUserRepository) UpdateLastSeen(_ context.Context, userID uuid.UUID, lastSeen time.Time) error {
	u, ok := s.usersByID[userID]
	if !ok {
		return sentinal_errors.ErrNotFound
	}
	u.LastSeenAt = sql.NullTime{Time: lastSeen, Valid: true}
	s.usersByID[userID] = u
	return nil
}

func (s *stubUserRepository) GetUserContacts(context.Context, uuid.UUID) ([]user.UserContact, error) {
	return nil, nil
}

func (s *stubUserRepository) AddUserContact(context.Context, *user.UserContact) error {
	return nil
}

func (s *stubUserRepository) RemoveUserContact(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (s *stubUserRepository) BlockContact(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (s *stubUserRepository) UnblockContact(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (s *stubUserRepository) GetBlockedContacts(context.Context, uuid.UUID) ([]user.UserContact, error) {
	return nil, nil
}

func (s *stubUserRepository) AddDevice(ctx context.Context, d *user.Device) error {
	return s.UpsertDevice(ctx, d)
}

func (s *stubUserRepository) GetUserDevices(_ context.Context, userID uuid.UUID) ([]user.Device, error) {
	devices := make([]user.Device, 0)
	for _, d := range s.devicesByID {
		if d.UserID == userID {
			devices = append(devices, d)
		}
	}
	return devices, nil
}

func (s *stubUserRepository) GetDeviceByID(_ context.Context, deviceID uuid.UUID) (user.Device, error) {
	d, ok := s.devicesByID[deviceID]
	if !ok {
		return user.Device{}, sentinal_errors.ErrNotFound
	}
	return d, nil
}

func (s *stubUserRepository) DeactivateDevice(_ context.Context, deviceID uuid.UUID) error {
	d, ok := s.devicesByID[deviceID]
	if !ok {
		return sentinal_errors.ErrNotFound
	}
	d.IsActive = false
	s.devicesByID[deviceID] = d
	return nil
}

func (s *stubUserRepository) UpdateDeviceLastSeen(_ context.Context, deviceID uuid.UUID) error {
	d, ok := s.devicesByID[deviceID]
	if !ok {
		return sentinal_errors.ErrNotFound
	}
	d.LastSeenAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	s.devicesByID[deviceID] = d
	return nil
}

func (s *stubUserRepository) AddFcmToken(context.Context, *user.FcmToken) error {
	return nil
}

func (s *stubUserRepository) GetUserFcmTokens(context.Context, uuid.UUID) ([]user.FcmToken, error) {
	return nil, nil
}

func (s *stubUserRepository) DeactivateFcmToken(context.Context, uuid.UUID) error {
	return nil
}

func (s *stubUserRepository) CreateSession(_ context.Context, session *user.UserSession) error {
	if session == nil {
		return sentinal_errors.ErrInvalidInput
	}
	s.sessions[session.ID] = *session
	s.sessionsByHash[session.RefreshTokenHash] = session.ID
	s.userSessionIndexes[session.UserID] = append(s.userSessionIndexes[session.UserID], session.ID)
	return nil
}

func (s *stubUserRepository) GetSessionByID(_ context.Context, sessionID uuid.UUID) (user.UserSession, error) {
	session, ok := s.sessions[sessionID]
	if !ok {
		return user.UserSession{}, sentinal_errors.ErrNotFound
	}
	return session, nil
}

func (s *stubUserRepository) GetSessionByRefreshTokenHash(_ context.Context, refreshTokenHash string) (user.UserSession, error) {
	sessionID, ok := s.sessionsByHash[refreshTokenHash]
	if !ok {
		return user.UserSession{}, sentinal_errors.ErrNotFound
	}
	return s.sessions[sessionID], nil
}

func (s *stubUserRepository) GetUserSessions(_ context.Context, userID uuid.UUID) ([]user.UserSession, error) {
	ids := s.userSessionIndexes[userID]
	out := make([]user.UserSession, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.sessions[id])
	}
	return out, nil
}

func (s *stubUserRepository) UpdateSession(_ context.Context, session user.UserSession) error {
	if _, ok := s.sessions[session.ID]; !ok {
		return sentinal_errors.ErrNotFound
	}
	s.sessions[session.ID] = session
	s.sessionsByHash[session.RefreshTokenHash] = session.ID
	return nil
}

func (s *stubUserRepository) RevokeSession(_ context.Context, sessionID uuid.UUID) error {
	session, ok := s.sessions[sessionID]
	if !ok {
		return sentinal_errors.ErrNotFound
	}
	session.IsRevoked = true
	s.sessions[sessionID] = session
	return nil
}

func (s *stubUserRepository) RevokeAllUserSessions(_ context.Context, userID uuid.UUID) error {
	for _, sessionID := range s.userSessionIndexes[userID] {
		session := s.sessions[sessionID]
		session.IsRevoked = true
		s.sessions[sessionID] = session
	}
	return nil
}

func (s *stubUserRepository) CleanExpiredSessions(context.Context) error {
	return nil
}

func (s *stubUserRepository) UpsertDevice(_ context.Context, d *user.Device) error {
	if d == nil {
		return sentinal_errors.ErrInvalidInput
	}
	key := s.deviceLookupKey(d.UserID, d.DeviceID)
	if existingID, ok := s.devicesByLookup[key]; ok {
		existing := s.devicesByID[existingID]
		existing.DeviceName = d.DeviceName
		existing.DeviceType = d.DeviceType
		existing.IsActive = true
		existing.LastSeenAt = d.LastSeenAt
		s.devicesByID[existingID] = existing
		d.ID = existingID
		return nil
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	s.devicesByLookup[key] = d.ID
	s.devicesByID[d.ID] = *d
	return nil
}

func (s *stubUserRepository) deviceLookupKey(userID uuid.UUID, deviceID string) string {
	return userID.String() + ":" + strings.TrimSpace(deviceID)
}

var _ services.OAuthProviderClient = (*stubOAuthProviderClient)(nil)
var _ repository.OAuthIdentityRepository = (*stubOAuthIdentityRepository)(nil)
var _ repository.UserRepository = (*stubUserRepository)(nil)
