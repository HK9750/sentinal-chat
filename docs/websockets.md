# WebSockets

This file defines the websocket contract the project should implement.

Current repo reality:

- `README.md` documents websocket behavior.
- There is no websocket server implementation yet.
- There is no Redis Pub/Sub bridge yet.

So this document is the target contract to build.

## 1. Endpoint

### GET `/v1/ws`

- Current status: documented only
- Auth methods:
  - `Authorization: Bearer <access_token>` header
  - or `?token=<access_token>` query param
- Connection should be rejected with `401` if token is invalid.

## 2. Connection lifecycle

### On connect

Server should:

- authenticate token
- resolve `user_id`, `session_id`, `device_id`
- mark user online if desired
- subscribe connection to:
  - personal user channel
  - all conversation channels for conversations the user belongs to
  - active call channel if the user is in a live call
- send an initial `connection:ready` event

Example:

```json
{
  "type": "connection:ready",
  "data": {
    "user_id": "uuid",
    "session_id": "uuid-or-string",
    "device_id": "device-id",
    "server_time": "2026-03-10T10:00:00Z"
  }
}
```

### On disconnect

Server should:

- unregister connection
- possibly mark user offline if this was the last live connection
- clear ephemeral typing and call-presence state owned by that connection

## 3. Event envelope

Use one envelope for every inbound and outbound frame.

```json
{
  "type": "message:new",
  "request_id": "client-generated-optional-id",
  "conversation_id": "uuid-optional",
  "call_id": "uuid-optional",
  "data": {},
  "sent_at": "2026-03-10T10:00:00Z"
}
```

### Envelope fields

- `type`: event name
- `request_id`: optional client correlation id
- `conversation_id`: required for conversation-scoped events
- `call_id`: required for call-scoped events
- `data`: event payload
- `sent_at`: server timestamp on outbound events

## 4. Inbound client events

`README.md` explicitly mentions these inbound events:

- `typing:start`
- `typing:stop`
- `read`
- `ping`

The list below keeps those and adds one missing but necessary delivery event.

### `ping`

- Purpose:
  - keep connection alive
  - measure latency
- Payload:

```json
{
  "type": "ping",
  "data": {
    "client_time": "2026-03-10T10:00:00Z"
  }
}
```

- Server response:

```json
{
  "type": "pong",
  "data": {
    "client_time": "2026-03-10T10:00:00Z",
    "server_time": "2026-03-10T10:00:00Z"
  }
}
```

### `typing:start`

- Scope: conversation
- Payload:

```json
{
  "type": "typing:start",
  "conversation_id": "uuid",
  "data": {
    "thread_id": null
  }
}
```

- Server should:
  - validate current user is participant
  - set ephemeral typing state in Redis with short TTL
  - fan out `typing:started` to other participants only

### `typing:stop`

- Scope: conversation
- Payload:

```json
{
  "type": "typing:stop",
  "conversation_id": "uuid",
  "data": {
    "thread_id": null
  }
}
```

- Server should:
  - clear typing key
  - fan out `typing:stopped`

### `read`

- Scope: conversation or message batch
- Payload:

```json
{
  "type": "read",
  "conversation_id": "uuid",
  "data": {
    "message_ids": ["uuid-1", "uuid-2"],
    "up_to_seq_id": 144
  }
}
```

- Server should:
  - update message read receipts
  - update participant `last_read_sequence`
  - emit `message:read` event to other participants

### `delivered`

- Scope: conversation or message batch
- Status in repo: not documented as inbound, but needed
- Payload:

```json
{
  "type": "delivered",
  "conversation_id": "uuid",
  "data": {
    "message_ids": ["uuid-1", "uuid-2"]
  }
}
```

- Server should:
  - update delivered receipts
  - emit `message:delivered`

## 5. Outbound server events

`README.md` explicitly mentions these outbound events:

- `message:new`
- `message:read`
- `message:delivered`
- `typing:started`
- `typing:stopped`
- `call:offer`
- `call:answer`
- `call:ice`
- `call:ended`

### `message:new`

Sent when a new message is accepted and committed.

```json
{
  "type": "message:new",
  "conversation_id": "uuid",
  "data": {
    "message_id": "uuid",
    "seq_id": 145,
    "sender_id": "uuid",
    "type": "TEXT",
    "client_message_id": "client-key",
    "created_at": "2026-03-10T10:00:00Z"
  }
}
```

### `message:updated`

- Not documented in README, but strongly recommended.
- Use for edit flows.

```json
{
  "type": "message:updated",
  "conversation_id": "uuid",
  "data": {
    "message_id": "uuid",
    "edited_at": "2026-03-10T10:01:00Z"
  }
}
```

### `message:deleted`

- Not documented in README, but strongly recommended.
- Use for soft-delete flows.

```json
{
  "type": "message:deleted",
  "conversation_id": "uuid",
  "data": {
    "message_id": "uuid",
    "deleted_at": "2026-03-10T10:02:00Z"
  }
}
```

### `message:delivered`

```json
{
  "type": "message:delivered",
  "conversation_id": "uuid",
  "data": {
    "message_id": "uuid",
    "user_id": "recipient-uuid",
    "delivered_at": "2026-03-10T10:03:00Z"
  }
}
```

### `message:read`

