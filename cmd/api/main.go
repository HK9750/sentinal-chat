package main

import (
	"fmt"
	"log"
	"net/http"

	"sentinal-chat/config"
	"sentinal-chat/internal/domain/call"
	"sentinal-chat/internal/domain/conversation"
	"sentinal-chat/internal/domain/encryption"
	"sentinal-chat/internal/domain/event"
	"sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/domain/user"
	"sentinal-chat/pkg/database"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()

	// Connect to Database
	database.Connect(cfg)

	// Run Raw Migrations (Extensions, Enums, Procedures)
	if err := database.ApplyRawMigrations("migrations"); err != nil {
		log.Fatalf("Failed to apply raw migrations: %v", err)
	}

	// Run GORM AutoMigrate for Tables
	if err := database.DB.AutoMigrate(
		&user.User{},
		&user.UserSettings{},
		&user.Device{},
		&user.PushToken{},
		&user.UserSession{},
		&user.UserContact{},
		&conversation.Conversation{},
		&conversation.Participant{},
		&conversation.ConversationSequence{},
		&message.Message{},
		&message.MessageReaction{},
		&message.MessageReceipt{},
		&message.MessageMention{},
		&message.StarredMessage{},
		&encryption.IdentityKey{},
		&encryption.SignedPreKey{},
		&encryption.OneTimePreKey{},
		&encryption.EncryptedSession{},
		&call.Call{},
		&call.CallParticipant{},
		&call.CallQualityMetric{},
		&call.TurnCredential{},
		&event.OutboxEvent{},
		&event.CommandLog{},
		&event.AccessPolicy{},
	); err != nil {
		log.Fatalf("Failed to apply GORM migrations: %v", err)
	}

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	log.Printf("Starting server on port %s", cfg.AppPort)
	if err := r.Run(fmt.Sprintf(":%s", cfg.AppPort)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
