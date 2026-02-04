package main

import (
	"log"
	"time"

	"sentinal-chat/config"
	"sentinal-chat/internal/handler"
	"sentinal-chat/internal/repository"
	"sentinal-chat/internal/server"
	"sentinal-chat/internal/services"
	"sentinal-chat/pkg/database"
	"sentinal-chat/pkg/logger"
)

func main() {
	cfg := config.LoadConfig()

	// Connect to Database
	database.Connect(cfg)

	// Run full migration (raw SQL + GORM AutoMigrate for all entities)
	if err := database.RunFullMigration("migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Printf("Starting server on port %s", cfg.AppPort)
	logInstance := logger.New(logger.DevelopmentMode)
	logger.SetGlobalLogger(logInstance)
	if cfg.AppMode == server.ReleaseMode {
		logInstance = logger.New(logger.ProductionMode)
		logger.SetGlobalLogger(logInstance)
	}

	userRepo := repository.NewUserRepository(database.GetDB())
	authService := services.NewAuthService(
		userRepo,
		cfg.JWTSecret,
		time.Duration(cfg.JWTExpiryMin)*time.Minute,
		time.Duration(cfg.RefreshExpiry)*24*time.Hour,
	)
	authHandler := handler.NewAuthHandler(authService)
	serverInstance := server.New(cfg, logInstance)
	serverInstance.SetupRoutes(&server.Handlers{Auth: authHandler}, authService)

	if err := serverInstance.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
