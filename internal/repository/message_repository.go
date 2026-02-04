package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"sentinal-chat/internal/domain/message"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresMessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &PostgresMessageRepository{db: db}
}

func (r *PostgresMessageRepository) Create(ctx context.Context, m *message.Message) error {
	res := r.db.WithContext(ctx).Create(m)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) GetByID(ctx context.Context, id uuid.UUID) (message.Message, error) {
	var m message.Message
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return message.Message{}, sentinal_errors.ErrNotFound
		}
		return message.Message{}, err
	}
	return m, nil
}

func (r *PostgresMessageRepository) Update(ctx context.Context, m message.Message) error {
	res := r.db.WithContext(ctx).Save(&m)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresMessageRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&message.Message{}).
		Where("id = ?", id).
		Update("deleted_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresMessageRepository) HardDelete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&message.Message{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresMessageRepository) GetConversationMessages(ctx context.Context, conversationID uuid.UUID, beforeSeq int64, limit int) ([]message.Message, error) {
	var messages []message.Message
	q := r.db.WithContext(ctx).
		Where("conversation_id = ? AND deleted_at IS NULL", conversationID)

	if beforeSeq > 0 {
		q = q.Where("seq_id < ?", beforeSeq)
	}

	err := q.Order("seq_id DESC").Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *PostgresMessageRepository) GetMessagesBySeqRange(ctx context.Context, conversationID uuid.UUID, startSeq, endSeq int64) ([]message.Message, error) {
	var messages []message.Message
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND seq_id >= ? AND seq_id <= ? AND deleted_at IS NULL", conversationID, startSeq, endSeq).
		Order("seq_id ASC").
		Find(&messages).Error
	if err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *PostgresMessageRepository) GetUnreadMessages(ctx context.Context, conversationID, userID uuid.UUID) ([]message.Message, error) {
	var messages []message.Message

	// Get messages that don't have a read receipt from this user
	subQuery := r.db.Model(&message.MessageReceipt{}).
		Select("message_id").
		Where("user_id = ? AND read_at IS NOT NULL", userID)

	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND sender_id != ? AND deleted_at IS NULL AND id NOT IN (?)",
			conversationID, userID, subQuery).
		Order("seq_id ASC").
		Find(&messages).Error

	if err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *PostgresMessageRepository) SearchMessages(ctx context.Context, conversationID uuid.UUID, query string, page, limit int) ([]message.Message, int64, error) {
	var messages []message.Message
	var total int64

	q := r.db.WithContext(ctx).
		Model(&message.Message{}).
		Where("conversation_id = ? AND content ILIKE ? AND deleted_at IS NULL", conversationID, "%"+query+"%")

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&messages).Error; err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

func (r *PostgresMessageRepository) GetMessagesByType(ctx context.Context, conversationID uuid.UUID, msgType string, limit int) ([]message.Message, error) {
	var messages []message.Message
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND type = ? AND deleted_at IS NULL", conversationID, msgType).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *PostgresMessageRepository) GetLatestMessage(ctx context.Context, conversationID uuid.UUID) (message.Message, error) {
	var m message.Message
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND deleted_at IS NULL", conversationID).
		Order("seq_id DESC").
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return message.Message{}, sentinal_errors.ErrNotFound
		}
		return message.Message{}, err
	}
	return m, nil
}

func (r *PostgresMessageRepository) MarkAsEdited(ctx context.Context, messageID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&message.Message{}).
		Where("id = ?", messageID).
		Update("edited_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresMessageRepository) GetMessageCountSince(ctx context.Context, conversationID uuid.UUID, since time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&message.Message{}).
		Where("conversation_id = ? AND created_at > ? AND deleted_at IS NULL", conversationID, since).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *PostgresMessageRepository) GetByIdempotencyKey(ctx context.Context, key string) (message.Message, error) {
	var m message.Message
	err := r.db.WithContext(ctx).
		Where("idempotency_key = ?", key).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return message.Message{}, sentinal_errors.ErrNotFound
		}
		return message.Message{}, err
	}
	return m, nil
}

