# Agents Guide

This document defines how agents should extend Sentinal Chat without changing the overall structure. It is based on the current codebase and `docs/database.md` and is the primary guidance for event-driven flows (outbox + Redis + WebSockets) and design pattern usage.

## Codebase Structure Map

```
cmd/
  api/main.go
  migrate/main.go
config/
internal/
  domain/
  handler/
  middleware/
  repository/
  server/
  services/
  transport/httpdto/
pkg/
  database/
  errors/
  logger/
docs/
  database.md
```

## Event-Driven Outbox Flow (Authoritative)

### Transaction Rule
All state-changing commands must write domain state and an outbox event in the same database transaction. This guarantees durability and prevents missing events.

### Event Emission Pipeline

1. Handler receives request.
2. Service validates and builds a command.
3. Repository writes data.
4. Repository creates outbox record.
5. Worker publishes to Redis.
6. WebSocket observers fan-out to clients.

### Outbox Tables
- `outbox_events` is the primary event queue.
- `outbox_event_deliveries` tracks delivery attempts and status.
- `command_log` stores idempotency and audit trail.

### Redis Channel Taxonomy

```
channel:conversation:{conversation_id}
channel:user:{user_id}
channel:presence:{user_id}
channel:call:{call_id}
channel:system:outbox
```

### Event Envelope

```
{
  "event_type": "message.created",
  "aggregate_type": "message",
  "aggregate_id": "uuid",
  "occurred_at": "2026-01-16T10:00:00Z",
  "payload": { ... }
}
```

## Required Design Patterns

### 1) Observer Pattern (WebSockets)

Use Observer for WebSocket fan-out. The WebSocket hub is the Subject, each connection is an Observer. Redis Pub/Sub subscriptions feed into the hub and are delivered to all observers.

Where to use:
- `internal/interfaces/websocket` or `internal/server` WebSocket handlers.
- Hub holds subscribers by conversation/user/channel.
- Each event published from Redis triggers a notify loop on observers.

Implementation guidance:
- Define a `Hub` that registers/unregisters clients.
- Each `Client` has a `Send` channel.
- Use `Publish(event)` to notify all clients in a room.

### 2) Command Pattern

Use Command pattern for all state-changing operations. Commands are validated objects passed to a handler that executes the transaction and writes the outbox event.

Where to use:
- `internal/services` and `internal/repository` for commands like:
  - `SendMessageCommand`
  - `CreateConversationCommand`
  - `UpdateProfileCommand`

Implementation guidance:
- Create command structs with `Validate` methods.
- Use a command handler that:
  - checks permissions
  - writes data
  - writes outbox event
  - writes command_log entry

### 3) Proxy Pattern

Use Proxy for access control, rate limiting, caching, and security checks before executing commands.

Where to use:
- `internal/middleware` for HTTP-level proxies (auth, rate limit).
- `internal/services` for service-level proxies (permission checks).
- `access_policies` table for permission decisions.

Implementation guidance:
- Create proxy services that wrap command handlers.
- Example: `AccessControlProxy` checks `access_policies` before allowing `SendMessageCommand`.

## Transport Layer Standards

All request and response DTOs must live in `internal/transport/httpdto`.

Required response shape:

```
type Response[T any] struct {
  Success bool
  Data    T
  Error   string
  Code    string
}
```

Handlers must respond with:
- `NewSuccessResponse(data)` on success
- `NewErrorResponse(message, code)` on failure

## Required Event Emission Rules

- Every command that changes state must emit at least one outbox event.
- Do not publish directly to Redis inside transaction.
- Only worker services publish from outbox.
- WebSocket events must flow only from Redis to hubs.

## Agent Notes for Future Work

- Add worker process for outbox polling (batch by `created_at` where `processed_at` is null).
- Implement Redis publisher and subscriber in `internal/infrastructure` when needed.
- Implement WebSocket hub in `internal/interfaces` or `internal/server`.
- Maintain full API response conformity with `httpdto.Response`.
