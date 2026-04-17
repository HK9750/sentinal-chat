DROP INDEX IF EXISTS idx_poll_votes_poll;
DROP INDEX IF EXISTS idx_poll_options_poll_id_id;
DROP INDEX IF EXISTS idx_poll_options_poll_option_unique;
DROP INDEX IF EXISTS idx_messages_poll_id;
DROP INDEX IF EXISTS idx_polls_message;
DROP INDEX IF EXISTS idx_polls_message_unique;

ALTER TABLE poll_votes
    DROP CONSTRAINT IF EXISTS fk_poll_votes_option_in_poll;

ALTER TABLE messages
    DROP CONSTRAINT IF EXISTS fk_messages_poll_id;
