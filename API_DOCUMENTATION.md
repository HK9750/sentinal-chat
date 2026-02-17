# Sentinal Chat API Documentation

This document provides the request and response structures for all API endpoints in the Sentinal Chat backend.

## Base URL
```
http://localhost:<port>/v1
```

## Standard Response Format
All API responses follow this structure:
```json
{
  "success": true,
  "data": { ... },
  "error": "error message if any",
  "code": "ERROR_CODE if any"
}
```

---

## Authentication Endpoints (`/auth`)

### POST /auth/register
Register a new user.

**Request:**
```json
{
  "email": "string (required)",
  "username": "string (required)",
  "phone_number": "string (optional)",
  "password": "string (required)",
  "display_name": "string (optional)",
  "device_id": "string (required)",
  "device_name": "string (optional)",
  "device_type": "string (optional)"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "access_token": "string",
    "refresh_token": "string",
    "expires_in": 86400,
    "session_id": "string",
    "user": {
      "id": "string",
      "display_name": "string",
      "username": "string",
      "email": "string",
      "phone_number": "string"
    }
  }
}
```

### POST /auth/login
Authenticate a user.

**Request:**
```json
{
  "identity": "string (required) - email, username, or phone",
  "password": "string (required)",
  "device_id": "string (required)",
  "device_name": "string (optional)",
  "device_type": "string (optional)"
}
```

**Response:** Same as `/auth/register`

### POST /auth/refresh
Refresh access token.

**Request:**
```json
{
  "session_id": "string (required)",
  "refresh_token": "string (required)"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "access_token": "string",
    "refresh_token": "string",
    "expires_at": "ISO8601 string"
  }
}
```

### POST /auth/logout
Logout user (requires authentication).

**Request:**
```json
{
  "session_id": "string (required)"
}
```

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /auth/logout-all
Logout from all sessions (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### GET /auth/sessions
List all active user sessions (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "sessions": [
      {
        "id": "string",
        "device_id": "string",
        "device_name": "string",
        "device_type": "string",
        "ip_address": "string",
        "last_active": "ISO8601 string",
        "created_at": "ISO8601 string",
        "is_current": true
      }
    ]
  }
}
```

### POST /auth/password/forgot
Request password reset.

**Request:**
```json
{
  "identity": "string (required) - email or username"
}
```

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /auth/password/reset
Reset password.

**Request:**
```json
{
  "identity": "string (required)",
  "new_password": "string (required)"
}
```

**Response:**
```json
{
  "success": true,
  "data": null
}
```

---

## User Endpoints (`/users`)

### GET /users
List users (requires authentication).

**Query Parameters:**
- `page` (int, optional)
- `limit` (int, optional)
- `search` (string, optional)

**Response:**
```json
{
  "success": true,
  "data": {
    "users": [
      {
        "id": "string",
        "email": "string",
        "username": "string",
        "display_name": "string",
        "avatar_url": "string",
        "created_at": "ISO8601 string"
      }
    ],
    "total": 100
  }
}
```

### GET /users/me
Get current user's profile (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "string",
    "email": "string",
    "username": "string",
    "display_name": "string",
    "avatar_url": "string",
    "created_at": "ISO8601 string"
  }
}
```

### PUT /users/me
Update current user's profile (requires authentication).

**Request:**
```json
{
  "display_name": "string (optional)",
  "avatar_url": "string (optional)",
  "bio": "string (optional)"
}
```

**Response:** Same as GET /users/me

### DELETE /users/me
Delete current user's profile (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### GET /users/me/settings
Get user settings (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "string",
    "privacy_last_seen": "string",
    "privacy_profile_photo": "string",
    "privacy_about": "string",
    "privacy_groups": "string",
    "read_receipts": true,
    "notifications_enabled": true,
    "notification_sound": "string",
    "notification_vibrate": true,
    "theme": "string",
    "language": "string",
    "enter_to_send": true,
    "media_auto_download_wifi": true,
    "media_auto_download_mobile": true,
    "updated_at": "ISO8601 string"
  }
}
```

### PUT /users/me/settings
Update user settings (requires authentication).

**Request:**
```json
{
  "notifications_enabled": true,
  "theme": "string",
  "language": "string"
}
```

**Response:** Same as GET /users/me/settings

### GET /users/me/contacts
List user contacts (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "contacts": [
      {
        "user_id": "string",
        "contact_user_id": "string",
        "nickname": "string",
        "is_blocked": false,
        "created_at": "ISO8601 string"
      }
    ]
  }
}
```

