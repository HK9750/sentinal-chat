package websocket

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"sentinal-chat/internal/events"
	redisclient "sentinal-chat/internal/redis"
	"sentinal-chat/internal/repository"
	"sentinal-chat/pkg/logger"
)

type OutboxWorker struct {
	repo   repository.OutboxRepository
	redis  *redisclient.Client
	logger *logger.Logger
	quit   chan struct{}
}

func NewOutboxWorker(repo repository.OutboxRepository, redis *redisclient.Client, log *logger.Logger) *OutboxWorker {
	return &OutboxWorker{repo: repo, redis: redis, logger: log, quit: make(chan struct{})}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	if w == nil || w.repo == nil || w.redis == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.quit:
				return
			case <-ticker.C:
				w.processBatch(ctx)
			}
		}
	}()
}

func (w *OutboxWorker) Stop() {
	if w == nil {
		return
	}
	close(w.quit)
}

func (w *OutboxWorker) processBatch(ctx context.Context) {
	eventsBatch, err := w.repo.GetPending(ctx, 50)
	if err != nil {
		return
	}
	for _, event := range eventsBatch {
		claimed, err := w.repo.MarkProcessing(ctx, event.ID.String())
		if err != nil || !claimed {
			continue
		}
		var envelope EventEnvelope
		if err := json.Unmarshal(event.Payload, &envelope); err != nil {
			_ = w.repo.MarkFailed(ctx, event.ID.String(), err.Error())
			continue
		}
		channel := channelForEnvelope(envelope)
		if channel == "" {
			_ = w.repo.MarkCompleted(ctx, event.ID.String())
			continue
		}
		if err := w.redis.Publish(ctx, channel, event.Payload); err != nil {
			_ = w.repo.ScheduleRetry(ctx, event.ID.String(), time.Now().Add(2*time.Second), err.Error())
			continue
		}
		_ = w.repo.MarkCompleted(ctx, event.ID.String())
	}
}

func channelForEnvelope(envelope EventEnvelope) string {
	if strings.TrimSpace(envelope.CallID) != "" {
		return events.CallChannel(envelope.CallID)
	}
	if strings.TrimSpace(envelope.ConversationID) != "" {
		return events.ConversationChannel(envelope.ConversationID)
	}
	return ""
}
