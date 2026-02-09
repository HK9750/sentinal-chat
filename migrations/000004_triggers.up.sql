CREATE OR REPLACE FUNCTION fn_outbox_on_message_ciphertext()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    msg_row RECORD;
BEGIN
    SELECT id, conversation_id, sender_id, seq_id, type, created_at
    INTO msg_row
    FROM messages
    WHERE id = NEW.message_id;

    IF msg_row.id IS NULL THEN
        RETURN NEW;
    END IF;

    INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
    VALUES (
        'message',
        NEW.message_id,
        'message.created',
        jsonb_build_object(
            'message_id', NEW.message_id,
            'conversation_id', msg_row.conversation_id,
            'sender_id', msg_row.sender_id,
            'sender_device_id', NEW.sender_device_id,
            'recipient_user_id', NEW.recipient_user_id,
            'recipient_device_id', NEW.recipient_device_id,
            'ciphertext', encode(NEW.ciphertext, 'base64'),
            'header', NEW.header,
            'seq_id', msg_row.seq_id,
            'type', msg_row.type,
            'created_at', msg_row.created_at
        )
    );

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

DROP TRIGGER IF EXISTS tr_outbox_on_message_ciphertext ON message_ciphertexts;
CREATE TRIGGER tr_outbox_on_message_ciphertext
AFTER INSERT ON message_ciphertexts FOR EACH ROW
EXECUTE FUNCTION fn_outbox_on_message_ciphertext();
