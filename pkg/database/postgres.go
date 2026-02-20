package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"sentinal-chat/config"
	"sentinal-chat/internal/domain/user"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

// Singleton instance variables
var (
	DB     *sql.DB
	dbOnce sync.Once
	dbCfg  *DatabaseConfig
)

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

// DefaultDatabaseConfig returns sensible default database configuration
func DefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		MaxIdleConns:    10,
		MaxOpenConns:    100,
		ConnMaxLifetime: time.Hour,
	}
}

// GetInstance returns the singleton database instance.
// Panics if Connect() has not been called.
func GetInstance() *sql.DB {
	if DB == nil {
		panic("database not initialized. Call Connect() first")
	}
	return DB
}

// IsInitialized returns true if the database has been initialized
func IsInitialized() bool {
	return DB != nil
}

// Connect establishes a connection to the PostgreSQL database.
// This function is safe to call multiple times - only the first call will create the connection.
func Connect(cfg *config.Config) {
	ConnectWithOptions(cfg, DefaultDatabaseConfig())
}

// ConnectWithOptions establishes a connection with custom configuration.
// This function is safe to call multiple times - only the first call will create the connection.
func ConnectWithOptions(cfg *config.Config, config *DatabaseConfig) {
	dbOnce.Do(func() {
		dbCfg = config
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
			cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort)

		var err error
		DB, err = sql.Open("pgx", dsn)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}

		DB.SetMaxIdleConns(config.MaxIdleConns)
		DB.SetMaxOpenConns(config.MaxOpenConns)
		DB.SetConnMaxLifetime(config.ConnMaxLifetime)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := DB.PingContext(ctx); err != nil {
			log.Fatalf("Failed to ping database: %v", err)
		}

		log.Println("Database connection established (singleton)")
	})
}

// GetDB returns the current database instance.
// Kept for backward compatibility - use GetInstance() for new code.
func GetDB() *sql.DB {
	return GetInstance()
}

// WithTx executes fn within a database transaction.
func WithTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	if db == nil {
		return errors.New("database not initialized")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("tx error: %v (rollback error: %w)", err, rollbackErr)
		}
		return err
	}

	return tx.Commit()
}

// Ping checks if the database connection is alive
func Ping() error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	return DB.Ping()
}

// Close closes the database connection
func Close() error {
	if DB == nil {
		return nil
	}
	return DB.Close()
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
	if DB == nil {
		return errors.New("database not initialized")
	}
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
		if _, err := DB.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", fileName, err)
		}
		log.Printf("Successfully applied migration: %s", fileName)
	}
	return nil
}

// RollbackMigrations applies down migrations in reverse order
func RollbackMigrations(migrationsDir string) error {
	if DB == nil {
		return errors.New("database not initialized")
	}
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
		if _, err := DB.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", fileName, err)
		}
		log.Printf("Successfully rolled back migration: %s", fileName)
	}
	return nil
}

