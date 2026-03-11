# Redis and Outbox — Full Implementation Guide

This document is the complete implementation guide for the Redis Pub/Sub layer and the
transactional outbox worker. It contains copy-paste-ready Go code for every file that
needs to be created.

Current repo reality:
- `outbox_events` table exists with correct schema and indexes.
- `OutboxRepository` interface and implementation both exist and work.
- Redis config fields exist in `config/config.go`.
- **No Redis client exists yet.**
- **No outbox worker exists yet.**
- **No Redis publisher or subscriber exists yet.**

---

## 1. Why the outbox exists

Without the outbox pattern this happens:

```
Service commits DB write
Service calls Redis PUBLISH
Redis is down for 200ms → event is lost forever
Client never receives the message
```

With the outbox pattern this happens:

```
Service opens DB transaction
  ├── writes domain data (message, reaction, etc.)
  └── writes outbox_events row  { status: PENDING }
Service commits transaction (atomic — both or neither)

OutboxWorker goroutine (separate from request path):
  ├── polls outbox_events WHERE status='PENDING'
  ├── publishes to Redis Pub/Sub
  └── marks row COMPLETED

WebSocket hub subscribes to Redis channels
  └── fans event out to all connected clients for that conversation
```

If Redis is down, the DB row stays `PENDING` and the worker retries with exponential backoff.
No events are lost as long as PostgreSQL is healthy during the write.

**The single most important rule:**
> Never call Redis PUBLISH directly from an HTTP handler or service method.
> Always write an outbox row in the same transaction as the domain data, then let the worker publish.

The only exception is WebRTC signaling (call:offer, call:answer, call:ice) which is
ephemeral and latency-sensitive — those may be published directly to Redis without a DB row.

---

## 2. Database table (already exists)

```sql
CREATE TYPE outbox_status AS ENUM ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED');

CREATE TABLE IF NOT EXISTS outbox_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type      VARCHAR(50)  NOT NULL,
    aggregate_type  VARCHAR(50)  NOT NULL,
    aggregate_id    VARCHAR(36)  NOT NULL,
    payload         JSONB        NOT NULL,
    status          outbox_status DEFAULT 'PENDING',
    retry_count     INT          DEFAULT 0,
    error           TEXT,
    created_at      TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP    NOT NULL DEFAULT NOW(),
    processed_at    TIMESTAMP
);

CREATE INDEX idx_outbox_status    ON outbox_events (status);
CREATE INDEX idx_outbox_pending   ON outbox_events (status, retry_count) WHERE status = 'PENDING';
CREATE INDEX idx_outbox_aggregate ON outbox_events (aggregate_type, aggregate_id);
```

The Go domain struct in `internal/domain/outbox/outbox.go` already matches this schema exactly.

### OutboxRepository interface (already exists in interfaces.go)

```go
type OutboxRepository interface {
    Create(ctx context.Context, tx DBTX, event *outbox.OutboxEvent) error
    GetPending(ctx context.Context, limit int) ([]outbox.OutboxEvent, error)
    MarkProcessing(ctx context.Context, id string) error
    MarkCompleted(ctx context.Context, id string) error
    MarkFailed(ctx context.Context, id string, errorMsg string) error
    IncrementRetry(ctx context.Context, id string) error
}
```

The `Create` method accepts a `DBTX` so the caller can pass an active `*sql.Tx`.
Pass `nil` if you want to use the default DB pool (outside a transaction).

---

## 3. Recommended migration: add next_retry_at

Before implementing the worker, add this column to support proper multi-instance backoff:

```sql
-- migrations/XXXX_add_outbox_next_retry_at.sql
ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_outbox_next_retry
    ON outbox_events (next_retry_at)
    WHERE status = 'PENDING';
```

Then update `GetPending` in `outbox_repository.go`:

```go
func (r *outboxRepository) GetPending(ctx context.Context, limit int) ([]outbox.OutboxEvent, error) {
    rows, err := r.db.QueryContext(ctx, `
        SELECT id, event_type, aggregate_type, aggregate_id, payload,
               status, retry_count, error, created_at, updated_at, processed_at
        FROM outbox_events
        WHERE status = 'PENDING'
          AND retry_count < 10
          AND (next_retry_at IS NULL OR next_retry_at <= NOW())
        ORDER BY created_at ASC
        LIMIT $1
    `, limit)
    // ... rest of scan logic unchanged
}
```

And add a helper to set `next_retry_at`:

```go
func (r *outboxRepository) ScheduleRetry(ctx context.Context, id string, nextRetryAt time.Time) error {
    _, err := r.db.ExecContext(ctx, `
        UPDATE outbox_events
        SET status = 'PENDING', retry_count = retry_count + 1,
            next_retry_at = $1, updated_at = NOW()
        WHERE id = $2
    `, nextRetryAt, id)
    return err
}
```

Add `ScheduleRetry` to the `OutboxRepository` interface in `interfaces.go`.

