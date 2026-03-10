# API Endpoints

This file defines the backend HTTP API surface that should exist for this project.

It mixes three kinds of information:

- `implemented`: already registered in Gin.
- `repo-backed`: not wired yet, but strongly supported by current SQL schema and repository code.
- `recommended`: needed to expose existing domain features cleanly, even when the exact path is not yet present in the repo.

## Global API conventions

### Base URL

- Public API prefix: `/v1`
- Health utilities stay at root:
  - `/ping`
  - `/health`
  - `/goroutines`

### Response envelope

Successful responses:

```json
{"success": true, "data": {}}
```

Error responses:

```json
{"success": false, "error": "message", "code": "ERROR_CODE"}
```

### Auth rules

- Use `Authorization: Bearer <access_token>` for authenticated endpoints.
- Auth middleware stores these values in request context:
  - `user_id`
  - `session_id`
  - `device_id`
- Access tokens should carry:
  - user id
  - session id
  - optional device id

### Common parameter conventions

- All ids are UUIDs.
- Pagination should use:
  - `page` default `1`
  - `limit` default `20`, max `100`
- Time values should be ISO-8601 UTC strings.
- Nullable fields should be omitted or set to `null`.

### Useful status labels in this file

- `implemented`: route exists now.
- `repo-backed`: route is not registered but the repositories and tables exist.
- `recommended`: route should be added for a complete product surface.
- `documented-only`: route name appears in docs but there is no concrete backend support yet.

---

## 1. Utility Endpoints

### GET `/ping`

- Status: `implemented`
- Auth: no
- Does:
  - returns a simple liveliness response
- Response:

```json
{"success": true, "data": {"message": "pong"}}
```

### GET `/health`

- Status: `implemented`
- Auth: no
- Does:
  - runs database health check
  - returns `UNHEALTHY` when DB is unavailable
- Response on success:

```json
{"success": true, "data": {"status": "healthy"}}
```

### GET `/goroutines`

- Status: `implemented`
- Auth: no
- Does:
  - returns current Go runtime goroutine count

---

## 2. Auth Endpoints

These paths are explicitly documented in `README.md` and partially supported by middleware, config, sessions, devices, and users tables.

### POST `/v1/auth/register`

- Status: `repo-backed`
- Auth: no
- Request body:

```json
{
  "display_name": "Hasnain",
  "email": "hasnain@example.com",
  "username": "hasnain",
  "phone_number": "+923001112233",
  "password": "plain-text-password",
  "device": {
    "device_id": "web-123",
    "device_name": "Chrome on Linux",
    "device_type": "web"
  }
}
```

- Does:
  - validates at least one unique login identifier: email, username, or phone number
  - hashes password
  - creates user row
  - optionally creates device row
  - creates refresh session row
  - returns access token and refresh token
- Writes:
  - `users`
  - `devices`
  - `user_sessions`
- Important constraints:
  - `email`, `username`, and `phone_number` are case-insensitive unique fields via `CITEXT`
  - `(user_id, device_id)` must be unique in `devices`

### POST `/v1/auth/login`

- Status: `repo-backed`
- Auth: no
- Request body:

```json
{
  "identifier": "hasnain@example.com",
  "password": "plain-text-password",
  "device": {
    "device_id": "web-123",
    "device_name": "Chrome on Linux",
    "device_type": "web"
  }
}
```

- Does:
  - resolves identifier against email, username, or phone number
  - verifies password hash
  - upserts or creates device record
  - creates a new refresh session
  - returns access and refresh tokens
- Writes:
  - `devices` or device last seen data
  - `user_sessions`
  - `users.last_seen_at` and online state if desired

### POST `/v1/auth/refresh`

- Status: `repo-backed`
- Auth: no
- Request body:

```json
{
  "refresh_token": "opaque-token"
}
```

- Does:
  - hashes incoming refresh token
  - validates non-revoked, non-expired session
  - rotates refresh token
  - returns new access token and refresh token
- Reads/Writes:
  - `user_sessions`

### POST `/v1/auth/logout`

- Status: `repo-backed`
- Auth: yes
- Request body:

```json
{
  "session_id": "uuid-optional-if-current-session-implicit"
}
```

- Does:
  - revokes current or specified session