### POST /users/me/contacts
Add a contact (requires authentication).

**Request:**
```json
{
  "contact_user_id": "string (required)"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "success": true
  }
}
```

### DELETE /users/me/contacts/:id
Remove a contact (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /users/me/contacts/:id/block
Block a contact (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /users/me/contacts/:id/unblock
Unblock a contact (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### GET /users/me/contacts/blocked
List blocked contacts (requires authentication).

**Response:** Same as GET /users/me/contacts

### GET /users/me/devices
List user devices (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "devices": [
      {
        "id": "string",
        "device_name": "string",
        "device_type": "string",
        "last_active": "ISO8601 string",
        "is_active": true
      }
    ]
  }
}
```

### GET /users/me/devices/:id
Get device details (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "string",
    "device_name": "string",
    "device_type": "string",
    "last_active": "ISO8601 string",
    "is_active": true
  }
}
```

### DELETE /users/me/devices/:id
Deactivate a device (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### GET /users/me/push-tokens
List push tokens (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "tokens": [
      {
        "id": "string",
        "token": "string",
        "platform": "string",
        "created_at": "ISO8601 string"
      }
    ]
  }
}
```

### DELETE /users/me/sessions/:id
Revoke a session (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### DELETE /users/me/sessions
Revoke all sessions (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

---

## Conversation Endpoints (`/conversations`)

### POST /conversations
Create a new conversation (requires authentication).

**Request:**
```json
{
  "type": "string (required) - 'DM' or 'GROUP'",
  "subject": "string (optional)",
  "description": "string (optional)",
  "participants": ["string array of user IDs (required)"]
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "string",
    "type": "string",
    "subject": "string",
    "description": "string",
    "creator_id": "string",
    "invite_link": "string",
    "participant_count": 2,
    "last_message_at": "ISO8601 string",
    "created_at": "ISO8601 string"
  }
}
```

### GET /conversations
List user conversations (requires authentication).

**Query Parameters:**
- `page` (int, optional)
- `limit` (int, optional)

**Response:**
```json
{
  "success": true,
  "data": {
    "conversations": [
      {
        "id": "string",
        "type": "string",
        "subject": "string",
        "description": "string",
        "avatar_url": "string",
        "creator_id": "string",
        "invite_link": "string",
        "participant_count": 2,
        "last_message_at": "ISO8601 string",
        "created_at": "ISO8601 string"
      }
    ],
    "total": 50
  }
}
```

### GET /conversations/:id
Get conversation by ID (requires authentication).

**Response:** Same as GET /conversations

### PUT /conversations/:id
Update conversation (requires authentication).

**Request:**
```json
{
  "subject": "string (optional)",
  "description": "string (optional)",
  "avatar_url": "string (optional)"
}
```

**Response:** Same as GET /conversations

### DELETE /conversations/:id
Delete conversation (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### GET /conversations/direct
Get direct conversation between two users (requires authentication).

**Query Parameters:**
- `user_id_1` (string, required)
- `user_id_2` (string, required)

**Response:** Same as GET /conversations

### GET /conversations/search
Search conversations (requires authentication).

**Query Parameters:**
- `query` (string, required)

**Response:** Same as GET /conversations (without total)

### GET /conversations/type
Get conversations by type (requires authentication).

**Query Parameters:**
- `type` (string, required) - "DM" or "GROUP"

**Response:** Same as GET /conversations

### GET /conversations/invite
Get conversation by invite link (requires authentication).

**Query Parameters:**
- `link` (string, required)

**Response:** Same as GET /conversations

### POST /conversations/:id/invite
Regenerate invite link (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "invite_link": "string"
  }
}
```

### POST /conversations/:id/participants
Add participant to conversation (requires authentication).

**Request:**
```json
{
  "user_id": "string (required)",
  "role": "string (optional) - 'member', 'admin', 'owner'"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "string",
    "username": "string",
    "role": "string",
    "joined_at": "ISO8601 string"
  }
}
```

### DELETE /conversations/:id/participants/:user_id
Remove participant from conversation (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### GET /conversations/:id/participants
List conversation participants (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "participants": [
      {
        "user_id": "string",
        "username": "string",
        "role": "string",
        "joined_at": "ISO8601 string"
      }
    ]
  }
}
```

### PUT /conversations/:id/participants/:user_id/role
Update participant role (requires authentication).

**Request:**
```json
{
  "role": "string (required)"
}
```

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /conversations/:id/mute
Mute conversation (requires authentication).

**Request:**
```json
{
  "until": "string (required) - RFC3339 format"
}
```

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /conversations/:id/unmute
Unmute conversation (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /conversations/:id/pin
Pin conversation (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /conversations/:id/unpin
Unpin conversation (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /conversations/:id/archive
Archive conversation (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /conversations/:id/unarchive
Unarchive conversation (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /conversations/:id/read-sequence
Update last read sequence (requires authentication).

**Request:**
```json
{
  "seq_id": 123
}
```

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### GET /conversations/:id/sequence
Get conversation sequence (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "conversation_id": "string",
    "last_sequence": 100,
    "updated_at": "ISO8601 string"
  }
}
```

### POST /conversations/:id/sequence
Increment conversation sequence (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "sequence": 101
  }
}
```

---

## Message Endpoints (`/messages`)

### POST /messages
Send a message (requires authentication).

**Request:**
```json
{
  "conversation_id": "string (required)",
  "ciphertexts": [
    {
      "recipient_device_id": "string (required)",
      "ciphertext": "string (required) - base64 encoded",
      "header": {}
    }
  ],
  "message_type": "string (optional)",
  "client_message_id": "string (optional)",
  "idempotency_key": "string (optional)"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "string",
    "conversation_id": "string",
    "sender_id": "string",
    "client_message_id": "string",
    "sequence_number": 1,
    "created_at": "ISO8601 string"
  }
}
```

### GET /messages
List messages (requires authentication).

**Query Parameters:**
- `conversation_id` (string, required)
- `before_seq` (int64, optional)
- `limit` (int, optional)

**Response:**
```json
{
  "success": true,
  "data": {
    "messages": [
      {
        "id": "string",
        "conversation_id": "string",
        "sender_id": "string",
        "client_message_id": "string",
        "sequence_number": 1,
        "is_deleted": false,
        "is_edited": false,
        "ciphertext": "string (base64)",
        "header": "string",
        "recipient_device_id": "string",
        "created_at": "ISO8601 string",
        "updated_at": "ISO8601 string"
      }
    ]
  }
}
```

### GET /messages/:id
Get message by ID (not implemented for E2E).

### PUT /messages/:id
Update message (not implemented for E2E).

### DELETE /messages/:id
Soft delete a message (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### DELETE /messages/:id/hard
Hard delete a message (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /messages/:id/read
Mark message as read (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### POST /messages/:id/delivered
Mark message as delivered (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

---

## Call Endpoints (`/calls`)

### POST /calls
Create a call (requires authentication).

**Request:**
```json
{
  "conversation_id": "string (required)",
  "type": "string (required) - 'AUDIO' or 'VIDEO'",
  "initiator_id": "string (required)"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "string",
    "conversation_id": "string",
    "type": "string",
    "status": "RINGING",
    "initiator_id": "string",
    "created_at": "ISO8601 string"
  }
}
```

### GET /calls/:id
Get call by ID (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "string",
    "conversation_id": "string",
    "type": "string",
    "status": "string",
    "initiator_id": "string",
    "started_at": "ISO8601 string",
    "ended_at": "ISO8601 string",
    "duration": 60
  }
}
```

