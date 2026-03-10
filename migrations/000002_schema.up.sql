-- ============================================================
-- USERS & AUTH
-- ============================================================

CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone_number    CITEXT UNIQUE,
    username        CITEXT UNIQUE,
    email           CITEXT UNIQUE,
    password_hash   TEXT NOT NULL,
    display_name    TEXT NOT NULL,
    bio             TEXT,
    avatar_url      TEXT,
    is_online       BOOLEAN DEFAULT FALSE,
    last_seen_at    TIMESTAMP,
    is_active       BOOLEAN DEFAULT TRUE,
    is_verified     BOOLEAN DEFAULT FALSE,
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

-- Devices: required for multi-device E2E message routing.
-- Server knows WHICH devices exist so it can deliver encrypted blobs.
-- NO keys are stored here — keys live only on client devices.
CREATE TABLE IF NOT EXISTS devices (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id       TEXT NOT NULL,          -- client-generated stable identifier
    device_name     TEXT,                   -- "Hasnain's iPhone"
    device_type     TEXT,                   -- ios, android, web, desktop
    is_active       BOOLEAN DEFAULT TRUE,
    registered_at   TIMESTAMP DEFAULT NOW(),
    last_seen_at    TIMESTAMP,
    UNIQUE (user_id, device_id)
);

-- FCM / Push notification tokens
CREATE TABLE IF NOT EXISTS fcm_tokens (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id       UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    platform        TEXT NOT NULL,          -- android, ios, web
    token           TEXT NOT NULL,
    is_active       BOOLEAN DEFAULT TRUE,
    created_at      TIMESTAMP DEFAULT NOW(),
    last_used_at    TIMESTAMP,
    UNIQUE (device_id, token)
);

-- Auth sessions (refresh token rotation)
CREATE TABLE IF NOT EXISTS user_sessions (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id           UUID REFERENCES devices(id) ON DELETE SET NULL,
    refresh_token_hash  TEXT NOT NULL,
    expires_at          TIMESTAMP NOT NULL,
    is_revoked          BOOLEAN DEFAULT FALSE,
    created_at          TIMESTAMP DEFAULT NOW()
);

-- Contacts: user adds another user as a contact
CREATE TABLE IF NOT EXISTS user_contacts (
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contact_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nickname        TEXT,
    is_blocked      BOOLEAN DEFAULT FALSE,
    created_at      TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (user_id, contact_user_id)
);

-- ============================================================
-- CONVERSATIONS & PARTICIPANTS
-- ============================================================

CREATE TABLE IF NOT EXISTS conversations (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type                    conversation_type NOT NULL,
    -- group fields (NULL for DMs)
    subject                 TEXT,
    description             TEXT,
    avatar_url              TEXT,
    invite_link             TEXT,
    invite_link_revoked_at  TIMESTAMP,
    -- DM dedup pair (auto-populated by trigger, always sorted)
    dm_user_id_a            UUID REFERENCES users(id),
    dm_user_id_b            UUID REFERENCES users(id),
    -- disappearing messages
    disappearing_mode       disappearing_mode DEFAULT 'OFF',
    created_by              UUID REFERENCES users(id),
    created_at              TIMESTAMP DEFAULT NOW(),
    updated_at              TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS participants (
    conversation_id     UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role                participant_role DEFAULT 'MEMBER',
    joined_at           TIMESTAMP DEFAULT NOW(),
    added_by            UUID REFERENCES users(id),
    muted_until         TIMESTAMP,
    archived            BOOLEAN DEFAULT FALSE,
    last_read_sequence  BIGINT DEFAULT 0,
    PRIMARY KEY (conversation_id, user_id)
);

-- Monotonic sequence counter per conversation for message ordering
CREATE TABLE IF NOT EXISTS conversation_sequences (
    conversation_id UUID PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    last_sequence   BIGINT DEFAULT 0,
    updated_at      TIMESTAMP DEFAULT NOW()
);

-- ============================================================
-- MESSAGES (E2E encrypted — content is an opaque ciphertext blob)
-- ============================================================

CREATE TABLE IF NOT EXISTS messages (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id     UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_message_id   TEXT,               -- client idempotency key
    seq_id              BIGINT,             -- auto-assigned by trigger
    type                message_type DEFAULT 'TEXT',
    -- The actual message content, E2E encrypted (base64-encoded ciphertext).
    -- Server cannot read this. Only recipients with the session key can decrypt.
    encrypted_content   TEXT,
    -- Metadata the server needs (unencrypted)
    is_forwarded        BOOLEAN DEFAULT FALSE,
    reply_to_msg_id     UUID REFERENCES messages(id),
    poll_id             UUID,               -- FK added after polls table
    mention_count       INTEGER DEFAULT 0,
    -- Timestamps
    created_at          TIMESTAMP DEFAULT NOW(),
    edited_at           TIMESTAMP,
    deleted_at          TIMESTAMP,          -- soft delete
    expires_at          TIMESTAMP,          -- disappearing messages
    UNIQUE (conversation_id, client_message_id)
);

-- ============================================================
-- MESSAGE FEATURES
-- ============================================================

-- Reactions (emoji reactions on messages)
CREATE TABLE IF NOT EXISTS message_reactions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id      UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reaction_code   VARCHAR NOT NULL,       -- emoji unicode or shortcode
    created_at      TIMESTAMP DEFAULT NOW(),
    UNIQUE (message_id, user_id, reaction_code)
);

-- Delivery receipts: sent -> delivered -> read -> played (for audio/video)
CREATE TABLE IF NOT EXISTS message_receipts (
    message_id      UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          delivery_status DEFAULT 'SENT',
    delivered_at    TIMESTAMP,
    read_at         TIMESTAMP,
    played_at       TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id)
);

