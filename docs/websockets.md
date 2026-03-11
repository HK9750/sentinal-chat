# WebSockets — Full Implementation Guide

This document is the complete WebSocket contract and implementation guide for Sentinal Chat.
It contains the full Go code for the hub, connection lifecycle, inbound event handlers,
outbound event fan-out via Redis Pub/Sub, and the WebSocket route registration.

Current repo reality:
- No WebSocket code exists anywhere in the project yet.
- The Redis client and Pub/Sub publisher are defined in `docs/redis-outbox.md`.
- The event type constants live in `internal/events/types.go` (to be created).
- Auth middleware (`internal/middleware/auth_middleware.go`) is fully implemented.
- This document is the build target.

---

## Table of Contents

1. [Endpoint and Authentication](#1-endpoint-and-authentication)
2. [Event Envelope](#2-event-envelope)
3. [Inbound Client Events](#3-inbound-client-events)
4. [Outbound Server Events](#4-outbound-server-events)
5. [Redis Channel Mapping](#5-redis-channel-mapping)
6. [Go Implementation: Hub](#6-go-implementation-hub)
7. [Go Implementation: Client (Connection)](#7-go-implementation-client-connection)
8. [Go Implementation: WS Handler (Gin Route)](#8-go-implementation-ws-handler-gin-route)
9. [Go Implementation: Inbound Frame Router](#9-go-implementation-inbound-frame-router)
10. [Wiring into cmd/api/main.go](#10-wiring-into-cmdapimainago)
11. [Delivery and Validation Rules](#11-delivery-and-validation-rules)
12. [Error Frames](#12-error-frames)

---

## 1. Endpoint and Authentication

### Endpoint

```
GET /v1/ws
```

**Status:** Not yet registered. Register it in `cmd/api/main.go` on a public group
(authentication is handled manually inside the handler, not via middleware, because
WebSocket upgrades cannot return HTTP error bodies easily).

### Auth methods (both must work)

```
Authorization: Bearer <access_token>     (header)
?token=<access_token>                    (query param — used by browser clients)
```

### Rejection behavior

- If no token is provided: close with `1008 Policy Violation`.
- If token is invalid or expired: close with `1008 Policy Violation`.
- If session is revoked: close with `1008 Policy Violation`.

---

## 2. Event Envelope

Every inbound and outbound WebSocket frame uses this JSON envelope.

```go
// internal/server/ws_types.go

package server

import (
    "encoding/json"
    "time"
)

// InboundFrame is the structure clients send to the server.
type InboundFrame struct {
    Type           string          `json:"type"`
    RequestID      string          `json:"request_id,omitempty"`
    ConversationID string          `json:"conversation_id,omitempty"`
    CallID         string          `json:"call_id,omitempty"`
    Data           json.RawMessage `json:"data,omitempty"`
}

// OutboundFrame is the structure the server sends to clients.
type OutboundFrame struct {
    Type           string      `json:"type"`
    RequestID      string      `json:"request_id,omitempty"`
    ConversationID string      `json:"conversation_id,omitempty"`
    CallID         string      `json:"call_id,omitempty"`
    Data           interface{} `json:"data,omitempty"`
    SentAt         time.Time   `json:"sent_at"`
}

// ErrorData is the payload inside an error frame.
type ErrorData struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

### Field descriptions

| Field | Required | Description |
|---|---|---|
| `type` | yes | Event name, e.g. `message:new`, `typing:start`, `ping` |
| `request_id` | no | Client-generated correlation ID echoed back in responses |
| `conversation_id` | context-dependent | Required for conversation-scoped events |
| `call_id` | context-dependent | Required for call-scoped events |
| `data` | context-dependent | Event-specific payload |
| `sent_at` | outbound only | Server timestamp on all outbound frames |

---

## 3. Inbound Client Events

These are frames the **client sends to the server**.

### `ping`

Keep-alive and latency measurement.

```json
{
  "type": "ping",
  "data": { "client_time": "2025-01-01T10:00:00Z" }
}
```

Server response:
```json
{
  "type": "pong",
  "sent_at": "2025-01-01T10:00:00.012Z",
  "data": {
    "client_time": "2025-01-01T10:00:00Z",
    "server_time": "2025-01-01T10:00:00.012Z"
  }
}
```

Server actions:
- Reply immediately with `pong`.
- Renew the user's presence TTL in Redis (`PresenceStore.Renew`).

---

### `typing:start`

User has started composing in a conversation.

```json
{
  "type": "typing:start",
  "conversation_id": "<uuid>",
  "data": {}
}
```

Server actions:
1. Validate `conversation_id` is present.
2. Verify caller is a participant (`ConversationRepository.IsParticipant`).
3. Set ephemeral typing key in Redis (`TypingStore.SetTyping`, TTL 8s).
4. Publish `typing:started` to `channel:conversation:<id>` — **exclude the sender**.
5. Do NOT write to DB. Do NOT write an outbox event.

---

### `typing:stop`

User has stopped composing.

```json
{
  "type": "typing:stop",
  "conversation_id": "<uuid>",
  "data": {}
}
```

Server actions:
1. Validate `conversation_id`.
2. Clear typing key in Redis (`TypingStore.ClearTyping`).
3. Publish `typing:stopped` to `channel:conversation:<id>` — exclude sender.

---

### `read`

Mark messages as read up to a sequence number.

```json
{
  "type": "read",
  "conversation_id": "<uuid>",
  "data": {
    "message_ids": ["<uuid>", "<uuid>"],
    "up_to_seq_id": 144
  }
}
```

Server actions:
1. Validate `conversation_id`.
2. Verify participant.
3. For each `message_id`: call `MessageRepository.MarkAsRead(ctx, msgID, userID)`.
4. If `up_to_seq_id` is set: call `ConversationRepository.UpdateLastReadSequence`.
5. Write `outbox_events` row with `event_type = "message:read"` — let the outbox worker
   publish the `message:read` event to the conversation channel.

Note: Writing via the outbox here is acceptable because read receipts are not
latency-critical and benefit from the at-least-once delivery guarantee.
However, for very low latency you may publish directly to Redis after the DB write —
but still write the outbox row for durability.

---

### `delivered`

Acknowledge message delivery (auto-sent on connect for messages received while offline).

```json
{
  "type": "delivered",
  "conversation_id": "<uuid>",
  "data": {
    "message_ids": ["<uuid>", "<uuid>"]
  }
}
```

Server actions:
1. Validate `conversation_id`.
2. Call `MessageRepository.BulkMarkAsDelivered(ctx, messageIDs, userID)`.
3. Write outbox row with `event_type = "message:delivered"`.

---

## 4. Outbound Server Events

These are frames the **server sends to clients**.

### `connection:ready`

Sent immediately after a successful WebSocket handshake and authentication.

```json
{
  "type": "connection:ready",
  "sent_at": "2025-01-01T10:00:00Z",
  "data": {
    "user_id": "<uuid>",
    "session_id": "<uuid>",
    "device_id": "device-fingerprint",
    "server_time": "2025-01-01T10:00:00Z"
  }
}
```

---

### `message:new`

A new message was committed. Sent to all participants except the sender.

```json
{
  "type": "message:new",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:00Z",
  "data": {
    "message_id": "<uuid>",
    "seq_id": 145,
    "sender_id": "<uuid>",
    "type": "TEXT",
    "client_message_id": "client-key",
    "is_forwarded": false,
    "reply_to_msg_id": null,
    "mention_count": 0,
    "created_at": "2025-01-01T10:00:00Z"
  }
}
```

---

### `message:updated`

A message was edited.

```json
{
  "type": "message:updated",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:05Z",
  "data": {
    "message_id": "<uuid>",
    "edited_by": "<uuid>",
    "edited_at": "2025-01-01T10:00:05Z"
  }
}
```

---

### `message:deleted`

A message was soft-deleted.

```json
{
  "type": "message:deleted",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:10Z",
  "data": {
    "message_id": "<uuid>",
    "deleted_by": "<uuid>",
    "deleted_at": "2025-01-01T10:00:10Z"
  }
}
```

---

### `message:delivered`

A recipient device has confirmed delivery.

```json
{
  "type": "message:delivered",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:02Z",
  "data": {
    "message_id": "<uuid>",
    "user_id": "<recipient-uuid>",
    "delivered_at": "2025-01-01T10:00:02Z"
  }
}
```

---

### `message:read`

A participant has read up to a sequence number.

```json
{
  "type": "message:read",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:03Z",
  "data": {
    "message_id": "<uuid>",
    "user_id": "<reader-uuid>",
    "read_at": "2025-01-01T10:00:03Z",
    "last_read_sequence": 145
  }
}
```

---

### `typing:started`

A participant is composing. Sent to all other participants.

```json
{
  "type": "typing:started",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:00Z",
  "data": {
    "user_id": "<uuid>"
  }
}
```

---

### `typing:stopped`

A participant stopped composing (explicit stop or TTL expiry).

```json
{
  "type": "typing:stopped",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:08Z",
  "data": {
    "user_id": "<uuid>"
  }
}
```

---

### `reaction:added`

A reaction was added to a message.

```json
{
  "type": "reaction:added",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:15Z",
  "data": {
    "message_id": "<uuid>",
    "user_id": "<uuid>",
    "reaction_code": ":thumbsup:"
  }
}
```

---

### `reaction:removed`

A reaction was removed.

```json
{
  "type": "reaction:removed",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:20Z",
  "data": {
    "message_id": "<uuid>",
    "user_id": "<uuid>",
    "reaction_code": ":thumbsup:"
  }
}
```

---

### `message:pinned`

A message was pinned.

```json
{
  "type": "message:pinned",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:25Z",
  "data": {
    "message_id": "<uuid>",
    "pinned_by": "<uuid>",
    "pinned_at": "2025-01-01T10:00:25Z"
  }
}
```

---

### `message:unpinned`

```json
{
  "type": "message:unpinned",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:30Z",
  "data": {
    "message_id": "<uuid>",
    "unpinned_by": "<uuid>"
  }
}
```

---

### `conversation:cleared`

A participant cleared their chat history.

```json
{
  "type": "conversation:cleared",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:35Z",
  "data": {
    "user_id": "<uuid>",
    "cleared_at": "2025-01-01T10:00:35Z"
  }
}
```

---

### `presence:online`

```json
{
  "type": "presence:online",
  "sent_at": "2025-01-01T10:00:00Z",
  "data": {
    "user_id": "<uuid>"
  }
}
```

---

### `presence:offline`

```json
{
  "type": "presence:offline",
  "sent_at": "2025-01-01T10:00:00Z",
  "data": {
    "user_id": "<uuid>",
    "last_seen_at": "2025-01-01T10:00:00Z"
  }
}
```

---

### `call:offer`

WebRTC SDP offer routed to a specific user.

```json
{
  "type": "call:offer",
  "call_id": "<uuid>",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:00Z",
  "data": {
    "from_user_id": "<uuid>",
    "to_user_id": "<uuid>",
    "sdp": "v=0\r\n..."
  }
}
```

---

### `call:answer`

```json
{
  "type": "call:answer",
  "call_id": "<uuid>",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:05Z",
  "data": {
    "from_user_id": "<uuid>",
    "to_user_id": "<uuid>",
    "sdp": "v=0\r\n..."
  }
}
```

---

### `call:ice`

```json
{
  "type": "call:ice",
  "call_id": "<uuid>",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:06Z",
  "data": {
    "from_user_id": "<uuid>",
    "to_user_id": "<uuid>",
    "candidate": "candidate:0 1 UDP ...",
    "sdp_mid": "0",
    "sdp_mline_index": 0
  }
}
```

---

### `call:ringing`

```json
{
  "type": "call:ringing",
  "call_id": "<uuid>",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:01Z",
  "data": {
    "user_id": "<callee-uuid>"
  }
}
```

---

### `call:connected`

```json
{
  "type": "call:connected",
  "call_id": "<uuid>",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:10Z",
  "data": {
    "connected_at": "2025-01-01T10:00:10Z"
  }
}
```

---

### `call:ended`

```json
{
  "type": "call:ended",
  "call_id": "<uuid>",
  "conversation_id": "<uuid>",
  "sent_at": "2025-01-01T10:00:50Z",
  "data": {
    "ended_at": "2025-01-01T10:00:50Z",
    "reason": "COMPLETED",
    "duration_seconds": 40
  }
}
```

---

## 5. Redis Channel Mapping

| Channel pattern | Events published there |
|---|---|
| `channel:conversation:{id}` | `message:new`, `message:updated`, `message:deleted`, `message:delivered`, `message:read`, `message:played`, `typing:started`, `typing:stopped`, `reaction:added`, `reaction:removed`, `message:pinned`, `message:unpinned`, `conversation:cleared`, `conversation:updated`, `conversation:participant_added`, `conversation:participant_removed` |
| `channel:user:{id}` | `presence:online`, `presence:offline`, direct `call:offer`, `call:answer`, `call:ice` targeted to one user |
| `channel:call:{id}` | `call:created`, `call:ringing`, `call:connected`, `call:ended`, `call:participant_updated` |
| `channel:presence:{id}` | `presence:online`, `presence:offline` |

### Redis key taxonomy (ephemeral, not outbox-backed)

```
presence:user:{user_id}                      TTL 65s, renewed by ping
connections:user:{user_id}                   integer connection count
typing:conversation:{conv_id}:user:{user_id} TTL 8s, renewed by typing:start
```

---

## 6. Go Implementation: Hub

Create the file `internal/server/hub.go`.

```go
// internal/server/hub.go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"sentinal-chat/internal/events"
	redisStore "sentinal-chat/internal/redis"
	"sentinal-chat/pkg/logger"
)

// Hub manages all active WebSocket connections, routes inbound frames,
// and fans out Redis Pub/Sub events to the appropriate connections.
type Hub struct {
	// mu protects the connections and subscriptions maps.
	mu sync.RWMutex

	// connections maps user_id → set of active Client pointers.
	connections map[string]map[*Client]struct{}

	// register is the channel through which new clients register themselves.
	register chan *Client

	// unregister is the channel through which clients remove themselves.
	unregister chan *Client

	// broadcast sends an OutboundFrame to all connections of a specific user.
	broadcast chan userMessage

	publisher *redisStore.PubSubPublisher
	presence  *redisStore.PresenceStore
	typing    *redisStore.TypingStore
	logger    *logger.Logger
}

// userMessage targets a frame at a specific user.
type userMessage struct {
	userID string
	frame  OutboundFrame
}

// NewHub creates a Hub. Call Run() in a goroutine before accepting connections.
func NewHub(
	publisher *redisStore.PubSubPublisher,
	presence *redisStore.PresenceStore,
	typing *redisStore.TypingStore,
	l *logger.Logger,
) *Hub {
	return &Hub{
		connections: make(map[string]map[*Client]struct{}),
		register:    make(chan *Client, 256),
		unregister:  make(chan *Client, 256),
		broadcast:   make(chan userMessage, 1024),
		publisher:   publisher,
		presence:    presence,
		typing:      typing,
		logger:      l,
	}
}

// Run starts the hub event loop. Blocks until ctx is cancelled.
// Start in a goroutine: go hub.Run(ctx)
func (h *Hub) Run(ctx context.Context) {
	h.logger.Infof("ws hub started")
	defer h.logger.Infof("ws hub stopped")

	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown: close all connections.
			h.mu.Lock()
			for _, clients := range h.connections {
				for c := range clients {
					close(c.send)
				}
			}
			h.connections = make(map[string]map[*Client]struct{})
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.addClient(ctx, client)

		case client := <-h.unregister:
			h.removeClient(ctx, client)

		case msg := <-h.broadcast:
			h.deliverToUser(msg.userID, msg.frame)
		}
	}
}

// SendToUser queues an OutboundFrame for delivery to all connections of a user.
// Safe to call from any goroutine.
func (h *Hub) SendToUser(userID string, frame OutboundFrame) {
	select {
	case h.broadcast <- userMessage{userID: userID, frame: frame}:
	default:
		h.logger.Warnf("ws hub: broadcast channel full, dropping frame for user %s type %s", userID, frame.Type)
	}
}

// SendToConversation queues an OutboundFrame for all participants currently
// connected to this server instance. It is typically called by the Redis subscriber
// goroutine after receiving a published envelope.
//
// Pass senderID to exclude the sender (pass empty string to send to everyone).
func (h *Hub) SendToConversation(participantIDs []string, senderID string, frame OutboundFrame) {
	for _, uid := range participantIDs {
		if uid == senderID {
			continue
		}
		h.SendToUser(uid, frame)
	}
}

// addClient registers a client, marks the user online, and subscribes to their channels.
func (h *Hub) addClient(ctx context.Context, client *Client) {
	h.mu.Lock()
	if h.connections[client.userID] == nil {
		h.connections[client.userID] = make(map[*Client]struct{})
	}
	h.connections[client.userID][client] = struct{}{}
	isFirstConn := len(h.connections[client.userID]) == 1
	h.mu.Unlock()

	// Mark online on first connection.
	if isFirstConn {
		if err := h.presence.SetOnline(ctx, client.userID); err != nil {
			h.logger.Warnf("ws hub: set online %s: %v", client.userID, err)
		}
		// Publish presence:online to their contacts via Redis.
		// This is best-effort; the outbox is not used for ephemeral presence.
		h.publishPresence(ctx, client.userID, true)
	}

	h.logger.Infof("ws hub: client registered user=%s device=%s total_connections=%d",
		client.userID, client.deviceID, h.connectionCount(client.userID))

	// Send connection:ready frame directly to this client.
	client.sendFrame(OutboundFrame{
		Type:   events.ConnectionReady,
		SentAt: time.Now().UTC(),
		Data: map[string]interface{}{
			"user_id":     client.userID,
			"session_id":  client.sessionID,
			"device_id":   client.deviceID,
			"server_time": time.Now().UTC(),
		},
	})
}

// removeClient unregisters a client and marks the user offline if no connections remain.
func (h *Hub) removeClient(ctx context.Context, client *Client) {
	h.mu.Lock()
	if clients, ok := h.connections[client.userID]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.connections, client.userID)
		}
	}
	remaining := len(h.connections[client.userID])
	h.mu.Unlock()

	// Close the send channel so the write pump exits.
	close(client.send)

	if remaining == 0 {
		if err := h.presence.SetOffline(ctx, client.userID); err != nil {
			h.logger.Warnf("ws hub: set offline %s: %v", client.userID, err)
		}
		h.publishPresence(ctx, client.userID, false)
	}

	h.logger.Infof("ws hub: client unregistered user=%s remaining_connections=%d",
		client.userID, remaining)
}

// deliverToUser sends a frame to all active connections of a user.
func (h *Hub) deliverToUser(userID string, frame OutboundFrame) {
	h.mu.RLock()
	clients := h.connections[userID]
	h.mu.RUnlock()

	for c := range clients {
		select {
		case c.send <- frame:
		default:
			// Client send buffer full — consider it stale.
			h.logger.Warnf("ws hub: send buffer full for user %s, dropping", userID)
		}
	}
}

// connectionCount returns the number of active connections for a user (lock must not be held).
func (h *Hub) connectionCount(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections[userID])
}

// IsConnected returns true if the user has at least one active connection on this instance.
func (h *Hub) IsConnected(userID string) bool {
	return h.connectionCount(userID) > 0
}

// publishPresence publishes a presence:online or presence:offline event to Redis.
func (h *Hub) publishPresence(ctx context.Context, userID string, online bool) {
	evtType := events.PresenceOnline
	data := map[string]interface{}{"user_id": userID}
	if !online {
		evtType = events.PresenceOffline
		data["last_seen_at"] = time.Now().UTC()
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	channel := events.ChannelForPresence(userID)
	if err := h.publisher.Publish(ctx, channel, payload); err != nil {
		h.logger.Warnf("ws hub: publish presence %s: %v", userID, err)
	}
}

// SubscribeConversation starts a Redis Pub/Sub subscription for a conversation channel
// and fans received messages out to connected users.
// Call this in a goroutine when a conversation's first participant connects.
//
// participantsFn is called on each received message to get the current participant list
// to fan the event out to. It should hit the DB or a local cache.
func (h *Hub) SubscribeConversation(
	ctx context.Context,
	conversationID string,
	participantsFn func(ctx context.Context) ([]string, error),
) {
	channel := events.ChannelForConversation(conversationID)
	sub := h.publisher.Subscribe(ctx, channel)
	defer sub.Close()

	h.logger.Infof("ws hub: subscribed to %s", channel)

	redisCh := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-redisCh:
			if !ok {
				return
			}
			h.fanOutConversationMessage(ctx, msg, participantsFn)
		}
	}
}

// SubscribeUser starts a Redis Pub/Sub subscription for a user's personal channel
// and routes received messages to their connections.
func (h *Hub) SubscribeUser(ctx context.Context, userID string) {
	channel := events.ChannelForUser(userID)
	sub := h.publisher.Subscribe(ctx, channel)
	defer sub.Close()

	h.logger.Infof("ws hub: subscribed to %s", channel)

	redisCh := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-redisCh:
			if !ok {
				return
			}
			h.fanOutUserMessage(userID, msg)
		}
	}
}

// fanOutConversationMessage deserializes a Redis message and delivers it to participants.
func (h *Hub) fanOutConversationMessage(
	ctx context.Context,
	msg *goredis.Message,
	participantsFn func(ctx context.Context) ([]string, error),
) {
	var envelope redisStore.OutboxEnvelope
	if err := json.Unmarshal([]byte(msg.Payload), &envelope); err != nil {
		h.logger.Errorf("ws hub: unmarshal envelope: %v", err)
		return
	}

	frame := OutboundFrame{
		Type:           envelope.EventType,
		ConversationID: envelope.ConversationID,
		SentAt:         time.Now().UTC(),
		Data:           json.RawMessage(envelope.Payload),
	}

	participants, err := participantsFn(ctx)
	if err != nil {
		h.logger.Errorf("ws hub: get participants for %s: %v", envelope.ConversationID, err)
		return
	}

	// Exclude the actor (sender) — they already know about their own action.
	senderID := envelope.UserID
	for _, uid := range participants {
		if uid == senderID {
			continue
		}
		h.SendToUser(uid, frame)
	}
}

// fanOutUserMessage deserializes a Redis message and delivers it to the target user.
func (h *Hub) fanOutUserMessage(userID string, msg *goredis.Message) {
	var envelope redisStore.OutboxEnvelope
	if err := json.Unmarshal([]byte(msg.Payload), &envelope); err != nil {
		h.logger.Errorf("ws hub: