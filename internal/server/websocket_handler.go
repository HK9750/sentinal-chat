package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"sentinal-chat/internal/services"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub         *Hub
	authService *services.AuthService
	logger      *WebSocketLogger
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *Hub, authService *services.AuthService) *WebSocketHandler {
	return &WebSocketHandler{
		hub:         hub,
		authService: authService,
		logger:      NewWebSocketLogger(),
	}
}

// Handle upgrades HTTP to WebSocket
func (h *WebSocketHandler) Handle(c *gin.Context) {
	token := h.extractToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	claims, err := h.authService.ParseAccessToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	var deviceID uuid.UUID
	if claims.DeviceID != "" {
		deviceID, _ = uuid.Parse(claims.DeviceID)
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", userID, "", err)
		return
	}

	clientID := uuid.New().String()
	client := NewClient(h.hub, conn, userID, deviceID, clientID, *h.logger)

	h.hub.register <- client
}

func (h *WebSocketHandler) extractToken(c *gin.Context) string {
	// Check query parameter
	token := c.Query("token")
	if token != "" {
		return token
	}

	// Check Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}

	return ""
}
