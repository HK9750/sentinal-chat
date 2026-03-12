package services

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	convdomain "sentinal-chat/internal/domain/conversation"
	"sentinal-chat/internal/domain/outbox"
	"sentinal-chat/internal/events"
	chatproxy "sentinal-chat/internal/proxy"
	"sentinal-chat/internal/repository"
	chatws "sentinal-chat/internal/websocket"
	sentinal_errors "sentinal-chat/pkg/errors"
)

type ConversationService struct {
	conversations repository.ConversationRepository
	users         repository.UserRepository
	outbox        repository.OutboxRepository
	command       *CommandService
	proxy         *chatproxy.MembershipProxy
	messageSvc    *MessageService
}

func NewConversationService(conversations repository.ConversationRepository, users repository.UserRepository, outboxRepo repository.OutboxRepository, commandSvc *CommandService) *ConversationService {
	return &ConversationService{
		conversations: conversations,
		users:         users,
		outbox:        outboxRepo,
		command:       commandSvc,
		proxy:         chatproxy.NewMembershipProxy(conversations),
	}
}

func (s *ConversationService) AttachMessageService(messageSvc *MessageService) {
	s.messageSvc = messageSvc
}

func (s *ConversationService) Create(ctx context.Context, in CreateConversationInput) (ConversationView, error) {
	if s == nil || s.conversations == nil || s.users == nil {
		return ConversationView{}, sentinal_errors.ErrServiceUnavailable
	}
	if in.CreatorID == uuid.Nil {
		return ConversationView{}, sentinal_errors.ErrUnauthorized
	}
	convType := strings.ToUpper(strings.TrimSpace(in.Type))
	if convType != "DM" && convType != "GROUP" {
		return ConversationView{}, sentinal_errors.ErrInvalidInput
	}
	if _, err := s.users.GetUserByID(ctx, in.CreatorID); err != nil {
		return ConversationView{}, err
	}

	participantIDs := chatDedupeUUIDs(in.ParticipantIDs)
	if convType == "DM" {
		if len(participantIDs) != 1 || participantIDs[0] == in.CreatorID {
			return ConversationView{}, sentinal_errors.ErrInvalidInput
		}
		existing, err := s.conversations.GetDirectConversation(ctx, in.CreatorID, participantIDs[0])
		if err == nil {
			return s.buildConversationView(ctx, existing, in.CreatorID)
		}
		if err != sentinal_errors.ErrNotFound {
			return ConversationView{}, err
		}
	}
	if convType == "GROUP" && len(participantIDs) == 0 {
		return ConversationView{}, sentinal_errors.ErrInvalidInput
	}

	now := time.Now().UTC()
	conv := convdomain.Conversation{
		ID:               uuid.New(),
		Type:             convType,
		Subject:          nullableConversationString(strings.TrimSpace(in.Subject)),
		Description:      nullableConversationString(strings.TrimSpace(in.Description)),
		AvatarURL:        nullableConversationString(strings.TrimSpace(in.AvatarURL)),
		DisappearingMode: normalizeDisappearingMode(in.DisappearingMode),
		CreatedBy:        chatNullableUUID(in.CreatorID),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if convType == "DM" {
		conv.DMUserIDA = chatNullableUUID(minUUID(in.CreatorID, participantIDs[0]))
		conv.DMUserIDB = chatNullableUUID(maxUUID(in.CreatorID, participantIDs[0]))
		conv.Subject = sql.NullString{}
		conv.Description = sql.NullString{}
		conv.AvatarURL = sql.NullString{}
	}
	if err := s.conversations.Create(ctx, &conv); err != nil {
		return ConversationView{}, err
	}

	owner := convdomain.Participant{
		ConversationID:   conv.ID,
		UserID:           in.CreatorID,
		Role:             "OWNER",
		JoinedAt:         now,
		AddedBy:          chatNullableUUID(in.CreatorID),
		LastReadSequence: 0,
	}
	if err := s.conversations.AddParticipant(ctx, &owner); err != nil {
		return ConversationView{}, err
	}

	for _, participantID := range participantIDs {
		if _, err := s.users.GetUserByID(ctx, participantID); err != nil {
			return ConversationView{}, err
		}
		participant := convdomain.Participant{
			ConversationID:   conv.ID,
			UserID:           participantID,
			Role:             "MEMBER",
			JoinedAt:         now,
			AddedBy:          chatNullableUUID(in.CreatorID),
			LastReadSequence: 0,
		}
		if err := s.conversations.AddParticipant(ctx, &participant); err != nil {
			return ConversationView{}, err
		}
	}

	if s.outbox != nil {
		envelope := chatws.NewConversationEvent(events.ConversationCreated, conv.ID, map[string]any{
			"conversation_id": conv.ID.String(),
			"created_by":      in.CreatorID.String(),
			"type":            conv.Type,
		})
		if event, err := chatws.NewOutboxEvent(events.ConversationCreated, outbox.AggregateConversation, conv.ID, envelope); err == nil {
			_ = s.outbox.Create(ctx, nil, event)
		}
	}

	created, err := s.conversations.GetByID(ctx, conv.ID)
	if err != nil {
		return ConversationView{}, err
	}
	return s.buildConversationView(ctx, created, in.CreatorID)
}

func (s *ConversationService) List(ctx context.Context, userID uuid.UUID, page, limit int) ([]ConversationView, int64, error) {
	conversations, total, err := s.conversations.GetUserConversations(ctx, userID, chatNormalizePage(page), chatNormalizeLimit(limit, 50))
	if err != nil {
		return nil, 0, err
	}
	items := make([]ConversationView, 0, len(conversations))
	for _, conv := range conversations {
		item, buildErr := s.buildConversationView(ctx, conv, userID)
		if buildErr != nil {
			return nil, 0, buildErr
		}
		items = append(items, item)
	}
	return items, total, nil
}

func (s *ConversationService) Get(ctx context.Context, conversationID, userID uuid.UUID) (ConversationView, error) {
	if err := s.proxy.RequireParticipant(ctx, conversationID, userID); err != nil {
		return ConversationView{}, err
	}
	conv, err := s.conversations.GetByID(ctx, conversationID)
	if err != nil {
		return ConversationView{}, err
	}
	return s.buildConversationView(ctx, conv, userID)
}

func (s *ConversationService) AddParticipant(ctx context.Context, in AddParticipantInput) (ConversationView, error) {
	if err := s.proxy.RequireAdminOrOwner(ctx, in.ConversationID, in.ActorID); err != nil {
		return ConversationView{}, err
	}
	conv, err := s.conversations.GetByID(ctx, in.ConversationID)
	if err != nil {
		return ConversationView{}, err
	}
	if conv.Type != "GROUP" {
		return ConversationView{}, sentinal_errors.ErrConflict
	}
	if _, err := s.users.GetUserByID(ctx, in.UserID); err != nil {
		return ConversationView{}, err
	}
	participant := convdomain.Participant{
		ConversationID:   in.ConversationID,
		UserID:           in.UserID,
		Role:             normalizeParticipantRole(in.Role),
		JoinedAt:         time.Now().UTC(),
		AddedBy:          chatNullableUUID(in.ActorID),
		LastReadSequence: 0,
	}
	if err := s.conversations.AddParticipant(ctx, &participant); err != nil {
		return ConversationView{}, err
	}
	if s.outbox != nil {
		envelope := chatws.NewConversationEvent(events.ConversationParticipantAdded, in.ConversationID, map[string]any{
			"conversation_id": in.ConversationID.String(),
			"user_id":         in.UserID.String(),
			"added_by":        in.ActorID.String(),
			"role":            participant.Role,
		})
		if event, err := chatws.NewOutboxEvent(events.ConversationParticipantAdded, outbox.AggregateConversation, in.ConversationID, envelope); err == nil {
			_ = s.outbox.Create(ctx, nil, event)
		}
	}
	updated, err := s.conversations.GetByID(ctx, in.ConversationID)
	if err != nil {
		return ConversationView{}, err
	}
	return s.buildConversationView(ctx, updated, in.ActorID)
}

func (s *ConversationService) RemoveParticipant(ctx context.Context, in RemoveParticipantInput) (ConversationView, error) {
	if in.ActorID != in.UserID {
		if err := s.proxy.RequireAdminOrOwner(ctx, in.ConversationID, in.ActorID); err != nil {
			return ConversationView{}, err
		}
	} else if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.ActorID); err != nil {
		return ConversationView{}, err
	}
	if err := s.conversations.RemoveParticipant(ctx, in.ConversationID, in.UserID); err != nil {
		return ConversationView{}, err
	}
	if s.outbox != nil {
		envelope := chatws.NewConversationEvent(events.ConversationParticipantRemoved, in.ConversationID, map[string]any{
			"conversation_id": in.ConversationID.String(),
			"user_id":         in.UserID.String(),
			"removed_by":      in.ActorID.String(),
		})
		if event, err := chatws.NewOutboxEvent(events.ConversationParticipantRemoved, outbox.AggregateConversation, in.ConversationID, envelope); err == nil {
			_ = s.outbox.Create(ctx, nil, event)
		}
	}
	updated, err := s.conversations.GetByID(ctx, in.ConversationID)
	if err != nil {
		if err == sentinal_errors.ErrNotFound {
			return ConversationView{ID: in.ConversationID.String()}, nil
		}
		return ConversationView{}, err
	}
	return s.buildConversationView(ctx, updated, in.ActorID)
}

