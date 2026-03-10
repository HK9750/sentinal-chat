# Commands

This file defines the command pattern for chat actions, especially actions that can be undone.

Current repo reality:

- `command_logs` table exists.
- command repository exists.
- SQL comments describe undo semantics.
- actual command execution layer does not exist yet.
- undo endpoint does not exist yet.

## 1. Why commands exist

Commands give the system a durable record of state-changing chat actions.

They are useful for:

- undo support
- audit trails
- measuring execution success/failure
- tying chat actions to outbox events

## 2. Current SQL model

```sql
CREATE TABLE IF NOT EXISTS command_logs (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    command_type        VARCHAR(50) NOT NULL,
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id     UUID REFERENCES conversations(id) ON DELETE CASCADE,
    status              command_status DEFAULT 'PENDING',
    payload             JSONB NOT NULL,
    undo_payload        JSONB,
    error_message       TEXT,
    execution_time_ms   INTEGER,
    created_at          TIMESTAMP DEFAULT NOW(),
    executed_at         TIMESTAMP,
    undone_at           TIMESTAMP
);
```

## 3. Command types already documented in SQL comments

These are the supported command names already described by the schema comments:

- `DELETE_MESSAGE`
- `EDIT_MESSAGE`
- `PIN_MESSAGE`
- `UNPIN_MESSAGE`
- `REACT_MESSAGE`
- `CLEAR_CHAT`

`command_type` is only `VARCHAR(50)`, so the DB does not enforce this list. The application must.

## 4. Current status mismatch that must be fixed

### SQL enum

- `PENDING`
- `EXECUTED`
- `FAILED`
- `UNDONE`

### Go constants today

- `PENDING`
- `EXECUTING`
- `COMPLETED`
- `FAILED`
- `UNDONE`

### Required fix

Pick one of these and standardize everywhere:

#### Option A: keep SQL as source of truth

- use only:
  - `PENDING`
  - `EXECUTED`
  - `FAILED`
  - `UNDONE`
- remove `EXECUTING` and `COMPLETED` from Go

#### Option B: migrate SQL enum

- if you really want `EXECUTING` and `COMPLETED`, migrate database enum and all repository logic

Recommended choice:

- use Option A, because it matches the current schema and is simpler

## 5. Recommended command lifecycle

Use this lifecycle:

- `PENDING`
  - command log row created but action not finished yet
- `EXECUTED`
  - state mutation completed successfully
- `FAILED`
  - command could not complete; inspect `error_message`
- `UNDONE`
  - previously executed action was reversed successfully

## 6. Recommended execution pipeline

For every command-backed action:

1. handler validates request
2. service builds command payload and undo payload
3. start DB transaction
4. insert `command_logs` row as `PENDING`
5. perform domain write
6. update command row to `EXECUTED`
7. insert outbox row in same transaction
8. commit transaction
9. outbox worker publishes realtime event

If any step fails:

- update command row to `FAILED`
- store `error_message`
- rollback transaction if main write has not committed

## 7. Recommended HTTP endpoints for command logs

### GET `/v1/commands?limit=50`

- Auth: yes
- Does:
  - returns recent command logs for current user

### GET `/v1/commands/:id`

- Auth: yes
- Does:
  - returns one command log

### POST `/v1/commands/:id/undo`

- Auth: yes
- Does:
  - validates ownership
  - validates undo window
  - validates current status is `EXECUTED`
  - applies reverse mutation using `undo_payload`
  - marks command `UNDONE`
  - writes outbox event

## 8. Undo rules

SQL comments say undo should be allowed within a time window, but there is no DB field for the window.

### Recommended rule

- define `COMMAND_UNDO_WINDOW_SECONDS` in config
- default to `300` seconds

### Recommended `CanUndo` logic

A command can be undone only if:

- command belongs to current user
- status is `EXECUTED`
- `undone_at IS NULL`
- `executed_at IS NOT NULL`
- `now - executed_at <= undo_window`

### Current repo limitation

The existing `CanUndo` method only checks:

- same user
- status equals Go `COMPLETED`
- `undone_at == nil`

That is not enough and is currently mismatched with the DB enum.

## 9. Payload design by command type

The command system is only useful if `payload` and `undo_payload` are precise.

### `DELETE_MESSAGE`

When used:

- soft-delete a message

`payload`:

```json
{
  "message_id": "uuid",
  "conversation_id": "uuid",
  "deleted_by": "uuid",
  "deleted_at": "2026-03-10T10:00:00Z"
}
```

`undo_payload`:

```json
{
  "message_id": "uuid",
  "restore_deleted_at_to": null
}
```

Execute does:

- set `messages.deleted_at = now`

Undo does:

- set `messages.deleted_at = null`

Realtime event:

- `message:deleted`
- undo emits `command:undone` and optionally `message:restored`

### `EDIT_MESSAGE`

When used:

- update encrypted content

`payload`:

