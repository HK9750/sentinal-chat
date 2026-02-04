# Two-Day Plan: Auth, Outbox, Redis Pub/Sub, WebSockets

This plan details the next two days of work to complete authentication end-to-end and ship a clean outbox + Redis Pub/Sub + WebSocket flow. It includes architecture, code structure, and step-by-step execution with clear deliverables.

## Day 1: Authentication Flow (Complete)

### Goals
- Production-grade authentication flow with short-lived access tokens and long-lived refresh tokens.
- Device-aware sessions and multi-device management.
- Solid security controls: rate limiting, token rotation, replay protection, password hashing.

### Why This Matters
- Every other system (WebSocket auth, outbox permissions, call signaling) depends on a correct, secure auth layer.
- Token and session design determines if you can safely scale horizontally without sticky sessions.
- Device and session tracking is the foundation for secure logouts, multi-device encryption, and abuse detection.

### Scope
1. HTTP auth endpoints
2. Auth service and repository integration
3. Token lifecycle
4. Session and device tracking
5. Auth middleware
6. Rate limiting and brute-force protection

### Code Structure

```
internal/
  handler/
    auth_handler.go
  middleware/
    auth_middleware.go
  services/
    auth_service.go
  repository/
    user_repository.go
  domain/
    user/entity.go
pkg/
  errors/
  logger/
```

**Responsibility breakdown**
- `auth_handler.go` parses HTTP input, calls the service, maps errors to HTTP status codes, and returns JSON.
- `auth_service.go` owns business logic: validation, password checks, token issuance, session rotation, and device updates.
- `auth_middleware.go` validates JWT, loads the session, and attaches user context to requests.
- `user_repository.go` provides all persistence operations for users, sessions, devices, and tokens.

### Endpoints

```
POST /v1/auth/register
POST /v1/auth/login
POST /v1/auth/refresh
POST /v1/auth/logout
POST /v1/auth/logout-all
GET  /v1/auth/sessions
POST /v1/auth/password/forgot
POST /v1/auth/password/reset
```

**Typical response shape**
- Success responses should include a stable envelope, even for auth:

```json
{
  "success": true,
  "data": {
    "access_token": "...",
    "refresh_token": "...",
    "expires_in": 900,
    "user": {
      "id": "uuid",
      "display_name": "...",
      "username": "..."
    }
  }
}
```

### Data Model Touchpoints

```
users
devices
user_sessions
push_tokens
user_settings
```

**How these tables connect**
- `users` is the identity record.
- `devices` identifies each client device (for session and E2E key ownership).
- `user_sessions` stores refresh-token sessions (revocation, expiry, auditing).
- `push_tokens` maps to device for notifications.
- `user_settings` is created at registration for a complete profile from day 1.

### Auth Flow Details

#### Register
- Validate unique email/username/phone.
- Create user.
- Create default user_settings.
- Create device row (optional, if device info provided).
- Create session with refresh token hash.
- Return access token + refresh token.

**Implementation sketch (service layer)**

```go
func (s *AuthService) Register(ctx context.Context, in RegisterInput) (AuthResult, error) {
    if err := in.Validate(); err != nil {
        return AuthResult{}, err
    }

    if err := s.ensureUniqueIdentity(ctx, in); err != nil {
        return AuthResult{}, err
    }

    hash, err := s.passwordHasher.Hash(in.Password)
    if err != nil {
        return AuthResult{}, err
    }

    u := &user.User{
        ID:           uuid.New(),
        Email:        toNullString(in.Email),
        Username:     toNullString(in.Username),
        PhoneNumber:  toNullString(in.PhoneNumber),
        PasswordHash: hash,
        DisplayName:  in.DisplayName,
        IsActive:     true,
        IsVerified:   false,
        CreatedAt:    time.Now(),
        UpdatedAt:    time.Now(),
    }

    refreshToken := s.tokenIssuer.NewRefreshToken()
    refreshHash := s.tokenIssuer.HashRefreshToken(refreshToken)

    session := &user.UserSession{
        ID:               uuid.New(),
        UserID:           u.ID,
        DeviceID:         in.DeviceID,
        RefreshTokenHash: refreshHash,
        ExpiresAt:        time.Now().Add(s.refreshTTL),
        CreatedAt:        time.Now(),
    }

    err = s.db.Transaction(func(tx *gorm.DB) error {
        if err := s.repo.CreateUserTx(ctx, tx, u); err != nil {
            return err
        }
        if err := s.repo.CreateUserSettingsTx(ctx, tx, u.ID); err != nil {
            return err
        }
        if in.DeviceID.Valid {
            if err := s.repo.UpsertDeviceTx(ctx, tx, u.ID, in); err != nil {
                return err
            }
        }
        return s.repo.CreateSessionTx(ctx, tx, session)
    })
    if err != nil {
        return AuthResult{}, err
    }

    accessToken, expiresIn := s.tokenIssuer.NewAccessToken(u.ID, session.ID, in.DeviceID)
    return AuthResult{User: *u, AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: expiresIn}, nil
}
```