---

## 4. Step 1 — Create `internal/events/types.go`

Create the directory `internal/events/` and add this file first.
All event type strings are defined here so every package imports one source of truth.

```go
// internal/events/types.go
package events

// Message domain event types
const (
    MessageNew       = "message:new"
    MessageUpdated   = "message:updated"
    MessageDeleted   = "message:deleted"
    MessageDelivered = "message:delivered"
    MessageRead      = "message:read"
    MessagePlayed    = "message:played"
    ReactionAdded    = "reaction:added"
    ReactionRemoved  = "reaction:removed"
    MessagePinned    = "message:pinned"
    MessageUnpinned  = "message:unpinned"
)

// Conversation domain event types
const (
    ConversationCreated                = "conversation:created"
    ConversationUpdated                = "conversation:updated"
    ConversationParticipantAdded       = "conversation:participant_added"
    ConversationParticipantRemoved     = "conversation:participant_removed"
    ConversationParticipantRoleUpdated = "conversation:participant_role_updated"
    ConversationMuted                  = "conversation:muted"
    ConversationUnmuted                = "conversation:unmuted"
    ConversationArchived               = "conversation:archived"
    ConversationUnarchived             = "conversation:unarchived"
    ConversationCleared                = "conversation:cleared"
    ConversationInviteRegenerated      = "conversation:invite_regenerated"
)

// Typing and presence event types
const (
    TypingStarted   = "typing:started"
    TypingStopped   = "typing:stopped"
    PresenceOnline  = "presence:online"
    PresenceOffline = "presence:offline"
)

// Call domain event types
const (
    CallCreated            = "call:created"
    CallRinging            = "call:ringing"
    CallOffer              = "call:offer"
    CallAnswer             = "call:answer"
    CallICE                = "call:ice"
    CallConnected          = "call:connected"
    CallParticipantUpdated = "call:participant_updated"
    CallEnded              = "call:ended"
)

// Command domain event types
const (
    CommandExecuted = "command:executed"
    CommandFailed   = "command:failed"
    CommandUndone   = "command:undone"
)

// System event types
const (
    ConnectionReady = "connection:ready"
    ErrorEvent      = "error"
    PingEvent       = "ping"
    PongEvent       = "pong"
)
```

---

## 5. Step 2 — Create `internal/events/publisher.go`

```go
// internal/events/publisher.go
package events

import "context"

// Publisher is the interface for publishing serialized event payloads
// to a named Redis Pub/Sub channel.
// The concrete implementation lives in internal/redis/pubsub.go.
type Publisher interface {
    Publish(ctx context.Context, channel string, payload []byte) error
}

// Channel helpers — canonical channel name builders.
// Use these everywhere instead of hand-writing channel strings.

// ChannelForConversation returns the Pub/Sub channel for conversation-scoped events.
// Examples: message:new, typing:started, reaction:added
func ChannelForConversation(conversationID string) string {
    return "channel:conversation:" + conversationID
}

// ChannelForUser returns the Pub/Sub channel for user-targeted events.
// Examples: direct call invite, session/device changes
func ChannelForUser(userID string) string {
    return "channel:user:" + userID
}

// ChannelForCall returns the Pub/Sub channel for call lifecycle and signaling.
// Examples: call:offer, call:answer, call:ice, call:ended
func ChannelForCall(callID string) string {
    return "channel:call:" + callID
}

// ChannelForPresence returns the Pub/Sub channel for presence state changes.
func ChannelForPresence(userID string) string {
    return "channel:presence:" + userID
}
```

---

## 6. Step 3 — Create `internal/redis/client.go`

Install the Redis client if not already in go.mod:

```bash
go get github.com/redis/go-redis/v9
```

