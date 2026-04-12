package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"sentinal-chat/internal/domain/conversation"
	sentinal_errors "sentinal-chat/pkg/errors"
	"sentinal-chat/pkg/logger"

	"github.com/google/uuid"
)

type PostgresConversationRepository struct {
	db     DBTX
	logger *logger.Logger
}

func NewConversationRepository(db DBTX, l *logger.Logger) ConversationRepository {
	return &PostgresConversationRepository{db: db, logger: l}
}

const convColumns = `id, type, subject, description, avatar_url, invite_link, invite_link_revoked_at,
       dm_user_id_a, dm_user_id_b, disappearing_mode, created_by, created_at, updated_at`

func scanConversation(scanner interface {
	Scan(dest ...any) error
}) (conversation.Conversation, error) {
	var c conversation.Conversation
	err := scanner.Scan(
		&c.ID,
		&c.Type,
		&c.Subject,
		&c.Description,
		&c.AvatarURL,
		&c.InviteLink,
		&c.InviteLinkRevokedAt,
		&c.DMUserIDA,
		&c.DMUserIDB,
		&c.DisappearingMode,
		&c.CreatedBy,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	return c, err
}

func (r *PostgresConversationRepository) Create(ctx context.Context, c *conversation.Conversation) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO conversations (
	            id, type, subject, description, avatar_url, invite_link, invite_link_revoked_at,
            dm_user_id_a, dm_user_id_b, disappearing_mode, created_by, created_at, updated_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
    `,
		c.ID,
		c.Type,
		c.Subject,
		c.Description,
		c.AvatarURL,
		c.InviteLink,
		c.InviteLinkRevokedAt,
		c.DMUserIDA,
		c.DMUserIDB,
		c.DisappearingMode,
		c.CreatedBy,
		c.CreatedAt,
		c.UpdatedAt,
	)
	if err != nil {
		logSQLError(r.logger, "conversation.create", `
        INSERT INTO conversations (
	            id, type, subject, description, avatar_url, invite_link, invite_link_revoked_at,
            dm_user_id_a, dm_user_id_b, disappearing_mode, created_by, created_at, updated_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
    `, []any{
			c.ID,
			c.Type,
			c.Subject,
			c.Description,
			c.AvatarURL,
			c.InviteLink,
			c.InviteLinkRevokedAt,
			c.DMUserIDA,
			c.DMUserIDB,
			c.DisappearingMode,
			c.CreatedBy,
			c.CreatedAt,
			c.UpdatedAt,
		}, err)
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresConversationRepository) GetByID(ctx context.Context, id uuid.UUID) (conversation.Conversation, error) {
	c, err := scanConversation(r.db.QueryRowContext(ctx, `
        SELECT `+convColumns+` FROM conversations WHERE id = $1
    `, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return conversation.Conversation{}, sentinal_errors.ErrNotFound
		}
		return conversation.Conversation{}, err
	}

	participants, err := r.GetParticipants(ctx, c.ID)
	if err != nil {
		return conversation.Conversation{}, err
	}
	c.Participants = participants
	return c, nil
}

func (r *PostgresConversationRepository) Update(ctx context.Context, c conversation.Conversation) error {
	res, err := r.db.ExecContext(ctx, `
        UPDATE conversations
        SET type = $1, subject = $2, description = $3, avatar_url = $4, invite_link = $5,
            invite_link_revoked_at = $6, dm_user_id_a = $7, dm_user_id_b = $8,
            disappearing_mode = $9, created_by = $10, updated_at = $11
        WHERE id = $12
    `,
		c.Type,
		c.Subject,
		c.Description,
		c.AvatarURL,
		c.InviteLink,
		c.InviteLinkRevokedAt,
		c.DMUserIDA,
		c.DMUserIDB,
		c.DisappearingMode,
		c.CreatedBy,
		c.UpdatedAt,
		c.ID,
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

func (r *PostgresConversationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM conversations WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresConversationRepository) GetUserConversations(ctx context.Context, userID uuid.UUID, page, limit int) ([]conversation.Conversation, int64, error) {
	var conversations []conversation.Conversation
	var total int64

	if err := r.db.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM conversations c
		WHERE c.id IN (SELECT conversation_id FROM participants WHERE user_id = $1 AND archived = false)
    `, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
        SELECT `+convColumns+`,
               (SELECT MAX(m.created_at) FROM messages m WHERE m.conversation_id = c.id) AS last_message_at
        FROM conversations c
        WHERE c.id IN (SELECT conversation_id FROM participants WHERE user_id = $1 AND archived = false)
        ORDER BY COALESCE((SELECT MAX(m.created_at) FROM messages m WHERE m.conversation_id = c.id), c.created_at) DESC
        OFFSET $2 LIMIT $3
    `, userID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var c conversation.Conversation
		var lastMessageAt sql.NullTime
		if err := rows.Scan(
			&c.ID,
			&c.Type,
			&c.Subject,
			&c.Description,
			&c.AvatarURL,
			&c.InviteLink,
			&c.InviteLinkRevokedAt,
			&c.DMUserIDA,
			&c.DMUserIDB,
			&c.DisappearingMode,
			&c.CreatedBy,
			&c.CreatedAt,
			&c.UpdatedAt,
			&lastMessageAt,
		); err != nil {
			return nil, 0, err
		}
		if lastMessageAt.Valid {
			c.LastMessageAt = &lastMessageAt.Time
		}
		participants, err := r.GetParticipants(ctx, c.ID)
		if err != nil {
			return nil, 0, err
		}
		c.Participants = participants
		conversations = append(conversations, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return conversations, total, nil
}

func (r *PostgresConversationRepository) GetDirectConversation(ctx context.Context, userID1, userID2 uuid.UUID) (conversation.Conversation, error) {
	if userID2.String() < userID1.String() {
		userID1, userID2 = userID2, userID1
	}

	c, err := scanConversation(r.db.QueryRowContext(ctx, `
        SELECT `+convColumns+`
        FROM conversations
        WHERE type = 'DM'
          AND dm_user_id_a = $1
          AND dm_user_id_b = $2
        LIMIT 1
    `, userID1, userID2))
	if err != nil {
		logSQLError(r.logger, "conversation.get_direct", `
        SELECT `+convColumns+`
        FROM conversations
        WHERE type = 'DM'
          AND dm_user_id_a = $1
          AND dm_user_id_b = $2
        LIMIT 1
    `, []any{userID1, userID2}, err)
		if errors.Is(err, sql.ErrNoRows) {
			return conversation.Conversation{}, sentinal_errors.ErrNotFound
		}
		return conversation.Conversation{}, err
	}
	participants, err := r.GetParticipants(ctx, c.ID)
	if err != nil {
		return conversation.Conversation{}, err
	}
	c.Participants = participants
	return c, nil
}

func (r *PostgresConversationRepository) SearchConversations(ctx context.Context, userID uuid.UUID, query string) ([]conversation.Conversation, error) {
	var conversations []conversation.Conversation
	rows, err := r.db.QueryContext(ctx, `
        SELECT `+convColumns+`
        FROM conversations c
		WHERE c.id IN (SELECT conversation_id FROM participants WHERE user_id = $1 AND archived = false)
          AND c.subject ILIKE $2
    `, userID, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		participants, err := r.GetParticipants(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		c.Participants = participants
		conversations = append(conversations, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return conversations, nil
}

func (r *PostgresConversationRepository) GetConversationsByType(ctx context.Context, userID uuid.UUID, convType string) ([]conversation.Conversation, error) {
	var conversations []conversation.Conversation
	rows, err := r.db.QueryContext(ctx, `
        SELECT `+convColumns+`
        FROM conversations c
		WHERE c.id IN (SELECT conversation_id FROM participants WHERE user_id = $1 AND archived = false)
          AND c.type = $2
    `, userID, convType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		participants, err := r.GetParticipants(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		c.Participants = participants
		conversations = append(conversations, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return conversations, nil
}

func (r *PostgresConversationRepository) GetByInviteLink(ctx context.Context, link string) (conversation.Conversation, error) {
	c, err := scanConversation(r.db.QueryRowContext(ctx, `
        SELECT `+convColumns+`
        FROM conversations
        WHERE invite_link = $1 AND (invite_link_revoked_at IS NULL OR invite_link_revoked_at > NOW())
    `, link))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return conversation.Conversation{}, sentinal_errors.ErrNotFound
		}
		return conversation.Conversation{}, err
	}
	participants, err := r.GetParticipants(ctx, c.ID)
	if err != nil {
		return conversation.Conversation{}, err
	}
	c.Participants = participants
	return c, nil
}

func (r *PostgresConversationRepository) RegenerateInviteLink(ctx context.Context, conversationID uuid.UUID) (string, error) {
	newLink := uuid.New().String()
	res, err := r.db.ExecContext(ctx, `
        UPDATE conversations
        SET invite_link = $1, invite_link_revoked_at = NULL
        WHERE id = $2
    `, newLink, conversationID)
	if err != nil {
		return "", err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return "", sentinal_errors.ErrNotFound
	}
	return newLink, err
}

func (r *PostgresConversationRepository) AddParticipant(ctx context.Context, p *conversation.Participant) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO participants (conversation_id, user_id, role, joined_at, added_by, muted_until, archived, last_read_sequence)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `,
		p.ConversationID,
		p.UserID,
		p.Role,
		p.JoinedAt,
		p.AddedBy,
		p.MutedUntil,
		p.Archived,
		p.LastReadSequence,
	)
	if err != nil {
		logSQLError(r.logger, "conversation.add_participant", `
        INSERT INTO participants (conversation_id, user_id, role, joined_at, added_by, muted_until, archived, last_read_sequence)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `, []any{
			p.ConversationID,
			p.UserID,
			p.Role,
			p.JoinedAt,
			p.AddedBy,
			p.MutedUntil,
			p.Archived,
			p.LastReadSequence,
		}, err)
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresConversationRepository) RemoveParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM participants WHERE conversation_id = $1 AND user_id = $2", conversationID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresConversationRepository) GetParticipants(ctx context.Context, conversationID uuid.UUID) ([]conversation.Participant, error) {
	var participants []conversation.Participant
	rows, err := r.db.QueryContext(ctx, `
        SELECT p.conversation_id, p.user_id, p.role, p.joined_at, p.added_by, p.muted_until,
               p.archived, p.last_read_sequence,
               COALESCE(u.display_name, ''), COALESCE(u.username, ''), COALESCE(u.avatar_url, ''), COALESCE(u.is_online, false)
        FROM participants p
        LEFT JOIN users u ON u.id = p.user_id
        WHERE p.conversation_id = $1
    `, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p conversation.Participant
		if err := rows.Scan(
			&p.ConversationID,
			&p.UserID,
			&p.Role,
			&p.JoinedAt,
			&p.AddedBy,
			&p.MutedUntil,
			&p.Archived,
			&p.LastReadSequence,
			&p.DisplayName,
			&p.Username,
			&p.AvatarURL,
			&p.IsOnline,
		); err != nil {
			return nil, err
		}
		participants = append(participants, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return participants, nil
}

func (r *PostgresConversationRepository) GetParticipant(ctx context.Context, conversationID, userID uuid.UUID) (conversation.Participant, error) {
	var p conversation.Participant
	err := r.db.QueryRowContext(ctx, `
	        SELECT p.conversation_id, p.user_id, p.role, p.joined_at, p.added_by, p.muted_until,
	               p.archived, p.last_read_sequence,
	               COALESCE(u.display_name, ''), COALESCE(u.username, ''), COALESCE(u.avatar_url, ''), COALESCE(u.is_online, false)
	        FROM participants p
	        LEFT JOIN users u ON u.id = p.user_id
	        WHERE p.conversation_id = $1 AND p.user_id = $2
	    `, conversationID, userID).Scan(
		&p.ConversationID,
		&p.UserID,
		&p.Role,
		&p.JoinedAt,
		&p.AddedBy,
		&p.MutedUntil,
		&p.Archived,
		&p.LastReadSequence,
		&p.DisplayName,
		&p.Username,
		&p.AvatarURL,
		&p.IsOnline,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return conversation.Participant{}, sentinal_errors.ErrNotFound
		}
		return conversation.Participant{}, err
	}
	return p, nil
}

func (r *PostgresConversationRepository) UpdateParticipantRole(ctx context.Context, conversationID, userID uuid.UUID, role string) error {
	res, err := r.db.ExecContext(ctx, "UPDATE participants SET role = $1 WHERE conversation_id = $2 AND user_id = $3", role, conversationID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresConversationRepository) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM participants WHERE conversation_id = $1 AND user_id = $2", conversationID, userID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresConversationRepository) GetParticipantCount(ctx context.Context, conversationID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM participants WHERE conversation_id = $1", conversationID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *PostgresConversationRepository) MuteConversation(ctx context.Context, conversationID, userID uuid.UUID, until time.Time) error {
	res, err := r.db.ExecContext(ctx, "UPDATE participants SET muted_until = $1 WHERE conversation_id = $2 AND user_id = $3", until, conversationID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresConversationRepository) UnmuteConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE participants SET muted_until = NULL WHERE conversation_id = $1 AND user_id = $2", conversationID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresConversationRepository) ArchiveConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE participants SET archived = true WHERE conversation_id = $1 AND user_id = $2", conversationID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresConversationRepository) UnarchiveConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE participants SET archived = false WHERE conversation_id = $1 AND user_id = $2", conversationID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresConversationRepository) UpdateLastReadSequence(ctx context.Context, conversationID, userID uuid.UUID, seqID int64) error {
	res, err := r.db.ExecContext(ctx, "UPDATE participants SET last_read_sequence = $1 WHERE conversation_id = $2 AND user_id = $3", seqID, conversationID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresConversationRepository) ClearConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO conversation_clears (conversation_id, user_id, cleared_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT (conversation_id, user_id) DO UPDATE SET cleared_at = NOW()
    `, conversationID, userID)
	return err
}

func (r *PostgresConversationRepository) SetConversationClear(ctx context.Context, conversationID, userID uuid.UUID, clearedAt *time.Time) error {
	if clearedAt == nil {
		res, err := r.db.ExecContext(ctx, `DELETE FROM conversation_clears WHERE conversation_id = $1 AND user_id = $2`, conversationID, userID)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err == nil && rows == 0 {
			return sentinal_errors.ErrNotFound
		}
		return err
	}
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO conversation_clears (conversation_id, user_id, cleared_at)
        VALUES ($1, $2, $3)
        ON CONFLICT (conversation_id, user_id) DO UPDATE SET cleared_at = EXCLUDED.cleared_at
    `, conversationID, userID, clearedAt.UTC())
	return err
}

func (r *PostgresConversationRepository) GetConversationClear(ctx context.Context, conversationID, userID uuid.UUID) (conversation.ConversationClear, error) {
	var cc conversation.ConversationClear
	err := r.db.QueryRowContext(ctx, `
        SELECT conversation_id, user_id, cleared_at
        FROM conversation_clears WHERE conversation_id = $1 AND user_id = $2
    `, conversationID, userID).Scan(&cc.ConversationID, &cc.UserID, &cc.ClearedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return conversation.ConversationClear{}, sentinal_errors.ErrNotFound
		}
		return conversation.ConversationClear{}, err
	}
	return cc, nil
}

