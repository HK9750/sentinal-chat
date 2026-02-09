package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/events"
	"sentinal-chat/internal/redis"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// WSMessage represents an incoming WebSocket message
// See Appendix C of database.md
type WSMessage struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

// WSResponse represents an outgoing WebSocket response
type WSResponse struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Success   bool   `json:"success"`
	Payload   any    `json:"payload,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Handler handles WebSocket connections and messages
type Handler struct {
	auth       *services.AuthService
	hub        *Hub
	bus        *commands.Bus
	authorizer *ChannelAuthorizer
	presence   *redis.PresenceStore
}

// NewHandler creates a new WebSocket handler
func NewHandler(auth *services.AuthService, hub *Hub, bus *commands.Bus, authorizer *ChannelAuthorizer, presence *redis.PresenceStore) *Handler {
	return &Handler{auth: auth, hub: hub, bus: bus, authorizer: authorizer, presence: presence}
}

// Connect handles WebSocket connection upgrade and message processing
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
	deviceID := c.Query("device_id")
	client := NewClient(conn, userID)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register client with the hub
	h.hub.Register(client)

	// Track presence - mark user as online
	if h.presence != nil {
		h.presence.SetOnline(ctx, userID, deviceID, client.ID)
		h.presence.TrackUserConnection(ctx, userID, client.ID, deviceID)
	}

	// Auto-subscribe to user's personal channel
	userChannel := events.ChannelPrefixUser + userID
	h.hub.Subscribe(client, userChannel)

	// Auto-subscribe to presence channel
	presenceChannel := events.ChannelPrefixPresence + userID
	h.hub.Subscribe(client, presenceChannel)

	// Start write loop in goroutine
	go client.WriteLoop(ctx)

	// Send connected acknowledgment
	h.sendResponse(client, WSResponse{
		Type:    "connected",
		Success: true,
		Payload: map[string]any{
			"client_id":  client.ID,
			"user_id":    userID,
			"subscribed": []string{userChannel, presenceChannel},
		},
	})

	// Read loop
	h.readLoop(ctx, client, userID)

	// Cleanup on disconnect
	h.hub.Unregister(client)

	// Track presence - handle disconnect
	if h.presence != nil {
		h.presence.RemoveUserConnection(context.Background(), userID, client.ID)
	}
}

// readLoop handles incoming messages from the client
func (h *Handler) readLoop(ctx context.Context, client *Client, userID string) {
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		// Update heartbeat on pong
		if h.presence != nil {
			h.presence.Heartbeat(ctx, userID)
		}
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, data, err := client.Conn.ReadMessage()
		if err != nil {
			return
		}
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Update heartbeat on any message
		if h.presence != nil {
			h.presence.Heartbeat(ctx, userID)
		}

		// Parse incoming message
		var msg WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			h.sendResponse(client, WSResponse{
				Type:      "error",
				RequestID: msg.RequestID,
				Success:   false,
				Error:     "invalid message format",
			})
			continue
		}

		// Generate request ID if not provided
		if msg.RequestID == "" {
			msg.RequestID = uuid.New().String()
		}

		// Handle the message
		h.handleMessage(ctx, client, userID, msg)
	}
}

// handleMessage routes incoming messages to appropriate handlers
func (h *Handler) handleMessage(ctx context.Context, client *Client, userID string, msg WSMessage) {
	switch msg.Type {
	// Channel management
	case "subscribe":
		h.handleSubscribe(ctx, client, userID, msg)
	case "unsubscribe":
		h.handleUnsubscribe(ctx, client, msg)

	// System messages
	case "ping":
		h.sendResponse(client, WSResponse{Type: "pong", RequestID: msg.RequestID, Success: true})

	// Message actions (routed to command bus)
	case "message.send", "message.edit", "message.delete", "message.react",
		"message.read", "message.typing", "message.star", "message.unstar":
		h.handleCommand(ctx, client, userID, msg)

	// Call actions (routed to command bus)
	case "call.offer", "call.answer", "call.ice", "call.join", "call.leave", "call.end":
		h.handleCommand(ctx, client, userID, msg)

	// Presence actions
	case "presence.update":
		h.handleCommand(ctx, client, userID, msg)

	default:
		h.sendResponse(client, WSResponse{
			Type:      "error",
			RequestID: msg.RequestID,
			Success:   false,
			Error:     "unknown message type: " + msg.Type,
		})
	}
}

// handleSubscribe handles channel subscription requests
func (h *Handler) handleSubscribe(ctx context.Context, client *Client, userID string, msg WSMessage) {
	var payload struct {
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		h.sendResponse(client, WSResponse{
			Type:      "subscribed",
			RequestID: msg.RequestID,
			Success:   false,
			Error:     "invalid payload",
		})
		return
	}

	// Validate channel access using authorizer
	canSubscribe, err := h.canSubscribe(ctx, userID, payload.Channel)
	if err != nil || !canSubscribe {
		h.sendResponse(client, WSResponse{
			Type:      "subscribed",
			RequestID: msg.RequestID,
			Success:   false,
			Error:     "access denied to channel",
		})
		return
	}

	h.hub.Subscribe(client, payload.Channel)
	h.sendResponse(client, WSResponse{
		Type:      "subscribed",
		RequestID: msg.RequestID,
		Success:   true,
		Payload:   map[string]string{"channel": payload.Channel},
	})
}

// handleUnsubscribe handles channel unsubscription requests
func (h *Handler) handleUnsubscribe(ctx context.Context, client *Client, msg WSMessage) {
	var payload struct {
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		h.sendResponse(client, WSResponse{
			Type:      "unsubscribed",
			RequestID: msg.RequestID,
			Success:   false,
			Error:     "invalid payload",
		})
		return
	}

	h.hub.Unsubscribe(client, payload.Channel)
	h.sendResponse(client, WSResponse{
		Type:      "unsubscribed",
		RequestID: msg.RequestID,
		Success:   true,
		Payload:   map[string]string{"channel": payload.Channel},
	})
}

// handleCommand routes commands to the command bus
func (h *Handler) handleCommand(ctx context.Context, client *Client, userID string, msg WSMessage) {
	if h.bus == nil {
		h.sendResponse(client, WSResponse{
			Type:      msg.Type + ".response",
			RequestID: msg.RequestID,
			Success:   false,
			Error:     "command bus not available",
		})
		return
	}

	// Create command from WebSocket message
	cmd, err := h.parseCommand(userID, msg)
	if err != nil {
		h.sendResponse(client, WSResponse{
			Type:      msg.Type + ".response",
			RequestID: msg.RequestID,
			Success:   false,
			Error:     err.Error(),
		})
		return
	}

	// Execute command
	result, err := h.bus.Execute(ctx, cmd)
	if err != nil {
		h.sendResponse(client, WSResponse{
			Type:      msg.Type + ".response",
			RequestID: msg.RequestID,
			Success:   false,
			Error:     err.Error(),
		})
		return
	}

	h.sendResponse(client, WSResponse{
		Type:      msg.Type + ".response",
		RequestID: msg.RequestID,
		Success:   true,
		Payload:   result.Payload,
	})
}

// parseCommand converts a WebSocket message to a command
func (h *Handler) parseCommand(userID string, msg WSMessage) (commands.Command, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}

	switch msg.Type {
	case "message.typing":
		var p struct {
			ConversationID string `json:"conversation_id"`
			IsTyping       bool   `json:"is_typing"`
		}
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return nil, err
		}
		convID, _ := uuid.Parse(p.ConversationID)
		return commands.TypingCommand{
			ConversationID: convID,
			UserID:         userUUID,
			IsTyping:       p.IsTyping,
		}, nil

	case "message.read":
		var p struct {
			MessageID      string `json:"message_id"`
			ConversationID string `json:"conversation_id"`
		}
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return nil, err
		}
		msgID, _ := uuid.Parse(p.MessageID)
		convID, _ := uuid.Parse(p.ConversationID)
		return commands.MarkMessageReadCommand{
			MessageID:      msgID,
			UserID:         userUUID,
			ConversationID: convID,
		}, nil

	case "call.offer":
		var p struct {
			CallID string `json:"call_id"`
			ToID   string `json:"to_id"`
			SDP    string `json:"sdp"`
		}
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return nil, err
		}
		callID, _ := uuid.Parse(p.CallID)
		toID, _ := uuid.Parse(p.ToID)
		return commands.SendOfferCommand{
			CallID: callID,
			FromID: userUUID,
			ToID:   toID,
			SDP:    p.SDP,
		}, nil

	case "call.answer":
		var p struct {
			CallID string `json:"call_id"`
			ToID   string `json:"to_id"`
			SDP    string `json:"sdp"`
		}
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return nil, err
		}
		callID, _ := uuid.Parse(p.CallID)
		toID, _ := uuid.Parse(p.ToID)
		return commands.SendAnswerCommand{
			CallID: callID,
			FromID: userUUID,
			ToID:   toID,
			SDP:    p.SDP,
		}, nil

	case "call.ice":
		var p struct {
			CallID        string `json:"call_id"`
			ToID          string `json:"to_id"`
			Candidate     string `json:"candidate"`
			SDPMid        string `json:"sdp_mid"`
			SDPMLineIndex int    `json:"sdp_mline_index"`
		}
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return nil, err
		}
		callID, _ := uuid.Parse(p.CallID)
		toID, _ := uuid.Parse(p.ToID)
		return commands.SendICECandidateCommand{
			CallID:        callID,
			FromID:        userUUID,
			ToID:          toID,
			Candidate:     p.Candidate,
			SDPMid:        p.SDPMid,
			SDPMLineIndex: p.SDPMLineIndex,
		}, nil

	case "presence.update":
		var p struct {
			IsOnline bool `json:"is_online"`
		}
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return nil, err
		}
		return commands.UpdatePresenceCommand{
			UserID:   userUUID,
			IsOnline: p.IsOnline,
		}, nil

	default:
		// For other commands, use SimpleCommand with raw payload
		return commands.SimpleCommand{Type: msg.Type, Payload: []byte(msg.Payload)}, nil
	}
}

// canSubscribe validates if a user can subscribe to a channel
func (h *Handler) canSubscribe(ctx context.Context, userID, channel string) (bool, error) {
	// If no authorizer is configured, allow subscription to own channels only
	if h.authorizer == nil {
		if strings.HasSuffix(channel, userID) {
			return true, nil
		}
		return false, nil
	}

	// Use the authorizer for proper channel access validation
	return h.authorizer.CanSubscribe(ctx, userID, channel)
}

// sendResponse sends a response to the client
func (h *Handler) sendResponse(client *Client, resp WSResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	client.SendMessage(data)
}