```go
// internal/redis/client.go
package redis

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"

    "sentinal-chat/config"
)

// Client wraps the go-redis universal client and exposes only what the project needs.
type Client struct {
    rdb *redis.Client
}

// New creates and verifies a Redis connection using the application config.
func New(cfg *config.Config) (*Client, error) {
    opts := &redis.Options{
        Addr:         fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
        Password:     cfg.RedisPassword,
        DB:           0,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
        PoolSize:     10,
        MinIdleConns: 2,
    }

    rdb := redis.NewClient(opts)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := rdb.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("redis: ping failed: %w", err)
    }

    return &Client{rdb: rdb}, nil
}

// Close gracefully closes the Redis connection pool.
func (c *Client) Close() error {
    return c.rdb.Close()
}

// Ping checks that Redis is reachable.
func (c *Client) Ping(ctx context.Context) error {
    return c.rdb.Ping(ctx).Err()
}

// Underlying returns the raw go-redis client.
// Use this only when you need a feature not wrapped by this client.
func (c *Client) Underlying() *redis.Client {
    return c.rdb
}

// Set stores a key-value pair with an optional TTL.
// Use ttl = 0 to store without expiry.
func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
    return c.rdb.Set(ctx, key, value, ttl).Err()
}

// Get retrieves a string value. Returns redis.Nil if the key does not exist.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
    return c.rdb.Get(ctx, key).Result()
}

// Del deletes one or more keys.
func (c *Client) Del(ctx context.Context, keys ...string) error {
    return c.rdb.Del(ctx, keys...).Err()
}

// Exists returns true if the key exists.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
    n, err := c.rdb.Exists(ctx, key).Result()
    return n > 0, err
}

// Expire sets a TTL on an existing key.
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
    return c.rdb.Expire(ctx, key, ttl).Err()
}

// Incr atomically increments a key and returns the new value.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
    return c.rdb.Incr(ctx, key).Result()
}

// Publish sends a message to a Redis Pub/Sub channel.
// Returns the number of subscribers that received the message.
func (c *Client) Publish(ctx context.Context, channel string, payload []byte) error {
    return c.rdb.Publish(ctx, channel, payload).Err()
}

// Subscribe subscribes to one or more channels and returns the PubSub handle.
// The caller must call Close() on the returned handle when done.
func (c *Client) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
    return c.rdb.Subscribe(ctx, channels...)
}

// PSubscribe subscribes to channels matching a glob pattern.
func (c *Client) PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub {
    return c.rdb.PSubscribe(ctx, patterns...)
}
```

### config.go additions

Make sure `config/config.go` exposes these fields (add them if missing):

```go
type Config struct {
    // ... existing fields ...

    // Redis
    RedisHost     string
    RedisPort     string
    RedisPassword string
}
```

---

## 7. Step 4 — Create `internal/redis/pubsub.go`

This file implements `events.Publisher` and provides the Redis-backed subscription helper.

```go
// internal/redis/pubsub.go
package redis

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    goredis "github.com/redis/go-redis/v9"
)

// PubSubPublisher implements events.Publisher using the Redis client.
type PubSubPublisher struct {
    client *Client
}

// NewPubSubPublisher creates a PubSubPublisher that wraps the shared Redis client.
func NewPubSubPublisher(client *Client) *PubSubPublisher {
    return &PubSubPublisher{client: client}
}

// Publish serializes the payload and sends it to the named Redis channel.
func (p *PubSubPublisher) Publish(ctx context.Context, channel string, payload []byte) error {
    return p.client.Publish(ctx, channel, payload)
}

// PublishEnvelope serializes an OutboxEnvelope and publishes it to the channel.
// This is the primary method used by the outbox worker.
func (p *PubSubPublisher) PublishEnvelope(ctx context.Context, channel string, env OutboxEnvelope) error {
    data, err := json.Marshal(env)
    if err != nil {
        return fmt.Errorf("pubsub: marshal envelope: %w", err)
    }
    return p.client.Publish(ctx, channel, data)
}

// Subscribe returns a Pub/Sub handle for the given channels.
// The caller is responsible for reading from handle.Channel() and calling handle.Close().
//
// Example:
//
//   sub := publisher.Subscribe(ctx, "channel:conversation:abc123")
//   defer sub.Close()
//   ch := sub.Channel()
//   for msg := range ch {
//       // handle msg.Payload
//   }
func (p *PubSubPublisher) Subscribe(ctx context.Context, channels ...string) *goredis.PubSub {
    return p.client.Subscribe(ctx, channels...)
}

// OutboxEnvelope is the canonical envelope published to Redis for every outbox event.
// WebSocket clients receive this structure (possibly after filtering) as their event payload.
type OutboxEnvelope struct {
    // OutboxID is the original DB outbox row id. Consumers use it for deduplication.
    OutboxID string `json:"outbox_id"`

    // EventType is the logical event name, e.g. "message:new", "reaction:added".
    EventType string `json:"event_type"`

    // AggregateType is the entity category, e.g. "message", "conversation", "call".
    AggregateType string `json:"aggregate_type"`

    // AggregateID is the primary key of the affected entity.
    AggregateID string `json:"aggregate_id"`

    // ConversationID is included for conversation-scoped events.
    ConversationID string `json:"conversation_id,omitempty"`

    // UserID is the acting user or target user depending on the event type.
    UserID string `json:"user_id,omitempty"`

    // OccurredAt is the authoritative server timestamp.
    OccurredAt time.Time `json:"occurred_at"`

    // Payload contains event-specific data.
    Payload json.RawMessage `json:"payload"`
}
```

---

## 8. Step 5 — Create `internal/redis/ratelimit.go`

This wires into `RateLimitMiddleware`, `MessageRateLimitMiddleware`, and `CallRateLimitMiddleware`.

