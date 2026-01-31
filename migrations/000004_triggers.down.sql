DROP TRIGGER IF EXISTS tr_outbox_on_message ON messages;
DROP TRIGGER IF EXISTS tr_messages_consume_prekey ON messages;
DROP TRIGGER IF EXISTS tr_messages_assign_sequence ON messages;
DROP TRIGGER IF EXISTS tr_conversations_updated ON conversations;
DROP TRIGGER IF EXISTS tr_users_updated ON users;
DROP FUNCTION IF EXISTS fn_outbox_on_message();
