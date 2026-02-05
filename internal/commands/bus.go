package commands

import (
	"context"
	"sync"
)

type Bus struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

func NewBus() *Bus {
	return &Bus{handlers: make(map[string]Handler)}
}

func (b *Bus) Register(commandType string, handler Handler) {
	b.mu.Lock()
	b.handlers[commandType] = handler
	b.mu.Unlock()
}

func (b *Bus) Execute(ctx context.Context, cmd Command) (Result, error) {
	b.mu.RLock()
	h, ok := b.handlers[cmd.CommandType()]
	b.mu.RUnlock()
	if !ok {
		return Result{}, ErrHandlerNotFound
	}
	return h.Handle(ctx, cmd)
}
