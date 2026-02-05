package websocket

import (
	"context"
	"sync"
)

type Hub struct {
	mu       sync.RWMutex
	clients  map[string]map[*Client]struct{}
	register chan *Client
	remove   chan *Client
}

func NewHub() *Hub {
	return &Hub{
		clients:  make(map[string]map[*Client]struct{}),
		register: make(chan *Client, 256),
		remove:   make(chan *Client, 256),
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.add(client)
		case client := <-h.remove:
			h.removeClient(client)
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.remove <- client
}

func (h *Hub) Broadcast(key string, payload []byte) {
	h.mu.RLock()
	clients := h.clients[key]
	for c := range clients {
		c.SendMessage(payload)
	}
	h.mu.RUnlock()
}

func (h *Hub) add(client *Client) {
	h.mu.Lock()
	if _, ok := h.clients[client.Key]; !ok {
		h.clients[client.Key] = make(map[*Client]struct{})
	}
	h.clients[client.Key][client] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	if set, ok := h.clients[client.Key]; ok {
		delete(set, client)
		if len(set) == 0 {
			delete(h.clients, client.Key)
		}
	}
	h.mu.Unlock()
}
