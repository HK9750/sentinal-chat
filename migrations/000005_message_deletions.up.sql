-- ============================================================
-- PER-USER MESSAGE DELETIONS (Delete for me)
-- ============================================================

CREATE TABLE IF NOT EXISTS message_deletions (
    message_id   UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    deleted_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_message_deletions_user
    ON message_deletions (user_id, deleted_at DESC);

CREATE INDEX IF NOT EXISTS idx_message_deletions_message
    ON message_deletions (message_id);
