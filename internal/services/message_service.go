package services

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"sentinal-chat/internal/domain/command"
	msgdomain "sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/domain/outbox"
	chatproxy "sentinal-chat/internal/proxy"
	"sentinal-chat/internal/repository"
	chatws "sentinal-chat/internal/websocket"
	sentinal_errors "sentinal-chat/pkg/errors"
)

type MessageService struct {
	messages      repository.MessageRepository
	conversations repository.ConversationRepository
	outbox        repository.OutboxRepository
	command       *CommandService
	proxy         *chatproxy.MembershipProxy
}

func NewMessageService(messages repository.MessageRepository, conversations repository.ConversationRepository, outboxRepo repository.OutboxRepository, commandSvc *CommandService) *MessageService {
	return &MessageService{
		messages:      messages,
		conversations: conversations,
		outbox:        outboxRepo,
		command:       commandSvc,
		proxy:         chatproxy.NewMembershipProxy(conversations),
	}
}

func (s *MessageService) Send(ctx context.Context, in SendMessageInput) (MessageView, error) {
	if s == nil || s.messages == nil || s.conversations == nil {
		return MessageView{}, sentinal_errors.ErrServiceUnavailable
	}
	if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.SenderID); err != nil {
		return MessageView{}, err
	}
	if strings.TrimSpace(in.Type) == "" || strings.TrimSpace(in.EncryptedContent) == "" {
		return MessageView{}, sentinal_errors.ErrInvalidInput
	}
	if in.ClientMessageID != "" {
		existing, err := s.messages.GetByClientMessageID(ctx, in.ConversationID, in.ClientMessageID)
		if err == nil {
			return s.GetByID(ctx, existing.ID, in.SenderID)
		}
		if !errors.Is(err, sentinal_errors.ErrNotFound) {
			return MessageView{}, err
		}
	}

	now := time.Now().UTC()
	msg := msgdomain.Message{
		ID:               uuid.New(),
		ConversationID:   in.ConversationID,
		SenderID:         in.SenderID,
		ClientMessageID:  chatNullableString(in.ClientMessageID),
		Type:             strings.ToUpper(strings.TrimSpace(in.Type)),
		EncryptedContent: chatNullableString(strings.TrimSpace(in.EncryptedContent)),
		CreatedAt:        now,
		ExpiresAt:        chatNullableTimePtr(in.ExpiresAt),
	}
	if in.ReplyToMsgID != nil {
		msg.ReplyToMsgID = chatNullableUUID(*in.ReplyToMsgID)
	}
	if err := s.messages.Create(ctx, &msg); err != nil {
		return MessageView{}, err
	}
	stored, err := s.messages.GetByID(ctx, msg.ID)
	if err != nil {
		return MessageView{}, err
	}

	for _, attachmentID := range chatDedupeUUIDs(in.AttachmentIDs) {
		if err := s.messages.LinkAttachmentToMessage(ctx, &msgdomain.MessageAttachment{MessageID: stored.ID, AttachmentID: attachmentID}); err != nil && !errors.Is(err, sentinal_errors.ErrAlreadyExists) {
			return MessageView{}, err
		}
	}
	for _, mentioned := range chatDedupeUUIDs(in.MentionUserIDs) {
		_ = s.messages.AddMention(ctx, &msgdomain.MessageMention{MessageID: stored.ID, UserID: mentioned})
	}

	var pollView *PollView
	if in.Poll != nil {
		poll, err := s.createPoll(ctx, stored.ID, in.SenderID, *in.Poll)
		if err != nil {
			return MessageView{}, err
		}
		pollView = &poll
	}

	view, err := s.GetByID(ctx, stored.ID, in.SenderID)
	if err != nil {
		return MessageView{}, err
	}
	if pollView != nil {
		view.Poll = pollView
	}

	if s.outbox != nil {
		envelope := chatws.NewMessageEvent("message:new", in.ConversationID, map[string]any{
			"message": view,
		})
		if event, err := chatws.NewOutboxEvent("message:new", outbox.AggregateMessage, stored.ID, envelope); err == nil {
			_ = s.outbox.Create(ctx, nil, event)
		}
	}
	return view, nil
}

