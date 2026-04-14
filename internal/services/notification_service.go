package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	convdomain "sentinal-chat/internal/domain/conversation"
	"sentinal-chat/internal/domain/notification"
	"sentinal-chat/internal/events"
	"sentinal-chat/internal/repository"
	chatws "sentinal-chat/internal/websocket"
	sentinal_errors "sentinal-chat/pkg/errors"
	"sentinal-chat/pkg/logger"
)

type NotificationService struct {
	notifications repository.NotificationRepository
	conversations repository.ConversationRepository
	messages      repository.MessageRepository
	broadcaster   chatws.Broadcaster
	logger        *logger.Logger
}

func NewNotificationService(
	notifications repository.NotificationRepository,
	conversations repository.ConversationRepository,
	messages repository.MessageRepository,
	broadcaster chatws.Broadcaster,
	l *logger.Logger,
) *NotificationService {
	var componentLogger *logger.Logger
	if l != nil {
		componentLogger = l.WithComponent("notification_service")
	}
	return &NotificationService{
		notifications: notifications,
		conversations: conversations,
		messages:      messages,
		broadcaster:   broadcaster,
		logger:        componentLogger,
	}
}

func (s *NotificationService) GetSettings(ctx context.Context, userID uuid.UUID) (NotificationSettingsView, error) {
	if s == nil || s.notifications == nil {
		return NotificationSettingsView{}, sentinal_errors.ErrServiceUnavailable
	}
	settings, err := s.notifications.GetSettings(ctx, userID)
	if err != nil {
		if err == sentinal_errors.ErrNotFound {
			return NotificationSettingsView{
				InAppEnabled:       true,
				SoundEnabled:       true,
				ShowMessagePreview: true,
			}, nil
		}
		return NotificationSettingsView{}, err
	}
	return NotificationSettingsView{
		InAppEnabled:       settings.InAppEnabled,
		SoundEnabled:       settings.SoundEnabled,
		ShowMessagePreview: settings.ShowMessagePreview,
	}, nil
}

func (s *NotificationService) UpdateSettings(ctx context.Context, userID uuid.UUID, input UpdateNotificationSettingsInput) (NotificationSettingsView, error) {
	if s == nil || s.notifications == nil {
		return NotificationSettingsView{}, sentinal_errors.ErrServiceUnavailable
	}
	settings, err := s.notifications.GetSettings(ctx, userID)
	if err != nil {
		if err != sentinal_errors.ErrNotFound {
			return NotificationSettingsView{}, err
		}
		settings = notification.UserNotificationSettings{
			UserID:             userID,
			InAppEnabled:       true,
			SoundEnabled:       true,
			ShowMessagePreview: true,
		}
	}
	if input.InAppEnabled != nil {
		settings.InAppEnabled = *input.InAppEnabled
	}
	if input.SoundEnabled != nil {
		settings.SoundEnabled = *input.SoundEnabled
	}
	if input.ShowMessagePreview != nil {
		settings.ShowMessagePreview = *input.ShowMessagePreview
	}
	if err := s.notifications.UpsertSettings(ctx, &settings); err != nil {
		return NotificationSettingsView{}, err
	}

	view := NotificationSettingsView{
		InAppEnabled:       settings.InAppEnabled,
		SoundEnabled:       settings.SoundEnabled,
		ShowMessagePreview: settings.ShowMessagePreview,
	}
	_ = s.publishSettingsUpdated(ctx, userID, view)
	return view, nil
}

func (s *NotificationService) List(ctx context.Context, userID uuid.UUID, page, limit int, unreadOnly bool) ([]NotificationView, int64, error) {
	if s == nil || s.notifications == nil {
		return nil, 0, sentinal_errors.ErrServiceUnavailable
	}
	items, total, err := s.notifications.ListByUser(ctx, userID, page, limit, unreadOnly)
	if err != nil {
		return nil, 0, err
	}
	views := make([]NotificationView, 0, len(items))
	for _, item := range items {
		view, mapErr := mapNotificationToView(item)
		if mapErr != nil {
			return nil, 0, mapErr
		}
		views = append(views, view)
	}
	return views, total, nil
}

