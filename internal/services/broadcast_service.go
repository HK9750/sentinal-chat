package services

import (
	"context"

	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/domain/broadcast"
	"sentinal-chat/internal/repository"

	"github.com/google/uuid"
)

type BroadcastService struct {
	repo      repository.BroadcastRepository
	bus       *commands.Bus
	eventRepo repository.EventRepository
}

func NewBroadcastService(repo repository.BroadcastRepository, eventRepo repository.EventRepository, bus *commands.Bus) *BroadcastService {
	return &BroadcastService{repo: repo, eventRepo: eventRepo, bus: bus}
}

func (s *BroadcastService) Create(ctx context.Context, b *broadcast.BroadcastList) error {
	if err := s.repo.Create(ctx, b); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "broadcast", "broadcast.created", b.ID, b)
}

func (s *BroadcastService) GetByID(ctx context.Context, id uuid.UUID) (broadcast.BroadcastList, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *BroadcastService) Update(ctx context.Context, b broadcast.BroadcastList) error {
	return s.repo.Update(ctx, b)
}

func (s *BroadcastService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *BroadcastService) GetUserBroadcastLists(ctx context.Context, ownerID uuid.UUID) ([]broadcast.BroadcastList, error) {
	return s.repo.GetUserBroadcastLists(ctx, ownerID)
}

func (s *BroadcastService) SearchBroadcastLists(ctx context.Context, ownerID uuid.UUID, query string) ([]broadcast.BroadcastList, error) {
	return s.repo.SearchBroadcastLists(ctx, ownerID, query)
}

func (s *BroadcastService) AddRecipient(ctx context.Context, r *broadcast.BroadcastRecipient) error {
	if err := s.repo.AddRecipient(ctx, r); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "broadcast", "broadcast.recipient_added", r.BroadcastID, r)
}

func (s *BroadcastService) RemoveRecipient(ctx context.Context, broadcastID, userID uuid.UUID) error {
	if err := s.repo.RemoveRecipient(ctx, broadcastID, userID); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "broadcast", "broadcast.recipient_removed", broadcastID, map[string]any{"broadcast_id": broadcastID, "user_id": userID})
}

func (s *BroadcastService) GetRecipients(ctx context.Context, broadcastID uuid.UUID) ([]broadcast.BroadcastRecipient, error) {
	return s.repo.GetRecipients(ctx, broadcastID)
}

func (s *BroadcastService) GetRecipientCount(ctx context.Context, broadcastID uuid.UUID) (int64, error) {
	return s.repo.GetRecipientCount(ctx, broadcastID)
}

func (s *BroadcastService) IsRecipient(ctx context.Context, broadcastID, userID uuid.UUID) (bool, error) {
	return s.repo.IsRecipient(ctx, broadcastID, userID)
}

func (s *BroadcastService) BulkAddRecipients(ctx context.Context, broadcastID uuid.UUID, userIDs []uuid.UUID) error {
	return s.repo.BulkAddRecipients(ctx, broadcastID, userIDs)
}

func (s *BroadcastService) BulkRemoveRecipients(ctx context.Context, broadcastID uuid.UUID, userIDs []uuid.UUID) error {
	return s.repo.BulkRemoveRecipients(ctx, broadcastID, userIDs)
}