func (s *MessageService) Edit(ctx context.Context, in EditMessageInput) (MessageView, error) {
	if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.EditorID); err != nil {
		return MessageView{}, err
	}
	msg, err := s.messages.GetByID(ctx, in.MessageID)
	if err != nil {
		return MessageView{}, err
	}
	if msg.ConversationID != in.ConversationID || msg.SenderID != in.EditorID {
		return MessageView{}, sentinal_errors.ErrForbidden
	}
	oldContent := msg.EncryptedContent.String
	version := 1
	if edits, editErr := s.messages.GetMessageEdits(ctx, in.MessageID); editErr == nil {
		version = len(edits) + 1
	}
	if oldContent != "" {
		_ = s.messages.CreateMessageEdit(ctx, &msgdomain.MessageEdit{
			ID:               uuid.New(),
			MessageID:        msg.ID,
			EncryptedContent: oldContent,
			EditedBy:         in.EditorID,
			EditedAt:         time.Now().UTC(),
			VersionNumber:    version,
		})
	}
	msg.EncryptedContent = chatNullableString(strings.TrimSpace(in.EncryptedContent))
	msg.EditedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	msg.ExpiresAt = chatNullableTimePtr(in.ExpiresAt)
	if err := s.messages.Update(ctx, msg); err != nil {
		return MessageView{}, err
	}
	if s.command != nil {
		_, _ = s.command.Record(ctx, command.CommandEditMessage, in.EditorID, &in.ConversationID, map[string]any{
			"message_id": in.MessageID.String(),
		}, map[string]any{"encrypted_content": oldContent})
	}
	view, err := s.GetByID(ctx, in.MessageID, in.EditorID)
	if err != nil {
		return MessageView{}, err
	}
	if s.outbox != nil {
		envelope := chatws.NewMessageEvent("message:edited", in.ConversationID, map[string]any{"message": view})
		if event, err := chatws.NewOutboxEvent("message:edited", outbox.AggregateMessage, in.MessageID, envelope); err == nil {
			_ = s.outbox.Create(ctx, nil, event)
		}
	}
	return view, nil
}

func (s *MessageService) Delete(ctx context.Context, in DeleteMessageInput) (MessageView, error) {
	if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.ActorID); err != nil {
		return MessageView{}, err
	}
	msg, err := s.messages.GetByID(ctx, in.MessageID)
	if err != nil {
		return MessageView{}, err
	}
	if msg.ConversationID != in.ConversationID || msg.SenderID != in.ActorID {
		return MessageView{}, sentinal_errors.ErrForbidden
	}
	if err := s.messages.SoftDelete(ctx, in.MessageID); err != nil {
		return MessageView{}, err
	}
	if s.command != nil {
		_, _ = s.command.Record(ctx, command.CommandDeleteMessage, in.ActorID, &in.ConversationID, map[string]any{
			"message_id": in.MessageID.String(),
		}, nil)
	}
	view, err := s.GetByID(ctx, in.MessageID, in.ActorID)
	if err != nil {
		return MessageView{}, err
	}
	if s.outbox != nil {
		envelope := chatws.NewMessageEvent("message:deleted", in.ConversationID, map[string]any{"message": view})
		if event, err := chatws.NewOutboxEvent("message:deleted", outbox.AggregateMessage, in.MessageID, envelope); err == nil {
			_ = s.outbox.Create(ctx, nil, event)
		}
	}
	return view, nil
}

func (s *MessageService) AddReaction(ctx context.Context, in ReactionInput) ([]ReactionView, error) {
	if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.ActorID); err != nil {
		return nil, err
	}
	reaction := &msgdomain.MessageReaction{
		ID:           uuid.New(),
		MessageID:    in.MessageID,
		UserID:       in.ActorID,
		ReactionCode: strings.TrimSpace(in.Code),
		CreatedAt:    time.Now().UTC(),
	}
	if reaction.ReactionCode == "" {
		return nil, sentinal_errors.ErrInvalidInput
	}
	if err := s.messages.AddReaction(ctx, reaction); err != nil {
		return nil, err
	}
	return s.listReactions(ctx, in.MessageID)
}