func (r *PostgresMessageRepository) GetByClientMessageID(ctx context.Context, clientMsgID string) (message.Message, error) {
	var m message.Message
	err := r.db.WithContext(ctx).
		Where("client_message_id = ?", clientMsgID).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return message.Message{}, sentinal_errors.ErrNotFound
		}
		return message.Message{}, err
	}
	return m, nil
}

func (r *PostgresMessageRepository) AddReaction(ctx context.Context, reaction *message.MessageReaction) error {
	res := r.db.WithContext(ctx).Create(reaction)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, reactionCode string) error {
	res := r.db.WithContext(ctx).
		Delete(&message.MessageReaction{}, "message_id = ? AND user_id = ? AND reaction_code = ?", messageID, userID, reactionCode)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresMessageRepository) GetMessageReactions(ctx context.Context, messageID uuid.UUID) ([]message.MessageReaction, error) {
	var reactions []message.MessageReaction
	err := r.db.WithContext(ctx).
		Where("message_id = ?", messageID).
		Find(&reactions).Error
	if err != nil {
		return nil, err
	}
	return reactions, nil
}

func (r *PostgresMessageRepository) GetUserReaction(ctx context.Context, messageID, userID uuid.UUID) (message.MessageReaction, error) {
	var reaction message.MessageReaction
	err := r.db.WithContext(ctx).
		Where("message_id = ? AND user_id = ?", messageID, userID).
		First(&reaction).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return message.MessageReaction{}, sentinal_errors.ErrNotFound
		}
		return message.MessageReaction{}, err
	}
	return reaction, nil
}

func (r *PostgresMessageRepository) CreateReceipt(ctx context.Context, receipt *message.MessageReceipt) error {
	res := r.db.WithContext(ctx).Create(receipt)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) UpdateReceipt(ctx context.Context, receipt message.MessageReceipt) error {
	res := r.db.WithContext(ctx).Save(&receipt)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) GetMessageReceipts(ctx context.Context, messageID uuid.UUID) ([]message.MessageReceipt, error) {
	var receipts []message.MessageReceipt
	err := r.db.WithContext(ctx).
		Where("message_id = ?", messageID).
		Find(&receipts).Error
	if err != nil {
		return nil, err
	}
	return receipts, nil
}

