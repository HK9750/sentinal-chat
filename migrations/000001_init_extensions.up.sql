-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "citext";

-- ============================================================
-- ENUMS (Idempotent)
-- ============================================================

-- Conversation types: direct message or group
DO $$ BEGIN
    CREATE TYPE conversation_type AS ENUM ('DM', 'GROUP');
EXCEPTION WHEN duplicate_object THEN null; END $$;

-- Participant role within a conversation
DO $$ BEGIN
    CREATE TYPE participant_role AS ENUM ('OWNER', 'ADMIN', 'MEMBER');
EXCEPTION WHEN duplicate_object THEN null; END $$;

-- Message content types
DO $$ BEGIN
    CREATE TYPE message_type AS ENUM (
        'TEXT', 'IMAGE', 'VIDEO', 'AUDIO', 'FILE',
        'GIF', 'EMOJI', 'POLL', 'SYSTEM'
    );
EXCEPTION WHEN duplicate_object THEN null; END $$;

-- Delivery lifecycle
DO $$ BEGIN
    CREATE TYPE delivery_status AS ENUM ('SENT', 'DELIVERED', 'READ', 'PLAYED');
EXCEPTION WHEN duplicate_object THEN null; END $$;

-- Disappearing messages schedule
DO $$ BEGIN
    CREATE TYPE disappearing_mode AS ENUM ('OFF', '24_HOURS', '7_DAYS', '90_DAYS');
EXCEPTION WHEN duplicate_object THEN null; END $$;

-- Call types
DO $$ BEGIN
    CREATE TYPE call_type AS ENUM ('AUDIO', 'VIDEO');
EXCEPTION WHEN duplicate_object THEN null; END $$;

-- How a call ended
DO $$ BEGIN
    CREATE TYPE call_end_reason AS ENUM ('COMPLETED', 'MISSED', 'DECLINED', 'FAILED', 'TIMEOUT');
EXCEPTION WHEN duplicate_object THEN null; END $$;

-- Call participant status
DO $$ BEGIN
    CREATE TYPE participant_call_status AS ENUM ('INVITED', 'RINGING', 'CONNECTED', 'LEFT', 'DECLINED');
EXCEPTION WHEN duplicate_object THEN null; END $$;

-- Command pattern lifecycle
DO $$ BEGIN
    CREATE TYPE command_status AS ENUM ('PENDING', 'EXECUTED', 'FAILED', 'UNDONE');
EXCEPTION WHEN duplicate_object THEN null; END $$;

-- Outbox reliable delivery status
DO $$ BEGIN
    CREATE TYPE outbox_status AS ENUM ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED');
EXCEPTION WHEN duplicate_object THEN null; END $$;

-- Upload session status
DO $$ BEGIN
    CREATE TYPE upload_status AS ENUM ('IN_PROGRESS', 'COMPLETED', 'FAILED');
EXCEPTION WHEN duplicate_object THEN null; END $$;

-- ============================================================
-- FUNCTIONS
-- ============================================================

-- Auto-increment sequence number per conversation
CREATE OR REPLACE FUNCTION fn_assign_message_sequence()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    next_seq BIGINT;
BEGIN
    INSERT INTO conversation_sequences (conversation_id, last_sequence)
    VALUES (NEW.conversation_id, 0)
    ON CONFLICT (conversation_id) DO NOTHING;

    UPDATE conversation_sequences
    SET last_sequence = last_sequence + 1, updated_at = NOW()
    WHERE conversation_id = NEW.conversation_id
    RETURNING last_sequence INTO next_seq;

    NEW.seq_id := next_seq;
    RETURN NEW;
END;
$$;

-- Auto-update updated_at on row change
CREATE OR REPLACE FUNCTION fn_update_timestamp()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

-- Soft-redact view-once attachment after it is viewed
CREATE OR REPLACE FUNCTION fn_mark_view_once()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RETURN NEW;
END;
$$;