### GET /calls
List calls by conversation (requires authentication).

**Query Parameters:**
- `conversation_id` (string, required)
- `page` (int, optional)
- `limit` (int, optional)

**Response:**
```json
{
  "success": true,
  "data": {
    "calls": [...],
    "total": 10
  }
}
```

### GET /calls/user
List calls by user (requires authentication).

**Query Parameters:**
- `user_id` (string, required)
- `page` (int, optional)
- `limit` (int, optional)

### GET /calls/active
List active calls for user (requires authentication).

**Query Parameters:**
- `user_id` (string, required)

### GET /calls/missed
List missed calls (requires authentication).

**Query Parameters:**
- `user_id` (string, required)
- `since` (string, optional - RFC3339)

### POST /calls/:id/participants
Add participant to call (requires authentication).

**Request:**
```json
{
  "user_id": "string (required)"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "string",
    "status": "INVITED",
    "audio_muted": false,
    "video_muted": false,
    "joined_at": "ISO8601 string"
  }
}
```

### DELETE /calls/:id/participants/:user_id
Remove participant from call (requires authentication).

### GET /calls/:id/participants
List call participants (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "participants": [
      {
        "user_id": "string",
        "status": "string",
        "audio_muted": false,
        "video_muted": false,
        "joined_at": "ISO8601 string",
        "left_at": "ISO8601 string"
      }
    ]
  }
}
```

### PUT /calls/:id/participants/:user_id/status
Update participant status (requires authentication).

**Request:**
```json
{
  "status": "string (required) - 'INVITED', 'JOINED', 'LEFT'"
}
```

### PUT /calls/:id/participants/:user_id/mute
Update participant mute status (requires authentication).

**Request:**
```json
{
  "audio_muted": true,
  "video_muted": true
}
```

### POST /calls/quality
Record call quality metric (requires authentication).

**Request:**
```json
{
  "call_id": "string (required)",
  "user_id": "string (required)",
  "timestamp": "string (optional)",
  "packets_sent": 1000,
  "packets_received": 1000,
  "packet_loss": 0.1,
  "packets_lost": 1,
  "jitter": 0.5,
  "latency": 50,
  "bitrate": 1000000,
  "frame_rate": 30,
  "resolution": "1920x1080",
  "audio_level": 0.5,
  "connection_type": "string",
  "ice_candidate_type": "string"
}
```

### POST /calls/:id/connected
Mark call as connected (requires authentication).

### POST /calls/:id/end
End a call (requires authentication).

**Request:**
```json
{
  "reason": "string (optional) - 'COMPLETED', 'MISSED', 'DECLINED', 'FAILED', 'TIMEOUT', 'NETWORK_ERROR'"
}
```

### GET /calls/:id/duration
Get call duration (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "duration": 60
  }
}
```

