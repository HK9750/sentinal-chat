package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"sentinal-chat/internal/domain/user"

	"github.com/google/uuid"
)

const defaultSeedPassword = "Test@123!"

type SeedConfig struct {
	AdminEmail       string
	AdminPassword    string
	AdminUsername    string
	AdminDisplayName string
	CreateTestUsers  bool
	TestUserCount    int
	Now              time.Time
}

type SeedResult struct {
	AdminUser       *user.User
	TestUsers       []*user.User
	CreatedUsers    int
	ExistingUsers   int
	CreatedDevices  int
	UpdatedDevices  int
	CreatedContacts int
	CreatedDMs      int
	CreatedOAuthIDs int
}

type seedUserSpec struct {
	Email       string
	Username    string
	DisplayName string
	Bio         string
	Phone       string
	AvatarURL   string
	DeviceName  string
	DeviceType  string
	DeviceID    string
	Verified    bool
	Provider    string
	ProviderID  string
}

func DefaultSeedConfig() *SeedConfig {
	return &SeedConfig{
		AdminEmail:       "admin@sentinal.chat",
		AdminPassword:    "Admin@123!",
		AdminUsername:    "admin",
		AdminDisplayName: "System Admin",
		CreateTestUsers:  true,
		TestUserCount:    6,
	}
}

func Seed(cfg *SeedConfig) (*SeedResult, error) {
	if cfg == nil {
		cfg = DefaultSeedConfig()
	}
	if err := validateSeedConfig(cfg); err != nil {
		return nil, err
	}

	now := seedNow(cfg)
	ctx := context.Background()
	result := &SeedResult{}

	log.Println("Starting database seed...")

	adminUser, adminCreated, err := upsertSeedUser(ctx, seedUserSpec{
		Email:       cfg.AdminEmail,
		Username:    cfg.AdminUsername,
		DisplayName: cfg.AdminDisplayName,
		Bio:         "System administrator account",
		Phone:       "+10000000000",
		AvatarURL:   "https://images.unsplash.com/photo-1500648767791-00dcc994a43e?auto=format&fit=crop&w=400&q=80",
		DeviceName:  "Admin Console",
		DeviceType:  "web",
		DeviceID:    "seed-admin-web",
		Verified:    true,
	}, cfg.AdminPassword, now)
	if err != nil {
		return nil, fmt.Errorf("seed admin user: %w", err)
	}
	result.AdminUser = adminUser
	result.CreatedUsers += boolToInt(adminCreated)
	result.ExistingUsers += boolToInt(!adminCreated)

	deviceCreated, err := ensureDevice(ctx, adminUser.ID, "seed-admin-web", "Admin Console", "web", now)
	if err != nil {
		return nil, fmt.Errorf("seed admin device: %w", err)
	}
	result.CreatedDevices += boolToInt(deviceCreated)
	result.UpdatedDevices += boolToInt(!deviceCreated)

	if !cfg.CreateTestUsers || cfg.TestUserCount <= 0 {
		log.Printf("Database seed complete. created_users=%d existing_users=%d", result.CreatedUsers, result.ExistingUsers)
		return result, nil
	}

	passwordHash, err := HashPassword(defaultSeedPassword)
	if err != nil {
		return nil, fmt.Errorf("hash seed password: %w", err)
	}

	specs := defaultSeedUsers()
	if cfg.TestUserCount < len(specs) {
		specs = specs[:cfg.TestUserCount]
	}

	testUsers := make([]*user.User, 0, len(specs))
	for idx, spec := range specs {
		seedTime := now.Add(time.Duration(idx+1) * time.Minute)
		seededUser, created, userErr := upsertSeedUserWithHash(ctx, spec, passwordHash, seedTime)
		if userErr != nil {
			return nil, fmt.Errorf("seed test user %s: %w", spec.Email, userErr)
		}
		testUsers = append(testUsers, seededUser)
		result.CreatedUsers += boolToInt(created)
		result.ExistingUsers += boolToInt(!created)

		deviceCreated, deviceErr := ensureDevice(ctx, seededUser.ID, spec.DeviceID, spec.DeviceName, spec.DeviceType, seedTime)
		if deviceErr != nil {
			return nil, fmt.Errorf("seed device for %s: %w", spec.Email, deviceErr)
		}
		result.CreatedDevices += boolToInt(deviceCreated)
		result.UpdatedDevices += boolToInt(!deviceCreated)

		if spec.Provider != "" && spec.ProviderID != "" {
			createdOAuth, oauthErr := ensureOAuthIdentity(ctx, seededUser.ID, spec.Provider, spec.ProviderID, spec.Email, spec.Verified, seedTime)
			if oauthErr != nil {
				return nil, fmt.Errorf("seed oauth identity for %s: %w", spec.Email, oauthErr)
			}
			result.CreatedOAuthIDs += boolToInt(createdOAuth)
		}
	}
	result.TestUsers = testUsers

	contactsCreated, err := seedContacts(ctx, append([]*user.User{adminUser}, testUsers...), now)
	if err != nil {
		return nil, fmt.Errorf("seed contacts: %w", err)
	}
	result.CreatedContacts = contactsCreated

	dmsCreated, err := seedDirectConversations(ctx, append([]*user.User{adminUser}, testUsers...), now)
	if err != nil {
		return nil, fmt.Errorf("seed direct conversations: %w", err)
	}
	result.CreatedDMs = dmsCreated

	log.Printf(
		"Database seed complete. created_users=%d existing_users=%d created_devices=%d updated_devices=%d created_contacts=%d created_dms=%d created_oauth_identities=%d",
		result.CreatedUsers,
		result.ExistingUsers,
		result.CreatedDevices,
		result.UpdatedDevices,
		result.CreatedContacts,
		result.CreatedDMs,
		result.CreatedOAuthIDs,
	)

	return result, nil
}

