package commands

import "context"

type Command interface {
	CommandType() string
	Validate() error
	IdempotencyKey() string
}

type Result struct {
	AggregateID string
	Payload     interface{}
}

type Handler[T Command] interface {
	Handle(ctx context.Context, cmd T) (Result, error)
}