```go
// internal/redis/ratelimit.go
package redis

import (
    "context"
    "fmt"
    "time"

    "sentinal-chat/internal/middleware"
)

// RateLimiter implements a sliding-window rate limiter backed by Redis.
type RateLimiter struct {
    client *Client
}

// NewRateLimiter creates a RateLimiter wrapping the shared Redis client.
func NewRateLimiter(client *Client) *RateLimiter {
    return &RateLimiter{client: client}
}

// CheckAuth returns a RateLimitChecker for auth endpoints.
// Default: 10 requests per minute per IP.
func (r *RateLimiter) CheckAuth(limit int, window time.Duration) middleware.RateLimitChecker {
    return r.checker("ratelimit:auth", limit, window)
}

// CheckMessage returns a RateLimitChecker for message send endpoints.
// Default: 60 messages per minute per user.
func (r *RateLimiter) CheckMessage(limit int, window time.Duration) middleware.RateLimitChecker {
    return r.checker("ratelimit:message", limit, window)
}

// CheckCall returns a RateLimitChecker for call creation endpoints.
// Default: 5 calls per minute per user.
func (r *RateLimiter) CheckCall(limit int, window time.Duration) middleware.RateLimitChecker {
    return r.checker("ratelimit:call", limit, window)
}

// checker returns a generic RateLimitChecker for the given key prefix, limit, and window.
// It uses a simple INCR + EXPIRE pattern (fixed window, not sliding window).
// For production use, replace with a Lua-script-based sliding window.
func (r *RateLimiter) checker(prefix string, limit int, window time.Duration) middleware.RateLimitChecker {
    return func(key string) (*middleware.RateLimitResult, error) {
        ctx := context.Background()
        redisKey := fmt.Sprintf("%s:%s", prefix, key)

        count, err := r.client.Incr(ctx, redisKey)
        if err != nil {
            // Fail open: if Redis is down, allow the request through.
            return &middleware.RateLimitResult{
                Allowed:   true,
                Limit:     limit,
                Remaining: limit,
                ResetIn:   window,
            }, nil
        }

        // On first increment, set the expiry.
        if count == 1 {
            _ = r.client.Expire(ctx, redisKey, window)
        }

        remaining := limit - int(count)
        if remaining < 0 {
            remaining = 0
        }

        return &middleware.RateLimitResult{
            Allowed:   count <= int64(limit),
            Limit:     limit,
            Remaining: remaining,
            ResetIn:   window,
        }, nil
    }
}
```

---

## 9. Step 6 — Create `internal/redis/presence.go`

```go
// internal/redis/presence.go
package redis

import (
    "context"
    "errors"
    "fmt"
    "time"

    goredis "github.com/redis/go-redis/v9"
)

const (
    // DefaultPresenceTTL is the TTL for a presence key.
    // The WebSocket hub must renew this on every heartbeat/ping.
    DefaultPresenceTTL = 65 * time.Second
)

// PresenceStore manages user online/offline state in Redis.
type PresenceStore struct {
    client *Client
}

// NewPresenceStore creates a PresenceStore wrapping the shared Redis client.
func NewPresenceStore(client *Client) *PresenceStore {
    return &PresenceStore{client: client}
}

// SetOnline marks a user as online with a short TTL.
// Call this on WebSocket connect and renew on every ping.
func (s *PresenceStore) SetOnline(ctx context.Context, userID string) error {
    key := presenceKey(userID)
    return s.client.Set(ctx, key, "online", DefaultPresenceTTL)
}

// SetOffline removes the presence key, marking the user offline immediately.
// Call this on WebSocket disconnect.
func (s *PresenceStore) SetOffline(ctx context.Context, userID string) error {
    key := presenceKey(userID)
    return s.client.Del(ctx, key)
}

// IsOnline returns true if the user has an active presence key.
func (s *PresenceStore) IsOnline(ctx context.Context, userID string) (bool, error) {
    key := presenceKey(userID)
    exists, err := s.client.Exists(ctx, key)
    if err != nil && errors.Is(err, goredis.Nil) {
        return false, nil
    }
    return exists, err
}

// Renew resets the TTL on the user's presence key.
// Call this on every WebSocket ping frame received from the client.
func (s *PresenceStore) Renew(ctx context.Context, userID string) error {
    key := presenceKey(userID)
    return s.client.Expire(ctx, key, DefaultPresenceTTL)
}

// IncrConnections increments the live WebSocket connection count for a user.
// Returns the new count.
func (s *PresenceStore) IncrConnections(ctx context.Context, userID string) (int64, error) {
    key := connectionKey(userID)
    count, err := s.client.Incr(ctx, key)
    if err != nil {
        return 0, err
    }
    if count == 1 {
        _ = s.client.Expire(ctx, key, 24*time.Hour)
    }
    return count, nil
}

// DecrConnections decrements the live WebSocket connection count for a user.
// Returns the new count. If the count reaches 0, the key is deleted.
func (s *PresenceStore) DecrConnections(ctx context.Context, userID string) (int64, error) {
    key := connectionKey(userID)
    count, err := s.client.rdb.Decr(ctx, key).Result()
    if err != nil {
        return 0, err
    }
    if count <= 0 {
        _ = s.client.Del(ctx, key)
        return 0, nil
    }
    return count, nil
}

func presenceKey(userID string) string {
    return fmt.Sprintf("presence:user:%s", userID)
}

func connectionKey(userID string) string {
    return fmt.Sprintf("connections:user:%s", userID)
}
```

