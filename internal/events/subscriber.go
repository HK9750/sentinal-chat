package events

import "context"

type Subscriber interface {
	Subscribe(ctx context.Context, channels []string, handler func(channel string, payload []byte)) error
}
