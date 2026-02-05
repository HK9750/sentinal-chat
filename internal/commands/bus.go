package commands

import (
	"context"
	"sync"
)

type Bus struct {
	mu         sync.RWMutex
	handlers   map[string]Handler
	actorProxy Proxy
}

func NewBus() *Bus {
	return &Bus{handlers: make(map[string]Handler)}
}

func NewBusWithProxy(proxy Proxy) *Bus {
	return &Bus{handlers: make(map[string]Handler), actorProxy: proxy}
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
	if b.actorProxy != nil {
		if err := b.actorProxy.Authorize(ctx, cmd); err != nil {
			return Result{}, err
		}
	}
	return h.Handle(ctx, cmd)
}
