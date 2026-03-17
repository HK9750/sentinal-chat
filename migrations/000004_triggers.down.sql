DROP TRIGGER IF EXISTS tr_attachments_view_once ON attachments;
DROP TRIGGER IF EXISTS tr_conversations_dm_pair ON conversations;
DROP FUNCTION IF EXISTS fn_set_dm_pair();
DROP TRIGGER IF EXISTS tr_oauth_identities_updated ON oauth_identities;
DROP TRIGGER IF EXISTS tr_conversations_updated ON conversations;
DROP TRIGGER IF EXISTS tr_messages_assign_sequence ON messages;
DROP TRIGGER IF EXISTS tr_users_updated ON users;
