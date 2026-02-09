# Sentinal Chat System Flow

This document explains the end-to-end flow of the backend for lead engineers. It covers HTTP endpoints, WebSocket behavior, Redis pub/sub, and the command/proxy/observer patterns used in the codebase.

## Scope and conventions

- Base API prefix: `/v1`.
- All HTTP responses use the `httpdto.Response` envelope: `{ success, data, error, code }`.
- Auth: JWT access token in `Authorization: Bearer <token>`.
- Device context: access token carries `did` and Auth middleware adds `device_id` into request context.
- E2E: server stores ciphertext only; clients encrypt/decrypt; ciphertext is base64 in HTTP/WS.

## Core architecture and data flow

### HTTP request lifecycle

1) Middleware:
   - Request ID, CORS, logging, error handler.
   - Auth middleware validates JWT + session and injects `{user_id, session_id, device_id}` into context.
   - Rate limiting for auth endpoints and message/call creation.
2) Handler parses DTO, validates IDs, and calls the service.
3) Service either:
   - Executes a command via the command bus (which runs AccessControl proxy), or
   - Calls repositories directly.
4) Repository writes to Postgres.
5) Outbox events are written either by service helpers or DB triggers.
6) Outbox processor publishes events to Redis.
7) Redis bridge pushes payloads to WebSocket hubs by channel.

### WebSocket lifecycle

1) Client connects to `GET /v1/ws?token=...&device_id=...`.
2) Server validates token, registers client in hub, and marks presence online in Redis.
3) Auto-subscribes to:
   - `channel:user:{user_id}`
   - `channel:presence:{user_id}`
4) Client can subscribe/unsubscribe to additional channels using WS messages.
5) Command messages are parsed and executed via the command bus.
6) Redis pub/sub events are bridged into the hub and broadcast to subscribed clients.

### Command bus and proxy

- `commands.Bus` routes a command by `CommandType()` to a handler.
- The bus runs an `AccessControl` proxy before executing the handler.
- WebSocket commands always go through the bus.
- HTTP handlers may or may not go through the bus; some call services directly.

### Outbox and Redis

- Services use `createOutboxEvent` to insert `outbox_events` rows inside the same DB transaction.
- The outbox processor polls, wraps payloads in `events.Envelope`, and publishes to Redis.
- Message creation is special: DB trigger on `message_ciphertexts` inserts `message.created` events.

Envelope shape:

```json
{
  "event_type": "message.created",
  "aggregate_type": "message",
  "aggregate_id": "<uuid>",
  "occurred_at": "2026-01-16T10:00:00Z",
  "payload": { "...": "..." }
}
```

Routing (`outbox/processor.go`):

- `message.created` -> `channel:user:{recipient_user_id}` (uses payload field)
- Other message, receipt, reaction, poll -> `channel:conversation:{conversation_id}`
- Conversation/participant -> `channel:conversation:{conversation_id}`
- Call -> `channel:call:{call_id}`
- Presence -> `channel:presence:{user_id}`
- Encryption/user/upload -> `channel:user:{user_id}` (payload-based when available)
- Broadcast -> `channel:broadcast:{broadcast_id}`
- Default -> `channel:system:outbox`

Redis bridge subscribes to:

```
channel:user:*
channel:conversation:*
channel:call:*
channel:presence:*
channel:broadcast:*
channel:upload:*
channel:system:outbox
```

### E2E message model

- Clients encrypt per device using Signal-style sessions.
- `message_ciphertexts` stores ciphertext per recipient device.
- `message.created` events are emitted per ciphertext row by DB trigger.
- HTTP list returns device-scoped ciphertexts using `device_id` from auth context.
- Ciphertext is base64 in API and WebSocket payloads.

## WebSocket API

### Connect

`GET /v1/ws?token=<access_token>&device_id=<device_id>`

Flow:
- Validates token and session.
- Registers client in hub and presence store.
- Auto-subscribes to user and presence channels.
- Sends a `connected` response with client ID and subscriptions.

### Subscribe / Unsubscribe

Incoming:

```json
{ "type": "subscribe", "request_id": "...", "payload": { "channel": "channel:conversation:<id>" } }
```

Flow:
- Authorizer validates channel access (conversation membership, call participant, broadcast recipient, etc.).
- Hub subscribes/unsubscribes client.

### Command messages

Handled via command bus:

- `message.send` (typed)
- `message.read` (typed)
- `message.typing` (typed)
- `call.offer`, `call.answer`, `call.ice` (typed)
- `presence.update` (typed)