func (s *ConversationService) Clear(ctx context.Context, in ClearConversationInput) error {
	if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.ActorID); err != nil {
		return err
	}
	if err := s.conversations.ClearConversation(ctx, in.ConversationID, in.ActorID); err != nil {
		return err
	}
	if s.command != nil {
		_, _ = s.command.Record(ctx, "CLEAR_CHAT", in.ActorID, &in.ConversationID, map[string]any{
			"conversation_id": in.ConversationID.String(),
		}, nil)
	}
	return nil
}

func (s *ConversationService) buildConversationView(ctx context.Context, conv convdomain.Conversation, userID uuid.UUID) (ConversationView, error) {
	var lastMessage *MessageView
	if s.messageSvc != nil {
		msg, err := s.messageSvc.getLatestView(ctx, conv.ID, userID)
		if err == nil {
			lastMessage = &msg
		}
	}
	unreadCount := int64(0)
	lastRead := int64(0)
	participant, err := s.conversations.GetParticipant(ctx, conv.ID, userID)
	if err == nil {
		lastRead = participant.LastReadSequence
		if s.messageSvc != nil {
			unreadCount, _ = s.messageSvc.unreadCountSince(ctx, conv.ID, lastRead)
		}
	}
	return conversationToView(conv, lastMessage, unreadCount, lastRead), nil
}

func normalizeDisappearingMode(value string) string {
	mode := strings.ToUpper(strings.TrimSpace(value))
	switch mode {
	case "24_HOURS", "7_DAYS", "90_DAYS":
		return mode
	default:
		return "OFF"
	}
}

func normalizeParticipantRole(value string) string {
	role := strings.ToUpper(strings.TrimSpace(value))
	if role == "ADMIN" || role == "OWNER" {
		return role
	}
	return "MEMBER"
}

func nullableConversationString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
