-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "citext";

-- Enums (Idempotent)
DO $$ BEGIN
    CREATE TYPE conversation_type AS ENUM ('DM', 'GROUP');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- System-wide user roles (different from participant_role which is for conversations)
DO $$ BEGIN
    CREATE TYPE user_role AS ENUM ('SUPER_ADMIN', 'ADMIN', 'MODERATOR', 'USER');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE participant_role AS ENUM ('OWNER', 'ADMIN', 'MEMBER');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE message_type AS ENUM (
      'TEXT', 'IMAGE', 'VIDEO', 'AUDIO', 'FILE',
      'LOCATION', 'CONTACT', 'SYSTEM', 'STICKER', 'GIF', 'POLL'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE delivery_status AS ENUM ('PENDING', 'SENT', 'DELIVERED', 'READ', 'PLAYED');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE privacy_setting AS ENUM ('EVERYONE', 'CONTACTS', 'NOBODY');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE theme_mode AS ENUM ('SYSTEM', 'LIGHT', 'DARK');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE language_code AS ENUM ('en', 'es', 'fr', 'de', 'pt', 'ru', 'hi', 'zh', 'ar', 'ja');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE disappearing_mode AS ENUM ('OFF', '24_HOURS', '7_DAYS', '90_DAYS');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE call_type AS ENUM ('AUDIO', 'VIDEO');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE call_topology AS ENUM ('P2P');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE outbox_status AS ENUM ('PENDING','PROCESSING','COMPLETED','FAILED');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE call_end_reason AS ENUM ('COMPLETED', 'MISSED', 'DECLINED', 'FAILED', 'TIMEOUT', 'NETWORK_ERROR');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE participant_call_status AS ENUM ('INVITED', 'RINGING', 'CONNECTED', 'ON_HOLD', 'LEFT', 'DECLINED');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE command_status AS ENUM ('PENDING', 'EXECUTED', 'FAILED');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE upload_status AS ENUM ('IN_PROGRESS', 'COMPLETED', 'FAILED');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Functions & Triggers

-- Message Sequence Assignment
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

-- Auto-Update Timestamps
CREATE OR REPLACE FUNCTION fn_update_timestamp()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

-- View Once Expiry
CREATE OR REPLACE FUNCTION fn_mark_view_once() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.view_once = TRUE AND NEW.viewed_at IS NOT NULL THEN
    UPDATE attachments SET url = NULL WHERE id = NEW.id; -- soft redact
  END IF;
  RETURN NEW;
END;
$$;
