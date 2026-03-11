# Commands — Full Implementation Guide

This document is the complete implementation guide for the command pattern in Sentinal Chat.
It contains all payload shapes, the full Go code for the service, handler, and DTOs,
and the precise undo logic per command type.

Current repo reality:
- `command_logs` table and SQL migrations exist.
- `CommandRepository` interface and implementation both exist.
- `internal/domain/command/command.go` exists but has a **critical enum bug** (see §2).
- No command service exists yet.
- No command handler exists yet.
- No command DTOs exist yet.
- The `CanUndo` method in the repository checks for `"COMPLETED"` which will never match
  the DB enum value `"EXECUTED"`. This is broken and must be fixed.

---

## Table of Contents

1. [Why Commands Exist](#1-why-commands-exist)
2. [Bug Fix: Status Enum Mismatch](#2-bug-fix-status-enum-mismatch)
3. [SQL Schema (already exists)](#3-sql-schema-already-exists)
4. [Supported Command Types](#4-supported-command-types)
5. [Payload and Undo Payload Shapes](#5-payload-and-undo-payload-shapes)
6. [Command Lifecycle](#6-command-lifecycle)
7. [Undo Rules](#7-undo-rules)
8. [Go Implementation: DTOs](#8-go-implementation-dtos)
9. [Go Implementation: Service Models](#9-go-implementation-service-models)
10. [Go Implementation: Command Service](#10-go-implementation-command-service)
11. [Go Implementation: Command Handler](#11-go-implementation-command-handler)
12. [Outbox Integration](#12-outbox-integration)
13. [HTTP Endpoints](#13-http-endpoints)
14. [Wiring into cmd/api/main.go](#14-wiring-into-cmdapimainago)

---

## 1. Why Commands Exist

Commands give the system a durable, undoable, auditable record of every state-changing
chat action. Without a command log:

- There is no undo.
- There is no audit trail for moderation.
- There is no way to recover from partial failures.

The command pattern here is simple:

1. Service creates a `command_logs` row as `PENDING`.
2. Service performs the domain mutation (update/delete/insert).
3. Service updates the command row to `EXECUTED`.
4. Service writes an `outbox_events` row.
5. All four steps happen in a single DB transaction.

If any step fails, the whole transaction rolls back and no partial state is committed.

---

## 2. Bug Fix: Status Enum Mismatch

**This must be fixed before any command code is written. Fix it first.**

### The problem

`internal/domain/command/command.go` currently has:

```go
// WRONG — does not match SQL
const (
    StatusPending   Status = "PENDING"
    StatusExecuting Status = "EXECUTING"   // ← not in SQL enum
    StatusCompleted Status = "COMPLETED"   // ← not in SQL enum
    StatusFailed    Status = "FAILED"
    StatusUndone    Status = "UNDONE"
)
```

The SQL enum is:

```sql
CREATE TYPE command_status AS ENUM (
    'PENDING',
    'EXECUTED',   -- not COMPLETED
    'FAILED',
    'UNDONE'
    -- no EXECUTING
);
```

### The fix

Replace the entire constants block in `internal/domain/command/command.go`:

```go
// internal/domain/command/command.go

package command

import (
    "encoding/json"
    "time"

    "github.com/google/uuid"
)

// Status represents the command execution state.
// Values MUST match the SQL enum command_status exactly.
type Status string

const (
    // StatusPending means the command log row was created but the action is not yet complete.
    StatusPending Status = "PENDING"

    // StatusExecuted means the domain mutation completed successfully.
    StatusExecuted Status = "EXECUTED"

    // StatusFailed means the command could not complete. See ErrorMessage for details.
    StatusFailed Status = "FAILED"

    // StatusUndone means the command was successfully reversed.
    StatusUndone Status = "UNDONE"
)

// CommandType enumerates the supported command action types.
// The DB column is VARCHAR(50) so these are validated in the application.
type CommandType string

const (
    CommandDeleteMessage CommandType = "DELETE_MESSAGE"
    CommandEditMessage   CommandType = "EDIT_MESSAGE"
    CommandPinMessage    CommandType = "PIN_MESSAGE"
    CommandUnpinMessage  CommandType = "UNPIN_MESSAGE"
    CommandReactMessage  CommandType = "REACT_MESSAGE"
    CommandClearChat     CommandType = "CLEAR_CHAT"
)

// IsValid returns true if the command type is supported.
func (ct CommandType) IsValid() bool {
    switch ct {
    case CommandDeleteMessage, CommandEditMessage,
        CommandPinMessage, CommandUnpinMessage,
        CommandReactMessage, CommandClearChat:
        return true
    }
    return false
}

// CommandLog stores command execution history and undo data.
type CommandLog struct {
    ID              uuid.UUID
    CommandType     CommandType
    UserID          uuid.UUID
    ConversationID  *uuid.UUID
    Status          Status
    Payload         json.RawMessage
    UndoPayload     json.RawMessage
    ErrorMessage    string
    ExecutionTimeMs int
    CreatedAt       time.Time
    ExecutedAt      *time.Time
    UndoneAt        *time.Time
}

// TableName returns the database table name.
func (CommandLog) TableName() string {
    return "command_logs"
}
```

Also search `internal/repository/command_repository.go` for every occurrence of
`"COMPLETED"` and `"EXECUTING"` and replace them with `"EXECUTED"` and `"PENDING"`.

---

## 3. SQL Schema (already exists)

```sql
CREATE TYPE command_status AS ENUM ('PENDING', 'EXECUTED', 'FAILED', 'UNDONE');

CREATE TABLE IF NOT EXISTS command_logs (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    command_type        VARCHAR(50)     NOT NULL,
    user_id             UUID            NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id     UUID            REFERENCES conversations(id) ON DELETE CASCADE,
    status              command_status  DEFAULT 'PENDING',
    payload             JSONB           NOT NULL,
    undo_payload        JSONB,
    error_message       TEXT,
    execution_time_ms   INTEGER,
    created_at          TIMESTAMP       DEFAULT NOW(),
    executed_at         TIMESTAMP,
    undone_at           TIMESTAMP
);

CREATE INDEX idx_command_logs_user_created ON command_logs (user_id, created_at DESC);
CREATE INDEX idx_command_logs_conv         ON command_logs (conversation_id);
CREATE INDEX idx_command_logs_status       ON command_logs (status);
```

### CommandRepository interface (already exists)

```go
type CommandRepository interface {
    CreateLog(ctx context.Context, log *command.CommandLog) error
    UpdateLog(ctx context.Context, log *command.CommandLog) error
    GetLogByID(ctx context.Context, id uuid.UUID) (command.CommandLog, error)
    GetPendingCommands(ctx context.Context, limit int) ([]command.CommandLog, error)
    GetCommandsByUser(ctx context.Context, userID uuid.UUID, limit int) ([]command.CommandLog, error)
    CanUndo(ctx context.Context, commandID uuid.UUID, userID uuid.UUID) (bool, error)
}
```

The `CanUndo` repository method currently only checks `status == "COMPLETED"` (wrong) and
does not check the time window. After fixing the enum, also update `CanUndo` to check
`status == "EXECUTED"` AND `undone_at IS NULL` AND `executed_at IS NOT NULL`.
The time-window check should be done in the service, not the repository.

---

## 4. Supported Command Types

| Command type | Action | Undoable |
|---|---|---|
| `DELETE_MESSAGE` | Soft-delete a message | Yes — restores `deleted_at = NULL` |
| `EDIT_MESSAGE` | Update encrypted content | Yes — restores previous content |
| `PIN_MESSAGE` | Pin a message in a conversation | Yes — unpins |
| `UNPIN_MESSAGE` | Remove a pinned message | Yes — re-pins |
| `REACT_MESSAGE` | Add a reaction to a message | Yes — removes the reaction |
| `CLEAR_CHAT` | Hide prior history for the requesting user | Yes (within window) |

---

## 5. Payload and Undo Payload Shapes

These are the exact JSONB structures stored in `command_logs.payload` and
`command_logs.undo_payload`. Keep them precise — they are used by the undo executor.

### `DELETE_MESSAGE`

```json
// payload
{
  "message_id":      "uuid",
  "conversation_id": "uuid",
  "deleted_by":      "uuid",
  "deleted_at":      "2025-01-01T10:00:00Z"
}

// undo_payload
{
  "message_id": "uuid"
}
```

Execute: `UPDATE messages SET deleted_at = NOW() WHERE id = message_id`
Undo:    `UPDATE messages SET deleted_at = NULL WHERE id = message_id`

---

### `EDIT_MESSAGE`

```json
// payload
{
  "message_id":           "uuid",
  "conversation_id":      "uuid",
  "new_encrypted_content": "base64-ciphertext",
  "edited_by":            "uuid",
  "edited_at":            "2025-01-01T10:01:00Z"
}

// undo_payload
{
  "message_id":                "uuid",
  "previous_encrypted_content": "base64-old-ciphertext",
  "previous_edited_at":         null
}
```

Execute:
1. `INSERT INTO message_edits (id, message_id, encrypted_content, edited_by, edited_at, version_number) VALUES (...)`
2. `UPDATE messages SET encrypted_content = new_encrypted_content, edited_at = NOW() WHERE id = message_id`

Undo:
1. `UPDATE messages SET encrypted_content = previous_encrypted_content, edited_at = previous_edited_at WHERE id = message_id`
   (Also optionally insert another `message_edits` row to audit the undo.)

---

### `PIN_MESSAGE`

```json
// payload
{
  "conversation_id": "uuid",
  "message_id":      "uuid",
  "pinned_by":       "uuid",
  "pinned_at":       "2025-01-01T10:02:00Z"
}

// undo_payload
{
  "conversation_id": "uuid",
  "message_id":      "uuid"
}
```

Execute: `INSERT INTO pinned_messages (conversation_id, message_id, pinned_by, pinned_at) VALUES (...)`
Undo:    `DELETE FROM pinned_messages WHERE conversation_id = ? AND message_id = ?`

---

### `UNPIN_MESSAGE`

```json
// payload
{
  "conversation_id": "uuid",
  "message_id":      "uuid",
  "unpinned_by":     "uuid",
  "unpinned_at":     "2025-01-01T10:03:00Z"
}

// undo_payload
{
  "conversation_id": "uuid",
  "message_id":      "uuid",
  "pinned_by":       "uuid",
  "pinned_at":       "2025-01-01T10:02:00Z"
}
```

Execute: `DELETE FROM pinned_messages WHERE conversation_id = ? AND message_id = ?`
Undo:    `INSERT INTO pinned_messages (conversation_id, message_id, pinned_by, pinned_at) VALUES (...)`

---

### `REACT_MESSAGE`

```json
// payload
{
  "message_id":      "uuid",
  "conversation_id": "uuid",
  "user_id":         "uuid",
  "reaction_code":   ":thumbsup:"
}

// undo_payload
{
  "message_id":    "uuid",
  "user_id":       "uuid",
  "reaction_code": ":thumbsup:"
}
```

Execute: `INSERT INTO message_reactions (id, message_id, user_id, reaction_code, created_at) VALUES (...)`
Undo:    `DELETE FROM message_reactions WHERE message_id = ? AND user_id = ? AND reaction_code = ?`

---

### `CLEAR_CHAT`

```json
// payload
{
  "conversation_id": "uuid",
  "user_id":         "uuid",
  "cleared_at":      "2025-01-01T10:04:00Z"
}

// undo_payload
{
  "conversation_id":     "uuid",
  "user_id":             "uuid",
  "previous_cleared_at": null
}
```

Execute: `INSERT INTO conversation_clears (conversation_id, user_id, cleared_at) VALUES (...) ON CONFLICT (...) DO UPDATE SET cleared_at = ...`
Undo:    If `previous_cleared_at` is null: `DELETE FROM conversation_clears WHERE conversation_id = ? AND user_id = ?`
         If not null: `UPDATE conversation_clears SET cleared_at = previous_cleared_at WHERE ...`

---

## 6. Command Lifecycle

```
PENDING
   │
   ├── domain write + outbox write succeed ──► EXECUTED
   │
   └── domain write fails ──────────────────► FAILED

EXECUTED
   │
   └── undo requested (within window) ──────► UNDONE
```

State transitions:
- `PENDING → EXECUTED`: happy path, all writes committed.
- `PENDING → FAILED`: transaction rolled back, error stored.
- `EXECUTED → UNDONE`: undo applied, reverse mutation committed.
- `UNDONE → (nothing)`: terminal state, cannot be re-undone.
- `FAILED → (nothing)`: terminal state for now (operator replay is a future concern).

---

## 7. Undo Rules

A command CAN be undone only when ALL of the following are true:

1. `command_logs.user_id == requesting user_id`
2. `command_logs.status == 'EXECUTED'`
3. `command_logs.undone_at IS NULL`
4. `command_logs.executed_at IS NOT NULL`
5. `NOW() - command_logs.executed_at <= COMMAND_UNDO_WINDOW_SECONDS` (default 300s = 5 minutes)
6. The referenced message/conversation still exists and has not been hard-deleted.
7. The user still has participant access to the conversation.

These checks run in the **service layer**. The repository's `CanUndo` only checks the
first three (DB-level checks). The service adds the time-window and permission checks.

### Undo window configuration

Add to `config/config.go`:

```go
// CommandUndoWindowSeconds is how long after execution a command can be undone.
// Default: 300 (5 minutes).
CommandUndoWindowSeconds int
```

Load from env:
```go
CommandUndoWindowSeconds: getEnvInt("COMMAND_UNDO_WINDOW_SECONDS", 300),
```

---

## 8. Go Implementation: DTOs

Create `internal/transport/httpdto/command_dto.go`:

```go
// internal/transport/httpdto/command_dto.go
package httpdto

import "time"

// --- Response payloads ---

// CommandLogPayload is the response shape for a single command log entry.
type CommandLogPayload struct {
    ID              string      `json:"id"`
    CommandType     string      `json:"command_type"`
    UserID          string      `json:"user_id"`
    ConversationID  *string     `json:"conversation_id,omitempty"`
    Status          string      `json:"status"`
    Payload         interface{} `json:"payload,omitempty"`
    UndoPayload     interface{} `json:"undo_payload,omitempty"`
    ErrorMessage    string      `json:"error_message,omitempty"`
    ExecutionTimeMs int         `json:"execution_time_ms"`
    CreatedAt       time.Time   `json:"created_at"`
    ExecutedAt      *time.Time  `json:"executed_at,omitempty"`
    UndoneAt        *time.Time  `json:"undone_at,omitempty"`
    CanUndo         bool        `json:"can_undo"`
    UndoExpiresAt   *time.Time  `json:"undo_expires_at,omitempty"`
}

// CommandListPayload wraps a list of command logs.
type CommandListPayload struct {
    Items []CommandLogPayload `json:"items"`
    Total int                 `json:"total"`
    Limit int                 `json:"limit"`
}

// UndoCommandPayload is returned after a successful undo.
type UndoCommandPayload struct {
    CommandID   string    `json:"command_id"`
    Status      string    `json:"status"`
    UndoneAt    time.Time `json:"undone_at"`
    CommandType string    `json:"command_type"`
}
```

---

## 9. Go Implementation: Service Models

Create `internal/services/command_models.go`:

```go
// internal/services/command_models.go
package services

import (
    "encoding/json"
    "time"
)

// --- View types returned by CommandService ---

// CommandLogView is the service-layer representation of a command log.
// It is safe to pass to the transport layer.
type CommandLogView struct {
    ID              string
    CommandType     string
    UserID          string
    ConversationID  *string
    Status          string
    Payload         json.RawMessage
    UndoPayload     json.RawMessage
    ErrorMessage    string
    ExecutionTimeMs int
    CreatedAt       time.Time
    ExecutedAt      *time.Time
    UndoneAt        *time.Time
    CanUndo         bool
    UndoExpiresAt   *time.Time
}

// --- Input types for delete/edit/pin/react/clear ---
// These are used by other services (MessageService, ConversationService) to create
// command logs as part of their own transactions. They are NOT called by the handler.

// DeleteMessageCommandInput is passed to MessageService.DeleteMessage
// to ensure a command log is written atomically with the delete.
type DeleteMessageCommandInput struct {
    MessageID      string
    ConversationID string
    DeletedBy      string
}

// EditMessageCommandInput is passed to MessageService.EditMessage.
type EditMessageCommandInput struct {
    MessageID              string
    ConversationID         string
    NewEncryptedContent    string
    PreviousContent        string
    PreviousEditedAt       *time.Time
    EditedBy               string
}

// PinMessageCommandInput is passed to MessageService.PinMessage.
type PinMessageCommandInput struct {
    ConversationID string
    MessageID      string
    PinnedBy       string
}

// UnpinMessageCommandInput is passed to MessageService.UnpinMessage.
type UnpinMessageCommandInput struct {
    ConversationID  string
    MessageID       string
    UnpinnedBy      string
    OriginalPinnedBy string
    OriginalPinnedAt time.Time
}

// ReactMessageCommandInput is passed to MessageService.AddReaction.
type ReactMessageCommandInput struct {
    MessageID      string
    ConversationID string
    UserID         string
    ReactionCode   string
}

// ClearChatCommandInput is passed to ConversationService.ClearChat.
type ClearChatCommandInput struct {
    ConversationID  string
    UserID          string
    PreviousClearedAt *time.Time
}
```

---

## 10. Go Implementation: Command Service

Create `internal/services/command_service.go`:

```go
// internal/services/command_service.go
package services

import (
    "context"
    "encoding/json"
    "errors"
    "time"

    "github.com/google/uuid"

    "sentinal-chat/config"
    "sentinal-chat/internal/domain/command"
    "sentinal-chat/internal/domain/message"
    "sentinal-chat/internal/domain/outbox"
    "sentinal-chat/internal/events"
    "sentinal-chat/internal/repository"
    sentinal_errors "sentinal-chat/pkg/errors"
    "sentinal-chat/pkg/logger"
)

var (
    // ErrCommandNotUndoable is returned when a command cannot be undone.
    ErrCommandNotUndoable = errors.New("command cannot be undone")

    // ErrUndoWindowExpired is returned when the undo window has passed.
    ErrUndoWindowExpired = errors.New("undo window has expired")
)

// CommandService provides read access to command logs and executes undo operations.
// Write access (creating command logs) is handled within other services' transactions.
type CommandService struct {
    commandRepo  repository.CommandRepository
    messageRepo  repository.MessageRepository
    convRepo     repository.ConversationRepository
    outboxRepo   repository.OutboxRepository
    db           repository.TxBeginner
    cfg          *config.Config
    logger       *logger.Logger
}

// NewCommandService creates a CommandService.
func NewCommandService(
    commandRepo repository.CommandRepository,
    messageRepo repository.MessageRepository,
    convRepo    repository.ConversationRepository,
    outboxRepo  repository.OutboxRepository,
    db          repository.TxBeginner,
    cfg         *config.Config,
    l           *logger.Logger,
) (*CommandService, error) {
    if commandRepo == nil || messageRepo == nil || convRepo == nil || outboxRepo == nil || db == nil {
        return nil, sentinal_errors.ErrServiceUnavailable
    }
    return &CommandService{
        commandRepo: commandRepo,
        messageRepo: messageRepo,
        convRepo:    convRepo,
        outboxRepo:  outboxRepo,
        db:          db,
        cfg:         cfg,
        logger:      l,
    }, nil
}

// GetByID returns a single command log if the requesting user owns it.
func (s *CommandService) GetByID(ctx context.Context, actorID, commandID uuid.UUID) (CommandLogView, error) {
    log, err := s.commandRepo.GetLogByID(ctx, commandID)
    if err != nil {
        return CommandLogView{}, err
    }
    if log.UserID != actorID {
        return CommandLogView{}, sentinal_errors.ErrForbidden
    }
    return s.toView(log), nil
}

// ListForUser returns recent command logs for the requesting user.
func (s *CommandService) ListForUser(ctx context.Context, actorID uuid.UUID, limit int) ([]CommandLogView, error) {
    if limit <= 0 || limit > 100 {
        limit = 50
    }
    logs, err := s.commandRepo.GetCommandsByUser(ctx, actorID, limit)
    if err != nil {
        return nil, err
    }
    views := make([]CommandLogView, 0, len(logs))
    for _, log := range logs {
        views = append(views, s.toView(log))
    }
    return views, nil
}

// Undo reverses a previously executed command if it is still within the undo window.
// The full reverse mutation is applied atomically with the status update and outbox event.
func (s *CommandService) Undo(ctx context.Context, actorID, commandID uuid.UUID) (CommandLogView, error) {
    log, err := s.commandRepo.GetLogByID(ctx, commandID)
    if err != nil {
        return CommandLogView{}, err
    }

    // 1. Ownership check
    if log.UserID != actorID {
        return CommandLogView{}, sentinal_errors.ErrForbidden
    }

    // 2. Status check
    if log.Status != command.StatusExecuted {
        return CommandLogView{}, ErrCommandNotUndoable
    }

    // 3. Already undone check
    if log.UndoneAt != nil {
        return CommandLogView{}, ErrCommandNotUndoable
    }

    // 4. Undo window check
    if log.ExecutedAt == nil {
        return CommandLogView{}, ErrCommandNotUndoable
    }
    window := time.Duration(s.cfg.CommandUndoWindowSeconds) * time.Second
    if window <= 0 {
        window = 300 * time.Second
    }
    if time.Since(*log.ExecutedAt) > window {
        return CommandLogView{}, ErrUndoWindowExpired
    }

    // 5. Execute the undo in a transaction
    now := time.Now().UTC()
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return CommandLogView{}, err
    }
    defer tx.Rollback()

    if err := s.applyUndo(ctx, tx, log); err != nil {
        return CommandLogView{}, err
    }

    // Update command log to UNDONE
    log.Status = command.StatusUndone
    log.UndoneAt = &now
    if err := s.commandRepo.UpdateLog(ctx, &log); err != nil {
        return CommandLogView{}, err
    }

    // Write outbox event
    payload, _ := json.Marshal(map[string]interface{}{
        "command_id":   log.ID.String(),
        "command_type": string(log.CommandType),
        "user_id":      actorID.String(),
        "undone_at":    now,
    })
    convIDStr := ""
    if log.ConversationID != nil {
        convIDStr = log.ConversationID.String()
    }
    if err := s.outboxRepo.Create(ctx, tx, &outbox.OutboxEvent{
        ID:            uuid.New(),
        EventType:     events.CommandUndone,
        AggregateType: "command",
        AggregateID:   log.ID.String(),
        Payload:       payload,
        Status:        outbox.StatusPending,
        CreatedAt:     now,
        UpdatedAt:     now,
    }); err != nil {
        return CommandLogView{}, err
    }
    _ = convIDStr // used in full implementation to also emit domain-level outbox event

    if err := tx.Commit(); err != nil {
        return CommandLogView{}, err
    }

    return s.toView(log), nil
}

// applyUndo executes the reverse mutation for the given command log.
// Called inside an active transaction.
func (s *CommandService) applyUndo(ctx context.Context, tx repository.DBTX, log command.CommandLog) error {
    switch log.CommandType {
    case command.CommandDeleteMessage:
        return s.undoDeleteMessage(ctx, tx, log.UndoPayload)
    case command.CommandEditMessage:
        return s.undoEditMessage(ctx, tx, log.UndoPayload)
    case command.CommandPinMessage:
        return s.undoPinMessage(ctx, tx, log.UndoPayload)
    case command.CommandUnpinMessage:
        return s.undoUnpinMessage(ctx, tx, log.UndoPayload)
    case command.CommandReactMessage:
        return s.undoReactMessage(ctx, tx, log.UndoPayload)
    case command.CommandClearChat:
        return s.undoClearChat(ctx, tx, log.UndoPayload)
    default:
        return sentinal_errors.ErrInvalidInput
    }
}

// --- Undo executors per command type ---

// undoDeleteMessage restores a soft-deleted message by clearing deleted_at.
func (s *CommandService) undoDeleteMessage(ctx context.Context, tx repository.DBTX, raw json.RawMessage) error {
    var p struct {
        MessageID string `json:"message_id"`
    }
    if err := json.Unmarshal(raw, &p); err != nil {
        return sentinal_errors.ErrInvalidInput
    }
    msgID, err := uuid.Parse(p.MessageID)
    if err != nil {
        return sentinal_errors.ErrInvalidInput
    }
    // Restore deleted_at = NULL
    _, err = tx.ExecContext(ctx,
        `UPDATE messages SET deleted_at = NULL, updated_at = NOW() WHERE id = $1`, msgID)
    return err
}

// undoEditMessage restores the previous encrypted content of a message.
func (s *CommandService) undoEditMessage(ctx context.Context, tx repository.DBTX, raw json.RawMessage) error {
    var p struct {
        MessageID               string     `json:"message_id"`
        PreviousEncryptedContent string    `json:"previous_encrypted_content"`
        PreviousEditedAt         *time.Time `json:"previous_edited_at"`
    }
    if err := json.Unmarshal(raw, &p); err != nil {
        return sentinal_errors.ErrInvalidInput
    }
    msgID, err := uuid.Parse(p.MessageID)
    if err != nil {
        return sentinal_errors.ErrInvalidInput
    }

    if p.PreviousEncryptedContent == "" {
        // Edge case: original message had no content (e.g. media-only); set NULL.
        _, err = tx.ExecContext(ctx,
            `UPDATE messages SET encrypted_content = NULL, edited_at = $1, updated_at = NOW() WHERE id = $2`,
            p.PreviousEditedAt, msgID)
    } else {
        _, err = tx.ExecContext(ctx,
            `UPDATE messages SET encrypted_content = $1, edited_at = $2, updated_at = NOW() WHERE id = $3`,
            p.PreviousEncryptedContent, p.PreviousEditedAt, msgID)
    }
    return err
}

// undoPinMessage removes a pinned message (reverses a PIN_MESSAGE command).
func (s *CommandService) undoPinMessage(ctx context.Context, tx repository.DBTX, raw json.RawMessage) error {
    var p struct {
        ConversationID string `json:"conversation_id"`
        MessageID      string `json:"message_id"`
    }
    if err := json.Unmarshal(raw, &p); err != nil {
        return sentinal_errors.ErrInvalidInput
    }
    convID, err := uuid.Parse(p.ConversationID)
    if err != nil {
        return sentinal_errors.ErrInvalidInput
    }
    msgID, err := uuid.Parse(p.MessageID)
    if err != nil {
        return sentinal_errors.ErrInvalidInput
    }
    _, err = tx.ExecContext(ctx,
        `DELETE FROM pinned_messages WHERE conversation_id = $1 AND message_id = $2`, convID, msgID)
    return err
}

// undoUnpinMessage re-inserts a pinned message row (reverses an UNPIN_MESSAGE command).
func (s *CommandService) undoUnpinMessage(ctx context.Context, tx repository.DBTX, raw json.RawMessage) error {
    var p struct {
        ConversationID   string    `json:"conversation_id"`
        MessageID        string    `json:"message_id"`
        PinnedBy         string    `json:"pinned_by"`
        PinnedAt         time.Time `json:"pinned_at"`
    }
    if err := json.Unmarshal(raw, &p); err != nil {
        return sentinal_errors.ErrInvalidInput
    }
    convID, _ := uuid.Parse(p.ConversationID)
    msgID, _  := uuid.Parse(p.MessageID)
    pinnedBy, _ := uuid.Parse(p.PinnedBy)

    _, err := tx.ExecContext(ctx, `
        INSERT INTO pinned_messages (conversation_id, message_id, pinned_by, pinned_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (conversation_id, message_id) DO NOTHING
    `, convID, msgID, pinnedBy, p.PinnedAt)
    return err
}

// undoReactMessage removes a reaction (reverses a REACT_MESSAGE command).
func (s *CommandService) undoReactMessage(ctx context.Context, tx repository.DBTX, raw json.RawMessage) error {
    var p struct {
        MessageID    string `json:"message_id"`
        UserID       string `json:"user_id"`
        ReactionCode string `json:"reaction_code"`
    }
    if err := json.Unmarshal(raw, &p); err != nil {
        return sentinal_errors.ErrInvalidInput
    }
    msgID, _  := uuid.Parse(p.MessageID)
    userID, _ := uuid.Parse(p.UserID)

    _, err := tx.ExecContext(ctx, `
        DELETE FROM message_reactions
        WHERE message_id = $1 AND user_id = $2 AND reaction_code = $3
    `, msgID, userID, p.ReactionCode)
    return err
}

// undoClearChat restores or removes a conversation_clears row.
func (s *CommandService) undoClearChat(ctx context.Context, tx repository.DBTX, raw json.RawMessage) error {
    var p struct {
        ConversationID    string     `json:"conversation_id"`
        UserID            string     `json:"user_id"`
        PreviousClearedAt *time.Time `json:"previous_cleared_at"`
    }
    if err := json.Unmarshal(raw, &p); err != nil {
        return sentinal_errors.ErrInvalidInput
    }
    convID, _ := uuid.Parse(p.ConversationID)
    userID, _ := uuid.Parse(p.UserID)

    if p.PreviousClearedAt == nil {
        // There was no clear before the command — delete the row entirely.
        _, err := tx.ExecContext(ctx, `
            DELETE FROM conversation_clears
            WHERE conversation_id = $1 AND user_id = $2
        `, convID, userID)
        return err
    }

    // Restore the previous cleared_at value.
    _, err := tx.ExecContext(ctx, `
        UPDATE conversation_clears
        SET cleared_at = $1
        WHERE conversation_id = $2 AND user_id = $3
    `, p.PreviousClearedAt, convID, userID)
    return err
}

// toView converts a domain CommandLog to a CommandLogView.
func (s *CommandService) toView(log command.CommandLog) CommandLogView {
    window := time.Duration(s.cfg.CommandUndoWindowSeconds) * time.Second
    if window <= 0 {
        window = 300 * time.Second
    }

    canUndo := false
    var undoExpiresAt *time.Time

    if log.Status == command.StatusExecuted &&
        log.UndoneAt == nil &&
        log.ExecutedAt != nil &&
        time.Since(*log.ExecutedAt) <= window {
        canUndo = true
        exp := log.ExecutedAt.Add(window)
        undoExpiresAt = &exp
    }

    convIDStr := (*string)(nil)
    if log.ConversationID != nil {
        s := log.ConversationID.String()
        convIDStr = &s
    }

    return CommandLogView{
        ID:              log.ID.String(),
        CommandType:     string(log.CommandType),
        UserID:          log.UserID.String(),
        ConversationID:  convIDStr,
        Status:          string(log.Status),
        Payload:         log.Payload,
        UndoPayload:     log.UndoPayload,
        ErrorMessage:    log.ErrorMessage,
        ExecutionTimeMs: log.ExecutionTimeMs,
        CreatedAt:       log.CreatedAt,
        ExecutedAt:      log.ExecutedAt,
        UndoneAt:        log.UndoneAt,
        CanUndo:         canUndo,
        UndoExpiresAt:   undoExpiresAt,
    }
}
```

---

## 11. Go Implementation: Command Handler

Create `internal/handler/command_handler.go`:

```go
// internal/handler/command_handler.go
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

// CommandHandler provides HTTP handlers for command log access and undo.
type CommandHandler struct {
    service *services.CommandService
    logger  *logger.Logger
}

// NewCommandHandler creates a CommandHandler.
func NewCommandHandler(service *services.CommandService, l *logger.Logger) *CommandHandler {
    return &CommandHandler{service: service, logger: l}
}

// RegisterRoutes attaches command routes to a protected router group.
func (h *CommandHandler) RegisterRoutes(router gin.IRouter) {
    router.GET("/commands",      h.List)
    router.GET("/commands/:id",  h.GetByID)
    router.POST("/commands/:id/undo", h.Undo)
}

// List returns recent command logs for the authenticated user.
//
// GET /v1/commands?limit=50
func (h *CommandHandler) List(c *gin.Context) {
    actorID, ok := h.mustUserID(c)
    if !ok {
        return
    }

    limit := parsePositiveIntQuery(c, "limit", 50)

    views, err := h.service.ListForUser(c.Request.Context(), actorID, limit)
    if err != nil {
        h.writeError(c, err)
        return
    }

    items := make([]httpdto.CommandLogPayload, 0, len(views))
    for _, v := range views {
        items = append(items, toCommandPayload(v))
    }

    httpdto.WriteSuccess(c, http.StatusOK, httpdto.CommandListPayload{
        Items: items,
        Total: len(items),
        Limit: limit,
    })
}

// GetByID returns a single command log by ID.
//
// GET /v1/commands/:id
func (h *CommandHandler) GetByID(c *gin.Context) {
    actorID, ok := h.mustUserID(c)
    if !ok {
        return
    }

    commandID, err := parseUUIDParam(c, "id")
    if err != nil {
        h.writeError(c, sentinal_errors.ErrInvalidInput)
        return
    }

    view, err := h.service.GetByID(c.Request.Context(), actorID, commandID)
    if err != nil {
        h.writeError(c, err)
        return
    }

    httpdto.WriteSuccess(c, http.StatusOK, toCommandPayload(view))
}

// Undo reverses a previously executed command if it is still within the undo window.
//
// POST /v1/commands/:id/undo
func (h *CommandHandler) Undo(c *gin.Context) {
    actorID, ok := h.mustUserID(c)
    if !ok {
        return
    }

    commandID, err := parseUUIDParam(c, "id")
    if err != nil {
        h.writeError(c, sentinal_errors.ErrInvalidInput)
        return
    }

    view, err := h.service.Undo(c.Request.Context(), actorID, commandID)
    if err != nil {
        h.writeError(c, err)
        return
    }

    httpdto.WriteSuccess(c, http.StatusOK, httpdto.UndoCommandPayload{
        CommandID:   view.ID,
        Status:      view.Status,
        UndoneAt:    *view.UndoneAt,
        CommandType: view.CommandType,
    })
}

// mustUserID extracts user_id set by AuthMiddleware. Writes 401 and returns false if missing.
func (h *CommandHandler) mustUserID(c *gin.Context) (uuid.UUID, bool) {
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

// writeError maps service errors to HTTP status codes.
func (h *CommandHandler) writeError(c *gin.Context, err error) {
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
    case errors.Is(err, services.ErrCommandNotUndoable):
        status = http.StatusConflict
        code = "COMMAND_NOT_UNDOABLE"
        message = "command cannot be undone"
    case errors.Is(err, services.ErrUndoWindowExpired):
        status = http.StatusConflict
        code = "UNDO_WINDOW_EXPIRED"
        message = "undo window has expired"
    }

    if status >= http.StatusInternalServerError && h.logger != nil {
        h.logger.Errorf("command handler error: %v", err)
    }

    httpdto.WriteError(c, status, message, code)
}

// toCommandPayload converts a service view to a DTO payload.
func toCommandPayload(v services.CommandLogView) httpdto.CommandLogPayload {
    return httpdto.CommandLogPayload{
        ID:              v.ID,
        CommandType:     v.CommandType,
        UserID:          v.UserID,
        ConversationID:  v.ConversationID,
        Status:          v.Status,
        Payload:         v.Payload,
        UndoPayload:     v.UndoPayload,
        ErrorMessage:    v.ErrorMessage,
        ExecutionTimeMs: v.ExecutionTimeMs,
        CreatedAt:       v.CreatedAt,
        ExecutedAt:      v.ExecutedAt,
        UndoneAt:        v.UndoneAt,
        CanUndo:         v.CanUndo,
        UndoExpiresAt:   v.UndoExpiresAt,
    }
}
```

---

## 12. Outbox Integration

### How other services write command logs + outbox in one transaction

The `CommandService` only handles **read** and **undo**. The actual command creation happens
inside `MessageService`, `ConversationService`, etc. Here is the exact pattern to follow
inside `MessageService.DeleteMessage`:

```go
// Inside MessageService.DeleteMessage — abbreviated example

func (s *MessageService) DeleteMessage(ctx context.Context, actorID, messageID uuid.UUID) error {
    msg, err := s.messageRepo.GetByID(ctx, messageID)
    if err != nil {
        return err
    }

    // Permission check
    ok, err := s.convRepo.IsParticipant(ctx, msg.ConversationID, actorID)
    if err != nil || !ok {
        return sentinal_errors.ErrForbidden
    }

    now := time.Now().UTC()

    // Build command payload and undo payload
    cmdPayload, _ := json.Marshal(map[string]interface{}{
        "message_id":      messageID.String(),
        "conversation_id": msg.ConversationID.String(),
        "deleted_by":      actorID.String(),
        "deleted_at":      now,
    })
    undoPayload, _ := json.Marshal(map[string]interface{}{
        "message_id": messageID.String(),
    })

    // Begin transaction
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 1. Insert command log as PENDING
    cmdLog := &command.CommandLog{
        ID:             uuid.New(),
        CommandType:    command.CommandDeleteMessage,
        UserID:         actorID,
        ConversationID: &msg.ConversationID,
        Status:         command.StatusPending,
        Payload:        cmdPayload,
        UndoPayload:    undoPayload,
        CreatedAt:      now,
    }
    if err := s.commandRepo.CreateLog(ctx, cmdLog); err != nil {
        return err
    }

    // 2. Perform domain mutation — soft delete the message
    if err := s.messageRepo.SoftDelete(ctx, messageID); err != nil {
        cmdLog.Status = command.StatusFailed
        cmdLog.ErrorMessage = err.Error()
        _ = s.commandRepo.UpdateLog(ctx, cmdLog)
        return err
    }

    // 3. Update command log to EXECUTED
    executed := now
    cmdLog.Status = command.StatusExecuted
    cmdLog.ExecutedAt = &executed
    if err := s.commandRepo.UpdateLog(ctx, cmdLog); err != nil {
        return err
    }

    // 4. Write outbox event
    outboxPayload, _ := json.Marshal(map[string]interface{}{
        "message_id":      messageID.String(),
        "conversation_id": msg.ConversationID.String(),
        "deleted_by":      actorID.String(),
        "deleted_at":      now,
    })
    if err := s.outboxRepo.Create(ctx, tx, &outbox.OutboxEvent{
        ID:            uuid.New(),
        EventType:     events.MessageDeleted,
        AggregateType: "message",
        AggregateID:   messageID.String(),
        Payload:       outboxPayload,
        Status:        outbox.StatusPending,
        CreatedAt:     now,
        UpdatedAt:     now,
    }); err != nil {
        return err
    }

    return tx.Commit()
}
```

Follow this exact same pattern for `EditMessage`, `PinMessage`, `UnpinMessage`,
`AddReaction`, and `ClearChat`.

---

## 13. HTTP Endpoints

### `GET /v1/commands?limit=50`

- **Auth:** Required
- **Query params:** `limit` (int, default 50, max 100)
- **Returns:** List of command logs for the authenticated user, newest first.

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "uuid",
        "command_type": "DELETE_MESSAGE",
        "user_id": "uuid",
        "conversation_id": "uuid",
        "status": "EXECUTED",
        "payload": { "message_id": "uuid", "conversation_id": "uuid", "deleted_by": "uuid", "deleted_at": "2025-01-01T10:00:00Z" },
        "undo_payload": { "message_id": "uuid" },
        "error_message": "",
        "execution_time_ms": 12,
        "created_at": "2025-01-01T10:00:00Z",
        "executed_at": "2025-01-01T10:00:00Z",
        "undone_at": null,
        "can_undo": true,
        "undo_expires_at": "2025-01-01T10:05:00Z"
      }
    ],
    "total": 1,
    "limit": 50
  }
}
```

---

### `GET /v1/commands/:id`

- **Auth:** Required
- **Returns:** Single command log. Returns `403` if the command belongs to another user.

---

### `POST /v1/commands/:id/undo`

- **Auth:** Required
- **Body:** None
- **Returns:**

```json
{
  "success": true,
  "data": {
    "command_id":   "uuid",
    "status":       "UNDONE",
    "undone_at":    "2025-01-01T10:02:30Z",
    "command_type": "DELETE_MESSAGE"
  }
}
```

**Error responses:**

| Condition | HTTP | Code |
|---|---|---|
| Command not found | 404 | `NOT_FOUND` |
| Command belongs to another user | 403 | `FORBIDDEN` |
| Status is not `EXECUTED` | 409 | `COMMAND_NOT_UNDOABLE` |
| Undo window expired | 409 | `UNDO_WINDOW_EXPIRED` |
| Command already undone | 409 | `COMMAND_NOT_UNDOABLE` |

---

## 14. Wiring into cmd/api/main.go

After creating all the files above, wire the handler into `cmd/api/main.go`:

```go
// cmd/api/main.go  (abbreviated — add to existing wiring)

package main

import (
    // ... existing imports ...
    "sentinal-chat/internal/handler"
    "sentinal-chat/internal/services"
)

func main() {
    // ... existing setup: cfg, logger, db, repos ...

    // --- Command service ---
    commandSvc, err := services.NewCommandService(
        commandRepo,
        messageRepo,
        convRepo,
        outboxRepo,
        db,       // implements repository.TxBeginner
        cfg,
        l,
    )
    if err != nil {
        l.Fatalf("command service: %v", err)
    }

    commandHandler := handler.NewCommandHandler(commandSvc, l)

    // Protected router group (auth middleware applied)
    protected := srv.Engine().Group("/v1")
    protected.Use(middleware.AuthMiddleware(
        authSvc.ParseAccessToken,
        authSvc.ValidateAccessSession,
    ))

    // ... register other handlers ...
    commandHandler.RegisterRoutes(protected)
}
```

### Environment variable to add

Add to your `.env` and `config/config.go`:

```env
COMMAND_UNDO_WINDOW_SECONDS=300
```

```go
// config/config.go
CommandUndoWindowSeconds int  // default 300
```

---

## Summary Checklist

Before shipping the command system, verify:

- [ ] `internal/domain/command/command.go` — enum constants fixed (`EXECUTED` not `COMPLETED`, no `EXECUTING`)
- [ ] `internal/repository/command_repository.go` — `CanUndo` checks `status = 'EXECUTED'`, no string `"COMPLETED"`
- [ ] `internal/transport/httpdto/command_dto.go` — created
- [ ] `internal/services/command_models.go` — created
- [ ] `internal/services/command_service.go` — created with all undo executors
- [ ] `internal/handler/command_handler.go` — created
- [ ] `config/config.go` — `CommandUndoWindowSeconds` field added
- [ ] `cmd/api/main.go` — `CommandHandler` registered on protected group
- [ ] Every write method in `MessageService` and `ConversationService` that maps to a command type creates a `command_logs` row + outbox row in the same transaction