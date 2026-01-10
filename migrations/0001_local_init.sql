-- 0001_local_init.sql

-- -----------------------------------------------------------------------------
-- EXTENSIONS
-- -----------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "citext";

-- -----------------------------------------------------------------------------
-- ENUMS
-- -----------------------------------------------------------------------------
CREATE TYPE conversation_type AS ENUM ('DM', 'GROUP');
CREATE TYPE participant_role AS ENUM ('OWNER', 'ADMIN', 'MEMBER');
CREATE TYPE message_type AS ENUM ('TEXT', 'IMAGE', 'VIDEO', 'AUDIO', 'FILE', 'LOCATION', 'CONTACT', 'SYSTEM', 'STICKER', 'GIF');
CREATE TYPE delivery_status AS ENUM ('PENDING', 'SENT', 'DELIVERED', 'READ');

-- Settings Enums
CREATE TYPE privacy_setting AS ENUM ('EVERYONE', 'CONTACTS', 'NOBODY');
CREATE TYPE theme_mode AS ENUM ('SYSTEM', 'LIGHT', 'DARK');
CREATE TYPE language_code AS ENUM ('en', 'es', 'fr', 'de', 'pt', 'ru', 'hi', 'zh'); -- Expand as needed

-- -----------------------------------------------------------------------------
-- USERS
-- -----------------------------------------------------------------------------
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone_number CITEXT UNIQUE,  
    username CITEXT UNIQUE,
    display_name TEXT NOT NULL,
    bio TEXT,                    
    avatar_url TEXT,
    is_online BOOLEAN DEFAULT FALSE,
    last_seen_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_users_phone ON users(phone_number);
CREATE INDEX idx_users_username ON users(username);

-- -----------------------------------------------------------------------------
-- USER SETTINGS
-- "Perfect settings table"
-- -----------------------------------------------------------------------------
CREATE TABLE user_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    
    -- PRIVACY
    privacy_last_seen privacy_setting DEFAULT 'EVERYONE',
    privacy_profile_photo privacy_setting DEFAULT 'EVERYONE',
    privacy_about privacy_setting DEFAULT 'EVERYONE',
    privacy_groups privacy_setting DEFAULT 'EVERYONE', -- Who can add me to groups
    read_receipts BOOLEAN DEFAULT TRUE,
    
    -- NOTIFICATIONS (Global defaults, can be overridden per chat)
    notifications_enabled BOOLEAN DEFAULT TRUE,
    notification_sound TEXT DEFAULT 'default', 
    notification_vibrate BOOLEAN DEFAULT TRUE,
    
    -- APP PREFERENCES
    theme theme_mode DEFAULT 'SYSTEM',
    language language_code DEFAULT 'en',
    enter_to_send BOOLEAN DEFAULT TRUE,
    media_auto_download_wifi BOOLEAN DEFAULT TRUE,
    media_auto_download_mobile BOOLEAN DEFAULT FALSE,
    
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- -----------------------------------------------------------------------------
-- CONVERSATIONS
-- -----------------------------------------------------------------------------
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type conversation_type NOT NULL,
    subject TEXT,                
    description TEXT,
    avatar_url TEXT,
    expiry_seconds INT,          
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_conversations_updated ON conversations(updated_at DESC);

-- -----------------------------------------------------------------------------
-- PARTICIPANTS
-- -----------------------------------------------------------------------------
CREATE TABLE participants (
    conversation_id UUID REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role participant_role NOT NULL DEFAULT 'MEMBER',
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    added_by UUID REFERENCES users(id),
    
    -- Chat Specific Settings
    muted_until TIMESTAMP WITH TIME ZONE,
    pinned_at TIMESTAMP WITH TIME ZONE,
    archived BOOLEAN DEFAULT FALSE,

    last_read_sequence BIGINT DEFAULT 0,

    PRIMARY KEY (conversation_id, user_id)
);

CREATE INDEX idx_participants_user ON participants(user_id);

-- -----------------------------------------------------------------------------
-- CONVERSATION SEQUENCES
-- -----------------------------------------------------------------------------
CREATE TABLE conversation_sequences (
    conversation_id UUID PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    last_sequence BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- -----------------------------------------------------------------------------
-- MESSAGES
-- -----------------------------------------------------------------------------
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL, 
    seq_id BIGINT NOT NULL, 
    
    type message_type NOT NULL DEFAULT 'TEXT',
    content TEXT, 
    metadata JSONB DEFAULT '{}'::jsonb, 
    
    is_forwarded BOOLEAN DEFAULT FALSE,
    forwarded_from_msg_id UUID REFERENCES messages(id) ON DELETE SET NULL, 
    reply_to_msg_id UUID REFERENCES messages(id) ON DELETE SET NULL,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    edited_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,

    UNIQUE(conversation_id, seq_id)
);

CREATE INDEX idx_messages_history ON messages(conversation_id, seq_id DESC);
CREATE INDEX idx_messages_content_gin ON messages USING gin(to_tsvector('english', coalesce(content, '')));

-- -----------------------------------------------------------------------------
-- REACTIONS
-- -----------------------------------------------------------------------------
CREATE TABLE message_reactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reaction_code VARCHAR(64) NOT NULL, 
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(message_id, user_id)
);

CREATE INDEX idx_reactions_message ON message_reactions(message_id);

-- -----------------------------------------------------------------------------
-- RECEIPTS
-- -----------------------------------------------------------------------------
CREATE TABLE message_receipts (
    message_id UUID REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    status delivery_status NOT NULL DEFAULT 'PENDING',
    delivered_at TIMESTAMP WITH TIME ZONE,
    read_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id)
);

CREATE INDEX idx_receipts_message ON message_receipts(message_id);

-- -----------------------------------------------------------------------------
-- ATTACHMENTS
-- -----------------------------------------------------------------------------
CREATE TABLE attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    uploader_id UUID REFERENCES users(id) ON DELETE SET NULL,
    url TEXT NOT NULL,
    filename TEXT,
    mime_type TEXT NOT NULL, 
    size_bytes BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE message_attachments (
    message_id UUID REFERENCES messages(id) ON DELETE CASCADE,
    attachment_id UUID REFERENCES attachments(id) ON DELETE CASCADE,
    PRIMARY KEY (message_id, attachment_id)
);

-- -----------------------------------------------------------------------------
-- EVENTS
-- -----------------------------------------------------------------------------
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    aggregate_type TEXT NOT NULL, 
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,     
    payload JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_outbox_pending ON outbox_events(created_at) WHERE processed_at IS NULL;

-- -----------------------------------------------------------------------------
-- TRIGGER
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION fn_assign_message_sequence()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    next_seq BIGINT;
BEGIN
    INSERT INTO conversation_sequences (conversation_id, last_sequence)
    VALUES (NEW.conversation_id, 0)
    ON CONFLICT (conversation_id) DO NOTHING;

    UPDATE conversation_sequences
    SET last_sequence = last_sequence + 1,
        updated_at = NOW()
    WHERE conversation_id = NEW.conversation_id
    RETURNING last_sequence INTO next_seq;

    NEW.seq_id := next_seq;
    RETURN NEW;
END;
$$;

CREATE TRIGGER tr_messages_assign_sequence
BEFORE INSERT ON messages
FOR EACH ROW
EXECUTE PROCEDURE fn_assign_message_sequence();