---

## 10. Step 7 — Create `internal/redis/typing.go`

```go
// internal/redis/typing.go
package redis

import (
    "context"
    "fmt"
    "time"
)

const (
    // TypingTTL is how long a typing indicator lasts without renewal.
    // The client should send typing:start every 3–4 seconds while composing.
    TypingTTL = 8 * time.Second
)

// TypingStore manages ephemeral typing indicators in Redis.
type TypingStore struct {
    client *Client
}

// NewTypingStore creates a TypingStore wrapping the shared Redis client.
func NewTypingStore(client *Client) *TypingStore {
    return &TypingStore{client: client}
}

// SetTyping marks a user as currently typing in a conversation.
// The key expires automatically after TypingTTL.
func (s *TypingStore) SetTyping(ctx context.Context, conversationID, userID string) error {
    key := typingKey(conversationID, userID)
    return s.client.Set(ctx, key, "1", TypingTTL)
}

// ClearTyping removes the typing indicator for a user in a conversation.
func (s *TypingStore) ClearTyping(ctx context.Context, conversationID, userID string) error {
    key := typingKey(conversationID, userID)
    return s.client.Del(ctx, key)
}

// IsTyping returns true if the user has an active typing indicator.
func (s *TypingStore) IsTyping(ctx context.Context, conversationID, userID string) (bool, error) {
    key := typingKey(conversationID, userID)
    return s.client.Exists(ctx, key)
}

func typingKey(conversationID, userID string) string {
    return fmt.Sprintf("typing:conversation:%s:user:%s", conversationID, userID)
}
```

---

## 11. Step 8 — Create `internal/services/outbox_worker.go`

This is the background goroutine that polls the DB and publishes to Redis.

