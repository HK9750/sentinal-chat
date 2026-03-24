package services

import (
	"database/sql"
	"time"

	convdomain "sentinal-chat/internal/domain/conversation"
	msgdomain "sentinal-chat/internal/domain/message"
)

func conversationToView(conv convdomain.Conversation, lastMessage *MessageView, unreadCount int64, lastReadSequence int64) ConversationView {
	participants := make([]ParticipantView, 0, len(conv.Participants))
	for _, participant := range conv.Participants {
		participants = append(participants, participantToView(participant))
	}

	var subject *string
	if conv.Subject.Valid {
		subject = chatStringPtr(conv.Subject.String)
	}
	var description *string
	if conv.Description.Valid {
		description = chatStringPtr(conv.Description.String)
	}
	var avatarURL *string
	if conv.AvatarURL.Valid {
		avatarURL = chatStringPtr(conv.AvatarURL.String)
	}
	var createdBy *string
	if conv.CreatedBy.Valid {
		createdBy = chatStringPtr(conv.CreatedBy.UUID.String())
	}

	view := ConversationView{
		ID:               conv.ID.String(),
		Type:             conv.Type,
		Subject:          subject,
		Description:      description,
		AvatarURL:        avatarURL,
		DisappearingMode: conv.DisappearingMode,
		CreatedBy:        createdBy,
		CreatedAt:        conv.CreatedAt,
		UpdatedAt:        conv.UpdatedAt,
		LastMessageAt:    conv.LastMessageAt,
		Participants:     participants,
		UnreadCount:      unreadCount,
		LastReadSequence: lastReadSequence,
	}
	if lastMessage != nil {
		var content *string
		if lastMessage.Content != nil && *lastMessage.Content != "" {
			content = lastMessage.Content
		}

		var mimeType *string
		var filename *string
		var durationSeconds *int32
		if len(lastMessage.Attachments) > 0 {
			firstAttachment := lastMessage.Attachments[0]
			if firstAttachment.MimeType != "" {
				mimeType = chatStringPtr(firstAttachment.MimeType)
			}
			if firstAttachment.Filename != nil && *firstAttachment.Filename != "" {
				filename = firstAttachment.Filename
			}
			if firstAttachment.DurationSeconds != nil {
				durationSeconds = firstAttachment.DurationSeconds
			}
		}

		view.LastMessage = &MessageSummaryView{
			ID:                 lastMessage.ID,
			SenderID:           lastMessage.SenderID,
			Kind:               lastMessage.Type,
			Content:            content,
			AttachmentMimeType: mimeType,
			AttachmentFilename: filename,
			DurationSeconds:    durationSeconds,
			CreatedAt:          lastMessage.CreatedAt,
			SeqID:              lastMessage.SeqID,
			ReceiptStatus:      latestReceiptStatus(lastMessage.Receipts, lastMessage.SenderID),
			DeletedAt:          lastMessage.DeletedAt,
		}
	}
	return view
}

func participantToView(participant convdomain.Participant) ParticipantView {
	var mutedUntil *time.Time
	if participant.MutedUntil.Valid {
		mutedUntil = chatTimePtr(participant.MutedUntil.Time)
	}
	return ParticipantView{
		UserID:      participant.UserID.String(),
		DisplayName: participant.DisplayName,
		Username:    participant.Username,
		AvatarURL:   participant.AvatarURL,
		Role:        participant.Role,
		JoinedAt:    participant.JoinedAt,
		MutedUntil:  mutedUntil,
		Archived:    participant.Archived,
		IsOnline:    participant.IsOnline,
		LastReadSeq: participant.LastReadSequence,
	}
}

