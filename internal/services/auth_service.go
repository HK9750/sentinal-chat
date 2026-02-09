package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"sentinal-chat/config"
	"sentinal-chat/internal/domain/user"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo   repository.UserRepository
	jwtSecret  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewAuthService(userRepo repository.UserRepository, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		jwtSecret:  []byte(cfg.JWTSecret),
		accessTTL:  time.Duration(cfg.JWTExpiryHours) * time.Hour,
		refreshTTL: time.Duration(cfg.RefreshExpiry) * 24 * time.Hour,
	}
}

type RegisterInput struct {
	Email       string
	Username    string
	PhoneNumber string
	Password    string
	DisplayName string
	DeviceID    string
	DeviceName  string
	DeviceType  string
}

type LoginInput struct {
	Identity   string
	Password   string
	DeviceID   string
	DeviceName string
	DeviceType string
}

type RefreshInput struct {
	SessionID    string
	RefreshToken string
}

type ResetInput struct {
	Identity    string
	NewPassword string
}

type AuthResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token,omitempty"`
	ExpiresIn    int64    `json:"expires_in"`
	SessionID    string   `json:"session_id"`
	User         UserInfo `json:"user"`
}

type UserInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username,omitempty"`
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
}

type SessionInfo struct {
	ID        string    `json:"id"`
	DeviceID  string    `json:"device_id,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	IsRevoked bool      `json:"is_revoked"`
}

type AccessClaims struct {
	UserID    string `json:"sub"`
	SessionID string `json:"sid"`
	DeviceID  string `json:"did,omitempty"`
	jwt.RegisteredClaims
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (AuthResponse, error) {
	if err := validateRegister(in); err != nil {
		return AuthResponse{}, err
	}

	if err := s.ensureIdentityAvailable(ctx, in); err != nil {
		return AuthResponse{}, err
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		return AuthResponse{}, err
	}

	newUser := &user.User{
		ID:           uuid.New(),
		Email:        toNullString(in.Email),
		Username:     toNullString(in.Username),
		PhoneNumber:  toNullString(in.PhoneNumber),
		PasswordHash: hash,
		DisplayName:  in.DisplayName,
		IsActive:     true,
		IsVerified:   false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return AuthResponse{}, err
	}

	settings := &user.UserSettings{
		UserID:               newUser.ID,
		PrivacyLastSeen:      "EVERYONE",
		PrivacyProfilePhoto:  "EVERYONE",
		PrivacyAbout:         "EVERYONE",
		PrivacyGroups:        "EVERYONE",
		ReadReceipts:         true,
		NotificationsEnabled: true,
		Theme:                "SYSTEM",
		Language:             "en",
		UpdatedAt:            time.Now(),
	}
	if err := s.userRepo.CreateUserSettings(ctx, settings); err != nil {
		return AuthResponse{}, err
	}

	deviceID, err := s.getOrCreateDevice(ctx, newUser.ID, in.DeviceID, in.DeviceName, in.DeviceType)
	if err != nil {
		return AuthResponse{}, err
	}

	refreshToken, err := generateToken(32)
	if err != nil {
		return AuthResponse{}, err
	}

	refreshHash := s.hashRefreshToken(refreshToken)
	createdAt := time.Now()
	session := &user.UserSession{
		ID:               uuid.New(),
		UserID:           newUser.ID,
		DeviceID:         deviceID,
		RefreshTokenHash: refreshHash,
		ExpiresAt:        createdAt.Add(s.refreshTTL),
		CreatedAt:        createdAt,
	}

	if err := s.userRepo.CreateSession(ctx, session); err != nil {
		return AuthResponse{}, err
	}

	accessToken, expiresIn, err := s.newAccessToken(newUser.ID, session.ID, deviceID)
	if err != nil {
		return AuthResponse{}, err
	}

	return AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		SessionID:    session.ID.String(),
		User:         toUserInfo(*newUser),
	}, nil
}

func (s *AuthService) Login(ctx context.Context, in LoginInput) (AuthResponse, error) {
	if err := validateLogin(in); err != nil {
		return AuthResponse{}, err
	}

	u, err := s.getUserByIdentity(ctx, in.Identity)
	if err != nil {
		return AuthResponse{}, err
	}

	if !u.IsActive {
		return AuthResponse{}, sentinal_errors.ErrForbidden
	}

	if err := comparePassword(u.PasswordHash, in.Password); err != nil {
		return AuthResponse{}, sentinal_errors.ErrUnauthorized
	}

	deviceID, err := s.getOrCreateDevice(ctx, u.ID, in.DeviceID, in.DeviceName, in.DeviceType)
	if err != nil {
		return AuthResponse{}, err
	}

	refreshToken, err := generateToken(32)
	if err != nil {
		return AuthResponse{}, err
	}

	refreshHash := s.hashRefreshToken(refreshToken)
	createdAt := time.Now()
	session := &user.UserSession{
		ID:               uuid.New(),
		UserID:           u.ID,
		DeviceID:         deviceID,
		RefreshTokenHash: refreshHash,
		ExpiresAt:        createdAt.Add(s.refreshTTL),
		CreatedAt:        createdAt,
	}

	if err := s.userRepo.CreateSession(ctx, session); err != nil {
		return AuthResponse{}, err
	}

	_ = s.userRepo.UpdateOnlineStatus(ctx, u.ID, true)

	accessToken, expiresIn, err := s.newAccessToken(u.ID, session.ID, deviceID)
	if err != nil {
		return AuthResponse{}, err
	}

	return AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		SessionID:    session.ID.String(),
		User:         toUserInfo(u),
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, in RefreshInput) (AuthResponse, error) {
	if in.SessionID == "" || in.RefreshToken == "" {
		return AuthResponse{}, sentinal_errors.ErrInvalidInput
	}

	sessionID, err := uuid.Parse(in.SessionID)
	if err != nil {
		return AuthResponse{}, sentinal_errors.ErrInvalidInput
	}

	session, err := s.userRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return AuthResponse{}, err
	}

	if session.IsRevoked || time.Now().After(session.ExpiresAt) {
		return AuthResponse{}, sentinal_errors.ErrUnauthorized
	}

	if !s.compareRefreshToken(session.RefreshTokenHash, in.RefreshToken) {
		_ = s.userRepo.RevokeSession(ctx, session.ID)
		return AuthResponse{}, sentinal_errors.ErrUnauthorized
	}

	newRefresh, err := generateToken(32)
	if err != nil {
		return AuthResponse{}, err
	}

	session.RefreshTokenHash = s.hashRefreshToken(newRefresh)
	session.ExpiresAt = time.Now().Add(s.refreshTTL)

	if err := s.userRepo.UpdateSession(ctx, session); err != nil {
		return AuthResponse{}, err
	}

	accessToken, expiresIn, err := s.newAccessToken(session.UserID, session.ID, session.DeviceID)
	if err != nil {
		return AuthResponse{}, err
	}

	userInfo, err := s.userRepo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return AuthResponse{}, err
	}

	return AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		ExpiresIn:    expiresIn,
		SessionID:    session.ID.String(),
		User:         toUserInfo(userInfo),
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return sentinal_errors.ErrInvalidInput
	}
	parsedID, err := uuid.Parse(sessionID)
	if err != nil {
		return sentinal_errors.ErrInvalidInput
	}
	return s.userRepo.RevokeSession(ctx, parsedID)
}

func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.userRepo.RevokeAllUserSessions(ctx, userID)
}

func (s *AuthService) Sessions(ctx context.Context, userID uuid.UUID) ([]SessionInfo, error) {
	sessions, err := s.userRepo.GetUserSessions(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		item := SessionInfo{
			ID:        session.ID.String(),
			ExpiresAt: session.ExpiresAt,
			CreatedAt: session.CreatedAt,
			IsRevoked: session.IsRevoked,
		}
		if session.DeviceID.Valid {
			item.DeviceID = session.DeviceID.UUID.String()
		}
		result = append(result, item)
	}

	return result, nil
}

func (s *AuthService) PasswordForgot(ctx context.Context, identity string) error {
	if identity == "" {
		return sentinal_errors.ErrInvalidInput
	}

	_, err := s.getUserByIdentity(ctx, identity)
	if err != nil {
		if errors.Is(err, sentinal_errors.ErrNotFound) {
			return nil
		}
		return err
	}

	return nil
}

func (s *AuthService) PasswordReset(ctx context.Context, in ResetInput) error {
	if in.Identity == "" || in.NewPassword == "" {
		return sentinal_errors.ErrInvalidInput
	}

	if len(in.NewPassword) < 8 {
		return sentinal_errors.ErrInvalidInput
	}

	u, err := s.getUserByIdentity(ctx, in.Identity)
	if err != nil {
		return err
	}

	newHash, err := hashPassword(in.NewPassword)
	if err != nil {
		return err
	}

	u.PasswordHash = newHash
	u.UpdatedAt = time.Now()

	if err := s.userRepo.UpdateUser(ctx, u); err != nil {
		return err
	}

	return s.userRepo.RevokeAllUserSessions(ctx, u.ID)
}

func (s *AuthService) ParseAccessToken(tokenString string) (AccessClaims, error) {
	if tokenString == "" {
		return AccessClaims{}, sentinal_errors.ErrUnauthorized
	}

	parsed, err := jwt.ParseWithClaims(tokenString, &AccessClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, sentinal_errors.ErrUnauthorized
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return AccessClaims{}, sentinal_errors.ErrUnauthorized
	}

	claims, ok := parsed.Claims.(*AccessClaims)
	if !ok || !parsed.Valid {
		return AccessClaims{}, sentinal_errors.ErrUnauthorized
	}

	return *claims, nil
}

func (s *AuthService) ValidateSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) (user.UserSession, error) {
	session, err := s.userRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return user.UserSession{}, err
	}
	if session.UserID != userID {
		return user.UserSession{}, sentinal_errors.ErrUnauthorized
	}
	return session, nil
}

func HTTPStatus(err error) int {
	switch {
	case errors.Is(err, sentinal_errors.ErrInvalidInput):
		return 400
	case errors.Is(err, sentinal_errors.ErrUnauthorized):
		return 401
	case errors.Is(err, sentinal_errors.ErrForbidden):
		return 403
	case errors.Is(err, sentinal_errors.ErrNotFound):
		return 404
	case errors.Is(err, sentinal_errors.ErrAlreadyExists), errors.Is(err, sentinal_errors.ErrConflict):
		return 409
	case errors.Is(err, sentinal_errors.ErrRateLimited):
		return 429
	default:
		return 500
	}
}

type ctxKey string

var userIDKey ctxKey = "user_id"
var sessionIDKey ctxKey = "session_id"
var deviceIDKey ctxKey = "device_id"

func WithUserSessionContext(ctx context.Context, userID, sessionID uuid.UUID, deviceID uuid.NullUUID) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	ctx = context.WithValue(ctx, sessionIDKey, sessionID)
	if deviceID.Valid {
		ctx = context.WithValue(ctx, deviceIDKey, deviceID)
	}
	return ctx
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	value := ctx.Value(userIDKey)
	if value == nil {
		return uuid.Nil, false
	}
	userID, ok := value.(uuid.UUID)
	return userID, ok
}

func SessionIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	value := ctx.Value(sessionIDKey)
	if value == nil {
		return uuid.Nil, false
	}
	sessionID, ok := value.(uuid.UUID)
	return sessionID, ok
}

func DeviceIDFromContext(ctx context.Context) (uuid.NullUUID, bool) {
	value := ctx.Value(deviceIDKey)
	if value == nil {
		return uuid.NullUUID{}, false
	}
	deviceID, ok := value.(uuid.NullUUID)
	return deviceID, ok
}

func (s *AuthService) ensureIdentityAvailable(ctx context.Context, in RegisterInput) error {
	if in.Email != "" {
		if _, err := s.userRepo.GetUserByEmail(ctx, in.Email); err == nil {
			return sentinal_errors.ErrAlreadyExists
		} else if !errors.Is(err, sentinal_errors.ErrNotFound) {
			return err
		}
	}

	if in.Username != "" {
		if _, err := s.userRepo.GetUserByUsername(ctx, in.Username); err == nil {
			return sentinal_errors.ErrAlreadyExists
		} else if !errors.Is(err, sentinal_errors.ErrNotFound) {
			return err
		}
	}

	if in.PhoneNumber != "" {
		if _, err := s.userRepo.GetUserByPhoneNumber(ctx, in.PhoneNumber); err == nil {
			return sentinal_errors.ErrAlreadyExists
		} else if !errors.Is(err, sentinal_errors.ErrNotFound) {
			return err
		}
	}

	return nil
}

func (s *AuthService) getUserByIdentity(ctx context.Context, identity string) (user.User, error) {
	if identity == "" {
		return user.User{}, sentinal_errors.ErrInvalidInput
	}

	if strings.Contains(identity, "@") {
		u, err := s.userRepo.GetUserByEmail(ctx, identity)
		if err == nil {
			return u, nil
		}
		if !errors.Is(err, sentinal_errors.ErrNotFound) {
			return user.User{}, err
		}
	}

	u, err := s.userRepo.GetUserByUsername(ctx, identity)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, sentinal_errors.ErrNotFound) {
		return user.User{}, err
	}

	u, err = s.userRepo.GetUserByPhoneNumber(ctx, identity)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, sentinal_errors.ErrNotFound) {
		return user.User{}, err
	}

	return user.User{}, sentinal_errors.ErrNotFound
}

func (s *AuthService) getOrCreateDevice(ctx context.Context, userID uuid.UUID, deviceID, deviceName, deviceType string) (uuid.NullUUID, error) {
	if deviceID == "" {
		return uuid.NullUUID{Valid: false}, nil
	}

	devices, err := s.userRepo.GetUserDevices(ctx, userID)
	if err != nil {
		return uuid.NullUUID{}, err
	}

	for _, d := range devices {
		if d.DeviceID == deviceID {
			_ = s.userRepo.UpdateDeviceLastSeen(ctx, d.ID)
			return uuid.NullUUID{UUID: d.ID, Valid: true}, nil
		}
	}

	newDevice := &user.Device{
		ID:           uuid.New(),
		UserID:       userID,
		DeviceID:     deviceID,
		DeviceName:   deviceName,
		DeviceType:   deviceType,
		IsActive:     true,
		RegisteredAt: time.Now(),
		LastSeenAt:   sql.NullTime{Time: time.Now(), Valid: true},
	}

	if err := s.userRepo.AddDevice(ctx, newDevice); err != nil {
		if errors.Is(err, sentinal_errors.ErrAlreadyExists) {
			devices, fetchErr := s.userRepo.GetUserDevices(ctx, userID)
			if fetchErr != nil {
				return uuid.NullUUID{}, fetchErr
			}
			for _, d := range devices {
				if d.DeviceID == deviceID {
					return uuid.NullUUID{UUID: d.ID, Valid: true}, nil
				}
			}
		}
		return uuid.NullUUID{}, err
	}

	return uuid.NullUUID{UUID: newDevice.ID, Valid: true}, nil
}

func (s *AuthService) newAccessToken(userID, sessionID uuid.UUID, deviceID uuid.NullUUID) (string, int64, error) {
	now := time.Now()
	expiresAt := now.Add(s.accessTTL)

	claims := AccessClaims{
		UserID:    userID.String(),
		SessionID: sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	if deviceID.Valid {
		claims.DeviceID = deviceID.UUID.String()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", 0, err
	}

	return signed, int64(s.accessTTL.Seconds()), nil
}

func (s *AuthService) hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *AuthService) compareRefreshToken(hash, token string) bool {
	computed := s.hashRefreshToken(token)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(computed)) == 1
}

func validateRegister(in RegisterInput) error {
	if in.Password == "" || in.DisplayName == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if in.Email == "" && in.Username == "" && in.PhoneNumber == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if len(in.Password) < 8 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func validateLogin(in LoginInput) error {
	if in.Identity == "" || in.Password == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func comparePassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func generateToken(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func toNullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func toUserInfo(u user.User) UserInfo {
	info := UserInfo{
		ID:          u.ID.String(),
		DisplayName: u.DisplayName,
	}
	if u.Username.Valid {
		info.Username = u.Username.String
	}
	if u.Email.Valid {
		info.Email = u.Email.String
	}
	if u.PhoneNumber.Valid {
		info.PhoneNumber = u.PhoneNumber.String
	}
	return info
}
