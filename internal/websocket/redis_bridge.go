package websocket

import (
	"context"

	"sentinal-chat/internal/events"
)

type RedisBridge struct {
	subscriber events.Subscriber
	hub        *Hub
}

func NewRedisBridge(subscriber events.Subscriber, hub *Hub) *RedisBridge {
	return &RedisBridge{subscriber: subscriber, hub: hub}
}

func (b *RedisBridge) Run(ctx context.Context, channels []string) error {
	return b.subscriber.Subscribe(ctx, channels, func(channel string, payload []byte) {
		b.hub.Broadcast(channelKey(channel), payload)
	})
}

func channelKey(channel string) string {
	return channel
}