**Example request**

```http
POST /v1/auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "StrongPassword123!",
  "display_name": "Jane",
  "device": {
    "device_id": "iphone-15-pro",
    "device_name": "Jane iPhone",
    "device_type": "ios"
  }
}
```

#### Login
- Verify credentials.
- Create device if not exists; update last_seen.
- Create session row and return tokens.
- Optional: update is_online and last_seen.

**Implementation sketch (password and session)**

```go
func (s *AuthService) Login(ctx context.Context, in LoginInput) (AuthResult, error) {
    u, err := s.repo.GetUserByIdentity(ctx, in.Identity)
    if err != nil {
        return AuthResult{}, err
    }

    if !u.IsActive {
        return AuthResult{}, sentinal_errors.ErrForbidden
    }

    if err := s.passwordHasher.Compare(u.PasswordHash, in.Password); err != nil {
        return AuthResult{}, sentinal_errors.ErrUnauthorized
    }

    refreshToken := s.tokenIssuer.NewRefreshToken()
    refreshHash := s.tokenIssuer.HashRefreshToken(refreshToken)

    session := &user.UserSession{
        ID:               uuid.New(),
        UserID:           u.ID,
        DeviceID:         in.DeviceID,
        RefreshTokenHash: refreshHash,
        ExpiresAt:        time.Now().Add(s.refreshTTL),
        CreatedAt:        time.Now(),
    }

    err = s.db.Transaction(func(tx *gorm.DB) error {
        if in.DeviceID.Valid {
            if err := s.repo.UpsertDeviceTx(ctx, tx, u.ID, in); err != nil {
                return err
            }
        }
        if err := s.repo.CreateSessionTx(ctx, tx, session); err != nil {
            return err
        }
        return s.repo.UpdateOnlineStatusTx(ctx, tx, u.ID, true)
    })
    if err != nil {
        return AuthResult{}, err
    }

    accessToken, expiresIn := s.tokenIssuer.NewAccessToken(u.ID, session.ID, in.DeviceID)
    return AuthResult{User: u, AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: expiresIn}, nil
}
```

#### Refresh
- Validate refresh token (hash compare).
- Ensure session is not revoked and not expired.
- Rotate refresh token: update hash + new expiry.
- Issue new access token.

**Implementation sketch (rotation)**

```go
func (s *AuthService) Refresh(ctx context.Context, in RefreshInput) (AuthResult, error) {
    session, err := s.repo.GetSessionByID(ctx, in.SessionID)
    if err != nil {
        return AuthResult{}, err
    }

    if session.IsRevoked || time.Now().After(session.ExpiresAt) {
        return AuthResult{}, sentinal_errors.ErrUnauthorized
    }

    if !s.tokenIssuer.CompareRefreshHash(session.RefreshTokenHash, in.RefreshToken) {
        return AuthResult{}, sentinal_errors.ErrUnauthorized
    }

    newRefresh := s.tokenIssuer.NewRefreshToken()
    newHash := s.tokenIssuer.HashRefreshToken(newRefresh)

    session.RefreshTokenHash = newHash
    session.ExpiresAt = time.Now().Add(s.refreshTTL)

    if err := s.repo.UpdateSession(ctx, session); err != nil {
        return AuthResult{}, err
    }

    accessToken, expiresIn := s.tokenIssuer.NewAccessToken(session.UserID, session.ID, session.DeviceID)
    return AuthResult{AccessToken: accessToken, RefreshToken: newRefresh, ExpiresIn: expiresIn}, nil
}
```

#### Logout
- Revoke single session.

**Implementation sketch**

```go
func (s *AuthService) Logout(ctx context.Context, sessionID uuid.UUID) error {
    return s.repo.RevokeSession(ctx, sessionID)
}
```

#### Logout All
- Revoke all sessions for user.

**Implementation sketch**

```go
func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
    return s.repo.RevokeAllUserSessions(ctx, userID)
}
```

### Token Strategy

