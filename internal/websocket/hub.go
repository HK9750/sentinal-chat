package websocket

import (
	"context"
	"sync"
)

// subscriptionRequest represents a channel subscription/unsubscription request
type subscriptionRequest struct {
	client    *Client
	channel   string
	subscribe bool // true = subscribe, false = unsubscribe
}

// Hub manages WebSocket client connections and channel subscriptions
type Hub struct {
	mu sync.RWMutex

	// clients maps client ID to client (for cleanup)
	clients map[string]*Client

	// channels maps channel name to set of clients subscribed to it
	channels map[string]map[*Client]struct{}

	// Control channels
	register     chan *Client             // New client connections
	unregister   chan *Client             // Client disconnections
	subscription chan subscriptionRequest // Subscribe/unsubscribe requests
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:      make(map[string]*Client),
		channels:     make(map[string]map[*Client]struct{}),
		register:     make(chan *Client, 256),
		unregister:   make(chan *Client, 256),
		subscription: make(chan subscriptionRequest, 512),
	}
}

// Run starts the hub's event loop
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.addClient(client)
		case client := <-h.unregister:
			h.removeClient(client)
		case req := <-h.subscription:
			if req.subscribe {
				h.subscribeToChannel(req.client, req.channel)
			} else {
				h.unsubscribeFromChannel(req.client, req.channel)
			}
		}
	}
}

// Register adds a new client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Subscribe subscribes a client to a channel
func (h *Hub) Subscribe(client *Client, channel string) {
	h.subscription <- subscriptionRequest{
		client:    client,
		channel:   channel,
		subscribe: true,
	}
}

// Unsubscribe unsubscribes a client from a channel
func (h *Hub) Unsubscribe(client *Client, channel string) {
	h.subscription <- subscriptionRequest{
		client:    client,
		channel:   channel,
		subscribe: false,
	}
}

// Broadcast sends a message to all clients subscribed to a channel
func (h *Hub) Broadcast(channel string, payload []byte) {
	h.mu.RLock()
	clients := h.channels[channel]
	for c := range clients {
		c.SendMessage(payload)
	}
	h.mu.RUnlock()
}

// BroadcastToUser sends a message to all connections for a specific user
func (h *Hub) BroadcastToUser(userID string, payload []byte) {
	h.mu.RLock()
	for _, client := range h.clients {
		if client.UserID == userID {
			client.SendMessage(payload)
		}
	}
	h.mu.RUnlock()
}

// GetClientCount returns the number of connected clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetChannelSubscriberCount returns the number of subscribers for a channel
func (h *Hub) GetChannelSubscriberCount(channel string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.channels[channel])
}

// addClient adds a new client to the hub (internal)
func (h *Hub) addClient(client *Client) {
	h.mu.Lock()
	h.clients[client.ID] = client
	h.mu.Unlock()
}

// removeClient removes a client and all its subscriptions (internal)
func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove client from all channels
	for channel := range client.channels {
		if subscribers, ok := h.channels[channel]; ok {
			delete(subscribers, client)
			if len(subscribers) == 0 {
				delete(h.channels, channel)
			}
		}
	}

	// Remove client from clients map
	delete(h.clients, client.ID)

	// Close send channel
	close(client.Send)
}

// subscribeToChannel subscribes a client to a channel (internal)
func (h *Hub) subscribeToChannel(client *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Create channel set if not exists
	if _, ok := h.channels[channel]; !ok {
		h.channels[channel] = make(map[*Client]struct{})
	}

	// Add client to channel
	h.channels[channel][client] = struct{}{}

	// Update client's channel list
	client.Subscribe(channel)
}

// unsubscribeFromChannel unsubscribes a client from a channel (internal)
func (h *Hub) unsubscribeFromChannel(client *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subscribers, ok := h.channels[channel]; ok {
		delete(subscribers, client)
		if len(subscribers) == 0 {
			delete(h.channels, channel)
		}
	}

	// Update client's channel list
	client.Unsubscribe(channel)
}