Other message types (e.g., `message.delete`, `message.react`, `message.star`) currently fall back to `SimpleCommand` with raw payload and require matching handlers; many are not fully wired in parsing logic.

### WebSocket message.send

Payload uses base64 ciphertexts per device:

```json
{
  "type": "message.send",
  "payload": {
    "conversation_id": "...",
    "ciphertexts": [
      { "recipient_device_id": "...", "ciphertext": "<base64>", "header": { "version": 1, "cipher": "signal" } }
    ],
    "message_type": "TEXT",
    "client_message_id": "...",
    "idempotency_key": "..."
  }
}
```

Flow:
- Server decodes ciphertexts, builds `SendMessageCommand`, executes via bus.
- `message.created` events are emitted by DB trigger, routed to user channels.

## HTTP endpoints and flows

### System

#### GET /ping

- No auth.
- Returns `{ message: "pong" }`.
- No outbox or Redis.

#### GET /health

- No auth.
- Runs DB health check; returns `healthy` or `UNHEALTHY`.

#### GET /goroutines

- No auth.
- Returns Go runtime goroutine count.

### Auth

#### POST /v1/auth/register

Flow:
- Validates request and identity uniqueness.
- Creates user, settings, device, and session.
- Returns access and refresh tokens.
Notes:
- Auth endpoints are IP rate limited.
- No outbox events.

#### POST /v1/auth/login

Flow:
- Validates credentials and device.
- Creates session and returns tokens.
Notes:
- Auth endpoints are IP rate limited.
- No outbox events.

#### POST /v1/auth/refresh

Flow:
- Validates session + refresh token.
- Rotates refresh token and returns new tokens.
- Revokes session on invalid token.

#### POST /v1/auth/logout

Flow:
- Revokes a single session.

#### POST /v1/auth/logout-all

Flow:
- Revokes all sessions for the user.

#### GET /v1/auth/sessions

Flow:
- Lists sessions for the current user.

#### POST /v1/auth/password/forgot

Flow:
- No-op if identity not found; returns success.

#### POST /v1/auth/password/reset

Flow:
- Updates password and revokes all sessions.

### Messages

#### POST /v1/messages

Auth: required (message rate limited).

Flow:
- Validates conversation ID and ciphertext array.
- Base64-decodes ciphertexts and builds `SendMessageCommand`.
- Command bus -> AccessControl -> MessageService.
- Creates message row and one `message_ciphertexts` row per device.
- DB trigger inserts `message.created` outbox events per device.

Outbox/Redis:
- `message.created` (trigger) -> `channel:user:{recipient_user_id}`.

Notes:
- Send ciphertexts for recipient devices and the sender's other devices.

#### GET /v1/messages

Auth: required.

Flow:
- Uses `device_id` from auth context.
- Queries `messages` joined to `message_ciphertexts` for that device.
- Returns base64 ciphertext, header, recipient_device_id.

Outbox/Redis: none.

#### GET /v1/messages/:id

- Not supported for E2E; returns `NOT_SUPPORTED`.

#### PUT /v1/messages/:id

- Not supported for E2E; returns `NOT_SUPPORTED`.

#### DELETE /v1/messages/:id

Flow:
- Soft deletes message (direct repo call).

Outbox/Redis: none (no event emitted in HTTP path).

#### DELETE /v1/messages/:id/hard

Flow:
- Hard deletes message (direct repo call).

Outbox/Redis: none.

#### POST /v1/messages/:id/read

Flow:
- Marks message as read (direct repo call).

Outbox/Redis: none (WS `message.read` emits events).

#### POST /v1/messages/:id/delivered

Flow:
- Marks message as delivered (direct repo call).

Outbox/Redis: none (WS `message.delivered` emits events).

### Conversations

#### POST /v1/conversations

Auth: required.

Flow:
- Validates participants and builds `CreateConversationCommand`.
- Command bus creates conversation and participants.
- Inserts `conversation.created` outbox event.

Outbox/Redis:
- `conversation.created` -> `channel:conversation:{conversation_id}`.

#### GET /v1/conversations

Flow:
- Lists conversations for current user.

#### GET /v1/conversations/:id

Flow:
- Fetches conversation by ID.

#### PUT /v1/conversations/:id

Flow:
- Updates conversation record (direct repo call).

Outbox/Redis: none (command bus `conversation.update_group` emits events).

#### DELETE /v1/conversations/:id