- Writes:
  - `user_sessions.is_revoked = true`

### POST `/v1/auth/logout-all`

- Status: `repo-backed`
- Auth: yes
- Does:
  - revokes all sessions for current user
- Writes:
  - all matching `user_sessions`

### GET `/v1/auth/sessions`

- Status: `repo-backed`
- Auth: yes
- Does:
  - returns active and historical sessions for current user
  - should include related device info when available
- Reads:
  - `user_sessions`
  - `devices`

### POST `/v1/auth/password/forgot`

- Status: `documented-only`
- Auth: no
- Current repo support:
  - no reset token table
  - no mail/SMS delivery implementation
- To make this real, add:
  - password reset token storage
  - expiry handling
  - mail or SMS sender

### POST `/v1/auth/password/reset`

- Status: `documented-only`
- Auth: no
- Current repo support:
  - no reset token schema
- Needed request body:

```json
{
  "token": "reset-token",
  "new_password": "new-plain-text-password"
}
```

---

## 3. User Endpoints

`README.md` only documents the `/v1/users` prefix. The exact endpoints below are the cleanest mapping for the existing repositories.

### GET `/v1/users/me`

- Status: `recommended`
- Auth: yes
- Does:
  - returns current user profile
- Reads:
  - `users`

### PUT `/v1/users/me`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "display_name": "Hasnain",
  "bio": "Builder",
  "avatar_url": "https://cdn.example.com/avatar.jpg",
  "email": "hasnain@example.com",
  "username": "hasnain",
  "phone_number": "+923001112233"
}
```

- Does:
  - updates editable profile fields
- Writes:
  - `users`
- Important:
  - unique identifier conflicts must return already exists error

### GET `/v1/users/:id`

- Status: `recommended`
- Auth: yes
- Does:
  - returns another user profile for chat/search/contact flows

### GET `/v1/users/search?q=...&page=1&limit=20`

- Status: `recommended`
- Auth: yes
- Does:
  - searches by display name, username, or email
- Reads:
  - `users`

### GET `/v1/users/contacts`

- Status: `recommended`
- Auth: yes
- Does:
  - returns current user's contact list
- Reads:
  - `user_contacts`

### POST `/v1/users/contacts`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "contact_user_id": "uuid",
  "nickname": "Work"
}
```

- Does:
  - creates a contact relation
- Writes:
  - `user_contacts`

### DELETE `/v1/users/contacts/:contact_user_id`

- Status: `recommended`
- Auth: yes
- Does:
  - removes a contact relation

### POST `/v1/users/contacts/:contact_user_id/block`

- Status: `recommended`
- Auth: yes
- Does:
  - sets `is_blocked = true`
  - creates contact row first if it does not exist

### POST `/v1/users/contacts/:contact_user_id/unblock`

- Status: `recommended`
- Auth: yes
- Does:
  - sets `is_blocked = false`

### GET `/v1/users/contacts/blocked`

- Status: `recommended`
- Auth: yes
- Does:
  - lists blocked contacts

### POST `/v1/users/devices`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "device_id": "ios-abc-123",
  "device_name": "Hasnain's iPhone",
  "device_type": "ios"
}
```

- Does:
  - registers a device for the current user
- Writes:
  - `devices`

### GET `/v1/users/devices`

- Status: `recommended`
- Auth: yes
- Does:
  - returns active and inactive devices for current user

### DELETE `/v1/users/devices/:device_uuid`

- Status: `recommended`
- Auth: yes
- Does:
  - deactivates a device
- Writes:
  - `devices.is_active = false`

### POST `/v1/users/devices/:device_uuid/fcm-tokens`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "platform": "android",
  "token": "push-token"
}
```

- Does:
  - attaches an FCM token to a device
- Writes:
  - `fcm_tokens`

### GET `/v1/users/fcm-tokens`

- Status: `recommended`
- Auth: yes
- Does:
  - lists push tokens for the current user

### DELETE `/v1/users/fcm-tokens/:token_id`

- Status: `recommended`
- Auth: yes
- Does:
  - deactivates an FCM token

---

## 4. Conversation Endpoints

Most of these are explicitly listed in `README.md` and map well to repository methods.

### POST `/v1/conversations`