func (r *PostgresMessageRepository) MarkAsDelivered(ctx context.Context, messageID, userID uuid.UUID) error {
	now := time.Now()
	res := r.db.WithContext(ctx).
		Model(&message.MessageReceipt{}).
		Where("message_id = ? AND user_id = ?", messageID, userID).
		Updates(map[string]interface{}{
			"status":       "DELIVERED",
			"delivered_at": now,
			"updated_at":   now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// Create receipt if doesn't exist
		receipt := &message.MessageReceipt{
			MessageID:   messageID,
			UserID:      userID,
			Status:      "DELIVERED",
			DeliveredAt: toNullTime(now),
			UpdatedAt:   now,
		}
		return r.db.WithContext(ctx).Create(receipt).Error
	}
	return nil
}

func (r *PostgresMessageRepository) MarkAsRead(ctx context.Context, messageID, userID uuid.UUID) error {
	now := time.Now()
	res := r.db.WithContext(ctx).
		Model(&message.MessageReceipt{}).
		Where("message_id = ? AND user_id = ?", messageID, userID).
		Updates(map[string]interface{}{
			"status":     "READ",
			"read_at":    now,
			"updated_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		receipt := &message.MessageReceipt{
			MessageID: messageID,
			UserID:    userID,
			Status:    "READ",
			ReadAt:    toNullTime(now),
			UpdatedAt: now,
		}
		return r.db.WithContext(ctx).Create(receipt).Error
	}
	return nil
}

func (r *PostgresMessageRepository) MarkAsPlayed(ctx context.Context, messageID, userID uuid.UUID) error {
	now := time.Now()
	res := r.db.WithContext(ctx).
		Model(&message.MessageReceipt{}).
		Where("message_id = ? AND user_id = ?", messageID, userID).
		Updates(map[string]interface{}{
			"played_at":  now,
			"updated_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) BulkMarkAsDelivered(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, msgID := range messageIDs {
			res := tx.Model(&message.MessageReceipt{}).
				Where("message_id = ? AND user_id = ?", msgID, userID).
				Updates(map[string]interface{}{
					"status":       "DELIVERED",
					"delivered_at": now,
					"updated_at":   now,
				})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				receipt := &message.MessageReceipt{
					MessageID:   msgID,
					UserID:      userID,
					Status:      "DELIVERED",
					DeliveredAt: toNullTime(now),
					UpdatedAt:   now,
				}
				if err := tx.Create(receipt).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *PostgresMessageRepository) BulkMarkAsRead(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, msgID := range messageIDs {
			res := tx.Model(&message.MessageReceipt{}).
				Where("message_id = ? AND user_id = ?", msgID, userID).
				Updates(map[string]interface{}{
					"status":     "READ",
					"read_at":    now,
					"updated_at": now,
				})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				receipt := &message.MessageReceipt{
					MessageID: msgID,
					UserID:    userID,
					Status:    "READ",
					ReadAt:    toNullTime(now),
					UpdatedAt: now,
				}
				if err := tx.Create(receipt).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *PostgresMessageRepository) AddMention(ctx context.Context, m *message.MessageMention) error {
	res := r.db.WithContext(ctx).Create(m)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) GetMessageMentions(ctx context.Context, messageID uuid.UUID) ([]message.MessageMention, error) {
	var mentions []message.MessageMention
	err := r.db.WithContext(ctx).
		Where("message_id = ?", messageID).
		Find(&mentions).Error
	if err != nil {
		return nil, err
	}
	return mentions, nil
}

func (r *PostgresMessageRepository) GetUserMentions(ctx context.Context, userID uuid.UUID, page, limit int) ([]message.Message, int64, error) {
	var messages []message.Message
	var total int64

	subQuery := r.db.Model(&message.MessageMention{}).
		Select("message_id").
		Where("user_id = ?", userID)

	q := r.db.WithContext(ctx).
		Model(&message.Message{}).
		Where("id IN (?) AND deleted_at IS NULL", subQuery)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&messages).Error; err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

func (r *PostgresMessageRepository) StarMessage(ctx context.Context, s *message.StarredMessage) error {
	res := r.db.WithContext(ctx).Create(s)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) UnstarMessage(ctx context.Context, userID, messageID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Delete(&message.StarredMessage{}, "user_id = ? AND message_id = ?", userID, messageID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresMessageRepository) GetUserStarredMessages(ctx context.Context, userID uuid.UUID, page, limit int) ([]message.StarredMessage, int64, error) {
	var starred []message.StarredMessage
	var total int64

	q := r.db.WithContext(ctx).
		Model(&message.StarredMessage{}).
		Where("user_id = ?", userID)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := q.Order("starred_at DESC").Offset(offset).Limit(limit).Find(&starred).Error; err != nil {
		return nil, 0, err
	}

	return starred, total, nil
}

func (r *PostgresMessageRepository) IsMessageStarred(ctx context.Context, userID, messageID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&message.StarredMessage{}).
		Where("user_id = ? AND message_id = ?", userID, messageID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresMessageRepository) CreateAttachment(ctx context.Context, a *message.Attachment) error {
	res := r.db.WithContext(ctx).Create(a)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) GetAttachmentByID(ctx context.Context, id uuid.UUID) (message.Attachment, error) {
	var a message.Attachment
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return message.Attachment{}, sentinal_errors.ErrNotFound
		}
		return message.Attachment{}, err
	}
	return a, nil
}

func (r *PostgresMessageRepository) LinkAttachmentToMessage(ctx context.Context, ma *message.MessageAttachment) error {
	res := r.db.WithContext(ctx).Create(ma)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]message.Attachment, error) {
	var attachments []message.Attachment

	subQuery := r.db.Model(&message.MessageAttachment{}).
		Select("attachment_id").
		Where("message_id = ?", messageID)

	err := r.db.WithContext(ctx).
		Where("id IN (?)", subQuery).
		Find(&attachments).Error

	if err != nil {
		return nil, err
	}
	return attachments, nil
}

func (r *PostgresMessageRepository) MarkViewOnceViewed(ctx context.Context, attachmentID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&message.Attachment{}).
		Where("id = ? AND view_once = true", attachmentID).
		Update("viewed_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresMessageRepository) CreateLinkPreview(ctx context.Context, lp *message.LinkPreview) error {
	res := r.db.WithContext(ctx).Create(lp)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) GetLinkPreviewByHash(ctx context.Context, urlHash string) (message.LinkPreview, error) {
	var lp message.LinkPreview
	err := r.db.WithContext(ctx).Where("url_hash = ?", urlHash).First(&lp).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return message.LinkPreview{}, sentinal_errors.ErrNotFound
		}
		return message.LinkPreview{}, err
	}
	return lp, nil
}

