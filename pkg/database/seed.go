package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"sentinal-chat/internal/domain/broadcast"
	"sentinal-chat/internal/domain/call"
	"sentinal-chat/internal/domain/conversation"
	"sentinal-chat/internal/domain/event"
	"sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/domain/user"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
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
		// Create test users
		testUsers, err := seedTestUsers(cfg.TestUserCount)
		if err != nil {
			return nil, fmt.Errorf("failed to seed test users: %w", err)
		}
		result.TestUsers = testUsers

		// Create sample conversations
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

			// Create sample messages
			msgs, err := seedMessages(convs, testUsers, deviceMap)
			if err != nil {
				return nil, fmt.Errorf("failed to seed messages: %w", err)
			}
			result.Messages = msgs
		}

		// Create sample broadcast lists
		if err := seedBroadcasts(testUsers); err != nil {
			return nil, fmt.Errorf("failed to seed broadcasts: %w", err)
		}

		// Create sample event subscriptions
		if err := seedEventSubscriptions(); err != nil {
			return nil, fmt.Errorf("failed to seed event subscriptions: %w", err)
		}

		// Create sample SFU servers
		if err := seedSFUServers(); err != nil {
			return nil, fmt.Errorf("failed to seed SFU servers: %w", err)
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
		Role:         "SUPER_ADMIN", // System-wide admin role
		Bio:          "System Administrator",
		IsActive:     true,
		IsVerified:   true,
		IsOnline:     false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		// Check if admin already exists
		var existing user.User
		if err := tx.Where("email = ?", cfg.AdminEmail).First(&existing).Error; err == nil {
			log.Println("Admin user already exists, skipping creation")
			*adminUser = existing
			return nil
		}

		// Create admin user
		if err := tx.Create(adminUser).Error; err != nil {
			return err
		}

		// Create admin settings
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
		return tx.Create(settings).Error
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

	for i := 0; i < count && i < len(testUserData); i++ {
		data := testUserData[i]

		// Check if user exists
		var existing user.User
		if DB.Where("email = ?", data.email).First(&existing).Error == nil {
			log.Printf("Test user %s already exists, skipping", data.email)
			users = append(users, &existing)
			continue
		}

		newUser := &user.User{
			ID:           uuid.New(),
			Email:        sql.NullString{String: data.email, Valid: true},
			Username:     sql.NullString{String: data.username, Valid: true},
			PhoneNumber:  sql.NullString{String: data.phone, Valid: true},
			PasswordHash: string(hashedPassword),
			DisplayName:  data.displayName,
			Role:         "USER", // Regular user role
			Bio:          data.bio,
			IsActive:     true,
			IsVerified:   true,
			IsOnline:     i%2 == 0, // Some users online
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		err := DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(newUser).Error; err != nil {
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
			return tx.Create(settings).Error
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

	// Create a DM conversation between first two users
	if len(users) >= 2 {
		dmConv := &conversation.Conversation{
			ID:               uuid.New(),
			Type:             "DM",
			DisappearingMode: "OFF",
			GroupPermissions: "{}", // Empty JSON object for JSONB field
			CreatedBy:        uuid.NullUUID{UUID: users[0].ID, Valid: true},
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		err := DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(dmConv).Error; err != nil {
				return err
			}

			// Add participants
			for _, u := range users[:2] {
				participant := &conversation.Participant{
					ConversationID:   dmConv.ID,
					UserID:           u.ID,
					Role:             "MEMBER",
					JoinedAt:         time.Now(),
					LastReadSequence: 0,
					Permissions:      "{}", // Empty JSON object for JSONB field
				}
				if err := tx.Create(participant).Error; err != nil {
					return err
				}
			}

			// Create conversation sequence
			seq := &conversation.ConversationSequence{
				ConversationID: dmConv.ID,
				LastSequence:   0,
				UpdatedAt:      time.Now(),
			}
			return tx.Create(seq).Error
		})

		if err != nil {
			return nil, err
		}
		conversations = append(conversations, dmConv)
		log.Printf("DM conversation seeded: %s", dmConv.ID)
	}

	// Create a group conversation
	if len(users) >= 3 {
		groupPerms, _ := json.Marshal(map[string]interface{}{
			"send_messages":   true,
			"send_media":      true,
			"add_members":     false,
			"edit_group_info": false,
		})

		groupConv := &conversation.Conversation{
			ID:               uuid.New(),
			Type:             "GROUP",
			Subject:          sql.NullString{String: "Sentinal Chat Team", Valid: true},
			Description:      sql.NullString{String: "Welcome to our team chat!", Valid: true},
			DisappearingMode: "OFF",
			GroupPermissions: string(groupPerms),
			CreatedBy:        uuid.NullUUID{UUID: users[0].ID, Valid: true},
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		err := DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(groupConv).Error; err != nil {
				return err
			}

			// Add participants with different roles
			roles := []string{"OWNER", "ADMIN", "MEMBER"}
			for i, u := range users {
				if i >= len(roles) {
					break
				}
				participant := &conversation.Participant{
					ConversationID:   groupConv.ID,
					UserID:           u.ID,
					Role:             roles[i],
					JoinedAt:         time.Now(),
					LastReadSequence: 0,
					Permissions:      "{}", // Empty JSON object for JSONB field
				}
				if i > 0 {
					participant.AddedBy = uuid.NullUUID{UUID: users[0].ID, Valid: true}
				}
				if err := tx.Create(participant).Error; err != nil {
					return err
				}
			}

			// Create conversation sequence
			seq := &conversation.ConversationSequence{
				ConversationID: groupConv.ID,
				LastSequence:   0,
				UpdatedAt:      time.Now(),
			}
			return tx.Create(seq).Error
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

			if err := DB.Create(msg).Error; err != nil {
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
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	DeviceID  uuid.UUID `gorm:"type:uuid;not null"`
	PublicKey []byte    `gorm:"not null"`
	IsActive  bool      `gorm:"default:true"`
	CreatedAt time.Time
}

func (identityKeySeed) TableName() string { return "identity_keys" }

type signedPreKeySeed struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	DeviceID  uuid.UUID `gorm:"type:uuid;not null"`
	KeyID     int       `gorm:"not null"`
	PublicKey []byte    `gorm:"not null"`
	Signature []byte    `gorm:"not null"`
	CreatedAt time.Time
	IsActive  bool `gorm:"default:true"`
}

func (signedPreKeySeed) TableName() string { return "signed_prekeys" }

type oneTimePreKeySeed struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID             uuid.UUID `gorm:"type:uuid;not null"`
	DeviceID           uuid.UUID `gorm:"type:uuid;not null"`
	KeyID              int       `gorm:"not null"`
	PublicKey          []byte    `gorm:"not null"`
	UploadedAt         time.Time
	ConsumedAt         sql.NullTime
	ConsumedBy         uuid.NullUUID
	ConsumedByDeviceID uuid.NullUUID `gorm:"type:uuid"`
}

func (oneTimePreKeySeed) TableName() string { return "onetime_prekeys" }

type messageCiphertextSeed struct {
	ID                uuid.UUID     `gorm:"type:uuid;primaryKey"`
	MessageID         uuid.UUID     `gorm:"type:uuid;not null"`
	RecipientUserID   uuid.UUID     `gorm:"type:uuid;not null"`
	RecipientDeviceID uuid.UUID     `gorm:"type:uuid;not null"`
	SenderDeviceID    uuid.NullUUID `gorm:"type:uuid"`
	Ciphertext        []byte        `gorm:"not null"`
	Header            string        `gorm:"type:jsonb"`
	CreatedAt         time.Time
}

func (messageCiphertextSeed) TableName() string { return "message_ciphertexts" }

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
			if err := DB.Create(&device).Error; err != nil {
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
			if err := DB.Create(&identity).Error; err != nil {
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
			if err := DB.Create(&signed).Error; err != nil {
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
				if err := DB.Create(&prekey).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func seedMessageCiphertexts(msg *message.Message, deviceMap map[uuid.UUID][]user.Device) error {
	var participants []conversation.Participant
	if err := DB.Where("conversation_id = ?", msg.ConversationID).Find(&participants).Error; err != nil {
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
			if err := DB.Create(&ciphertext).Error; err != nil {
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

	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(broadcastList).Error; err != nil {
			return err
		}

		// Add recipients
		for _, u := range users[1:] {
			recipient := &broadcast.BroadcastRecipient{
				BroadcastID: broadcastList.ID,
				UserID:      u.ID,
				AddedAt:     time.Now(),
			}
			if err := tx.Create(recipient).Error; err != nil {
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

// seedEventSubscriptions creates sample event subscriptions
func seedEventSubscriptions() error {
	subscriptions := []event.EventSubscription{
		{
			ID:             uuid.New(),
			SubscriberName: "notification-service",
			EventType:      "message.created",
			IsActive:       true,
			CreatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			SubscriberName: "notification-service",
			EventType:      "message.read",
			IsActive:       true,
			CreatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			SubscriberName: "analytics-service",
			EventType:      "user.registered",
			IsActive:       true,
			CreatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			SubscriberName: "analytics-service",
			EventType:      "call.ended",
			IsActive:       true,
			CreatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			SubscriberName: "search-service",
			EventType:      "message.created",
			IsActive:       true,
			CreatedAt:      time.Now(),
		},
	}

	for _, sub := range subscriptions {
		// Check if exists
		var existing event.EventSubscription
		if DB.Where("subscriber_name = ? AND event_type = ?", sub.SubscriberName, sub.EventType).First(&existing).Error == nil {
			continue
		}

		if err := DB.Create(&sub).Error; err != nil {
			return fmt.Errorf("failed to create subscription %s-%s: %w", sub.SubscriberName, sub.EventType, err)
		}
	}

	log.Println("Event subscriptions seeded")
	return nil
}

// seedSFUServers creates sample SFU servers for WebRTC
func seedSFUServers() error {
	servers := []call.SFUServer{
		{
			ID:            uuid.New(),
			Hostname:      "sfu-us-east-1.sentinal.chat",
			Region:        "us-east-1",
			Capacity:      1000,
			CurrentLoad:   0,
			IsHealthy:     true,
			LastHeartbeat: sql.NullTime{Time: time.Now(), Valid: true},
			CreatedAt:     time.Now(),
		},
		{
			ID:            uuid.New(),
			Hostname:      "sfu-eu-west-1.sentinal.chat",
			Region:        "eu-west-1",
			Capacity:      800,
			CurrentLoad:   0,
			IsHealthy:     true,
			LastHeartbeat: sql.NullTime{Time: time.Now(), Valid: true},
			CreatedAt:     time.Now(),
		},
		{
			ID:            uuid.New(),
			Hostname:      "sfu-ap-south-1.sentinal.chat",
			Region:        "ap-south-1",
			Capacity:      600,
			CurrentLoad:   0,
			IsHealthy:     true,
			LastHeartbeat: sql.NullTime{Time: time.Now(), Valid: true},
			CreatedAt:     time.Now(),
		},
	}

	for _, server := range servers {
		// Check if exists by hostname
		var existing call.SFUServer
		if DB.Where("hostname = ?", server.Hostname).First(&existing).Error == nil {
			continue
		}

		if err := DB.Create(&server).Error; err != nil {
			return fmt.Errorf("failed to create SFU server %s: %w", server.Hostname, err)
		}
	}

	log.Println("SFU servers seeded")
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