- Status: `repo-backed`
- Auth: yes
- Request body for group:

```json
{
  "type": "GROUP",
  "subject": "Core Team",
  "description": "Planning room",
  "avatar_url": "https://cdn.example.com/group.png",
  "disappearing_mode": "OFF",
  "participant_ids": ["uuid-1", "uuid-2"]
}
```

- Request body for DM:

```json
{
  "type": "DM",
  "participant_ids": ["other-user-uuid"]
}
```

- Does:
  - creates conversation row
  - creates participant rows
  - for DM, stores normalized pair in `dm_user_id_a` and `dm_user_id_b`
  - should reject DM creation with more than one other user
- Writes:
  - `conversations`
  - `participants`
- Important:
  - DM uniqueness is enforced by partial unique index on normalized pair

### GET `/v1/conversations?page=1&limit=20`

- Status: `repo-backed`
- Auth: yes
- Does:
  - returns conversations for current user
  - sorts by latest message time if present, otherwise conversation creation time
  - includes participants

### GET `/v1/conversations/:id`

- Status: `repo-backed`
- Auth: yes
- Does:
  - returns conversation and participants
  - should require current user to be a participant

### PUT `/v1/conversations/:id`

- Status: `repo-backed`
- Auth: yes
- Request body:

```json
{
  "subject": "New name",
  "description": "New description",
  "avatar_url": "https://cdn.example.com/new.png",
  "disappearing_mode": "24_HOURS"
}
```

- Does:
  - updates mutable conversation metadata

### DELETE `/v1/conversations/:id`

- Status: `repo-backed`
- Auth: yes
- Does:
  - deletes conversation row
  - cascades to related rows via FK rules
- Important:
  - use carefully; many chat products prefer soft-delete or leave semantics instead

### GET `/v1/conversations/direct?user_id=<uuid>`

- Status: `repo-backed`
- Auth: yes
- Does:
  - returns existing DM for current user and target user

### GET `/v1/conversations/search?q=...`

- Status: `repo-backed`
- Auth: yes
- Does:
  - searches current user's conversations by subject
- Limitation:
  - only group subject search makes sense; DMs have no subject by default

### GET `/v1/conversations/type?type=DM|GROUP`

- Status: `repo-backed`
- Auth: yes
- Does:
  - filters current user's conversations by type

### GET `/v1/conversations/invite?link=<invite_link>`

- Status: `repo-backed`
- Auth: yes
- Does:
  - resolves invite link to a conversation
  - only works while `invite_link_revoked_at` is null or in the future

### POST `/v1/conversations/:id/invite`

- Status: `repo-backed`
- Auth: yes
- Does:
  - regenerates invite link
  - clears revocation timestamp
- Response:

```json
{
  "success": true,
  "data": {
    "invite_link": "uuid-string"
  }
}
```

### POST `/v1/conversations/:id/participants`

- Status: `repo-backed`
- Auth: yes
- Request body:

```json
{
  "user_id": "uuid",
  "role": "MEMBER"
}
```

- Does:
  - adds participant
  - should verify role permissions in service layer
- Writes:
  - `participants`

### DELETE `/v1/conversations/:id/participants/:user_id`

- Status: `repo-backed`
- Auth: yes
- Does:
  - removes participant from conversation

### GET `/v1/conversations/:id/participants`

- Status: `repo-backed`
- Auth: yes
- Does:
  - returns participant list with joined user fields

### PUT `/v1/conversations/:id/participants/:user_id/role`

- Status: `repo-backed`
- Auth: yes
- Request body:

```json
{
  "role": "ADMIN"
}
```

- Does:
  - changes participant role

### POST `/v1/conversations/:id/mute`

- Status: `repo-backed`
- Auth: yes
- Request body:

```json
{
  "until": "2026-03-11T15:00:00Z"
}
```

- Does:
  - sets `participants.muted_until`

### POST `/v1/conversations/:id/unmute`

- Status: `repo-backed`
- Auth: yes
- Does:
  - clears `participants.muted_until`

### POST `/v1/conversations/:id/archive`

- Status: `repo-backed`
- Auth: yes
- Does:
  - sets `participants.archived = true`

### POST `/v1/conversations/:id/unarchive`

