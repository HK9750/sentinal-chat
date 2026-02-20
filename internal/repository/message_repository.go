package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"sentinal-chat/internal/domain/message"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type PostgresMessageRepository struct {
	db DBTX
}

func NewMessageRepository(db DBTX) MessageRepository {
	return &PostgresMessageRepository{db: db}
}

func (r *PostgresMessageRepository) Create(ctx context.Context, m *message.Message) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO messages (
            id, conversation_id, sender_id, client_message_id, idempotency_key, seq_id, type, metadata,
            is_forwarded, forwarded_from_msg_id, reply_to_msg_id, poll_id, link_preview_id, mention_count,
            created_at, edited_at, deleted_at, expires_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
    `,
		m.ID,
		m.ConversationID,
		m.SenderID,
		m.ClientMessageID,
		m.IdempotencyKey,
		m.SeqID,
		m.Type,
		m.Metadata,
		m.IsForwarded,
		m.ForwardedFromMsgID,
		m.ReplyToMsgID,
		m.PollID,
		m.LinkPreviewID,
		m.MentionCount,
		m.CreatedAt,
		m.EditedAt,
		m.DeletedAt,
		m.ExpiresAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresMessageRepository) CreateCiphertext(ctx context.Context, c *message.MessageCiphertext) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO message_ciphertexts (id, message_id, recipient_user_id, recipient_device_id, sender_device_id, ciphertext, header, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `,
		c.ID,
		c.MessageID,
		c.RecipientUserID,
		c.RecipientDeviceID,
		c.SenderDeviceID,
		c.Ciphertext,
		c.Header,
		c.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresMessageRepository) GetByID(ctx context.Context, id uuid.UUID) (message.Message, error) {
	var m message.Message
	err := r.db.QueryRowContext(ctx, `
        SELECT id, conversation_id, sender_id, client_message_id, idempotency_key, seq_id, type, metadata,
               is_forwarded, forwarded_from_msg_id, reply_to_msg_id, poll_id, link_preview_id, mention_count,
               created_at, edited_at, deleted_at, expires_at
        FROM messages WHERE id = $1
    `, id).Scan(
		&m.ID,
		&m.ConversationID,
		&m.SenderID,
		&m.ClientMessageID,
		&m.IdempotencyKey,
		&m.SeqID,
		&m.Type,
		&m.Metadata,
		&m.IsForwarded,
		&m.ForwardedFromMsgID,
		&m.ReplyToMsgID,
		&m.PollID,
		&m.LinkPreviewID,
		&m.MentionCount,
		&m.CreatedAt,
		&m.EditedAt,
		&m.DeletedAt,
		&m.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return message.Message{}, sentinal_errors.ErrNotFound
		}
		return message.Message{}, err
	}
	return m, nil
}

