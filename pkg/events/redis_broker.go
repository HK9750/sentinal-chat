package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v8"
)

type RedisBroker struct {
	Client *redis.Client
}

func NewRedisBroker(addr, password string, db int) *RedisBroker {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &RedisBroker{Client: rdb}
}

func (b *RedisBroker) Publish(ctx context.Context, channel string, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	return b.Client.Publish(ctx, channel, data).Err()
}

func (b *RedisBroker) Subscribe(ctx context.Context, channel string, handler Handler) error {
	pubsub := b.Client.Subscribe(ctx, channel)

	// Start a goroutine to listen for messages
	go func() {
		defer pubsub.Close()
		ch := pubsub.Channel()
		for msg := range ch {
			var event Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				// Log error (should have a logger here eventually)
				fmt.Printf("Error unmarshaling event: %v\n", err)
				continue
			}
			if err := handler(ctx, event); err != nil {
				fmt.Printf("Error handling event: %v\n", err)
			}
		}
	}()

	return nil
}