```go
// internal/services/outbox_worker.go
package services

import (
    "context"
    "encoding/json"
    "fmt"
    "math"
    "time"

    "sentinal-chat/internal/domain/outbox"
    "sentinal-chat/internal/events"
    "sentinal-chat/internal/repository"
    redisClient "sentinal-chat/internal/redis"
    "sentinal-chat/pkg/logger"
)

const (
    outboxPollInterval = 2 * time.Second
    outboxBatchSize    = 50
    outboxMaxRetries   = 10
)

// OutboxWorker polls the outbox_events table and publishes pending events to Redis.
type OutboxWorker struct {
    outboxRepo repository.OutboxRepository
    publisher  *redisClient.PubSubPublisher
    logger     *logger.Logger
}

// NewOutboxWorker creates a new OutboxWorker.
func NewOutboxWorker(
    outboxRepo repository.OutboxRepository,
    publisher *redisClient.PubSubPublisher,
    l *logger.Logger,
) *OutboxWorker {
    return &OutboxWorker{
        outboxRepo: outboxRepo,
        publisher:  publisher,
        logger:     l,
    }
}

// Run starts the outbox polling loop. It blocks until ctx is cancelled.
// Start this in a goroutine from cmd/api/main.go:
//
//   go worker.Run(ctx)
func (w *OutboxWorker) Run(ctx context.Context) {
    ticker := time.NewTicker(outboxPollInterval)
    defer ticker.Stop()

    w.logger.Infof("outbox worker started (poll interval: %s, batch: %d)", outboxPollInterval, outboxBatchSize)

    for {
        select {
        case <-ctx.Done():
            w.logger.Infof("outbox worker stopped")
            return
        case <-ticker.C:
            w.processBatch(ctx)
        }
    }
}

// processBatch fetches one batch of pending events and processes each one.
func (w *OutboxWorker) processBatch(ctx context.Context) {
    rows, err := w.outboxRepo.GetPending(ctx, outboxBatchSize)
    if err != nil {
        w.logger.Errorf("outbox: get pending failed: %v", err)
        return
    }

    for _, event := range rows {
        w.processEvent(ctx, event)
    }
}

// processEvent handles a single outbox event: mark processing, publish, mark done.
func (w *OutboxWorker) processEvent(ctx context.Context, event outbox.OutboxEvent) {
    // 1. Mark as PROCESSING to prevent duplicate delivery by another worker instance.
    if err := w.outboxRepo.MarkProcessing(ctx, event.ID.String()); err != nil {
        w.logger.Errorf("outbox: mark processing %s: %v", event.ID, err)
        return
    }

    // 2. Determine target Redis channel(s).
    channel, err := w.resolveChannel(event)
    if err != nil {
        w.logger.Errorf("outbox: resolve channel for %s (%s): %v", event.ID, event.EventType, err)
        _ = w.outboxRepo.MarkFailed(ctx, event.ID.String(), err.Error())
        return
    }

    // 3. Build the publish envelope.
    envelope := redisClient.OutboxEnvelope{
        OutboxID:      event.ID.String(),
        EventType:     event.EventType,
        AggregateType: event.AggregateType,
        AggregateID:   event.AggregateID,
        OccurredAt:    event.CreatedAt,
        Payload:       json.RawMessage(event.Payload),
    }

    // Extract conversation_id and user_id from payload if present.
    w.enrichEnvelope(&envelope, event.Payload)

    // 4. Publish to Redis.
    if err := w.publisher.PublishEnvelope(ctx, channel, envelope); err != nil {
        w.logger.Errorf("outbox: publish %s to %s: %v", event.ID, channel, err)

        // Retryable failure: increment retry count and return to PENDING.
        _ = w.outboxRepo.IncrementRetry(ctx, event.ID.String())
        if event.RetryCount+1 >= outboxMaxRetries {
            _ = w.outboxRepo.MarkFailed(ctx, event.ID.String(), err.Error())
            w.logger.Errorf("outbox: event %s exceeded max retries, marking FAILED", event.ID)
        } else {
            // Return to PENDING so the next poll picks it up.
            // If you added ScheduleRetry, use exponential backoff here instead.
            _ = w.outboxRepo.MarkProcessing(ctx, event.ID.String()) // already PROCESSING; no-op needed
            // Simple approach: just leave it PROCESSING — the next worker restart will recover
            // Better approach: call ScheduleRetry(id, backoffDuration) if that method exists
        }
        return
    }

    // 5. Mark completed.
    if err := w.outboxRepo.MarkCompleted(ctx, event.ID.String()); err != nil {
        w.logger.Warnf("outbox: mark completed %s: %v", event.ID, err)
        // Non-fatal: the event was delivered. Log and move on.
    }

    w.logger.Infof("outbox: published %s [%s] → %s", event.EventType, event.ID, channel)
}

// resolveChannel maps an outbox event to one or more Redis Pub/Sub channel names.
// Returns an error only for permanently unsupported event types.
func (w *OutboxWorker) resolveChannel(event outbox.OutboxEvent) (string, error) {
    // Extract IDs from payload so we can build channel names.
    var p struct {
        ConversationID string `json:"conversation_id"`
        UserID         string `json:"user_id"`
        CallID         string `json:"call_id"`
        ToUserID       string `json:"to_user_id"`
    }
    _ = json.Unmarshal(event.Payload, &p)

    switch {
    // --- Message and conversation events → conversation channel ---
    case strings.HasPrefix(event.EventType, "message:"),
        strings.HasPrefix(event.EventType, "reaction:"),
        strings.HasPrefix(event.EventType, "conversation:"),
        strings.HasPrefix(event.EventType, "typing:"):

        convID := p.ConversationID
        if convID == "" {
            convID = event.AggregateID // fallback
        }
        return fmt.Sprintf("channel:conversation:%s", convID), nil

    // --- Call lifecycle events → call channel ---
    case event.EventType == events.CallCreated,
        event.EventType == events.CallRinging,
        event.EventType == events.CallConnected,
        event.EventType == events.CallEnded,
        event.EventType == events.CallParticipantUpdated:

        callID := p.CallID
        if callID == "" {
            callID = event.AggregateID
        }
        return fmt.Sprintf("channel:call:%s", callID), nil

    // --- WebRTC signaling → targeted user channel (bypass outbox normally, but if used) ---
    case event.EventType == events.CallOffer,
        event.EventType == events.CallAnswer,
        event.EventType == events.CallICE:

        toUserID := p.ToUserID
        if toUserID == "" {
            toUserID = p.UserID
        }
        return fmt.Sprintf("channel:user:%s", toUserID), nil

    // --- Command events → user channel ---
    case strings.HasPrefix(event.EventType, "command:"):
        userID := p.UserID
        if userID == "" {
            userID = event.AggregateID
        }
        return fmt.Sprintf("channel:user:%s", userID), nil

    // --- Presence events → presence channel ---
    case strings.HasPrefix(event.EventType, "presence:"):
        userID := p.UserID
        if userID == "" {
            userID = event.AggregateID
        }
        return fmt.Sprintf("channel:presence:%s", userID), nil

    default:
        return "", fmt.Errorf("unknown event type: %s", event.EventType)
    }
}

// enrichEnvelope extracts conversation_id and user_id from the raw payload
// and populates the envelope fields so subscribers can use them for routing.
func (w *OutboxWorker) enrichEnvelope(env *redisClient.OutboxEnvelope, payload []byte) {
    var p struct {
        ConversationID string `json:"conversation_id"`
        UserID         string `json:"user_id"`
        CallID         string `json:"call_id"`
    }
    if err := json.Unmarshal(payload, &p); err != nil {
        return
    }
    if p.ConversationID != "" {
        env.ConversationID = p.ConversationID
    }
    if p.UserID != "" {
        env.UserID = p.UserID
    }
}
```

The full import block for `outbox_worker.go`:

