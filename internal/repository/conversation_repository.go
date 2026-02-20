package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"sentinal-chat/internal/domain/conversation"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type PostgresConversationRepository struct {
	db DBTX
}

func NewConversationRepository(db DBTX) ConversationRepository {
	return &PostgresConversationRepository{db: db}
}

func (r *PostgresConversationRepository) Create(ctx context.Context, c *conversation.Conversation) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO conversations (
            id, type, subject, description, avatar_url, expiry_seconds, disappearing_mode, message_expiry_seconds,
            group_permissions, invite_link, invite_link_revoked_at, created_by, created_at, updated_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
    `,
		c.ID,
		c.Type,
		c.Subject,
		c.Description,
		c.AvatarURL,
		c.ExpirySeconds,
		c.DisappearingMode,
		c.MessageExpirySeconds,
		c.GroupPermissions,
		c.InviteLink,
		c.InviteLinkRevokedAt,
		c.CreatedBy,
		c.CreatedAt,
		c.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresConversationRepository) GetByID(ctx context.Context, id uuid.UUID) (conversation.Conversation, error) {
	var c conversation.Conversation
	err := r.db.QueryRowContext(ctx, `
        SELECT id, type, subject, description, avatar_url, expiry_seconds, disappearing_mode, message_expiry_seconds,
               group_permissions, invite_link, invite_link_revoked_at, created_by, created_at, updated_at
        FROM conversations WHERE id = $1
    `, id).Scan(
		&c.ID,
		&c.Type,
		&c.Subject,
		&c.Description,
		&c.AvatarURL,
		&c.ExpirySeconds,
		&c.DisappearingMode,
		&c.MessageExpirySeconds,
		&c.GroupPermissions,
		&c.InviteLink,
		&c.InviteLinkRevokedAt,
		&c.CreatedBy,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
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
        SET type = $1, subject = $2, description = $3, avatar_url = $4, expiry_seconds = $5, disappearing_mode = $6,
            message_expiry_seconds = $7, group_permissions = $8, invite_link = $9, invite_link_revoked_at = $10,
            created_by = $11, updated_at = $12
        WHERE id = $13
    `,
		c.Type,
		c.Subject,
		c.Description,
		c.AvatarURL,
		c.ExpirySeconds,
		c.DisappearingMode,
		c.MessageExpirySeconds,
		c.GroupPermissions,
		c.InviteLink,
		c.InviteLinkRevokedAt,
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
        WHERE c.id IN (SELECT conversation_id FROM participants WHERE user_id = $1)
    `, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
        SELECT c.id, c.type, c.subject, c.description, c.avatar_url, c.expiry_seconds, c.disappearing_mode,
               c.message_expiry_seconds, c.group_permissions, c.invite_link, c.invite_link_revoked_at, c.created_by,
               c.created_at, c.updated_at
        FROM conversations c
        WHERE c.id IN (SELECT conversation_id FROM participants WHERE user_id = $1)
        ORDER BY c.updated_at DESC
        OFFSET $2 LIMIT $3
    `, userID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var c conversation.Conversation
		if err := rows.Scan(
			&c.ID,
			&c.Type,
			&c.Subject,
			&c.Description,
			&c.AvatarURL,
			&c.ExpirySeconds,
			&c.DisappearingMode,
			&c.MessageExpirySeconds,
			&c.GroupPermissions,
			&c.InviteLink,
			&c.InviteLinkRevokedAt,
			&c.CreatedBy,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, 0, err
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
	var c conversation.Conversation
	err := r.db.QueryRowContext(ctx, `
        SELECT c.id, c.type, c.subject, c.description, c.avatar_url, c.expiry_seconds, c.disappearing_mode,
               c.message_expiry_seconds, c.group_permissions, c.invite_link, c.invite_link_revoked_at, c.created_by,
               c.created_at, c.updated_at
        FROM conversations c
        WHERE c.type = 'DM' AND c.id IN (
            SELECT conversation_id
            FROM participants
            WHERE user_id IN ($1,$2)
            GROUP BY conversation_id
            HAVING COUNT(DISTINCT user_id) = 2
        )
        LIMIT 1
    `, userID1, userID2).Scan(
		&c.ID,
		&c.Type,
		&c.Subject,
		&c.Description,
		&c.AvatarURL,
		&c.ExpirySeconds,
		&c.DisappearingMode,
		&c.MessageExpirySeconds,
		&c.GroupPermissions,
		&c.InviteLink,
		&c.InviteLinkRevokedAt,
		&c.CreatedBy,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
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

func (r *PostgresConversationRepository) SearchConversations(ctx context.Context, userID uuid.UUID, query string) ([]conversation.Conversation, error) {
	var conversations []conversation.Conversation
	rows, err := r.db.QueryContext(ctx, `
        SELECT c.id, c.type, c.subject, c.description, c.avatar_url, c.expiry_seconds, c.disappearing_mode,
               c.message_expiry_seconds, c.group_permissions, c.invite_link, c.invite_link_revoked_at, c.created_by,
               c.created_at, c.updated_at
        FROM conversations c
        WHERE c.id IN (SELECT conversation_id FROM participants WHERE user_id = $1)
          AND c.subject ILIKE $2
    `, userID, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c conversation.Conversation
		if err := rows.Scan(
			&c.ID,
			&c.Type,
			&c.Subject,
			&c.Description,
			&c.AvatarURL,
			&c.ExpirySeconds,
			&c.DisappearingMode,
			&c.MessageExpirySeconds,
			&c.GroupPermissions,
			&c.InviteLink,
			&c.InviteLinkRevokedAt,
			&c.CreatedBy,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
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
        SELECT c.id, c.type, c.subject, c.description, c.avatar_url, c.expiry_seconds, c.disappearing_mode,
               c.message_expiry_seconds, c.group_permissions, c.invite_link, c.invite_link_revoked_at, c.created_by,
               c.created_at, c.updated_at
        FROM conversations c
        WHERE c.id IN (SELECT conversation_id FROM participants WHERE user_id = $1)
          AND c.type = $2
    `, userID, convType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c conversation.Conversation
		if err := rows.Scan(
			&c.ID,
			&c.Type,
			&c.Subject,
			&c.Description,
			&c.AvatarURL,
			&c.ExpirySeconds,
			&c.DisappearingMode,
			&c.MessageExpirySeconds,
			&c.GroupPermissions,
			&c.InviteLink,
			&c.InviteLinkRevokedAt,
			&c.CreatedBy,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
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
	var c conversation.Conversation
	err := r.db.QueryRowContext(ctx, `
        SELECT id, type, subject, description, avatar_url, expiry_seconds, disappearing_mode, message_expiry_seconds,
               group_permissions, invite_link, invite_link_revoked_at, created_by, created_at, updated_at
        FROM conversations
        WHERE invite_link = $1 AND (invite_link_revoked_at IS NULL OR invite_link_revoked_at > NOW())
    `, link).Scan(
		&c.ID,
		&c.Type,
		&c.Subject,
		&c.Description,
		&c.AvatarURL,
		&c.ExpirySeconds,
		&c.DisappearingMode,
		&c.MessageExpirySeconds,
		&c.GroupPermissions,
		&c.InviteLink,
		&c.InviteLinkRevokedAt,
		&c.CreatedBy,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
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
        INSERT INTO participants (conversation_id, user_id, role, joined_at, added_by, muted_until, pinned_at, archived, last_read_sequence, permissions)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
    `,
		p.ConversationID,
		p.UserID,
		p.Role,
		p.JoinedAt,
		p.AddedBy,
		p.MutedUntil,
		p.PinnedAt,
		p.Archived,
		p.LastReadSequence,
		p.Permissions,
	)
	if err != nil {
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
        SELECT conversation_id, user_id, role, joined_at, added_by, muted_until, pinned_at, archived, last_read_sequence, permissions
        FROM participants WHERE conversation_id = $1
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
			&p.PinnedAt,
			&p.Archived,
			&p.LastReadSequence,
			&p.Permissions,
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
        SELECT conversation_id, user_id, role, joined_at, added_by, muted_until, pinned_at, archived, last_read_sequence, permissions
        FROM participants WHERE conversation_id = $1 AND user_id = $2
    `, conversationID, userID).Scan(
		&p.ConversationID,
		&p.UserID,
		&p.Role,
		&p.JoinedAt,
		&p.AddedBy,
		&p.MutedUntil,
		&p.PinnedAt,
		&p.Archived,
		&p.LastReadSequence,
		&p.Permissions,
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

func (r *PostgresConversationRepository) PinConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE participants SET pinned_at = $1 WHERE conversation_id = $2 AND user_id = $3", time.Now(), conversationID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresConversationRepository) UnpinConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE participants SET pinned_at = NULL WHERE conversation_id = $1 AND user_id = $2", conversationID, userID)
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