- Access token: short TTL (10–15 minutes), JWT with user_id, session_id, device_id.
- Refresh token: long TTL (7–30 days), stored as hash in DB.
- Rotation: every refresh creates a new refresh token and invalidates the old one.

**Access token claims example**

```json
{
  "sub": "user_id",
  "sid": "session_id",
  "did": "device_id",
  "exp": 1700000000,
  "iat": 1699990000,
  "scope": ["chat:read", "chat:write"]
}
```

**Refresh token storage strategy**
- Store only a hash (HMAC-SHA256 or bcrypt) in `user_sessions.refresh_token_hash`.
- Keep refresh tokens random, long, and unguessable.
- For rotation, update the hash in the same transaction that issues the new token.

### Security Controls

- Rate limit `/login`, `/register`, `/refresh` by IP and user.
- Password hashing with bcrypt.
- Do not store refresh tokens in plaintext.
- Block login when account is inactive or banned.
- Audit log via command_log and outbox events when available.

**Rate limit strategy (Redis)**

```
ratelimit:{ip}:auth -> integer counter with TTL 60s
ratelimit:{user_id}:auth -> integer counter with TTL 60s
```

**Recommended limits**
- Login: 5 requests/minute per IP, 10 requests/minute per user.
- Register: 3 requests/minute per IP.
- Refresh: 10 requests/minute per session.

**Middleware check example**

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := extractBearerToken(r)
        claims, err := jwtVerifier.Verify(token)
        if err != nil {
            writeError(w, http.StatusUnauthorized)
            return
        }

        session, err := repo.GetSessionByID(r.Context(), claims.SessionID)
        if err != nil || session.IsRevoked || time.Now().After(session.ExpiresAt) {
            writeError(w, http.StatusUnauthorized)
            return
        }

        ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Common failure scenarios**
- Invalid password: return 401 without revealing identity existence.
- Refresh token mismatch: revoke the session to stop replay attempts.
- Session expired: respond with 401, require full login.
- Device mismatch: optionally log and revoke session.

### Deliverables

- Implement auth handlers and service logic.
- Fully wired session lifecycle.
- Middleware checks JWT, loads session, and enforces revoked/expired checks.
- Tests for core auth flows if time permits.

**Suggested tests**
- Register returns user + tokens and creates session row.
- Login rejects invalid credentials.
- Refresh rotates refresh token and invalidates old token.
- Logout revokes session; JWT rejected afterward.

## Day 2: Outbox + Redis Pub/Sub + WebSockets

### Goals
- Reliable outbox processing with retries.
- Redis Pub/Sub event fan-out.
- WebSocket hub that listens to Redis and pushes to connected clients.

### Why This Matters
- The outbox pattern guarantees no lost events when you write to Postgres and publish to Redis.
- Redis Pub/Sub gives fast, low-latency fan-out for real-time features.
- WebSockets deliver a WhatsApp-like experience with instant updates.

### Scope
1. Outbox worker that polls and publishes
2. Redis Pub/Sub channels and event format
3. WebSocket hub + client registry
4. Event routing and authorization

### Architecture Overview

```
Command Handler
  -> DB transaction
    -> write data
    -> insert outbox event

Outbox Worker
  -> fetch pending outbox_events
  -> publish to Redis Pub/Sub
  -> mark processed or retry

Redis Pub/Sub
  -> channel-based fan-out

WebSocket Hub
  -> subscribe to Redis
  -> broadcast to active clients
```

**Guaranteed delivery boundary**
- Outbox provides durability at the database boundary.
- Redis Pub/Sub is best-effort; events are durable only once they are written to outbox.
- If Redis is down, outbox events remain pending and are retried.

### Code Structure

```
internal/
  repository/
    event_repository.go
  worker/
    outbox/
      processor.go
  infrastructure/
    messaging/redis/
      publisher.go
      subscriber.go
  interfaces/
    websocket/
      hub.go
      client.go
      handler.go
  services/
    event_router.go
```

**Responsibility breakdown**
- `event_repository.go` reads and updates outbox rows and delivery logs.
- `worker/outbox/processor.go` polls and publishes pending events.
- `messaging/redis/publisher.go` owns publish logic and marshaling.
- `messaging/redis/subscriber.go` handles subscriptions for WebSocket fan-out.
- `websocket/hub.go` stores connected clients and routes messages.

### Outbox Event Model

```
outbox_events
  id
  aggregate_type
  aggregate_id
  event_type
  payload
  correlation_id
  created_at
  processed_at
  retry_count
  max_retries
  next_retry_at
  error_message
```