func (r *PostgresMessageRepository) GetLinkPreviewByID(ctx context.Context, id uuid.UUID) (message.LinkPreview, error) {
	var lp message.LinkPreview
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&lp).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return message.LinkPreview{}, sentinal_errors.ErrNotFound
		}
		return message.LinkPreview{}, err
	}
	return lp, nil
}

func (r *PostgresMessageRepository) CreatePoll(ctx context.Context, p *message.Poll) error {
	res := r.db.WithContext(ctx).Create(p)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) GetPollByID(ctx context.Context, id uuid.UUID) (message.Poll, error) {
	var p message.Poll
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return message.Poll{}, sentinal_errors.ErrNotFound
		}
		return message.Poll{}, err
	}
	return p, nil
}

func (r *PostgresMessageRepository) ClosePoll(ctx context.Context, pollID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&message.Poll{}).
		Where("id = ?", pollID).
		Update("closes_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresMessageRepository) AddPollOption(ctx context.Context, o *message.PollOption) error {
	res := r.db.WithContext(ctx).Create(o)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) GetPollOptions(ctx context.Context, pollID uuid.UUID) ([]message.PollOption, error) {
	var options []message.PollOption
	err := r.db.WithContext(ctx).
		Where("poll_id = ?", pollID).
		Order("position ASC").
		Find(&options).Error
	if err != nil {
		return nil, err
	}
	return options, nil
}

func (r *PostgresMessageRepository) VotePoll(ctx context.Context, v *message.PollVote) error {
	res := r.db.WithContext(ctx).Create(v)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresMessageRepository) RemoveVote(ctx context.Context, pollID, optionID, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Delete(&message.PollVote{}, "poll_id = ? AND option_id = ? AND user_id = ?", pollID, optionID, userID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresMessageRepository) GetPollVotes(ctx context.Context, pollID uuid.UUID) ([]message.PollVote, error) {
	var votes []message.PollVote
	err := r.db.WithContext(ctx).
		Where("poll_id = ?", pollID).
		Find(&votes).Error
	if err != nil {
		return nil, err
	}
	return votes, nil
}

func (r *PostgresMessageRepository) GetUserVotes(ctx context.Context, pollID, userID uuid.UUID) ([]message.PollVote, error) {
	var votes []message.PollVote
	err := r.db.WithContext(ctx).
		Where("poll_id = ? AND user_id = ?", pollID, userID).
		Find(&votes).Error
	if err != nil {
		return nil, err
	}
	return votes, nil
}

func (r *PostgresMessageRepository) DeleteExpiredMessages(ctx context.Context) (int64, error) {
	res := r.db.WithContext(ctx).
		Delete(&message.Message{}, "expires_at IS NOT NULL AND expires_at < NOW()")
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

func toNullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}