func (s *MessageService) RemoveReaction(ctx context.Context, in ReactionInput) ([]ReactionView, error) {
	if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.ActorID); err != nil {
		return nil, err
	}
	if err := s.messages.RemoveReaction(ctx, in.MessageID, in.ActorID, strings.TrimSpace(in.Code)); err != nil {
		return nil, err
	}
	return s.listReactions(ctx, in.MessageID)
}

func (s *MessageService) Pin(ctx context.Context, in PinMessageInput) (bool, error) {
	if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.ActorID); err != nil {
		return false, err
	}
	if in.Pinned {
		if err := s.messages.PinMessage(ctx, &msgdomain.PinnedMessage{ConversationID: in.ConversationID, MessageID: in.MessageID, PinnedBy: in.ActorID, PinnedAt: time.Now().UTC()}); err != nil {
			return false, err
		}
		if s.command != nil {
			_, _ = s.command.Record(ctx, command.CommandPinMessage, in.ActorID, &in.ConversationID, map[string]any{"message_id": in.MessageID.String()}, nil)
		}
	} else {
		if err := s.messages.UnpinMessage(ctx, in.ConversationID, in.MessageID); err != nil {
			return false, err
		}
		if s.command != nil {
			_, _ = s.command.Record(ctx, command.CommandUnpinMessage, in.ActorID, &in.ConversationID, map[string]any{"message_id": in.MessageID.String()}, nil)
		}
	}
	return in.Pinned, nil
}

func (s *MessageService) UpdateReceipt(ctx context.Context, in ReceiptInput) error {
	if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.ActorID); err != nil {
		return err
	}
	ids := chatDedupeUUIDs(in.MessageIDs)
	if len(ids) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	switch strings.ToUpper(strings.TrimSpace(in.Status)) {
	case "DELIVERED":
		if len(ids) == 1 {
			return s.messages.MarkAsDelivered(ctx, ids[0], in.ActorID)
		}
		return s.messages.BulkMarkAsDelivered(ctx, ids, in.ActorID)
	case "READ":
		if len(ids) == 1 {
			if err := s.messages.MarkAsRead(ctx, ids[0], in.ActorID); err != nil {
				return err
			}
		} else if err := s.messages.BulkMarkAsRead(ctx, ids, in.ActorID); err != nil {
			return err
		}
		if in.UpToSeqID != nil {
			return s.conversations.UpdateLastReadSequence(ctx, in.ConversationID, in.ActorID, *in.UpToSeqID)
		}
		return nil
	case "PLAYED":
		if len(ids) != 1 {
			return sentinal_errors.ErrInvalidInput
		}
		return s.messages.MarkAsPlayed(ctx, ids[0], in.ActorID)
	default:
		return sentinal_errors.ErrInvalidInput
	}
}

func (s *MessageService) History(ctx context.Context, conversationID, userID uuid.UUID, beforeSeq int64, limit int) ([]MessageView, error) {
	if err := s.proxy.RequireParticipant(ctx, conversationID, userID); err != nil {
		return nil, err
	}
	messages, err := s.messages.GetConversationMessages(ctx, conversationID, beforeSeq, chatNormalizeLimit(limit, 50))
	if err != nil {
		return nil, err
	}
	items := make([]MessageView, 0, len(messages))
	for _, msg := range messages {
		view, buildErr := s.buildMessageView(ctx, msg, userID)
		if buildErr != nil {
			return nil, buildErr
		}
		items = append(items, view)
	}
	return items, nil
}

func (s *MessageService) GetByID(ctx context.Context, messageID, userID uuid.UUID) (MessageView, error) {
	msg, err := s.messages.GetByID(ctx, messageID)
	if err != nil {
		return MessageView{}, err
	}
	if err := s.proxy.RequireParticipant(ctx, msg.ConversationID, userID); err != nil {
		return MessageView{}, err
	}
	return s.buildMessageView(ctx, msg, userID)
}

