package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Client represents a WebSocket client connection
type Client struct {
	ID       string          // Unique client ID
	UserID   string          // Authenticated user ID
	Conn     *websocket.Conn // WebSocket connection
	Send     chan []byte     // Outbound message channel
	channels map[string]bool // Subscribed channels
	mu       sync.RWMutex    // Protects channels map and conn writes
}

// NewClient creates a new WebSocket client
func NewClient(conn *websocket.Conn, userID string) *Client {
	return &Client{
		ID:       uuid.New().String(),
		UserID:   userID,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		channels: make(map[string]bool),
	}
}

// Subscribe adds a channel to the client's subscriptions (internal use only)
func (c *Client) Subscribe(channel string) {
	c.mu.Lock()
	c.channels[channel] = true
	c.mu.Unlock()
}

// Unsubscribe removes a channel from the client's subscriptions (internal use only)
func (c *Client) Unsubscribe(channel string) {
	c.mu.Lock()
	delete(c.channels, channel)
	c.mu.Unlock()
}

// IsSubscribed checks if client is subscribed to a channel
func (c *Client) IsSubscribed(channel string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.channels[channel]
}

// GetChannels returns a copy of all subscribed channels
func (c *Client) GetChannels() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	channels := make([]string, 0, len(c.channels))
	for ch := range c.channels {
		channels = append(channels, ch)
	}
	return channels
}

// WriteLoop handles outbound messages from the Send channel
func (c *Client) WriteLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.close()
			return
		case msg, ok := <-c.Send:
			if !ok {
				c.close()
				return
			}
			c.mu.Lock()
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			_ = c.Conn.WriteMessage(websocket.TextMessage, msg)
			c.mu.Unlock()
		case <-ticker.C:
			c.mu.Lock()
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			_ = c.Conn.WriteMessage(websocket.PingMessage, []byte("ping"))
			c.mu.Unlock()
		}
	}
}

// close closes the WebSocket connection
func (c *Client) close() {
	c.mu.Lock()
	_ = c.Conn.Close()
	c.mu.Unlock()
}

// SendMessage sends a message to the client's Send channel (non-blocking)
func (c *Client) SendMessage(msg []byte) {
	select {
	case c.Send <- msg:
	default:
		// Channel full, message dropped
	}
}
