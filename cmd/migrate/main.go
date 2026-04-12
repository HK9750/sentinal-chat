package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"sentinal-chat/config"
	"sentinal-chat/pkg/database"
)

const usage = `
Sentinal Chat - Database CLI Tool

Usage:
  migrate [command] [flags]

Commands:
  up          Run all SQL migrations
  down        Roll back all SQL migrations
  status      Show database status and core table counts
  seed        Seed only the admin account
  seed-dev    Seed admin plus development data
  reseed-dev  Truncate tables and reseed development data
  reset       Drop all tables and re-run migrations (dangerous)
  truncate    Truncate all data in known tables (dangerous)

Flags:
  -migrations string   Path to migrations directory (default "migrations")
  -admin-email string  Admin email for seeding (default "admin@sentinal.chat")
  -admin-pass string   Admin password for seeding
  -admin-user string   Admin username for seeding (default "admin")
  -admin-name string   Admin display name for seeding (default "System Administrator")
  -test-users int      Number of dev users to seed for seed-dev/reseed-dev (default 6)
  -force               Skip dangerous action countdowns

Examples:
  go run cmd/migrate/main.go up
  go run cmd/migrate/main.go status
  go run cmd/migrate/main.go seed -admin-pass "StrongPassword123!"
  go run cmd/migrate/main.go seed-dev -test-users 4
  go run cmd/migrate/main.go reseed-dev -force
`

type cliOptions struct {
	migrationsDir string
	adminEmail    string
	adminPass     string
	adminUser     string
	adminName     string
	testUsers     int
	force         bool
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	options := parseFlags()
	command := strings.ToLower(strings.TrimSpace(flag.Arg(0)))

	cfg := config.LoadConfig()
	database.Connect(cfg)
	defer database.Close()

	switch command {
	case "up":
		runMigrationsUp(options.migrationsDir)
	case "down":
		runMigrationsDown(options.migrationsDir)
	case "status":
		showStatus()
	case "seed":
		runSeed(false, options)
	case "seed-dev":
		runSeed(true, options)
	case "reseed-dev":
		runReseedDev(options)
	case "reset":
		runReset(options)
	case "truncate":
		runTruncate(options.force)
	default:
		failf("unknown command %q", command)
	}
}

func parseFlags() cliOptions {
	flag.Usage = func() { fmt.Print(usage) }

	migrationsDir := flag.String("migrations", "migrations", "Path to migrations directory")
	adminEmail := flag.String("admin-email", "admin@sentinal.chat", "Admin email for seeding")
	adminPass := flag.String("admin-pass", "", "Admin password for seeding")
	adminUser := flag.String("admin-user", "admin", "Admin username for seeding")
	adminName := flag.String("admin-name", "System Administrator", "Admin display name for seeding")
	testUsers := flag.Int("test-users", 6, "Number of dev users to seed")
	force := flag.Bool("force", false, "Skip dangerous action countdowns")

	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	return cliOptions{
		migrationsDir: strings.TrimSpace(*migrationsDir),
		adminEmail:    strings.TrimSpace(*adminEmail),
		adminPass:     strings.TrimSpace(*adminPass),
		adminUser:     strings.TrimSpace(*adminUser),
		adminName:     strings.TrimSpace(*adminName),
		testUsers:     *testUsers,
		force:         *force,
	}
}

func runMigrationsUp(migrationsDir string) {
	log.Printf("Running migrations from %q", migrationsDir)
	if err := database.RunFullMigration(migrationsDir); err != nil {
		failf("migration up failed: %v", err)
	}
	log.Println("Migrations completed successfully")
}

func runMigrationsDown(migrationsDir string) {
	confirmDangerousAction("rollback all down migrations", false)
	log.Printf("Rolling back migrations from %q", migrationsDir)
	if err := database.RollbackMigrations(migrationsDir); err != nil {
		failf("migration rollback failed: %v", err)
	}
	log.Println("Rollback completed successfully")
}