### GET /calls/quality
Get call quality metrics (requires authentication).

**Query Parameters:**
- `call_id` (string, required)

### GET /calls/quality/user
Get user's call quality metrics (requires authentication).

**Query Parameters:**
- `call_id` (string, required)
- `user_id` (string, required)

### GET /calls/quality/average
Get average call quality (requires authentication).

**Query Parameters:**
- `call_id` (string, required)

---

## Upload Endpoints (`/uploads`)

### POST /uploads
Create an upload session (requires authentication).

**Request:**
```json
{
  "file_name": "string (required)",
  "file_size": 1024000,
  "content_type": "string (required)",
  "uploader_id": "string (required)"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "string",
    "file_name": "string",
    "file_size": 1024000,
    "content_type": "string",
    "uploader_id": "string",
    "status": "IN_PROGRESS",
    "upload_url": "string",
    "created_at": "ISO8601 string"
  }
}
```

### GET /uploads/:id
Get upload session (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "string",
    "file_name": "string",
    "file_size": 1024000,
    "content_type": "string",
    "uploader_id": "string",
    "status": "string",
    "uploaded_bytes": 512000,
    "file_url": "string",
    "created_at": "ISO8601 string",
    "completed_at": "ISO8601 string"
  }
}
```

### PUT /uploads/:id
Update upload session (requires authentication).

**Request:**
```json
{
  "file_name": "string (optional)",
  "content_type": "string (optional)"
}
```

### DELETE /uploads/:id
Delete upload session (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": null
}
```

### GET /uploads
List user uploads (requires authentication).

