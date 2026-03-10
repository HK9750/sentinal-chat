package database

import (
	"context"
	"database/sql"
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

	_ "github.com/jackc/pgx/v5/stdlib"
)

// ========================================
// CONNECTION (Singleton)
// ========================================

// DB is the singleton database instance
var (
	DB     *sql.DB
	dbOnce sync.Once
	dbCfg  *DatabaseConfig
)

// DatabaseConfig holds database connection pool configuration
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

// Connect establishes a connection to the PostgreSQL database.
// Safe to call multiple times — only the first call creates the connection.
func Connect(cfg *config.Config) {
	ConnectWithOptions(cfg, DefaultDatabaseConfig())
}

// ConnectWithOptions establishes a connection with custom pool configuration.
// Safe to call multiple times — only the first call creates the connection.
func ConnectWithOptions(cfg *config.Config, dbConfig *DatabaseConfig) {
	dbOnce.Do(func() {
		dbCfg = dbConfig
		dsn := fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
			cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort,
		)

		var err error
		DB, err = sql.Open("pgx", dsn)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}

		DB.SetMaxIdleConns(dbConfig.MaxIdleConns)
		DB.SetMaxOpenConns(dbConfig.MaxOpenConns)
		DB.SetConnMaxLifetime(dbConfig.ConnMaxLifetime)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := DB.PingContext(ctx); err != nil {
			log.Fatalf("Failed to ping database: %v", err)
		}

		log.Println("Database connection established (singleton)")
	})
}

// GetInstance returns the singleton database instance.
// Panics if Connect() has not been called.
func GetInstance() *sql.DB {
	if DB == nil {
		panic("database not initialized. Call Connect() first")
	}
	return DB
}

// GetDB returns the singleton database instance (alias for GetInstance).
func GetDB() *sql.DB {
	return GetInstance()
}

// IsInitialized returns true if the database has been initialized
func IsInitialized() bool {
	return DB != nil
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
// TRANSACTIONS
// ========================================

// WithTx executes fn within a database transaction.
// Automatically rolls back on error and commits on success.
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

// ========================================
// MIGRATIONS
// ========================================

// RunFullMigration runs all .up.sql migrations from the given directory
func RunFullMigration(migrationsDir string) error {
	log.Println("Applying raw SQL migrations...")
	if err := ApplyRawMigrations(migrationsDir); err != nil {
		return fmt.Errorf("raw migrations failed: %w", err)
	}
	log.Println("Full migration completed successfully")
	return nil
}

// ApplyRawMigrations reads and executes all .up.sql files in sorted order
func ApplyRawMigrations(migrationsDir string) error {
	return applyMigrationsFiltered(migrationsDir, ".up.sql")
}

// RollbackMigrations applies .down.sql migrations in reverse order
func RollbackMigrations(migrationsDir string) error {
	if DB == nil {
		return errors.New("database not initialized")
	}

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

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

// applyMigrationsFiltered applies migrations matching the given suffix
func applyMigrationsFiltered(migrationsDir, suffix string) error {
	if DB == nil {
		return errors.New("database not initialized")
	}

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

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

// ========================================
// TABLE UTILITIES
// ========================================

// TableExists checks if a table exists in the database
func TableExists(tableName string) (bool, error) {
	if DB == nil {
		return false, errors.New("database not initialized")
	}
	var exists bool
	err := DB.QueryRow(`
		SELECT EXISTS (
			SELECT FROM pg_tables
			WHERE schemaname = 'public' AND tablename = $1
		);
	`, tableName).Scan(&exists)
	return exists, err
}

var tableNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// GetTableCount returns the number of rows in a table.
// Table name is validated to prevent SQL injection.
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

// TruncateAllTables truncates all tables (USE WITH CAUTION — for testing only)
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

// ========================================
// HEALTH CHECK
// ========================================

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