// RunFullMigration runs raw SQL migrations (contains complete schema)
func RunFullMigration(migrationsDir string) error {
	log.Println("Applying raw SQL migrations...")
	if err := ApplyRawMigrations(migrationsDir); err != nil {
		return fmt.Errorf("raw migrations failed: %w", err)
	}

	log.Println("Full migration completed successfully")
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
		IsVerified:   true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	ctx := context.Background()
	if err := WithTx(ctx, DB, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
            INSERT INTO users (id, email, username, phone_number, password_hash, display_name, is_active, is_verified, created_at, updated_at)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        `,
			adminUser.ID,
			adminUser.Email,
			adminUser.Username,
			adminUser.PhoneNumber,
			adminUser.PasswordHash,
			adminUser.DisplayName,
			adminUser.IsActive,
			adminUser.IsVerified,
			adminUser.CreatedAt,
			adminUser.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to create admin user: %w", err)
		}

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

		_, err = tx.ExecContext(ctx, `
            INSERT INTO user_settings (
                user_id, privacy_last_seen, privacy_profile_photo, privacy_about, privacy_groups,
                read_receipts, notifications_enabled, theme, language, updated_at
            ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        `,
			settings.UserID,
			settings.PrivacyLastSeen,
			settings.PrivacyProfilePhoto,
			settings.PrivacyAbout,
			settings.PrivacyGroups,
			settings.ReadReceipts,
			settings.NotificationsEnabled,
			settings.Theme,
			settings.Language,
			settings.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to create admin settings: %w", err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	log.Printf("Admin user created successfully: %s", input.Email)
	return adminUser, nil
}

// GetOrCreateAdminUser gets an existing admin user or creates one if it doesn't exist
func GetOrCreateAdminUser(input CreateAdminUserInput) (*user.User, bool, error) {
	ctx := context.Background()
	var existing user.User
	var err error

	if input.Username != "" {
		err = DB.QueryRowContext(ctx, `
            SELECT id, phone_number, username, email, password_hash, display_name, role, bio, avatar_url,
                   is_online, last_seen_at, is_active, is_verified, created_at, updated_at
            FROM users WHERE email = $1 OR username = $2 LIMIT 1
        `, input.Email, input.Username).Scan(
			&existing.ID,
			&existing.PhoneNumber,
			&existing.Username,
			&existing.Email,
			&existing.PasswordHash,
			&existing.DisplayName,
			&existing.Role,
			&existing.Bio,
			&existing.AvatarURL,
			&existing.IsOnline,
			&existing.LastSeenAt,
			&existing.IsActive,
			&existing.IsVerified,
			&existing.CreatedAt,
			&existing.UpdatedAt,
		)
	} else {
		err = DB.QueryRowContext(ctx, `
            SELECT id, phone_number, username, email, password_hash, display_name, role, bio, avatar_url,
                   is_online, last_seen_at, is_active, is_verified, created_at, updated_at
            FROM users WHERE email = $1 LIMIT 1
        `, input.Email).Scan(
			&existing.ID,
			&existing.PhoneNumber,
			&existing.Username,
			&existing.Email,
			&existing.PasswordHash,
			&existing.DisplayName,
			&existing.Role,
			&existing.Bio,
			&existing.AvatarURL,
			&existing.IsOnline,
			&existing.LastSeenAt,
			&existing.IsActive,
			&existing.IsVerified,
			&existing.CreatedAt,
			&existing.UpdatedAt,
		)
	}

	if err == nil {
		return &existing, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("failed to query user: %w", err)
	}

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

	ctx := context.Background()
	if err := WithTx(ctx, DB, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
            INSERT INTO users (id, email, username, phone_number, password_hash, display_name, is_active, is_verified, created_at, updated_at)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        `,
			newUser.ID,
			newUser.Email,
			newUser.Username,
			newUser.PhoneNumber,
			newUser.PasswordHash,
			newUser.DisplayName,
			newUser.IsActive,
			newUser.IsVerified,
			newUser.CreatedAt,
			newUser.UpdatedAt,
		)
		if err != nil {
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

		_, err = tx.ExecContext(ctx, `
            INSERT INTO user_settings (
                user_id, privacy_last_seen, privacy_profile_photo, privacy_about, privacy_groups,
                read_receipts, notifications_enabled, theme, language, updated_at
            ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        `,
			settings.UserID,
			settings.PrivacyLastSeen,
			settings.PrivacyProfilePhoto,
			settings.PrivacyAbout,
			settings.PrivacyGroups,
			settings.ReadReceipts,
			settings.NotificationsEnabled,
			settings.Theme,
			settings.Language,
			settings.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to create user settings: %w", err)
		}
		return nil
	}); err != nil {
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
		"message_ciphertexts",
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
		"outbox_events",
		"command_logs",
		"scheduled_messages",
		"message_versions",
	}

	ctx := context.Background()
	return WithTx(ctx, DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "SET session_replication_role = 'replica';"); err != nil {
			return err
		}

		for _, table := range tables {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE;", table)); err != nil {
				log.Printf("Warning: failed to truncate table %s: %v", table, err)
			}
		}

		_, err := tx.ExecContext(ctx, "SET session_replication_role = 'origin';")
		return err
	})
}

// DropAllTables drops all tables (USE WITH EXTREME CAUTION)
func DropAllTables() error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	_, err := DB.Exec(`
        DO $$ DECLARE
            r RECORD;
        BEGIN
            FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = current_schema()) LOOP
                EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
            END LOOP;
        END $$;
    `)
	return err
}

// TableExists checks if a table exists in the database
func TableExists(tableName string) (bool, error) {
	if DB == nil {
		return false, errors.New("database not initialized")
	}
	var exists bool
	err := DB.QueryRow(`
        SELECT EXISTS (
            SELECT FROM pg_tables
            WHERE schemaname = 'public'
            AND tablename = $1
        );
    `, tableName).Scan(&exists)
	return exists, err
}

var tableNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// GetTableCount returns the number of rows in a table
func GetTableCount(tableName string) (int64, error) {
	if DB == nil {
		return 0, errors.New("database not initialized")
	}
	if !tableNamePattern.MatchString(tableName) {
		return 0, fmt.Errorf("invalid table name: %s", tableName)
	}
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	err := DB.QueryRow(query).Scan(&count)
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
	if err := Ping(); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

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
