package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	Conn *websocket.Conn
	Key  string
	Send chan []byte
	mu   sync.Mutex
}

func NewClient(conn *websocket.Conn, key string) *Client {
	return &Client{
		Conn: conn,
		Key:  key,
		Send: make(chan []byte, 256),
	}
}

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

func (c *Client) close() {
	c.mu.Lock()
	_ = c.Conn.Close()
	c.mu.Unlock()
}

func (c *Client) SendMessage(msg []byte) {
	select {
	case c.Send <- msg:
	default:
	}
}
