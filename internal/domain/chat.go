package domain

import (
	"context"
	"time"
)

type Message struct {
	ID        string    `json:"id" db:"id"`
	ChatID    string    `json:"chat_id" db:"chat_id"`
	SenderID  string    `json:"sender_id" db:"sender_id"`
	Content   string    `json:"content" db:"content"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Chat struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"` // Optional, for group chats
	IsGroup   bool      `json:"is_group" db:"is_group"`
	UserIDs   []string  `json:"user_ids" db:"-"` // Basic representation
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type ChatRepository interface {
	CreateMessage(ctx context.Context, msg *Message) error
	GetMessages(ctx context.Context, chatID string, limit, offset int) ([]*Message, error)
	CreateChat(ctx context.Context, chat *Chat) error
	GetChat(ctx context.Context, chatID string) (*Chat, error)
}

type ChatService interface {
	SendMessage(ctx context.Context, senderID, chatID, content string) (*Message, error)
	GetChatHistory(ctx context.Context, chatID string) ([]*Message, error)
}
