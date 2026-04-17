-- ============================================================
-- POLL INTEGRITY HARDENING
-- ============================================================

-- Remove dangling message.poll_id pointers before adding FK.
UPDATE messages m
SET poll_id = NULL
WHERE m.poll_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1
      FROM polls p
      WHERE p.id = m.poll_id
  );

-- Ensure poll votes always point to options that belong to the same poll.
DELETE FROM poll_votes pv
WHERE NOT EXISTS (
    SELECT 1
    FROM poll_options po
    WHERE po.id = pv.option_id
      AND po.poll_id = pv.poll_id
);

-- Keep a single poll per message so each poll-message maps cleanly.
WITH ranked AS (
    SELECT id,
           message_id,
           ROW_NUMBER() OVER (PARTITION BY message_id ORDER BY created_at ASC, id ASC) AS rn
    FROM polls
    WHERE message_id IS NOT NULL
),
duplicates AS (
    SELECT id
    FROM ranked
    WHERE rn > 1
)
DELETE FROM polls p
USING duplicates d
WHERE p.id = d.id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_polls_message_unique
    ON polls (message_id)
    WHERE message_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_polls_message
    ON polls (message_id)
    WHERE message_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_messages_poll_id
    ON messages (poll_id)
    WHERE poll_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_poll_options_poll_option_unique
    ON poll_options (poll_id, lower(btrim(option_text)));

CREATE UNIQUE INDEX IF NOT EXISTS idx_poll_options_poll_id_id
    ON poll_options (poll_id, id);

CREATE INDEX IF NOT EXISTS idx_poll_votes_poll
    ON poll_votes (poll_id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_messages_poll_id'
    ) THEN
        ALTER TABLE messages
            ADD CONSTRAINT fk_messages_poll_id
            FOREIGN KEY (poll_id)
            REFERENCES polls(id)
            ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_poll_votes_option_in_poll'
    ) THEN
        ALTER TABLE poll_votes
            ADD CONSTRAINT fk_poll_votes_option_in_poll
            FOREIGN KEY (poll_id, option_id)
            REFERENCES poll_options(poll_id, id)
            ON DELETE CASCADE;
    END IF;
END $$;