func SeedMinimal(cfg *SeedConfig) (*user.User, error) {
	if cfg == nil {
		cfg = DefaultSeedConfig()
	}
	if err := validateSeedConfig(cfg); err != nil {
		return nil, err
	}

	seededUser, _, err := upsertSeedUser(context.Background(), seedUserSpec{
		Email:       cfg.AdminEmail,
		Username:    cfg.AdminUsername,
		DisplayName: cfg.AdminDisplayName,
		Bio:         "System administrator account",
		Phone:       "+10000000000",
		AvatarURL:   "https://images.unsplash.com/photo-1500648767791-00dcc994a43e?auto=format&fit=crop&w=400&q=80",
		DeviceName:  "Admin Console",
		DeviceType:  "web",
		DeviceID:    "seed-admin-web",
		Verified:    true,
	}, cfg.AdminPassword, seedNow(cfg))
	if err != nil {
		return nil, err
	}
	return seededUser, nil
}

func ClearAndReseed(cfg *SeedConfig) (*SeedResult, error) {
	log.Println("Clearing all data before reseed...")
	if err := TruncateAllTables(); err != nil {
		return nil, fmt.Errorf("truncate tables: %w", err)
	}
	return Seed(cfg)
}

func SeedDevelopment() (*SeedResult, error) {
	cfg := DefaultSeedConfig()
	cfg.CreateTestUsers = true
	cfg.TestUserCount = len(defaultSeedUsers())
	return Seed(cfg)
}

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

func validateSeedConfig(cfg *SeedConfig) error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	if strings.TrimSpace(cfg.AdminEmail) == "" || strings.TrimSpace(cfg.AdminUsername) == "" || strings.TrimSpace(cfg.AdminDisplayName) == "" {
		return errors.New("admin seed config is incomplete")
	}
	if len(strings.TrimSpace(cfg.AdminPassword)) < 8 {
		return errors.New("admin password must be at least 8 characters")
	}
	if cfg.TestUserCount < 0 {
		return errors.New("test user count cannot be negative")
	}
	return nil
}

func seedNow(cfg *SeedConfig) time.Time {
	if cfg != nil && !cfg.Now.IsZero() {
		return cfg.Now.UTC()
	}
	return time.Now().UTC()
}