- Status: `repo-backed`
- Auth: yes
- Does:
  - sets `participants.archived = false`

### POST `/v1/conversations/:id/read-sequence`

- Status: `repo-backed`
- Auth: yes
- Request body:

```json
{
  "seq_id": 144
}
```

- Does:
  - updates `participants.last_read_sequence`
  - should also emit read events through outbox/websocket

### GET `/v1/conversations/:id/sequence`

- Status: `repo-backed`
- Auth: yes
- Does:
  - returns current conversation sequence counter

### POST `/v1/conversations/:id/sequence`

- Status: `repo-backed`
- Auth: yes
- Does:
  - increments sequence manually
- Note:
  - this route is probably only useful internally; message insert trigger already manages sequence assignment

### POST `/v1/conversations/:id/clear`

- Status: `recommended`
- Auth: yes
- Does:
  - sets or updates `conversation_clears.cleared_at` for current user
  - hides older messages client-side without deleting them globally
- Writes:
  - `conversation_clears`

### POST `/v1/conversations/:id/pin`

- Status: `documented-only`
- Reality check:
  - the repo does not support conversation pinning
  - current pinning support is for messages, not conversations
- Recommendation:
  - remove this route from docs unless a `pinned_conversations` table is added

### POST `/v1/conversations/:id/unpin`

- Status: `documented-only`
- Same note as above.

---

## 5. Message Endpoints

### POST `/v1/messages`

- Status: `repo-backed`
- Auth: yes
- Request body:

```json
{
  "conversation_id": "uuid",
  "client_message_id": "client-idempotency-key",
  "type": "TEXT",
  "encrypted_content": "base64-ciphertext",
  "is_forwarded": false,
  "reply_to_msg_id": null,
  "expires_at": null,
  "mentions": [
    {"user_id": "uuid", "offset": 0, "length": 8}
  ],
  "attachments": [
    {"attachment_id": "uuid"}
  ],
  "poll": null
}
```

- Does:
  - creates message row
  - database trigger assigns `seq_id`
  - optionally links attachments
  - optionally creates mentions
  - optionally creates poll and poll options
  - should create outbox event for realtime fan-out
- Writes:
  - `messages`
  - `message_mentions`
  - `message_attachments`
  - `polls`
  - `poll_options`
  - `outbox_events`
- Important constraints:
  - unique per conversation by `(conversation_id, client_message_id)`
  - search is not possible server-side because payload is encrypted

### GET `/v1/messages?conversation_id=<uuid>&before_seq=<n>&limit=50`

- Status: `repo-backed`
- Auth: yes
- Does:
  - returns non-deleted messages for a conversation
  - paginates backward by sequence id
  - sorts descending by `seq_id`

### GET `/v1/messages/:id`

- Status: `repo-backed`
- Auth: yes
- Does:
  - fetches one message by id
- Limitation:
  - README already notes full message detail flows may be limited for E2EE product needs

### PUT `/v1/messages/:id`

- Status: `repo-backed`
- Auth: yes
- Request body:

```json
{
  "encrypted_content": "new-base64-ciphertext",
  "expires_at": null
}
```

- Does:
  - updates message row
  - should create a `message_edits` history row first
  - should mark `edited_at`
  - should log an `EDIT_MESSAGE` command
  - should create outbox event

### DELETE `/v1/messages/:id`

- Status: `repo-backed`
- Auth: yes
- Does:
  - soft deletes message by setting `deleted_at`
  - should log a `DELETE_MESSAGE` command
  - should create outbox event

### DELETE `/v1/messages/:id/hard`

- Status: `repo-backed`
- Auth: yes
- Does:
  - permanently deletes message row
  - cascades dependent rows
- Recommendation:
  - keep admin-only or internal only

### POST `/v1/messages/:id/read`

- Status: `repo-backed`
- Auth: yes
- Does:
  - marks or creates read receipt for current user
  - should also update conversation `last_read_sequence`
  - should emit outbox event
- Writes:
  - `message_receipts`
  - optionally `participants.last_read_sequence`

### POST `/v1/messages/:id/delivered`

- Status: `repo-backed`
- Auth: yes
- Does:
  - marks or creates delivered receipt for current user
  - should emit outbox event