**Notes**
- `processed_at` marks an event as done.
- `retry_count` and `next_retry_at` control backoff.
- `outbox_event_deliveries` provides an audit trail per attempt.

### Redis Pub/Sub Channel Plan

```
events:global                         -> all events (debug / analytics)
events:user:{user_id}                 -> per-user events (notifications, receipts)
events:conversation:{conversation_id} -> chat stream for members
events:call:{call_id}                 -> signaling + status
```

**Channel selection strategy**
- Use user channels for private data like receipts and direct notifications.
- Use conversation channels for chat message broadcast.
- Use call channels for signaling and call state changes.

### Event Payload Format

```
{
  "id": "event_id",
  "type": "message.created",
  "aggregate_type": "message",
  "aggregate_id": "uuid",
  "correlation_id": "uuid",
  "occurred_at": "timestamp",
  "payload": { ... }
}
```

**Typical payload example**

```json
{
  "id": "d3b4b64e-9f9b-4b4b-b0f8-7c1b0b5e8c1e",
  "type": "message.created",
  "aggregate_type": "message",
  "aggregate_id": "6a1c1c0f-0e60-46a5-9b9f-8e44b0d7d0f2",
  "correlation_id": "d2c9b9a0-4a42-42ea-8b46-28b5b171bdf1",
  "occurred_at": "2026-02-04T10:05:00Z",
  "payload": {
    "conversation_id": "...",
    "sender_id": "...",
    "content": "Hello"
  }
}
```

### Outbox Worker Logic

1. Fetch pending events (processed_at is null, next_retry_at passed)
2. Publish to Redis
3. Insert outbox_event_deliveries row
4. Mark processed on success
5. On failure, increment retry_count and set next_retry_at

**Implementation sketch (worker loop)**

```go
func (p *Processor) Run(ctx context.Context) {
    ticker := time.NewTicker(p.pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            p.processBatch(ctx)
        }
    }
}

func (p *Processor) processBatch(ctx context.Context) {
    events, err := p.repo.GetPendingOutboxEvents(ctx, p.batchSize)
    if err != nil {
        p.log.Error("outbox fetch failed", "err", err)
        return
    }

    for _, e := range events {
        if err := p.publisher.Publish(ctx, e); err != nil {
            nextRetry := time.Now().Add(p.backoff.ForAttempt(e.RetryCount + 1))
            _ = p.repo.MarkOutboxEventFailed(ctx, e.ID, nextRetry, err.Error())
            _ = p.repo.CreateOutboxEventDelivery(ctx, &event.OutboxEventDelivery{
                ID:            uuid.New(),
                EventID:       e.ID,
                AttemptNumber: e.RetryCount + 1,
                Status:        "FAILED",
                ErrorMessage:  toNullString(err.Error()),
                CreatedAt:     time.Now(),
            })
            continue
        }

        _ = p.repo.MarkOutboxEventProcessed(ctx, e.ID)
        _ = p.repo.CreateOutboxEventDelivery(ctx, &event.OutboxEventDelivery{
            ID:            uuid.New(),
            EventID:       e.ID,
            AttemptNumber: e.RetryCount + 1,
            Status:        "DELIVERED",
            DeliveredAt:   toNullTime(time.Now()),
            CreatedAt:     time.Now(),
        })
    }
}
```

**Backoff strategy**
- Simple exponential backoff with cap: `min(base * 2^attempt, max)`.
- Example: 1s, 2s, 4s, 8s, 16s, capped at 60s.

### WebSocket Flow

#### Connection
- Client connects with access token.
- Server validates and registers client to user_id room.
- Optional: join conversation rooms after permission check.

#### Publish
- Redis subscriber receives event.
- WebSocket hub routes to user or conversation channel.
- Send JSON event to all active clients.

#### Disconnect
- Client removed from registry.
- Optional: update presence in Redis.

**WebSocket hub structure (simplified)**

```go
type Hub struct {
    register   chan *Client
    unregister chan *Client
    clients    map[string]map[*Client]struct{}
}

func (h *Hub) Run(ctx context.Context) {
    for {
        select {
        case c := <-h.register:
            if _, ok := h.clients[c.UserID]; !ok {
                h.clients[c.UserID] = make(map[*Client]struct{})
            }
            h.clients[c.UserID][c] = struct{}{}
        case c := <-h.unregister:
            if set, ok := h.clients[c.UserID]; ok {
                delete(set, c)
                if len(set) == 0 {
                    delete(h.clients, c.UserID)
                }
            }
        case <-ctx.Done():
            return
        }
    }
}

func (h *Hub) BroadcastToUser(userID string, payload []byte) {
    for c := range h.clients[userID] {
        c.Send(payload)
    }
}
```

