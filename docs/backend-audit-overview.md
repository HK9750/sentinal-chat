# Backend Audit Overview

This document is the ground-truth status report for the Sentinal Chat backend.
It describes exactly what code exists, what is missing, what bugs must be fixed,
and the precise order in which remaining work should be done.

Last audited against:
- `internal/handler/auth_handler.go`
- `internal/handler/upload_handler.go`
- `internal/handler/auth_handler_test.go`
- `internal/server/server.go`
- `internal/services/auth_service.go`
- `internal/services/auth_models.go`
- `internal/services/token_service.go`
- `internal/services/upload_service.go`
- `internal/services/oauth_google.go`
- `internal/services/oauth_github.go`
- `internal/middleware/*.go`
- `internal/repository/interfaces.go`
- `internal/repository/*.go`
- `internal/domain/**/*.go`
- `internal/transport/httpdto/*.go`
- `config/config.go`
- `internal/storage/s3_client.go`
- `migrations/*.sql`

---

## 1. What exists today

### 1.1 HTTP server and routing

File: `internal/server/server.go`

The server is fully wired:
- Creates a Gin engine with `gin.New()` (no default logger — uses custom middleware).
- Applies `gin.Recovery()`.
- Applies global middleware in `SetupBaseRoutes()`:
  - `RequestIDMiddleware`
  - `CORSMiddleware` (origin from `config.FrontendURL`)
  - `LoggingMiddleware` (Zap)
  - `ErrorHandler`
- Registers three utility routes:
  - `GET /ping` → `{"success":true,"data":{"message":"pong"}}`
  - `GET /health` → calls `database.HealthCheck()`, returns `healthy` or `UNHEALTHY`
  - `GET /goroutines` → `{"goroutines": N}`
- Starts with graceful shutdown on `SIGTERM`/`SIGINT` with 5-second timeout.

**Nothing else is registered yet.** All `/v1/...` routes must be added in `cmd/api/main.go`.

---

### 1.2 Auth — fully implemented

#### Handler: `internal/handler/auth_handler.go`

Registered public routes (no auth required):
```
POST /auth/register
POST /auth/login
POST /auth/refresh
GET  /auth/oauth/:provider/url
POST /auth/oauth/:provider/exchange
```

Registered protected routes (auth middleware required):
```
POST /auth/logout
POST /auth/logout-all
GET  /auth/sessions
```

Key behaviors:
- `Register` and `Login` call the service, set the refresh token as an `HttpOnly` cookie,
  strip the refresh token from the JSON body, and return `201` / `200` respectively.
- `Refresh` reads the token from the JSON body first, falls back to the `refresh_token` cookie.
- `Logout` accepts an optional `session_id` body field. If the revoked session is the current one,
  the cookie is cleared.
- `LogoutAll` revokes all sessions and clears the cookie.
- `Sessions` returns all sessions for the user with `is_current` flagged on the active one.

The handler maps errors from sentinel errors + two service-specific errors:
```go
services.ErrInvalidCredentials      → 401  AUTH_INVALID_CREDENTIALS
services.ErrUnsupportedOAuthProvider → 400  AUTH_OAUTH_PROVIDER_UNSUPPORTED
services.ErrOAuthEmailUnverified     → 403  AUTH_OAUTH_EMAIL_UNVERIFIED
sentinal_errors.ErrAlreadyExists     → 409  AUTH_IDENTIFIER_TAKEN
```

#### Service: `internal/services/auth_service.go`

- `Register`: hashes password (bcrypt), creates user, upserts device, creates session, issues JWT pair.
- `Login`: finds user by email/username/phone, verifies password, upserts device, creates session.
- `Refresh`: finds session by refresh token hash, verifies not expired/revoked, rotates token pair.
- `Logout`: revokes one specific session (or current session if no `session_id` given).
- `LogoutAll`: revokes all sessions for the user.
- `ListSessions`: returns all sessions with device info, flags current.
- `ValidateAccessSession`: called by auth middleware — checks session is active, device matches.
- `ExchangeOAuth`: exchanges code+verifier with provider, upserts user + OAuth identity, issues session.
- `AuthorizeOAuth`: builds provider authorization URL with PKCE state.

#### Token service: `internal/services/token_service.go`

