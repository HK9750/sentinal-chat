package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sentinal-chat/config"
	"sentinal-chat/internal/domain/broadcast"
	"sentinal-chat/internal/domain/call"
	"sentinal-chat/internal/domain/conversation"
	"sentinal-chat/internal/domain/encryption"
	"sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/domain/upload"
	"sentinal-chat/internal/domain/user"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	LogLevel        logger.LogLevel
}

// DefaultDatabaseConfig returns sensible default database configuration
func DefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		MaxIdleConns:    10,
		MaxOpenConns:    100,
		ConnMaxLifetime: time.Hour,
		LogLevel:        logger.Info,
	}
}

// Connect establishes a connection to the PostgreSQL database
func Connect(cfg *config.Config) {
	ConnectWithOptions(cfg, DefaultDatabaseConfig())
}

// ConnectWithOptions establishes a connection with custom configuration
func ConnectWithOptions(cfg *config.Config, dbCfg *DatabaseConfig) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(dbCfg.LogLevel),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("Failed to get generic database object: %v", err)
	}

	// Connection pool settings
	sqlDB.SetMaxIdleConns(dbCfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(dbCfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(dbCfg.ConnMaxLifetime)

	log.Println("Database connection established")
}

// GetDB returns the current database instance
func GetDB() *gorm.DB {
	return DB
}

// Ping checks if the database connection is alive
func Ping() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying db: %w", err)
	}
	return sqlDB.Ping()
}

// Close closes the database connection
func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying db: %w", err)
	}
	return sqlDB.Close()
}

// ========================================
// MIGRATION FUNCTIONS
// ========================================

// ApplyRawMigrations reads .sql files from the migrations directory and executes them.
// It applies only "up" migrations in sorted order.
func ApplyRawMigrations(migrationsDir string) error {
	return ApplyRawMigrationsFiltered(migrationsDir, ".up.sql")
}

// ApplyRawMigrationsFiltered applies migrations matching the given suffix
func ApplyRawMigrationsFiltered(migrationsDir, suffix string) error {
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort files to ensure ordered execution
	var migrationFiles []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), suffix) {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}
	sort.Strings(migrationFiles)

	for _, fileName := range migrationFiles {
		path := filepath.Join(migrationsDir, fileName)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", fileName, err)
		}

		log.Printf("Applying migration: %s", fileName)
		if err := DB.Exec(string(content)).Error; err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", fileName, err)
		}
		log.Printf("Successfully applied migration: %s", fileName)
	}
	return nil
}

// RollbackMigrations applies down migrations in reverse order
func RollbackMigrations(migrationsDir string) error {
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Collect and sort down migrations in reverse order
	var migrationFiles []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".down.sql") {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(migrationFiles)))

	for _, fileName := range migrationFiles {
		path := filepath.Join(migrationsDir, fileName)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", fileName, err)
		}

		log.Printf("Rolling back migration: %s", fileName)
		if err := DB.Exec(string(content)).Error; err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", fileName, err)
		}
		log.Printf("Successfully rolled back migration: %s", fileName)
	}
	return nil
}

// MigrateDB runs GORM AutoMigrate for all domain entities
func MigrateDB(db *gorm.DB) error {
	log.Println("Starting GORM AutoMigrate...")

	entities := []interface{}{
		// User domain
		&user.User{},
		&user.UserSettings{},
		&user.Device{},
		&user.PushToken{},
		&user.UserSession{},
		&user.UserContact{},

		// Conversation domain
		&conversation.Conversation{},
		&conversation.Participant{},
		&conversation.ConversationSequence{},
		&conversation.ChatLabel{},
		&conversation.ConversationLabel{},
		&conversation.ConversationClear{},

		// Message domain
		&message.Message{},
		&message.MessageReaction{},
		&message.MessageReceipt{},
		&message.MessageMention{},
		&message.StarredMessage{},
		&message.Attachment{},
		&message.MessageAttachment{},
		&message.LinkPreview{},
		&message.Poll{},
		&message.PollOption{},
		&message.PollVote{},
		&message.MessageUserState{},

		// Call domain
		&call.Call{},
		&call.CallParticipant{},
		&call.CallQualityMetric{},

		// Encryption domain
		&encryption.IdentityKey{},
		&encryption.SignedPreKey{},
		&encryption.OneTimePreKey{},

		// Broadcast domain
		&broadcast.BroadcastList{},
		&broadcast.BroadcastRecipient{},

		// Upload domain
		&upload.UploadSession{},
	}

	if err := db.AutoMigrate(entities...); err != nil {
		return fmt.Errorf("failed to auto migrate: %w", err)
	}

	log.Println("GORM AutoMigrate completed successfully")
	return nil
}