func (s *NotificationService) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	if s == nil || s.notifications == nil {
		return sentinal_errors.ErrServiceUnavailable
	}
	if err := s.notifications.MarkRead(ctx, userID, notificationID); err != nil {
		return err
	}
	unreadCount, err := s.notifications.CountUnread(ctx, userID)
	if err == nil {
		s.emitBadge(ctx, userID, unreadCount)
	}
	envelope := chatws.NewUserEvent(events.NotificationRead, userID.String(), map[string]any{
		"notification_id": notificationID.String(),
	})
	s.emitToUser(ctx, userID, envelope)
	return nil
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	if s == nil || s.notifications == nil {
		return 0, sentinal_errors.ErrServiceUnavailable
	}
	updated, err := s.notifications.MarkAllRead(ctx, userID)
	if err != nil {
		return 0, err
	}
	s.emitBadge(ctx, userID, 0)
	envelope := chatws.NewUserEvent(events.NotificationReadAll, userID.String(), map[string]any{
		"updated": updated,
	})
	s.emitToUser(ctx, userID, envelope)
	return updated, nil
}

func (s *NotificationService) PublishMessageNotification(ctx context.Context, message MessageView) {
	if s == nil || s.notifications == nil || s.conversations == nil {
		return
	}

	conversationID, err := uuid.Parse(strings.TrimSpace(message.ConversationID))
	if err != nil {
		return
	}
	senderID, err := uuid.Parse(strings.TrimSpace(message.SenderID))
	if err != nil {
		return
	}
	messageID, err := uuid.Parse(strings.TrimSpace(message.ID))
	if err != nil {
		return
	}

	participants, err := s.conversations.GetParticipants(ctx, conversationID)
	if err != nil {
		return
	}

	conversationTitle := s.conversationTitle(conversationID, participants, senderID)
	senderName := s.participantDisplayName(senderID, participants)
	content := "sent a message"
	if message.Type == string(MessageKindAudio) {
		content = "sent a voice note"
	} else if message.Type == string(MessageKindFile) {
		content = "sent an attachment"
	} else if message.Content != nil && strings.TrimSpace(*message.Content) != "" {
		content = strings.TrimSpace(*message.Content)
	}

	for _, participant := range participants {
		recipientID := participant.UserID
		if recipientID == senderID {
			continue
		}
		if s.isConversationMuted(participant) {
			continue
		}
		settings := s.getEffectiveSettings(ctx, recipientID)
		if !settings.InAppEnabled {
			continue
		}

		body := content
		if settings.ShowMessagePreview {
			if senderName != "" {
				body = senderName + ": " + content
			}
		} else {
			if senderName != "" {
				body = senderName + " sent a new message"
			} else {
				body = "You have a new message"
			}
		}

		metadata := map[string]any{
			"kind":            "MESSAGE",
			"message_id":      message.ID,
			"conversation_id": message.ConversationID,
			"actor_id":        message.SenderID,
			"sound_enabled":   settings.SoundEnabled,
		}

		item, createErr := s.upsertNotification(ctx, notificationUpsertInput{
			UserID:         recipientID,
			ActorID:        &senderID,
			ConversationID: &conversationID,
			MessageID:      &messageID,
			Type:           "MESSAGE_NEW",
			Title:          conversationTitle,
			Body:           body,
			DeepLink:       "/chat?conversation=" + message.ConversationID,
			Metadata:       metadata,
			DedupeKey:      fmt.Sprintf("message:new:%s", message.ID),
		})
		if createErr != nil {
			s.logCreateError(ctx, "message", recipientID, createErr)
			continue
		}
		s.emitNotificationCreated(ctx, item, settings.SoundEnabled)
	}
}

