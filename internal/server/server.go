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
	"sentinal-chat/internal/services"
	"sentinal-chat/pkg/database"
	"sentinal-chat/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Mode constants for the gin engine
var (
	ReleaseMode = "release"
	DebugMode   = "debug"
	TestMode    = "test"
)

// Server wraps the HTTP server, gin engine, config, and logger
type Server struct {
	httpServer *http.Server
	engine     *gin.Engine
	config     *config.Config
	logger     *logger.Logger
}

type RouteHandlers struct {
	Auth         *handler.AuthHandler
	User         *handler.UserHandler
	Upload       *handler.UploadHandler
	Conversation *handler.ConversationHandler
	Message      *handler.MessageHandler
	WS           *handler.WSHandler
}

type RouteDependencies struct {
	Handlers    RouteHandlers
	AuthService *services.AuthService
}

// New creates a new Server instance and configures the gin engine mode
func New(cfg *config.Config, l *logger.Logger) *Server {
	switch cfg.AppMode {
	case DebugMode:
		gin.SetMode(gin.DebugMode)
	case TestMode:
		gin.SetMode(gin.TestMode)
	default:
		// Default to ReleaseMode to hide the [GIN-debug] route logs
		gin.SetMode(gin.ReleaseMode)
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

// InitRoutes registers middleware, diagnostics, and application routes.
func (s *Server) InitRoutes(deps RouteDependencies) {
	// Global middleware
	s.engine.Use(middleware.RequestIDMiddleware())
	s.engine.Use(middleware.CORSMiddleware(s.config.FrontendURL))
	s.engine.Use(middleware.LoggingMiddleware(s.logger))
	s.engine.Use(middleware.ErrorHandler(s.logger))

	// Diagnostic endpoints
	s.engine.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"message": "pong"}})
	})

	s.engine.GET("/health", func(c *gin.Context) {
		if err := database.HealthCheck(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   err.Error(),
				"code":    "UNHEALTHY",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "healthy"}})
	})

	s.engine.GET("/goroutines", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"goroutines": runtime.NumGoroutine()})
	})

	v1 := s.engine.Group("/v1")
	if deps.Handlers.WS != nil {
		deps.Handlers.WS.RegisterRoutes(v1)
	}
	if deps.Handlers.Auth != nil {
		deps.Handlers.Auth.RegisterPublicRoutes(v1)
	}

	protected := v1.Group("")
	if deps.AuthService != nil {
		protected.Use(middleware.AuthMiddleware(deps.AuthService.ParseAccessToken, deps.AuthService.ValidateAccessSession))
	}

	if deps.Handlers.Auth != nil {
		deps.Handlers.Auth.RegisterProtectedRoutes(protected)
	}
	if deps.Handlers.User != nil {
		deps.Handlers.User.RegisterRoutes(protected)
	}
	if deps.Handlers.Upload != nil {
		deps.Handlers.Upload.RegisterRoutes(protected)
	}
	if deps.Handlers.Conversation != nil {
		deps.Handlers.Conversation.RegisterRoutes(protected)
	}
	if deps.Handlers.Message != nil {
		deps.Handlers.Message.RegisterRoutes(protected)
	}
}

// Start begins listening and blocks until a shutdown signal is received.
// Performs graceful shutdown with a 5-second timeout.
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
		s.logger.Infof("Shutdown signal received. Shutting down in 5 seconds...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		if s.logger != nil {
			s.logger.Errorf("Error in graceful shutdown: %s", err)
		}
		return err
	}

	if s.logger != nil {
		s.logger.Infof("Server stopped gracefully")
	}

	return nil
}
