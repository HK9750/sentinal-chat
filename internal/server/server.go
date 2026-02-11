package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"sentinal-chat/config"
	"sentinal-chat/internal/handler"
	"sentinal-chat/internal/middleware"
	"sentinal-chat/internal/redis"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"
	"sentinal-chat/pkg/database"
	"sentinal-chat/pkg/logger"

	"github.com/gin-gonic/gin"
)

type Server struct {
	httpServer *http.Server
	engine     *gin.Engine
	config     *config.Config
	logger     *logger.Logger
}

var (
	ReleaseMode = "release"
	DebugMode   = "debug"
	TestMode    = "test"
)

type Handlers struct {
	Auth         *handler.AuthHandler
	Message      *handler.MessageHandler
	Conversation *handler.ConversationHandler
	User         *handler.UserHandler
	Call         *handler.CallHandler
	Upload       *handler.UploadHandler
	Encryption   *handler.EncryptionHandler
	Broadcast    *handler.BroadcastHandler
}

func New(cfg *config.Config, l *logger.Logger) *Server {
	if cfg.AppMode == ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	} else if cfg.AppMode == TestMode {
		gin.SetMode(gin.TestMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())

	return &Server{
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%s", cfg.AppPort),
			Handler: engine,
		},
		engine: engine,
		config: cfg,
		logger: l,
	}
}

