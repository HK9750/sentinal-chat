CREATE OR REPLACE FUNCTION fn_outbox_on_message()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
    VALUES ('message', NEW.id, 'message.created', row_to_json(NEW));
    RETURN NEW;
END;
$$;

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

DROP TRIGGER IF EXISTS tr_messages_consume_prekey ON messages;
CREATE TRIGGER tr_messages_consume_prekey
AFTER INSERT ON messages FOR EACH ROW
WHEN (NEW.metadata ? 'prekey_id')
EXECUTE FUNCTION fn_consume_prekey();

DROP TRIGGER IF EXISTS tr_outbox_on_message ON messages;
CREATE TRIGGER tr_outbox_on_message
AFTER INSERT ON messages FOR EACH ROW
EXECUTE FUNCTION fn_outbox_on_message();
