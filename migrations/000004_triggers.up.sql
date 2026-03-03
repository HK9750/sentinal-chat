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

-- Normalize DM pair columns
DROP TRIGGER IF EXISTS tr_conversations_dm_pair ON conversations;
DROP FUNCTION IF EXISTS fn_set_dm_pair();
CREATE OR REPLACE FUNCTION fn_set_dm_pair()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    user_a UUID;
    user_b UUID;
BEGIN
    IF NEW.type <> 'DM' THEN
        NEW.dm_user_id_a = NULL;
        NEW.dm_user_id_b = NULL;
        RETURN NEW;
    END IF;

    IF NEW.dm_user_id_a IS NOT NULL AND NEW.dm_user_id_b IS NOT NULL THEN
        IF NEW.dm_user_id_a > NEW.dm_user_id_b THEN
            user_a := NEW.dm_user_id_b;
            user_b := NEW.dm_user_id_a;
        ELSE
            user_a := NEW.dm_user_id_a;
            user_b := NEW.dm_user_id_b;
        END IF;
        NEW.dm_user_id_a = user_a;
        NEW.dm_user_id_b = user_b;
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER update_key_backups_updated_at
    BEFORE UPDATE ON key_backups
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();


CREATE TRIGGER tr_conversations_dm_pair
BEFORE INSERT OR UPDATE OF type, dm_user_id_a, dm_user_id_b ON conversations
FOR EACH ROW
EXECUTE FUNCTION fn_set_dm_pair();

-- Backfill DM pairs and merge duplicates at migration time
WITH dm_pairs AS (
  SELECT
    c.id AS conversation_id,
    LEAST(p1.user_id, p2.user_id) AS user_id_a,
    GREATEST(p1.user_id, p2.user_id) AS user_id_b
  FROM conversations c
  JOIN participants p1 ON p1.conversation_id = c.id
  JOIN participants p2 ON p2.conversation_id = c.id
  WHERE c.type = 'DM'
    AND p1.user_id <> p2.user_id
),
dm_unique AS (
  SELECT DISTINCT conversation_id, user_id_a, user_id_b
  FROM dm_pairs
)
UPDATE conversations c
SET dm_user_id_a = d.user_id_a,
    dm_user_id_b = d.user_id_b
FROM dm_unique d
WHERE c.id = d.conversation_id;

WITH dm_conversations AS (
  SELECT
    c.id AS conversation_id,
    c.created_at,
    c.dm_user_id_a AS user_id_a,
    c.dm_user_id_b AS user_id_b,
    ROW_NUMBER() OVER (
      PARTITION BY c.dm_user_id_a, c.dm_user_id_b
      ORDER BY c.created_at ASC, c.id ASC
    ) AS rn
  FROM conversations c
  WHERE c.type = 'DM'
    AND c.dm_user_id_a IS NOT NULL
    AND c.dm_user_id_b IS NOT NULL
),
duplicates AS (
  SELECT conversation_id, user_id_a, user_id_b
  FROM dm_conversations
  WHERE rn > 1
),
survivors AS (
  SELECT conversation_id, user_id_a, user_id_b
  FROM dm_conversations
  WHERE rn = 1
),
reassign_messages AS (
  UPDATE messages m
  SET conversation_id = s.conversation_id
  FROM duplicates d
  JOIN survivors s ON s.user_id_a = d.user_id_a AND s.user_id_b = d.user_id_b
  WHERE m.conversation_id = d.conversation_id
  RETURNING m.id
),
upsert_clears AS (
  INSERT INTO conversation_clears (conversation_id, user_id, cleared_at)
  SELECT s.conversation_id, cc.user_id, cc.cleared_at
  FROM conversation_clears cc
  JOIN duplicates d ON cc.conversation_id = d.conversation_id
  JOIN survivors s ON s.user_id_a = d.user_id_a AND s.user_id_b = d.user_id_b
  ON CONFLICT (conversation_id, user_id) DO UPDATE
  SET cleared_at = GREATEST(conversation_clears.cleared_at, EXCLUDED.cleared_at)
  RETURNING conversation_id
),
upsert_labels AS (
  INSERT INTO conversation_labels (conversation_id, label_id, user_id, created_at)
  SELECT s.conversation_id, cl.label_id, cl.user_id, cl.created_at
  FROM conversation_labels cl
  JOIN duplicates d ON cl.conversation_id = d.conversation_id
  JOIN survivors s ON s.user_id_a = d.user_id_a AND s.user_id_b = d.user_id_b
  ON CONFLICT DO NOTHING
  RETURNING conversation_id
),
update_calls AS (
  UPDATE calls c
  SET conversation_id = s.conversation_id
  FROM duplicates d
  JOIN survivors s ON s.user_id_a = d.user_id_a AND s.user_id_b = d.user_id_b
  WHERE c.conversation_id = d.conversation_id
  RETURNING c.id
),
update_scheduled AS (
  UPDATE scheduled_messages sm
  SET conversation_id = s.conversation_id
  FROM duplicates d
  JOIN survivors s ON s.user_id_a = d.user_id_a AND s.user_id_b = d.user_id_b
  WHERE sm.conversation_id = d.conversation_id
  RETURNING sm.id
),
delete_participants AS (
  DELETE FROM participants p
  USING duplicates d
  WHERE p.conversation_id = d.conversation_id
  RETURNING p.conversation_id
)
DELETE FROM conversations c
USING duplicates d
WHERE c.id = d.conversation_id;

INSERT INTO conversation_sequences (conversation_id, last_sequence, updated_at)
SELECT c.id, COALESCE(MAX(m.seq_id), 0), NOW()
FROM conversations c
LEFT JOIN messages m ON m.conversation_id = c.id
GROUP BY c.id
ON CONFLICT (conversation_id) DO UPDATE
SET last_sequence = EXCLUDED.last_sequence,
    updated_at = NOW();