func (s *NotificationService) PublishMissedCallNotification(ctx context.Context, callID, conversationID, actorID uuid.UUID, reason string) {
	if s == nil || s.notifications == nil || s.conversations == nil {
		return
	}
	if normalizeCallEndReason(reason) != "MISSED" {
		return
	}

	participants, err := s.conversations.GetParticipants(ctx, conversationID)
	if err != nil {
		return
	}
	conversationTitle := s.conversationTitle(conversationID, participants, actorID)
	actorName := s.participantDisplayName(actorID, participants)

	for _, participant := range participants {
		recipientID := participant.UserID
		if recipientID == actorID {
			continue
		}
		if s.isConversationMuted(participant) {
			continue
		}
		settings := s.getEffectiveSettings(ctx, recipientID)
		if !settings.InAppEnabled {
			continue
		}

		body := "You missed a call"
		if actorName != "" {
			body = actorName + " called you"
		}

		metadata := map[string]any{
			"kind":            "CALL",
			"call_id":         callID.String(),
			"conversation_id": conversationID.String(),
			"actor_id":        actorID.String(),
			"sound_enabled":   settings.SoundEnabled,
		}

		item, createErr := s.upsertNotification(ctx, notificationUpsertInput{
			UserID:         recipientID,
			ActorID:        &actorID,
			ConversationID: &conversationID,
			CallID:         &callID,
			Type:           "CALL_MISSED",
			Title:          conversationTitle,
			Body:           body,
			DeepLink:       "/chat?conversation=" + conversationID.String(),
			Metadata:       metadata,
			DedupeKey:      fmt.Sprintf("call:missed:%s:%s", callID.String(), recipientID.String()),
		})
		if createErr != nil {
			s.logCreateError(ctx, "missed_call", recipientID, createErr)
			continue
		}
		s.emitNotificationCreated(ctx, item, settings.SoundEnabled)
	}
}

func (s *NotificationService) logCreateError(ctx context.Context, source string, recipientID uuid.UUID, err error) {
	if s == nil || s.logger == nil || err == nil {
		return
	}
	s.logger.ErrorwCtx(ctx,
		"notification.create.failed",
		"source", strings.TrimSpace(source),
		"recipient_id", recipientID.String(),
		"error", err.Error(),
	)
}

type notificationUpsertInput struct {
	UserID         uuid.UUID
	ActorID        *uuid.UUID
	ConversationID *uuid.UUID
	MessageID      *uuid.UUID
	CallID         *uuid.UUID
	Type           string
	Title          string
	Body           string
	DeepLink       string
	Metadata       map[string]any
	DedupeKey      string
}

func (s *NotificationService) upsertNotification(ctx context.Context, in notificationUpsertInput) (notification.Notification, error) {
	if len(in.Metadata) == 0 {
		in.Metadata = map[string]any{}
	}
	encodedMetadata, err := json.Marshal(in.Metadata)
	if err != nil {
		return notification.Notification{}, err
	}

	item := &notification.Notification{
		UserID:   in.UserID,
		Type:     strings.TrimSpace(in.Type),
		Title:    strings.TrimSpace(in.Title),
		Body:     strings.TrimSpace(in.Body),
		DeepLink: strings.TrimSpace(in.DeepLink),
		Metadata: encodedMetadata,
		DedupeKey: sql.NullString{
			String: strings.TrimSpace(in.DedupeKey),
			Valid:  strings.TrimSpace(in.DedupeKey) != "",
		},
	}
	if in.ActorID != nil {
		item.ActorID = uuid.NullUUID{UUID: *in.ActorID, Valid: true}
	}
	if in.ConversationID != nil {
		item.ConversationID = uuid.NullUUID{UUID: *in.ConversationID, Valid: true}
	}
	if in.MessageID != nil {
		item.MessageID = uuid.NullUUID{UUID: *in.MessageID, Valid: true}
	}
	if in.CallID != nil {
		item.CallID = uuid.NullUUID{UUID: *in.CallID, Valid: true}
	}

	return s.notifications.UpsertByDedupeKey(ctx, item)
}