func upsertSeedUser(ctx context.Context, spec seedUserSpec, password string, now time.Time) (*user.User, bool, error) {
	passwordHash, err := HashPassword(strings.TrimSpace(password))
	if err != nil {
		return nil, false, err
	}
	return upsertSeedUserWithHash(ctx, spec, passwordHash, now)
}

func upsertSeedUserWithHash(ctx context.Context, spec seedUserSpec, passwordHash string, now time.Time) (*user.User, bool, error) {
	spec = normalizeSeedUserSpec(spec)

	seededUser := &user.User{
		ID:           uuid.New(),
		Email:        nullableString(spec.Email),
		Username:     nullableString(spec.Username),
		PhoneNumber:  nullableString(spec.Phone),
		PasswordHash: passwordHash,
		DisplayName:  spec.DisplayName,
		Bio:          spec.Bio,
		AvatarURL:    spec.AvatarURL,
		IsActive:     true,
		IsVerified:   spec.Verified,
		IsOnline:     false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	var created bool
	err := WithTx(ctx, DB, func(tx *sql.Tx) error {
		var existing user.User
		err := tx.QueryRowContext(ctx, `
			SELECT id, phone_number, username, email, password_hash, display_name, bio, avatar_url,
			       is_online, last_seen_at, is_active, is_verified, created_at, updated_at
			FROM users
			WHERE email = $1
		`, spec.Email).Scan(
			&existing.ID,
			&existing.PhoneNumber,
			&existing.Username,
			&existing.Email,
			&existing.PasswordHash,
			&existing.DisplayName,
			&existing.Bio,
			&existing.AvatarURL,
			&existing.IsOnline,
			&existing.LastSeenAt,
			&existing.IsActive,
			&existing.IsVerified,
			&existing.CreatedAt,
			&existing.UpdatedAt,
		)
		switch {
		case err == nil:
			seededUser.ID = existing.ID
			seededUser.CreatedAt = existing.CreatedAt
			_, err = tx.ExecContext(ctx, `
				UPDATE users
				SET phone_number = $1,
				    username = $2,
				    password_hash = $3,
				    display_name = $4,
				    bio = $5,
				    avatar_url = $6,
				    is_active = TRUE,
				    is_verified = $7,
				    updated_at = $8
				WHERE id = $9
			`, seededUser.PhoneNumber, seededUser.Username, seededUser.PasswordHash, seededUser.DisplayName, seededUser.Bio, seededUser.AvatarURL, seededUser.IsVerified, now, existing.ID)
			return err
		case errors.Is(err, sql.ErrNoRows):
			created = true
			_, err = tx.ExecContext(ctx, `
				INSERT INTO users (id, phone_number, username, email, password_hash, display_name, bio, avatar_url, is_online, last_seen_at, is_active, is_verified, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
			`, seededUser.ID, seededUser.PhoneNumber, seededUser.Username, seededUser.Email, seededUser.PasswordHash, seededUser.DisplayName, seededUser.Bio, seededUser.AvatarURL, seededUser.IsOnline, seededUser.LastSeenAt, seededUser.IsActive, seededUser.IsVerified, seededUser.CreatedAt, seededUser.UpdatedAt)
			return err
		default:
			return err
		}
	})
	if err != nil {
		return nil, false, err
	}

	return seededUser, created, nil
}

func ensureDevice(ctx context.Context, userID uuid.UUID, deviceID, deviceName, deviceType string, now time.Time) (bool, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return false, errors.New("device id is required")
	}

	var existingID uuid.UUID
	err := DB.QueryRowContext(ctx, `SELECT id FROM devices WHERE user_id = $1 AND device_id = $2`, userID, deviceID).Scan(&existingID)
	switch {
	case err == nil:
		_, err = DB.ExecContext(ctx, `
			UPDATE devices
			SET device_name = $1, device_type = $2, is_active = TRUE, last_seen_at = $3
			WHERE id = $4
		`, strings.TrimSpace(deviceName), strings.TrimSpace(deviceType), now, existingID)
		return false, err
	case errors.Is(err, sql.ErrNoRows):
		_, err = DB.ExecContext(ctx, `
			INSERT INTO devices (id, user_id, device_id, device_name, device_type, is_active, registered_at, last_seen_at)
			VALUES ($1,$2,$3,$4,$5,TRUE,$6,$7)
		`, uuid.New(), userID, deviceID, strings.TrimSpace(deviceName), strings.TrimSpace(deviceType), now, now)
		return true, err
	default:
		return false, err
	}
}

