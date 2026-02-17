package middleware

import (
	"net/http"
	"strings"

	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func AuthMiddleware(service *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearer(c)
		claims, err := service.ParseAccessToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
			c.Abort()
			return
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
			c.Abort()
			return
		}

		sessionID, err := uuid.Parse(claims.SessionID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
			c.Abort()
			return
		}

		session, err := service.ValidateSession(c.Request.Context(), sessionID, userID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
			c.Abort()
			return
		}

		if claims.DeviceID != "" && session.DeviceID != nil && session.DeviceID.String() != claims.DeviceID {
			c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
			c.Abort()
			return
		}

		devID := uuid.NullUUID{}
		if session.DeviceID != nil {
			devID = uuid.NullUUID{UUID: *session.DeviceID, Valid: true}
		}

		ctx := services.WithUserSessionContext(c.Request.Context(), userID, sessionID, devID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

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