-- Starred messages (per-user)
CREATE TABLE IF NOT EXISTS starred_messages (
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_id      UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    starred_at      TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (user_id, message_id)
);

-- Pinned messages (per-conversation, any participant can pin)
CREATE TABLE IF NOT EXISTS pinned_messages (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    message_id      UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    pinned_by       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pinned_at       TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (conversation_id, message_id)
);

-- Message edit history (encrypted — each version is a ciphertext blob)
CREATE TABLE IF NOT EXISTS message_edits (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id      UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    encrypted_content TEXT NOT NULL,         -- old content before the edit
    edited_by       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    edited_at       TIMESTAMP DEFAULT NOW(),
    version_number  INT NOT NULL
);

-- Mentions within a message
CREATE TABLE IF NOT EXISTS message_mentions (
    message_id  UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    "offset"    INTEGER NOT NULL,
    length      INTEGER NOT NULL,
    PRIMARY KEY (message_id, user_id, "offset")
);

-- ============================================================
-- ATTACHMENTS (files up to 15 MB: images, video, audio, files)
-- ============================================================

CREATE TABLE IF NOT EXISTS attachments (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    uploader_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    -- URL is encrypted — only the recipient can derive the decryption key
    encrypted_url       TEXT NOT NULL,
    filename            TEXT,
    mime_type           TEXT NOT NULL,
    size_bytes          BIGINT NOT NULL CHECK (size_bytes <= 15728640), -- 15 MB max
    -- View-once (one-time view image/video)
    view_once           BOOLEAN DEFAULT FALSE,
    viewed_at           TIMESTAMP,
    -- Media metadata (unencrypted, optional — for UI layout before download)
    thumbnail_url       TEXT,
    width               INTEGER,
    height              INTEGER,
    duration_seconds    INTEGER,
    created_at          TIMESTAMP DEFAULT NOW()
);

-- Many-to-many: a message can have multiple attachments
CREATE TABLE IF NOT EXISTS message_attachments (
    message_id      UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    attachment_id   UUID NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
    PRIMARY KEY (message_id, attachment_id)
);

-- Chunked upload tracking (for large files up to 15 MB)
CREATE TABLE IF NOT EXISTS upload_sessions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    uploader_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename        TEXT NOT NULL,
    mime_type       TEXT NOT NULL,
    size_bytes      BIGINT NOT NULL CHECK (size_bytes <= 15728640),
    chunk_size      INTEGER NOT NULL,
    uploaded_bytes  BIGINT DEFAULT 0,
    status          upload_status DEFAULT 'IN_PROGRESS',
    object_key      TEXT,
    file_url        TEXT,
    completed_at    TIMESTAMP,
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

-- ============================================================
-- POLLS
-- ============================================================

CREATE TABLE IF NOT EXISTS polls (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id      UUID REFERENCES messages(id) ON DELETE CASCADE,
    question        TEXT NOT NULL,
    allows_multiple BOOLEAN DEFAULT FALSE,
    closes_at       TIMESTAMP,
    created_at      TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS poll_options (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    poll_id         UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    option_text     TEXT NOT NULL,
    position        INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS poll_votes (
    poll_id     UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    option_id   UUID NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voted_at    TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (poll_id, option_id, user_id)
);

-- ============================================================
-- CALLS (Audio & Video — P2P WebRTC)
-- ============================================================

CREATE TABLE IF NOT EXISTS calls (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    initiated_by    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type            call_type NOT NULL,
    is_group_call   BOOLEAN DEFAULT FALSE,
    started_at      TIMESTAMP DEFAULT NOW(),
    connected_at    TIMESTAMP,
    ended_at        TIMESTAMP,
    end_reason      call_end_reason,
    duration_seconds INTEGER,
    created_at      TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS call_participants (
    call_id     UUID NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      participant_call_status DEFAULT 'INVITED',
    joined_at   TIMESTAMP,
    left_at     TIMESTAMP,
    muted_audio BOOLEAN DEFAULT FALSE,
    muted_video BOOLEAN DEFAULT FALSE,
    PRIMARY KEY (call_id, user_id)
);

-- ============================================================
-- COMMAND PATTERN (chat actions: undo message, etc.)
-- ============================================================
-- Design: every mutating chat action is logged as a "command".
-- The command stores what happened (payload) and how to reverse it (undo_payload).
-- Client sends POST /commands/{id}/undo to reverse within a time window.
--
-- Supported command_types:
--   DELETE_MESSAGE   — undo restores the message (clears deleted_at)
--   EDIT_MESSAGE     — undo reverts to previous version
--   PIN_MESSAGE      — undo unpins
--   UNPIN_MESSAGE    — undo re-pins
--   REACT_MESSAGE    — undo removes the reaction
--   CLEAR_CHAT       — undo restores cleared_at to NULL

CREATE TABLE IF NOT EXISTS command_logs (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    command_type        VARCHAR(50) NOT NULL,
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id     UUID REFERENCES conversations(id) ON DELETE CASCADE,
    status              command_status DEFAULT 'PENDING',
    payload             JSONB NOT NULL,     -- the action data
    undo_payload        JSONB,              -- data needed to reverse the action
    error_message       TEXT,
    execution_time_ms   INTEGER,
    created_at          TIMESTAMP DEFAULT NOW(),
    executed_at         TIMESTAMP,
    undone_at           TIMESTAMP
);

-- ============================================================
-- OUTBOX (reliable async event delivery)
-- ============================================================

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

-- ============================================================
-- CONVERSATION HOUSEKEEPING
-- ============================================================

-- Per-user "clear chat" timestamp
CREATE TABLE IF NOT EXISTS conversation_clears (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cleared_at      TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (conversation_id, user_id)
);
