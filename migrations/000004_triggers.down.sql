DROP TRIGGER IF EXISTS tr_messages_assign_sequence ON messages;
DROP TRIGGER IF EXISTS tr_conversations_updated ON conversations;
DROP TRIGGER IF EXISTS tr_conversations_dm_pair ON conversations;
DROP FUNCTION IF EXISTS fn_set_dm_pair();
ALTER TABLE conversations
  DROP COLUMN IF EXISTS dm_user_id_a,
  DROP COLUMN IF EXISTS dm_user_id_b;
DROP TRIGGER IF EXISTS tr_users_updated ON users;