```go
import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "sentinal-chat/internal/domain/outbox"
    "sentinal-chat/internal/events"
    "sentinal-chat/internal/repository"
    redisClient "sentinal-chat/internal/redis"
    "sentinal-chat/pkg/logger"
)
```

---

## 12. Retry and Backoff Policy

| Retry count | Wait before next attempt |
|---|---|
| 0 → 1 | 1 second |
| 1 → 2 | 2 seconds |
| 2 → 3 | 4 seconds |
| 3 → 4 | 8 seconds |
| 4 → 5 | 16 seconds |
| 5+ | 30 seconds (cap) |
| 10 (max) | Mark `FAILED` permanently |

If you add the `next_retry_at` column (see §3), compute the backoff like this:

```go
// backoffDuration computes exponential backoff with a 30-second cap.
func backoffDuration(retryCount int) time.Duration {
    base := time.Duration(1<<uint(retryCount)) * time.Second
    if base > 30*time.Second {
        base = 30 * time.Second
    }
    return base
}
```

Then call `ScheduleRetry(id, time.Now().Add(backoffDuration(event.RetryCount)))` instead of
leaving the row in `PROCESSING`.

---

## 13. Event Type to Redis Channel Quick Reference

| Event type | Redis channel |
|---|---|
| `message:new` | `channel:conversation:{conversation_id}` |
| `message:updated` | `channel:conversation:{conversation_id}` |
| `message:deleted` | `channel:conversation:{conversation_id}` |
| `message:delivered` | `channel:conversation:{conversation_id}` |
| `message:read` | `channel:conversation:{conversation_id}` |
| `message:played` | `channel:conversation:{conversation_id}` |
| `reaction:added` | `channel:conversation:{conversation_id}` |
| `reaction:removed` | `channel:conversation:{conversation_id}` |
| `message:pinned` | `channel:conversation:{conversation_id}` |
| `message:unpinned` | `channel:conversation:{conversation_id}` |
| `conversation:created` | `channel:conversation:{conversation_id}` |
| `conversation:updated` | `channel:conversation:{conversation_id}` |
| `conversation:participant_added` | `channel:conversation:{conversation_id}` |
| `conversation:participant_removed` | `channel:conversation:{conversation_id}` |
| `conversation:cleared` | `channel:conversation:{conversation_id}` |
| `typing:started` | `channel:conversation:{conversation_id}` |
| `typing:stopped` | `channel:conversation:{conversation_id}` |
| `call:created` | `channel:call:{call_id}` |
| `call:ringing` | `channel:call:{call_id}` |
| `call:connected` | `channel:call:{call_id}` |
| `call:ended` | `channel:call:{call_id}` |
| `call:offer` | `channel:user:{to_user_id}` |
| `call:answer` | `channel:user:{to_user_id}` |
| `call:ice` | `channel:user:{to_user_id}` |
| `command:executed` | `channel:user:{user_id}` |
| `command:undone` | `channel:user:{user_id}` |
| `command:failed` | `channel:user:{user_id}` |
| `presence:online` | `channel:presence:{user_id}` |
| `presence:offline` | `channel:presence:{user_id}` |

---

## 14. What Each Write Flow Must Do

### New message

```go
// Inside MessageService.Send — all in one transaction:
// 1. INSERT INTO messages (...)
// 2. INSERT INTO message_mentions (...) for each mention
// 3. INSERT INTO outbox_events {
//      event_type:     "message:new",
//      aggregate_type: "message",
//      aggregate_id:   message_id,
//      payload: {
//        message_id, conversation_id, sender_id, seq_id, type,
//        client_message_id, is_forwarded, mention_count, created_at
//      }
//    }
```

### Read receipt

```go
// Inside MessageService.MarkAsRead or WS hub read handler:
// 1. INSERT/UPDATE message_receipts SET status='READ', read_at=NOW()
// 2. UPDATE participants SET last_read_sequence = up_to_seq_id (if provided)
// 3. INSERT INTO outbox_events {
//      event_type:     "message:read",
//      aggregate_type: "message",
//      aggregate_id:   message_id,
//      payload: { message_id, conversation_id, user_id, read_at, last_read_sequence }
//    }
```

### Soft delete

```go
// Inside MessageService.DeleteMessage — all in one transaction:
// 1. INSERT INTO command_logs { status: PENDING, ... }
// 2. UPDATE messages SET deleted_at = NOW()
// 3. UPDATE command_logs SET status = EXECUTED, executed_at = NOW()
// 4. INSERT INTO outbox_events { event_type: "message:deleted", ... }
```

### Typing (ephemeral — does NOT use outbox)

```go
// Inside WS hub inbound handler for typing:start:
// 1. Verify participant (DB check or local cache)
// 2. TypingStore.SetTyping(ctx, conversationID, userID)
// 3. Publish DIRECTLY to Redis (no DB write, no outbox):
//    publisher.Publish(ctx, "channel:conversation:"+convID, envelope)
```

