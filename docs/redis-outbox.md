# Redis and Outbox

This file defines the reliable event delivery model that should connect PostgreSQL writes, Redis Pub/Sub, and websocket fan-out.

Current repo reality:

- Outbox table exists.
- Outbox repository exists.
- Redis config exists.
- No Redis client exists yet.
- No outbox worker exists yet.
- No Redis publisher or subscriber exists yet.

So this is the implementation contract to build.

## 1. Why the outbox exists

The outbox pattern is needed so that:

- chat state changes are committed safely in PostgreSQL
- event publication is not lost if Redis is temporarily unavailable
- websocket delivery becomes eventually consistent and retryable

The rule should be:

- every state-changing action writes domain data and one or more `outbox_events` rows in the same DB transaction

## 2. Database table

Current schema:

```sql
CREATE TABLE IF NOT EXISTS outbox_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type      VARCHAR(50) NOT NULL,
    aggregate_type  VARCHAR(50) NOT NULL,
    aggregate_id    VARCHAR(36) NOT NULL,
    payload         JSONB NOT NULL,
    status          outbox_status DEFAULT 'PENDING',
    retry_count     INT DEFAULT 0,
    error           TEXT,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    processed_at    TIMESTAMP
);
```

## 3. Current outbox statuses

SQL enum and Go model agree on these values:

- `PENDING`
- `PROCESSING`
- `COMPLETED`
- `FAILED`

## 4. Current repository behavior

Already present in code:

- `Create(ctx, tx, event)`
  - inserts an outbox row
  - can use a caller-provided transaction
- `GetPending(limit)`
  - fetches oldest `PENDING` rows where `retry_count < 10`
- `MarkProcessing(id)`
- `MarkCompleted(id)`
- `MarkFailed(id, errorMsg)`
- `IncrementRetry(id)`

## 5. Required worker flow

The worker loop should work like this:

1. poll `outbox_events` for oldest pending rows
2. mark row `PROCESSING`
3. publish payload to one or more Redis Pub/Sub channels
4. if publish succeeds:
   - mark row `COMPLETED`
   - set `processed_at`
5. if publish fails:
   - increment `retry_count`
   - store error message
   - decide whether to return row to `PENDING` or leave it `FAILED`

### Recommended status behavior

Use this exact state machine:

- `PENDING -> PROCESSING`
- `PROCESSING -> COMPLETED`
- `PROCESSING -> PENDING` when retryable failure happens and retry budget remains
- `PROCESSING -> FAILED` when retry budget is exhausted or error is permanent

Note:

- the current repository has no helper to move `FAILED` back to `PENDING`
- add that if you want operator-driven replay

## 6. Recommended event envelope

Publish this structure to Redis:

```json
{
  "outbox_id": "uuid",
  "event_type": "message:new",
  "aggregate_type": "message",
  "aggregate_id": "message-uuid",
  "conversation_id": "conversation-uuid",
  "user_id": "actor-uuid",
  "occurred_at": "2026-03-10T10:00:00Z",
  "payload": {}
}
```

### Envelope field meaning

- `outbox_id`: original DB outbox row id
- `event_type`: logical event name consumed by subscribers and websocket layer
- `aggregate_type`: entity type, for example `message`, `conversation`, `call`, `command`
- `aggregate_id`: entity id as string
- `conversation_id`: include when event is conversation scoped
- `user_id`: acting user or target user depending on event
- `occurred_at`: authoritative server timestamp
- `payload`: event-specific details

## 7. Recommended Redis channel taxonomy

The repo guide in `Agents.md` already hints at a clean taxonomy. Use it.

### Pub/Sub channels

- `channel:conversation:{conversation_id}`
  - message events
  - receipts
  - reactions
  - typing
  - pinned/unpinned
  - clear chat
- `channel:user:{user_id}`
  - targeted user notifications
  - direct call invite or offer
  - session/device changes
- `channel:presence:{user_id}`
  - online/offline transitions
- `channel:call:{call_id}`
  - call lifecycle and signaling
- `channel:system:outbox`
  - optional operational stream for debugging and admin consumers

## 8. Recommended Redis key taxonomy

These keys are not in code yet, but they fit the project well.

### Presence keys

- `presence:user:{user_id}`
  - value: `online`
  - TTL: short, renewed by heartbeat

### Connection count keys

- `connections:user:{user_id}`
  - value: integer count of active sockets

### Typing keys

- `typing:conversation:{conversation_id}:user:{user_id}`
  - value: `1`
  - TTL: 5 to 10 seconds

### Call signaling transient keys

- `call:signal:{call_id}:user:{user_id}`
  - optional list or stream if temporary buffering is required