```json
{
  "message_id": "uuid",
  "conversation_id": "uuid",
  "new_encrypted_content": "ciphertext",
  "edited_by": "uuid",
  "edited_at": "2026-03-10T10:01:00Z"
}
```

`undo_payload`:

```json
{
  "message_id": "uuid",
  "previous_encrypted_content": "old-ciphertext",
  "previous_edited_at": null
}
```

Execute does:

- insert row into `message_edits`
- update `messages.encrypted_content`
- set `messages.edited_at`

Undo does:

- restore previous encrypted content
- optionally write another history row for audit

Realtime event:

- `message:updated`

### `PIN_MESSAGE`

When used:

- pin a message in a conversation

`payload`:

```json
{
  "conversation_id": "uuid",
  "message_id": "uuid",
  "pinned_by": "uuid",
  "pinned_at": "2026-03-10T10:02:00Z"
}
```

`undo_payload`:

```json
{
  "conversation_id": "uuid",
  "message_id": "uuid"
}
```

Execute does:

- insert into `pinned_messages`

Undo does:

- delete from `pinned_messages`

Realtime event:

- `message:pinned`

### `UNPIN_MESSAGE`

When used:

- remove a pinned message

`payload`:

```json
{
  "conversation_id": "uuid",
  "message_id": "uuid",
  "unpinned_by": "uuid",
  "unpinned_at": "2026-03-10T10:03:00Z"
}
```

`undo_payload`:

```json
{
  "conversation_id": "uuid",
  "message_id": "uuid",
  "pinned_by": "uuid",
  "pinned_at": "2026-03-10T10:02:00Z"
}
```

Execute does:

- delete from `pinned_messages`

Undo does:

- recreate `pinned_messages` row

Realtime event:

- `message:unpinned`

### `REACT_MESSAGE`

When used:

- add a reaction

`payload`:

```json
{
  "message_id": "uuid",
  "conversation_id": "uuid",
  "user_id": "uuid",
  "reaction_code": ":thumbsup:"
}
```

`undo_payload`:

```json
{
  "message_id": "uuid",
  "user_id": "uuid",
  "reaction_code": ":thumbsup:"
}
```

Execute does:

- insert into `message_reactions`

Undo does:

- delete the reaction row

Realtime event:

- `reaction:added`
- undo may emit `reaction:removed`

### `CLEAR_CHAT`

When used:

- hide prior history for one user in one conversation

`payload`:

```json
{
  "conversation_id": "uuid",
  "user_id": "uuid",
  "cleared_at": "2026-03-10T10:04:00Z"
}
```

`undo_payload`:

```json
{
  "conversation_id": "uuid",
  "user_id": "uuid",
  "previous_cleared_at": null
}
```

Execute does:

- insert or update `conversation_clears`

Undo does:

- restore previous `cleared_at` value, or remove row if previous value was null

Realtime event:

- `conversation:cleared`

## 10. Command log response shape

Recommended response for command endpoints:

```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "command_type": "DELETE_MESSAGE",
    "user_id": "uuid",
    "conversation_id": "uuid",
    "status": "EXECUTED",
    "payload": {},
    "undo_payload": {},
    "error_message": "",
    "execution_time_ms": 14,
    "created_at": "2026-03-10T10:00:00Z",
    "executed_at": "2026-03-10T10:00:00Z",
    "undone_at": null,
    "can_undo": true,
    "undo_expires_at": "2026-03-10T10:05:00Z"
  }
}
```

## 11. Recommended indexing and querying behavior

Current indexes already support common queries:

- `idx_command_logs_user_created`
- `idx_command_logs_conv`
- `idx_command_logs_status`

That means useful product queries are:

- recent commands by current user
- pending commands for workers
- commands for a conversation timeline

## 12. Recommended outbox integration for commands

Every command execution should also emit an outbox event.

### On success

- `command:executed`

### On undo

- `command:undone`

### On failure

- `command:failed`

Example outbox row for a delete command:

```json
{
  "event_type": "command:executed",
  "aggregate_type": "command",
  "aggregate_id": "command-uuid",
  "payload": {
    "command_type": "DELETE_MESSAGE",
    "message_id": "message-uuid",
    "conversation_id": "conversation-uuid"
  }
}
```

## 13. Validation rules to enforce in services

- only the actor who created the command can undo it
- only supported `command_type` values are accepted
- referenced message/conversation must exist
- actor must still have permission in the target conversation
- undo window must not be expired
- commands already marked `UNDONE` cannot be undone again

## 14. Minimum implementation pieces still missing

Add these before command routes go live:

- command service layer
- enum alignment between SQL and Go
- actual undo executor per command type
- command DTOs
- command handlers
- outbox emission after command execution

## 15. One design recommendation

Do not let handlers write directly to domain tables and command logs separately.

Wrap each command-backed action in one service transaction so that:

- domain mutation
- command log mutation
- outbox insert

either all commit together or all fail together.