func messageToView(msg msgdomain.Message, attachments []msgdomain.Attachment, receipts []msgdomain.MessageReceipt, reactions []msgdomain.MessageReaction, poll *PollView, pinned, starred bool) MessageView {
	attachmentViews := make([]AttachmentView, 0, len(attachments))
	for _, attachment := range attachments {
		attachmentViews = append(attachmentViews, attachmentToView(attachment))
	}

	receiptViews := make([]ReceiptView, 0, len(receipts))
	for _, receipt := range receipts {
		receiptViews = append(receiptViews, receiptToView(receipt))
	}

	reactionViews := make([]ReactionView, 0, len(reactions))
	for _, reaction := range reactions {
		reactionViews = append(reactionViews, ReactionView{
			UserID:       reaction.UserID.String(),
			ReactionCode: reaction.ReactionCode,
			CreatedAt:    reaction.CreatedAt,
		})
	}

	var clientMessageID *string
	if msg.ClientMessageID.Valid {
		clientMessageID = chatStringPtr(msg.ClientMessageID.String)
	}
	var content *string
	if msg.Content.Valid {
		content = chatStringPtr(msg.Content.String)
	}
	var replyTo *string
	if msg.ReplyToMsgID.Valid {
		replyTo = chatStringPtr(msg.ReplyToMsgID.UUID.String())
	}

	return MessageView{
		ID:              msg.ID.String(),
		ConversationID:  msg.ConversationID.String(),
		SenderID:        msg.SenderID.String(),
		ClientMessageID: clientMessageID,
		SeqID:           chatNullInt64(msg.SeqID),
		Type:            msg.Type,
		Content:         content,
		IsForwarded:     msg.IsForwarded,
		ReplyToMsgID:    replyTo,
		MentionCount:    msg.MentionCount,
		CreatedAt:       msg.CreatedAt,
		EditedAt:        chatNullTime(msg.EditedAt),
		DeletedAt:       chatNullTime(msg.DeletedAt),
		ExpiresAt:       chatNullTime(msg.ExpiresAt),
		Attachments:     attachmentViews,
		Receipts:        receiptViews,
		Reactions:       reactionViews,
		Poll:            poll,
		Pinned:          pinned,
		IsStarred:       starred,
	}
}

func attachmentToView(attachment msgdomain.Attachment) AttachmentView {
	var filename *string
	if attachment.Filename.Valid {
		filename = chatStringPtr(attachment.Filename.String)
	}
	var thumbnailURL *string
	if attachment.ThumbnailURL.Valid {
		thumbnailURL = chatStringPtr(attachment.ThumbnailURL.String)
	}
	var width *int32
	if attachment.Width.Valid {
		value := attachment.Width.Int32
		width = &value
	}
	var height *int32
	if attachment.Height.Valid {
		value := attachment.Height.Int32
		height = &value
	}
	var duration *int32
	if attachment.DurationSeconds.Valid {
		value := attachment.DurationSeconds.Int32
		duration = &value
	}

	return AttachmentView{
		ID:              attachment.ID.String(),
		FileURL:         attachment.FileURL,
		Filename:        filename,
		MimeType:        attachment.MimeType,
		SizeBytes:       attachment.SizeBytes,
		ThumbnailURL:    thumbnailURL,
		Width:           width,
		Height:          height,
		DurationSeconds: duration,
		ViewOnce:        attachment.ViewOnce,
		ViewedAt:        chatNullTime(attachment.ViewedAt),
	}
}

func receiptToView(receipt msgdomain.MessageReceipt) ReceiptView {
	return ReceiptView{
		UserID:      receipt.UserID.String(),
		Status:      receipt.Status,
		DeliveredAt: chatNullTime(receipt.DeliveredAt),
		ReadAt:      chatNullTime(receipt.ReadAt),
		PlayedAt:    chatNullTime(receipt.PlayedAt),
		UpdatedAt:   receipt.UpdatedAt,
	}
}

func latestReceiptStatus(receipts []ReceiptView, senderID string) *string {
	status := "SENT"
	for _, receipt := range receipts {
		if receipt.UserID == senderID {
			continue
		}
		switch receipt.Status {
		case "PLAYED":
			status = "PLAYED"
		case "READ":
			if status != "PLAYED" {
				status = "READ"
			}
		case "DELIVERED":
			if status == "SENT" {
				status = "DELIVERED"
			}
		}
	}
	return chatStringPtr(status)
}

func chatNullTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	copy := value.Time
	return &copy
}

func chatNullInt64(value sql.NullInt64) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func chatStringPtr(value string) *string {
	copy := value
	return &copy
}

func chatTimePtr(value time.Time) *time.Time {
	copy := value
	return &copy
}