- Issues JWT access tokens (HS256, claims: `user_id`, `session_id`, `device_id`).
- Issues random refresh tokens (32-byte secure random, SHA-256 hashed for DB storage).
- `ParseAccessToken` validates signature, algorithm, expiry, and required claims.
- Configurable `accessTTL`, `refreshTTL`, `issuer`, 15-second leeway for clock skew.

#### OAuth providers: `oauth_google.go`, `oauth_github.go`

Both implement the `OAuthProviderClient` interface:
```go
type OAuthProviderClient interface {
    AuthorizationURL(input OAuthAuthorizeInput) (OAuthAuthorizeResult, error)
    ExchangeCode(ctx context.Context, input OAuthExchangeInput) (OAuthIdentity, error)
}
```
PKCE flow is fully supported. Both providers fetch user profile and verify email.

---

### 1.3 Uploads — fully implemented

#### Handler: `internal/handler/upload_handler.go`

Registered protected routes:
```
POST   /uploads
POST   /uploads/bulk
POST   /attachments
GET    /attachments/:id
POST   /attachments/:id/viewed
GET    /messages/:id/attachments
```

#### Service: `internal/services/upload_service.go`

- `UploadFile`: validates metadata, uploads the incoming multipart file directly to S3, returns `file_url` and object metadata.
- `UploadFiles`: uploads many multipart files concurrently with goroutines.
- `CreateAttachment`: validates provided file metadata, inserts `attachments` row, and optionally links to a message via `message_attachments`.
- `GetAttachment`: checks user can access the attachment (must be uploader or participant in
  the conversation the message belongs to).
- `MarkAttachmentViewed`: marks `view_once` attachments as viewed (one-time only).
- `GetMessageAttachments`: returns all attachments for a message.

---

### 1.4 Middleware — all six implemented

| File | Purpose |
|---|---|
| `auth_middleware.go` | Extracts + validates JWT, injects `user_id`/`session_id`/`device_id` into Gin context |
| `cors_middleware.go` | CORS with allowed origin from config |
| `error_middleware.go` | Catches unhandled panics, logs, returns 500 envelope |
| `logging_middleware.go` | Logs method, path, status, latency, request ID via Zap |
| `ratelimit_middleware.go` | Three variants: auth (by IP), message (by user_id), call (by user_id) |
| `request_id_middleware.go` | Generates `X-Request-ID` per request |

The rate limit middleware accepts a `RateLimitChecker func(key string) (*RateLimitResult, error)` —
this function is injected from outside. **The Redis sliding-window implementation of this checker
does not exist yet** and must be written in `internal/redis/ratelimit.go`.

Auth middleware context keys:
```go
c.Set("user_id",    userID)     // uuid.UUID
c.Set("session_id", sessionID)  // string
c.Set("device_id",  deviceID)   // string
```

---

### 1.5 Domain models — all implemented

All domain structs are in `internal/domain/`. They are pure Go — no GORM tags, no JSON tags.

| Package | Structs |
|---|---|
| `user` | `User`, `Device`, `FcmToken`, `UserSession`, `UserContact` |
| `conversation` | `Conversation`, `Participant`, `ConversationSequence`, `ConversationClear` |
| `message` | `Message`, `MessageReaction`, `MessageReceipt`, `MessageMention`, `StarredMessage`, `PinnedMessage`, `MessageEdit` |
| `message` (attachments.go) | `Attachment`, `MessageAttachment` |
| `message` (polls.go) | `Poll`, `PollOption`, `PollVote` |
| `call` | `Call`, `CallParticipant` |
| `command` | `CommandLog`, `Status` |
| `outbox` | `OutboxEvent`, `Status` |

---

### 1.6 Repository layer — all interfaces + implementations exist

All repository interfaces are declared in `internal/repository/interfaces.go`.
Each interface has a concrete implementation in its own file.

| Interface | Implementation file |
|---|---|
| `UserRepository` | `user_repository.go` |
| `ConversationRepository` | `conversation_repository.go` |
| `MessageRepository` | `message_repository.go` |
| `CallRepository` | `call_repository.go` |
| `OutboxRepository` | `outbox_repository.go` |
| `CommandRepository` | `command_repository.go` |
| `OAuthIdentityRepository` | `oauth_identity_repository.go` |

