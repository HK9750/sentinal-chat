package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MessageService struct {
	db               *gorm.DB
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	eventPublisher   *EventPublisher
}

type CiphertextPayload struct {
	RecipientDeviceID uuid.UUID
	Ciphertext        []byte
	Header            map[string]interface{}
}

type SendMessageInput struct {
	ConversationID uuid.UUID
	SenderID       uuid.UUID
	Ciphertexts    []CiphertextPayload
	MessageType    string
	ClientMsgID    string
	IdempotencyKey string
	Metadata       map[string]interface{}
}

func NewMessageService(db *gorm.DB, messageRepo repository.MessageRepository, conversationRepo repository.ConversationRepository, eventPublisher *EventPublisher) *MessageService {
	return &MessageService{
		db:               db,
		messageRepo:      messageRepo,
		conversationRepo: conversationRepo,
		eventPublisher:   eventPublisher,
	}
}

func (s *MessageService) SendMessage(ctx context.Context, input SendMessageInput) (message.Message, error) {
	return s.executeSendMessage(ctx, input)
}

func (s *MessageService) GetConversationMessages(ctx context.Context, conversationID uuid.UUID, beforeSeq int64, limit int, userID uuid.UUID) ([]message.Message, error) {
	if s.conversationRepo == nil {
		return nil, sentinal_errors.ErrForbidden
	}
	ok, err := s.conversationRepo.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, sentinal_errors.ErrForbidden
	}
	if limit <= 0 {
		limit = 50
	}
	deviceID, ok := DeviceIDFromContext(ctx)
	if !ok || !deviceID.Valid {
		return nil, sentinal_errors.ErrInvalidInput
	}
	return s.messageRepo.GetConversationMessages(ctx, conversationID, beforeSeq, limit, deviceID.UUID)
}

func (s *MessageService) GetByID(ctx context.Context, messageID uuid.UUID, userID uuid.UUID) (message.Message, error) {
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return message.Message{}, err
	}
	if s.conversationRepo != nil {
		ok, err := s.conversationRepo.IsParticipant(ctx, msg.ConversationID, userID)
		if err != nil {
			return message.Message{}, err
		}
		if !ok {
			return message.Message{}, sentinal_errors.ErrForbidden
		}
	}
	return msg, nil
}

func (s *MessageService) Delete(ctx context.Context, messageID uuid.UUID) error {
	return s.messageRepo.SoftDelete(ctx, messageID)
}

func (s *MessageService) HardDelete(ctx context.Context, messageID uuid.UUID) error {
	return s.messageRepo.HardDelete(ctx, messageID)
}

func (s *MessageService) Update(ctx context.Context, msg message.Message) error {
	return s.messageRepo.Update(ctx, msg)
}

func (s *MessageService) AddReaction(ctx context.Context, reaction *message.MessageReaction) error {
	return s.messageRepo.AddReaction(ctx, reaction)
}

func (s *MessageService) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, reactionCode string) error {
	return s.messageRepo.RemoveReaction(ctx, messageID, userID, reactionCode)
}