### POST `/v1/messages/:id/played`

- Status: `recommended`
- Auth: yes
- Does:
  - updates played timestamp for audio/video messages
- Writes:
  - `message_receipts.played_at`

---

## 6. Message Feature Endpoints

These are not all listed in `README.md`, but the repository layer clearly supports them.

### POST `/v1/messages/:id/reactions`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "reaction_code": ":thumbsup:"
}
```

- Does:
  - creates reaction row
  - should log `REACT_MESSAGE` command
  - should create outbox event

### DELETE `/v1/messages/:id/reactions/:reaction_code`

- Status: `recommended`
- Auth: yes
- Does:
  - removes current user's reaction

### GET `/v1/messages/:id/reactions`

- Status: `recommended`
- Auth: yes
- Does:
  - returns all reactions for one message

### POST `/v1/messages/:id/star`

- Status: `recommended`
- Auth: yes
- Does:
  - stars message for current user
- Writes:
  - `starred_messages`

### DELETE `/v1/messages/:id/star`

- Status: `recommended`
- Auth: yes
- Does:
  - unstars message for current user

### GET `/v1/messages/starred?page=1&limit=20`

- Status: `recommended`
- Auth: yes
- Does:
  - lists current user's starred messages

### POST `/v1/messages/:id/pin`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "conversation_id": "uuid"
}
```

- Does:
  - pins message in a conversation
  - should log `PIN_MESSAGE` command
  - should create outbox event

### DELETE `/v1/messages/:id/pin?conversation_id=<uuid>`

- Status: `recommended`
- Auth: yes
- Does:
  - unpins message in a conversation
  - should log `UNPIN_MESSAGE` command
  - should create outbox event

### GET `/v1/conversations/:id/pinned-messages`

- Status: `recommended`
- Auth: yes
- Does:
  - lists pinned messages for a conversation

### GET `/v1/messages/:id/edits`

- Status: `recommended`
- Auth: yes
- Does:
  - returns edit history rows

### GET `/v1/messages/:id/receipts`

- Status: `recommended`
- Auth: yes
- Does:
  - returns all receipts for a message

### GET `/v1/users/me/mentions?page=1&limit=20`

- Status: `recommended`
- Auth: yes
- Does:
  - returns messages that mention current user

### GET `/v1/conversations/:id/messages/by-type?type=IMAGE&limit=50`

- Status: `recommended`
- Auth: yes
- Does:
  - returns recent messages of a given message type

### GET `/v1/conversations/:id/messages/range?start_seq=1&end_seq=100`

- Status: `recommended`
- Auth: yes
- Does:
  - returns messages in an inclusive sequence window

### GET `/v1/conversations/:id/messages/unread`

- Status: `recommended`
- Auth: yes
- Does:
  - returns unread messages for current user in a conversation

---

## 7. Attachment and Upload Endpoints

The schema supports both attachment metadata and upload sessions. S3 presign helper also exists.

### POST `/v1/uploads`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "filename": "photo.jpg",
  "mime_type": "image/jpeg",
  "size_bytes": 1048576,
  "chunk_size": 262144
}
```

- Does:
  - validates file size limit (`<= 15 MB`)
  - creates upload session
  - generates object key
  - returns presigned upload URL and required headers
- Writes:
  - `upload_sessions`

### GET `/v1/uploads/:id`

- Status: `repo-backed`
- Auth: yes
- Does:
  - returns upload session by id

### PATCH `/v1/uploads/:id/progress`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "uploaded_bytes": 524288
}
```

- Does:
  - updates progress counter

### POST `/v1/uploads/:id/complete`

- Status: `recommended`
- Auth: yes
- Does:
  - marks upload completed
  - sets uploaded bytes to full size
  - sets completion timestamp

### POST `/v1/uploads/:id/fail`

- Status: `recommended`
- Auth: yes
- Does:
  - marks upload failed

### GET `/v1/uploads`

- Status: `recommended`
- Auth: yes
- Query options:
  - `status=IN_PROGRESS`
  - `status=COMPLETED`
  - `page`
  - `limit`
- Does:
  - returns current user's upload sessions