**Query Parameters:**
- `uploader_id` (string, required)

### GET /uploads/completed
List completed uploads (requires authentication).

**Query Parameters:**
- `uploader_id` (string, required)
- `page` (int, optional)
- `limit` (int, optional)

### GET /uploads/in-progress
List in-progress uploads (requires authentication).

**Query Parameters:**
- `uploader_id` (string, required)

### POST /uploads/:id/progress
Update upload progress (requires authentication).

**Request:**
```json
{
  "uploaded_bytes": 512000
}
```

### POST /uploads/:id/complete
Mark upload as completed (requires authentication).

### POST /uploads/:id/fail
Mark upload as failed (requires authentication).

### GET /uploads/stale
List stale uploads (requires authentication).

**Query Parameters:**
- `older_than_sec` (int, required)

### DELETE /uploads/stale
Delete stale uploads (requires authentication).

**Query Parameters:**
- `older_than_sec` (int, required)

---

## Encryption Endpoints (`/encryption`)

### POST /encryption/identity
Upload identity key (requires authentication).

**Request:**
```json
{
  "user_id": "string (required)",
  "device_id": "string (required)",
  "public_key": "string (required) - base64 encoded"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "string",
    "user_id": "string",
    "device_id": "string",
    "public_key": "",
    "is_active": true,
    "created_at": "ISO8601 string"
  }
}
```

### GET /encryption/identity
Get identity key (requires authentication).

**Query Parameters:**
- `user_id` (string, required)
- `device_id` (string, required)

### PUT /encryption/identity/:id/deactivate
Deactivate identity key (requires authentication).

### DELETE /encryption/identity/:id
Delete identity key (requires authentication).

### POST /encryption/signed-prekeys
Upload signed prekey (requires authentication).

**Request:**
```json
{
  "user_id": "string (required)",
  "device_id": "string (required)",
  "key_id": 1,
  "public_key": "string (required) - base64 encoded",
  "signature": "string (required) - base64 encoded"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "string",
    "user_id": "string",
    "device_id": "string",
    "key_id": 1,
    "public_key": "",
    "signature": "",
    "created_at": "ISO8601 string",
    "is_active": true
  }
}
```

### GET /encryption/signed-prekeys
Get signed prekey (requires authentication).

**Query Parameters:**
- `user_id` (string, required)
- `device_id` (string, required)
- `key_id` (int, optional)

### GET /encryption/signed-prekeys/active
Get active signed prekey (requires authentication).

**Query Parameters:**
- `user_id` (string, required)
- `device_id` (string, required)

### POST /encryption/signed-prekeys/rotate
Rotate signed prekey (requires authentication).

**Request:**
```json
{
  "user_id": "string (required)",
  "device_id": "string (required)",
  "key": {
    "key_id": 2,
    "public_key": "string (required) - base64",
    "signature": "string (required) - base64"
  }
}
```

### PUT /encryption/signed-prekeys/:id/deactivate
Deactivate signed prekey (requires authentication).

### POST /encryption/onetime-prekeys
Upload one-time prekeys (requires authentication).

**Request:**
```json
{
  "keys": [
    {
      "user_id": "string (required)",
      "device_id": "string (required)",
      "key_id": 1,
      "public_key": "string (required) - base64"
    }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "uploaded": 10
  }
}
```

### POST /encryption/onetime-prekeys/consume
Consume one-time prekey (requires authentication).

**Query Parameters:**
- `user_id` (string, required)
- `device_id` (string, required)
- `consumed_by` (string, required)
- `consumed_device_id` (string, required)

### GET /encryption/onetime-prekeys/count
Get prekey count (requires authentication).

**Query Parameters:**
- `user_id` (string, required)
- `device_id` (string, required)

**Response:**
```json
{
  "success": true,
  "data": {
    "count": 100
  }
}
```

### GET /encryption/bundles
Get key bundle (requires authentication).

**Query Parameters:**
- `user_id` (string, required)
- `device_id` (string, required)
- `consumer_device_id` (string, required)

