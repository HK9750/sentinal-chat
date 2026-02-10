-- Drop triggers if they exist, then recreate
DROP TRIGGER IF EXISTS tr_messages_assign_sequence ON messages;
CREATE TRIGGER tr_messages_assign_sequence
BEFORE INSERT ON messages FOR EACH ROW
EXECUTE FUNCTION fn_assign_message_sequence();

DROP TRIGGER IF EXISTS tr_users_updated ON users;
CREATE TRIGGER tr_users_updated
BEFORE UPDATE ON users FOR EACH ROW
EXECUTE FUNCTION fn_update_timestamp();

DROP TRIGGER IF EXISTS tr_conversations_updated ON conversations;
CREATE TRIGGER tr_conversations_updated
BEFORE UPDATE ON conversations FOR EACH ROW
EXECUTE FUNCTION fn_update_timestamp();
