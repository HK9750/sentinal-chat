package main

import (
	"context"
	"log"
	"time"

	"sentinal-chat/config"
	"sentinal-chat/internal/handler"
	"sentinal-chat/internal/middleware"
	"sentinal-chat/internal/repository"
	"sentinal-chat/internal/server"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/storage"
	"sentinal-chat/pkg/database"
	"sentinal-chat/pkg/logger"
)

func main() {
	// Load configuration from environment / .env
	cfg := config.LoadConfig()

	// Connect to the database (singleton)
	database.Connect(cfg)

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

	// Repositories
	db := database.GetDB()
	userRepo := repository.NewUserRepository(db)
	oauthRepo := repository.NewOAuthIdentityRepository(db)
	uploadRepo := repository.NewUploadRepository(db)
	messageRepo := repository.NewMessageRepository(db)
	conversationRepo := repository.NewConversationRepository(db)

	// Token service
	tokenService, err := services.NewTokenService(
		cfg.JWTSecret,
		time.Duration(cfg.JWTExpiryMin)*time.Minute,
		time.Duration(cfg.RefreshExpiry)*24*time.Hour,
		"sentinal-chat",
	)
	if err != nil {
		log.Fatalf("Failed to initialize token service: %v", err)
	}

	// OAuth providers
	oauthClients := map[services.AuthProvider]services.OAuthProviderClient{}
	if googleClient, googleErr := services.NewGoogleOAuthClient(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURI); googleErr == nil {
		oauthClients[services.AuthProviderGoogle] = googleClient
	}
	if githubClient, githubErr := services.NewGitHubOAuthClient(cfg.GithubClientID, cfg.GithubClientSecret, cfg.GithubRedirectURI); githubErr == nil {
		oauthClients[services.AuthProviderGitHub] = githubClient
	}

	authService, err := services.NewAuthService(userRepo, oauthRepo, tokenService, oauthClients)
	if err != nil {
		log.Fatalf("Failed to initialize auth service: %v", err)
	}
	authHandler := handler.NewAuthHandler(authService, cfg, logInstance)

	s3Client, err := storage.NewClient(context.Background(), storage.S3Config{
		Region:     cfg.S3Region,
		Bucket:     cfg.S3Bucket,
		AccessKey:  cfg.S3AccessKeyID,
		SecretKey:  cfg.S3SecretKey,
		Endpoint:   cfg.S3Endpoint,
		PublicBase: cfg.S3PublicBase,
		PresignTTL: time.Duration(cfg.S3PresignTTL) * time.Second,
	})
	if err != nil {
		logInstance.Errorf("S3 upload client disabled: %v", err)
	}

	// Upload API services
	uploadService := services.NewUploadService(uploadRepo, messageRepo, conversationRepo, s3Client)
	uploadHandler := handler.NewUploadHandler(uploadService, logInstance)

	v1 := srv.Engine().Group("/v1")
	authHandler.RegisterPublicRoutes(v1)

	v1.Use(middleware.AuthMiddleware(authService.ParseAccessToken, authService.ValidateAccessSession))
	authHandler.RegisterProtectedRoutes(v1)
	uploadHandler.RegisterRoutes(v1)

	// defer the close of db
	defer func() {
		defer database.Close()
	}()

	// Start the server (blocks until shutdown signal)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
