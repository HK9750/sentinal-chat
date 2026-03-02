package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"time"

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
	AdminUser *user.User
	TestUsers []*user.User
}

// Seed runs the complete database seeding (users + devices + encryption keys only)
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
			if err := DB.QueryRow(`
                INSERT INTO devices (id, user_id, device_id, device_name, device_type, is_active, registered_at, last_seen_at)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
                ON CONFLICT (user_id, device_id) DO UPDATE SET last_seen_at = EXCLUDED.last_seen_at
                RETURNING id
            `, device.ID, device.UserID, device.DeviceID, device.DeviceName, device.DeviceType, device.IsActive, device.RegisteredAt, device.LastSeenAt).Scan(&device.ID); err != nil {
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
			identityID := uuid.New()
			pubKey := randomBytes(32)
			if _, err := DB.Exec(`
                INSERT INTO identity_keys (id, user_id, device_id, public_key, is_active, created_at)
                VALUES ($1,$2,$3,$4,$5,$6)
                ON CONFLICT (user_id, device_id) DO UPDATE SET public_key = EXCLUDED.public_key, is_active = EXCLUDED.is_active
            `, identityID, userID, device.ID, pubKey, true, time.Now()); err != nil {
				return err
			}

			signedID := uuid.New()
			if _, err := DB.Exec(`
                INSERT INTO signed_prekeys (id, user_id, device_id, key_id, public_key, signature, created_at, is_active)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
                ON CONFLICT (device_id, key_id) DO UPDATE SET public_key = EXCLUDED.public_key, signature = EXCLUDED.signature, is_active = EXCLUDED.is_active
            `, signedID, userID, device.ID, 1, randomBytes(32), randomBytes(64), time.Now(), true); err != nil {
				return err
			}

			for k := 0; k < 5; k++ {
				prekeyID := uuid.New()
				if _, err := DB.Exec(`
                    INSERT INTO onetime_prekeys (id, user_id, device_id, key_id, public_key, uploaded_at)
                    VALUES ($1,$2,$3,$4,$5,$6)
                    ON CONFLICT (device_id, key_id) DO NOTHING
                `, prekeyID, userID, device.ID, 100+k, randomBytes(32), time.Now()); err != nil {
					return err
				}
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