### POST `/v1/attachments`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "encrypted_url": "encrypted-storage-url",
  "filename": "photo.jpg",
  "mime_type": "image/jpeg",
  "size_bytes": 1048576,
  "view_once": false,
  "thumbnail_url": "https://cdn.example.com/thumb.jpg",
  "width": 1280,
  "height": 720,
  "duration_seconds": null
}
```

- Does:
  - stores attachment metadata after successful upload
- Writes:
  - `attachments`

### GET `/v1/attachments/:id`

- Status: `repo-backed`
- Auth: yes
- Does:
  - returns attachment metadata

### POST `/v1/attachments/:id/viewed`

- Status: `recommended`
- Auth: yes
- Does:
  - marks a view-once attachment as viewed
  - DB trigger then redacts `encrypted_url`

---

## 8. Poll Endpoints

### POST `/v1/polls`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "message_id": "uuid",
  "question": "Where should we meet?",
  "allows_multiple": false,
  "closes_at": null,
  "options": [
    {"option_text": "Office", "position": 1},
    {"option_text": "Cafe", "position": 2}
  ]
}
```

- Does:
  - creates poll and poll options
  - typically should be called as part of `POST /v1/messages` for `type = POLL`

### GET `/v1/polls/:id`

- Status: `repo-backed`
- Auth: yes
- Does:
  - returns poll metadata

### GET `/v1/polls/:id/options`

- Status: `recommended`
- Auth: yes
- Does:
  - lists options ordered by `position`

### POST `/v1/polls/:id/votes`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "option_id": "uuid"
}
```

- Does:
  - creates vote row
  - service layer must enforce single-choice behavior when `allows_multiple = false`

### DELETE `/v1/polls/:id/votes/:option_id`

- Status: `recommended`
- Auth: yes
- Does:
  - removes current user's vote for one option

### GET `/v1/polls/:id/votes`

- Status: `recommended`
- Auth: yes
- Does:
  - returns all votes for the poll

### POST `/v1/polls/:id/close`

- Status: `recommended`
- Auth: yes
- Does:
  - sets `closes_at = now`

---

## 9. Call Endpoints

`README.md` documents only the `/v1/calls` area. The repository can already support a useful initial HTTP surface.

### POST `/v1/calls`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "conversation_id": "uuid",
  "type": "AUDIO",
  "is_group_call": false,
  "participant_ids": ["uuid-1"]
}
```

- Does:
  - creates call row
  - creates `call_participants` rows
  - should emit websocket events for ringing/signaling
- Writes:
  - `calls`
  - `call_participants`
  - `outbox_events`

### GET `/v1/calls/:id`

- Status: `recommended`
- Auth: yes
- Does:
  - returns call detail

### GET `/v1/calls?conversation_id=<uuid>&page=1&limit=20`

- Status: `recommended`
- Auth: yes
- Does:
  - lists calls for a conversation

### GET `/v1/calls/me?page=1&limit=20`

- Status: `recommended`
- Auth: yes
- Does:
  - lists calls where current user is initiator or participant

### GET `/v1/calls/active`

- Status: `recommended`
- Auth: yes
- Does:
  - returns active calls for current user
- Important mismatch to fix first:
  - repository currently uses `JOINED`, but SQL enum uses `CONNECTED`

### GET `/v1/calls/missed?since=<timestamp>`

- Status: `recommended`
- Auth: yes
- Does:
  - returns missed calls since a timestamp

### POST `/v1/calls/:id/connect`

- Status: `recommended`
- Auth: yes
- Does:
  - marks call connected

### POST `/v1/calls/:id/end`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "reason": "COMPLETED"
}
```

- Does:
  - sets `ended_at`
  - stores `end_reason`
  - computes `duration_seconds` if `connected_at` is present

### GET `/v1/calls/:id/duration`

- Status: `recommended`
- Auth: yes
- Does:
  - returns call duration

### GET `/v1/calls/:id/participants`

- Status: `recommended`
- Auth: yes
- Does:
  - returns call participants

### POST `/v1/calls/:id/participants`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "user_id": "uuid",
  "status": "INVITED"
}
```

- Does:
  - adds participant to the call

### DELETE `/v1/calls/:id/participants/:user_id`

- Status: `recommended`
- Auth: yes
- Does:
  - marks participant as left