func (s *Server) SetupRoutes(handlers *Handlers, authService *services.AuthService, rateLimiter *redis.RateLimiter, cacheStore *redis.CacheStore, wsHandler *WebSocketHandler) {
	s.engine.Use(middleware.RequestIDMiddleware())
	s.engine.Use(middleware.CORSMiddleware())
	s.engine.Use(middleware.LoggingMiddleware(s.logger))
	s.engine.Use(middleware.ErrorHandler(s.logger))

	// Apply rate limiting middleware globally for auth endpoints
	if rateLimiter != nil {
		s.engine.Use(middleware.RateLimitMiddleware(rateLimiter))
	}

	// Store cacheStore reference for potential future use
	_ = cacheStore

	// WebSocket endpoint
	if wsHandler != nil {
		s.engine.GET("/v1/ws", wsHandler.Handle)
	}

	s.engine.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"message": "pong"}))
	})

	s.engine.GET("/health", func(c *gin.Context) {
		if err := database.HealthCheck(); err != nil {
			c.JSON(http.StatusServiceUnavailable, httpdto.NewErrorResponse(err.Error(), "UNHEALTHY"))
			return
		}
		c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"status": "healthy"}))
	})

	s.engine.GET("/goroutines", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"goroutines": runtime.NumGoroutine(),
		})
	})

	auth := s.engine.Group("/v1/auth")
	{
		auth.POST("/register", handlers.Auth.Register)
		auth.POST("/login", handlers.Auth.Login)
		auth.POST("/refresh", handlers.Auth.Refresh)
		auth.POST("/logout", middleware.AuthMiddleware(authService), handlers.Auth.Logout)
		auth.POST("/logout-all", middleware.AuthMiddleware(authService), handlers.Auth.LogoutAll)
		auth.GET("/sessions", middleware.AuthMiddleware(authService), handlers.Auth.Sessions)
		auth.POST("/password/forgot", handlers.Auth.PasswordForgot)
		auth.POST("/password/reset", handlers.Auth.PasswordReset)
	}

	if handlers.Message != nil {
		messages := s.engine.Group("/v1/messages")
		messages.Use(middleware.AuthMiddleware(authService))
		if rateLimiter != nil {
			messages.POST("", middleware.MessageRateLimitMiddleware(rateLimiter), handlers.Message.Send)
		} else {
			messages.POST("", handlers.Message.Send)
		}
		messages.GET("", handlers.Message.List)
		messages.GET("/:id", handlers.Message.GetByID)
		messages.PUT("/:id", handlers.Message.Update)
		messages.DELETE("/:id", handlers.Message.Delete)
		messages.DELETE("/:id/hard", handlers.Message.HardDelete)
		messages.POST("/:id/read", handlers.Message.MarkRead)
		messages.POST("/:id/delivered", handlers.Message.MarkDelivered)
	}

	if handlers.Conversation != nil {
		conversations := s.engine.Group("/v1/conversations")
		conversations.Use(middleware.AuthMiddleware(authService))
		conversations.POST("", handlers.Conversation.Create)
		conversations.GET("", handlers.Conversation.List)
		conversations.GET("/:id", handlers.Conversation.GetByID)
		conversations.PUT("/:id", handlers.Conversation.Update)
		conversations.DELETE("/:id", handlers.Conversation.Delete)
		conversations.GET("/direct", handlers.Conversation.GetDirect)
		conversations.GET("/search", handlers.Conversation.Search)
		conversations.GET("/type", handlers.Conversation.GetByType)
		conversations.GET("/invite", handlers.Conversation.GetByInviteLink)
		conversations.POST("/:id/invite", handlers.Conversation.RegenerateInviteLink)
		conversations.POST("/:id/participants", handlers.Conversation.AddParticipant)
		conversations.DELETE("/:id/participants/:user_id", handlers.Conversation.RemoveParticipant)
		conversations.GET("/:id/participants", handlers.Conversation.ListParticipants)
		conversations.PUT("/:id/participants/:user_id/role", handlers.Conversation.UpdateParticipantRole)
		conversations.POST("/:id/mute", handlers.Conversation.Mute)
		conversations.POST("/:id/unmute", handlers.Conversation.Unmute)
		conversations.POST("/:id/pin", handlers.Conversation.Pin)
		conversations.POST("/:id/unpin", handlers.Conversation.Unpin)
		conversations.POST("/:id/archive", handlers.Conversation.Archive)
		conversations.POST("/:id/unarchive", handlers.Conversation.Unarchive)
		conversations.POST("/:id/read-sequence", handlers.Conversation.UpdateLastReadSequence)
		conversations.GET("/:id/sequence", handlers.Conversation.GetSequence)
		conversations.POST("/:id/sequence", handlers.Conversation.IncrementSequence)
	}

	if handlers.User != nil {
		users := s.engine.Group("/v1/users")
		users.Use(middleware.AuthMiddleware(authService))
		users.GET("", handlers.User.List)
		users.GET("/me", handlers.User.GetProfile)
		users.PUT("/me", handlers.User.UpdateProfile)
		users.DELETE("/me", handlers.User.DeleteProfile)
		users.GET("/me/settings", handlers.User.GetSettings)
		users.PUT("/me/settings", handlers.User.UpdateSettings)
		users.GET("/me/contacts", handlers.User.ListContacts)
		users.POST("/me/contacts", handlers.User.AddContact)
		users.DELETE("/me/contacts/:id", handlers.User.RemoveContact)
		users.POST("/me/contacts/:id/block", handlers.User.BlockContact)
		users.POST("/me/contacts/:id/unblock", handlers.User.UnblockContact)
		users.GET("/me/contacts/blocked", handlers.User.BlockedContacts)
		users.GET("/me/devices", handlers.User.ListDevices)
		users.GET("/me/devices/:id", handlers.User.GetDevice)
		users.DELETE("/me/devices/:id", handlers.User.DeactivateDevice)
		users.GET("/me/push-tokens", handlers.User.ListPushTokens)
		users.DELETE("/me/sessions/:id", handlers.User.RevokeSession)
		users.DELETE("/me/sessions", handlers.User.RevokeAllSessions)
	}

	if handlers.Call != nil {
		calls := s.engine.Group("/v1/calls")
		calls.Use(middleware.AuthMiddleware(authService))
		if rateLimiter != nil {
			calls.POST("", middleware.CallRateLimitMiddleware(rateLimiter), handlers.Call.Create)
		} else {
			calls.POST("", handlers.Call.Create)
		}
		calls.GET("/:id", handlers.Call.GetByID)
		calls.GET("", handlers.Call.ListByConversation)
		calls.GET("/user", handlers.Call.ListByUser)
		calls.GET("/active", handlers.Call.ActiveCalls)
		calls.GET("/missed", handlers.Call.MissedCalls)
		calls.POST("/:id/participants", handlers.Call.AddParticipant)
		calls.DELETE("/:id/participants/:user_id", handlers.Call.RemoveParticipant)
		calls.GET("/:id/participants", handlers.Call.ListParticipants)
		calls.PUT("/:id/participants/:user_id/status", handlers.Call.UpdateParticipantStatus)
		calls.PUT("/:id/participants/:user_id/mute", handlers.Call.UpdateParticipantMute)
		calls.POST("/quality", handlers.Call.RecordQualityMetric)
		calls.POST("/:id/connected", handlers.Call.MarkConnected)
		calls.POST("/:id/end", handlers.Call.EndCall)
		calls.GET("/:id/duration", handlers.Call.GetCallDuration)
		calls.GET("/quality", handlers.Call.GetCallQualityMetrics)
		calls.GET("/quality/user", handlers.Call.GetUserCallQualityMetrics)
		calls.GET("/quality/average", handlers.Call.GetAverageCallQuality)
	}

	if handlers.Upload != nil {
		uploads := s.engine.Group("/v1/uploads")
		uploads.Use(middleware.AuthMiddleware(authService))
		uploads.POST("", handlers.Upload.Create)
		uploads.GET("/:id", handlers.Upload.GetByID)
		uploads.PUT("/:id", handlers.Upload.Update)
		uploads.DELETE("/:id", handlers.Upload.Delete)
		uploads.GET("", handlers.Upload.ListUser)
		uploads.GET("/completed", handlers.Upload.ListCompleted)
		uploads.GET("/in-progress", handlers.Upload.ListInProgress)
		uploads.POST("/:id/progress", handlers.Upload.UpdateProgress)
		uploads.POST("/:id/complete", handlers.Upload.MarkCompleted)
		uploads.POST("/:id/fail", handlers.Upload.MarkFailed)
		uploads.GET("/stale", handlers.Upload.ListStale)
		uploads.DELETE("/stale", handlers.Upload.DeleteStale)
	}

	if handlers.Encryption != nil {
		enc := s.engine.Group("/v1/encryption")
		enc.Use(middleware.AuthMiddleware(authService))
		enc.POST("/identity", handlers.Encryption.UploadIdentityKey)
		enc.GET("/identity", handlers.Encryption.GetIdentityKey)
		enc.PUT("/identity/:id/deactivate", handlers.Encryption.DeactivateIdentityKey)
		enc.DELETE("/identity/:id", handlers.Encryption.DeleteIdentityKey)
		enc.POST("/signed-prekeys", handlers.Encryption.UploadSignedPreKey)
		enc.GET("/signed-prekeys", handlers.Encryption.GetSignedPreKey)
		enc.GET("/signed-prekeys/active", handlers.Encryption.GetActiveSignedPreKey)
		enc.POST("/signed-prekeys/rotate", handlers.Encryption.RotateSignedPreKey)
		enc.PUT("/signed-prekeys/:id/deactivate", handlers.Encryption.DeactivateSignedPreKey)
		enc.POST("/onetime-prekeys", handlers.Encryption.UploadOneTimePreKeys)
		enc.POST("/onetime-prekeys/consume", handlers.Encryption.ConsumeOneTimePreKey)
		enc.GET("/onetime-prekeys/count", handlers.Encryption.GetPreKeyCount)
		enc.GET("/bundles", handlers.Encryption.GetKeyBundle)
		enc.GET("/keys/active", handlers.Encryption.HasActiveKeys)
	}

	if handlers.Broadcast != nil {
		broadcasts := s.engine.Group("/v1/broadcasts")
		broadcasts.Use(middleware.AuthMiddleware(authService))
		broadcasts.POST("", handlers.Broadcast.Create)
		broadcasts.GET("/:id", handlers.Broadcast.GetByID)
		broadcasts.PUT("/:id", handlers.Broadcast.Update)
		broadcasts.DELETE("/:id", handlers.Broadcast.Delete)
		broadcasts.GET("", handlers.Broadcast.ListByOwner)
		broadcasts.GET("/search", handlers.Broadcast.Search)
		broadcasts.POST("/:id/recipients", handlers.Broadcast.AddRecipient)
		broadcasts.DELETE("/:id/recipients/:user_id", handlers.Broadcast.RemoveRecipient)
		broadcasts.GET("/:id/recipients", handlers.Broadcast.ListRecipients)
		broadcasts.GET("/:id/recipients/count", handlers.Broadcast.RecipientCount)
		broadcasts.GET("/:id/recipients/:user_id", handlers.Broadcast.IsRecipient)
		broadcasts.POST("/:id/recipients/bulk", handlers.Broadcast.BulkAddRecipients)
		broadcasts.DELETE("/:id/recipients/bulk", handlers.Broadcast.BulkRemoveRecipients)
	}
}

func (s *Server) Start() error {
	go func() {
		if s.logger != nil {
			s.logger.Infof("Starting the server on port %s...", s.config.AppPort)
		}
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			if s.logger != nil {
				s.logger.Errorf("Error in starting the server: %s", err)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	if s.logger != nil {
		s.logger.Infof("Server is running on :%s", s.config.AppPort)
	}

	<-quit

	if s.logger != nil {
		s.logger.Infof("Quitting signal received.. Shutting down after 5 seconds")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		if s.logger != nil {
			s.logger.Infof("Error in the graceful shutdown of the server: %s", err)
		}
		return err
	}

	if s.logger != nil {
		s.logger.Infof("Server stopped gracefully")
	}

	return nil
}
