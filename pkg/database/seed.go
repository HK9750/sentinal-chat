package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"sentinal-chat/internal/domain/broadcast"
	"sentinal-chat/internal/domain/conversation"
	"sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/domain/user"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// SeedConfig holds configuration for seeding the database
type SeedConfig struct {
	AdminEmail       string
	AdminPassword    string
	AdminUsername    string
	AdminDisplayName string
	CreateTestUsers  bool
	TestUserCount    int
}

// DefaultSeedConfig returns default seed configuration
func DefaultSeedConfig() *SeedConfig {
	return &SeedConfig{
		AdminEmail:       "admin@sentinal.chat",
		AdminPassword:    "Admin@123!",
		AdminUsername:    "admin",
		AdminDisplayName: "System Admin",
		CreateTestUsers:  true,
		TestUserCount:    5,
	}
}

// SeedResult holds the result of the seeding operation
type SeedResult struct {
	AdminUser     *user.User
	TestUsers     []*user.User
	Conversations []*conversation.Conversation
	Messages      []*message.Message
}

// Seed runs the complete database seeding
func Seed(cfg *SeedConfig) (*SeedResult, error) {
	if cfg == nil {
		cfg = DefaultSeedConfig()
	}

	result := &SeedResult{}
	deviceMap := make(map[uuid.UUID][]user.Device)

	log.Println("Starting database seeding...")

	// Create admin user
	adminUser, err := seedAdminUser(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to seed admin user: %w", err)
	}
	result.AdminUser = adminUser

	adminDevices, err := seedDevices([]*user.User{adminUser})
	if err != nil {
		return nil, fmt.Errorf("failed to seed admin devices: %w", err)
	}
	for userID, list := range adminDevices {
		deviceMap[userID] = append(deviceMap[userID], list...)
	}

	if cfg.CreateTestUsers {
		testUsers, err := seedTestUsers(cfg.TestUserCount)
		if err != nil {
			return nil, fmt.Errorf("failed to seed test users: %w", err)
		}
		result.TestUsers = testUsers

		if len(testUsers) >= 2 {
			devices, err := seedDevices(testUsers)
			if err != nil {
				return nil, fmt.Errorf("failed to seed test devices: %w", err)
			}
			for userID, list := range devices {
				deviceMap[userID] = append(deviceMap[userID], list...)
			}
			if err := seedEncryptionKeys(deviceMap); err != nil {
				return nil, fmt.Errorf("failed to seed encryption keys: %w", err)
			}

			convs, err := seedConversations(testUsers)
			if err != nil {
				return nil, fmt.Errorf("failed to seed conversations: %w", err)
			}
			result.Conversations = convs

			msgs, err := seedMessages(convs, testUsers, deviceMap)
			if err != nil {
				return nil, fmt.Errorf("failed to seed messages: %w", err)
			}
			result.Messages = msgs
		}

		if err := seedBroadcasts(testUsers); err != nil {
			return nil, fmt.Errorf("failed to seed broadcasts: %w", err)
		}
	}

	log.Println("Database seeding completed successfully!")
	return result, nil
}

// SeedMinimal runs minimal seeding (admin user only)
func SeedMinimal(cfg *SeedConfig) (*user.User, error) {
	if cfg == nil {
		cfg = DefaultSeedConfig()
	}
	return seedAdminUser(cfg)
}

