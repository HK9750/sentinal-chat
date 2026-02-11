package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

// RedisEventBus implements EventBus using Redis Pub/Sub
type RedisEventBus struct {
	client   *redis.Client
	resolver ChannelResolver
	handlers map[EventType][]EventHandler
	pubsub   *redis.PubSub
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	running  bool
}

func NewRedisEventBus(client *redis.Client, resolver ChannelResolver) *RedisEventBus {
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisEventBus{
		client:   client,
		resolver: resolver,
		handlers: make(map[EventType][]EventHandler),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (b *RedisEventBus) Start() error {
	b.running = true
	b.pubsub = b.client.PSubscribe(b.ctx, "channel:*")
	go b.listen()
	return nil
}

func (b *RedisEventBus) Stop() error {
	b.cancel()
	b.running = false
	if b.pubsub != nil {
		b.pubsub.Close()
	}
	return nil
}

func (b *RedisEventBus) Publish(ctx context.Context, event Event) error {
	if !b.running {
		return fmt.Errorf("event bus not started")
	}

	channels := b.resolver.ResolveChannels(event)
	if len(channels) == 0 {
		return nil
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	for _, channel := range channels {
		if err := b.client.Publish(ctx, channel, data).Err(); err != nil {
			fmt.Printf("Failed to publish to %s: %v\n", channel, err)
		}
	}
	return nil
}

func (b *RedisEventBus) Subscribe(eventType EventType, handler EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
	return nil
}

func (b *RedisEventBus) listen() {
	ch := b.pubsub.Channel()
	for {
		select {
		case <-b.ctx.Done():
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}

			var base BaseEvent
			if err := json.Unmarshal([]byte(msg.Payload), &base); err != nil {
				continue
			}

			b.dispatch(base.EventTypeVal, []byte(msg.Payload))
		}
	}
}

func (b *RedisEventBus) dispatch(eventType EventType, data []byte) {
	b.mu.RLock()
	handlers := b.handlers[eventType]
	b.mu.RUnlock()

	for _, handler := range handlers {
		go func(h EventHandler) {
			event := b.unmarshalEvent(eventType, data)
			if event != nil {
				_ = h.Handle(b.ctx, event)
			}
		}(handler)
	}
}

func (b *RedisEventBus) unmarshalEvent(eventType EventType, data []byte) Event {
	switch eventType {
	case EventMessageNew:
		var e MessageNewEvent
		if err := json.Unmarshal(data, &e); err == nil {
			return &e
		}
	case EventMessageRead:
		var e MessageReadEvent
		if err := json.Unmarshal(data, &e); err == nil {
			return &e
		}
	case EventMessageDelivered:
		var e MessageDeliveredEvent
		if err := json.Unmarshal(data, &e); err == nil {
			return &e
		}
	case EventTypingStarted, EventTypingStopped:
		var e TypingEvent
		if err := json.Unmarshal(data, &e); err == nil {
			return &e
		}
	case EventPresenceOnline, EventPresenceOffline:
		var e PresenceEvent
		if err := json.Unmarshal(data, &e); err == nil {
			return &e
		}
	case EventCallOffer, EventCallAnswer, EventCallICE:
		var e CallSignalingEvent
		if err := json.Unmarshal(data, &e); err == nil {
			return &e
		}
	case EventCallEnded:
		var e CallEndedEvent
		if err := json.Unmarshal(data, &e); err == nil {
			return &e
		}
	}
	return nil
}
