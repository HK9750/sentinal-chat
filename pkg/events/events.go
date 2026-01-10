package events

import "context"

type Event struct {
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp int64       `json:"timestamp"`
}

type Handler func(ctx context.Context, event Event) error

type Publisher interface {
	Publish(ctx context.Context, channel string, event Event) error
}

type Subscriber interface {
	Subscribe(ctx context.Context, channel string, handler Handler) error
}

type Broker interface {
	Publisher
	Subscriber
}
