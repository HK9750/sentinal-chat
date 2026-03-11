DROP TRIGGER IF EXISTS tr_oauth_identities_updated ON oauth_identities;
DROP INDEX IF EXISTS idx_oauth_identities_provider_email;
DROP INDEX IF EXISTS idx_oauth_identities_user;
DROP TABLE IF EXISTS oauth_identities;