// seedAdminUser creates the admin user
func seedAdminUser(cfg *SeedConfig) (*user.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	adminUser := &user.User{
		ID:           uuid.New(),
		Email:        sql.NullString{String: cfg.AdminEmail, Valid: true},
		Username:     sql.NullString{String: cfg.AdminUsername, Valid: true},
		PasswordHash: string(hashedPassword),
		DisplayName:  cfg.AdminDisplayName,
		Role:         "SUPER_ADMIN",
		Bio:          "System Administrator",
		IsActive:     true,
		IsVerified:   true,
		IsOnline:     false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	ctx := context.Background()
	err = WithTx(ctx, DB, func(tx *sql.Tx) error {
		var existing user.User
		if err := tx.QueryRowContext(ctx, "SELECT id FROM users WHERE email = $1", cfg.AdminEmail).Scan(&existing.ID); err == nil {
			log.Println("Admin user already exists, skipping creation")
			adminUser.ID = existing.ID
			return nil
		}

		_, err := tx.ExecContext(ctx, `
            INSERT INTO users (id, email, username, password_hash, display_name, role, bio, is_active, is_verified, is_online, created_at, updated_at)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
        `,
			adminUser.ID,
			adminUser.Email,
			adminUser.Username,
			adminUser.PasswordHash,
			adminUser.DisplayName,
			adminUser.Role,
			adminUser.Bio,
			adminUser.IsActive,
			adminUser.IsVerified,
			adminUser.IsOnline,
			adminUser.CreatedAt,
			adminUser.UpdatedAt,
		)
		if err != nil {
			return err
		}

		settings := &user.UserSettings{
			UserID:                  adminUser.ID,
			PrivacyLastSeen:         "CONTACTS",
			PrivacyProfilePhoto:     "CONTACTS",
			PrivacyAbout:            "CONTACTS",
			PrivacyGroups:           "CONTACTS",
			ReadReceipts:            true,
			NotificationsEnabled:    true,
			NotificationSound:       "default",
			NotificationVibrate:     true,
			Theme:                   "SYSTEM",
			Language:                "en",
			EnterToSend:             true,
			MediaAutoDownloadWiFi:   true,
			MediaAutoDownloadMobile: false,
			UpdatedAt:               time.Now(),
		}
		_, err = tx.ExecContext(ctx, `
            INSERT INTO user_settings (
                user_id, privacy_last_seen, privacy_profile_photo, privacy_about, privacy_groups,
                read_receipts, notifications_enabled, notification_sound, notification_vibrate,
                theme, language, enter_to_send, media_auto_download_wifi, media_auto_download_mobile, updated_at
            ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
        `,
			settings.UserID,
			settings.PrivacyLastSeen,
			settings.PrivacyProfilePhoto,
			settings.PrivacyAbout,
			settings.PrivacyGroups,
			settings.ReadReceipts,
			settings.NotificationsEnabled,
			settings.NotificationSound,
			settings.NotificationVibrate,
			settings.Theme,
			settings.Language,
			settings.EnterToSend,
			settings.MediaAutoDownloadWiFi,
			settings.MediaAutoDownloadMobile,
			settings.UpdatedAt,
		)
		return err
	})

	if err != nil {
		return nil, err
	}

	log.Printf("Admin user seeded: %s (%s)", cfg.AdminEmail, adminUser.ID)
	return adminUser, nil
}

// seedTestUsers creates test users for development
func seedTestUsers(count int) ([]*user.User, error) {
	users := make([]*user.User, 0, count)

	testUserData := []struct {
		email       string
		username    string
		displayName string
		bio         string
		phone       string
	}{
		{"alice@test.com", "alice", "Alice Johnson", "Coffee enthusiast & coder", "+1234567001"},
		{"bob@test.com", "bob", "Bob Smith", "Tech lover", "+1234567002"},
		{"charlie@test.com", "charlie", "Charlie Brown", "Always curious", "+1234567003"},
		{"diana@test.com", "diana", "Diana Prince", "Wonder woman of tech", "+1234567004"},
		{"edward@test.com", "edward", "Edward Chen", "Full-stack developer", "+1234567005"},
		{"fiona@test.com", "fiona", "Fiona Green", "UX Designer", "+1234567006"},
		{"george@test.com", "george", "George Miller", "DevOps engineer", "+1234567007"},
		{"hannah@test.com", "hannah", "Hannah White", "Data scientist", "+1234567008"},
	}

	password := "Test@123!"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	for i := 0; i < count && i < len(testUserData); i++ {
		data := testUserData[i]

		var existingID uuid.UUID
		if err := DB.QueryRowContext(ctx, "SELECT id FROM users WHERE email = $1", data.email).Scan(&existingID); err == nil {
			log.Printf("Test user %s already exists, skipping", data.email)
			users = append(users, &user.User{ID: existingID, Email: sql.NullString{String: data.email, Valid: true}})
			continue
		}

		newUser := &user.User{
			ID:           uuid.New(),
			Email:        sql.NullString{String: data.email, Valid: true},
			Username:     sql.NullString{String: data.username, Valid: true},
			PhoneNumber:  sql.NullString{String: data.phone, Valid: true},
			PasswordHash: string(hashedPassword),
			DisplayName:  data.displayName,
			Role:         "USER",
			Bio:          data.bio,
			IsActive:     true,
			IsVerified:   true,
			IsOnline:     i%2 == 0,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		err := WithTx(ctx, DB, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
                INSERT INTO users (id, email, username, phone_number, password_hash, display_name, role, bio, is_active, is_verified, is_online, created_at, updated_at)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
            `, newUser.ID, newUser.Email, newUser.Username, newUser.PhoneNumber, newUser.PasswordHash, newUser.DisplayName, newUser.Role, newUser.Bio, newUser.IsActive, newUser.IsVerified, newUser.IsOnline, newUser.CreatedAt, newUser.UpdatedAt)
			if err != nil {
				return err
			}

			settings := &user.UserSettings{
				UserID:               newUser.ID,
				PrivacyLastSeen:      "EVERYONE",
				PrivacyProfilePhoto:  "EVERYONE",
				PrivacyAbout:         "EVERYONE",
				PrivacyGroups:        "EVERYONE",
				ReadReceipts:         true,
				NotificationsEnabled: true,
				Theme:                "SYSTEM",
				Language:             "en",
				UpdatedAt:            time.Now(),
			}
			_, err = tx.ExecContext(ctx, `
                INSERT INTO user_settings (
                    user_id, privacy_last_seen, privacy_profile_photo, privacy_about, privacy_groups,
                    read_receipts, notifications_enabled, theme, language, updated_at
                ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
            `, settings.UserID, settings.PrivacyLastSeen, settings.PrivacyProfilePhoto, settings.PrivacyAbout, settings.PrivacyGroups, settings.ReadReceipts, settings.NotificationsEnabled, settings.Theme, settings.Language, settings.UpdatedAt)
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create test user %s: %w", data.email, err)
		}

		users = append(users, newUser)
		log.Printf("Test user seeded: %s", data.email)
	}

	return users, nil
}

// seedConversations creates sample conversations
func seedConversations(users []*user.User) ([]*conversation.Conversation, error) {
	conversations := make([]*conversation.Conversation, 0)

	if len(users) >= 2 {
		emptyJSON := "{}"
		dmConv := &conversation.Conversation{
			ID:               uuid.New(),
			Type:             "DM",
			DisappearingMode: "OFF",
			GroupPermissions: &emptyJSON,
			CreatedBy:        uuid.NullUUID{UUID: users[0].ID, Valid: true},
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		ctx := context.Background()
		err := WithTx(ctx, DB, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
                INSERT INTO conversations (id, type, subject, description, avatar_url, expiry_seconds, disappearing_mode, message_expiry_seconds, group_permissions, invite_link, invite_link_revoked_at, created_by, created_at, updated_at)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
            `, dmConv.ID, dmConv.Type, dmConv.Subject, dmConv.Description, dmConv.AvatarURL, dmConv.ExpirySeconds, dmConv.DisappearingMode, dmConv.MessageExpirySeconds, dmConv.GroupPermissions, dmConv.InviteLink, dmConv.InviteLinkRevokedAt, dmConv.CreatedBy, dmConv.CreatedAt, dmConv.UpdatedAt)
			if err != nil {
				return err
			}

			for _, u := range users[:2] {
				participant := &conversation.Participant{
					ConversationID:   dmConv.ID,
					UserID:           u.ID,
					Role:             "MEMBER",
					JoinedAt:         time.Now(),
					LastReadSequence: 0,
					Permissions:      &emptyJSON,
				}
				_, err := tx.ExecContext(ctx, `
                    INSERT INTO participants (conversation_id, user_id, role, joined_at, added_by, muted_until, pinned_at, archived, last_read_sequence, permissions)
                    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
                `, participant.ConversationID, participant.UserID, participant.Role, participant.JoinedAt, participant.AddedBy, participant.MutedUntil, participant.PinnedAt, participant.Archived, participant.LastReadSequence, participant.Permissions)
				if err != nil {
					return err
				}
			}

			_, err = tx.ExecContext(ctx, `
                INSERT INTO conversation_sequences (conversation_id, last_sequence, updated_at)
                VALUES ($1,$2,$3)
            `, dmConv.ID, int64(0), time.Now())
			return err
		})

		if err != nil {
			return nil, err
		}
		conversations = append(conversations, dmConv)
		log.Printf("DM conversation seeded: %s", dmConv.ID)
	}

	if len(users) >= 3 {
		groupPerms, _ := json.Marshal(map[string]interface{}{
			"send_messages":   true,
			"send_media":      true,
			"add_members":     false,
			"edit_group_info": false,
		})
		groupPermsStr := string(groupPerms)

		groupConv := &conversation.Conversation{
			ID:               uuid.New(),
			Type:             "GROUP",
			Subject:          sql.NullString{String: "Sentinal Chat Team", Valid: true},
			Description:      sql.NullString{String: "Welcome to our team chat!", Valid: true},
			DisappearingMode: "OFF",
			GroupPermissions: &groupPermsStr,
			CreatedBy:        uuid.NullUUID{UUID: users[0].ID, Valid: true},
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		ctx := context.Background()
		err := WithTx(ctx, DB, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
                INSERT INTO conversations (id, type, subject, description, avatar_url, expiry_seconds, disappearing_mode, message_expiry_seconds, group_permissions, invite_link, invite_link_revoked_at, created_by, created_at, updated_at)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
            `, groupConv.ID, groupConv.Type, groupConv.Subject, groupConv.Description, groupConv.AvatarURL, groupConv.ExpirySeconds, groupConv.DisappearingMode, groupConv.MessageExpirySeconds, groupConv.GroupPermissions, groupConv.InviteLink, groupConv.InviteLinkRevokedAt, groupConv.CreatedBy, groupConv.CreatedAt, groupConv.UpdatedAt)
			if err != nil {
				return err
			}

			roles := []string{"OWNER", "ADMIN", "MEMBER"}
			for i, u := range users {
				if i >= len(roles) {
					break
				}
				emptyJSON := "{}"
				participant := &conversation.Participant{
					ConversationID:   groupConv.ID,
					UserID:           u.ID,
					Role:             roles[i],
					JoinedAt:         time.Now(),
					LastReadSequence: 0,
					Permissions:      &emptyJSON,
				}
				if i > 0 {
					participant.AddedBy = uuid.NullUUID{UUID: users[0].ID, Valid: true}
				}
				_, err := tx.ExecContext(ctx, `
                    INSERT INTO participants (conversation_id, user_id, role, joined_at, added_by, muted_until, pinned_at, archived, last_read_sequence, permissions)
                    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
                `, participant.ConversationID, participant.UserID, participant.Role, participant.JoinedAt, participant.AddedBy, participant.MutedUntil, participant.PinnedAt, participant.Archived, participant.LastReadSequence, participant.Permissions)
				if err != nil {
					return err
				}
			}

			_, err = tx.ExecContext(ctx, `
                INSERT INTO conversation_sequences (conversation_id, last_sequence, updated_at)
                VALUES ($1,$2,$3)
            `, groupConv.ID, int64(0), time.Now())
			return err
		})

		if err != nil {
			return nil, err
		}
		conversations = append(conversations, groupConv)
		log.Printf("Group conversation seeded: %s", groupConv.ID)
	}

	return conversations, nil
}

// seedMessages creates sample messages in conversations
func seedMessages(convs []*conversation.Conversation, users []*user.User, deviceMap map[uuid.UUID][]user.Device) ([]*message.Message, error) {
	messages := make([]*message.Message, 0)

	sampleMessages := []struct {
		content  string
		msgType  string
		metadata map[string]interface{}
	}{
		{"Hello everyone", "TEXT", nil},
		{"Welcome to Sentinal Chat", "TEXT", nil},
		{"This is a test message", "TEXT", nil},
		{"How is everyone doing today?", "TEXT", nil},
		{"Great to be here", "TEXT", nil},
		{"Check out this cool feature", "TEXT", nil},
		{"Looking forward to our collaboration", "TEXT", nil},
		{"Let me know if you need anything", "TEXT", nil},
	}

	for _, conv := range convs {
		for i, msgData := range sampleMessages {
			if i >= len(users) {
				break
			}

			metadata, _ := json.Marshal(msgData.metadata)
			if msgData.metadata == nil {
				metadata = []byte("{}")
			}

			msg := &message.Message{
				ID:             uuid.New(),
				ConversationID: conv.ID,
				SenderID:       users[i%len(users)].ID,
				Type:           msgData.msgType,
				Metadata:       string(metadata),
				IsForwarded:    false,
				MentionCount:   0,
				CreatedAt:      time.Now().Add(time.Duration(i) * time.Minute),
			}
			msg.Metadata = string(mustJSON(map[string]interface{}{"e2ee": true, "sample": true}))

			if _, err := DB.Exec(`
                INSERT INTO messages (id, conversation_id, sender_id, type, metadata, is_forwarded, mention_count, created_at)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
            `, msg.ID, msg.ConversationID, msg.SenderID, msg.Type, msg.Metadata, msg.IsForwarded, msg.MentionCount, msg.CreatedAt); err != nil {
				return nil, fmt.Errorf("failed to create message: %w", err)
			}

			if err := seedMessageCiphertexts(msg, deviceMap); err != nil {
				return nil, fmt.Errorf("failed to seed message ciphertexts: %w", err)
			}
			messages = append(messages, msg)
		}
	}

	log.Printf("Seeded %d messages", len(messages))
	return messages, nil
}

type identityKeySeed struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	DeviceID  uuid.UUID
	PublicKey []byte
	IsActive  bool
	CreatedAt time.Time
}

type signedPreKeySeed struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	DeviceID  uuid.UUID
	KeyID     int
	PublicKey []byte
	Signature []byte
	CreatedAt time.Time
	IsActive  bool
}

type oneTimePreKeySeed struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	DeviceID           uuid.UUID
	KeyID              int
	PublicKey          []byte
	UploadedAt         time.Time
	ConsumedAt         sql.NullTime
	ConsumedBy         uuid.NullUUID
	ConsumedByDeviceID uuid.NullUUID
}

type messageCiphertextSeed struct {
	ID                uuid.UUID
	MessageID         uuid.UUID
	RecipientUserID   uuid.UUID
	RecipientDeviceID uuid.UUID
	SenderDeviceID    uuid.NullUUID
	Ciphertext        []byte
	Header            string
	CreatedAt         time.Time
}

func seedDevices(users []*user.User) (map[uuid.UUID][]user.Device, error) {
	deviceMap := make(map[uuid.UUID][]user.Device)
	for i, u := range users {
		for d := 0; d < 2; d++ {
			device := user.Device{
				ID:           uuid.New(),
				UserID:       u.ID,
				DeviceID:     fmt.Sprintf("device-%d-%d", i+1, d+1),
				DeviceName:   fmt.Sprintf("Device %d", d+1),
				DeviceType:   "MOBILE",
				IsActive:     true,
				RegisteredAt: time.Now(),
				LastSeenAt:   sql.NullTime{Time: time.Now(), Valid: true},
			}
			if _, err := DB.Exec(`
                INSERT INTO devices (id, user_id, device_id, device_name, device_type, is_active, registered_at, last_seen_at)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
            `, device.ID, device.UserID, device.DeviceID, device.DeviceName, device.DeviceType, device.IsActive, device.RegisteredAt, device.LastSeenAt); err != nil {
				return nil, err
			}
			deviceMap[u.ID] = append(deviceMap[u.ID], device)
		}
	}
	return deviceMap, nil
}

func seedEncryptionKeys(deviceMap map[uuid.UUID][]user.Device) error {
	for userID, devices := range deviceMap {
		for _, device := range devices {
			identity := identityKeySeed{
				ID:        uuid.New(),
				UserID:    userID,
				DeviceID:  device.ID,
				PublicKey: randomBytes(32),
				IsActive:  true,
				CreatedAt: time.Now(),
			}
			if _, err := DB.Exec(`
                INSERT INTO identity_keys (id, user_id, device_id, public_key, is_active, created_at)
                VALUES ($1,$2,$3,$4,$5,$6)
            `, identity.ID, identity.UserID, identity.DeviceID, identity.PublicKey, identity.IsActive, identity.CreatedAt); err != nil {
				return err
			}

			signed := signedPreKeySeed{
				ID:        uuid.New(),
				UserID:    userID,
				DeviceID:  device.ID,
				KeyID:     1,
				PublicKey: randomBytes(32),
				Signature: randomBytes(64),
				IsActive:  true,
				CreatedAt: time.Now(),
			}
			if _, err := DB.Exec(`
                INSERT INTO signed_prekeys (id, user_id, device_id, key_id, public_key, signature, created_at, is_active)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
            `, signed.ID, signed.UserID, signed.DeviceID, signed.KeyID, signed.PublicKey, signed.Signature, signed.CreatedAt, signed.IsActive); err != nil {
				return err
			}

			for k := 0; k < 5; k++ {
				prekey := oneTimePreKeySeed{
					ID:         uuid.New(),
					UserID:     userID,
					DeviceID:   device.ID,
					KeyID:      100 + k,
					PublicKey:  randomBytes(32),
					UploadedAt: time.Now(),
				}
				if _, err := DB.Exec(`
                    INSERT INTO onetime_prekeys (id, user_id, device_id, key_id, public_key, uploaded_at)
                    VALUES ($1,$2,$3,$4,$5,$6)
                `, prekey.ID, prekey.UserID, prekey.DeviceID, prekey.KeyID, prekey.PublicKey, prekey.UploadedAt); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func seedMessageCiphertexts(msg *message.Message, deviceMap map[uuid.UUID][]user.Device) error {
	var participants []conversation.Participant
	rows, err := DB.Query("SELECT conversation_id, user_id FROM participants WHERE conversation_id = $1", msg.ConversationID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var p conversation.Participant
		if err := rows.Scan(&p.ConversationID, &p.UserID); err != nil {
			return err
		}
		participants = append(participants, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	senderDevices := deviceMap[msg.SenderID]
	var senderDeviceID uuid.NullUUID
	if len(senderDevices) > 0 {
		senderDeviceID = uuid.NullUUID{UUID: senderDevices[0].ID, Valid: true}
	}

	header := string(mustJSON(map[string]interface{}{
		"version": 1,
		"cipher":  "signal",
	}))

	for _, p := range participants {
		devices := deviceMap[p.UserID]
		for _, device := range devices {
			ciphertext := messageCiphertextSeed{
				ID:                uuid.New(),
				MessageID:         msg.ID,
				RecipientUserID:   p.UserID,
				RecipientDeviceID: device.ID,
				SenderDeviceID:    senderDeviceID,
				Ciphertext:        randomBytes(64),
				Header:            header,
				CreatedAt:         time.Now(),
			}
			if _, err := DB.Exec(`
                INSERT INTO message_ciphertexts (id, message_id, recipient_user_id, recipient_device_id, sender_device_id, ciphertext, header, created_at)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
            `, ciphertext.ID, ciphertext.MessageID, ciphertext.RecipientUserID, ciphertext.RecipientDeviceID, ciphertext.SenderDeviceID, ciphertext.Ciphertext, ciphertext.Header, ciphertext.CreatedAt); err != nil {
				return err
			}
		}
	}

	return nil
}

func randomBytes(size int) []byte {
	buf := make([]byte, size)
	_, _ = rand.Read(buf)
	return buf
}

func mustJSON(data map[string]interface{}) []byte {
	raw, _ := json.Marshal(data)
	return raw
}

// seedBroadcasts creates sample broadcast lists
func seedBroadcasts(users []*user.User) error {
	if len(users) < 2 {
		return nil
	}

	owner := users[0]

	broadcastList := &broadcast.BroadcastList{
		ID:          uuid.New(),
		OwnerID:     owner.ID,
		Name:        "Announcements",
		Description: sql.NullString{String: "Important announcements", Valid: true},
		CreatedAt:   time.Now(),
	}

	ctx := context.Background()
	err := WithTx(ctx, DB, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
            INSERT INTO broadcast_lists (id, owner_id, name, description, created_at)
            VALUES ($1,$2,$3,$4,$5)
        `, broadcastList.ID, broadcastList.OwnerID, broadcastList.Name, broadcastList.Description, broadcastList.CreatedAt)
		if err != nil {
			return err
		}

		for _, u := range users[1:] {
			recipient := &broadcast.BroadcastRecipient{
				BroadcastID: broadcastList.ID,
				UserID:      u.ID,
				AddedAt:     time.Now(),
			}
			if _, err := tx.ExecContext(ctx, `
                INSERT INTO broadcast_recipients (broadcast_id, user_id, added_at)
                VALUES ($1,$2,$3)
            `, recipient.BroadcastID, recipient.UserID, recipient.AddedAt); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	log.Printf("Broadcast list seeded: %s", broadcastList.Name)
	return nil
}

// ClearAndReseed clears all data and runs seed again (USE WITH CAUTION)
func ClearAndReseed(cfg *SeedConfig) (*SeedResult, error) {
	log.Println("Clearing all data...")
	if err := TruncateAllTables(); err != nil {
		return nil, fmt.Errorf("failed to truncate tables: %w", err)
	}

	log.Println("Running seed...")
	return Seed(cfg)
}

// SeedDevelopment is a convenience function for development environment
func SeedDevelopment() (*SeedResult, error) {
	cfg := DefaultSeedConfig()
	cfg.CreateTestUsers = true
	cfg.TestUserCount = 8
	return Seed(cfg)
}

// SeedProduction is a convenience function for production environment (admin only)
func SeedProduction(adminEmail, adminPassword string) (*user.User, error) {
	cfg := &SeedConfig{
		AdminEmail:       adminEmail,
		AdminPassword:    adminPassword,
		AdminUsername:    "admin",
		AdminDisplayName: "System Administrator",
		CreateTestUsers:  false,
	}
	return SeedMinimal(cfg)
}
