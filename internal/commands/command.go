package commands

import (
	"context"

	"github.com/google/uuid"
)

type Command interface {
	CommandType() string
	Validate() error
	IdempotencyKey() string
}

type RequiresAuth interface {
	ActorID() uuid.UUID
}

type Result struct {
	AggregateID string
	Payload     interface{}
}

type Handler interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

type HandlerFunc func(ctx context.Context, cmd Command) (Result, error)

func (h HandlerFunc) Handle(ctx context.Context, cmd Command) (Result, error) {
	return h(ctx, cmd)
}
