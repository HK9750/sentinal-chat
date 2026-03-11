package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"sentinal-chat/internal/middleware"
	"sentinal-chat/pkg/database"
	sentinal_errors "sentinal-chat/pkg/errors"
)

type TokenService struct {
	secret           []byte
	accessTTL        time.Duration
	refreshTTL       time.Duration
	issuer           string
	acceptedAlgs     []string
	signingMethod    jwt.SigningMethod
	validationLeeway time.Duration
}

type AccessTokenClaims struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	DeviceID  string `json:"device_id,omitempty"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken           string
	AccessExpiresAt       time.Time
	AccessExpiresIn       int
	RefreshToken          string
	RefreshTokenHash      string
	RefreshTokenExpiresAt time.Time
}

func NewTokenService(secret string, accessTTL, refreshTTL time.Duration, issuer string) (*TokenService, error) {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return nil, errors.New("jwt secret is required")
	}
	if accessTTL <= 0 {
		return nil, errors.New("access ttl must be positive")
	}
	if refreshTTL <= 0 {
		return nil, errors.New("refresh ttl must be positive")
	}
	if issuer == "" {
		issuer = "sentinal-chat"
	}

	return &TokenService{
		secret:           []byte(trimmed),
		accessTTL:        accessTTL,
		refreshTTL:       refreshTTL,
		issuer:           issuer,
		acceptedAlgs:     []string{jwt.SigningMethodHS256.Alg()},
		signingMethod:    jwt.SigningMethodHS256,
		validationLeeway: 15 * time.Second,
	}, nil
}

func (s *TokenService) ParseAccessToken(token string) (*middleware.TokenClaims, error) {
	claims := new(AccessTokenClaims)
	parser := jwt.NewParser(
		jwt.WithValidMethods(s.acceptedAlgs),
		jwt.WithIssuedAt(),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(s.validationLeeway),
	)

	parsedToken, err := parser.ParseWithClaims(token, claims, func(_ *jwt.Token) (any, error) {
		return s.secret, nil
	})
	if err != nil || !parsedToken.Valid {
		return nil, sentinal_errors.ErrUnauthorized
	}

	if claims.UserID == "" || claims.SessionID == "" {
		return nil, sentinal_errors.ErrUnauthorized
	}

	return &middleware.TokenClaims{
		UserID:    claims.UserID,
		SessionID: claims.SessionID,
		DeviceID:  claims.DeviceID,
	}, nil
}

func (s *TokenService) GenerateTokenPair(userID, sessionID uuid.UUID, deviceID string) (TokenPair, error) {
	now := time.Now().UTC()
	accessExpiresAt := now.Add(s.accessTTL)

	claims := AccessTokenClaims{
		UserID:    userID.String(),
		SessionID: sessionID.String(),
		DeviceID:  strings.TrimSpace(deviceID),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExpiresAt),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(s.signingMethod, claims)
	accessToken, err := token.SignedString(s.secret)
	if err != nil {
		return TokenPair{}, err
	}

	refreshRaw, err := database.GenerateSecureToken(32)
	if err != nil {
		return TokenPair{}, err
	}

	refreshExpiresAt := now.Add(s.refreshTTL)

	return TokenPair{
		AccessToken:           accessToken,
		AccessExpiresAt:       accessExpiresAt,
		AccessExpiresIn:       int(s.accessTTL / time.Second),
		RefreshToken:          refreshRaw,
		RefreshTokenHash:      HashRefreshToken(refreshRaw),
		RefreshTokenExpiresAt: refreshExpiresAt,
	}, nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
