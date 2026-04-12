-- ============================================================
-- IN-APP NOTIFICATIONS
-- ============================================================

CREATE TABLE IF NOT EXISTS user_notification_settings (
    user_id               UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    in_app_enabled        BOOLEAN NOT NULL DEFAULT TRUE,
    sound_enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    show_message_preview  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notifications (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    conversation_id  UUID REFERENCES conversations(id) ON DELETE CASCADE,
    message_id       UUID REFERENCES messages(id) ON DELETE CASCADE,
    call_id          UUID REFERENCES calls(id) ON DELETE CASCADE,
    type             VARCHAR(64) NOT NULL,
    title            TEXT NOT NULL,
    body             TEXT NOT NULL,
    deep_link        TEXT NOT NULL,
    metadata         JSONB NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key       TEXT,
    is_read          BOOLEAN NOT NULL DEFAULT FALSE,
    read_at          TIMESTAMP,
    created_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_user_dedupe
    ON notifications (user_id, dedupe_key)
    WHERE dedupe_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_notifications_user_created
    ON notifications (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
    ON notifications (user_id, is_read, created_at DESC)
    WHERE is_read = FALSE;