func (s *MessageService) MarkAsRead(ctx context.Context, messageID, userID uuid.UUID) error {
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return err
	}

	if s.db == nil {
		return s.messageRepo.MarkAsRead(ctx, messageID, userID)
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		msgRepo := repository.NewMessageRepository(tx)
		if err := msgRepo.MarkAsRead(ctx, messageID, userID); err != nil {
			return err
		}

		if s.eventPublisher != nil {
			if err := s.eventPublisher.PublishMessageRead(ctx, tx, messageID, msg.ConversationID, userID); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *MessageService) MarkAsDelivered(ctx context.Context, messageID, userID uuid.UUID) error {
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return err
	}

	if s.db == nil {
		return s.messageRepo.MarkAsDelivered(ctx, messageID, userID)
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		msgRepo := repository.NewMessageRepository(tx)
		if err := msgRepo.MarkAsDelivered(ctx, messageID, userID); err != nil {
			return err
		}

		if s.eventPublisher != nil {
			if err := s.eventPublisher.PublishMessageDelivered(ctx, tx, messageID, msg.ConversationID, userID); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *MessageService) MarkAsPlayed(ctx context.Context, messageID, userID uuid.UUID) error {
	return s.messageRepo.MarkAsPlayed(ctx, messageID, userID)
}

func (s *MessageService) GetMessagesBySeqRange(ctx context.Context, conversationID uuid.UUID, startSeq, endSeq int64) ([]message.Message, error) {
	return s.messageRepo.GetMessagesBySeqRange(ctx, conversationID, startSeq, endSeq)
}

func (s *MessageService) GetUnreadMessages(ctx context.Context, conversationID, userID uuid.UUID) ([]message.Message, error) {
	return s.messageRepo.GetUnreadMessages(ctx, conversationID, userID)
}

func (s *MessageService) SearchMessages(ctx context.Context, conversationID uuid.UUID, query string, page, limit int) ([]message.Message, int64, error) {
	return s.messageRepo.SearchMessages(ctx, conversationID, query, page, limit)
}

func (s *MessageService) GetMessagesByType(ctx context.Context, conversationID uuid.UUID, msgType string, limit int) ([]message.Message, error) {
	return s.messageRepo.GetMessagesByType(ctx, conversationID, msgType, limit)
}

func (s *MessageService) GetLatestMessage(ctx context.Context, conversationID uuid.UUID) (message.Message, error) {
	return s.messageRepo.GetLatestMessage(ctx, conversationID)
}

func (s *MessageService) MarkAsEdited(ctx context.Context, messageID uuid.UUID) error {
	return s.messageRepo.MarkAsEdited(ctx, messageID)
}

func (s *MessageService) GetMessageCountSince(ctx context.Context, conversationID uuid.UUID, since time.Time) (int64, error) {
	return s.messageRepo.GetMessageCountSince(ctx, conversationID, since)
}

func (s *MessageService) GetByIdempotencyKey(ctx context.Context, key string) (message.Message, error) {
	return s.messageRepo.GetByIdempotencyKey(ctx, key)
}

func (s *MessageService) GetByClientMessageID(ctx context.Context, clientMsgID string) (message.Message, error) {
	return s.messageRepo.GetByClientMessageID(ctx, clientMsgID)
}

func (s *MessageService) GetMessageReactions(ctx context.Context, messageID uuid.UUID) ([]message.MessageReaction, error) {
	return s.messageRepo.GetMessageReactions(ctx, messageID)
}

func (s *MessageService) GetUserReaction(ctx context.Context, messageID, userID uuid.UUID) (message.MessageReaction, error) {
	return s.messageRepo.GetUserReaction(ctx, messageID, userID)
}

func (s *MessageService) CreateReceipt(ctx context.Context, receipt *message.MessageReceipt) error {
	return s.messageRepo.CreateReceipt(ctx, receipt)
}

func (s *MessageService) UpdateReceipt(ctx context.Context, receipt message.MessageReceipt) error {
	return s.messageRepo.UpdateReceipt(ctx, receipt)
}

func (s *MessageService) GetMessageReceipts(ctx context.Context, messageID uuid.UUID) ([]message.MessageReceipt, error) {
	return s.messageRepo.GetMessageReceipts(ctx, messageID)
}

func (s *MessageService) BulkMarkAsDelivered(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	return s.messageRepo.BulkMarkAsDelivered(ctx, messageIDs, userID)
}

func (s *MessageService) BulkMarkAsRead(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	return s.messageRepo.BulkMarkAsRead(ctx, messageIDs, userID)
}

func (s *MessageService) AddMention(ctx context.Context, mention *message.MessageMention) error {
	return s.messageRepo.AddMention(ctx, mention)
}

func (s *MessageService) GetMessageMentions(ctx context.Context, messageID uuid.UUID) ([]message.MessageMention, error) {
	return s.messageRepo.GetMessageMentions(ctx, messageID)
}

func (s *MessageService) GetUserMentions(ctx context.Context, userID uuid.UUID, page, limit int) ([]message.Message, int64, error) {
	return s.messageRepo.GetUserMentions(ctx, userID, page, limit)
}

func (s *MessageService) StarMessage(ctx context.Context, sMsg *message.StarredMessage) error {
	return s.messageRepo.StarMessage(ctx, sMsg)
}

func (s *MessageService) UnstarMessage(ctx context.Context, userID, messageID uuid.UUID) error {
	return s.messageRepo.UnstarMessage(ctx, userID, messageID)
}

func (s *MessageService) GetUserStarredMessages(ctx context.Context, userID uuid.UUID, page, limit int) ([]message.StarredMessage, int64, error) {
	return s.messageRepo.GetUserStarredMessages(ctx, userID, page, limit)
}

func (s *MessageService) IsMessageStarred(ctx context.Context, userID, messageID uuid.UUID) (bool, error) {
	return s.messageRepo.IsMessageStarred(ctx, userID, messageID)
}

func (s *MessageService) CreateAttachment(ctx context.Context, a *message.Attachment) error {
	return s.messageRepo.CreateAttachment(ctx, a)
}

func (s *MessageService) GetAttachmentByID(ctx context.Context, id uuid.UUID) (message.Attachment, error) {
	return s.messageRepo.GetAttachmentByID(ctx, id)
}

func (s *MessageService) LinkAttachmentToMessage(ctx context.Context, ma *message.MessageAttachment) error {
	return s.messageRepo.LinkAttachmentToMessage(ctx, ma)
}

func (s *MessageService) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]message.Attachment, error) {
	return s.messageRepo.GetMessageAttachments(ctx, messageID)
}

