package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TokenValidator defines the interface required by AuthMiddleware.
// Implementations should parse a JWT and return user/session/device identifiers.
type TokenValidator interface {
	ParseAccessToken(token string) (*TokenClaims, error)
	ValidateSession(ctx gin.Context, sessionID, userID uuid.UUID) error
}

// TokenClaims holds the parsed claims from a JWT access token
type TokenClaims struct {
	UserID    string
	SessionID string
	DeviceID  string
}

// AuthMiddleware verifies the Authorization Bearer token and injects
// user_id, session_id, and device_id into the request context.
func AuthMiddleware(validate func(token string) (*TokenClaims, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearer(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "missing token", "code": "UNAUTHORIZED"})
			c.Abort()
			return
		}

		claims, err := validate(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized", "code": "UNAUTHORIZED"})
			c.Abort()
			return
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized", "code": "UNAUTHORIZED"})
			c.Abort()
			return
		}

		// Store parsed values in context for downstream handlers
		c.Set("user_id", userID)
		c.Set("session_id", claims.SessionID)
		c.Set("device_id", claims.DeviceID)

		c.Next()
	}
}

// extractBearer extracts a Bearer token from the Authorization header
func extractBearer(c *gin.Context) string {
	value := c.GetHeader("Authorization")
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