Flow:
- Deletes conversation record (direct repo call).

Outbox/Redis: none.

#### GET /v1/conversations/direct

Flow:
- Fetches direct conversation between two users.

#### GET /v1/conversations/search

Flow:
- Searches conversations for current user.

#### GET /v1/conversations/type

Flow:
- Lists conversations by type for current user.

#### GET /v1/conversations/invite

Flow:
- Fetches conversation by invite link.

#### POST /v1/conversations/:id/invite

Flow:
- Regenerates invite link (direct repo call).

Outbox/Redis: none (command bus `conversation.generate_invite_link` emits events).

#### POST /v1/conversations/:id/participants

Flow:
- Adds participant (direct repo call).

Outbox/Redis: none (command bus `conversation.add_member` emits events).

#### DELETE /v1/conversations/:id/participants/:user_id

Flow:
- Removes participant (direct repo call).

Outbox/Redis: none.

#### GET /v1/conversations/:id/participants

Flow:
- Lists participants.

#### PUT /v1/conversations/:id/participants/:user_id/role

Flow:
- Updates participant role (direct repo call).

Outbox/Redis: none.

#### POST /v1/conversations/:id/mute

Flow:
- Mutes conversation for user (direct repo call).

Outbox/Redis: none (command bus `conversation.mute` emits events).

#### POST /v1/conversations/:id/unmute

Flow:
- Unmutes conversation for user (direct repo call).

#### POST /v1/conversations/:id/pin

Flow:
- Pins conversation for user (direct repo call).

#### POST /v1/conversations/:id/unpin

Flow:
- Unpins conversation for user (direct repo call).

#### POST /v1/conversations/:id/archive

Flow:
- Archives conversation for user (direct repo call).

#### POST /v1/conversations/:id/unarchive

Flow:
- Unarchives conversation for user (direct repo call).

#### POST /v1/conversations/:id/read-sequence

Flow:
- Updates last read sequence (direct repo call).

#### GET /v1/conversations/:id/sequence

Flow:
- Returns conversation sequence state.

#### POST /v1/conversations/:id/sequence

Flow:
- Increments conversation sequence.

### Users

#### GET /v1/users

Flow:
- Lists users with pagination and search.

#### GET /v1/users/me

Flow:
- Returns current user profile.

#### PUT /v1/users/me

Flow:
- Updates profile.
- Emits `user.updated` outbox event.

#### DELETE /v1/users/me

Flow:
- Deletes user.
- Emits `user.deleted` outbox event.

#### GET /v1/users/me/settings

Flow:
- Returns user settings.

#### PUT /v1/users/me/settings

Flow:
- Updates settings.
- Emits `settings.updated` outbox event.

#### GET /v1/users/me/contacts

Flow:
- Lists contacts.

#### POST /v1/users/me/contacts

Flow:
- Adds contact.
- Emits `contact.added` outbox event.

#### DELETE /v1/users/me/contacts/:id

Flow:
- Removes contact.
- Emits `contact.removed` outbox event.

#### POST /v1/users/me/contacts/:id/block

Flow:
- Blocks contact.
- Emits `contact.blocked` outbox event.

#### POST /v1/users/me/contacts/:id/unblock

Flow:
- Unblocks contact.
- Emits `contact.unblocked` outbox event.

#### GET /v1/users/me/contacts/blocked

Flow:
- Lists blocked contacts.

#### GET /v1/users/me/devices

Flow:
- Lists devices for the user.

#### GET /v1/users/me/devices/:id

Flow:
- Returns a device by ID.

#### DELETE /v1/users/me/devices/:id

Flow:
- Deactivates a device.
- Emits `device.deactivated` outbox event.

#### GET /v1/users/me/push-tokens

Flow:
- Lists push tokens.

#### DELETE /v1/users/me/sessions/:id

Flow:
- Revokes one session.

#### DELETE /v1/users/me/sessions

Flow:
- Revokes all sessions.

### Calls

#### POST /v1/calls

Auth: required (call rate limited).

Flow:
- Creates a call record.
- Emits `call.initiated` outbox event.

Notes:
- Does not set Redis call state; WS `call.initiate` does.

#### GET /v1/calls/:id

Flow:
- Returns call by ID.

#### GET /v1/calls

Flow:
- Lists calls by conversation.

#### GET /v1/calls/user

Flow:
- Lists calls by user.

#### GET /v1/calls/active

Flow:
- Lists active calls by user.

#### GET /v1/calls/missed