func (s *MessageService) MarkViewOnceViewed(ctx context.Context, attachmentID uuid.UUID) error {
	return s.messageRepo.MarkViewOnceViewed(ctx, attachmentID)
}

func (s *MessageService) CreateLinkPreview(ctx context.Context, lp *message.LinkPreview) error {
	return s.messageRepo.CreateLinkPreview(ctx, lp)
}

func (s *MessageService) GetLinkPreviewByHash(ctx context.Context, urlHash string) (message.LinkPreview, error) {
	return s.messageRepo.GetLinkPreviewByHash(ctx, urlHash)
}

func (s *MessageService) GetLinkPreviewByID(ctx context.Context, id uuid.UUID) (message.LinkPreview, error) {
	return s.messageRepo.GetLinkPreviewByID(ctx, id)
}

func (s *MessageService) CreatePoll(ctx context.Context, p *message.Poll) error {
	return s.messageRepo.CreatePoll(ctx, p)
}

func (s *MessageService) GetPollByID(ctx context.Context, id uuid.UUID) (message.Poll, error) {
	return s.messageRepo.GetPollByID(ctx, id)
}

func (s *MessageService) ClosePoll(ctx context.Context, pollID uuid.UUID) error {
	return s.messageRepo.ClosePoll(ctx, pollID)
}

func (s *MessageService) AddPollOption(ctx context.Context, o *message.PollOption) error {
	return s.messageRepo.AddPollOption(ctx, o)
}

func (s *MessageService) GetPollOptions(ctx context.Context, pollID uuid.UUID) ([]message.PollOption, error) {
	return s.messageRepo.GetPollOptions(ctx, pollID)
}

func (s *MessageService) VotePoll(ctx context.Context, v *message.PollVote) error {
	return s.messageRepo.VotePoll(ctx, v)
}

func (s *MessageService) RemoveVote(ctx context.Context, pollID, optionID, userID uuid.UUID) error {
	return s.messageRepo.RemoveVote(ctx, pollID, optionID, userID)
}

func (s *MessageService) GetPollVotes(ctx context.Context, pollID uuid.UUID) ([]message.PollVote, error) {
	return s.messageRepo.GetPollVotes(ctx, pollID)
}

func (s *MessageService) GetUserVotes(ctx context.Context, pollID, userID uuid.UUID) ([]message.PollVote, error) {
	return s.messageRepo.GetUserVotes(ctx, pollID, userID)
}

func (s *MessageService) DeleteExpiredMessages(ctx context.Context) (int64, error) {
	return s.messageRepo.DeleteExpiredMessages(ctx)
}

