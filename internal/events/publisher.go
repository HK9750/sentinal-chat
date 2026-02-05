package events

import (
	"context"
)

type Publisher interface {
	Publish(ctx context.Context, channel string, payload []byte) error
}