The `OutboxRepository.Create` signature accepts a `DBTX` parameter so the caller can pass an
active transaction:
```go
Create(ctx context.Context, tx DBTX, event *outbox.OutboxEvent) error
```
This is the correct design. Every service that writes outbox events must pass the transaction here.

---

### 1.7 HTTP DTOs — partial

| File | Status |
|---|---|
| `response.go` | ✅ `Response[T]`, `WriteSuccess`, `WriteError` |
| `auth_dto.go` | ✅ Full request + payload structs for all auth endpoints |
| `upload_dto.go` | ✅ Full request + payload structs for upload + attachment endpoints |
| `user_dto.go` | ❌ Missing |
| `conversation_dto.go` | ❌ Missing |
| `message_dto.go` | ❌ Missing |
| `call_dto.go` | ❌ Missing |
| `command_dto.go` | ❌ Missing |

---

### 1.8 SQL Migrations

All migrations exist and cover:
- `users`, `devices`, `fcm_tokens`, `user_sessions`, `user_contacts`
- `conversations`, `participants`, `conversation_sequences`, `conversation_clears`
- `messages`, `message_receipts`, `message_reactions`, `message_mentions`
- `starred_messages`, `pinned_messages`, `message_edits`, `message_attachments`
- `attachments`
- `polls`, `poll_options`, `poll_votes`
- `calls`, `call_participants`
- `command_logs`
- `outbox_events`

Indexes:
- `idx_outbox_status` on `outbox_events(status)`
- `idx_outbox_pending` on `outbox_events(status, retry_count)` WHERE `status = 'PENDING'`
- `idx_outbox_aggregate` on `outbox_events(aggregate_type, aggregate_id)`
- `idx_command_logs_user_created`, `idx_command_logs_conv`, `idx_command_logs_status`
- Indexes on participants, messages, receipts for conversation queries

PostgreSQL triggers:
- `messages` table: trigger assigns `seq_id` by incrementing `conversation_sequences.last_sequence`
  on every INSERT. This means `seq_id` is DB-assigned and should NOT be set by application code.

---

## 2. What is missing

### 2.1 Redis infrastructure — nothing exists yet

There is no Redis client, no Pub/Sub publisher, no subscriber, no presence store,
no typing store, and no rate limiter implementation.

Files to create:

```
internal/redis/client.go
internal/redis/pubsub.go
internal/redis/ratelimit.go
internal/redis/presence.go
internal/redis/typing.go
```

The `ratelimit_middleware.go` expects a `RateLimitChecker func(key string) (*RateLimitResult, error)`
injected from outside. The concrete implementation must live in `internal/redis/ratelimit.go`.

---

### 2.2 User handler and service — nothing exists yet

Files to create:
```
internal/services/user_service.go
internal/services/user_models.go
internal/handler/user_handler.go
internal/transport/httpdto/user_dto.go
```

Routes to register (all protected):
```
GET    /v1/users/me
PUT    /v1/users/me
GET    /v1/users/:id
GET    /v1/users/search
GET    /v1/users/contacts
POST   /v1/users/contacts
DELETE /v1/users/contacts/:contact_user_id
POST   /v1/users/contacts/:contact_user_id/block
POST   /v1/users/contacts/:contact_user_id/unblock
GET    /v1/users/contacts/blocked
POST   /v1/users/devices
GET    /v1/users/devices
DELETE /v1/users/devices/:device_uuid
POST   /v1/users/devices/:device_uuid/push-tokens
GET    /v1/users/push-tokens
DELETE /v1/users/push-tokens/:token_id
GET    /v1/users/me/mentions
```

Repository methods available (no new DB work needed):
- `GetUserByID`, `UpdateUser`, `SearchUsers`
- `GetUserContacts`, `AddUserContact`, `RemoveUserContact`, `BlockContact`, `UnblockContact`, `GetBlockedContacts`
- `GetUserDevices`, `DeactivateDevice`, `AddFcmToken`, `GetUserFcmTokens`, `DeactivateFcmToken`
- `GetUserMentions` (in MessageRepository)

---

### 2.3 Conversation handler and service — nothing exists yet

Files to create:
```
internal/services/conversation_service.go
internal/services/conversation_models.go
internal/handler/conversation_handler.go
internal/transport/httpdto/conversation_dto.go
```

