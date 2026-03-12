# Sentinal Chat — Backend API

Real-time chat backend written in Go. Supports end-to-end encrypted messaging, conversations, file uploads, P2P/group calls, and WebSocket delivery via a Redis-backed outbox.

---

## Table of Contents

1. [Current Implementation Status](#current-implementation-status)
2. [Tech Stack](#tech-stack)
3. [Project Layout](#project-layout)
4. [Architecture](#architecture)
5. [Domain Models](#domain-models)
6. [Configuration](#configuration)
7. [Running Locally](#running-locally)
8. [Migrations and Seeding](#migrations-and-seeding)
9. [Auth System](#auth-system)
10. [HTTP API Overview](#http-api-overview)
11. [WebSocket Overview](#websocket-overview)
12. [Outbox and Real-time Delivery](#outbox-and-real-time-delivery)
13. [Rate Limiting](#rate-limiting)
14. [Error Handling](#error-handling)
15. [Implementation Roadmap](#implementation-roadmap)

---

## Current Implementation Status

### ✅ Built and wired

| Area | Files |
|---|---|
| HTTP server bootstrap | `cmd/api/main.go`, `internal/server/server.go` |
| Config loading | `config/config.go` |
| Auth handler | `internal/handler/auth_handler.go` |
| Auth service | `internal/services/auth_service.go` |
| Token service (JWT + refresh) | `internal/services/token_service.go` |
| OAuth providers (Google, GitHub) | `internal/services/oauth_google.go`, `oauth_github.go` |
| Upload handler | `internal/handler/upload_handler.go` |
| Upload service | `internal/services/upload_service.go` |
| S3 storage client | `internal/storage/s3_client.go` |
| Auth middleware | `internal/middleware/auth_middleware.go` |
| CORS middleware | `internal/middleware/cors_middleware.go` |
| Logging middleware | `internal/middleware/logging_middleware.go` |
| Error handler middleware | `internal/middleware/error_middleware.go` |
| Rate limit middleware | `internal/middleware/ratelimit_middleware.go` |
| Request ID middleware | `internal/middleware/request_id_middleware.go` |
| All domain models | `internal/domain/*/entity.go` |
| All repository interfaces | `internal/repository/interfaces.go` |
| All repository implementations | `internal/repository/*.go` |
| HTTP DTO layer | `internal/transport/httpdto/` |
| Utility routes | `GET /ping`, `GET /health`, `GET /goroutines` |
| Auth routes | `POST /v1/auth/register`, `login`, `refresh`, `logout`, `logout-all`, `GET sessions`, `oauth` |
| Upload routes | `POST /v1/uploads`, `POST /v1/uploads/bulk`, attachment routes |
| SQL migrations | `migrations/` |

### ❌ Not yet built

| Area | What's needed |
|---|---|
| User handler + service | `GET/PUT /v1/users/me`, contacts, devices, push tokens |
| Conversation handler + service | Full CRUD, participants, mute, archive, pin, clear |
| Message handler + service | Send, list, edit, delete, receipts, reactions, stars, pins, polls |
| Call handler + service | Create, connect, end, participants, quality metrics |
| Command handler + service | Command log, undo, outbox emission |
| Redis client | `internal/redis/client.go` |
| Redis Pub/Sub publisher | `internal/redis/pubsub.go` |
| Outbox worker | `internal/services/outbox_worker.go` |
| Event types | `internal/events/types.go` |
| WebSocket hub | `internal/server/hub.go` |
| WebSocket handler | `GET /v1/ws` |

---

## Tech Stack

| Component | Technology |
|---|---|
| Language | Go 1.22+ |
| HTTP framework | [Gin](https://github.com/gin-gonic/gin) |
| ORM / data access | [GORM](https://gorm.io) + raw `database/sql` |
| Database | PostgreSQL 15+ |
| Cache / Pub-Sub / Rate limit | Redis 7+ |
| Real-time transport | Gorilla WebSocket |
| JWT | `github.com/golang-jwt/jwt/v5` |
| Logging | Uber Zap (wrapped in `pkg/logger`) |
| Object storage | AWS S3 (or compatible) |
| Auth | JWT access tokens + SHA-256 hashed refresh tokens |
| OAuth | Google PKCE, GitHub PKCE |

---

## Project Layout

```
sentinal-chat/
├── cmd/
│   ├── api/
│   │   └── main.go              # HTTP server entry point
│   └── migrate/
│       └── main.go              # Migration + seed CLI
├── config/
│   └── config.go                # Env-based config loading
├── internal/
│   ├── domain/                  # Pure domain structs (no DB tags)
│   │   ├── call/entity.go
│   │   ├── command/command.go
│   │   ├── conversation/entity.go
│   │   ├── message/entity.go
│   │   ├── message/attachments.go
│   │   ├── message/polls.go
│   │   ├── outbox/outbox.go
│   │   ├── upload/entity.go
│   │   └── user/entity.go
│   ├── handler/                 # Gin HTTP handlers
│   │   ├── auth_handler.go      ✅
│   │   ├── upload_handler.go    ✅
│   │   ├── user_handler.go      ❌ TODO
│   │   ├── conversation_handler.go ❌ TODO
│   │   ├── message_handler.go   ❌ TODO
│   │   ├── call_handler.go      ❌ TODO
│   │   └── command_handler.go   ❌ TODO
│   ├── middleware/
│   │   ├── auth_middleware.go   ✅
│   │   ├── cors_middleware.go   ✅
│   │   ├── error_middleware.go  ✅
│   │   ├── logging_middleware.go ✅
│   │   ├── ratelimit_middleware.go ✅
│   │   └── request_id_middleware.go ✅
│   ├── repository/
│   │   ├── interfaces.go        ✅  All repo interfaces
│   │   ├── user_repository.go   ✅
│   │   ├── conversation_repository.go ✅
│   │   ├── message_repository.go ✅
│   │   ├── call_repository.go   ✅
│   │   ├── upload_repository.go ✅
│   │   ├── outbox_repository.go ✅
│   │   ├── command_repository.go ✅
│   │   ├── oauth_identity_repository.go ✅
│   │   └── sql_helpers.go       ✅
│   ├── server/
│   │   ├── server.go            ✅  Gin engine, graceful shutdown
│   │   └── hub.go               ❌  TODO: WebSocket hub
│   ├── services/
│   │   ├── auth_service.go      ✅
│   │   ├── auth_models.go       ✅
│   │   ├── token_service.go     ✅
│   │   ├── upload_service.go    ✅
│   │   ├── oauth_google.go      ✅
│   │   ├── oauth_github.go      ✅
│   │   ├── user_service.go      ❌ TODO
│   │   ├── conversation_service.go ❌ TODO
│   │   ├── message_service.go   ❌ TODO
│   │   ├── call_service.go      ❌ TODO
│   │   ├── command_service.go   ❌ TODO
│   │   └── outbox_worker.go     ❌ TODO
│   ├── storage/
│   │   └── s3_client.go         ✅
│   └── transport/
│       └── httpdto/
│           ├── response.go      ✅
│           ├── auth_dto.go      ✅
│           ├── upload_dto.go    ✅
│           ├── user_dto.go      ❌ TODO
│           ├── conversation_dto.go ❌ TODO
│           ├── message_dto.go   ❌ TODO
│           └── call_dto.go      ❌ TODO
├── migrations/
│   └── *.sql
├── pkg/
│   ├── database/                # DB connection helpers
│   ├── errors/                  # Sentinel errors
│   └── logger/                  # Zap wrapper
├── docs/
│   ├── api-endpoints.md         # Full HTTP contract
│   ├── websockets.md            # WebSocket contract + Go impl guide
│   ├── redis-outbox.md          # Outbox worker + Redis impl guide
│   ├── commands.md              # Command pattern impl guide
│   └── backend-audit-overview.md
├── docker-compose.yml
├── Makefile
└── go.mod
```

---

## Architecture

```
 HTTP Client / WS Client
        │
        ▼
 ┌─────────────────────────────────────────────────────┐
 │  Gin Router  (internal/server/server.go)            │
 │                                                     │
 │  Middleware chain (per request):                    │
 │   RequestID → CORS → Logging → Auth → RateLimit     │
 └──────────────────────┬──────────────────────────────┘
                        │
          ┌─────────────┴──────────────┐
          │                            │
          ▼                            ▼
   HTTP Handlers                  WS Hub  ← TODO
   (internal/handler)          (internal/server/hub.go)
          │                            ▲
          ▼                            │ Redis Pub/Sub
   Services Layer              ┌───────┴────────┐
   (internal/services)         │  Outbox Worker │  ← TODO
          │                    │  (polls DB)    │
          ├── DB writes         └───────┬────────┘
          │   (via repository)          │ publishes
          │                            ▼
          └──────────────────► Redis Pub/Sub Channels
                                channel:conversation:{id}
                                channel:user:{id}
                                channel:call:{id}

 ┌──────────────────────────────────────────────────────┐
 │  PostgreSQL                                          │
 │  users, devices, sessions, conversations,            │
 │  participants, messages, receipts, reactions,        │
 │  calls, uploads, attachments, polls, commands,       │
 │  outbox_events                                       │
 └──────────────────────────────────────────────────────┘
```

### Data flow for a new message (target state)

```
Client POST /v1/messages
    │
    ▼
MessageHandler.Send()
    │
    ▼
MessageService.Send(ctx, input)
    │  opens DB transaction
    ├─► messages.INSERT
    ├─► message_mentions.INSERT (if any)
    ├─► outbox_events.INSERT  { event_type: "message:new" }
    │  commit
    │
    ▼
OutboxWorker (background goroutine)
    │  polls outbox_events WHERE status='PENDING'
    ├─► Redis PUBLISH channel:conversation:{id}  { envelope }
    └─► outbox_events.UPDATE status='COMPLETED'

Redis Subscriber (WS Hub)
    │  receives published envelope
    └─► fans out to all active WS connections for that conversation
```

---

## Domain Models

All domain structs live in `internal/domain/`. They have no GORM or JSON tags — they are pure Go.

### `user.User`

```go
type User struct {
    ID           uuid.UUID
    PhoneNumber  sql.NullString
    Username     sql.NullString
    Email        sql.NullString
    PasswordHash string
    DisplayName  string
    Bio          string
    AvatarURL    string
    IsOnline     bool
    LastSeenAt   sql.NullTime
    IsActive     bool
    IsVerified   bool
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### `user.Device`

```go
type Device struct {
    ID           uuid.UUID
    UserID       uuid.UUID
    DeviceID     string   // client-provided fingerprint
    DeviceName   string
    DeviceType   string   // ios | android | web | desktop
    IsActive     bool
    RegisteredAt time.Time
    LastSeenAt   sql.NullTime
}
```

### `user.UserSession`

```go
type UserSession struct {
    ID               uuid.UUID
    UserID           uuid.UUID
    DeviceID         *uuid.UUID
    RefreshTokenHash string   // SHA-256 of the raw refresh token
    ExpiresAt        time.Time
    IsRevoked        bool
    AuthProvider     string   // password | google | github
    CreatedAt        time.Time
}
```

### `conversation.Conversation`

```go
type Conversation struct {
    ID               uuid.UUID
    Type             string          // DM | GROUP
    Subject          sql.NullString
    Description      sql.NullString
    AvatarURL        sql.NullString
    InviteLink       sql.NullString
    DMUserIDA        uuid.NullUUID
    DMUserIDB        uuid.NullUUID
    DisappearingMode string          // OFF | AFTER_24H | AFTER_7D | AFTER_90D
    CreatedBy        uuid.NullUUID
    CreatedAt        time.Time
    UpdatedAt        time.Time
    LastMessageAt    *time.Time      // computed via subquery
    Participants     []Participant
}
```

### `message.Message`

```go
type Message struct {
    ID               uuid.UUID
    ConversationID   uuid.UUID
    SenderID         uuid.UUID
    ClientMessageID  sql.NullString
    SeqID            sql.NullInt64   // set by Postgres trigger on INSERT
    Type             string          // TEXT | IMAGE | VIDEO | AUDIO | FILE | POLL | CALL
    EncryptedContent sql.NullString  // base64 E2EE ciphertext
    IsForwarded      bool
    ReplyToMsgID     uuid.NullUUID
    PollID           uuid.NullUUID
    MentionCount     int
    CreatedAt        time.Time
    EditedAt         sql.NullTime
    DeletedAt        sql.NullTime
    ExpiresAt        sql.NullTime
}
```

### `outbox.OutboxEvent`

```go
type OutboxEvent struct {
    ID            uuid.UUID
    EventType     string    // "message:new", "message:read", etc.
    AggregateType string    // "message", "conversation", "call", "command"
    AggregateID   string
    Payload       []byte    // JSON
    Status        Status    // PENDING | PROCESSING | COMPLETED | FAILED
    RetryCount    int
    Error         string
    CreatedAt     time.Time
    UpdatedAt     time.Time
    ProcessedAt   *time.Time
}
```

### `command.CommandLog`

```go
type CommandLog struct {
    ID              uuid.UUID
    CommandType     string          // DELETE_MESSAGE | EDIT_MESSAGE | PIN_MESSAGE | ...
    UserID          uuid.UUID
    ConversationID  *uuid.UUID
    Status          Status          // PENDING | EXECUTED | FAILED | UNDONE
    Payload         json.RawMessage
    UndoPayload     json.RawMessage
    ErrorMessage    string
    ExecutionTimeMs int
    CreatedAt       time.Time
    ExecutedAt      *time.Time
    UndoneAt        *time.Time
}
```

---

## Configuration

All config is read from environment variables. The app loads a `.env` file if present.

```env
# Server
APP_PORT=8080
APP_MODE=debug          # debug | release | test
FRONTEND_URL=http://localhost:3000

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=sentinal_chat

# JWT
JWT_SECRET=change-me-to-a-long-random-string
JWT_EXPIRY_HOURS=12
REFRESH_EXPIRY_DAYS=14

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# S3 / Object Storage
S3_BUCKET=sentinal-chat-uploads
S3_REGION=us-east-1
AWS_ACCESS_KEY_ID=
AWS_SECRET_ACCESS_KEY=

# OAuth (optional)
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=

# Cookie
COOKIE_DOMAIN=localhost
COOKIE_SECURE=false

# PgAdmin (docker-compose only)
PGADMIN_EMAIL=admin@sentinal.chat
PGADMIN_PASSWORD=Admin@123!
```

---

## Running Locally

### 1. Start infrastructure

```bash
make up
# or
docker-compose up -d
```

This starts PostgreSQL and Redis.

### 2. Run migrations

```bash
make migrate-up
# or
go run cmd/migrate/main.go up
```

### 3. Start the API

```bash
make run
# or
go run cmd/api/main.go
```

The server listens on `APP_PORT` (default `8080`).

### 4. Verify

```bash
curl http://localhost:8080/ping
# {"success":true,"data":{"message":"pong"}}

curl http://localhost:8080/health
# {"success":true,"data":{"status":"healthy"}}
```

---

## Migrations and Seeding

The API auto-runs SQL migrations on startup. You can also use the CLI tool:

```bash
go run cmd/migrate/main.go up          # apply all pending migrations
go run cmd/migrate/main.go down        # roll back one migration
go run cmd/migrate/main.go status      # show applied / pending migrations
go run cmd/migrate/main.go seed        # seed with production-safe reference data
go run cmd/migrate/main.go seed-dev    # seed with dev/test demo data
go run cmd/migrate/main.go reset       # drop all tables and re-run migrations
go run cmd/migrate/main.go truncate    # truncate all tables (keep schema)
```

Make targets:

```bash
make migrate-up
make migrate-down
make migrate-status
make migrate-seed
make migrate-seed-dev
make migrate-reset
make migrate-truncate
```

---

## Auth System

### Token model

| Token | Storage | TTL | Transport |
|---|---|---|---|
| Access token | JWT (stateless) | `JWT_EXPIRY_HOURS` | `Authorization: Bearer <token>` |
| Refresh token | SHA-256 hash stored in `user_sessions` | `REFRESH_EXPIRY_DAYS` | `HttpOnly` cookie **or** response body |

### JWT claims

```go
type AccessTokenClaims struct {
    UserID    string `json:"user_id"`
    SessionID string `json:"session_id"`
    DeviceID  string `json:"device_id,omitempty"`
    jwt.RegisteredClaims
}
```

### Auth middleware

`AuthMiddleware` in `internal/middleware/auth_middleware.go` injects three values into the Gin context for every protected route:

```go
c.Set("user_id",    userID)      // uuid.UUID
c.Set("session_id", sessionID)   // string (UUID)
c.Set("device_id",  deviceID)    // string
```

Handlers retrieve them like this:

```go
userIDVal, _ := c.Get("user_id")
userID := userIDVal.(uuid.UUID)
```

### Session validation

Every request to a protected endpoint:
1. Parses and verifies the JWT signature and expiry.
2. Loads the `user_sessions` row to confirm the session is not revoked.
3. If `device_id` is present in claims, verifies the device is still active and belongs to the same user.
4. Updates `devices.last_seen_at` on every valid authenticated request.

### OAuth flow

```
Client                      API                      Provider (Google/GitHub)
  │                          │                               │
  │  GET /v1/auth/oauth      │                               │
  │  /:provider/url          │                               │
  │ ?code_challenge=...      │                               │
  │─────────────────────────►│                               │
  │◄─────────────────────────│                               │
  │  { authorization_url }   │                               │
  │                          │                               │
  │  (user visits URL,       │                               │
  │   approves in browser)   │──── PKCE exchange ───────────►│
  │                          │◄─────────────────────────────│
  │                          │   { access_token, profile }  │
  │  POST /v1/auth/oauth     │                               │
  │  /:provider/exchange     │                               │
  │  { code, code_verifier } │                               │
  │─────────────────────────►│                               │
  │◄─────────────────────────│                               │
  │  { access_token, user }  │                               │
```

---

## HTTP API Overview

### Response envelope

Every response uses a consistent wrapper:

```json
// success
{ "success": true, "data": {} }

// error
{ "success": false, "error": "message", "code": "ERROR_CODE" }
```

Implemented in `internal/transport/httpdto/response.go`:

```go
type Response[T any] struct {
    Success bool   `json:"success"`
    Data    T      `json:"data,omitempty"`
    Error   string `json:"error,omitempty"`
    Code    string `json:"code,omitempty"`
}

func WriteSuccess[T any](c *gin.Context, status int, data T) {
    c.JSON(status, Response[T]{Success: true, Data: data})
}

func WriteError(c *gin.Context, status int, message, code string) {
    c.JSON(status, Response[any]{Success: false, Error: message, Code: code})
}
```

### Route groups

```
GET  /ping
GET  /health
GET  /goroutines

POST /v1/auth/register              ✅
POST /v1/auth/login                 ✅
POST /v1/auth/refresh               ✅
GET  /v1/auth/oauth/:provider/url   ✅
POST /v1/auth/oauth/:provider/exchange ✅
POST /v1/auth/logout                ✅ (protected)
POST /v1/auth/logout-all            ✅ (protected)
GET  /v1/auth/sessions              ✅ (protected)

GET  /v1/users/me                   ❌ TODO
PUT  /v1/users/me                   ❌ TODO
GET  /v1/users/:id                  ❌ TODO
GET  /v1/users/search               ❌ TODO
GET  /v1/users/contacts             ❌ TODO
POST /v1/users/contacts             ❌ TODO
...

POST /v1/conversations              ❌ TODO
GET  /v1/conversations              ❌ TODO
...

POST /v1/messages                   ❌ TODO
GET  /v1/messages                   ❌ TODO
...

POST /v1/uploads                    ✅ multipart single upload
POST /v1/uploads/bulk               ✅ multipart bulk upload
POST /v1/attachments                ✅
GET  /v1/attachments/:id            ✅
POST /v1/attachments/:id/viewed     ✅
GET  /v1/messages/:id/attachments   ✅

POST /v1/calls                      ❌ TODO
...

GET  /v1/ws                         ❌ TODO WebSocket
```

See `docs/api-endpoints.md` for the full contract of every endpoint.

### Upload flow

The upload API is now intentionally simple:

1. `POST /v1/uploads` with `multipart/form-data` field `file` for one real file upload.
2. `POST /v1/uploads/bulk` with repeated `files` fields for bulk uploads.
3. Use the returned `file_url`, `filename`, `mime_type`, and `size_bytes` when calling `POST /v1/attachments`.

Example attachment body:

```json
{
  "message_id": "uuid-optional",
  "file_url": "https://cdn.example.com/uploads/user/file.png",
  "filename": "file.png",
  "mime_type": "image/png",
  "size_bytes": 1024,
  "view_once": false
}
```

---

## WebSocket Overview

**Endpoint:** `GET /v1/ws?token=<access_token>`  
**Status:** Not yet built.

Once built, the hub will:
- Authenticate via JWT token in query param or `Authorization` header.
- Subscribe the connection to all conversation channels the user belongs to.
- Forward Redis Pub/Sub messages to connected sockets.
- Handle inbound frames: `ping`, `typing:start`, `typing:stop`, `read`, `delivered`.

See `docs/websockets.md` for the full contract and implementation guide.

---

## Outbox and Real-time Delivery

The project uses the **transactional outbox pattern** for reliable event delivery.

1. Every state-changing service writes domain data **and** an `outbox_events` row in the same DB transaction.
2. An `OutboxWorker` goroutine polls `outbox_events` for `PENDING` rows, publishes them to Redis Pub/Sub, and marks them `COMPLETED`.
3. The WebSocket hub subscribes to Redis channels and fans events out to connected clients.

**Status:** The `outbox_events` table and `OutboxRepository` exist. The Redis client and worker goroutine still need to be written.

See `docs/redis-outbox.md` for the full implementation guide including Go code.

---

## Rate Limiting

Three rate limit middleware functions exist in `internal/middleware/ratelimit_middleware.go`:

| Middleware | Key | Applied to |
|---|---|---|
| `RateLimitMiddleware` | Client IP | Auth endpoints (`/v1/auth/login`, `/register`, `/refresh`, oauth) |
| `MessageRateLimitMiddleware` | `user_id` | Message send endpoints |
| `CallRateLimitMiddleware` | `user_id` | Call creation endpoints |

Headers returned on every rate-limited response:

```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
X-RateLimit-Reset: 60
```

The `RateLimitChecker` function type is injected — the Redis sliding window implementation will live in `internal/redis/ratelimit.go`.

---

## Error Handling

### Sentinel errors (`pkg/errors`)

| Error | HTTP Status | Code |
|---|---|---|
| `ErrInvalidInput` | 400 | `INVALID_INPUT` |
| `ErrUnauthorized` | 401 | `UNAUTHORIZED` |
| `ErrForbidden` | 403 | `FORBIDDEN` |
| `ErrNotFound` | 404 | `NOT_FOUND` |
| `ErrAlreadyExists` | 409 | `ALREADY_EXISTS` |
| `ErrConflict` | 409 | `CONFLICT` |
| `ErrInvalidTransition` | 409 | `CONFLICT` |
| `ErrTooLarge` | 413 | `TOO_LARGE` |
| `ErrServiceUnavailable` | 503 | `SERVICE_UNAVAILABLE` |

### Handler pattern

Every handler has a private `writeError(c *gin.Context, err error)` method that maps sentinel errors to HTTP status codes. Example from `auth_handler.go`:

```go
func (h *AuthHandler) writeError(c *gin.Context, err error) {
    status := http.StatusInternalServerError
    code := "INTERNAL_ERROR"
    message := "internal server error"

    switch {
    case errors.Is(err, sentinal_errors.ErrInvalidInput):
        status = http.StatusBadRequest
        code = "AUTH_INVALID_INPUT"
        message = "invalid input"
    case errors.Is(err, services.ErrInvalidCredentials):
        status = http.StatusUnauthorized
        code = "AUTH_INVALID_CREDENTIALS"
        message = "invalid credentials"
    // ...
    }

    httpdto.WriteError(c, status, message, code)
}
```

---

## Implementation Roadmap

Follow this order to build out the remaining surface area efficiently.

### Phase 1 — Redis infrastructure

Build these first. Every subsequent phase depends on them.

```
internal/redis/client.go          Redis connection pool
internal/redis/pubsub.go          Publish + Subscribe helpers
internal/redis/ratelimit.go       Sliding window rate limiter (wire into middleware)
internal/redis/presence.go        Online/offline TTL keys
internal/redis/typing.go          Typing TTL keys
```

### Phase 2 — User and conversation services

```
internal/services/user_service.go
internal/handler/user_handler.go
internal/transport/httpdto/user_dto.go

internal/services/conversation_service.go
internal/handler/conversation_handler.go
internal/transport/httpdto/conversation_dto.go
```

### Phase 3 — Messaging

```
internal/services/message_service.go      (includes outbox write in same tx)
internal/handler/message_handler.go
internal/transport/httpdto/message_dto.go
```

### Phase 4 — Outbox worker + WebSocket hub

```
internal/events/types.go                  Canonical event type constants
internal/events/publisher.go              Thin wrapper over redis pubsub
internal/services/outbox_worker.go        Background polling goroutine
internal/server/hub.go                    WS connection registry + fan-out
```

Wire `hub.go` into `cmd/api/main.go` and start the outbox worker as a goroutine alongside the HTTP server.

### Phase 5 — Calls

```
internal/services/call_service.go
internal/handler/call_handler.go
internal/transport/httpdto/call_dto.go
```

### Phase 6 — Commands

Fix the `command.Status` enum mismatch (Go has `EXECUTING`/`COMPLETED`; SQL has `EXECUTED`), then build:

```
internal/services/command_service.go
internal/handler/command_handler.go
internal/transport/httpdto/command_dto.go
```

See `docs/commands.md` for full payload shapes and undo logic.

### Phase 7 — Polls and broadcasts

```
internal/services/poll_service.go
internal/handler/poll_handler.go
```

---

## Known Issues to Fix Before Going Further

| Issue | Location | Fix |
|---|---|---|
| `command.Status` Go constants (`EXECUTING`, `COMPLETED`) do not match SQL enum (`EXECUTED`) | `internal/domain/command/command.go` | Remove `EXECUTING`/
