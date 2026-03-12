# Agents Guide

This document is the authoritative implementation guide for every engineer (human or AI) working on this codebase.
It is grounded in the actual code that exists today and gives exact, copy-paste-ready skeletons for every pattern
used in the project. Read this before writing any new file.

---

## Table of Contents

1. [Codebase Structure Map](#1-codebase-structure-map)
2. [Architecture Overview](#2-architecture-overview)
3. [Conventions That Must Never Break](#3-conventions-that-must-never-break)
4. [Pattern: Repository](#4-pattern-repository)
5. [Pattern: Service](#5-pattern-service)
6. [Pattern: Handler](#6-pattern-handler)
7. [Pattern: DTO](#7-pattern-dto)
8. [Pattern: Middleware](#8-pattern-middleware)
9. [Pattern: Transaction Management](#9-pattern-transaction-management)
10. [Pattern: Outbox Write](#10-pattern-outbox-write)
11. [Pattern: Redis Client and Pub/Sub](#11-pattern-redis-client-and-pubsub)
12. [Pattern: WebSocket Hub](#12-pattern-websocket-hub)
13. [Pattern: Command Pattern](#13-pattern-command-pattern)
14. [Wiring a New Feature End-to-End](#14-wiring-a-new-feature-end-to-end)
15. [Error Handling Reference](#15-error-handling-reference)
16. [Known Bugs That Must Be Fixed](#16-known-bugs-that-must-be-fixed)
17. [Build Order Roadmap](#17-build-order-roadmap)

---

## 1. Codebase Structure Map

```
sentinal-chat/
├── cmd/
│   ├── api/main.go                     # HTTP server entry point
│   └── migrate/main.go                 # Migration + seed CLI
├── config/config.go                    # Env-var config loading
├── internal/
│   ├── domain/                         # Pure Go structs — NO tags, NO imports from other internal packages
│   │   ├── call/entity.go              ✅ Call, CallParticipant
│   │   ├── command/command.go          ✅ CommandLog, Status consts (has bug — see §16)
│   │   ├── conversation/entity.go      ✅ Conversation, Participant, ConversationSequence, ConversationClear
│   │   ├── message/entity.go           ✅ Message, MessageReaction, MessageReceipt, MessageMention, StarredMessage, PinnedMessage, MessageEdit
│   │   ├── message/attachments.go      ✅ Attachment, MessageAttachment
│   │   ├── message/polls.go            ✅ Poll, PollOption, PollVote
│   │   ├── outbox/outbox.go            ✅ OutboxEvent, Status consts
│   │   └── user/entity.go              ✅ User, Device, FcmToken, UserSession, UserContact
│   ├── handler/
│   │   ├── auth_handler.go             ✅ Register, Login, Refresh, OAuth, Logout, Sessions
│   │   ├── upload_handler.go           ✅ Direct multipart uploads, Attachments
│   │   ├── user_handler.go             ❌ TODO
│   │   ├── conversation_handler.go     ❌ TODO
│   │   ├── message_handler.go          ❌ TODO
│   │   ├── call_handler.go             ❌ TODO
│   │   └── command_handler.go          ❌ TODO
│   ├── middleware/
│   │   ├── auth_middleware.go          ✅ JWT parse + session validate
│   │   ├── cors_middleware.go          ✅
│   │   ├── error_middleware.go         ✅
│   │   ├── logging_middleware.go       ✅
│   │   ├── ratelimit_middleware.go     ✅ (checker fn injected — Redis impl still needed)
│   │   └── request_id_middleware.go    ✅
│   ├── repository/
│   │   ├── interfaces.go               ✅ All repository interfaces
│   │   ├── user_repository.go          ✅
│   │   ├── conversation_repository.go  ✅
│   │   ├── message_repository.go       ✅
│   │   ├── call_repository.go          ✅
│   │   ├── upload_repository.go        ✅
│   │   ├── outbox_repository.go        ✅
│   │   ├── command_repository.go       ✅
│   │   ├── oauth_identity_repository.go ✅
│   │   └── sql_helpers.go              ✅
│   ├── server/
│   │   ├── server.go                   ✅ Gin engine, middleware chain, graceful shutdown
│   │   └── hub.go                      ❌ TODO WebSocket hub
│   ├── services/
│   │   ├── auth_service.go             ✅
│   │   ├── auth_models.go              ✅
│   │   ├── token_service.go            ✅
│   │   ├── upload_service.go           ✅
│   │   ├── oauth_google.go             ✅
│   │   ├── oauth_github.go             ✅
│   │   ├── user_service.go             ❌ TODO
│   │   ├── conversation_service.go     ❌ TODO
│   │   ├── message_service.go          ❌ TODO
│   │   ├── call_service.go             ❌ TODO
│   │   ├── command_service.go          ❌ TODO
│   │   └── outbox_worker.go            ❌ TODO
│   ├── storage/s3_client.go            ✅
│   └── transport/httpdto/
│       ├── response.go                 ✅ Response[T], WriteSuccess, WriteError
│       ├── auth_dto.go                 ✅
│       ├── upload_dto.go               ✅
│       ├── user_dto.go                 ❌ TODO
│       ├── conversation_dto.go         ❌ TODO
│       ├── message_dto.go              ❌ TODO
│       └── call_dto.go                 ❌ TODO
├── pkg/
│   ├── database/                       ✅ DB connection, HealthCheck, GenerateSecureToken
│   ├── errors/                         ✅ Sentinel error values
│   └── logger/                         ✅ Zap wrapper
└── migrations/*.sql                    ✅ All schema
```

---

## 2. Architecture Overview

```
HTTP Request
     │
     ▼
Gin Router  (internal/server/server.go)
     │
     ├── Middleware: RequestID → CORS → Logging → ErrorHandler
     │
     ├── Public group  (no auth)
     │       └── Handler (internal/handler/)
     │               └── calls Service (internal/services/)
     │                       └── calls Repository (internal/repository/)
     │                               └── PostgreSQL
     │
     └── Protected group  (auth middleware applied)
             └── Handler
                     └── Service
                             ├── Repository  → PostgreSQL
                             └── OutboxRepo  → outbox_events (same tx)
                                                   ▲
                                    OutboxWorker polls │
                                                   ▼
                                              Redis PUBLISH
                                                   ▲
                                       WebSocket Hub subscribes
                                                   ▼
                                        Connected WS clients
```

**The single most important rule:**
> Every state-changing service method must write domain data AND an `outbox_events` row inside the **same** database transaction. Never publish directly to Redis from a handler or service.

---

## 3. Conventions That Must Never Break

### 3.1 Response shape

Always use `httpdto.WriteSuccess` and `httpdto.WriteError`. Never call `c.JSON` directly from a handler for domain responses.

```go
// Success — 200 or 201
httpdto.WriteSuccess(c, http.StatusOK, myPayload)
httpdto.WriteSuccess(c, http.StatusCreated, myPayload)

// Error
httpdto.WriteError(c, http.StatusBadRequest, "invalid input", "INVALID_INPUT")
```

### 3.2 DTOs live in httpdto

- Request structs: `*Request` suffix, use `binding` tags.
- Response payload structs: `*Payload` suffix, use `json` tags.
- Never use domain structs as JSON responses.

### 3.3 Services return domain views, not domain entities

Define a `*View` or `*Result` struct in the service models file. Never return `domain.User` directly from a service — the caller should not need to know about `sql.NullString`.

### 3.4 Handler has a private writeError method

Copy the exact error mapping pattern from `auth_handler.go`. Each handler handles its own domain errors + the shared sentinel errors.

### 3.5 Context propagation

Always accept and forward `context.Context` through every layer. Never use `context.Background()` inside a request path.

### 3.6 UUIDs

Use `github.com/google/uuid`. Parse with `uuid.Parse(strings.TrimSpace(raw))`. Nil-check with `== uuid.Nil`.

---

## 4. Pattern: Repository

Repositories implement the interfaces in `internal/repository/interfaces.go`. They use raw `database/sql` via a `DBTX` interface (either `*sql.DB` or `*sql.Tx`).

### 4.1 The DBTX interface

```go
// Already defined in internal/repository/sql_helpers.go
type DBTX interface {
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
```

### 4.2 Skeleton: new repository file

Create `internal/repository/conversation_repository.go`:

```go
package repository

import (
    "context"
    "database/sql"
    "errors"
    "time"

    "github.com/google/uuid"

    "sentinal-chat/internal/domain/conversation"
    sentinal_errors "sentinal-chat/pkg/errors"
)

type conversationRepository struct {
    db DBTX
}

func NewConversationRepository(db DBTX) ConversationRepository {
    return &conversationRepository{db: db}
}

func (r *conversationRepository) Create(ctx context.Context, c *conversation.Conversation) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO conversations
            (id, type, subject, description, avatar_url, invite_link,
             dm_user_id_a, dm_user_id_b, disappearing_mode, created_by, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
    `,
        c.ID, c.Type, c.Subject, c.Description, c.AvatarURL, c.InviteLink,
        c.DMUserIDA, c.DMUserIDB, c.DisappearingMode, c.CreatedBy,
        c.CreatedAt, c.UpdatedAt,
    )
    return err
}

func (r *conversationRepository) GetByID(ctx context.Context, id uuid.UUID) (conversation.Conversation, error) {
    row := r.db.QueryRowContext(ctx, `
        SELECT id, type, subject, description, avatar_url, invite_link,
               dm_user_id_a, dm_user_id_b, disappearing_mode, created_by, created_at, updated_at
        FROM conversations
        WHERE id = $1
    `, id)

    var c conversation.Conversation
    err := row.Scan(
        &c.ID, &c.Type, &c.Subject, &c.Description, &c.AvatarURL, &c.InviteLink,
        &c.DMUserIDA, &c.DMUserIDB, &c.DisappearingMode, &c.CreatedBy,
        &c.CreatedAt, &c.UpdatedAt,
    )
    if errors.Is(err, sql.ErrNoRows) {
        return conversation.Conversation{}, sentinal_errors.ErrNotFound
    }
    return c, err
}

func (r *conversationRepository) Update(ctx context.Context, c conversation.Conversation) error {
    _, err := r.db.ExecContext(ctx, `
        UPDATE conversations
        SET subject = $1, description = $2, avatar_url = $3,
            disappearing_mode = $4, updated_at = $5
        WHERE id = $6
    `, c.Subject, c.Description, c.AvatarURL, c.DisappearingMode, time.Now(), c.ID)
    return err
}

func (r *conversationRepository) Delete(ctx context.Context, id uuid.UUID) error {
    _, err := r.db.ExecContext(ctx, `DELETE FROM conversations WHERE id = $1`, id)
    return err
}

// --- participants ---

func (r *conversationRepository) AddParticipant(ctx context.Context, p *conversation.Participant) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO participants (conversation_id, user_id, role, joined_at, added_by)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (conversation_id, user_id) DO NOTHING
    `, p.ConversationID, p.UserID, p.Role, p.JoinedAt, p.AddedBy)
    return err
}

func (r *conversationRepository) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
    var exists bool
    err := r.db.QueryRowContext(ctx, `
        SELECT EXISTS(
            SELECT 1 FROM participants
            WHERE conversation_id = $1 AND user_id = $2
        )
    `, conversationID, userID).Scan(&exists)
    return exists, err
}

// ... implement remaining interface methods following the same pattern
```

### 4.3 Rules

- Map `sql.ErrNoRows` → `sentinal_errors.ErrNotFound`. Do this at the repository level, not in services.
- Never log inside repositories. Surface the error up.
- For transactional writes, the caller passes a `DBTX` — the repo just uses whatever it receives.

---

## 5. Pattern: Service

Services contain all business logic. They call repositories. They must never touch Gin, HTTP, or JSON encoding.

### 5.1 Service models file

For each domain, create a `*_models.go` file next to the service. Define all `Input` and `Result`/`View` types there. Keep `*_service.go` clean.

Example: `internal/services/conversation_models.go`

```go
package services

import (
    "time"

    "github.com/google/uuid"
)

// --- inputs ---

type CreateConversationInput struct {
    Type            string      // "DM" | "GROUP"
    Subject         string      // GROUP only
    Description     string
    AvatarURL       string
    DisappearingMode string
    ParticipantIDs  []uuid.UUID
    CreatedBy       uuid.UUID
}

type UpdateConversationInput struct {
    ConversationID   uuid.UUID
    Subject          string
    Description      string
    AvatarURL        string
    DisappearingMode string
    ActorID          uuid.UUID
}

// --- views ---

type ConversationView struct {
    ID               string
    Type             string
    Subject          *string
    Description      *string
    AvatarURL        *string
    InviteLink       *string
    DisappearingMode string
    CreatedBy        *string
    CreatedAt        time.Time
    UpdatedAt        time.Time
    LastMessageAt    *time.Time
    Participants     []ParticipantView
}

type ParticipantView struct {
    UserID           string
    DisplayName      string
    Username         string
    AvatarURL        string
    Role             string
    IsOnline         bool
    JoinedAt         time.Time
    MutedUntil       *time.Time
    Archived         bool
    LastReadSequence int64
}
```

### 5.2 Service skeleton

`internal/services/conversation_service.go`:

```go
package services

import (
    "context"
    "errors"
    "time"

    "github.com/google/uuid"

    "sentinal-chat/internal/domain/conversation"
    "sentinal-chat/internal/domain/outbox"
    "sentinal-chat/internal/repository"
    sentinal_errors "sentinal-chat/pkg/errors"
    "sentinal-chat/pkg/database"
)

type ConversationService struct {
    convRepo   repository.ConversationRepository
    userRepo   repository.UserRepository
    outboxRepo repository.OutboxRepository
    db         database.TxBeginner   // interface for starting transactions
}

func NewConversationService(
    convRepo repository.ConversationRepository,
    userRepo repository.UserRepository,
    outboxRepo repository.OutboxRepository,
    db database.TxBeginner,
) (*ConversationService, error) {
    if convRepo == nil || userRepo == nil || outboxRepo == nil || db == nil {
        return nil, sentinal_errors.ErrServiceUnavailable
    }
    return &ConversationService{
        convRepo:   convRepo,
        userRepo:   userRepo,
        outboxRepo: outboxRepo,
        db:         db,
    }, nil
}

func (s *ConversationService) Create(ctx context.Context, input CreateConversationInput) (ConversationView, error) {
    if err := validateCreateConversationInput(input); err != nil {
        return ConversationView{}, err
    }

    // For DM: check whether a conversation already exists
    if input.Type == "DM" && len(input.ParticipantIDs) == 1 {
        existing, err := s.convRepo.GetDirectConversation(ctx, input.CreatedBy, input.ParticipantIDs[0])
        if err == nil {
            return toConversationView(existing), nil
        }
        if !errors.Is(err, sentinal_errors.ErrNotFound) {
            return ConversationView{}, err
        }
    }

    now := time.Now().UTC()
    conv := &conversation.Conversation{
        ID:               uuid.New(),
        Type:             input.Type,
        DisappearingMode: orDefault(input.DisappearingMode, "OFF"),
        CreatedBy:        uuid.NullUUID{UUID: input.CreatedBy, Valid: true},
        CreatedAt:        now,
        UpdatedAt:        now,
    }
    // set optional fields
    if input.Subject != "" {
        conv.Subject = toNullStr(input.Subject)
    }

    // Build participant list including the creator
    allParticipants := deduplicateUUIDs(append([]uuid.UUID{input.CreatedBy}, input.ParticipantIDs...))

    // ------------------------------------------------------------------
    // TRANSACTION: create conversation + participants + outbox event
    // ------------------------------------------------------------------
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return ConversationView{}, err
    }
    defer tx.Rollback()

    if err := s.convRepo.Create(ctx, conv); err != nil {
        return ConversationView{}, err
    }

    for _, uid := range allParticipants {
        role := "MEMBER"
        if uid == input.CreatedBy {
            role = "OWNER"
        }
        p := &conversation.Participant{
            ConversationID: conv.ID,
            UserID:         uid,
            Role:           role,
            JoinedAt:       now,
            AddedBy:        uuid.NullUUID{UUID: input.CreatedBy, Valid: true},
        }
        if err := s.convRepo.AddParticipant(ctx, p); err != nil {
            return ConversationView{}, err
        }
    }

    // Outbox event — see §10 for full envelope details
    outboxPayload := mustMarshalJSON(map[string]any{
        "conversation_id": conv.ID.String(),
        "type":            conv.Type,
        "created_by":      input.CreatedBy.String(),
        "participant_ids":  uuidsToStrings(allParticipants),
    })
    if err := s.outboxRepo.Create(ctx, tx, &outbox.OutboxEvent{
        ID:            uuid.New(),
        EventType:     "conversation:created",
        AggregateType: "conversation",
        AggregateID:   conv.ID.String(),
        Payload:       outboxPayload,
        Status:        outbox.StatusPending,
        CreatedAt:     now,
        UpdatedAt:     now,
    }); err != nil {
        return ConversationView{}, err
    }

    if err := tx.Commit(); err != nil {
        return ConversationView{}, err
    }

    return toConversationView(*conv), nil
}

func (s *ConversationService) GetByID(ctx context.Context, actorID, conversationID uuid.UUID) (ConversationView, error) {
    // Permission check: actor must be a participant
    ok, err := s.convRepo.IsParticipant(ctx, conversationID, actorID)
    if err != nil {
        return ConversationView{}, err
    }
    if !ok {
        return ConversationView{}, sentinal_errors.ErrForbidden
    }

    conv, err := s.convRepo.GetByID(ctx, conversationID)
    if err != nil {
        return ConversationView{}, err
    }

    return toConversationView(conv), nil
}

// --- helpers ---

func validateCreateConversationInput(in CreateConversationInput) error {
    if in.Type != "DM" && in.Type != "GROUP" {
        return sentinal_errors.ErrInvalidInput
    }
    if in.Type == "DM" && len(in.ParticipantIDs) != 1 {
        return sentinal_errors.ErrInvalidInput
    }
    if in.Type == "GROUP" && len(in.ParticipantIDs) == 0 {
        return sentinal_errors.ErrInvalidInput
    }
    return nil
}

func toConversationView(c conversation.Conversation) ConversationView {
    view := ConversationView{
        ID:               c.ID.String(),
        Type:             c.Type,
        DisappearingMode: c.DisappearingMode,
        CreatedAt:        c.CreatedAt,
        UpdatedAt:        c.UpdatedAt,
        LastMessageAt:    c.LastMessageAt,
    }
    if c.Subject.Valid       { view.Subject = &c.Subject.String }
    if c.Description.Valid   { view.Description = &c.Description.String }
    if c.AvatarURL.Valid     { view.AvatarURL = &c.AvatarURL.String }
    if c.InviteLink.Valid    { view.InviteLink = &c.InviteLink.String }
    if c.CreatedBy.Valid     { s := c.CreatedBy.UUID.String(); view.CreatedBy = &s }
    return view
}
```

### 5.3 Rules

- Validate input at the top of every public service method. Return `sentinal_errors.ErrInvalidInput` early.
- Permission checks happen in services, not handlers.
- Services must not import `github.com/gin-gonic/gin`.
- Never call `log.Fatal` or `os.Exit` from a service.

---

## 6. Pattern: Handler

Handlers are thin. They bind the request, call the service, map the result to a payload, and write the response. No business logic lives here.

### 6.1 Handler skeleton

`internal/handler/conversation_handler.go`:

```go
package handler

import (
    "errors"
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"

    "sentinal-chat/internal/services"
    "sentinal-chat/internal/transport/httpdto"
    sentinal_errors "sentinal-chat/pkg/errors"
    "sentinal-chat/pkg/logger"
)

type ConversationHandler struct {
    service *services.ConversationService
    logger  *logger.Logger
}

func NewConversationHandler(service *services.ConversationService, l *logger.Logger) *ConversationHandler {
    return &ConversationHandler{service: service, logger: l}
}

// RegisterRoutes attaches all conversation routes to a router group.
// Call this from cmd/api/main.go on a protected router group.
func (h *ConversationHandler) RegisterRoutes(router gin.IRouter) {
    router.POST("/conversations", h.Create)
    router.GET("/conversations", h.List)
    router.GET("/conversations/direct", h.GetDirect)
    router.GET("/conversations/search", h.Search)
    router.GET("/conversations/type", h.ListByType)
    router.GET("/conversations/invite", h.GetByInviteLink)
    router.GET("/conversations/:id", h.GetByID)
    router.PUT("/conversations/:id", h.Update)
    router.DELETE("/conversations/:id", h.Delete)
    router.POST("/conversations/:id/invite", h.RegenerateInvite)
    router.POST("/conversations/:id/participants", h.AddParticipant)
    router.DELETE("/conversations/:id/participants/:user_id", h.RemoveParticipant)
    router.GET("/conversations/:id/participants", h.ListParticipants)
    router.PUT("/conversations/:id/participants/:user_id/role", h.UpdateParticipantRole)
    router.POST("/conversations/:id/mute", h.Mute)
    router.POST("/conversations/:id/unmute", h.Unmute)
    router.POST("/conversations/:id/archive", h.Archive)
    router.POST("/conversations/:id/unarchive", h.Unarchive)
    router.POST("/conversations/:id/read-sequence", h.UpdateReadSequence)
    router.GET("/conversations/:id/sequence", h.GetSequence)
    router.POST("/conversations/:id/clear", h.Clear)
    router.GET("/conversations/:id/pinned-messages", h.GetPinnedMessages)
}

func (h *ConversationHandler) Create(c *gin.Context) {
    actorID, ok := h.mustUserID(c)
    if !ok {
        return
    }

    var req httpdto.CreateConversationRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        h.writeError(c, sentinal_errors.ErrInvalidInput)
        return
    }

    participantIDs := make([]uuid.UUID, 0, len(req.ParticipantIDs))
    for _, raw := range req.ParticipantIDs {
        id, err := uuid.Parse(strings.TrimSpace(raw))
        if err != nil {
            h.writeError(c, sentinal_errors.ErrInvalidInput)
            return
        }
        participantIDs = append(participantIDs, id)
    }

    view, err := h.service.Create(c.Request.Context(), services.CreateConversationInput{
        Type:             strings.ToUpper(strings.TrimSpace(req.Type)),
        Subject:          strings.TrimSpace(req.Subject),
        Description:      strings.TrimSpace(req.Description),
        AvatarURL:        strings.TrimSpace(req.AvatarURL),
        DisappearingMode: strings.ToUpper(strings.TrimSpace(req.DisappearingMode)),
        ParticipantIDs:   participantIDs,
        CreatedBy:        actorID,
    })
    if err != nil {
        h.writeError(c, err)
        return
    }

    httpdto.WriteSuccess(c, http.StatusCreated, toConversationPayload(view))
}

func (h *ConversationHandler) GetByID(c *gin.Context) {
    actorID, ok := h.mustUserID(c)
    if !ok {
        return
    }

    convID, err := parseUUIDParam(c, "id")
    if err != nil {
        h.writeError(c, sentinal_errors.ErrInvalidInput)
        return
    }

    view, err := h.service.GetByID(c.Request.Context(), actorID, convID)
    if err != nil {
        h.writeError(c, err)
        return
    }

    httpdto.WriteSuccess(c, http.StatusOK, toConversationPayload(view))
}

// mustUserID extracts user_id from context (set by AuthMiddleware).
// Writes 401 and returns false if missing.
func (h *ConversationHandler) mustUserID(c *gin.Context) (uuid.UUID, bool) {
    value, exists := c.Get("user_id")
    if !exists {
        h.writeError(c, sentinal_errors.ErrUnauthorized)
        return uuid.Nil, false
    }
    id, ok := value.(uuid.UUID)
    if !ok || id == uuid.Nil {
        h.writeError(c, sentinal_errors.ErrUnauthorized)
        return uuid.Nil, false
    }
    return id, true
}

// writeError maps sentinel and domain errors to HTTP status codes.
// Add domain-specific error cases at the top of the switch.
func (h *ConversationHandler) writeError(c *gin.Context, err error) {
    status := http.StatusInternalServerError
    code := "INTERNAL_ERROR"
    message := "internal server error"

    switch {
    case errors.Is(err, sentinal_errors.ErrInvalidInput):
        status = http.StatusBadRequest
        code = "INVALID_INPUT"
        message = "invalid input"
    case errors.Is(err, sentinal_errors.ErrUnauthorized):
        status = http.StatusUnauthorized
        code = "UNAUTHORIZED"
        message = "unauthorized"
    case errors.Is(err, sentinal_errors.ErrForbidden):
        status = http.StatusForbidden
        code = "FORBIDDEN"
        message = "forbidden"
    case errors.Is(err, sentinal_errors.ErrNotFound):
        status = http.StatusNotFound
        code = "NOT_FOUND"
        message = "not found"
    case errors.Is(err, sentinal_errors.ErrAlreadyExists), errors.Is(err, sentinal_errors.ErrConflict):
        status = http.StatusConflict
        code = "CONFLICT"
        message = "conflict"
    case errors.Is(err, sentinal_errors.ErrServiceUnavailable):
        status = http.StatusServiceUnavailable
        code = "SERVICE_UNAVAILABLE"
        message = "service unavailable"
    }

    if status >= http.StatusInternalServerError && h.logger != nil {
        h.logger.Errorf("conversation handler error: %v", err)
    }

    httpdto.WriteError(c, status, message, code)
}

// toConversationPayload converts a service view to a DTO payload.
// Define httpdto.ConversationPayload in internal/transport/httpdto/conversation_dto.go.
func toConversationPayload(view services.ConversationView) httpdto.ConversationPayload {
    return httpdto.ConversationPayload{
        ID:               view.ID,
        Type:             view.Type,
        Subject:          view.Subject,
        Description:      view.Description,
        AvatarURL:        view.AvatarURL,
        InviteLink:       view.InviteLink,
        DisappearingMode: view.DisappearingMode,
        CreatedBy:        view.CreatedBy,
        CreatedAt:        view.CreatedAt,
        UpdatedAt:        view.UpdatedAt,
        LastMessageAt:    view.LastMessageAt,
    }
}
```

---

## 7. Pattern: DTO

All request and response structs live in `internal/transport/httpdto/`.

### 7.1 Naming rules

| Type | Suffix | Example |
|---|---|---|
| JSON request body | `Request` | `CreateConversationRequest` |
| URL query params | `Query` | `ListMessagesQuery` |
| Response payload | `Payload` | `ConversationPayload` |
| List wrapper | `ListPayload` | `ConversationListPayload` |

### 7.2 Request struct example

```go
// internal/transport/httpdto/conversation_dto.go
package httpdto

import "time"

type CreateConversationRequest struct {
    Type             string   `json:"type"              binding:"required,oneof=DM GROUP"`
    Subject          string   `json:"subject"           binding:"omitempty,max=255"`
    Description      string   `json:"description"       binding:"omitempty,max=1000"`
    AvatarURL        string   `json:"avatar_url"        binding:"omitempty,url,max=2048"`
    DisappearingMode string   `json:"disappearing_mode" binding:"omitempty,oneof=OFF AFTER_24H AFTER_7D AFTER_90D"`
    ParticipantIDs   []string `json:"participant_ids"   binding:"required,min=1,dive,uuid"`
}

type UpdateConversationRequest struct {
    Subject          string `json:"subject"           binding:"omitempty,max=255"`
    Description      string `json:"description"       binding:"omitempty,max=1000"`
    AvatarURL        string `json:"avatar_url"        binding:"omitempty,url,max=2048"`
    DisappearingMode string `json:"disappearing_mode" binding:"omitempty,oneof=OFF AFTER_24H AFTER_7D AFTER_90D"`
}

type MuteConversationRequest struct {
    Until *time.Time `json:"until" binding:"omitempty"`
}

type AddParticipantRequest struct {
    UserID string `json:"user_id" binding:"required,uuid"`
    Role   string `json:"role"    binding:"omitempty,oneof=MEMBER ADMIN"`
}

type UpdateParticipantRoleRequest struct {
    Role string `json:"role" binding:"required,oneof=MEMBER ADMIN OWNER"`
}

type UpdateReadSequenceRequest struct {
    SeqID int64 `json:"seq_id" binding:"required,min=1"`
}
```

### 7.3 Response payload example

```go
type ConversationPayload struct {
    ID               string               `json:"id"`
    Type             string               `json:"type"`
    Subject          *string              `json:"subject,omitempty"`
    Description      *string              `json:"description,omitempty"`
    AvatarURL        *string              `json:"avatar_url,omitempty"`
    InviteLink       *string              `json:"invite_link,omitempty"`
    DisappearingMode string               `json:"disappearing_mode"`
    CreatedBy        *string              `json:"created_by,omitempty"`
    CreatedAt        time.Time            `json:"created_at"`
    UpdatedAt        time.Time            `json:"updated_at"`
    LastMessageAt    *time.Time           `json:"last_message_at,omitempty"`
    Participants     []ParticipantPayload `json:"participants,omitempty"`
}

type ParticipantPayload struct {
    UserID           string     `json:"user_id"`
    DisplayName      string     `json:"display_name"`
    Username         string     `json:"username"`
    AvatarURL        string     `json:"avatar_url"`
    Role             string     `json:"role"`
    IsOnline         bool       `json:"is_online"`
    JoinedAt         time.Time  `json:"joined_at"`
    MutedUntil       *time.Time `json:"muted_until,omitempty"`
    Archived         bool       `json:"archived"`
    LastReadSequence int64      `json:"last_read_sequence"`
}

type ConversationListPayload struct {
    Items  []ConversationPayload `json:"items"`
    Page   int                   `json:"page"`
    Limit  int                   `json:"limit"`
    Total  int64                 `json:"total"`
}
```

### 7.4 Rules

- Never embed `time.Time` zero values — use `*time.Time` and `omitempty`.
- Never embed domain structs (e.g. `message.Message`) in a payload.
- Use `binding:"required"` on every field that must be present.
- Use `binding:"uuid"` for UUID string fields.
- Use `binding:"oneof=A B C"` for enum fields.

---

## 8. Pattern: Middleware

All middleware lives in `internal/middleware/`. New middleware must follow the Gin `HandlerFunc` signature.

### 8.1 Middleware that already exists

| File | How to use |
|---|---|
| `auth_middleware.go` | `router.Use(middleware.AuthMiddleware(parseToken, validateSession))` |
| `ratelimit_middleware.go` | `router.Use(middleware.RateLimitMiddleware(rateLimiter.CheckAuth(10, time.Minute)))` |
| `cors_middleware.go` | `engine.Use(middleware.CORSMiddleware(cfg.FrontendURL))` |
| `logging_middleware.go` | `engine.Use(middleware.LoggingMiddleware(logger))` |
| `error_middleware.go` | `engine.Use(middleware.ErrorHandler(logger))` |
| `request_id_middleware.go` | `engine.Use(middleware.RequestIDMiddleware())` |

### 8.2 Auth context values

The auth middleware sets these three values. Every protected handler reads them the same way:

```go
// Read user_id (always present on protected routes)
userIDVal, _ := c.Get("user_id")
userID := userIDVal.(uuid.UUID)

// Read session_id (string UUID)
sessionIDVal, _ := c.Get("session_id")
sessionID := sessionIDVal.(string)

// Read device_id (string, may be empty)
deviceIDVal, _ := c.Get("device_id")
deviceID, _ := deviceIDVal.(string)
```

### 8.3 Rate limit wiring (once Redis is ready)

```go
// In cmd/api/main.go, after creating the Redis client:
rateLimiter := redis.NewRateLimiter(redisClient)

// Apply to auth group (rate limit by IP):
authGroup.Use(middleware.RateLimitMiddleware(rateLimiter.CheckAuth(10, time.Minute)))

// Apply to message routes (rate limit by user_id):
msgGroup.Use(middleware.MessageRateLimitMiddleware(rateLimiter.CheckMessage(60, time.Minute)))

// Apply to call routes (rate limit by user_id):
callGroup.Use(middleware.CallRateLimitMiddleware(rateLimiter.CheckCall(5, time.Minute)))
```

---

## 9. Pattern: Transaction Management

Transactions are opened in the **service layer**. Repositories accept a `DBTX` interface so
they work inside or outside a transaction without modification.

### 9.1 The TxBeginner interface

Add this to `pkg/database/` (or `internal/repository/`):

```go
// pkg/database/tx.go
package database

import (
    "context"
    "database/sql"
)

// TxBeginner is implemented by *sql.DB and allows services to start transactions.
type TxBeginner interface {
    BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}
```

`*sql.DB` already implements this interface — no adapter needed.

### 9.2 Transaction pattern in a service

```go
func (s *SomeService) DoSomething(ctx context.Context, input SomeInput) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback() // no-op if Commit was called

    // Pass tx to repositories
    if err := s.repo.SomeWrite(ctx, tx, data); err != nil {
        return err  // defer will rollback
    }

    if err := s.outboxRepo.Create(ctx, tx, event); err != nil {
        return err  // defer will rollback
    }

    return tx.Commit()
}
```

### 9.3 Repository methods that need a transaction

Only methods that **must** be atomic alongside other writes need to accept `DBTX`.
Read-only methods use the default `r.db` pool — do not change them.

The `OutboxRepository.Create` already has this signature:
```go
Create(ctx context.Context, tx DBTX, event *outbox.OutboxEvent) error
```
Pass `nil` to use the default pool (i.e., outside a transaction).

---

## 10. Pattern: Outbox Write

Every state-changing service method that should trigger a real-time event must write
an `outbox_events` row in the **same** DB transaction as the domain data.

### 10.1 Building the outbox event

```go
import (
    "encoding/json"
    "time"

    "github.com/google/uuid"
    "sentinal-chat/internal/domain/outbox"
    "sentinal-chat/internal/events"
)

func buildOutboxEvent(eventType, aggregateType, aggregateID string, payload interface{}) (*outbox.OutboxEvent, error) {
    data, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }
    now := time.Now().UTC()
    return &outbox.OutboxEvent{
        ID:            uuid.New(),
        EventType:     eventType,
        AggregateType: aggregateType,
        AggregateID:   aggregateID,
        Payload:       data,
        Status:        outbox.StatusPending,
        CreatedAt:     now,
        UpdatedAt:     now,
    }, nil
}
```

### 10.2 Outbox payload for message:new

```go
type MessageNewPayload struct {
    MessageID      string    `json:"message_id"`
    ConversationID string    `json:"conversation_id"`
    SenderID       string    `json:"sender_id"`
    SeqID          int64     `json:"seq_id"`
    Type           string    `json:"type"`
    ClientMsgID    string    `json:"client_message_id,omitempty"`
    IsForwarded    bool      `json:"is_forwarded"`
    MentionCount   int       `json:"mention_count"`
    CreatedAt      time.Time `json:"created_at"`
}
```

### 10.3 Channel routing

The outbox worker decides the Redis channel based on `event_type`:

| Event type prefix | Redis channel |
|---|---|
| `message:*`, `reaction:*`, `typing:*` | `channel:conversation:{conversation_id}` |
| `conversation:*` | `channel:conversation:{conversation_id}` |
| `call:created`, `call:ended`, `call:connected` | `channel:call:{call_id}` |
| `call:offer`, `call:answer`, `call:ice` | `channel:user:{to_user_id}` (direct) |
| `presence:*` | `channel:presence:{user_id}` |
| `command:*` | `channel:user:{user_id}` |

Store `conversation_id` in the outbox payload so the worker can extract it.

---

## 11. Pattern: Redis Client and Pub/Sub

See `docs/redis-outbox.md` for the full implementation. Quick reference:

### 11.1 Files to create

```
internal/redis/client.go      Redis connection pool + Set/Get/Publish/Subscribe
internal/redis/pubsub.go      PubSubPublisher + OutboxEnvelope struct
internal/redis/ratelimit.go   RateLimiter implementing RateLimitChecker
internal/redis/presence.go    PresenceStore (online/offline TTL keys)
internal/redis/typing.go      TypingStore (typing indicator TTL keys)
```

### 11.2 Dependency injection

All Redis stores are created once in `cmd/api/main.go` and injected into services and the hub:

```go
redisClient, err := redis.New(cfg)
if err != nil {
    l.Fatalf("redis: %v", err)
}
defer redisClient.Close()

publisher  := redis.NewPubSubPublisher(redisClient)
rateLimiter := redis.NewRateLimiter(redisClient)
presence   := redis.NewPresenceStore(redisClient)
typing     := redis.NewTypingStore(redisClient)
```

---

## 12. Pattern: WebSocket Hub

See `docs/websockets.md` for the full Go implementation.

### 12.1 Files to create

```
internal/server/hub.go       Hub struct, Run loop, register/unregister, fan-out
internal/server/ws_client.go Client struct, read pump, write pump
internal/server/ws_handler.go Gin handler for GET /v1/ws
internal/server/ws_types.go  InboundFrame, OutboundFrame, ErrorData
```

### 12.2 Starting the hub

```go
// cmd/api/main.go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

hub := server.NewHub(publisher, presence, typing, l)
go hub.Run(ctx)

// Also start the outbox worker
outboxWorker := services.NewOutboxWorker(outboxRepo, publisher, l)
go outboxWorker.Run(ctx)
```

---

## 13. Pattern: Command Pattern

See `docs/commands.md` for the full Go implementation. Quick reference:

### 13.1 When to write a command log

Write a `command_logs` row only for these action types:
- `DELETE_MESSAGE`
- `EDIT_MESSAGE`
- `PIN_MESSAGE`
- `UNPIN_MESSAGE`
- `REACT_MESSAGE`
- `CLEAR_CHAT`

Do NOT write command logs for: message send, conversation create, participant add/remove,
read receipts, delivered receipts, or any read operations.

### 13.2 Transaction order (always follow this sequence)

```
1. Insert command_logs row (status = PENDING)
2. Perform domain mutation
3. Update command_logs row (status = EXECUTED or FAILED)
4. Insert outbox_events row
5. Commit transaction
```

---

## 14. Wiring a New Feature End-to-End

Follow these steps every time you add a new feature:

### Step 1: Domain model (if new entity needed)

Add struct to `internal/domain/<entity>/entity.go`.
No tags. No imports from other internal packages.

### Step 2: Repository interface

Add methods to the correct interface in `internal/repository/interfaces.go`.

### Step 3: Repository implementation

Add methods to the existing `*_repository.go` file.
Map `sql.ErrNoRows` to `sentinal_errors.ErrNotFound` at this layer.

### Step 4: Service models

Create or update `internal/services/<feature>_models.go`.
Define `*Input` structs for inputs and `*View` structs for outputs.

### Step 5: Service

Create or update `internal/services/<feature>_service.go`.
- Validate input early.
- Check permissions (participant, ownership, etc.).
- Open a transaction for multi-write operations.
- Write outbox event in same transaction for state-changing operations.
- Return a `*View` struct — never a domain struct.

### Step 6: DTOs

Create or update `internal/transport/httpdto/<feature>_dto.go`.
- `*Request` structs with `binding` tags for inputs.
- `*Payload` structs with `json` tags for outputs.

### Step 7: Handler

Create or update `internal/handler/<feature>_handler.go`.
- `RegisterRoutes(router gin.IRouter)` method.
- Each handler method: bind → call service → write response.
- Private `mustUserID(c)` helper.
- Private `writeError(c, err)` helper mapping sentinel errors.

### Step 8: Wire in cmd/api/main.go

```go
featureSvc, err := services.NewFeatureService(repo, outboxRepo, db, l)
if err != nil { l.Fatalf(...) }
featureHandler := handler.NewFeatureHandler(featureSvc, l)
featureHandler.RegisterRoutes(protected)
```

---

## 15. Error Handling Reference

### Sentinel errors in `pkg/errors`

```go
var (
    ErrInvalidInput       = errors.New("invalid input")
    ErrUnauthorized       = errors.New("unauthorized")
    ErrForbidden          = errors.New("forbidden")
    ErrNotFound           = errors.New("not found")
    ErrAlreadyExists      = errors.New("already exists")
    ErrConflict           = errors.New("conflict")
    ErrInvalidTransition  = errors.New("invalid state transition")
    ErrTooLarge           = errors.New("too large")
    ErrServiceUnavailable = errors.New("service unavailable")
)
```

### Standard HTTP mapping

| Error | Status | Code |
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
| Unknown error | 500 | `INTERNAL_ERROR` |

### Domain-specific errors

Define service-specific errors as package-level `var` in the service file:

```go
var (
    ErrInvalidCredentials       = errors.New("invalid credentials")
    ErrUnsupportedOAuthProvider = errors.New("unsupported oauth provider")
    ErrOAuthEmailUnverified     = errors.New("oauth email is not verified")
    ErrCommandNotUndoable       = errors.New("command cannot be undone")
    ErrUndoWindowExpired        = errors.New("undo window has expired")
)
```

Always use `errors.Is` for comparison — never compare error strings.

---

## 16. Known Bugs That Must Be Fixed

### Bug 1 — command.Status enum mismatch (CRITICAL)

**File:** `internal/domain/command/command.go`

```go
// Current (WRONG)
StatusExecuting Status = "EXECUTING"  // not in SQL
StatusCompleted Status = "COMPLETED"  // not in SQL

// Fix to:
StatusExecuted Status = "EXECUTED"    // matches SQL enum
// Remove StatusExecuting entirely
```

Also fix `internal/repository/command_repository.go`: replace every `"COMPLETED"` string
with `"EXECUTED"`, and remove any use of `"EXECUTING"`.

### Bug 2 — call_participants.status uses invalid value (CRITICAL)

**File:** `internal/repository/call_repository.go`

```go
// Current (WRONG)
"JOINED"  // not in SQL enum

// Fix to:
"CONNECTED"  // valid SQL enum value
```

The SQL enum for `call_participant_status` is:
`INVITED`, `RINGING`, `CONNECTED`, `LEFT`, `DECLINED`

---

## 17. Build Order Roadmap

Follow this order. Each step builds on the previous.

### Week 1 — Infrastructure and User/Conversation APIs

```
Day 1:  Fix Bug 1 and Bug 2
Day 1:  Create internal/events/types.go and events/publisher.go
Day 2:  Create internal/redis/client.go + pubsub.go + ratelimit.go + presence.go + typing.go
Day 3:  Wire Redis into cmd/api/main.go; wire rate limit middleware
Day 4:  User service + handler + DTOs  (GET/PUT /v1/users/me, contacts, devices)
Day 5:  Conversation service + handler + DTOs  (CRUD, participants, mute, archive)
```

### Week 2 — Messaging

```
Day 1:  Message service + handler + DTOs  (send, list, receipts)
Day 2:  Message features  (reactions, stars, pins, edits, mentions)
Day 3:  Poll service + handler + DTOs
Day 4:  Outbox worker (internal/services/outbox_worker.go)
Day 5:  WebSocket hub + ws_client + ws_handler  (GET /v1/ws)
```

### Week 3 — Calls and Commands

```
Day 1:  Call service + handler + DTOs
Day 2:  WebRTC signaling via WebSocket (call:offer, answer, ice)
Day 3:  Command service + handler + DTOs  (after fixing enums)
Day 4:  End-to-end test: send message → outbox worker → ws hub → client receives event
Day 5:  Cleanup, diagnostics, documentation
```

### Milestone check after each week

After each week, verify:
- All new routes appear in `GET /goroutines` output (server is running).
- `GET /health` still returns `healthy`.
- No new diagnostic errors introduced.
- Existing auth and upload tests still pass.