func (s *NotificationService) emitNotificationCreated(ctx context.Context, item notification.Notification, soundEnabled bool) {
	view, err := mapNotificationToView(item)
	if err != nil {
		return
	}
	envelope := chatws.NewUserEvent(events.NotificationNew, item.UserID.String(), map[string]any{
		"notification":  view,
		"sound_enabled": soundEnabled,
	})
	s.emitToUser(ctx, item.UserID, envelope)
	unreadCount, err := s.notifications.CountUnread(ctx, item.UserID)
	if err == nil {
		s.emitBadge(ctx, item.UserID, unreadCount)
	}
}

func (s *NotificationService) emitBadge(ctx context.Context, userID uuid.UUID, unreadCount int64) {
	envelope := chatws.NewUserEvent(events.NotificationBadge, userID.String(), map[string]any{
		"unread_count": unreadCount,
	})
	s.emitToUser(ctx, userID, envelope)
}

func (s *NotificationService) publishSettingsUpdated(ctx context.Context, userID uuid.UUID, settings NotificationSettingsView) error {
	envelope := chatws.NewUserEvent(events.NotificationSettingsUpdated, userID.String(), map[string]any{
		"settings": settings,
	})
	s.emitToUser(ctx, userID, envelope)
	return nil
}

func (s *NotificationService) emitToUser(ctx context.Context, userID uuid.UUID, envelope chatws.EventEnvelope) {
	if s == nil || s.broadcaster == nil {
		return
	}
	s.broadcaster.SendToUser(userID, envelope)
	_ = s.broadcaster.PublishToUser(ctx, userID, envelope)
}

func (s *NotificationService) getEffectiveSettings(ctx context.Context, userID uuid.UUID) NotificationSettingsView {
	settings, err := s.GetSettings(ctx, userID)
	if err != nil {
		return NotificationSettingsView{
			InAppEnabled:       true,
			SoundEnabled:       true,
			ShowMessagePreview: true,
		}
	}
	return settings
}

func (s *NotificationService) isConversationMuted(participant convdomain.Participant) bool {
	if !participant.MutedUntil.Valid {
		return false
	}
	return participant.MutedUntil.Time.After(time.Now().UTC())
}

func (s *NotificationService) participantDisplayName(userID uuid.UUID, participants []convdomain.Participant) string {
	for _, participant := range participants {
		if participant.UserID != userID {
			continue
		}
		name := strings.TrimSpace(participant.DisplayName)
		if name != "" {
			return name
		}
		if strings.TrimSpace(participant.Username) != "" {
			return strings.TrimSpace(participant.Username)
		}
	}
	return ""
}

func (s *NotificationService) conversationTitle(conversationID uuid.UUID, participants []convdomain.Participant, actorID uuid.UUID) string {
	if len(participants) <= 2 {
		for _, participant := range participants {
			if participant.UserID == actorID {
				continue
			}
			name := strings.TrimSpace(participant.DisplayName)
			if name != "" {
				return name
			}
			if strings.TrimSpace(participant.Username) != "" {
				return strings.TrimSpace(participant.Username)
			}
		}
	}
	return "Conversation"
}

func mapNotificationToView(item notification.Notification) (NotificationView, error) {
	metadata := map[string]any{}
	if len(item.Metadata) > 0 {
		if err := json.Unmarshal(item.Metadata, &metadata); err != nil {
			return NotificationView{}, err
		}
	}
	view := NotificationView{
		ID:        item.ID.String(),
		UserID:    item.UserID.String(),
		Type:      item.Type,
		Title:     item.Title,
		Body:      item.Body,
		DeepLink:  item.DeepLink,
		Metadata:  metadata,
		IsRead:    item.IsRead,
		ReadAt:    chatNullTime(item.ReadAt),
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
	if item.ActorID.Valid {
		value := item.ActorID.UUID.String()
		view.ActorID = &value
	}
	if item.ConversationID.Valid {
		value := item.ConversationID.UUID.String()
		view.ConversationID = &value
	}
	if item.MessageID.Valid {
		value := item.MessageID.UUID.String()
		view.MessageID = &value
	}
	if item.CallID.Valid {
		value := item.CallID.UUID.String()
		view.CallID = &value
	}
	return view, nil
}
