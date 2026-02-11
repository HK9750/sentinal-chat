package main

import (
	"log"

	"sentinal-chat/config"
	"sentinal-chat/internal/events"
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

	// Initialize logger singleton
	if cfg.AppMode == server.ReleaseMode {
		logger.Init(logger.ProductionMode)
	} else {
		logger.Init(logger.DevelopmentMode)
	}
	logInstance := logger.GetGlobalLogger()

	// Repositories - using database singleton
	userRepo := repository.NewUserRepository(database.GetInstance())
	encryptionRepo := repository.NewEncryptionRepository(database.GetInstance())
	messageRepo := repository.NewMessageRepository(database.GetInstance())
	conversationRepo := repository.NewConversationRepository(database.GetInstance())
	uploadRepo := repository.NewUploadRepository(database.GetInstance())
	broadcastRepo := repository.NewBroadcastRepository(database.GetInstance())
	callRepo := repository.NewCallRepository(database.GetInstance())

	// Initialize Redis singleton
	redis.Initialize(redis.Config{
		Host:     cfg.RedisHost,
		Port:     cfg.RedisPort,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	redisClient := redis.GetClient()
	signalingStore := redis.NewSignalingStore(redisClient)
	rateLimiter := redis.NewRateLimiter(redisClient, redis.DefaultRateLimitConfig())
	cacheStore := redis.NewCacheStore(redisClient, redis.DefaultCacheConfig())

	// Initialize Event Bus (Redis Pub/Sub)
	channelResolver := events.NewHybridChannelResolver()
	eventBus := events.NewRedisEventBus(redisClient, channelResolver)
	if err := eventBus.Start(); err != nil {
		log.Fatalf("Failed to start event bus: %v", err)
	}

	// Create Outbox Repository
	outboxRepo := repository.NewOutboxRepository(database.GetInstance())

	// Create Event Publisher
	eventPublisher := services.NewEventPublisher(outboxRepo)

	// Start Outbox Worker
	outboxWorker := services.NewOutboxWorker(outboxRepo, eventBus)
	outboxWorker.Start()

	//Services
	authService := services.NewAuthService(userRepo, cfg)
	messageService := services.NewMessageService(database.GetDB(), messageRepo, conversationRepo, eventPublisher)
	conversationService := services.NewConversationService(database.GetDB(), conversationRepo, eventPublisher)
	userService := services.NewUserService(userRepo)
	uploadService := services.NewUploadService(uploadRepo)
	encryptionService := services.NewEncryptionService(encryptionRepo)
	broadcastService := services.NewBroadcastService(broadcastRepo)
	callService := services.NewCallService(database.GetDB(), callRepo, signalingStore, eventPublisher)

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

	// Graceful shutdown
	defer func() {
		outboxWorker.Stop()
		eventBus.Stop()
	}()

	// Server start
	if err := serverInstance.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
