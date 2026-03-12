-- ============================================================
-- USERS
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_users_phone
    ON users (phone_number) WHERE phone_number IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_username
    ON users (username) WHERE username IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_email
    ON users (email) WHERE email IS NOT NULL;

-- ============================================================
-- SESSIONS
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_sessions_user
    ON user_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_active
    ON user_sessions (expires_at) WHERE is_revoked = FALSE;

-- ============================================================
-- DEVICES & FCM
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_devices_user
    ON devices (user_id);
CREATE INDEX IF NOT EXISTS idx_fcm_tokens_user
    ON fcm_tokens (user_id);

-- ============================================================
-- CONTACTS
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_contacts_contact
    ON user_contacts (contact_user_id);

-- ============================================================
-- CONVERSATIONS
-- ============================================================
CREATE UNIQUE INDEX IF NOT EXISTS idx_conversations_dm_unique_pair
    ON conversations (dm_user_id_a, dm_user_id_b)
    WHERE type = 'DM';

-- ============================================================
-- PARTICIPANTS
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_participants_user
    ON participants (user_id);
CREATE INDEX IF NOT EXISTS idx_participants_conv
    ON participants (conversation_id);

-- ============================================================
-- MESSAGES
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_messages_conv_seq
    ON messages (conversation_id, seq_id DESC);
CREATE INDEX IF NOT EXISTS idx_messages_sender
    ON messages (sender_id);
CREATE INDEX IF NOT EXISTS idx_messages_expires
    ON messages (expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_deleted
    ON messages (deleted_at) WHERE deleted_at IS NOT NULL;

-- ============================================================
-- RECEIPTS & REACTIONS
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_receipts_message
    ON message_receipts (message_id);
CREATE INDEX IF NOT EXISTS idx_receipts_user_status
    ON message_receipts (user_id, status);
CREATE INDEX IF NOT EXISTS idx_reactions_message
    ON message_reactions (message_id);

-- ============================================================
-- STARRED & PINNED
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_starred_user
    ON starred_messages (user_id);
CREATE INDEX IF NOT EXISTS idx_pinned_conv
    ON pinned_messages (conversation_id);

-- ============================================================
-- ATTACHMENTS
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_attachments_uploader
    ON attachments (uploader_id);

-- ============================================================
-- POLLS
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_poll_options_poll
    ON poll_options (poll_id);
CREATE INDEX IF NOT EXISTS idx_poll_votes_user
    ON poll_votes (user_id);

-- ============================================================
-- CALLS
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_calls_conv
    ON calls (conversation_id);
CREATE INDEX IF NOT EXISTS idx_call_participants_user
    ON call_participants (user_id);

-- ============================================================
-- COMMAND LOGS
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_command_logs_user_created
    ON command_logs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_command_logs_conv
    ON command_logs (conversation_id) WHERE conversation_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_command_logs_status
    ON command_logs (status) WHERE status = 'PENDING';

-- ============================================================
-- MESSAGE EDITS
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_message_edits_message
    ON message_edits (message_id, version_number DESC);

-- ============================================================
-- OUTBOX
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_outbox_status
    ON outbox_events (status);
CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON outbox_events (status, retry_count) WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_outbox_aggregate
    ON outbox_events (aggregate_type, aggregate_id);

-- ============================================================
-- CONVERSATION CLEARS
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_conversation_clears_user
    ON conversation_clears (user_id);