Routes to register (all protected):
```
POST   /v1/conversations
GET    /v1/conversations
GET    /v1/conversations/direct
GET    /v1/conversations/search
GET    /v1/conversations/type
GET    /v1/conversations/invite
GET    /v1/conversations/:id
PUT    /v1/conversations/:id
DELETE /v1/conversations/:id
POST   /v1/conversations/:id/invite
POST   /v1/conversations/:id/participants
DELETE /v1/conversations/:id/participants/:user_id
GET    /v1/conversations/:id/participants
PUT    /v1/conversations/:id/participants/:user_id/role
POST   /v1/conversations/:id/mute
POST   /v1/conversations/:id/unmute
POST   /v1/conversations/:id/archive
POST   /v1/conversations/:id/unarchive
POST   /v1/conversations/:id/read-sequence
GET    /v1/conversations/:id/sequence
POST   /v1/conversations/:id/clear
GET    /v1/conversations/:id/pinned-messages
```

Outbox events that conversation service must emit (same DB transaction):
```
conversation:created
conversation:updated
conversation:participant_added
conversation:participant_removed
conversation:participant_role_updated
conversation:muted
conversation:unmuted
conversation:archived
conversation:unarchived
conversation:cleared
conversation:invite_regenerated
```

DM creation rule: before creating a new DM conversation, call
`ConversationRepository.GetDirectConversation(userA, userB)`. If it already exists, return it.

---

### 2.4 Message handler and service — nothing exists yet

Files to create:
```
internal/services/message_service.go
internal/services/message_models.go
internal/handler/message_handler.go
internal/transport/httpdto/message_dto.go
```

Routes to register (all protected):
```
POST   /v1/messages
GET    /v1/messages
GET    /v1/messages/:id
PUT    /v1/messages/:id
DELETE /v1/messages/:id
DELETE /v1/messages/:id/hard
POST   /v1/messages/:id/delivered
POST   /v1/messages/:id/read
POST   /v1/messages/:id/played
POST   /v1/messages/bulk-delivered
POST   /v1/messages/bulk-read
POST   /v1/messages/:id/reactions
DELETE /v1/messages/:id/reactions/:reaction_code
GET    /v1/messages/:id/reactions
POST   /v1/messages/:id/star
DELETE /v1/messages/:id/star
GET    /v1/messages/starred
POST   /v1/messages/:id/pin
DELETE /v1/messages/:id/pin
GET    /v1/messages/:id/edits
GET    /v1/messages/:id/receipts
GET    /v1/conversations/:id/messages/by-type
GET    /v1/conversations/:id/messages/range
GET    /v1/conversations/:id/messages/unread
```

Important: `seq_id` is set by the DB trigger on INSERT. Do not set it in the application.
`client_message_id` must be checked for idempotency before inserting
(use `MessageRepository.GetByClientMessageID`).

Outbox events message service must emit:
```
message:new
message:updated
message:deleted
message:delivered
message:read
message:played
reaction:added
reaction:removed
message:pinned
message:unpinned
```

Poll routes (create under a separate poll handler or extend message handler):
```
POST   /v1/polls
GET    /v1/polls/:id
GET    /v1/polls/:id/options
POST   /v1/polls/:id/votes
DELETE /v1/polls/:id/votes/:option_id
GET    /v1/polls/:id/votes
GET    /v1/polls/:id/my-votes
POST   /v1/polls/:id/close
```

---

### 2.5 Call handler and service — nothing exists yet

Files to create:
```
internal/services/call_service.go
internal/services/call_models.go
internal/handler/call_handler.go
internal/transport/httpdto/call_dto.go
```

Routes to register (all protected):
```
POST   /v1/calls
GET    /v1/calls/:id
GET    /v1/calls
GET    /v1/calls/me
GET    /v1/calls/active
GET    /v1/calls/missed
POST   /v1/calls/:id/connect
POST   /v1/calls/:id/end
GET    /v1/calls/:id/duration
GET    /v1/calls/:id/participants
POST   /v1/calls/:id/participants
DELETE /v1/calls/:id/participants/:user_id
PATCH  /v1/calls/:id/participants/:user_id/status
PATCH  /v1/calls/:id/participants/:user_id/mute
GET    /v1/calls/:id/participants/active-count
```

Outbox events call service must emit:
```
call:created
call:connected
call:ended
call:participant_updated
```

