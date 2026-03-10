package main

import (
	"log"

	"sentinal-chat/config"
	"sentinal-chat/internal/server"
	"sentinal-chat/pkg/database"
	"sentinal-chat/pkg/logger"
)

func main() {
	// Load configuration from environment / .env
	cfg := config.LoadConfig()

	// Connect to the database (singleton)
	database.Connect(cfg)
	defer database.Close()

	// Run migrations on startup
	if err := database.RunFullMigration("migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize logger singleton based on app mode
	if cfg.AppMode == server.ReleaseMode {
		logger.Init(logger.ProductionMode)
	} else {
		logger.Init(logger.DevelopmentMode)
	}
	logInstance := logger.GetGlobalLogger()

	// Create and configure the server
	srv := server.New(cfg, logInstance)
	srv.SetupBaseRoutes()

	// Start the server (blocks until shutdown signal)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