func (s *MessageService) executeSendMessage(ctx context.Context, input SendMessageInput) (message.Message, error) {
	if input.ConversationID == uuid.Nil || input.SenderID == uuid.Nil {
		return message.Message{}, sentinal_errors.ErrInvalidInput
	}
	if len(input.Ciphertexts) == 0 {
		return message.Message{}, sentinal_errors.ErrInvalidInput
	}
	for _, payload := range input.Ciphertexts {
		if payload.RecipientDeviceID == uuid.Nil || len(payload.Ciphertext) == 0 {
			return message.Message{}, sentinal_errors.ErrInvalidInput
		}
	}

	if s.conversationRepo != nil {
		ok, err := s.conversationRepo.IsParticipant(ctx, input.ConversationID, input.SenderID)
		if err != nil {
			return message.Message{}, err
		}
		if !ok {
			return message.Message{}, sentinal_errors.ErrForbidden
		}
	}

	if s.db == nil {
		return s.executeSendMessageDirect(ctx, input)
	}

	var result message.Message
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		msgRepo := repository.NewMessageRepository(tx)
		prevMsgRepo := s.messageRepo
		s.messageRepo = msgRepo
		defer func() {
			s.messageRepo = prevMsgRepo
		}()

		res, err := s.executeSendMessageDirect(ctx, input)
		if err != nil {
			return err
		}
		result = res

		// Write to outbox for reliable event delivery
		if s.eventPublisher != nil {
			if err := s.eventPublisher.PublishMessageNew(ctx, tx, res.ID, res.ConversationID, res.SenderID); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return message.Message{}, err
	}
	return result, nil
}

func (s *MessageService) executeSendMessageDirect(ctx context.Context, input SendMessageInput) (message.Message, error) {
	msg := message.Message{
		ID:             uuid.New(),
		ConversationID: input.ConversationID,
		SenderID:       input.SenderID,
		Type:           msgTypeOrDefault(input.MessageType),
		CreatedAt:      time.Now(),
	}
	if input.ClientMsgID != "" {
		msg.ClientMessageID = msgNullString(input.ClientMsgID)
	}
	if input.IdempotencyKey != "" {
		idempotencyKey := input.IdempotencyKey + ":" + input.ConversationID.String()
		msg.IdempotencyKey = msgNullString(idempotencyKey)
	}

	if input.Metadata == nil {
		input.Metadata = map[string]interface{}{}
	}
	input.Metadata["e2ee"] = true

	if err := s.messageRepo.Create(ctx, &msg); err != nil {
		return message.Message{}, err
	}

	deviceID, ok := DeviceIDFromContext(ctx)
	if !ok || !deviceID.Valid {
		return message.Message{}, sentinal_errors.ErrInvalidInput
	}

	input.Metadata["sender_device_id"] = deviceID.UUID.String()
	raw, err := json.Marshal(input.Metadata)
	if err != nil {
		return message.Message{}, err
	}
	msg.Metadata = string(raw)
	if err := s.messageRepo.Update(ctx, msg); err != nil {
		return message.Message{}, err
	}

	for _, payload := range input.Ciphertexts {
		recipientUserID, err := s.lookupUserIDByDevice(ctx, payload.RecipientDeviceID)
		if err != nil {
			return message.Message{}, err
		}
		header := payload.Header
		if header == nil {
			header = map[string]interface{}{"version": 1, "cipher": "signal"}
		}
		headerRaw, _ := json.Marshal(header)
		cipher := &message.MessageCiphertext{
			ID:                uuid.New(),
			MessageID:         msg.ID,
			RecipientUserID:   recipientUserID,
			RecipientDeviceID: payload.RecipientDeviceID,
			SenderDeviceID:    deviceID,
			Ciphertext:        payload.Ciphertext,
			Header:            string(headerRaw),
			CreatedAt:         time.Now(),
		}
		if err := s.messageRepo.CreateCiphertext(ctx, cipher); err != nil {
			return message.Message{}, err
		}
	}

	return msg, nil
}

func (s *MessageService) lookupUserIDByDevice(ctx context.Context, deviceID uuid.UUID) (uuid.UUID, error) {
	if s.db == nil {
		return uuid.Nil, sentinal_errors.ErrInvalidInput
	}
	var row struct {
		UserID uuid.UUID
	}
	if err := s.db.WithContext(ctx).Table("devices").Select("user_id").Where("id = ?", deviceID).Scan(&row).Error; err != nil {
		return uuid.Nil, err
	}
	if row.UserID == uuid.Nil {
		return uuid.Nil, sentinal_errors.ErrNotFound
	}
	return row.UserID, nil
}

func msgTypeOrDefault(value string) string {
	if strings.TrimSpace(value) == "" {
		return "TEXT"
	}
	return value
}

func msgNullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