func ensureOAuthIdentity(ctx context.Context, userID uuid.UUID, provider, providerUserID, email string, verified bool, now time.Time) (bool, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	providerUserID = strings.TrimSpace(providerUserID)
	if provider == "" || providerUserID == "" {
		return false, errors.New("oauth identity is incomplete")
	}

	var existingID uuid.UUID
	err := DB.QueryRowContext(ctx, `SELECT id FROM oauth_identities WHERE provider = $1 AND provider_user_id = $2`, provider, providerUserID).Scan(&existingID)
	switch {
	case err == nil:
		_, err = DB.ExecContext(ctx, `
			UPDATE oauth_identities
			SET user_id = $1, provider_email = $2, email_verified = $3, updated_at = $4
			WHERE id = $5
		`, userID, nullableString(strings.ToLower(email)), verified, now, existingID)
		return false, err
	case errors.Is(err, sql.ErrNoRows):
		_, err = DB.ExecContext(ctx, `
			INSERT INTO oauth_identities (id, user_id, provider, provider_user_id, provider_email, email_verified, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		`, uuid.New(), userID, provider, providerUserID, nullableString(strings.ToLower(email)), verified, now, now)
		return true, err
	default:
		return false, err
	}
}

func seedContacts(ctx context.Context, users []*user.User, now time.Time) (int, error) {
	created := 0
	for i := 0; i < len(users)-1; i++ {
		pairs := [][2]*user.User{{users[i], users[i+1]}, {users[i+1], users[i]}}
		for _, pair := range pairs {
			result, err := DB.ExecContext(ctx, `
				INSERT INTO user_contacts (user_id, contact_user_id, nickname, is_blocked, created_at)
				VALUES ($1,$2,$3,FALSE,$4)
				ON CONFLICT (user_id, contact_user_id) DO NOTHING
			`, pair[0].ID, pair[1].ID, pair[1].DisplayName, now)
			if err != nil {
				return created, err
			}
			rows, _ := result.RowsAffected()
			created += int(rows)
		}
	}
	return created, nil
}

func seedDirectConversations(ctx context.Context, users []*user.User, now time.Time) (int, error) {
	if len(users) < 2 {
		return 0, nil
	}

	created := 0
	for i := 0; i < len(users)-1; i++ {
		a := users[i]
		b := users[i+1]
		left, right := sortedUUIDPair(a.ID, b.ID)

		var conversationID uuid.UUID
		err := DB.QueryRowContext(ctx, `
			SELECT id FROM conversations
			WHERE type = 'DM' AND dm_user_id_a = $1 AND dm_user_id_b = $2
		`, left, right).Scan(&conversationID)
		switch {
		case err == nil:
			continue
		case !errors.Is(err, sql.ErrNoRows):
			return created, err
		}

		conversationID = uuid.New()
		err = WithTx(ctx, DB, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO conversations (id, type, subject, description, avatar_url, invite_link, invite_link_revoked_at, dm_user_id_a, dm_user_id_b, disappearing_mode, created_by, created_at, updated_at)
				VALUES ($1,'DM',NULL,NULL,NULL,NULL,NULL,$2,$3,'OFF',$4,$5,$6)
			`, conversationID, left, right, a.ID, now, now); err != nil {
				return err
			}

			participants := []uuid.UUID{a.ID, b.ID}
			sort.Slice(participants, func(i, j int) bool { return participants[i].String() < participants[j].String() })
			for _, participantID := range participants {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO participants (conversation_id, user_id, role, joined_at, added_by, archived, last_read_sequence)
					VALUES ($1,$2,'MEMBER',$3,$4,FALSE,0)
				`, conversationID, participantID, now, a.ID); err != nil {
					return err
				}
			}

			if _, err := tx.ExecContext(ctx, `
				INSERT INTO conversation_sequences (conversation_id, last_sequence, updated_at)
				VALUES ($1,0,$2)
			`, conversationID, now); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return created, err
		}

		created++
	}

	return created, nil
}

