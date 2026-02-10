package main

import (
	"log"

	"sentinal-chat/config"
	"sentinal-chat/internal/handler"
	"sentinal-chat/internal/redis"
	"sentinal-chat/internal/repository"
	"sentinal-chat/internal/server"
	"sentinal-chat/internal/services"
	"sentinal-chat/pkg/database"
	"sentinal-chat/pkg/logger"
)

func main() {
	// Load config
	cfg := config.LoadConfig()

	// Connect the database
	database.Connect(cfg)

	// Run migrations
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

	// Repositories
	userRepo := repository.NewUserRepository(database.GetDB())
	encryptionRepo := repository.NewEncryptionRepository(database.GetDB())
	messageRepo := repository.NewMessageRepository(database.GetDB())
	conversationRepo := repository.NewConversationRepository(database.GetDB())
	uploadRepo := repository.NewUploadRepository(database.GetDB())
	broadcastRepo := repository.NewBroadcastRepository(database.GetDB())
	callRepo := repository.NewCallRepository(database.GetDB())

	//Redis 
	redisClient := redis.NewClient(redis.Config{
		Host:     cfg.RedisHost,
		Port:     cfg.RedisPort,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	signalingStore := redis.NewSignalingStore(redisClient)
	rateLimiter := redis.NewRateLimiter(redisClient, redis.DefaultRateLimitConfig())
	cacheStore := redis.NewCacheStore(redisClient, redis.DefaultCacheConfig())

	//Services
	authService := services.NewAuthService(userRepo, cfg)
	messageService := services.NewMessageService(database.GetDB(), messageRepo, conversationRepo)
	conversationService := services.NewConversationService(database.GetDB(), conversationRepo)
	userService := services.NewUserService(userRepo)
	uploadService := services.NewUploadService(uploadRepo)
	encryptionService := services.NewEncryptionService(encryptionRepo)
	broadcastService := services.NewBroadcastService(broadcastRepo)
	callService := services.NewCallService(callRepo, signalingStore)
	
	//Handlers
	authHandler := handler.NewAuthHandler(authService)
	messageHandler := handler.NewMessageHandler(messageService)
	conversationHandler := handler.NewConversationHandler(conversationService)
	userHandler := handler.NewUserHandler(userService)
	uploadHandler := handler.NewUploadHandler(uploadService)
	encryptionHandler := handler.NewEncryptionHandler(encryptionService)
	broadcastHandler := handler.NewBroadcastHandler(broadcastService)
	callHandler := handler.NewCallHandler(callService)

	// Server Instance init
	serverInstance := server.New(cfg, logInstance)

	// struct to init the handlers
	handlers := &server.Handlers{
		Auth:         authHandler,
		Message:      messageHandler,
		Conversation: conversationHandler,
		User:         userHandler,
		Call:         callHandler,
		Upload:       uploadHandler,
		Encryption:   encryptionHandler,
		Broadcast:    broadcastHandler,
	}

	// Setup routes
	serverInstance.SetupRoutes(handlers, authService, rateLimiter, cacheStore)

	// Server start
	if err := serverInstance.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