### Rate limit keys

- `ratelimit:auth:{ip}`
- `ratelimit:message:{user_id}`
- `ratelimit:call:{user_id}`

## 9. Recommended event type catalog

The outbox table stores free-form `event_type`, so define a canonical list early.

### Message domain

- `message:new`
- `message:updated`
- `message:deleted`
- `message:delivered`
- `message:read`
- `message:played`
- `reaction:added`
- `reaction:removed`
- `message:pinned`
- `message:unpinned`

### Conversation domain

- `conversation:created`
- `conversation:updated`
- `conversation:participant_added`
- `conversation:participant_removed`
- `conversation:participant_role_updated`
- `conversation:muted`
- `conversation:unmuted`
- `conversation:archived`
- `conversation:unarchived`
- `conversation:cleared`
- `conversation:invite_regenerated`

### Typing and presence domain

- `typing:started`
- `typing:stopped`
- `presence:online`
- `presence:offline`

### Call domain

- `call:created`
- `call:ringing`
- `call:offer`
- `call:answer`
- `call:ice`
- `call:connected`
- `call:participant_updated`
- `call:ended`

### Command domain

- `command:executed`
- `command:failed`
- `command:undone`

## 10. What each write flow should do

### New message flow

Inside one DB transaction:

- insert into `messages`
- insert mentions, attachments, poll rows as needed
- insert `outbox_events` row with:
  - `event_type = message:new`
  - `aggregate_type = message`
  - `aggregate_id = <message_id>`

Then worker publishes to:

- `channel:conversation:{conversation_id}`

### Read receipt flow

Inside one DB transaction:

- update or insert `message_receipts`
- update participant `last_read_sequence` if provided
- insert outbox row with `message:read`

Publish to:

- `channel:conversation:{conversation_id}`

### Soft delete flow

Inside one DB transaction:

- update `messages.deleted_at`
- insert command log
- insert outbox row with `message:deleted`

### Call signaling flow

For signaling messages that are ephemeral and not domain state:

- you may publish directly to Redis without DB outbox if durability is not needed
- if you want full reliability and auditability, also write an outbox row

Recommended split:

- call lifecycle events use outbox
- raw ICE candidate forwarding may bypass outbox if low-latency matters more than durability

## 11. Retry rules

Recommended policy:

- max retries: `10`
- backoff: exponential with jitter
- example schedule:
  - retry 1: 1s
  - retry 2: 2s
  - retry 3: 4s
  - retry 4: 8s
  - retry 5+: cap at 30s or 60s

Because the current schema does not store `next_retry_at`, you have two options:

- add a `next_retry_at` column
- or let worker sleep/backoff in memory only

Recommended choice:

- add `next_retry_at` for cleaner multi-instance behavior

## 12. Idempotency rules

### Why idempotency matters

- worker restarts can republish the same event
- Redis subscribers can reconnect and see repeated effects if consumers are not careful

### Recommended approach

- consumers should treat `outbox_id` as unique event id
- websocket hub should dedupe recent `outbox_id` values briefly in memory
- clients may also ignore duplicate event ids

## 13. Failure handling and dead-letter strategy

When event delivery fails permanently:

- mark row `FAILED`
- keep full error text in `error`
- expose metrics and an operator replay tool

Recommended admin operations:

- list failed outbox rows
- replay failed outbox row by id
- replay all failed rows for aggregate id

## 14. Observability

Track at least these metrics:

- pending outbox count
- processing outbox count
- failed outbox count
- average processing latency
- oldest pending event age
- publish success rate
- publish failure rate by event type

Useful log fields:

- `outbox_id`
- `event_type`
- `aggregate_type`
- `aggregate_id`
- `retry_count`
- `channel`
- `error`

## 15. Current schema/index details worth keeping

The current indexes are good and should stay:

- `idx_outbox_status` on `status`
- `idx_outbox_pending` on `(status, retry_count)` where `status = 'PENDING'`
- `idx_outbox_aggregate` on `(aggregate_type, aggregate_id)`

These support:

- fast worker polling
- debugging by aggregate
- targeted replay and tracing

## 16. Minimum code components to add

To make this real, add:

- `internal/redis/client.go`
- `internal/redis/pubsub.go`
- `internal/services/outbox_worker.go`
- `internal/events/types.go`
- `internal/events/publisher.go`
- websocket subscriber bridge in `internal/server`

## 17. One critical implementation rule

Never publish realtime chat state directly from HTTP handlers before the DB transaction commits.

Always do:

- DB state write
- outbox row write in same transaction
- async publish from worker

That rule is the whole point of the outbox.