Flow:
- Lists missed calls since timestamp.

#### POST /v1/calls/:id/participants

Flow:
- Adds participant.
- Emits `call.participant_added` outbox event.

#### DELETE /v1/calls/:id/participants/:user_id

Flow:
- Removes participant.
- Emits `call.participant_removed` outbox event.

#### GET /v1/calls/:id/participants

Flow:
- Lists call participants.

#### PUT /v1/calls/:id/participants/:user_id/status

Flow:
- Updates participant status (direct repo call).

Outbox/Redis: none.

#### PUT /v1/calls/:id/participants/:user_id/mute

Flow:
- Updates mute state (direct repo call).

Outbox/Redis: none.

#### POST /v1/calls/quality

Flow:
- Records quality metrics.

#### POST /v1/calls/:id/connected

Flow:
- Marks call as connected.
- Emits `call.connected` outbox event.

#### POST /v1/calls/:id/end

Flow:
- Ends the call.
- Emits `call.ended` outbox event.

#### GET /v1/calls/:id/duration

Flow:
- Returns call duration.

#### GET /v1/calls/quality

Flow:
- Lists quality metrics for a call.

#### GET /v1/calls/quality/user

Flow:
- Lists quality metrics for a user in a call.

#### GET /v1/calls/quality/average

Flow:
- Returns average call quality.

#### POST /v1/calls/turn

Flow:
- Creates TURN credentials.

#### GET /v1/calls/turn

Flow:
- Lists active TURN credentials.

#### DELETE /v1/calls/turn/expired

Flow:
- Deletes expired TURN credentials.

#### POST /v1/calls/sfu

Flow:
- Creates an SFU server record.

#### GET /v1/calls/sfu/:id

Flow:
- Returns SFU server by ID.

#### GET /v1/calls/sfu

Flow:
- Lists healthy SFU servers.

#### GET /v1/calls/sfu/least

Flow:
- Returns least loaded SFU server.

#### PUT /v1/calls/sfu/:id/load

Flow:
- Updates SFU server load.

#### PUT /v1/calls/sfu/:id/health

Flow:
- Updates SFU server health.

#### PUT /v1/calls/sfu/:id/heartbeat

Flow:
- Updates SFU server heartbeat.

#### POST /v1/calls/assignments

Flow:
- Assigns a call to an SFU server.

#### GET /v1/calls/assignments

Flow:
- Lists call-server assignments.

#### DELETE /v1/calls/assignments

Flow:
- Removes call-server assignment.

### Uploads

#### POST /v1/uploads

Flow:
- Creates upload session.
- Emits `upload.created` outbox event.

#### GET /v1/uploads/:id

Flow:
- Returns upload session.

#### PUT /v1/uploads/:id

Flow:
- Updates upload session metadata (direct repo call).

Outbox/Redis: none.

#### DELETE /v1/uploads/:id

Flow:
- Deletes upload session.
- Emits `upload.deleted` outbox event.

#### GET /v1/uploads

Flow:
- Lists uploads for a user.

#### GET /v1/uploads/completed

Flow:
- Lists completed uploads for a user.

#### GET /v1/uploads/in-progress

Flow:
- Lists in-progress uploads for a user.

#### POST /v1/uploads/:id/progress

Flow:
- Updates upload progress (direct repo call).

Outbox/Redis: none (command bus `upload.progress` emits events).

#### POST /v1/uploads/:id/complete

Flow:
- Marks upload completed.
- Emits `upload.completed` outbox event.

#### POST /v1/uploads/:id/fail

Flow:
- Marks upload failed.
- Emits `upload.failed` outbox event.

#### GET /v1/uploads/stale

Flow:
- Lists stale uploads.

#### DELETE /v1/uploads/stale

Flow:
- Deletes stale uploads (direct repo call).

Outbox/Redis: none.

### Encryption (E2E keys)

#### POST /v1/encryption/identity

Flow:
- Uploads identity key for a device.
- Emits `identity_key.created` outbox event.
- Response redacts `public_key`.

#### GET /v1/encryption/identity

Flow:
- Returns identity key for user/device.
- Response redacts `public_key`.

#### PUT /v1/encryption/identity/:id/deactivate

Flow:
- Deactivates identity key (direct repo call).

Outbox/Redis: none.

#### DELETE /v1/encryption/identity/:id

Flow:
- Deletes identity key (direct repo call).

#### POST /v1/encryption/signed-prekeys

