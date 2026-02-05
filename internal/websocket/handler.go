package websocket

import (
	"context"
	"net/http"
	"strings"
	"time"

	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Handler struct {
	auth *services.AuthService
	hub  *Hub
}

func NewHandler(auth *services.AuthService, hub *Hub) *Handler {
	return &Handler{auth: auth, hub: hub}
}

func (h *Handler) Connect(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	claims, err := h.auth.ParseAccessToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	userID := strings.TrimSpace(claims.UserID)
	client := NewClient(conn, "channel:user:"+userID)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.hub.Register(client)
	go client.WriteLoop(ctx)

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	}

	h.hub.Unregister(client)
}
