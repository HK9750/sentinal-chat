package server

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"sentinal-chat/internal/events"
	"sentinal-chat/internal/services"
)

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	clients             map[uuid.UUID]map[string]*Client
	register            chan *Client
	unregister          chan *Client
	broadcast           chan *BroadcastMessage
	eventBus            events.EventBus
	conversationService *services.ConversationService
	messageService      *services.MessageService
	userService         *services.UserService
	rateLimiter         *WebSocketRateLimiter
	logger              *WebSocketLogger
	mu                  sync.RWMutex
	stopChan            chan struct{}
	wg                  sync.WaitGroup
	isRunning           int32
}

// BroadcastMessage represents a message to broadcast
type BroadcastMessage struct {
	UserIDs        []uuid.UUID
	ConversationID *uuid.UUID
	Event          events.Event
	Payload        []byte
}

// WebSocketRateLimiter tracks connections per user/IP
type WebSocketRateLimiter struct {
	connectionsPerUser map[uuid.UUID][]time.Time
	connectionsPerIP   map[string][]time.Time
	mu                 sync.RWMutex
}

func NewWebSocketRateLimiter() *WebSocketRateLimiter {
	wrl := &WebSocketRateLimiter{
		connectionsPerUser: make(map[uuid.UUID][]time.Time),
		connectionsPerIP:   make(map[string][]time.Time),
	}
	go wrl.cleanupLoop()
	return wrl
}

func (w *WebSocketRateLimiter) AllowConnection(userID uuid.UUID) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-1 * time.Minute)

	validConnections := []time.Time{}
	for _, t := range w.connectionsPerUser[userID] {
		if t.After(windowStart) {
			validConnections = append(validConnections, t)
		}
	}

	if len(validConnections) >= 10 {
		return false
	}

	w.connectionsPerUser[userID] = append(validConnections, now)
	return true
}

func (w *WebSocketRateLimiter) RecordConnection(userID uuid.UUID) {
	// Already recorded in AllowConnection
}

func (w *WebSocketRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		w.cleanup()
	}
}

func (w *WebSocketRateLimiter) cleanup() {
	w.mu.Lock()
	defer w.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)

	for userID, times := range w.connectionsPerUser {
		valid := []time.Time{}
		for _, t := range times {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(w.connectionsPerUser, userID)
		} else {
			w.connectionsPerUser[userID] = valid
		}
	}
}

// NewHub creates a new Hub
func NewHub(
	eventBus events.EventBus,
	conversationService *services.ConversationService,
	messageService *services.MessageService,
	userService *services.UserService,
) *Hub {
	return &Hub{
		clients:             make(map[uuid.UUID]map[string]*Client),
		register:            make(chan *Client, 256),
		unregister:          make(chan *Client, 256),
		broadcast:           make(chan *BroadcastMessage, 256),
		eventBus:            eventBus,
		conversationService: conversationService,
		messageService:      messageService,
		userService:         userService,
		rateLimiter:         NewWebSocketRateLimiter(),
		logger:              NewWebSocketLogger(),
		stopChan:            make(chan struct{}),
	}
}

// Run starts the Hub
func (h *Hub) Run() {
	atomic.StoreInt32(&h.isRunning, 1)
	defer atomic.StoreInt32(&h.isRunning, 0)

	h.wg.Add(1)
	go h.subscribeToEvents()

	for {
		select {
		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case msg := <-h.broadcast:
			h.handleBroadcast(msg)

		case <-h.stopChan:
			h.wg.Wait()
			return
		}
	}
}

func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.rateLimiter.AllowConnection(client.userID) {
		h.logger.Warn("connection rate limit exceeded", client.userID, client.clientID)
		client.conn.Close()
		return
	}

	if h.clients[client.userID] == nil {
		h.clients[client.userID] = make(map[string]*Client)
	}

	const maxConnectionsPerUser = 10
	if len(h.clients[client.userID]) >= maxConnectionsPerUser {
		h.logger.Warn("max connections per user reached", client.userID, client.clientID)
		for id, c := range h.clients[client.userID] {
			h.removeClient(c)
			delete(h.clients[client.userID], id)
			break
		}
	}

	h.clients[client.userID][client.clientID] = client
	h.rateLimiter.RecordConnection(client.userID)

	if h.conversationService != nil {
		conversations, _, err := h.conversationService.GetUserConversations(context.Background(), client.userID, 1, 1000)
		if err == nil {
			for _, conv := range conversations {
				client.conversations[conv.ID] = true
			}
		}
	}

	if h.userService != nil {
		h.userService.UpdateOnlineStatus(context.Background(), client.userID, client.userID, true)
	}

	h.logger.Info("client connected", client.userID, client.clientID)

	go client.writePump()
	go client.readPump()
}

func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if userClients, ok := h.clients[client.userID]; ok {
		if _, ok := userClients[client.clientID]; ok {
			delete(userClients, client.clientID)
			h.removeClient(client)

			if len(userClients) == 0 {
				delete(h.clients, client.userID)
				if h.userService != nil {
					h.userService.UpdateOnlineStatus(context.Background(), client.userID, client.userID, false)
				}
			}

			h.logger.Info("client disconnected", client.userID, client.clientID)
		}
	}
}

func (h *Hub) removeClient(client *Client) {
	close(client.send)
	client.conn.Close()
}

func (h *Hub) handleBroadcast(msg *BroadcastMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, _ := json.Marshal(msg.Event)

	if msg.ConversationID != nil {
		h.broadcastToConversation(*msg.ConversationID, data)
	} else if len(msg.UserIDs) > 0 {
		for _, userID := range msg.UserIDs {
			h.broadcastToUser(userID, data)
		}
	}
}

func (h *Hub) broadcastToUser(userID uuid.UUID, data []byte) {
	if userClients, ok := h.clients[userID]; ok {
		for _, client := range userClients {
			select {
			case client.send <- data:
			default:
				h.logger.Warn("client send buffer full", client.userID, client.clientID)
			}
		}
	}
}

func (h *Hub) broadcastToConversation(convID uuid.UUID, data []byte) {
	for _, userClients := range h.clients {
		for _, client := range userClients {
			if client.conversations[convID] {
				select {
				case client.send <- data:
				default:
					h.logger.Warn("client send buffer full", client.userID, client.clientID)
				}
			}
		}
	}
}

func (h *Hub) subscribeToEvents() {
	defer h.wg.Done()

	eventTypes := []events.EventType{
		events.EventMessageNew,
		events.EventMessageRead,
		events.EventTypingStarted,
		events.EventTypingStopped,
		events.EventCallOffer,
		events.EventCallAnswer,
		events.EventCallICE,
		events.EventCallEnded,
	}

	for _, eventType := range eventTypes {
		h.eventBus.Subscribe(eventType, &WebSocketEventHandler{hub: h})
	}
}

// Stop gracefully shuts down the Hub
func (h *Hub) Stop() {
	close(h.stopChan)
	h.wg.Wait()

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, userClients := range h.clients {
		for _, client := range userClients {
			h.removeClient(client)
		}
	}
	h.clients = make(map[uuid.UUID]map[string]*Client)
}

// WebSocketEventHandler implements events.EventHandler
type WebSocketEventHandler struct {
	hub *Hub
}

func (h *WebSocketEventHandler) Handle(ctx context.Context, event events.Event) error {
	h.hub.broadcast <- &BroadcastMessage{
		Event: event,
	}
	return nil
}