**Redis subscriber to hub**

```go
func (s *Subscriber) Listen(ctx context.Context, channels []string, onMessage func([]byte)) error {
    sub := s.client.Subscribe(ctx, channels...)
    defer sub.Close()

    for {
        msg, err := sub.ReceiveMessage(ctx)
        if err != nil {
            return err
        }
        onMessage([]byte(msg.Payload))
    }
}
```

### Suggested Redis Usage Patterns

```
presence:{user_id}                 -> online/offline, TTL 60s
presence:{user_id}:typing:{conv}   -> typing indicator, TTL 5s
ws:user:{user_id}                  -> set of active socket IDs
```

**Presence update example**
- On WebSocket connect: `SETEX presence:{user_id} 60 "online"`
- On heartbeat: refresh TTL
- On disconnect: let TTL expire (or `DEL` immediately)

### Deliverables

- Outbox worker running on a timer or background goroutine.
- Redis pub/sub publisher and subscriber.
- WebSocket hub connected to Redis subscriber.
- Event routing rules for user and conversation events.
- Basic observability hooks (log publish success/failure).

**Observability essentials**
- Log outbox lag: now - created_at for oldest pending.
- Log publish failures with event_id and retry_count.
- Track WebSocket connected clients count.

## Suggested Next Steps After Day 2

1. Add integration tests for outbox processing.
2. Add Redis streams for durable event delivery (if needed).
3. Implement backpressure for WebSocket clients.
4. Add metrics around event lag and processing latency.

## Concrete Implementation Examples

### 1) Insert Outbox Event in a Command

```go
func (s *MessageService) SendMessage(ctx context.Context, in SendMessageInput) (message.Message, error) {
    var msg message.Message

    err := s.db.Transaction(func(tx *gorm.DB) error {
        msg = message.Message{
            ID:             uuid.New(),
            ConversationID: in.ConversationID,
            SenderID:       in.SenderID,
            Content:        toNullString(in.Content),
            Type:           "TEXT",
            CreatedAt:      time.Now(),
        }
        if err := tx.Create(&msg).Error; err != nil {
            return err
        }

        outbox := event.OutboxEvent{
            ID:            uuid.New(),
            AggregateType: "message",
            AggregateID:   msg.ID,
            EventType:     "message.created",
            Payload:       mustJSON(msg),
            CreatedAt:     time.Now(),
        }
        return tx.Create(&outbox).Error
    })
    if err != nil {
        return message.Message{}, err
    }

    return msg, nil
}
```

### 2) Publish to Redis (Publisher)

```go
type RedisPublisher struct {
    client *redis.Client
}

func (p *RedisPublisher) Publish(ctx context.Context, e event.OutboxEvent) error {
    payload := EventEnvelope{
        ID:            e.ID.String(),
        Type:          e.EventType,
        AggregateType: e.AggregateType,
        AggregateID:   e.AggregateID.String(),
        CorrelationID: e.CorrelationID.UUID.String(),
        OccurredAt:    e.CreatedAt.UTC(),
        Payload:       json.RawMessage(e.Payload),
    }

    data, err := json.Marshal(payload)
    if err != nil {
        return err
    }

    channel := routeChannel(payload)
    return p.client.Publish(ctx, channel, data).Err()
}
```

### 3) Route Channels by Event Type

```go
func routeChannel(e EventEnvelope) string {
    switch e.AggregateType {
    case "message":
        return "events:conversation:" + e.PayloadConversationID()
    case "receipt":
        return "events:user:" + e.PayloadUserID()
    case "call":
        return "events:call:" + e.PayloadCallID()
    default:
        return "events:global"
    }
}
```

### 4) WebSocket Handler (Handshake)

```go
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    token := r.URL.Query().Get("token")
    claims, err := h.jwtVerifier.Verify(token)
    if err != nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }

    client := NewClient(conn, claims.UserID)
    h.hub.register <- client
    go client.WritePump()
    go client.ReadPump()
}
```

### 5) Example WebSocket Message to Client

```json
{
  "type": "message.created",
  "conversation_id": "...",
  "message": {
    "id": "...",
    "sender_id": "...",
    "content": "Hello"
  }
}
```
