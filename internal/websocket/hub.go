package websocket

import (
	"context"
	"encoding/json"
	"errors"
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
	SendToDevice(deviceID uuid.UUID, envelope EventEnvelope)
	PublishConversation(ctx context.Context, conversationID uuid.UUID, envelope EventEnvelope) error
	PublishToUser(ctx context.Context, userID uuid.UUID, envelope EventEnvelope) error
	PublishToDevice(ctx context.Context, deviceID uuid.UUID, envelope EventEnvelope) error
}

type Hub struct {
	logger        *logger.Logger
	redis         *redisclient.Client
	conversations repository.ConversationRepository
	mu            sync.RWMutex
	clients       map[string]*Client
	byUser        map[string]map[string]*Client
	byDevice      map[string]map[string]*Client
	listenOnce    sync.Once
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
		byDevice:      map[string]map[string]*Client{},
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
	if strings.TrimSpace(client.DeviceID) != "" {
		if h.byDevice[client.DeviceID] == nil {
			h.byDevice[client.DeviceID] = map[string]*Client{}
		}
		h.byDevice[client.DeviceID][client.ID] = client
	}
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
	if strings.TrimSpace(client.DeviceID) != "" {
		if group := h.byDevice[client.DeviceID]; group != nil {
			delete(group, client.ID)
			if len(group) == 0 {
				delete(h.byDevice, client.DeviceID)
			}
		}
	}
	close(client.Send)
}

func (h *Hub) SendToUser(userID uuid.UUID, envelope EventEnvelope) {
	envelope.UserID = userID.String()
	body, err := json.Marshal(envelope)
	if err != nil {
		h.logf("hub.send_to_user.marshal", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.byUser[userID.String()] {
		select {
		case client.Send <- body:
		default:
			h.logf("hub.send_to_user.queue_full", errors.New("client send buffer full"))
		}
	}
}

func (h *Hub) SendToDevice(deviceID uuid.UUID, envelope EventEnvelope) {
	envelope.DeviceID = deviceID.String()
	body, err := json.Marshal(envelope)
	if err != nil {
		h.logf("hub.send_to_device.marshal", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.byDevice[deviceID.String()] {
		select {
		case client.Send <- body:
		default:
			h.logf("hub.send_to_device.queue_full", errors.New("client send buffer full"))
		}
	}
}

func (h *Hub) PublishToDevice(ctx context.Context, deviceID uuid.UUID, envelope EventEnvelope) error {
	if h.redis == nil {
		return nil
	}
	envelope.DeviceID = deviceID.String()
	envelope.Source = localPublishSource
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return h.redis.Publish(ctx, events.DeviceChannel(deviceID.String()), body)
}

func (h *Hub) BroadcastConversation(ctx context.Context, conversationID uuid.UUID, envelope EventEnvelope, excludeUser *uuid.UUID) error {
	participants, err := h.conversations.GetParticipants(ctx, conversationID)
	if err != nil {
		return err
	}
	envelope.ConversationID = conversationID.String()
	for _, participant := range participants {
		if excludeUser != nil && participant.UserID == *excludeUser {
			continue
		}
		h.SendToUser(participant.UserID, envelope)
	}
	return nil
}

func (h *Hub) PublishConversation(ctx context.Context, conversationID uuid.UUID, envelope EventEnvelope) error {
	if h.redis == nil {
		return nil
	}
	envelope.ConversationID = conversationID.String()
	envelope.Source = localPublishSource
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return h.redis.Publish(ctx, events.ConversationChannel(conversationID.String()), body)
}

func (h *Hub) PublishToUser(ctx context.Context, userID uuid.UUID, envelope EventEnvelope) error {
	if h.redis == nil {
		return nil
	}
	envelope.UserID = userID.String()
	envelope.Source = localPublishSource
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return h.redis.Publish(ctx, events.UserChannel(userID.String()), body)
}

func (h *Hub) StartRedisListener(ctx context.Context) {
	if h.redis == nil {
		return
	}
	h.listenOnce.Do(func() {
		pubsub := h.redis.PSubscribe(ctx, "conversation:*", "call:*", "user:*", "device:*")
		if pubsub == nil {
			return
		}
		go func() {
			defer pubsub.Close()
			for {
				msg, err := pubsub.ReceiveMessage(ctx)
				if err != nil {
					if ctx.Err() == nil {
						h.logf("hub.redis_listener.receive", err)
					}
					return
				}
				h.dispatchRedisEnvelope(ctx, []byte(msg.Payload))
			}
		}()
	})
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

func (h *Hub) dispatchRedisEnvelope(ctx context.Context, payload []byte) {
	var envelope EventEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		h.logf("hub.redis_listener.unmarshal", err)
		return
	}
	conversationID := parseEnvelopeConversationID(envelope)
	callID := strings.TrimSpace(envelope.CallID)
	targetUserID := parseEnvelopeUserID(envelope)
	targetDeviceID := parseEnvelopeDeviceID(envelope)
	if strings.TrimSpace(envelope.Source) == localPublishSource {
		return
	}

	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for _, client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		if !h.shouldDeliverRedisEnvelope(ctx, client, conversationID, callID, targetUserID, targetDeviceID) {
			continue
		}
		select {
		case client.Send <- payload:
		default:
			h.logf("hub.redis_listener.queue_full", errors.New("client send buffer full"))
		}
	}
}

func (h *Hub) shouldDeliverRedisEnvelope(ctx context.Context, client *Client, conversationID uuid.UUID, callID string, targetUserID uuid.UUID, targetDeviceID uuid.UUID) bool {
	if targetDeviceID != uuid.Nil {
		return strings.TrimSpace(client.DeviceID) == targetDeviceID.String()
	}
	if targetUserID != uuid.Nil {
		return client.UserID == targetUserID
	}
	if conversationID != uuid.Nil {
		participant, err := h.conversations.IsParticipant(ctx, conversationID, client.UserID)
		if err != nil {
			h.logf("hub.redis_listener.participant_check", err)
			return false
		}
		return participant
	}
	if callID != "" {
		return true
	}
	return false
}

func parseEnvelopeConversationID(envelope EventEnvelope) uuid.UUID {
	conversationIDStr := strings.TrimSpace(envelope.ConversationID)
	if conversationIDStr == "" {
		return uuid.Nil
	}
	conversationID, err := uuid.Parse(conversationIDStr)
	if err != nil {
		return uuid.Nil
	}
	return conversationID
}

func parseEnvelopeUserID(envelope EventEnvelope) uuid.UUID {
	userIDStr := strings.TrimSpace(envelope.UserID)
	if userIDStr == "" {
		return uuid.Nil
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil
	}
	return userID
}

func parseEnvelopeDeviceID(envelope EventEnvelope) uuid.UUID {
	deviceIDStr := strings.TrimSpace(envelope.DeviceID)
	if deviceIDStr == "" {
		return uuid.Nil
	}
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		return uuid.Nil
	}
	return deviceID
}

func (h *Hub) logf(operation string, err error) {
	if h == nil || h.logger == nil || err == nil {
		return
	}
	h.logger.Errorf("%s: %v", operation, err)
}