func showStatus() {
	log.Println("Checking database status...")
	if err := database.Ping(); err != nil {
		failf("database ping failed: %v", err)
	}
	log.Println("Database connection: OK")

	tables := []string{
		"users",
		"user_notification_settings",
		"notifications",
		"devices",
		"user_sessions",
		"conversations",
		"participants",
		"messages",
		"attachments",
		"oauth_identities",
		"outbox_events",
	}

	for _, table := range tables {
		exists, err := database.TableExists(table)
		if err != nil {
			log.Printf("%-18s error: %v", table, err)
			continue
		}
		if !exists {
			log.Printf("%-18s missing", table)
			continue
		}
		count, err := database.GetTableCount(table)
		if err != nil {
			log.Printf("%-18s exists (count unavailable: %v)", table, err)
			continue
		}
		log.Printf("%-18s exists (%d rows)", table, count)
	}

	if err := database.HealthCheck(); err != nil {
		failf("health check failed: %v", err)
	}
	log.Println("Health check: PASSED")
}

func runSeed(devMode bool, options cliOptions) {
	seedConfig := buildSeedConfig(options, devMode)
	if devMode {
		log.Printf("Seeding development data with %d test users...", seedConfig.TestUserCount)
		result, err := database.Seed(seedConfig)
		if err != nil {
			failf("development seed failed: %v", err)
		}
		log.Printf("Seeded admin=%s test_users=%d created_users=%d existing_users=%d created_devices=%d updated_devices=%d created_contacts=%d created_dms=%d created_oauth_identities=%d",
			result.AdminUser.Email.String,
			len(result.TestUsers),
			result.CreatedUsers,
			result.ExistingUsers,
			result.CreatedDevices,
			result.UpdatedDevices,
			result.CreatedContacts,
			result.CreatedDMs,
			result.CreatedOAuthIDs,
		)
		return
	}

	log.Println("Seeding admin account...")
	adminUser, err := database.SeedMinimal(seedConfig)
	if err != nil {
		failf("admin seed failed: %v", err)
	}
	log.Printf("Admin account ready: %s (%s)", adminUser.Email.String, adminUser.ID)
}

func runReseedDev(options cliOptions) {
	confirmDangerousAction("truncate all known tables and reseed development data", options.force)
	seedConfig := buildSeedConfig(options, true)
	result, err := database.ClearAndReseed(seedConfig)
	if err != nil {
		failf("reseed failed: %v", err)
	}
	log.Printf("Reseed complete. admin=%s test_users=%d created_users=%d created_devices=%d created_contacts=%d created_dms=%d created_oauth_identities=%d",
		result.AdminUser.Email.String,
		len(result.TestUsers),
		result.CreatedUsers,
		result.CreatedDevices,
		result.CreatedContacts,
		result.CreatedDMs,
		result.CreatedOAuthIDs,
	)
}

func runReset(options cliOptions) {
	confirmDangerousAction("drop all tables and rerun migrations", options.force)
	if err := database.DropAllTables(); err != nil {
		failf("drop all tables failed: %v", err)
	}
	if err := database.RunFullMigration(options.migrationsDir); err != nil {
		failf("migration after reset failed: %v", err)
	}
	log.Println("Database reset completed successfully")
}

func runTruncate(force bool) {
	confirmDangerousAction("truncate all known tables", force)
	if err := database.TruncateAllTables(); err != nil {
		failf("truncate failed: %v", err)
	}
	log.Println("All known tables truncated successfully")
}

func buildSeedConfig(options cliOptions, devMode bool) *database.SeedConfig {
	adminPass := options.adminPass
	if adminPass == "" {
		adminPass = os.Getenv("SEED_ADMIN_PASSWORD")
	}
	if adminPass == "" {
		adminPass = "Admin@123!"
	}

	seedConfig := &database.SeedConfig{
		AdminEmail:       defaultIfEmpty(options.adminEmail, "admin@sentinal.chat"),
		AdminPassword:    adminPass,
		AdminUsername:    defaultIfEmpty(options.adminUser, "admin"),
		AdminDisplayName: defaultIfEmpty(options.adminName, "System Administrator"),
		CreateTestUsers:  devMode,
		TestUserCount:    options.testUsers,
	}

	if seedConfig.TestUserCount < 0 {
		failf("test-users cannot be negative")
	}

	return seedConfig
}

func confirmDangerousAction(action string, force bool) {
	if force {
		log.Printf("Force enabled, proceeding to %s", action)
		return
	}

	log.Printf("Dangerous action requested: %s", action)
	log.Println("Press Ctrl+C within 5 seconds to cancel...")
	for i := 5; i > 0; i-- {
		fmt.Printf("%d... ", i)
		time.Sleep(time.Second)
	}
	fmt.Println()
}

func defaultIfEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func failf(format string, args ...any) {
	log.Printf(format, args...)
	os.Exit(1)
}