func (r *PostgresMessageRepository) Update(ctx context.Context, m message.Message) error {
	res, err := r.db.ExecContext(ctx, `
        UPDATE messages
        SET conversation_id = $1, sender_id = $2, client_message_id = $3, idempotency_key = $4, seq_id = $5,
            type = $6, metadata = $7, is_forwarded = $8, forwarded_from_msg_id = $9, reply_to_msg_id = $10,
            poll_id = $11, link_preview_id = $12, mention_count = $13, edited_at = $14, deleted_at = $15, expires_at = $16
        WHERE id = $17
    `,
		m.ConversationID,
		m.SenderID,
		m.ClientMessageID,
		m.IdempotencyKey,
		m.SeqID,
		m.Type,
		m.Metadata,
		m.IsForwarded,
		m.ForwardedFromMsgID,
		m.ReplyToMsgID,
		m.PollID,
		m.LinkPreviewID,
		m.MentionCount,
		m.EditedAt,
		m.DeletedAt,
		m.ExpiresAt,
		m.ID,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresMessageRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE messages SET deleted_at = $1 WHERE id = $2", time.Now(), id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresMessageRepository) HardDelete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM messages WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresMessageRepository) GetConversationMessages(ctx context.Context, conversationID uuid.UUID, beforeSeq int64, limit int, recipientDeviceID uuid.UUID) ([]message.Message, error) {
	var messages []message.Message

	query := `
        SELECT m.id, m.conversation_id, m.sender_id, m.client_message_id, m.idempotency_key, m.seq_id, m.type, m.metadata,
               m.is_forwarded, m.forwarded_from_msg_id, m.reply_to_msg_id, m.poll_id, m.link_preview_id, m.mention_count,
               m.created_at, m.edited_at, m.deleted_at, m.expires_at,
               mc.ciphertext, mc.header, mc.recipient_device_id, mc.recipient_user_id, mc.sender_device_id
        FROM messages m
        JOIN message_ciphertexts mc ON mc.message_id = m.id
        WHERE m.conversation_id = $1 AND m.deleted_at IS NULL AND mc.recipient_device_id = $2
    `

	args := []interface{}{conversationID, recipientDeviceID}
	if beforeSeq > 0 {
		query += " AND m.seq_id < $3"
		args = append(args, beforeSeq)
	}
	query += fmt.Sprintf(" ORDER BY m.seq_id DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var m message.Message
		if err := rows.Scan(
			&m.ID,
			&m.ConversationID,
			&m.SenderID,
			&m.ClientMessageID,
			&m.IdempotencyKey,
			&m.SeqID,
			&m.Type,
			&m.Metadata,
			&m.IsForwarded,
			&m.ForwardedFromMsgID,
			&m.ReplyToMsgID,
			&m.PollID,
			&m.LinkPreviewID,
			&m.MentionCount,
			&m.CreatedAt,
			&m.EditedAt,
			&m.DeletedAt,
			&m.ExpiresAt,
			&m.Ciphertext,
			&m.Header,
			&m.RecipientDeviceID,
			&m.RecipientUserID,
			&m.SenderDeviceID,
		); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *PostgresMessageRepository) GetMessagesBySeqRange(ctx context.Context, conversationID uuid.UUID, startSeq, endSeq int64) ([]message.Message, error) {
	var messages []message.Message
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, conversation_id, sender_id, client_message_id, idempotency_key, seq_id, type, metadata,
               is_forwarded, forwarded_from_msg_id, reply_to_msg_id, poll_id, link_preview_id, mention_count,
               created_at, edited_at, deleted_at, expires_at
        FROM messages
        WHERE conversation_id = $1 AND seq_id >= $2 AND seq_id <= $3 AND deleted_at IS NULL
        ORDER BY seq_id ASC
    `, conversationID, startSeq, endSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var m message.Message
		if err := rows.Scan(
			&m.ID,
			&m.ConversationID,
			&m.SenderID,
			&m.ClientMessageID,
			&m.IdempotencyKey,
			&m.SeqID,
			&m.Type,
			&m.Metadata,
			&m.IsForwarded,
			&m.ForwardedFromMsgID,
			&m.ReplyToMsgID,
			&m.PollID,
			&m.LinkPreviewID,
			&m.MentionCount,
			&m.CreatedAt,
			&m.EditedAt,
			&m.DeletedAt,
			&m.ExpiresAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *PostgresMessageRepository) GetUnreadMessages(ctx context.Context, conversationID, userID uuid.UUID) ([]message.Message, error) {
	var messages []message.Message
	rows, err := r.db.QueryContext(ctx, `
        SELECT m.id, m.conversation_id, m.sender_id, m.client_message_id, m.idempotency_key, m.seq_id, m.type, m.metadata,
               m.is_forwarded, m.forwarded_from_msg_id, m.reply_to_msg_id, m.poll_id, m.link_preview_id, m.mention_count,
               m.created_at, m.edited_at, m.deleted_at, m.expires_at
        FROM messages m
        WHERE m.conversation_id = $1 AND m.sender_id != $2 AND m.deleted_at IS NULL AND NOT EXISTS (
            SELECT 1 FROM message_receipts r WHERE r.message_id = m.id AND r.user_id = $2 AND r.read_at IS NOT NULL
        )
        ORDER BY m.seq_id ASC
    `, conversationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var m message.Message
		if err := rows.Scan(
			&m.ID,
			&m.ConversationID,
			&m.SenderID,
			&m.ClientMessageID,
			&m.IdempotencyKey,
			&m.SeqID,
			&m.Type,
			&m.Metadata,
			&m.IsForwarded,
			&m.ForwardedFromMsgID,
			&m.ReplyToMsgID,
			&m.PollID,
			&m.LinkPreviewID,
			&m.MentionCount,
			&m.CreatedAt,
			&m.EditedAt,
			&m.DeletedAt,
			&m.ExpiresAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *PostgresMessageRepository) SearchMessages(ctx context.Context, conversationID uuid.UUID, query string, page, limit int) ([]message.Message, int64, error) {
	return nil, 0, sentinal_errors.ErrForbidden
}

func (r *PostgresMessageRepository) GetMessagesByType(ctx context.Context, conversationID uuid.UUID, msgType string, limit int) ([]message.Message, error) {
	var messages []message.Message
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, conversation_id, sender_id, client_message_id, idempotency_key, seq_id, type, metadata,
               is_forwarded, forwarded_from_msg_id, reply_to_msg_id, poll_id, link_preview_id, mention_count,
               created_at, edited_at, deleted_at, expires_at
        FROM messages
        WHERE conversation_id = $1 AND type = $2 AND deleted_at IS NULL
        ORDER BY created_at DESC
        LIMIT $3
    `, conversationID, msgType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var m message.Message
		if err := rows.Scan(
			&m.ID,
			&m.ConversationID,
			&m.SenderID,
			&m.ClientMessageID,
			&m.IdempotencyKey,
			&m.SeqID,
			&m.Type,
			&m.Metadata,
			&m.IsForwarded,
			&m.ForwardedFromMsgID,
			&m.ReplyToMsgID,
			&m.PollID,
			&m.LinkPreviewID,
			&m.MentionCount,
			&m.CreatedAt,
			&m.EditedAt,
			&m.DeletedAt,
			&m.ExpiresAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *PostgresMessageRepository) GetLatestMessage(ctx context.Context, conversationID uuid.UUID) (message.Message, error) {
	var m message.Message
	err := r.db.QueryRowContext(ctx, `
        SELECT id, conversation_id, sender_id, client_message_id, idempotency_key, seq_id, type, metadata,
               is_forwarded, forwarded_from_msg_id, reply_to_msg_id, poll_id, link_preview_id, mention_count,
               created_at, edited_at, deleted_at, expires_at
        FROM messages
        WHERE conversation_id = $1 AND deleted_at IS NULL
        ORDER BY seq_id DESC
        LIMIT 1
    `, conversationID).Scan(
		&m.ID,
		&m.ConversationID,
		&m.SenderID,
		&m.ClientMessageID,
		&m.IdempotencyKey,
		&m.SeqID,
		&m.Type,
		&m.Metadata,
		&m.IsForwarded,
		&m.ForwardedFromMsgID,
		&m.ReplyToMsgID,
		&m.PollID,
		&m.LinkPreviewID,
		&m.MentionCount,
		&m.CreatedAt,
		&m.EditedAt,
		&m.DeletedAt,
		&m.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return message.Message{}, sentinal_errors.ErrNotFound
		}
		return message.Message{}, err
	}
	return m, nil
}

func (r *PostgresMessageRepository) MarkAsEdited(ctx context.Context, messageID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE messages SET edited_at = $1 WHERE id = $2", time.Now(), messageID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresMessageRepository) GetMessageCountSince(ctx context.Context, conversationID uuid.UUID, since time.Time) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM messages
        WHERE conversation_id = $1 AND created_at > $2 AND deleted_at IS NULL
    `, conversationID, since).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *PostgresMessageRepository) GetByIdempotencyKey(ctx context.Context, key string) (message.Message, error) {
	var m message.Message
	err := r.db.QueryRowContext(ctx, `
        SELECT id, conversation_id, sender_id, client_message_id, idempotency_key, seq_id, type, metadata,
               is_forwarded, forwarded_from_msg_id, reply_to_msg_id, poll_id, link_preview_id, mention_count,
               created_at, edited_at, deleted_at, expires_at
        FROM messages WHERE idempotency_key = $1
    `, key).Scan(
		&m.ID,
		&m.ConversationID,
		&m.SenderID,
		&m.ClientMessageID,
		&m.IdempotencyKey,
		&m.SeqID,
		&m.Type,
		&m.Metadata,
		&m.IsForwarded,
		&m.ForwardedFromMsgID,
		&m.ReplyToMsgID,
		&m.PollID,
		&m.LinkPreviewID,
		&m.MentionCount,
		&m.CreatedAt,
		&m.EditedAt,
		&m.DeletedAt,
		&m.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return message.Message{}, sentinal_errors.ErrNotFound
		}
		return message.Message{}, err
	}
	return m, nil
}

func (r *PostgresMessageRepository) GetByClientMessageID(ctx context.Context, clientMsgID string) (message.Message, error) {
	var m message.Message
	err := r.db.QueryRowContext(ctx, `
        SELECT id, conversation_id, sender_id, client_message_id, idempotency_key, seq_id, type, metadata,
               is_forwarded, forwarded_from_msg_id, reply_to_msg_id, poll_id, link_preview_id, mention_count,
               created_at, edited_at, deleted_at, expires_at
        FROM messages WHERE client_message_id = $1
    `, clientMsgID).Scan(
		&m.ID,
		&m.ConversationID,
		&m.SenderID,
		&m.ClientMessageID,
		&m.IdempotencyKey,
		&m.SeqID,
		&m.Type,
		&m.Metadata,
		&m.IsForwarded,
		&m.ForwardedFromMsgID,
		&m.ReplyToMsgID,
		&m.PollID,
		&m.LinkPreviewID,
		&m.MentionCount,
		&m.CreatedAt,
		&m.EditedAt,
		&m.DeletedAt,
		&m.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return message.Message{}, sentinal_errors.ErrNotFound
		}
		return message.Message{}, err
	}
	return m, nil
}

func (r *PostgresMessageRepository) AddReaction(ctx context.Context, reaction *message.MessageReaction) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO message_reactions (id, message_id, user_id, reaction_code, created_at)
        VALUES ($1,$2,$3,$4,$5)
    `, reaction.ID, reaction.MessageID, reaction.UserID, reaction.ReactionCode, reaction.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresMessageRepository) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, reactionCode string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM message_reactions WHERE message_id = $1 AND user_id = $2 AND reaction_code = $3", messageID, userID, reactionCode)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresMessageRepository) GetMessageReactions(ctx context.Context, messageID uuid.UUID) ([]message.MessageReaction, error) {
	var reactions []message.MessageReaction
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, message_id, user_id, reaction_code, created_at
        FROM message_reactions WHERE message_id = $1
    `, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rct message.MessageReaction
		if err := rows.Scan(&rct.ID, &rct.MessageID, &rct.UserID, &rct.ReactionCode, &rct.CreatedAt); err != nil {
			return nil, err
		}
		reactions = append(reactions, rct)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return reactions, nil
}

func (r *PostgresMessageRepository) GetUserReaction(ctx context.Context, messageID, userID uuid.UUID) (message.MessageReaction, error) {
	var reaction message.MessageReaction
	err := r.db.QueryRowContext(ctx, `
        SELECT id, message_id, user_id, reaction_code, created_at
        FROM message_reactions WHERE message_id = $1 AND user_id = $2
    `, messageID, userID).Scan(&reaction.ID, &reaction.MessageID, &reaction.UserID, &reaction.ReactionCode, &reaction.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return message.MessageReaction{}, sentinal_errors.ErrNotFound
		}
		return message.MessageReaction{}, err
	}
	return reaction, nil
}

func (r *PostgresMessageRepository) CreateReceipt(ctx context.Context, receipt *message.MessageReceipt) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO message_receipts (message_id, user_id, status, delivered_at, read_at, played_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
    `, receipt.MessageID, receipt.UserID, receipt.Status, receipt.DeliveredAt, receipt.ReadAt, receipt.PlayedAt, receipt.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresMessageRepository) UpdateReceipt(ctx context.Context, receipt message.MessageReceipt) error {
	_, err := r.db.ExecContext(ctx, `
        UPDATE message_receipts
        SET status = $1, delivered_at = $2, read_at = $3, played_at = $4, updated_at = $5
        WHERE message_id = $6 AND user_id = $7
    `, receipt.Status, receipt.DeliveredAt, receipt.ReadAt, receipt.PlayedAt, receipt.UpdatedAt, receipt.MessageID, receipt.UserID)
	return err
}

func (r *PostgresMessageRepository) GetMessageReceipts(ctx context.Context, messageID uuid.UUID) ([]message.MessageReceipt, error) {
	var receipts []message.MessageReceipt
	rows, err := r.db.QueryContext(ctx, `
        SELECT message_id, user_id, status, delivered_at, read_at, played_at, updated_at
        FROM message_receipts WHERE message_id = $1
    `, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rcp message.MessageReceipt
		if err := rows.Scan(&rcp.MessageID, &rcp.UserID, &rcp.Status, &rcp.DeliveredAt, &rcp.ReadAt, &rcp.PlayedAt, &rcp.UpdatedAt); err != nil {
			return nil, err
		}
		receipts = append(receipts, rcp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return receipts, nil
}

func (r *PostgresMessageRepository) MarkAsDelivered(ctx context.Context, messageID, userID uuid.UUID) error {
	now := time.Now()
	res, err := r.db.ExecContext(ctx, `
        UPDATE message_receipts
        SET status = 'DELIVERED', delivered_at = $1, updated_at = $1
        WHERE message_id = $2 AND user_id = $3
    `, now, messageID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		receipt := &message.MessageReceipt{
			MessageID:   messageID,
			UserID:      userID,
			Status:      "DELIVERED",
			DeliveredAt: toNullTime(now),
			UpdatedAt:   now,
		}
		return r.CreateReceipt(ctx, receipt)
	}
	return err
}

func (r *PostgresMessageRepository) MarkAsRead(ctx context.Context, messageID, userID uuid.UUID) error {
	now := time.Now()
	res, err := r.db.ExecContext(ctx, `
        UPDATE message_receipts
        SET status = 'READ', read_at = $1, updated_at = $1
        WHERE message_id = $2 AND user_id = $3
    `, now, messageID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		receipt := &message.MessageReceipt{
			MessageID: messageID,
			UserID:    userID,
			Status:    "READ",
			ReadAt:    toNullTime(now),
			UpdatedAt: now,
		}
		return r.CreateReceipt(ctx, receipt)
	}
	return err
}

func (r *PostgresMessageRepository) MarkAsPlayed(ctx context.Context, messageID, userID uuid.UUID) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
        UPDATE message_receipts
        SET played_at = $1, updated_at = $1
        WHERE message_id = $2 AND user_id = $3
    `, now, messageID, userID)
	return err
}

func (r *PostgresMessageRepository) BulkMarkAsDelivered(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	now := time.Now()
	return WithTx(ctx, r.db, func(tx DBTX) error {
		for _, msgID := range messageIDs {
			res, err := tx.ExecContext(ctx, `
                UPDATE message_receipts
                SET status = 'DELIVERED', delivered_at = $1, updated_at = $1
                WHERE message_id = $2 AND user_id = $3
            `, now, msgID, userID)
			if err != nil {
				return err
			}
			rows, err := res.RowsAffected()
			if err == nil && rows == 0 {
				receipt := &message.MessageReceipt{
					MessageID:   msgID,
					UserID:      userID,
					Status:      "DELIVERED",
					DeliveredAt: toNullTime(now),
					UpdatedAt:   now,
				}
				if _, err := tx.ExecContext(ctx, `
                    INSERT INTO message_receipts (message_id, user_id, status, delivered_at, updated_at)
                    VALUES ($1,$2,$3,$4,$5)
                `, receipt.MessageID, receipt.UserID, receipt.Status, receipt.DeliveredAt, receipt.UpdatedAt); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *PostgresMessageRepository) BulkMarkAsRead(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	now := time.Now()
	return WithTx(ctx, r.db, func(tx DBTX) error {
		for _, msgID := range messageIDs {
			res, err := tx.ExecContext(ctx, `
                UPDATE message_receipts
                SET status = 'READ', read_at = $1, updated_at = $1
                WHERE message_id = $2 AND user_id = $3
            `, now, msgID, userID)
			if err != nil {
				return err
			}
			rows, err := res.RowsAffected()
			if err == nil && rows == 0 {
				receipt := &message.MessageReceipt{
					MessageID: msgID,
					UserID:    userID,
					Status:    "READ",
					ReadAt:    toNullTime(now),
					UpdatedAt: now,
				}
				if _, err := tx.ExecContext(ctx, `
                    INSERT INTO message_receipts (message_id, user_id, status, read_at, updated_at)
                    VALUES ($1,$2,$3,$4,$5)
                `, receipt.MessageID, receipt.UserID, receipt.Status, receipt.ReadAt, receipt.UpdatedAt); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *PostgresMessageRepository) AddMention(ctx context.Context, m *message.MessageMention) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO message_mentions (message_id, user_id, "offset", length)
        VALUES ($1,$2,$3,$4)
    `, m.MessageID, m.UserID, m.Offset, m.Length)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresMessageRepository) GetMessageMentions(ctx context.Context, messageID uuid.UUID) ([]message.MessageMention, error) {
	var mentions []message.MessageMention
	rows, err := r.db.QueryContext(ctx, `
        SELECT message_id, user_id, "offset", length
        FROM message_mentions WHERE message_id = $1
    `, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m message.MessageMention
		if err := rows.Scan(&m.MessageID, &m.UserID, &m.Offset, &m.Length); err != nil {
			return nil, err
		}
		mentions = append(mentions, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mentions, nil
}

func (r *PostgresMessageRepository) GetUserMentions(ctx context.Context, userID uuid.UUID, page, limit int) ([]message.Message, int64, error) {
	var messages []message.Message
	var total int64

	if err := r.db.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM messages
        WHERE id IN (SELECT message_id FROM message_mentions WHERE user_id = $1) AND deleted_at IS NULL
    `, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, conversation_id, sender_id, client_message_id, idempotency_key, seq_id, type, metadata,
               is_forwarded, forwarded_from_msg_id, reply_to_msg_id, poll_id, link_preview_id, mention_count,
               created_at, edited_at, deleted_at, expires_at
        FROM messages
        WHERE id IN (SELECT message_id FROM message_mentions WHERE user_id = $1) AND deleted_at IS NULL
        ORDER BY created_at DESC
        OFFSET $2 LIMIT $3
    `, userID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var m message.Message
		if err := rows.Scan(
			&m.ID,
			&m.ConversationID,
			&m.SenderID,
			&m.ClientMessageID,
			&m.IdempotencyKey,
			&m.SeqID,
			&m.Type,
			&m.Metadata,
			&m.IsForwarded,
			&m.ForwardedFromMsgID,
			&m.ReplyToMsgID,
			&m.PollID,
			&m.LinkPreviewID,
			&m.MentionCount,
			&m.CreatedAt,
			&m.EditedAt,
			&m.DeletedAt,
			&m.ExpiresAt,
		); err != nil {
			return nil, 0, err
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return messages, total, nil
}

func (r *PostgresMessageRepository) StarMessage(ctx context.Context, s *message.StarredMessage) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO starred_messages (user_id, message_id, starred_at)
        VALUES ($1,$2,$3)
    `, s.UserID, s.MessageID, s.StarredAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresMessageRepository) UnstarMessage(ctx context.Context, userID, messageID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM starred_messages WHERE user_id = $1 AND message_id = $2", userID, messageID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresMessageRepository) GetUserStarredMessages(ctx context.Context, userID uuid.UUID, page, limit int) ([]message.StarredMessage, int64, error) {
	var starred []message.StarredMessage
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM starred_messages WHERE user_id = $1", userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
        SELECT user_id, message_id, starred_at
        FROM starred_messages
        WHERE user_id = $1
        ORDER BY starred_at DESC
        OFFSET $2 LIMIT $3
    `, userID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var s message.StarredMessage
		if err := rows.Scan(&s.UserID, &s.MessageID, &s.StarredAt); err != nil {
			return nil, 0, err
		}
		starred = append(starred, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return starred, total, nil
}

func (r *PostgresMessageRepository) IsMessageStarred(ctx context.Context, userID, messageID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM starred_messages WHERE user_id = $1 AND message_id = $2", userID, messageID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresMessageRepository) CreateAttachment(ctx context.Context, a *message.Attachment) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO attachments (
            id, uploader_id, url, filename, mime_type, size_bytes, view_once, viewed_at, thumbnail_url,
            width, height, duration_seconds, encryption_key_hash, encryption_iv, created_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
    `,
		a.ID,
		a.UploaderID,
		a.URL,
		a.Filename,
		a.MimeType,
		a.SizeBytes,
		a.ViewOnce,
		a.ViewedAt,
		a.ThumbnailURL,
		a.Width,
		a.Height,
		a.DurationSeconds,
		a.EncryptionKeyHash,
		a.EncryptionIV,
		a.CreatedAt,
	)
	return err
}

func (r *PostgresMessageRepository) GetAttachmentByID(ctx context.Context, id uuid.UUID) (message.Attachment, error) {
	var a message.Attachment
	err := r.db.QueryRowContext(ctx, `
        SELECT id, uploader_id, url, filename, mime_type, size_bytes, view_once, viewed_at, thumbnail_url,
               width, height, duration_seconds, encryption_key_hash, encryption_iv, created_at
        FROM attachments WHERE id = $1
    `, id).Scan(
		&a.ID,
		&a.UploaderID,
		&a.URL,
		&a.Filename,
		&a.MimeType,
		&a.SizeBytes,
		&a.ViewOnce,
		&a.ViewedAt,
		&a.ThumbnailURL,
		&a.Width,
		&a.Height,
		&a.DurationSeconds,
		&a.EncryptionKeyHash,
		&a.EncryptionIV,
		&a.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return message.Attachment{}, sentinal_errors.ErrNotFound
		}
		return message.Attachment{}, err
	}
	return a, nil
}

func (r *PostgresMessageRepository) LinkAttachmentToMessage(ctx context.Context, ma *message.MessageAttachment) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO message_attachments (message_id, attachment_id)
        VALUES ($1,$2)
    `, ma.MessageID, ma.AttachmentID)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresMessageRepository) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]message.Attachment, error) {
	var attachments []message.Attachment
	rows, err := r.db.QueryContext(ctx, `
        SELECT a.id, a.uploader_id, a.url, a.filename, a.mime_type, a.size_bytes, a.view_once, a.viewed_at,
               a.thumbnail_url, a.width, a.height, a.duration_seconds, a.encryption_key_hash, a.encryption_iv, a.created_at
        FROM attachments a
        WHERE a.id IN (SELECT attachment_id FROM message_attachments WHERE message_id = $1)
    `, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a message.Attachment
		if err := rows.Scan(
			&a.ID,
			&a.UploaderID,
			&a.URL,
			&a.Filename,
			&a.MimeType,
			&a.SizeBytes,
			&a.ViewOnce,
			&a.ViewedAt,
			&a.ThumbnailURL,
			&a.Width,
			&a.Height,
			&a.DurationSeconds,
			&a.EncryptionKeyHash,
			&a.EncryptionIV,
			&a.CreatedAt,
		); err != nil {
			return nil, err
		}
		attachments = append(attachments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attachments, nil
}

func (r *PostgresMessageRepository) MarkViewOnceViewed(ctx context.Context, attachmentID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `
        UPDATE attachments SET viewed_at = $1 WHERE id = $2 AND view_once = true
    `, time.Now(), attachmentID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresMessageRepository) CreateLinkPreview(ctx context.Context, lp *message.LinkPreview) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO link_previews (id, url, url_hash, title, description, image_url, site_name, fetched_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `, lp.ID, lp.URL, lp.URLHash, lp.Title, lp.Description, lp.ImageURL, lp.SiteName, lp.FetchedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresMessageRepository) GetLinkPreviewByHash(ctx context.Context, urlHash string) (message.LinkPreview, error) {
	var lp message.LinkPreview
	err := r.db.QueryRowContext(ctx, `
        SELECT id, url, url_hash, title, description, image_url, site_name, fetched_at
        FROM link_previews WHERE url_hash = $1
    `, urlHash).Scan(
		&lp.ID,
		&lp.URL,
		&lp.URLHash,
		&lp.Title,
		&lp.Description,
		&lp.ImageURL,
		&lp.SiteName,
		&lp.FetchedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return message.LinkPreview{}, sentinal_errors.ErrNotFound
		}
		return message.LinkPreview{}, err
	}
	return lp, nil
}

func (r *PostgresMessageRepository) GetLinkPreviewByID(ctx context.Context, id uuid.UUID) (message.LinkPreview, error) {
	var lp message.LinkPreview
	err := r.db.QueryRowContext(ctx, `
        SELECT id, url, url_hash, title, description, image_url, site_name, fetched_at
        FROM link_previews WHERE id = $1
    `, id).Scan(
		&lp.ID,
		&lp.URL,
		&lp.URLHash,
		&lp.Title,
		&lp.Description,
		&lp.ImageURL,
		&lp.SiteName,
		&lp.FetchedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return message.LinkPreview{}, sentinal_errors.ErrNotFound
		}
		return message.LinkPreview{}, err
	}
	return lp, nil
}

func (r *PostgresMessageRepository) CreatePoll(ctx context.Context, p *message.Poll) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO polls (id, message_id, question, allows_multiple, closes_at, created_at)
        VALUES ($1,$2,$3,$4,$5,$6)
    `, p.ID, p.MessageID, p.Question, p.AllowsMultiple, p.ClosesAt, p.CreatedAt)
	return err
}

func (r *PostgresMessageRepository) GetPollByID(ctx context.Context, id uuid.UUID) (message.Poll, error) {
	var p message.Poll
	err := r.db.QueryRowContext(ctx, `
        SELECT id, message_id, question, allows_multiple, closes_at, created_at
        FROM polls WHERE id = $1
    `, id).Scan(&p.ID, &p.MessageID, &p.Question, &p.AllowsMultiple, &p.ClosesAt, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return message.Poll{}, sentinal_errors.ErrNotFound
		}
		return message.Poll{}, err
	}
	return p, nil
}

func (r *PostgresMessageRepository) ClosePoll(ctx context.Context, pollID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE polls SET closes_at = $1 WHERE id = $2", time.Now(), pollID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresMessageRepository) AddPollOption(ctx context.Context, o *message.PollOption) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO poll_options (id, poll_id, option_text, position)
        VALUES ($1,$2,$3,$4)
    `, o.ID, o.PollID, o.OptionText, o.Position)
	return err
}

func (r *PostgresMessageRepository) GetPollOptions(ctx context.Context, pollID uuid.UUID) ([]message.PollOption, error) {
	var options []message.PollOption
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, poll_id, option_text, position
        FROM poll_options WHERE poll_id = $1
        ORDER BY position ASC
    `, pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var o message.PollOption
		if err := rows.Scan(&o.ID, &o.PollID, &o.OptionText, &o.Position); err != nil {
			return nil, err
		}
		options = append(options, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return options, nil
}

func (r *PostgresMessageRepository) VotePoll(ctx context.Context, v *message.PollVote) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO poll_votes (poll_id, option_id, user_id, voted_at)
        VALUES ($1,$2,$3,$4)
    `, v.PollID, v.OptionID, v.UserID, v.VotedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresMessageRepository) RemoveVote(ctx context.Context, pollID, optionID, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM poll_votes WHERE poll_id = $1 AND option_id = $2 AND user_id = $3", pollID, optionID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresMessageRepository) GetPollVotes(ctx context.Context, pollID uuid.UUID) ([]message.PollVote, error) {
	var votes []message.PollVote
	rows, err := r.db.QueryContext(ctx, `
        SELECT poll_id, option_id, user_id, voted_at
        FROM poll_votes WHERE poll_id = $1
    `, pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v message.PollVote
		if err := rows.Scan(&v.PollID, &v.OptionID, &v.UserID, &v.VotedAt); err != nil {
			return nil, err
		}
		votes = append(votes, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return votes, nil
}

func (r *PostgresMessageRepository) GetUserVotes(ctx context.Context, pollID, userID uuid.UUID) ([]message.PollVote, error) {
	var votes []message.PollVote
	rows, err := r.db.QueryContext(ctx, `
        SELECT poll_id, option_id, user_id, voted_at
        FROM poll_votes WHERE poll_id = $1 AND user_id = $2
    `, pollID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v message.PollVote
		if err := rows.Scan(&v.PollID, &v.OptionID, &v.UserID, &v.VotedAt); err != nil {
			return nil, err
		}
		votes = append(votes, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return votes, nil
}

func (r *PostgresMessageRepository) DeleteExpiredMessages(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, "DELETE FROM messages WHERE expires_at IS NOT NULL AND expires_at < NOW()")
	if err != nil {
		return 0, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func toNullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}