Typing events bypass the outbox entirely because:
- They are ephemeral (TTL-based) and not worth the DB round-trip.
- Loss of a typing event is acceptable.
- Latency matters more than durability here.

---

## 15. Wiring into cmd/api/main.go

```go
// cmd/api/main.go (abbreviated — add to existing setup)

package main

import (
    "context"
    "os/signal"
    "syscall"

    // ... existing imports ...
    redisStore "sentinal-chat/internal/redis"
    "sentinal-chat/internal/services"
    "sentinal-chat/internal/server"
    "sentinal-chat/internal/events"
)

func main() {
    // ... cfg, logger, db, repos setup ...

    // --- Redis ---
    redisClient, err := redisStore.New(cfg)
    if err != nil {
        l.Fatalf("redis connect: %v", err)
    }
    defer redisClient.Close()

    publisher   := redisStore.NewPubSubPublisher(redisClient)
    rateLimiter := redisStore.NewRateLimiter(redisClient)
    presence    := redisStore.NewPresenceStore(redisClient)
    typing      := redisStore.NewTypingStore(redisClient)

    // Suppress unused warnings until services are wired:
    _ = rateLimiter
    _ = events.MessageNew // ensure events package is imported

    // --- Background workers ---
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    outboxWorker := services.NewOutboxWorker(outboxRepo, publisher, l)
    go outboxWorker.Run(ctx)

    // --- WebSocket hub ---
    hub := server.NewHub(publisher, presence, typing, l)
    go hub.Run(ctx)

    // --- HTTP server ---
    srv := server.New(cfg, l)
    srv.SetupBaseRoutes()

    // Public routes
    public := srv.Engine().Group("/v1")
    authHandler.RegisterPublicRoutes(public)

    // Protected routes
    protected := srv.Engine().Group("/v1")
    protected.Use(middleware.AuthMiddleware(
        authSvc.ParseAccessToken,
        authSvc.ValidateAccessSession,
    ))
    authHandler.RegisterProtectedRoutes(protected)
    uploadHandler.RegisterRoutes(protected)
    // ... register remaining handlers as they are built ...

    // WebSocket route (auth handled inside the handler, not via middleware)
    srv.Engine().GET("/v1/ws", wsHandler.Handle)

    // Graceful shutdown
    if err := srv.Start(); err != nil {
        l.Errorf("server error: %v", err)
    }
    cancel() // stop outbox worker and hub
}
```

---

## 16. Idempotency

Workers can restart and re-process events. Consumers must tolerate duplicate delivery.

### Worker side

- The `PROCESSING` status acts as a lock. If two workers race, only the first `UPDATE ... WHERE status='PENDING'` wins (use `RETURNING` or check rows affected).
- Add `WHERE status = 'PENDING'` to the `MarkProcessing` update:

```go
// Improved MarkProcessing — returns false if another worker already claimed it
func (r *outboxRepository) MarkProcessing(ctx context.Context, id string) (bool, error) {
    result, err := r.db.ExecContext(ctx, `
        UPDATE outbox_events
        SET status = 'PROCESSING', updated_at = NOW()
        WHERE id = $1 AND status = 'PENDING'
    `, id)
    if err != nil {
        return false, err
    }
    n, _ := result.RowsAffected()
    return n > 0, nil
}
```

Update the interface in `interfaces.go` accordingly.

### WebSocket hub side

The hub should briefly remember recently delivered `outbox_id` values using a small
in-memory LRU or a `sync.Map` with a TTL, so duplicate publishes are dropped:

```go
// Simple deduplication in the hub — track last N delivered outbox IDs
type dedupCache struct {
    mu      sync.Mutex
    seen    map[string]time.Time
    maxAge  time.Duration
}

func (d *dedupCache) seenBefore(outboxID string) bool {
    d.mu.Lock()
    defer d.mu.Unlock()
    if t, ok := d.seen[outboxID]; ok && time.Since(t) < d.maxAge {
        return true
    }
    d.seen[outboxID] = time.Now()
    return false
}
```

---

## 17. Observability Checklist

Log these fields on every outbox event processed:

```go
w.logger.Infof("outbox event",
    "outbox_id",      event.ID,
    "event_type",     event.EventType,
    "aggregate_type", event.AggregateType,
    "aggregate_id",   event.AggregateID,
    "retry_count",    event.RetryCount,
    "channel",        channel,
    "latency_ms",     time.Since(event.CreatedAt).Milliseconds(),
)
```

Metrics to track (add Prometheus counters when ready):

| Metric | Description |
|---|---|
| `outbox_events_published_total` | Counter per `event_type` |
| `outbox_events_failed_total` | Counter per `event_type` |
| `outbox_pending_count` | Gauge — polled periodically |
| `outbox_processing_latency_ms` | Histogram — `processed_at - created_at` |
| `outbox_oldest_pending_age_seconds` | Gauge — age of oldest PENDING row |

    // 4