WebRTC signaling (call:offer, call:answer, call:ice) should be published to Redis directly
without DB outbox, since they are ephemeral and latency-sensitive.

---

### 2.6 Command handler and service — nothing exists yet

Files to create:
```
internal/services/command_service.go
internal/services/command_models.go
internal/handler/command_handler.go
internal/transport/httpdto/command_dto.go
```

Routes to register (all protected):
```
GET  /v1/commands
GET  /v1/commands/:id
POST /v1/commands/:id/undo
```

Note: Command logs are written by other services (message, conversation) as part of their
transactions. The command handler only provides read access and undo capability.

Undo window: configurable via `COMMAND_UNDO_WINDOW_SECONDS` (default 300 = 5 minutes).

---

### 2.7 Outbox worker — does not exist yet

File to create: `internal/services/outbox_worker.go`

The worker is a long-running goroutine started in `cmd/api/main.go`. It:
1. Polls `outbox_events` WHERE `status='PENDING' AND retry_count < 10` every N seconds.
2. Marks each event `PROCESSING`.
3. Publishes to the appropriate Redis Pub/Sub channel.
4. Marks `COMPLETED` on success, increments retry + returns to `PENDING` on retryable failure.
5. Marks `FAILED` when retry budget exhausted.

See `docs/redis-outbox.md` for the full implementation including Go code.

---

### 2.8 WebSocket hub — does not exist yet

File to create: `internal/server/hub.go`

The hub:
1. Maintains a map of `user_id → []WebSocket connections`.
2. Subscribes to Redis Pub/Sub channels for each connected user's conversations.
3. Receives events from the outbox worker (via Redis) and fans them out to the correct connections.
4. Handles inbound frames: `ping`, `typing:start`, `typing:stop`, `read`, `delivered`.

WebSocket endpoint to register: `GET /v1/ws`

See `docs/websockets.md` for the full implementation.

---

### 2.9 Event type constants — do not exist yet

File to create: `internal/events/types.go`

```go
package events

const (
    // Message domain
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

    // Conversation domain
    ConversationCreated            = "conversation:created"
    ConversationUpdated            = "conversation:updated"
    ConversationParticipantAdded   = "conversation:participant_added"
    ConversationParticipantRemoved = "conversation:participant_removed"
    ConversationParticipantRoleUpdated = "conversation:participant_role_updated"
    ConversationMuted              = "conversation:muted"
    ConversationUnmuted            = "conversation:unmuted"
    ConversationArchived           = "conversation:archived"
    ConversationUnarchived         = "conversation:unarchived"
    ConversationCleared            = "conversation:cleared"
    ConversationInviteRegenerated  = "conversation:invite_regenerated"

    // Typing and presence
    TypingStarted  = "typing:started"
    TypingStopped  = "typing:stopped"
    PresenceOnline = "presence:online"
    PresenceOffline = "presence:offline"

    // Call domain
    CallCreated           = "call:created"
    CallRinging           = "call:ringing"
    CallOffer             = "call:offer"
    CallAnswer            = "call:answer"
    CallICE               = "call:ice"
    CallConnected         = "call:connected"
    CallParticipantUpdated = "call:participant_updated"
    CallEnded             = "call:ended"

    // Command domain
    CommandExecuted = "command:executed"
    CommandFailed   = "command:failed"
    CommandUndone   = "command:undone"
)
```

File to create: `internal/events/publisher.go`

```go
package events

import "context"

// Publisher is the interface for publishing events to Redis channels.
// The concrete implementation wraps the Redis Pub/Sub client.
type Publisher interface {
    Publish(ctx context.Context, channel string, payload []byte) error
}

// ChannelForConversation returns the Redis Pub/Sub channel name for a conversation.
func ChannelForConversation(conversationID string) string {
    return "channel:conversation:" + conversationID
}

// ChannelForUser returns the Redis Pub/Sub channel name for a user.
func ChannelForUser(userID string) string {
    return "channel:user:" + userID
}

// ChannelForCall returns the Redis Pub/Sub channel name for a call.
func ChannelForCall(callID string) string {
    return "channel:call:" + callID
}

// ChannelForPresence returns the Redis Pub/Sub channel name for user presence.
func ChannelForPresence(userID string) string {
    return "channel:presence:" + userID
}
```