**Response:**
```json
{
  "success": true,
  "data": {
    "identity_key": {
      "id": "string",
      "user_id": "string",
      "device_id": "string",
      "public_key": "base64 string",
      "is_active": true,
      "created_at": "ISO8601 string"
    },
    "signed_pre_key": {
      "id": "string",
      "user_id": "string",
      "device_id": "string",
      "key_id": 1,
      "public_key": "base64 string",
      "signature": "base64 string",
      "created_at": "ISO8601 string",
      "is_active": true
    },
    "one_time_pre_key": {
      "id": "string",
      "user_id": "string",
      "device_id": "string",
      "key_id": 1,
      "public_key": "base64 string",
      "uploaded_at": "ISO8601 string"
    }
  }
}
```

### GET /encryption/keys/active
Check if user has active keys (requires authentication).

**Query Parameters:**
- `user_id` (string, required)
- `device_id` (string, required)

**Response:**
```json
{
  "success": true,
  "data": {
    "has_active_keys": true
  }
}
```

---

## Broadcast Endpoints (`/broadcasts`)

### POST /broadcasts
Create a broadcast list (requires authentication).

**Request:**
```json
{
  "name": "string (required)",
  "description": "string (optional)",
  "recipients": ["string array of user IDs (optional)"]
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "string",
    "name": "string",
    "description": "string",
    "owner_id": "string",
    "recipient_count": 5,
    "created_at": "ISO8601 string"
  }
}
```

### GET /broadcasts/:id
Get broadcast by ID (requires authentication).

### PUT /broadcasts/:id
Update broadcast (requires authentication).

**Request:**
```json
{
  "name": "string (optional)",
  "description": "string (optional)"
}
```

### DELETE /broadcasts/:id
Delete broadcast (requires authentication).

### GET /broadcasts
List user's broadcast lists (requires authentication).

**Query Parameters:**
- `owner_id` (string, required)

**Response:**
```json
{
  "success": true,
  "data": {
    "broadcasts": [
      {
        "id": "string",
        "name": "string",
        "description": "string",
        "owner_id": "string",
        "recipient_count": 5,
        "created_at": "ISO8601 string"
      }
    ]
  }
}
```

### GET /broadcasts/search
Search broadcast lists (requires authentication).

**Query Parameters:**
- `owner_id` (string, required)
- `query` (string, required)

### POST /broadcasts/:id/recipients
Add recipient to broadcast (requires authentication).

**Request:**
```json
{
  "user_id": "string (required)"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "string",
    "added_at": "ISO8601 string",
    "username": "string",
    "avatar_url": "string"
  }
}
```

### DELETE /broadcasts/:id/recipients/:user_id
Remove recipient from broadcast (requires authentication).

### GET /broadcasts/:id/recipients
List broadcast recipients (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "recipients": [
      {
        "user_id": "string",
        "added_at": "ISO8601 string",
        "username": "string",
        "avatar_url": "string"
      }
    ]
  }
}
```

### GET /broadcasts/:id/recipients/count
Get recipient count (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "count": 10
  }
}
```

### GET /broadcasts/:id/recipients/:user_id
Check if user is recipient (requires authentication).

**Response:**
```json
{
  "success": true,
  "data": {
    "is_recipient": true
  }
}
```

### POST /broadcasts/:id/recipients/bulk
Bulk add recipients (requires authentication).

**Request:**
```json
{
  "user_ids": ["string array of user IDs"]
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "count": 10
  }
}
```

### DELETE /broadcasts/:id/recipients/bulk
Bulk remove recipients (requires authentication).

---

## WebSocket Endpoint

### GET /v1/ws
WebSocket endpoint for real-time communication.

---

## Health & Status Endpoints

### GET /ping
Health check endpoint.

**Response:**
```json
{
  "success": true,
  "data": {
    "message": "pong"
  }
}
```

### GET /health
Database health check.

**Response (Healthy):**
```json
{
  "success": true,
  "data": {
    "status": "healthy"
  }
}
```

**Response (Unhealthy):**
```json
{
  "success": false,
  "error": "error message",
  "code": "UNHEALTHY"
}
```

### GET /goroutines
Get current number of goroutines.

**Response:**
```json
{
  "goroutines": 10
}
```

---