func (s *MessageService) getLatestView(ctx context.Context, conversationID, userID uuid.UUID) (MessageView, error) {
	msg, err := s.messages.GetLatestMessage(ctx, conversationID)
	if err != nil {
		return MessageView{}, err
	}
	return s.buildMessageView(ctx, msg, userID)
}

func (s *MessageService) unreadCountSince(ctx context.Context, conversationID uuid.UUID, lastReadSeq int64) (int64, error) {
	messages, err := s.messages.GetMessagesBySeqRange(ctx, conversationID, lastReadSeq+1, 1<<62-1)
	if err != nil {
		return 0, err
	}
	return int64(len(messages)), nil
}

func (s *MessageService) buildMessageView(ctx context.Context, msg msgdomain.Message, userID uuid.UUID) (MessageView, error) {
	attachments, err := s.messages.GetMessageAttachments(ctx, msg.ID)
	if err != nil {
		return MessageView{}, err
	}
	receipts, err := s.messages.GetMessageReceipts(ctx, msg.ID)
	if err != nil {
		return MessageView{}, err
	}
	reactions, err := s.messages.GetMessageReactions(ctx, msg.ID)
	if err != nil {
		return MessageView{}, err
	}
	pinnedMessages, err := s.messages.GetPinnedMessages(ctx, msg.ConversationID)
	if err != nil {
		return MessageView{}, err
	}
	pinned := false
	for _, item := range pinnedMessages {
		if item.MessageID == msg.ID {
			pinned = true
			break
		}
	}
	starred, _ := s.messages.IsMessageStarred(ctx, userID, msg.ID)
	var pollView *PollView
	if msg.PollID.Valid {
		view, pollErr := s.loadPollView(ctx, msg.PollID.UUID, userID)
		if pollErr == nil {
			pollView = &view
		}
	}
	return messageToView(msg, attachments, receipts, reactions, pollView, pinned, starred), nil
}