---

## 3. Known bugs that must be fixed before proceeding

### 3.1 command.Status enum mismatch — MUST FIX

**Location:** `internal/domain/command/command.go`

**Problem:**

The Go constants do not match the SQL enum:

```go
// Go today — WRONG
const (
    StatusPending   Status = "PENDING"
    StatusExecuting Status = "EXECUTING"   // ❌ not in SQL
    StatusCompleted Status = "COMPLETED"   // ❌ not in SQL
    StatusFailed    Status = "FAILED"
    StatusUndone    Status = "UNDONE"
)
```

```sql
-- SQL enum — source of truth
CREATE TYPE command_status AS ENUM (
    'PENDING',
    'EXECUTED',   -- not COMPLETED
    'FAILED',
    'UNDONE'
    -- no EXECUTING
);
```

**Fix:** Update `command.go` to match SQL:

```go
const (
    StatusPending  Status = "PENDING"
    StatusExecuted Status = "EXECUTED"   // was COMPLETED
    StatusFailed   Status = "FAILED"
    StatusUndone   Status = "UNDONE"
    // Remove StatusExecuting entirely
)
```

Also update `internal/repository/command_repository.go` wherever `COMPLETED` or `EXECUTING`
are used as string literals. The `CanUndo` method currently checks for `COMPLETED` which will
never match the DB value `EXECUTED`.

---

### 3.2 call_participants.status uses invalid value — MUST FIX

**Location:** `internal/repository/call_repository.go`

**Problem:**

The call repository uses `JOINED` in at least one place, but the SQL enum is:

```sql
CREATE TYPE call_participant_status AS ENUM (
    'INVITED',
    'RINGING',
    'CONNECTED',
    'LEFT',
    'DECLINED'
);
```

`JOINED` is not a valid value and will cause a PostgreSQL error at runtime.

**Fix:** Replace every occurrence of `"JOINED"` in `call_repository.go` with `"CONNECTED"`.

---

## 4. Recommended implementation order

Follow this sequence. Each step depends on the previous.

### Step 1 — Fix the known bugs (30 minutes)

1. Fix `command.Status` constants in `internal/domain/command/command.go`.
2. Fix `JOINED` → `CONNECTED` in `internal/repository/call_repository.go`.
3. Run the existing test suite to confirm nothing broke.

### Step 2 — Redis infrastructure (1–2 days)

Create `internal/redis/` with:
- `client.go`: connection pool using `github.com/redis/go-redis/v9`.
- `pubsub.go`: `Publish(ctx, channel, payload)` and `Subscribe(ctx, channels...) *redis.PubSub`.
- `ratelimit.go`: sliding window counter using Redis `INCR` + `EXPIRE`.
  Wire the `RateLimitChecker` into `RateLimitMiddleware`, `MessageRateLimitMiddleware`, and
  `CallRateLimitMiddleware`.
- `presence.go`: `SetOnline(userID, ttl)`, `SetOffline(userID)`, `IsOnline(userID)`.
- `typing.go`: `SetTyping(conversationID, userID, ttl)`, `ClearTyping(...)`.

Create `internal/events/types.go` and `internal/events/publisher.go` as shown in §2.9.

### Step 3 — User service and handler (1 day)

Create `internal/services/user_service.go`, `user_models.go`, `internal/handler/user_handler.go`,
and `internal/transport/httpdto/user_dto.go`.

Register in `cmd/api/main.go`:
```go
userHandler.RegisterRoutes(protected)
```

### Step 4 — Conversation service and handler (1–2 days)

Create `conversation_service.go`, `conversation_models.go`, `conversation_handler.go`,
and `conversation_dto.go`.

Every write method (Create, Update, AddParticipant, etc.) must write an `outbox_events` row
in the same DB transaction using `OutboxRepository.Create(ctx, tx, event)`.

### Step 5 — Message service and handler (2–3 days)

Create `message_service.go`, `message_models.go`, `message_handler.go`, and `message_dto.go`.

Implement idempotency: check `GetByClientMessageID` before inserting.
Do not set `seq_id` — it is assigned by the DB trigger.
Every write (send, edit, delete, receipt, reaction, star, pin) must emit an outbox event.

### Step 6 — Outbox worker (1 day)

Create
