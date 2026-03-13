package services

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	calldomain "sentinal-chat/internal/domain/call"
	"sentinal-chat/internal/domain/outbox"
	"sentinal-chat/internal/events"
	chatproxy "sentinal-chat/internal/proxy"
	"sentinal-chat/internal/repository"
	chatws "sentinal-chat/internal/websocket"
	sentinal_errors "sentinal-chat/pkg/errors"
)

type CallService struct {
	calls         repository.CallRepository
	conversations repository.ConversationRepository
	outbox        repository.OutboxRepository
	proxy         *chatproxy.MembershipProxy
}

func NewCallService(calls repository.CallRepository, conversations repository.ConversationRepository, outboxRepo repository.OutboxRepository) *CallService {
	return &CallService{
		calls:         calls,
		conversations: conversations,
		outbox:        outboxRepo,
		proxy:         chatproxy.NewMembershipProxy(conversations),
	}
}

func (s *CallService) Start(ctx context.Context, in CallStartInput) (calldomain.Call, []conversationParticipant, error) {
	if s == nil || s.calls == nil || s.conversations == nil {
		return calldomain.Call{}, nil, sentinal_errors.ErrServiceUnavailable
	}
	if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.CallerID); err != nil {
		return calldomain.Call{}, nil, err
	}
	conv, err := s.conversations.GetByID(ctx, in.ConversationID)
	if err != nil {
		return calldomain.Call{}, nil, err
	}
	if conv.Type != "DM" {
		return calldomain.Call{}, nil, sentinal_errors.ErrConflict
	}
	callType := strings.ToUpper(strings.TrimSpace(in.Type))
	if callType != "AUDIO" && callType != "VIDEO" {
		return calldomain.Call{}, nil, sentinal_errors.ErrInvalidInput
	}
	now := time.Now().UTC()
	call := calldomain.Call{
		ID:             uuid.New(),
		ConversationID: in.ConversationID,
		InitiatedBy:    in.CallerID,
		Type:           callType,
		StartedAt:      now,
		CreatedAt:      now,
	}
	if err := s.calls.Create(ctx, &call); err != nil {
		return calldomain.Call{}, nil, err
	}
	participants, err := s.conversations.GetParticipants(ctx, in.ConversationID)
	if err != nil {
		return calldomain.Call{}, nil, err
	}
	resultParticipants := make([]conversationParticipant, 0, len(participants))
	for _, participant := range participants {
		status := "INVITED"
		joinedAt := sql.NullTime{}
		if participant.UserID == in.CallerID {
			status = "CONNECTED"
			joinedAt = sql.NullTime{Time: now, Valid: true}
		}
		if err := s.calls.AddParticipant(ctx, &calldomain.CallParticipant{CallID: call.ID, UserID: participant.UserID, Status: status, JoinedAt: joinedAt}); err != nil {
			return calldomain.Call{}, nil, err
		}
		resultParticipants = append(resultParticipants, conversationParticipant{UserID: participant.UserID, DisplayName: participant.DisplayName})
	}
	if s.outbox != nil {
		envelope := chatws.MarkLocal(chatws.NewCallEvent(events.CallIncoming, in.ConversationID, call.ID, map[string]any{"call_id": call.ID.String(), "initiated_by": in.CallerID.String(), "type": call.Type}))
		if event, err := chatws.NewOutboxEvent(events.CallIncoming, outbox.AggregateCall, call.ID, envelope); err == nil {
			if err := s.outbox.Create(ctx, nil, event); err != nil {
				return calldomain.Call{}, nil, err
			}
		} else {
			return calldomain.Call{}, nil, err
		}
	}
	return call, resultParticipants, nil
}

func (s *CallService) ForwardSignal(ctx context.Context, in CallSignalInput) error {
	if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.FromUserID); err != nil {
		return err
	}
	if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.ToUserID); err != nil {
		return err
	}
	if _, err := s.calls.GetByID(ctx, in.CallID); err != nil {
		return err
	}
	return nil
}

func (s *CallService) End(ctx context.Context, in CallEndInput) error {
	call, err := s.calls.GetByID(ctx, in.CallID)
	if err != nil {
		return err
	}
	if err := s.proxy.RequireParticipant(ctx, call.ConversationID, in.ActorID); err != nil {
		return err
	}
	if err := s.calls.EndCall(ctx, in.CallID, strings.ToUpper(strings.TrimSpace(in.Reason))); err != nil {
		return err
	}
	if s.outbox != nil {
		envelope := chatws.NewCallEvent(events.CallEnded, call.ConversationID, in.CallID, map[string]any{"call_id": in.CallID.String(), "reason": in.Reason, "actor_id": in.ActorID.String()})
		if event, err := chatws.NewOutboxEvent(events.CallEnded, outbox.AggregateCall, in.CallID, envelope); err == nil {
			if err := s.outbox.Create(ctx, nil, event); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

type conversationParticipant struct {
	UserID      uuid.UUID
	DisplayName string
}
