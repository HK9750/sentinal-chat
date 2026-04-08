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

const msgColumns = `id, conversation_id, sender_id, client_message_id, seq_id, type, content,
       is_forwarded, reply_to_msg_id, poll_id, mention_count,
       created_at, edited_at, deleted_at, expires_at`

const msgColumnsWithAliasM = `m.id, m.conversation_id, m.sender_id, m.client_message_id, m.seq_id, m.type, m.content,
       m.is_forwarded, m.reply_to_msg_id, m.poll_id, m.mention_count,
       m.created_at, m.edited_at, m.deleted_at, m.expires_at`

func scanMessage(scanner interface {
	Scan(dest ...any) error
}) (message.Message, error) {
	var m message.Message
	err := scanner.Scan(
		&m.ID,
		&m.ConversationID,
		&m.SenderID,
		&m.ClientMessageID,
		&m.SeqID,
		&m.Type,
		&m.Content,
		&m.IsForwarded,
		&m.ReplyToMsgID,
		&m.PollID,
		&m.MentionCount,
		&m.CreatedAt,
		&m.EditedAt,
		&m.DeletedAt,
		&m.ExpiresAt,
	)
	return m, err
}

func (r *PostgresMessageRepository) Create(ctx context.Context, m *message.Message) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO messages (
            id, conversation_id, sender_id, client_message_id, type, content,
            is_forwarded, reply_to_msg_id, poll_id, mention_count,
            created_at, edited_at, deleted_at, expires_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
    `,
		m.ID,
		m.ConversationID,
		m.SenderID,
		m.ClientMessageID,
		m.Type,
		m.Content,
		m.IsForwarded,
		m.ReplyToMsgID,
		m.PollID,
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

func (r *PostgresMessageRepository) GetByID(ctx context.Context, id uuid.UUID) (message.Message, error) {
	m, err := scanMessage(r.db.QueryRowContext(ctx, `
        SELECT `+msgColumns+` FROM messages WHERE id = $1
    `, id))
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
        SET conversation_id = $1, sender_id = $2, client_message_id = $3, seq_id = $4,
            type = $5, content = $6, is_forwarded = $7, reply_to_msg_id = $8,
            poll_id = $9, mention_count = $10, edited_at = $11, deleted_at = $12, expires_at = $13
        WHERE id = $14
    `,
		m.ConversationID,
		m.SenderID,
		m.ClientMessageID,
		m.SeqID,
		m.Type,
		m.Content,
		m.IsForwarded,
		m.ReplyToMsgID,
		m.PollID,
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

func (r *PostgresMessageRepository) Restore(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE messages SET deleted_at = NULL WHERE id = $1", id)
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

func (r *PostgresMessageRepository) GetConversationMessages(ctx context.Context, conversationID uuid.UUID, beforeSeq int64, limit int) ([]message.Message, error) {
	var messages []message.Message

	query := `SELECT ` + msgColumns + ` FROM messages WHERE conversation_id = $1 AND deleted_at IS NULL`
	args := []interface{}{conversationID}
	if beforeSeq > 0 {
		query += " AND seq_id < $2"
		args = append(args, beforeSeq)
	}
	query += fmt.Sprintf(" ORDER BY seq_id DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *PostgresMessageRepository) GetConversationMessagesVisible(ctx context.Context, conversationID, userID uuid.UUID, beforeSeq int64, limit int) ([]message.Message, error) {
	var messages []message.Message

	query := `
		SELECT ` + msgColumnsWithAliasM + `
		FROM messages m
		LEFT JOIN conversation_clears cc
			ON cc.conversation_id = m.conversation_id AND cc.user_id = $2
		WHERE m.conversation_id = $1
		  AND m.deleted_at IS NULL
		  AND (m.expires_at IS NULL OR m.expires_at > NOW())
		  AND (cc.cleared_at IS NULL OR m.created_at > cc.cleared_at)
		  AND NOT EXISTS (
			  SELECT 1
			  FROM message_deletions md
			  WHERE md.message_id = m.id AND md.user_id = $2
		  )`
	args := []interface{}{conversationID, userID}
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
		m, err := scanMessage(rows)
		if err != nil {
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
        SELECT `+msgColumns+`
        FROM messages
        WHERE conversation_id = $1 AND seq_id >= $2 AND seq_id <= $3 AND deleted_at IS NULL
        ORDER BY seq_id ASC
    `, conversationID, startSeq, endSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
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
        SELECT `+msgColumns+`
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
		m, err := scanMessage(rows)
		if err != nil {
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
	var messages []message.Message
	var total int64
	pattern := "%" + query + "%"

	if err := r.db.QueryRowContext(ctx, `
	        SELECT COUNT(*) FROM messages
	        WHERE conversation_id = $1 AND deleted_at IS NULL AND content ILIKE $2
	    `, conversationID, pattern).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
	        SELECT `+msgColumns+`
	        FROM messages
	        WHERE conversation_id = $1 AND deleted_at IS NULL AND content ILIKE $2
	        ORDER BY seq_id DESC
	        OFFSET $3 LIMIT $4
	    `, conversationID, pattern, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, 0, err
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return messages, total, nil
}

func (r *PostgresMessageRepository) GetMessagesByType(ctx context.Context, conversationID uuid.UUID, msgType string, limit int) ([]message.Message, error) {
	var messages []message.Message
	rows, err := r.db.QueryContext(ctx, `
        SELECT `+msgColumns+`
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
		m, err := scanMessage(rows)
		if err != nil {
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
	m, err := scanMessage(r.db.QueryRowContext(ctx, `
        SELECT `+msgColumns+`
        FROM messages
        WHERE conversation_id = $1 AND deleted_at IS NULL
        ORDER BY seq_id DESC
        LIMIT 1
    `, conversationID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return message.Message{}, sentinal_errors.ErrNotFound
		}
		return message.Message{}, err
	}
	return m, nil
}

func (r *PostgresMessageRepository) GetLatestMessageVisible(ctx context.Context, conversationID, userID uuid.UUID) (message.Message, error) {
	m, err := scanMessage(r.db.QueryRowContext(ctx, `
		SELECT `+msgColumnsWithAliasM+`
		FROM messages m
		LEFT JOIN conversation_clears cc
			ON cc.conversation_id = m.conversation_id AND cc.user_id = $2
		WHERE m.conversation_id = $1
		  AND m.deleted_at IS NULL
		  AND (m.expires_at IS NULL OR m.expires_at > NOW())
		  AND (cc.cleared_at IS NULL OR m.created_at > cc.cleared_at)
		  AND NOT EXISTS (
			  SELECT 1
			  FROM message_deletions md
			  WHERE md.message_id = m.id AND md.user_id = $2
		  )
		ORDER BY m.seq_id DESC
		LIMIT 1
	`, conversationID, userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return message.Message{}, sentinal_errors.ErrNotFound
		}
		return message.Message{}, err
	}
	return m, nil
}

func (r *PostgresMessageRepository) CountUnreadVisibleSince(ctx context.Context, conversationID, userID uuid.UUID, lastReadSeq int64) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM messages m
		LEFT JOIN conversation_clears cc
			ON cc.conversation_id = m.conversation_id AND cc.user_id = $2
		WHERE m.conversation_id = $1
		  AND m.seq_id > $3
		  AND m.sender_id != $2
		  AND m.deleted_at IS NULL
		  AND (m.expires_at IS NULL OR m.expires_at > NOW())
		  AND (cc.cleared_at IS NULL OR m.created_at > cc.cleared_at)
		  AND NOT EXISTS (
			  SELECT 1
			  FROM message_deletions md
			  WHERE md.message_id = m.id AND md.user_id = $2
		  )
	`, conversationID, userID, lastReadSeq).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
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

func (r *PostgresMessageRepository) GetByClientMessageID(ctx context.Context, conversationID uuid.UUID, clientMsgID string) (message.Message, error) {
	m, err := scanMessage(r.db.QueryRowContext(ctx, `
        SELECT `+msgColumns+`
        FROM messages WHERE conversation_id = $1 AND client_message_id = $2
    `, conversationID, clientMsgID))
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
        VALUES ($1,$2,$3::delivery_status,$4,$5,$6,$7)
    `, receipt.MessageID, receipt.UserID, receipt.Status, receipt.DeliveredAt, receipt.ReadAt, receipt.PlayedAt, receipt.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
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
        SET status = CASE WHEN status IN ('READ'::delivery_status, 'PLAYED'::delivery_status) THEN status ELSE 'DELIVERED'::delivery_status END,
            delivered_at = COALESCE(delivered_at, $1),
            updated_at = $1
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
        SET status = CASE WHEN status = 'PLAYED'::delivery_status THEN 'PLAYED'::delivery_status ELSE 'READ'::delivery_status END,
            delivered_at = COALESCE(delivered_at, $1),
            read_at = COALESCE(read_at, $1),
            updated_at = $1
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
			Status:      "READ",
			DeliveredAt: toNullTime(now),
			ReadAt:      toNullTime(now),
			UpdatedAt:   now,
		}
		return r.CreateReceipt(ctx, receipt)
	}
	return err
}

func (r *PostgresMessageRepository) MarkAsPlayed(ctx context.Context, messageID, userID uuid.UUID) error {
	now := time.Now()
	res, err := r.db.ExecContext(ctx, `
        UPDATE message_receipts
        SET status = 'PLAYED'::delivery_status,
            delivered_at = COALESCE(delivered_at, $1),
            read_at = COALESCE(read_at, $1),
            played_at = COALESCE(played_at, $1),
            updated_at = $1
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
			Status:      "PLAYED",
			DeliveredAt: toNullTime(now),
			ReadAt:      toNullTime(now),
			PlayedAt:    toNullTime(now),
			UpdatedAt:   now,
		}
		return r.CreateReceipt(ctx, receipt)
	}
	return err
}

func (r *PostgresMessageRepository) BulkMarkAsDelivered(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	now := time.Now()
	return WithTx(ctx, r.db, func(tx DBTX) error {
		for _, msgID := range messageIDs {
			res, err := tx.ExecContext(ctx, `
                UPDATE message_receipts
                SET status = CASE WHEN status IN ('READ'::delivery_status, 'PLAYED'::delivery_status) THEN status ELSE 'DELIVERED'::delivery_status END,
                    delivered_at = COALESCE(delivered_at, $1),
                    updated_at = $1
                WHERE message_id = $2 AND user_id = $3
            `, now, msgID, userID)
			if err != nil {
				return err
			}
			rows, err := res.RowsAffected()
			if err == nil && rows == 0 {
				if _, err := tx.ExecContext(ctx, `
                    INSERT INTO message_receipts (message_id, user_id, status, delivered_at, updated_at)
                    VALUES ($1,$2,$3::delivery_status,$4,$5)
                `, msgID, userID, "DELIVERED", now, now); err != nil {
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
                SET status = CASE WHEN status = 'PLAYED'::delivery_status THEN 'PLAYED'::delivery_status ELSE 'READ'::delivery_status END,
                    delivered_at = COALESCE(delivered_at, $1),
                    read_at = COALESCE(read_at, $1),
                    updated_at = $1
                WHERE message_id = $2 AND user_id = $3
            `, now, msgID, userID)
			if err != nil {
				return err
			}
			rows, err := res.RowsAffected()
			if err == nil && rows == 0 {
				if _, err := tx.ExecContext(ctx, `
                    INSERT INTO message_receipts (message_id, user_id, status, delivered_at, read_at, updated_at)
                    VALUES ($1,$2,$3::delivery_status,$4,$5,$6)
                `, msgID, userID, "READ", now, now, now); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// MarkAllPendingAsDelivered finds all messages across the user's conversations
// that were sent by OTHER users and have no delivery receipt yet, then bulk-creates
// DELIVERED receipts for them. Returns the list of affected messages so the caller
// can fan out receipt:update events to the original senders.
func (r *PostgresMessageRepository) MarkAllPendingAsDelivered(ctx context.Context, userID uuid.UUID) ([]MessageDeliveryUpdate, error) {
	now := time.Now()

	// Single query: find all messages in the user's conversations where:
	// 1. The message was sent by someone else (sender_id != userID)
	// 2. The message is not deleted
	// 3. No delivery receipt exists for this user yet
	// Then INSERT a DELIVERED receipt for each, returning the affected message metadata.
	rows, err := r.db.QueryContext(ctx, `
		INSERT INTO message_receipts (message_id, user_id, status, delivered_at, updated_at)
		SELECT m.id, $1, 'DELIVERED'::delivery_status, $2, $2
		FROM messages m
		INNER JOIN participants p ON p.conversation_id = m.conversation_id AND p.user_id = $1
		WHERE m.sender_id != $1
		  AND m.deleted_at IS NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM message_receipts mr
		      WHERE mr.message_id = m.id AND mr.user_id = $1
		  )
		ON CONFLICT (message_id, user_id) DO NOTHING
		RETURNING message_id, (SELECT conversation_id FROM messages WHERE id = message_id), (SELECT sender_id FROM messages WHERE id = message_id)
	`, userID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var updates []MessageDeliveryUpdate
	for rows.Next() {
		var u MessageDeliveryUpdate
		if err := rows.Scan(&u.MessageID, &u.ConversationID, &u.SenderID); err != nil {
			return nil, err
		}
		updates = append(updates, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return updates, nil
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
        SELECT `+msgColumns+`
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
		m, err := scanMessage(rows)
		if err != nil {
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

func (r *PostgresMessageRepository) PinMessage(ctx context.Context, p *message.PinnedMessage) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO pinned_messages (conversation_id, message_id, pinned_by, pinned_at)
        VALUES ($1,$2,$3,$4)
    `, p.ConversationID, p.MessageID, p.PinnedBy, p.PinnedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresMessageRepository) UnpinMessage(ctx context.Context, conversationID, messageID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM pinned_messages WHERE conversation_id = $1 AND message_id = $2", conversationID, messageID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresMessageRepository) GetPinnedMessages(ctx context.Context, conversationID uuid.UUID) ([]message.PinnedMessage, error) {
	var pinned []message.PinnedMessage
	rows, err := r.db.QueryContext(ctx, `
        SELECT conversation_id, message_id, pinned_by, pinned_at
        FROM pinned_messages WHERE conversation_id = $1
        ORDER BY pinned_at DESC
    `, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p message.PinnedMessage
		if err := rows.Scan(&p.ConversationID, &p.MessageID, &p.PinnedBy, &p.PinnedAt); err != nil {
			return nil, err
		}
		pinned = append(pinned, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pinned, nil
}

func (r *PostgresMessageRepository) CreateMessageEdit(ctx context.Context, e *message.MessageEdit) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, `
	        INSERT INTO message_edits (id, message_id, content, edited_by, edited_at, version_number)
	        VALUES ($1,$2,$3,$4,$5,$6)
	    `, e.ID, e.MessageID, e.Content, e.EditedBy, e.EditedAt, e.VersionNumber)
	return err
}

func (r *PostgresMessageRepository) GetMessageEdits(ctx context.Context, messageID uuid.UUID) ([]message.MessageEdit, error) {
	var edits []message.MessageEdit
	rows, err := r.db.QueryContext(ctx, `
	        SELECT id, message_id, content, edited_by, edited_at, version_number
	        FROM message_edits WHERE message_id = $1
	        ORDER BY version_number ASC
	    `, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e message.MessageEdit
		if err := rows.Scan(&e.ID, &e.MessageID, &e.Content, &e.EditedBy, &e.EditedAt, &e.VersionNumber); err != nil {
			return nil, err
		}
		edits = append(edits, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return edits, nil
}

func (r *PostgresMessageRepository) CreateAttachment(ctx context.Context, a *message.Attachment) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO attachments (
            id, uploader_id, file_url, filename, mime_type, size_bytes, view_once, viewed_at,
		thumbnail_url, width, height, duration_seconds, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		a.ID,
		a.UploaderID,
		a.FileURL,
		a.Filename,
		a.MimeType,
		a.SizeBytes,
		a.ViewOnce,
		a.ViewedAt,
		a.ThumbnailURL,
		a.Width,
		a.Height,
		a.DurationSeconds,
		a.CreatedAt,
	)
	return err
}

func (r *PostgresMessageRepository) CreateAttachmentWithLink(ctx context.Context, a *message.Attachment, ma *message.MessageAttachment) error {
	if a == nil || ma == nil {
		return sentinal_errors.ErrInvalidInput
	}
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	ma.AttachmentID = a.ID

	return WithTx(ctx, r.db, func(tx DBTX) error {
		if _, err := tx.ExecContext(ctx, `
        INSERT INTO attachments (
            id, uploader_id, file_url, filename, mime_type, size_bytes, view_once, viewed_at,
		thumbnail_url, width, height, duration_seconds, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
			a.ID,
			a.UploaderID,
			a.FileURL,
			a.Filename,
			a.MimeType,
			a.SizeBytes,
			a.ViewOnce,
			a.ViewedAt,
			a.ThumbnailURL,
			a.Width,
			a.Height,
			a.DurationSeconds,
			a.CreatedAt,
		); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
        INSERT INTO message_attachments (message_id, attachment_id)
        VALUES ($1,$2)
    `, ma.MessageID, ma.AttachmentID); err != nil {
			if isUniqueViolation(err) {
				return sentinal_errors.ErrAlreadyExists
			}
			return err
		}

		return nil
	})
}

func (r *PostgresMessageRepository) GetAttachmentByID(ctx context.Context, id uuid.UUID) (message.Attachment, error) {
	var a message.Attachment
	err := r.db.QueryRowContext(ctx, `
	        SELECT id, uploader_id, file_url, filename, mime_type, size_bytes, view_once, viewed_at,
               thumbnail_url, width, height, duration_seconds, created_at
        FROM attachments WHERE id = $1
	`, id).Scan(
		&a.ID,
		&a.UploaderID,
		&a.FileURL,
		&a.Filename,
		&a.MimeType,
		&a.SizeBytes,
		&a.ViewOnce,
		&a.ViewedAt,
		&a.ThumbnailURL,
		&a.Width,
		&a.Height,
		&a.DurationSeconds,
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

func (r *PostgresMessageRepository) CanUserAccessAttachment(ctx context.Context, attachmentID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
        SELECT EXISTS (
            SELECT 1
            FROM message_attachments ma
            JOIN messages m ON m.id = ma.message_id
            JOIN participants p ON p.conversation_id = m.conversation_id
            WHERE ma.attachment_id = $1
              AND p.user_id = $2
        )
    `, attachmentID, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
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
	        SELECT a.id, a.uploader_id, a.file_url, a.filename, a.mime_type, a.size_bytes, a.view_once, a.viewed_at,
               a.thumbnail_url, a.width, a.height, a.duration_seconds, a.created_at
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
			&a.FileURL,
			&a.Filename,
			&a.MimeType,
			&a.SizeBytes,
			&a.ViewOnce,
			&a.ViewedAt,
			&a.ThumbnailURL,
			&a.Width,
			&a.Height,
			&a.DurationSeconds,
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

func (r *PostgresMessageRepository) CreatePoll(ctx context.Context, p *message.Poll) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
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
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
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

func (r *PostgresMessageRepository) DeleteMessagesForUser(ctx context.Context, conversationID, userID uuid.UUID, messageIDs []uuid.UUID) error {
	if len(messageIDs) == 0 {
		return nil
	}

	return WithTx(ctx, r.db, func(tx DBTX) error {
		for _, messageID := range messageIDs {
			if messageID == uuid.Nil {
				continue
			}

			if _, err := tx.ExecContext(ctx, `
				INSERT INTO message_deletions (message_id, user_id, deleted_at)
				SELECT m.id, $2, NOW()
				FROM messages m
				WHERE m.id = $1 AND m.conversation_id = $3
				ON CONFLICT (message_id, user_id) DO UPDATE
				SET deleted_at = EXCLUDED.deleted_at
			`, messageID, userID, conversationID); err != nil {
				return err
			}
		}

		return nil
	})
}

func toNullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}
