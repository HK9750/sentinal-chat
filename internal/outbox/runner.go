package outbox

import (
	"context"
	"time"

	"sentinal-chat/internal/events"
	"sentinal-chat/internal/repository"
)

type Runner struct {
	processor *Processor
}

func NewRunner(processor *Processor) *Runner {
	return &Runner{processor: processor}
}

func (r *Runner) Start(ctx context.Context) {
	go r.processor.Run(ctx)
}

func DefaultProcessor(repo repository.EventRepository, publisher events.Publisher) *Processor {
	return NewProcessor(repo, publisher, 100, time.Second*2, 5)
}
