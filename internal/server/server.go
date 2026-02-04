package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sentinal-chat/config"
	"sentinal-chat/internal/handler"
	"sentinal-chat/internal/middleware"
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
	Auth *handler.AuthHandler
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

func (s *Server) SetupRoutes(handlers *Handlers, authService *services.AuthService) {
	s.engine.Use(middleware.RequestIDMiddleware())
	s.engine.Use(middleware.CORSMiddleware())
	s.engine.Use(middleware.LoggingMiddleware(s.logger))
	s.engine.Use(middleware.ErrorHandler(s.logger))

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
