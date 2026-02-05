package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Subscriber struct {
	client *redis.Client
}

func NewSubscriber(client *redis.Client) *Subscriber {
	return &Subscriber{client: client}
}

func (s *Subscriber) Subscribe(ctx context.Context, channels []string, handler func(channel string, payload []byte)) error {
	sub := s.client.PSubscribe(ctx, channels...)
	defer sub.Close()

	for {
		msg, err := sub.ReceiveMessage(ctx)
		if err != nil {
			return err
		}
		handler(msg.Channel, []byte(msg.Payload))
	}
}
