package services

import (
	"context"
	"database/sql"
	"time"

	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/domain/broadcast"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type BroadcastService struct {
	repo      repository.BroadcastRepository
	bus       *commands.Bus
	eventRepo repository.EventRepository
}

func NewBroadcastService(repo repository.BroadcastRepository, eventRepo repository.EventRepository, bus *commands.Bus) *BroadcastService {
	if bus == nil {
		bus = commands.NewBus()
	}
	svc := &BroadcastService{repo: repo, eventRepo: eventRepo, bus: bus}
	svc.RegisterHandlers(bus)
	return svc
}

func (s *BroadcastService) RegisterHandlers(bus *commands.Bus) {
	if bus == nil {
		return
	}

	// broadcast.create - Create a new broadcast list
	bus.Register("broadcast.create", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.CreateBroadcastListCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		b := &broadcast.BroadcastList{
			ID:        uuid.New(),
			OwnerID:   c.OwnerID,
			Name:      c.Name,
			CreatedAt: time.Now(),
		}
		if c.Description != "" {
			b.Description = sql.NullString{String: c.Description, Valid: true}
		}
		if err := s.Create(ctx, b); err != nil {
			return commands.Result{}, err
		}
		// Add initial recipients
		for _, recipientID := range c.RecipientIDs {
			r := &broadcast.BroadcastRecipient{
				BroadcastID: b.ID,
				UserID:      recipientID,
				AddedAt:     time.Now(),
			}
			_ = s.AddRecipient(ctx, r)
		}
		return commands.Result{AggregateID: b.ID.String(), Payload: b}, nil
	}))

	// broadcast.update - Update a broadcast list
	bus.Register("broadcast.update", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.UpdateBroadcastListCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		existing, err := s.GetByID(ctx, c.BroadcastID)
		if err != nil {
			return commands.Result{}, err
		}
		if existing.OwnerID != c.OwnerID {
			return commands.Result{}, sentinal_errors.ErrForbidden
		}
		if c.Name != "" {
			existing.Name = c.Name
		}
		if c.Description != "" {
			existing.Description = sql.NullString{String: c.Description, Valid: true}
		}
		if err := s.Update(ctx, existing); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "broadcast", "broadcast.updated", c.BroadcastID, existing)
		return commands.Result{AggregateID: c.BroadcastID.String(), Payload: existing}, nil
	}))

	// broadcast.delete - Delete a broadcast list
	bus.Register("broadcast.delete", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.DeleteBroadcastListCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		existing, err := s.GetByID(ctx, c.BroadcastID)
		if err != nil {
			return commands.Result{}, err
		}
		if existing.OwnerID != c.OwnerID {
			return commands.Result{}, sentinal_errors.ErrForbidden
		}
		if err := s.Delete(ctx, c.BroadcastID); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "broadcast", "broadcast.deleted", c.BroadcastID, map[string]any{"broadcast_id": c.BroadcastID})
		return commands.Result{AggregateID: c.BroadcastID.String()}, nil
	}))

	// broadcast.add_recipient - Add a recipient to broadcast list
	bus.Register("broadcast.add_recipient", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.AddBroadcastRecipientCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		existing, err := s.GetByID(ctx, c.BroadcastID)
		if err != nil {
			return commands.Result{}, err
		}
		if existing.OwnerID != c.OwnerID {
			return commands.Result{}, sentinal_errors.ErrForbidden
		}
		r := &broadcast.BroadcastRecipient{
			BroadcastID: c.BroadcastID,
			UserID:      c.RecipientID,
			AddedAt:     time.Now(),
		}
		if err := s.AddRecipient(ctx, r); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: c.BroadcastID.String(), Payload: r}, nil
	}))

	// broadcast.remove_recipient - Remove a recipient from broadcast list
	bus.Register("broadcast.remove_recipient", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.RemoveBroadcastRecipientCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		existing, err := s.GetByID(ctx, c.BroadcastID)
		if err != nil {
			return commands.Result{}, err
		}
		if existing.OwnerID != c.OwnerID {
			return commands.Result{}, sentinal_errors.ErrForbidden
		}
		if err := s.RemoveRecipient(ctx, c.BroadcastID, c.RecipientID); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: c.BroadcastID.String()}, nil
	}))

	// broadcast.send_message - Send a message to broadcast list
	bus.Register("broadcast.send_message", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.SendBroadcastMessageCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		existing, err := s.GetByID(ctx, c.BroadcastID)
		if err != nil {
			return commands.Result{}, err
		}
		if existing.OwnerID != c.SenderID {
			return commands.Result{}, sentinal_errors.ErrForbidden
		}
		// Create outbox event for broadcast message - actual message creation to recipients
		// would be handled by a separate processor/worker that fans out to each recipient's DM
		_ = createOutboxEvent(ctx, s.eventRepo, "broadcast", "broadcast.sent", c.BroadcastID, map[string]any{
			"broadcast_id": c.BroadcastID,
			"sender_id":    c.SenderID,
			"content":      c.Content,
			"message_type": c.MessageType,
		})
		return commands.Result{AggregateID: c.BroadcastID.String()}, nil
	}))
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