func (s *MessageService) createPoll(ctx context.Context, messageID, userID uuid.UUID, input CreatePollInput) (PollView, error) {
	_ = userID
	if strings.TrimSpace(input.Question) == "" || len(input.Options) < 2 {
		return PollView{}, sentinal_errors.ErrInvalidInput
	}
	poll := &msgdomain.Poll{
		ID:             uuid.New(),
		MessageID:      chatNullableUUID(messageID),
		Question:       strings.TrimSpace(input.Question),
		AllowsMultiple: input.AllowsMultiple,
		CreatedAt:      time.Now().UTC(),
	}
	if input.ClosesAt != nil {
		poll.ClosesAt = chatNullableTimePtr(input.ClosesAt)
	}
	if err := s.messages.CreatePoll(ctx, poll); err != nil {
		return PollView{}, err
	}
	options := make([]PollOptionView, 0, len(input.Options))
	for idx, optionText := range input.Options {
		option := &msgdomain.PollOption{ID: uuid.New(), PollID: poll.ID, OptionText: strings.TrimSpace(optionText), Position: idx + 1}
		if option.OptionText == "" {
			return PollView{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.messages.AddPollOption(ctx, option); err != nil {
			return PollView{}, err
		}
		options = append(options, PollOptionView{ID: option.ID.String(), Text: option.OptionText, Position: option.Position, Votes: 0})
	}
	return PollView{
		ID:             poll.ID.String(),
		Question:       poll.Question,
		AllowsMultiple: poll.AllowsMultiple,
		ClosesAt:       input.ClosesAt,
		Closed:         false,
		Options:        options,
	}, nil
}

func (s *MessageService) VotePoll(ctx context.Context, in VotePollInput) (PollView, error) {
	if err := s.proxy.RequireParticipant(ctx, in.ConversationID, in.ActorID); err != nil {
		return PollView{}, err
	}
	poll, err := s.messages.GetPollByID(ctx, in.PollID)
	if err != nil {
		return PollView{}, err
	}
	if poll.ClosesAt.Valid && poll.ClosesAt.Time.Before(time.Now()) {
		return PollView{}, sentinal_errors.ErrConflict
	}
	if !poll.AllowsMultiple && len(in.OptionIDs) > 1 {
		return PollView{}, sentinal_errors.ErrInvalidInput
	}
	existingVotes, err := s.messages.GetUserVotes(ctx, in.PollID, in.ActorID)
	if err == nil {
		for _, vote := range existingVotes {
			_ = s.messages.RemoveVote(ctx, in.PollID, vote.OptionID, in.ActorID)
		}
	}
	for _, optionID := range chatDedupeUUIDs(in.OptionIDs) {
		if err := s.messages.VotePoll(ctx, &msgdomain.PollVote{PollID: in.PollID, OptionID: optionID, UserID: in.ActorID, VotedAt: time.Now().UTC()}); err != nil {
			return PollView{}, err
		}
	}
	view, err := s.loadPollView(ctx, in.PollID, in.ActorID)
	if err != nil {
		return PollView{}, err
	}
	if s.outbox != nil {
		envelope := chatws.NewConversationEvent("poll:update", in.ConversationID, map[string]any{"poll": view})
		if event, err := chatws.NewOutboxEvent("poll:update", outbox.AggregatePoll, in.PollID, envelope); err == nil {
			_ = s.outbox.Create(ctx, nil, event)
		}
	}
	return view, nil
}

func (s *MessageService) ClosePoll(ctx context.Context, conversationID, pollID, actorID uuid.UUID) (PollView, error) {
	if err := s.proxy.RequireParticipant(ctx, conversationID, actorID); err != nil {
		return PollView{}, err
	}
	if err := s.messages.ClosePoll(ctx, pollID); err != nil {
		return PollView{}, err
	}
	view, err := s.loadPollView(ctx, pollID, actorID)
	if err != nil {
		return PollView{}, err
	}
	if s.outbox != nil {
		envelope := chatws.NewConversationEvent("poll:update", conversationID, map[string]any{"poll": view})
		if event, err := chatws.NewOutboxEvent("poll:update", outbox.AggregatePoll, pollID, envelope); err == nil {
			_ = s.outbox.Create(ctx, nil, event)
		}
	}
	return view, nil
}

func (s *MessageService) loadPollView(ctx context.Context, pollID, actorID uuid.UUID) (PollView, error) {
	poll, err := s.messages.GetPollByID(ctx, pollID)
	if err != nil {
		return PollView{}, err
	}
	options, err := s.messages.GetPollOptions(ctx, pollID)
	if err != nil {
		return PollView{}, err
	}
	votes, err := s.messages.GetPollVotes(ctx, pollID)
	if err != nil {
		return PollView{}, err
	}
	voteCount := make(map[uuid.UUID]int)
	myVotes := make([]string, 0)
	for _, vote := range votes {
		voteCount[vote.OptionID]++
		if vote.UserID == actorID {
			myVotes = append(myVotes, vote.OptionID.String())
		}
	}
	optionViews := make([]PollOptionView, 0, len(options))
	for _, option := range options {
		optionViews = append(optionViews, PollOptionView{ID: option.ID.String(), Text: option.OptionText, Position: option.Position, Votes: voteCount[option.ID]})
	}
	return PollView{
		ID:             poll.ID.String(),
		Question:       poll.Question,
		AllowsMultiple: poll.AllowsMultiple,
		ClosesAt:       chatNullTime(poll.ClosesAt),
		Closed:         poll.ClosesAt.Valid && poll.ClosesAt.Time.Before(time.Now()),
		Options:        optionViews,
		MyVotes:        myVotes,
	}, nil
}

func (s *MessageService) listReactions(ctx context.Context, messageID uuid.UUID) ([]ReactionView, error) {
	reactions, err := s.messages.GetMessageReactions(ctx, messageID)
	if err != nil {
		return nil, err
	}
	items := make([]ReactionView, 0, len(reactions))
	for _, reaction := range reactions {
		items = append(items, ReactionView{UserID: reaction.UserID.String(), ReactionCode: reaction.ReactionCode, CreatedAt: reaction.CreatedAt})
	}
	return items, nil
}
