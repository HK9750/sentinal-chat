ALTER TABLE user_sessions
    ADD COLUMN IF NOT EXISTS auth_provider TEXT;

UPDATE user_sessions
SET auth_provider = 'password'
WHERE auth_provider IS NULL OR btrim(auth_provider) = '';

ALTER TABLE user_sessions
    ALTER COLUMN auth_provider SET DEFAULT 'password';

ALTER TABLE user_sessions
    ALTER COLUMN auth_provider SET NOT NULL;