// RunFullMigration runs raw SQL migrations (contains complete schema)
// Note: Since raw SQL migrations already define complete schema,
// GORM AutoMigrate is not needed and would cause conflicts
func RunFullMigration(migrationsDir string) error {
	// Apply raw SQL migrations (extensions, types, functions, tables)
	log.Println("Applying raw SQL migrations...")
	if err := ApplyRawMigrations(migrationsDir); err != nil {
		return fmt.Errorf("raw migrations failed: %w", err)
	}

	log.Println("Full migration completed successfully")
	return nil
}

// RunGORMOnlyMigration runs GORM AutoMigrate without raw SQL migrations
// Use this when you don't have raw SQL migrations and want GORM to create tables
func RunGORMOnlyMigration() error {
	log.Println("Running GORM AutoMigrate...")
	if err := MigrateDB(DB); err != nil {
		return fmt.Errorf("GORM migration failed: %w", err)
	}

	log.Println("GORM migration completed successfully")
	return nil
}

// ========================================
// ADMIN/USER MANAGEMENT FUNCTIONS
// ========================================

// CreateAdminUserInput holds the input for creating an admin user
type CreateAdminUserInput struct {
	Email       string
	Username    string
	Password    string
	DisplayName string
	PhoneNumber string
}

// CreateAdminUser creates an admin user with the given credentials
func CreateAdminUser(input CreateAdminUserInput) (*user.User, error) {
	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	adminUser := &user.User{
		ID:           uuid.New(),
		Email:        sql.NullString{String: input.Email, Valid: input.Email != ""},
		Username:     sql.NullString{String: input.Username, Valid: input.Username != ""},
		PhoneNumber:  sql.NullString{String: input.PhoneNumber, Valid: input.PhoneNumber != ""},
		PasswordHash: string(hashedPassword),
		DisplayName:  input.DisplayName,
		IsActive:     true,
		IsVerified:   true, // Admin is auto-verified
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Create user in transaction
	err = DB.Transaction(func(tx *gorm.DB) error {
		// Create user
		if err := tx.Create(adminUser).Error; err != nil {
			return fmt.Errorf("failed to create admin user: %w", err)
		}

		// Create default settings for admin
		settings := &user.UserSettings{
			UserID:               adminUser.ID,
			PrivacyLastSeen:      "CONTACTS",
			PrivacyProfilePhoto:  "CONTACTS",
			PrivacyAbout:         "CONTACTS",
			PrivacyGroups:        "CONTACTS",
			ReadReceipts:         true,
			NotificationsEnabled: true,
			Theme:                "SYSTEM",
			Language:             "en",
			UpdatedAt:            time.Now(),
		}
		if err := tx.Create(settings).Error; err != nil {
			return fmt.Errorf("failed to create admin settings: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	log.Printf("Admin user created successfully: %s", input.Email)
	return adminUser, nil
}

// GetOrCreateAdminUser gets an existing admin user or creates one if it doesn't exist
func GetOrCreateAdminUser(input CreateAdminUserInput) (*user.User, bool, error) {
	var existingUser user.User

	// Check if user exists by email or username
	query := DB.Where("email = ?", input.Email)
	if input.Username != "" {
		query = query.Or("username = ?", input.Username)
	}

	err := query.First(&existingUser).Error
	if err == nil {
		// User exists
		return &existingUser, false, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, false, fmt.Errorf("failed to query user: %w", err)
	}

	// User doesn't exist, create new one
	newUser, err := CreateAdminUser(input)
	return newUser, true, err
}

// CreateUserWithDefaults creates a regular user with default settings
func CreateUserWithDefaults(input CreateAdminUserInput) (*user.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	newUser := &user.User{
		ID:           uuid.New(),
		Email:        sql.NullString{String: input.Email, Valid: input.Email != ""},
		Username:     sql.NullString{String: input.Username, Valid: input.Username != ""},
		PhoneNumber:  sql.NullString{String: input.PhoneNumber, Valid: input.PhoneNumber != ""},
		PasswordHash: string(hashedPassword),
		DisplayName:  input.DisplayName,
		IsActive:     true,
		IsVerified:   false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(newUser).Error; err != nil {
			return fmt.Errorf("failed to create user: %w", err)
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
		if err := tx.Create(settings).Error; err != nil {
			return fmt.Errorf("failed to create user settings: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return newUser, nil
}

// ========================================
// UTILITY FUNCTIONS
// ========================================

// TruncateAllTables truncates all tables (USE WITH CAUTION - for testing only)
func TruncateAllTables() error {
	tables := []string{
		"key_bundles",
		"upload_sessions",
		"conversation_clears",
		"message_user_states",
		"encrypted_sessions",
		"onetime_prekeys",
		"signed_prekeys",
		"identity_keys",
		"call_quality_metrics",
		"call_participants",
		"calls",
		"conversation_labels",
		"chat_labels",
		"broadcast_recipients",
		"broadcast_lists",
		"poll_votes",
		"poll_options",
		"polls",
		"message_attachments",
		"attachments",
		"link_previews",
		"starred_messages",
		"message_mentions",
		"message_receipts",
		"message_reactions",
		"messages",
		"conversation_sequences",
		"participants",
		"conversations",
		"user_contacts",
		"user_sessions",
		"push_tokens",
		"devices",
		"user_settings",
		"users",
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		// Disable foreign key checks temporarily
		if err := tx.Exec("SET session_replication_role = 'replica';").Error; err != nil {
			return err
		}

		for _, table := range tables {
			if err := tx.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE;", table)).Error; err != nil {
				log.Printf("Warning: failed to truncate table %s: %v", table, err)
			}
		}

		// Re-enable foreign key checks
		return tx.Exec("SET session_replication_role = 'origin';").Error
	})
}

// DropAllTables drops all tables (USE WITH EXTREME CAUTION)
func DropAllTables() error {
	return DB.Exec(`
		DO $$ DECLARE
			r RECORD;
		BEGIN
			FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = current_schema()) LOOP
				EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
			END LOOP;
		END $$;
	`).Error
}

// TableExists checks if a table exists in the database
func TableExists(tableName string) (bool, error) {
	var exists bool
	err := DB.Raw(`
		SELECT EXISTS (
			SELECT FROM pg_tables
			WHERE schemaname = 'public'
			AND tablename = ?
		);
	`, tableName).Scan(&exists).Error
	return exists, err
}

// GetTableCount returns the number of rows in a table
func GetTableCount(tableName string) (int64, error) {
	var count int64
	err := DB.Table(tableName).Count(&count).Error
	return count, err
}

// GenerateSecureToken generates a cryptographically secure random token
func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash compares a password with a hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// HealthCheck performs a comprehensive database health check
func HealthCheck() error {
	// Check connection
	if err := Ping(); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// Check if core tables exist
	coreTables := []string{"users", "conversations", "messages"}
	for _, table := range coreTables {
		exists, err := TableExists(table)
		if err != nil {
			return fmt.Errorf("failed to check table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("core table %s does not exist", table)
		}
	}

	return nil
}
