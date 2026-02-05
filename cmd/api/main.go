package main

import (
	"context"
	"log"
	"time"

	"sentinal-chat/config"
	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/handler"
	"sentinal-chat/internal/outbox"
	"sentinal-chat/internal/proxy"
	"sentinal-chat/internal/redis"
	"sentinal-chat/internal/repository"
	"sentinal-chat/internal/server"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/websocket"
	"sentinal-chat/pkg/database"
	"sentinal-chat/pkg/logger"
)

func main() {
	cfg := config.LoadConfig()

	database.Connect(cfg)

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
	authService := services.NewAuthService(userRepo, cfg)
	authHandler := handler.NewAuthHandler(authService)
	messageRepo := repository.NewMessageRepository(database.GetDB())
	conversationRepo := repository.NewConversationRepository(database.GetDB())
	eventRepo := repository.NewEventRepository(database.GetDB())
	uploadRepo := repository.NewUploadRepository(database.GetDB())
	broadcastRepo := repository.NewBroadcastRepository(database.GetDB())
	accessProxy := proxy.NewAccessControl(eventRepo, conversationRepo, broadcastRepo, uploadRepo)
	commandBus := commands.NewBusWithProxy(accessProxy)
	messageService := services.NewMessageService(database.GetDB(), messageRepo, eventRepo, accessProxy, commandBus)
	messageHandler := handler.NewMessageHandler(messageService)
	conversationService := services.NewConversationService(database.GetDB(), conversationRepo, eventRepo, accessProxy, commandBus)
	conversationHandler := handler.NewConversationHandler(conversationService)
	userService := services.NewUserService(userRepo, eventRepo, commandBus)
	userHandler := handler.NewUserHandler(userService)
	userService.RegisterHandlers()

	uploadService := services.NewUploadService(uploadRepo, eventRepo, commandBus)
	uploadHandler := handler.NewUploadHandler(uploadService)

	encryptionRepo := repository.NewEncryptionRepository(database.GetDB())
	encryptionService := services.NewEncryptionService(encryptionRepo, eventRepo, commandBus)
	encryptionHandler := handler.NewEncryptionHandler(encryptionService)

	broadcastService := services.NewBroadcastService(broadcastRepo, eventRepo, commandBus)
	broadcastHandler := handler.NewBroadcastHandler(broadcastService)

	// Redis infrastructure - must be initialized before services that depend on it
	redisClient := redis.NewClient(redis.Config{
		Host:     cfg.RedisHost,
		Port:     cfg.RedisPort,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	redisPublisher := redis.NewPublisher(redisClient)
	signalingStore := redis.NewSignalingStore(redisClient, redisPublisher)
	presenceStore := redis.NewPresenceStore(redisClient, redisPublisher, 5*time.Minute)
	subscriber := redis.NewSubscriber(redisClient)

	// Rate limiter and cache store (per database.md Appendix H)
	rateLimiter := redis.NewRateLimiter(redisClient, redis.DefaultRateLimitConfig())
	cacheStore := redis.NewCacheStore(redisClient, redis.DefaultCacheConfig())

	// Call service with signaling store for WebRTC
	callRepo := repository.NewCallRepository(database.GetDB())
	callService := services.NewCallService(callRepo, eventRepo, commandBus, signalingStore)
	callHandler := handler.NewCallHandler(callService)

	wsHub := websocket.NewHub()
	wsAuthorizer := websocket.NewChannelAuthorizer(conversationRepo, callRepo, broadcastRepo)
	wsHandler := websocket.NewHandler(authService, wsHub, commandBus, wsAuthorizer, presenceStore)
	go wsHub.Run(context.Background())

	bridge := websocket.NewRedisBridge(subscriber, wsHub)
	go bridge.Run(context.Background(), []string{
		"channel:user:*",
		"channel:conversation:*",
		"channel:call:*",
		"channel:presence:*",
		"channel:broadcast:*",
		"channel:upload:*",
		"channel:system:outbox",
	})

	processor := outbox.NewProcessor(eventRepo, redisPublisher, 100, time.Second*2, 5)
	go processor.Run(context.Background())

	conversationService.RegisterHandlers(commandBus)

	serverInstance := server.New(cfg, logInstance)

	handlers := &server.Handlers{
		Auth:         authHandler,
		Message:      messageHandler,
		Conversation: conversationHandler,
		User:         userHandler,
		Call:         callHandler,
		Upload:       uploadHandler,
		Encryption:   encryptionHandler,
		Broadcast:    broadcastHandler,
		WebSocket:    wsHandler,
	}

	serverInstance.SetupRoutes(handlers, authService, rateLimiter, cacheStore)

	if err := serverInstance.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