func (r *PostgresConversationRepository) GetConversationSequence(ctx context.Context, conversationID uuid.UUID) (conversation.ConversationSequence, error) {
	var seq conversation.ConversationSequence
	err := r.db.QueryRowContext(ctx, `
        SELECT conversation_id, last_sequence, updated_at
        FROM conversation_sequences WHERE conversation_id = $1
    `, conversationID).Scan(&seq.ConversationID, &seq.LastSequence, &seq.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return conversation.ConversationSequence{}, sentinal_errors.ErrNotFound
		}
		return conversation.ConversationSequence{}, err
	}
	return seq, nil
}

func (r *PostgresConversationRepository) IncrementSequence(ctx context.Context, conversationID uuid.UUID) (int64, error) {
	var seq conversation.ConversationSequence
	err := WithTx(ctx, r.db, func(tx DBTX) error {
		err := tx.QueryRowContext(ctx, `
            SELECT conversation_id, last_sequence, updated_at
            FROM conversation_sequences WHERE conversation_id = $1
        `, conversationID).Scan(&seq.ConversationID, &seq.LastSequence, &seq.UpdatedAt)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				seq = conversation.ConversationSequence{
					ConversationID: conversationID,
					LastSequence:   1,
					UpdatedAt:      time.Now(),
				}
				_, err := tx.ExecContext(ctx, `
                    INSERT INTO conversation_sequences (conversation_id, last_sequence, updated_at)
                    VALUES ($1,$2,$3)
                `, seq.ConversationID, seq.LastSequence, seq.UpdatedAt)
				return err
			}
			return err
		}

		seq.LastSequence++
		seq.UpdatedAt = time.Now()
		_, err = tx.ExecContext(ctx, `
            UPDATE conversation_sequences SET last_sequence = $1, updated_at = $2 WHERE conversation_id = $3
        `, seq.LastSequence, seq.UpdatedAt, seq.ConversationID)
		return err
	})
	if err != nil {
		return 0, err
	}
	return seq.LastSequence, nil
}