Flow:
- Uploads signed prekey.
- Emits `signed_prekey.created` outbox event.
- Response redacts `public_key` and `signature`.

#### GET /v1/encryption/signed-prekeys

Flow:
- Returns signed prekey.
- Response redacts `public_key` and `signature`.

#### GET /v1/encryption/signed-prekeys/active

Flow:
- Returns active signed prekey.
- Response redacts `public_key` and `signature`.

#### POST /v1/encryption/signed-prekeys/rotate

Flow:
- Rotates signed prekey (direct repo call).

Outbox/Redis: none.

#### PUT /v1/encryption/signed-prekeys/:id/deactivate

Flow:
- Deactivates signed prekey (direct repo call).

#### POST /v1/encryption/onetime-prekeys

Flow:
- Uploads one-time prekeys.
- Emits `onetime_prekeys.uploaded` outbox event (count).

#### POST /v1/encryption/onetime-prekeys/consume

Flow:
- Consumes one-time prekey for a target device.

Outbox/Redis: none (bundle fetch emits event).

#### GET /v1/encryption/onetime-prekeys/count

Flow:
- Returns available prekey count.

#### GET /v1/encryption/bundles

Flow:
- Validates consumer device ownership.
- Builds key bundle from identity + active signed prekey.
- Consumes a one-time prekey if available.
- Emits `key_bundle.fetched` outbox event.

#### GET /v1/encryption/keys/active

Flow:
- Returns whether device has active keys.

### Broadcasts

#### POST /v1/broadcasts

Flow:
- Creates broadcast list.
- Emits `broadcast.created` outbox event.

#### GET /v1/broadcasts/:id

Flow:
- Returns broadcast list by ID.

#### PUT /v1/broadcasts/:id

Flow:
- Updates broadcast list (direct repo call).

Outbox/Redis: none (command bus `broadcast.update` emits events).

#### DELETE /v1/broadcasts/:id

Flow:
- Deletes broadcast list.
- Emits `broadcast.deleted` outbox event.

#### GET /v1/broadcasts

Flow:
- Lists broadcast lists for owner.

#### GET /v1/broadcasts/search

Flow:
- Searches broadcast lists by owner and query.

#### POST /v1/broadcasts/:id/recipients

Flow:
- Adds recipient.
- Emits `broadcast.recipient_added` outbox event.

#### DELETE /v1/broadcasts/:id/recipients/:user_id

Flow:
- Removes recipient.
- Emits `broadcast.recipient_removed` outbox event.

#### GET /v1/broadcasts/:id/recipients

Flow:
- Lists recipients.

#### GET /v1/broadcasts/:id/recipients/count

Flow:
- Returns recipient count.

#### GET /v1/broadcasts/:id/recipients/:user_id

Flow:
- Returns whether user is a recipient.

#### POST /v1/broadcasts/:id/recipients/bulk

Flow:
- Bulk adds recipients (direct repo call).

Outbox/Redis: none.

#### DELETE /v1/broadcasts/:id/recipients/bulk

Flow:
- Bulk removes recipients (direct repo call).

Outbox/Redis: none.

## Redis data stores

### Presence store

- Keys:
  - `presence:{user_id}` for status payload
  - `presence:online` set of online users
  - `presence:heartbeat:all` for heartbeat timestamps
- Publishes to `channel:presence:{user_id}`.

### Signaling store (WebRTC)

- Keys:
  - `call:state:{call_id}` for call state
  - `call:offers:{user_id}` pending offers
  - `call:candidates:{call_id}:{from}:{to}` ICE candidates
- Publishes signaling events to:
  - `channel:call:{call_id}`
  - `channel:user:{target_user_id}`

### Rate limiting

- Keys:
  - `ratelimit:{user_id}:messages`
  - `ratelimit:{user_id}:calls`
  - `ratelimit:{ip}:auth`
  - `ratelimit:{user_id}:websocket`

## Patterns used

### Observer pattern

- WebSocket Hub is the subject; each Client is an observer.
- Redis bridge feeds events into the hub by channel.

### Command pattern

- Each command implements `CommandType()`, `Validate()`, `IdempotencyKey()`.
- Commands are registered in services and executed by the bus.

### Proxy pattern

- AccessControl implements `commands.Proxy`.
- The bus enforces access before executing handlers.

## Not supported or disabled routes

- Message detail and update endpoints return `NOT_SUPPORTED` for E2E.
- Encryption sessions and key bundle upsert routes are disabled (not registered in routing).