func defaultSeedUsers() []seedUserSpec {
	return []seedUserSpec{
		{Email: "alice@test.com", Username: "alice", DisplayName: "Alice Johnson", Bio: "Coffee enthusiast and mobile engineer", Phone: "+1234567001", AvatarURL: "https://images.unsplash.com/photo-1494790108377-be9c29b29330?auto=format&fit=crop&w=400&q=80", DeviceName: "Alice iPhone", DeviceType: "ios", DeviceID: "seed-alice-ios", Verified: true, Provider: "google", ProviderID: "google-alice-001"},
		{Email: "bob@test.com", Username: "bob", DisplayName: "Bob Smith", Bio: "Backend engineer who lives in terminals", Phone: "+1234567002", AvatarURL: "https://images.unsplash.com/photo-1500648767791-00dcc994a43e?auto=format&fit=crop&w=400&q=80", DeviceName: "Bob Pixel", DeviceType: "android", DeviceID: "seed-bob-android", Verified: true, Provider: "github", ProviderID: "github-bob-001"},
		{Email: "charlie@test.com", Username: "charlie", DisplayName: "Charlie Brown", Bio: "Curious product tinkerer", Phone: "+1234567003", AvatarURL: "https://images.unsplash.com/photo-1506794778202-cad84cf45f1d?auto=format&fit=crop&w=400&q=80", DeviceName: "Charlie MacBook", DeviceType: "desktop", DeviceID: "seed-charlie-desktop", Verified: true},
		{Email: "diana@test.com", Username: "diana", DisplayName: "Diana Prince", Bio: "Engineering lead and mentor", Phone: "+1234567004", AvatarURL: "https://images.unsplash.com/photo-1488426862026-3ee34a7d66df?auto=format&fit=crop&w=400&q=80", DeviceName: "Diana Surface", DeviceType: "web", DeviceID: "seed-diana-web", Verified: true, Provider: "google", ProviderID: "google-diana-001"},
		{Email: "edward@test.com", Username: "edward", DisplayName: "Edward Chen", Bio: "Full-stack developer shipping side quests", Phone: "+1234567005", AvatarURL: "https://images.unsplash.com/photo-1504593811423-6dd665756598?auto=format&fit=crop&w=400&q=80", DeviceName: "Edward ThinkPad", DeviceType: "desktop", DeviceID: "seed-edward-desktop", Verified: true},
		{Email: "fiona@test.com", Username: "fiona", DisplayName: "Fiona Green", Bio: "UX designer who prototypes everything", Phone: "+1234567006", AvatarURL: "https://images.unsplash.com/photo-1544005313-94ddf0286df2?auto=format&fit=crop&w=400&q=80", DeviceName: "Fiona iPad", DeviceType: "ios", DeviceID: "seed-fiona-ios", Verified: true, Provider: "github", ProviderID: "github-fiona-001"},
	}
}

func normalizeSeedUserSpec(spec seedUserSpec) seedUserSpec {
	spec.Email = strings.ToLower(strings.TrimSpace(spec.Email))
	spec.Username = strings.ToLower(strings.TrimSpace(spec.Username))
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	spec.Bio = strings.TrimSpace(spec.Bio)
	spec.Phone = strings.TrimSpace(spec.Phone)
	spec.AvatarURL = strings.TrimSpace(spec.AvatarURL)
	spec.DeviceName = strings.TrimSpace(spec.DeviceName)
	spec.DeviceType = strings.ToLower(strings.TrimSpace(spec.DeviceType))
	spec.DeviceID = strings.TrimSpace(spec.DeviceID)
	spec.Provider = strings.ToLower(strings.TrimSpace(spec.Provider))
	spec.ProviderID = strings.TrimSpace(spec.ProviderID)
	return spec
}

func nullableString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func sortedUUIDPair(a, b uuid.UUID) (uuid.UUID, uuid.UUID) {
	if a.String() <= b.String() {
		return a, b
	}
	return b, a
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
