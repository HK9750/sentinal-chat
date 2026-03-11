package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TokenClaims holds the parsed claims from a JWT access token
type TokenClaims struct {
	UserID    string
	SessionID string
	DeviceID  string
}

// AuthMiddleware verifies the Authorization Bearer token and injects
// user_id, session_id, and device_id into the request context.
func AuthMiddleware(
	parseToken func(token string) (*TokenClaims, error),
	validateSession func(ctx context.Context, claims *TokenClaims) error,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		if parseToken == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized", "code": "UNAUTHORIZED"})
			c.Abort()
			return
		}

		token := extractBearer(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "missing token", "code": "UNAUTHORIZED"})
			c.Abort()
			return
		}

		claims, err := parseToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized", "code": "UNAUTHORIZED"})
			c.Abort()
			return
		}
		if claims == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized", "code": "UNAUTHORIZED"})
			c.Abort()
			return
		}
		if validateSession != nil {
			if err := validateSession(c.Request.Context(), claims); err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized", "code": "UNAUTHORIZED"})
				c.Abort()
				return
			}
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
