CREATE TABLE IF NOT EXISTS oauth_identities (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          TEXT NOT NULL,
    provider_user_id  TEXT NOT NULL,
    provider_email    CITEXT,
    email_verified    BOOLEAN DEFAULT FALSE,
    created_at        TIMESTAMP DEFAULT NOW(),
    updated_at        TIMESTAMP DEFAULT NOW(),
    UNIQUE (provider, provider_user_id),
    UNIQUE (user_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_oauth_identities_user
    ON oauth_identities (user_id);

CREATE INDEX IF NOT EXISTS idx_oauth_identities_provider_email
    ON oauth_identities (provider, provider_email)
    WHERE provider_email IS NOT NULL;

DROP TRIGGER IF EXISTS tr_oauth_identities_updated ON oauth_identities;
CREATE TRIGGER tr_oauth_identities_updated
    BEFORE UPDATE ON oauth_identities FOR EACH ROW
    EXECUTE FUNCTION fn_update_timestamp();
