package repository

import (
	"fmt"
	"sentinal-chat/internal/domain"

	"gorm.io/gorm"
)

// InitSchema handles the database schema migration.
// It creates necessary extensions, enums, triggers, and runs Gorm auto-migration.
func InitSchema(db *gorm.DB) error {
	// 1. Extensions
	// Note: Creating extensions usually requires superuser privileges.
	// If this fails, ensure the extensions are pre-installed or the user has permissions.
	extensions := []string{
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`,
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto";`,
		`CREATE EXTENSION IF NOT EXISTS "citext";`,
	}

	for _, ext := range extensions {
		if err := db.Exec(ext).Error; err != nil {
			return fmt.Errorf("failed to create extension: %w", err)
		}
	}

	// 2. Enums
	// We use 'DO $$ BEGIN ... END $$' block to safely create types only if they don't exist.
	enums := []string{
		`DO $$ BEGIN
			CREATE TYPE conversation_type AS ENUM ('DM', 'GROUP');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;`,
		`DO $$ BEGIN
			CREATE TYPE participant_role AS ENUM ('OWNER', 'ADMIN', 'MEMBER');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;`,
		`DO $$ BEGIN
			CREATE TYPE message_type AS ENUM ('TEXT', 'IMAGE', 'VIDEO', 'AUDIO', 'FILE', 'LOCATION', 'CONTACT', 'SYSTEM', 'STICKER', 'GIF');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;`,
		`DO $$ BEGIN
			CREATE TYPE delivery_status AS ENUM ('PENDING', 'SENT', 'DELIVERED', 'READ');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;`,
		`DO $$ BEGIN
			CREATE TYPE privacy_setting AS ENUM ('EVERYONE', 'CONTACTS', 'NOBODY');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;`,
		`DO $$ BEGIN
			CREATE TYPE theme_mode AS ENUM ('SYSTEM', 'LIGHT', 'DARK');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;`,
		`DO $$ BEGIN
			CREATE TYPE language_code AS ENUM ('en', 'es', 'fr', 'de', 'pt', 'ru', 'hi', 'zh');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;`,
	}

	for _, enum := range enums {
		if err := db.Exec(enum).Error; err != nil {
			return fmt.Errorf("failed to create enum: %w", err)
		}
	}

	// 3. AutoMigrate Tables
	// This uses the Domain models to create tables, columns, and indexes.
	// Note: We register all models here.
	if err := db.AutoMigrate(
		&domain.User{},
		&domain.UserSettings{},
		&domain.Conversation{},
		&domain.Participant{},
		&domain.ConversationSequence{},
		&domain.Message{},
		&domain.MessageReaction{},
		&domain.MessageReceipt{},
		&domain.Attachment{},
		&domain.OutboxEvent{},
	); err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}

	// 4. Triggers & Functions
	// Specifically for Message Sequence numbering per Conversation.

	// Function: fn_assign_message_sequence
	fnAssignSeq := `
	CREATE OR REPLACE FUNCTION fn_assign_message_sequence()
	RETURNS trigger LANGUAGE plpgsql AS $$
	DECLARE
		next_seq BIGINT;
	BEGIN
		-- Initialize sequence for conversation if not exists
		INSERT INTO conversation_sequences (conversation_id, last_sequence)
		VALUES (NEW.conversation_id, 0)
		ON CONFLICT (conversation_id) DO NOTHING;

		-- Increment sequence and get new value
		UPDATE conversation_sequences
		SET last_sequence = last_sequence + 1,
			updated_at = NOW()
		WHERE conversation_id = NEW.conversation_id
		RETURNING last_sequence INTO next_seq;

		-- Assign to the new message
		NEW.seq_id := next_seq;
		RETURN NEW;
	END;
	$$;`

	if err := db.Exec(fnAssignSeq).Error; err != nil {
		return fmt.Errorf("failed to create function fn_assign_message_sequence: %w", err)
	}

	// Trigger: tr_messages_assign_sequence
	// We drop it first to ensure we can recreate it (idempotency).
	triggerSQL := `
	DROP TRIGGER IF EXISTS tr_messages_assign_sequence ON messages;
	CREATE TRIGGER tr_messages_assign_sequence
	BEFORE INSERT ON messages
	FOR EACH ROW
	EXECUTE PROCEDURE fn_assign_message_sequence();`

	if err := db.Exec(triggerSQL).Error; err != nil {
		return fmt.Errorf("failed to create trigger tr_messages_assign_sequence: %w", err)
	}

	return nil
}
