package server

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

// Rate limits per minute
type RateLimits struct {
	MaxTypingEvents    int
	MaxReadReceipts    int
	MaxPresenceUpdates int
	MaxCallSignals     int
	MaxPingMessages    int
}

var DefaultRateLimits = RateLimits{
	MaxTypingEvents:    60,
	MaxReadReceipts:    120,
	MaxPresenceUpdates: 30,
	MaxCallSignals:     120,
	MaxPingMessages:    60,
}

// ClientRateLimiter tracks rate limits per client
type ClientRateLimiter struct {
	typingTokens      int
	readReceiptTokens int
	presenceTokens    int
	callTokens        int
	pingTokens        int
	lastRefill        time.Time
	mu                sync.Mutex
}

func NewClientRateLimiter() *ClientRateLimiter {
	now := time.Now()
	return &ClientRateLimiter{
		typingTokens:      DefaultRateLimits.MaxTypingEvents,
		readReceiptTokens: DefaultRateLimits.MaxReadReceipts,
		presenceTokens:    DefaultRateLimits.MaxPresenceUpdates,
		callTokens:        DefaultRateLimits.MaxCallSignals,
		pingTokens:        DefaultRateLimits.MaxPingMessages,
		lastRefill:        now,
	}
}

func (rl *ClientRateLimiter) Allow(msgType string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)

	if elapsed >= time.Minute {
		rl.refillTokens()
		rl.lastRefill = now
	}

	switch msgType {
	case "typing:start", "typing:stop":
		if rl.typingTokens > 0 {
			rl.typingTokens--
			return true
		}
	case "read":
		if rl.readReceiptTokens > 0 {
			rl.readReceiptTokens--
			return true
		}
	case "presence":
		if rl.presenceTokens > 0 {
			rl.presenceTokens--
			return true
		}
	case "call":
		if rl.callTokens > 0 {
			rl.callTokens--
			return true
		}
	case "ping":
		if rl.pingTokens > 0 {
			rl.pingTokens--
			return true
		}
	}
	return false
}

func (rl *ClientRateLimiter) refillTokens() {
	rl.typingTokens = DefaultRateLimits.MaxTypingEvents
	rl.readReceiptTokens = DefaultRateLimits.MaxReadReceipts
	rl.presenceTokens = DefaultRateLimits.MaxPresenceUpdates
	rl.callTokens = DefaultRateLimits.MaxCallSignals
	rl.pingTokens = DefaultRateLimits.MaxPingMessages
}

// Client represents a single WebSocket connection
type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	send          chan []byte
	userID        uuid.UUID
	clientID      string
	deviceID      uuid.UUID
	conversations map[uuid.UUID]bool
	rateLimiter   *ClientRateLimiter
	isClosing     int32
	connectedAt   time.Time
	lastActivity  time.Time
	logger        WebSocketLogger
}

// ClientMessage represents a message from the client
type ClientMessage struct {
	Type           string    `json:"type"`
	ConversationID uuid.UUID `json:"conversation_id,omitempty"`
	MessageID      uuid.UUID `json:"message_id,omitempty"`
}

func NewClient(hub *Hub, conn *websocket.Conn, userID uuid.UUID, deviceID uuid.UUID, clientID string, logger WebSocketLogger) *Client {
	now := time.Now()
	return &Client{
		hub:           hub,
		conn:          conn,
		send:          make(chan []byte, 256),
		userID:        userID,
		deviceID:      deviceID,
		clientID:      clientID,
		conversations: make(map[uuid.UUID]bool),
		rateLimiter:   NewClientRateLimiter(),
		connectedAt:   now,
		lastActivity:  now,
		logger:        logger,
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		c.lastActivity = time.Now()
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("websocket unexpected close", c.userID, c.clientID, err)
			}
			break
		}

		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		c.lastActivity = time.Now()

		if err := c.handleMessage(message); err != nil {
			c.logger.Error("websocket handle message failed", c.userID, c.clientID, err)
		}
	}
}

func (c *Client) handleMessage(message []byte) error {
	var msg ClientMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return err
	}

	if !c.rateLimiter.Allow(msg.Type) {
		c.logger.Warn("rate limit exceeded", c.userID, c.clientID, zap.String("msg_type", msg.Type))
		return nil
	}

	switch msg.Type {
	case "typing:start":
		return c.handleTypingStart(msg)
	case "typing:stop":
		return c.handleTypingStop(msg)
	case "read":
		return c.handleReadReceipt(msg)
	case "ping":
		return c.handlePing()
	default:
		c.logger.Warn("unknown message type", c.userID, c.clientID, zap.String("msg_type", msg.Type))
		return nil
	}
}

func (c *Client) handleTypingStart(msg ClientMessage) error {
	if c.hub.conversationService == nil {
		return nil
	}
	return c.hub.conversationService.StartTyping(
		context.Background(),
		msg.ConversationID,
		c.userID,
		"",
	)
}

func (c *Client) handleTypingStop(msg ClientMessage) error {
	if c.hub.conversationService == nil {
		return nil
	}
	return c.hub.conversationService.StopTyping(
		context.Background(),
		msg.ConversationID,
		c.userID,
		"",
	)
}

func (c *Client) handleReadReceipt(msg ClientMessage) error {
	if c.hub.messageService == nil {
		return nil
	}
	return c.hub.messageService.MarkAsRead(
		context.Background(),
		msg.MessageID,
		c.userID,
	)
}

func (c *Client) handlePing() error {
	c.send <- []byte(`{"type":"pong"}`)
	return nil
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

			if time.Since(c.lastActivity) > pongWait*2 {
				c.logger.Info("client idle timeout", c.userID, c.clientID)
				return
			}
		}
	}
}
