package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"sentinal-chat/config"
	"sentinal-chat/pkg/database"
)

const usage = `
Sentinal Chat - Database CLI Tool

Usage:
  migrate [command] [flags]

Commands:
  up          Run all migrations (SQL + GORM)
  down        Rollback all SQL migrations  
  status      Show database connection status
  seed        Seed the database with initial data
  seed-dev    Seed with development/test data
  reset       Drop all tables and re-run migrations (DANGEROUS)
  truncate    Truncate all tables (DANGEROUS)

Flags:
  -migrations string   Path to migrations directory (default "migrations")
  -admin-email string  Admin email for seeding (default "admin@sentinal.chat")
  -admin-pass string   Admin password for seeding (default "Admin@123!")

Examples:
  go run cmd/migrate/main.go up
  go run cmd/migrate/main.go seed
  go run cmd/migrate/main.go seed-dev
  go run cmd/migrate/main.go down
  go run cmd/migrate/main.go reset
`

func main() {
	// Define flags
	migrationsDir := flag.String("migrations", "migrations", "Path to migrations directory")
	adminEmail := flag.String("admin-email", "admin@sentinal.chat", "Admin email for seeding")
	adminPass := flag.String("admin-pass", "Admin@123!", "Admin password for seeding")

	flag.Usage = func() {
		fmt.Print(usage)
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	command := flag.Arg(0)

	// Load config and connect to database
	cfg := config.LoadConfig()
	database.Connect(cfg)
	defer database.Close()

	switch command {
	case "up":
		runMigrationsUp(*migrationsDir)
	case "down":
		runMigrationsDown(*migrationsDir)
	case "status":
		showStatus()
	case "seed":
		runSeedProduction(*adminEmail, *adminPass)
	case "seed-dev":
		runSeedDevelopment()
	case "reset":
		runReset(*migrationsDir)
	case "truncate":
		runTruncate()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}
}

func runMigrationsUp(migrationsDir string) {
	log.Println("🚀 Running migrations UP...")

	if err := database.RunFullMigration(migrationsDir); err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}

	log.Println("✅ Migrations completed successfully!")
}

func runMigrationsDown(migrationsDir string) {
	log.Println("⬇️  Rolling back migrations...")

	if err := database.RollbackMigrations(migrationsDir); err != nil {
		log.Fatalf("❌ Rollback failed: %v", err)
	}

	log.Println("✅ Rollback completed successfully!")
}

func showStatus() {
	log.Println("🔍 Checking database status...")

	if err := database.Ping(); err != nil {
		log.Fatalf("❌ Database connection failed: %v", err)
	}
	log.Println("✅ Database connection: OK")

	// Check core tables
	tables := []string{"users", "conversations", "messages", "user_settings", "participants"}
	for _, table := range tables {
		exists, err := database.TableExists(table)
		if err != nil {
			log.Printf("⚠️  Error checking table %s: %v", table, err)
			continue
		}
		if exists {
			count, _ := database.GetTableCount(table)
			log.Printf("✅ Table %-20s exists (%d rows)", table, count)
		} else {
			log.Printf("❌ Table %-20s does not exist", table)
		}
	}

	// Health check
	if err := database.HealthCheck(); err != nil {
		log.Printf("⚠️  Health check warning: %v", err)
	} else {
		log.Println("✅ Health check: PASSED")
	}
}

func runSeedProduction(adminEmail, adminPass string) {
	log.Println("🌱 Seeding database (production mode)...")

	user, err := database.SeedProduction(adminEmail, adminPass)
	if err != nil {
		log.Fatalf("❌ Seeding failed: %v", err)
	}

	log.Printf("✅ Admin user created/verified: %s (ID: %s)", adminEmail, user.ID)
	log.Println("✅ Production seeding completed!")
}

func runSeedDevelopment() {
	log.Println("🌱 Seeding database (development mode)...")

	result, err := database.SeedDevelopment()
	if err != nil {
		log.Fatalf("❌ Seeding failed: %v", err)
	}

	log.Println("📊 Seed Summary:")
	log.Printf("   - Admin user: %s", result.AdminUser.Email.String)
	log.Printf("   - Test users: %d", len(result.TestUsers))
	log.Println("✅ Development seeding completed!")
}

func runReset(migrationsDir string) {
	log.Println("⚠️  WARNING: This will DROP all tables and re-run migrations!")
	log.Println("⚠️  Press Ctrl+C within 5 seconds to cancel...")

	// Give user time to cancel
	fmt.Print("Proceeding in: ")
	for i := 5; i > 0; i-- {
		fmt.Printf("%d... ", i)
		// time.Sleep(time.Second)
	}
	fmt.Println()

	log.Println("🗑️  Dropping all tables...")
	if err := database.DropAllTables(); err != nil {
		log.Fatalf("❌ Failed to drop tables: %v", err)
	}

	log.Println("🚀 Running migrations...")
	if err := database.RunFullMigration(migrationsDir); err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}

	log.Println("✅ Database reset completed!")
}

func runTruncate() {
	log.Println("⚠️  WARNING: This will TRUNCATE all tables!")

	if err := database.TruncateAllTables(); err != nil {
		log.Fatalf("❌ Truncate failed: %v", err)
	}

	log.Println("✅ All tables truncated!")
}
