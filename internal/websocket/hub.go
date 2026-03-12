package websocket

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	gorillawebsocket "github.com/gorilla/websocket"

	"sentinal-chat/internal/events"
	redisclient "sentinal-chat/internal/redis"
	"sentinal-chat/internal/repository"
	"sentinal-chat/pkg/logger"
)

type Broadcaster interface {
	BroadcastConversation(ctx context.Context, conversationID uuid.UUID, envelope EventEnvelope, excludeUser *uuid.UUID) error
	SendToUser(userID uuid.UUID, envelope EventEnvelope)
}

type Hub struct {
	logger        *logger.Logger
	redis         *redisclient.Client
	conversations repository.ConversationRepository
	mu            sync.RWMutex
	clients       map[string]*Client
	byUser        map[string]map[string]*Client
}

type Client struct {
	ID         string
	UserID     uuid.UUID
	SessionID  string
	DeviceID   string
	Conn       *gorillawebsocket.Conn
	Send       chan []byte
	Hub        *Hub
	CancelFunc context.CancelFunc
}

func NewHub(redis *redisclient.Client, conversations repository.ConversationRepository, log *logger.Logger) *Hub {
	return &Hub{
		logger:        log,
		redis:         redis,
		conversations: conversations,
		clients:       map[string]*Client{},
		byUser:        map[string]map[string]*Client{},
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client.ID] = client
	userKey := client.UserID.String()
	if h.byUser[userKey] == nil {
		h.byUser[userKey] = map[string]*Client{}
	}
	h.byUser[userKey][client.ID] = client
}

func (h *Hub) Unregister(client *Client) {
	if client == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, client.ID)
	userKey := client.UserID.String()
	if group := h.byUser[userKey]; group != nil {
		delete(group, client.ID)
		if len(group) == 0 {
			delete(h.byUser, userKey)
		}
	}
	close(client.Send)
}

func (h *Hub) SendToUser(userID uuid.UUID, envelope EventEnvelope) {
	body, err := json.Marshal(envelope)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.byUser[userID.String()] {
		select {
		case client.Send <- body:
		default:
		}
	}
}

func (h *Hub) BroadcastConversation(ctx context.Context, conversationID uuid.UUID, envelope EventEnvelope, excludeUser *uuid.UUID) error {
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	participants, err := h.conversations.GetParticipants(ctx, conversationID)
	if err != nil {
		return err
	}
	for _, participant := range participants {
		if excludeUser != nil && participant.UserID == *excludeUser {
			continue
		}
		h.SendToUser(participant.UserID, envelope)
	}
	if h.redis != nil {
		return h.redis.Publish(ctx, events.ConversationChannel(conversationID.String()), body)
	}
	return nil
}

func (h *Hub) ListenConversation(ctx context.Context, conversationID uuid.UUID) {
	if h.redis == nil {
		return
	}
	pubsub := h.redis.Subscribe(ctx, events.ConversationChannel(conversationID.String()))
	if pubsub == nil {
		return
	}
	go func() {
		defer pubsub.Close()
		for {
			msg, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				return
			}
			var envelope EventEnvelope
			if err := json.Unmarshal([]byte(msg.Payload), &envelope); err != nil {
				continue
			}
			conversationIDStr := strings.TrimSpace(envelope.ConversationID)
			var parsedConversationID uuid.UUID
			if conversationIDStr != "" {
				parsed, err := uuid.Parse(conversationIDStr)
				if err != nil {
					continue
				}
				parsedConversationID = parsed
			}
			h.mu.RLock()
			for _, client := range h.clients {
				if parsedConversationID != uuid.Nil {
					participant, err := h.conversations.IsParticipant(ctx, parsedConversationID, client.UserID)
					if err != nil || !participant {
						continue
					}
				}
				select {
				case client.Send <- []byte(msg.Payload):
				default:
				}
			}
			h.mu.RUnlock()
		}
	}()
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				_ = c.Conn.WriteMessage(gorillawebsocket.CloseMessage, []byte{})
				return
			}
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(gorillawebsocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(gorillawebsocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) Close() {
	if c.CancelFunc != nil {
		c.CancelFunc()
	}
	if c.Hub != nil {
		c.Hub.Unregister(c)
	}
}