```json
{
  "type": "message:read",
  "conversation_id": "uuid",
  "data": {
    "message_id": "uuid",
    "user_id": "reader-uuid",
    "read_at": "2026-03-10T10:04:00Z",
    "last_read_sequence": 145
  }
}
```

### `typing:started`

```json
{
  "type": "typing:started",
  "conversation_id": "uuid",
  "data": {
    "user_id": "uuid"
  }
}
```

### `typing:stopped`

```json
{
  "type": "typing:stopped",
  "conversation_id": "uuid",
  "data": {
    "user_id": "uuid"
  }
}
```

### `conversation:cleared`

- Recommended for clear-chat UX.

```json
{
  "type": "conversation:cleared",
  "conversation_id": "uuid",
  "data": {
    "user_id": "uuid",
    "cleared_at": "2026-03-10T10:05:00Z"
  }
}
```

### `reaction:added`

- Recommended for reactions.

```json
{
  "type": "reaction:added",
  "conversation_id": "uuid",
  "data": {
    "message_id": "uuid",
    "user_id": "uuid",
    "reaction_code": ":thumbsup:"
  }
}
```

### `reaction:removed`

- Recommended for reaction removal.

```json
{
  "type": "reaction:removed",
  "conversation_id": "uuid",
  "data": {
    "message_id": "uuid",
    "user_id": "uuid",
    "reaction_code": ":thumbsup:"
  }
}
```

### `message:pinned`

- Recommended for pinned message state.

```json
{
  "type": "message:pinned",
  "conversation_id": "uuid",
  "data": {
    "message_id": "uuid",
    "pinned_by": "uuid",
    "pinned_at": "2026-03-10T10:06:00Z"
  }
}
```

### `message:unpinned`

```json
{
  "type": "message:unpinned",
  "conversation_id": "uuid",
  "data": {
    "message_id": "uuid"
  }
}
```

## 6. Presence events

Not present in README, but needed for a chat product.

### `presence:online`

```json
{
  "type": "presence:online",
  "data": {
    "user_id": "uuid",
    "last_seen_at": null
  }
}
```

### `presence:offline`

```json
{
  "type": "presence:offline",
  "data": {
    "user_id": "uuid",
    "last_seen_at": "2026-03-10T10:07:00Z"
  }
}
```

## 7. Call signaling events

These should be routed by call id and by target user.

### `call:offer`

```json
{
  "type": "call:offer",
  "call_id": "uuid",
  "conversation_id": "uuid",
  "data": {
    "from_user_id": "uuid",
    "to_user_id": "uuid",
    "sdp": "offer-sdp"
  }
}
```

### `call:answer`

```json
{
  "type": "call:answer",
  "call_id": "uuid",
  "conversation_id": "uuid",
  "data": {
    "from_user_id": "uuid",
    "to_user_id": "uuid",
    "sdp": "answer-sdp"
  }
}
```

### `call:ice`

```json
{
  "type": "call:ice",
  "call_id": "uuid",
  "conversation_id": "uuid",
  "data": {
    "from_user_id": "uuid",
    "to_user_id": "uuid",
    "candidate": "candidate-string",
    "sdp_mid": "0",
    "sdp_mline_index": 0
  }
}
```

### `call:ringing`

- Recommended event.

```json
{
  "type": "call:ringing",
  "call_id": "uuid",
  "conversation_id": "uuid",
  "data": {
    "user_id": "callee-uuid"
  }
}
```

### `call:connected`

- Recommended event.

```json
{
  "type": "call:connected",
  "call_id": "uuid",
  "conversation_id": "uuid",
  "data": {
    "connected_at": "2026-03-10T10:08:00Z"
  }
}
```

### `call:ended`

```json
{
  "type": "call:ended",
  "call_id": "uuid",
  "conversation_id": "uuid",
  "data": {
    "ended_at": "2026-03-10T10:09:00Z",
    "reason": "COMPLETED",
    "duration_seconds": 420
  }
}
```

## 8. Redis channel mapping for websocket fan-out

Recommended canonical channels:

- `channel:user:{user_id}`
  - personal events
  - session/device updates
  - direct call signaling targeted to one user
- `channel:conversation:{conversation_id}`
  - message, typing, receipts, reactions, pins, clear-chat events
- `channel:call:{call_id}`
  - call lifecycle and signaling events
- `channel:presence:{user_id}`
  - presence state changes

## 9. Delivery rules

### Conversation events

- Send to all active participants except sender when appropriate.

### User-targeted events

- Send only to the target user's active sockets.

### Device-aware behavior

- If device id is important for E2EE fan-out, include it in event payload metadata.
- For example, message delivery acknowledgements may include receiving device id.

## 10. Reliability rules

- State-changing actions should not write directly to websocket connections only.
- They should:
  - commit DB change
  - write outbox event in same transaction
  - worker publishes event to Redis
  - websocket layer consumes Redis event and fans out to clients

## 11. Validation rules

- Reject frames missing required `conversation_id` or `call_id`.
- Reject events from users who are not conversation participants.
- Reject call signaling from users not in `call_participants`.
- Limit payload sizes.
- Apply rate limits to typing and signaling spam.

## 12. Errors

Send websocket error frames in a consistent envelope.

```json
{
  "type": "error",
  "request_id": "client-request-id",
  "data": {
    "code": "FORBIDDEN",
    "message": "user is not a participant in this conversation"
  }
}
```
