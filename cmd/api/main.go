package main

import (
	"context"
	"log"
	"time"

	"sentinal-chat/config"
	"sentinal-chat/internal/handler"
	redisclient "sentinal-chat/internal/redis"
	"sentinal-chat/internal/repository"
	"sentinal-chat/internal/server"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/storage"
	chatws "sentinal-chat/internal/websocket"
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

	// Repositories
	db := database.GetDB()
	userRepo := repository.NewUserRepository(db)
	oauthRepo := repository.NewOAuthIdentityRepository(db)
	messageRepo := repository.NewMessageRepository(db)
	conversationRepo := repository.NewConversationRepository(db, logInstance)
	callRepo := repository.NewCallRepository(db)
	outboxRepo := repository.NewOutboxRepository(db)
	commandRepo := repository.NewCommandRepository(db)

	// Token service
	tokenService, err := services.NewTokenService(
		cfg.JWTSecret,
		time.Duration(cfg.JWTExpiryHours)*time.Hour,
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
	userService := services.NewUserService(userRepo)
	authHandler := handler.NewAuthHandler(authService, cfg, logInstance)
	userHandler := handler.NewUserHandler(userService, logInstance)

	s3Client, err := storage.NewClient(context.Background(), storage.S3Config{
		Region:     cfg.S3Region,
		Bucket:     cfg.S3Bucket,
		AccessKey:  cfg.S3AccessKeyID,
		SecretKey:  cfg.S3SecretKey,
		Endpoint:   cfg.S3Endpoint,
		PublicBase: cfg.S3PublicBase,
	})
	if err != nil {
		logInstance.Errorf("S3 upload client disabled: %v (region=%q bucket=%q endpoint=%q public_base=%q access_key_configured=%t secret_configured=%t)", err, cfg.S3Region, cfg.S3Bucket, cfg.S3Endpoint, cfg.S3PublicBase, cfg.S3AccessKeyID != "", cfg.S3SecretKey != "")
	} else {
		logInstance.Infof("S3 upload client enabled (region=%q bucket=%q endpoint=%q public_base=%q)", cfg.S3Region, cfg.S3Bucket, cfg.S3Endpoint, cfg.S3PublicBase)
	}

	uploadService := services.NewUploadService(messageRepo, conversationRepo, s3Client)
	commandService := services.NewCommandService(commandRepo)
	messageService := services.NewMessageService(messageRepo, conversationRepo, outboxRepo, commandService)
	conversationService := services.NewConversationService(conversationRepo, userRepo, outboxRepo, commandService, callRepo)
	conversationService.AttachMessageService(messageService)
	callService := services.NewCallService(callRepo, conversationRepo, outboxRepo)

	uploadHandler := handler.NewUploadHandler(uploadService, logInstance)
	conversationHandler := handler.NewConversationHandler(conversationService, logInstance)
	messageHandler := handler.NewMessageHandler(messageService, logInstance)
	redisClient, err := redisclient.NewRedis(cfg)
	if err != nil {
		logInstance.Errorf("Redis realtime disabled: %v", err)
	}
	realtimeHub := chatws.NewHub(redisClient, conversationRepo, logInstance)
	realtimeService := services.NewRealtimeService(realtimeHub, conversationService, messageService, callService, commandService, userService)
	outboxWorker := chatws.NewOutboxWorker(outboxRepo, redisClient, logInstance)
	wsHandler := handler.NewWSHandler(authService, realtimeHub, realtimeService, cfg.FrontendURL, logInstance)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	realtimeHub.StartRedisListener(ctx)
	outboxWorker.Start(ctx)

	srv.InitRoutes(server.RouteDependencies{
		Handlers: server.RouteHandlers{
			Auth:         authHandler,
			User:         userHandler,
			Upload:       uploadHandler,
			Conversation: conversationHandler,
			Message:      messageHandler,
			WS:           wsHandler,
		},
		AuthService:    authService,
		MessageService: messageService,
	})

	// defer the close of db
	defer func() {
		if redisClient != nil {
			_ = redisClient.Close()
		}
		defer database.Close()
	}()

	// Start the server (blocks until shutdown signal)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
