-- Users
CREATE INDEX IF NOT EXISTS idx_users_phone ON users (phone_number) WHERE phone_number IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_username ON users (username) WHERE username IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_email ON users (email) WHERE email IS NOT NULL;

-- Sessions
CREATE INDEX IF NOT EXISTS idx_sessions_user ON user_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON user_sessions (expires_at) WHERE is_revoked = false;

-- Participants
CREATE INDEX IF NOT EXISTS idx_participants_user ON participants (user_id);
CREATE INDEX IF NOT EXISTS idx_participants_conv ON participants (conversation_id);
CREATE INDEX IF NOT EXISTS idx_participants_role ON participants (conversation_id, role);

-- Messages
CREATE INDEX IF NOT EXISTS idx_messages_conv_seq ON messages (conversation_id, seq_id DESC);
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages (sender_id);
CREATE INDEX IF NOT EXISTS idx_messages_expires ON messages (expires_at) WHERE expires_at IS NOT NULL;

-- Receipts & reactions
CREATE INDEX IF NOT EXISTS idx_receipts_message ON message_receipts (message_id);
CREATE INDEX IF NOT EXISTS idx_reactions_message ON message_reactions (message_id);

-- Attachments
CREATE INDEX IF NOT EXISTS idx_attachments_uploader ON attachments (uploader_id);

-- Calls
CREATE INDEX IF NOT EXISTS idx_call_sessions_conv ON calls (conversation_id);
CREATE INDEX IF NOT EXISTS idx_call_participants_user ON call_participants (user_id);

-- Polls
CREATE INDEX IF NOT EXISTS idx_poll_options_poll ON poll_options (poll_id);
CREATE INDEX IF NOT EXISTS idx_poll_votes_user ON poll_votes (user_id);

-- Broadcasts
CREATE INDEX IF NOT EXISTS idx_broadcast_owner ON broadcast_lists (owner_id);


-- Message User States
CREATE INDEX IF NOT EXISTS idx_message_user_states_user ON message_user_states (user_id);
CREATE INDEX IF NOT EXISTS idx_message_user_states_deleted ON message_user_states (is_deleted);

-- Conversation Clears
CREATE INDEX IF NOT EXISTS idx_conversation_clears_user ON conversation_clears (user_id);

-- Upload Sessions
CREATE INDEX IF NOT EXISTS idx_upload_sessions_uploader ON upload_sessions (uploader_id);

-- Outbox Session
CREATE INDEX IF NOT EXISTS idx_outbox_status ON outbox_events(status);
CREATE INDEX IF NOT EXISTS idx_outbox_created_at ON outbox_events(created_at);
CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox_events(status, retry_count) WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_outbox_aggregate ON outbox_events(aggregate_type, aggregate_id);
