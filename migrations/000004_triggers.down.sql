DROP TRIGGER IF EXISTS tr_messages_assign_sequence ON messages;
DROP TRIGGER IF EXISTS tr_conversations_updated ON conversations;
DROP TRIGGER IF EXISTS tr_conversations_dm_pair ON conversations;
DROP FUNCTION IF EXISTS fn_set_dm_pair();
DROP TRIGGER IF EXISTS tr_users_updated ON users;
DROP TRIGGER IF EXISTS update_key_backups_updated_at ON key_backups;