### PATCH `/v1/calls/:id/participants/:user_id/status`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "status": "CONNECTED"
}
```

- Does:
  - updates participant state
- Required fix:
  - align repo constants to SQL enum before shipping

### PATCH `/v1/calls/:id/participants/:user_id/mute`

- Status: `recommended`
- Auth: yes
- Request body:

```json
{
  "audio_muted": true,
  "video_muted": false
}
```

- Does:
  - updates mute flags

### GET `/v1/calls/:id/participants/active-count`

- Status: `recommended`
- Auth: yes
- Does:
  - returns active participant count
- Required fix:
  - count should use `CONNECTED`, not `JOINED`

---

## 10. Encryption Endpoints

`README.md` claims this area, but the current migrations do not create the required tables. These endpoints are still useful as the intended contract.

### POST `/v1/encryption/identity-keys`

- Status: `documented-only`
- Auth: yes
- Request body should include:
  - public identity key
  - device id

### POST `/v1/encryption/signed-prekeys`

- Status: `documented-only`
- Auth: yes

### POST `/v1/encryption/one-time-prekeys`

- Status: `documented-only`
- Auth: yes

### GET `/v1/encryption/bundle/:user_id?device_id=<device-id>`

- Status: `documented-only`
- Auth: yes
- Does:
  - returns identity key, signed prekey, and one available one-time prekey

### Reality check

- Before these can exist, add schema and repositories for:
  - `identity_keys`
  - `signed_prekeys`
  - `onetime_prekeys`
  - session material if needed

---

## 11. Broadcast Endpoints

`README.md` claims `/v1/broadcasts`, but the schema does not create broadcast tables.

### Recommended contract

### POST `/v1/broadcasts`

- Status: `documented-only`
- Request body:

```json
{
  "name": "Announcements",
  "recipient_ids": ["uuid-1", "uuid-2"]
}
```

### GET `/v1/broadcasts`

- Status: `documented-only`

### GET `/v1/broadcasts/:id`

- Status: `documented-only`

### PUT `/v1/broadcasts/:id`

- Status: `documented-only`

### DELETE `/v1/broadcasts/:id`

- Status: `documented-only`

### POST `/v1/broadcasts/:id/recipients`

- Status: `documented-only`

### DELETE `/v1/broadcasts/:id/recipients/:user_id`

- Status: `documented-only`

### Required missing schema

- `broadcast_lists`
- `broadcast_recipients`

---

## 12. Command Endpoints

The command system deserves its own file, but these are the HTTP routes the API should expose.

### GET `/v1/commands/:id`

- Status: `recommended`
- Auth: yes
- Does:
  - returns command log detail

### GET `/v1/commands?limit=50`

- Status: `recommended`
- Auth: yes
- Does:
  - returns current user's recent command logs

### POST `/v1/commands/:id/undo`

- Status: `repo-backed`
- Auth: yes
- Does:
  - validates ownership
  - validates command can still be undone
  - reverses command side effects
  - marks command as `UNDONE`
  - creates outbox events for client fan-out
- Important:
  - current repo only exposes `CanUndo`; actual undo execution is not implemented yet

---

## 13. Rate limiting recommendations

Middleware already implies these groups:

- Auth routes:
  - `/v1/auth/login`
  - `/v1/auth/register`
  - `/v1/auth/refresh`
  - `/v1/auth/password/forgot`
  - `/v1/auth/password/reset`
- Message routes:
  - all write-heavy message endpoints should use user-based limiter
- Call routes:
  - call creation and signaling endpoints should use user-based limiter

Recommended response headers on limited routes:

- `X-RateLimit-Limit`
- `X-RateLimit-Remaining`
- `X-RateLimit-Reset`

---

## 14. Key implementation notes to keep in mind

- Message `seq_id` is assigned by a DB trigger, not by the application.
- DM conversations are normalized and deduplicated at DB level.
- Search over encrypted content is intentionally unsupported.
- View-once attachments lose their `encrypted_url` after view mark is written.
- Poll single-choice rules are not DB-enforced and must live in service logic.
- Upload and attachment size ceiling is `15 MB`.
- Command status enum mismatch must be fixed before command endpoints ship.
- Call participant status mismatch must be fixed before call endpoints ship.
