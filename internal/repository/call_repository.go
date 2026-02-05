package repository

import (
	"context"
	"errors"
	"time"

	"sentinal-chat/internal/domain/call"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresCallRepository struct {
	db *gorm.DB
}

func NewCallRepository(db *gorm.DB) CallRepository {
	return &PostgresCallRepository{db: db}
}

func (r *PostgresCallRepository) Create(ctx context.Context, c *call.Call) error {
	res := r.db.WithContext(ctx).Create(c)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresCallRepository) GetByID(ctx context.Context, id uuid.UUID) (call.Call, error) {
	var c call.Call
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return call.Call{}, sentinal_errors.ErrNotFound
		}
		return call.Call{}, err
	}
	return c, nil
}

func (r *PostgresCallRepository) Update(ctx context.Context, c call.Call) error {
	res := r.db.WithContext(ctx).Save(&c)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresCallRepository) GetConversationCalls(ctx context.Context, conversationID uuid.UUID, page, limit int) ([]call.Call, int64, error) {
	var calls []call.Call
	var total int64

	q := r.db.WithContext(ctx).
		Model(&call.Call{}).
		Where("conversation_id = ?", conversationID)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := q.Order("started_at DESC").Offset(offset).Limit(limit).Find(&calls).Error; err != nil {
		return nil, 0, err
	}

	return calls, total, nil
}

func (r *PostgresCallRepository) GetUserCalls(ctx context.Context, userID uuid.UUID, page, limit int) ([]call.Call, int64, error) {
	var calls []call.Call
	var total int64

	// Get calls where user is initiator or participant
	subQuery := r.db.Model(&call.CallParticipant{}).
		Select("call_id").
		Where("user_id = ?", userID)

	q := r.db.WithContext(ctx).
		Model(&call.Call{}).
		Where("initiated_by = ? OR id IN (?)", userID, subQuery)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := q.Order("started_at DESC").Offset(offset).Limit(limit).Find(&calls).Error; err != nil {
		return nil, 0, err
	}

	return calls, total, nil
}

func (r *PostgresCallRepository) GetActiveCalls(ctx context.Context, userID uuid.UUID) ([]call.Call, error) {
	var calls []call.Call

	subQuery := r.db.Model(&call.CallParticipant{}).
		Select("call_id").
		Where("user_id = ? AND status IN ('INVITED', 'JOINED')", userID)

	err := r.db.WithContext(ctx).
		Where("ended_at IS NULL AND (initiated_by = ? OR id IN (?))", userID, subQuery).
		Find(&calls).Error

	if err != nil {
		return nil, err
	}
	return calls, nil
}

func (r *PostgresCallRepository) GetMissedCalls(ctx context.Context, userID uuid.UUID, since time.Time) ([]call.Call, error) {
	var calls []call.Call

	// Calls where user was invited but didn't join and call ended
	subQuery := r.db.Model(&call.CallParticipant{}).
		Select("call_id").
		Where("user_id = ? AND status = 'INVITED' AND joined_at IS NULL", userID)

	err := r.db.WithContext(ctx).
		Where("id IN (?) AND ended_at IS NOT NULL AND started_at > ?", subQuery, since).
		Order("started_at DESC").
		Find(&calls).Error

	if err != nil {
		return nil, err
	}
	return calls, nil
}

func (r *PostgresCallRepository) MarkConnected(ctx context.Context, callID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&call.Call{}).
		Where("id = ?", callID).
		Update("connected_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresCallRepository) EndCall(ctx context.Context, callID uuid.UUID, reason string) error {
	now := time.Now()

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var c call.Call
		if err := tx.Where("id = ?", callID).First(&c).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return sentinal_errors.ErrNotFound
			}
			return err
		}

		updates := map[string]interface{}{
			"ended_at":   now,
			"end_reason": reason,
		}

		// Calculate duration if connected
		if c.ConnectedAt.Valid {
			duration := int32(now.Sub(c.ConnectedAt.Time).Seconds())
			updates["duration_seconds"] = duration
		}

		return tx.Model(&call.Call{}).Where("id = ?", callID).Updates(updates).Error
	})
}

func (r *PostgresCallRepository) GetCallDuration(ctx context.Context, callID uuid.UUID) (int32, error) {
	var c call.Call
	err := r.db.WithContext(ctx).Select("duration_seconds").Where("id = ?", callID).First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, sentinal_errors.ErrNotFound
		}
		return 0, err
	}
	if c.DurationSeconds.Valid {
		return c.DurationSeconds.Int32, nil
	}
	return 0, nil
}

func (r *PostgresCallRepository) AddParticipant(ctx context.Context, p *call.CallParticipant) error {
	res := r.db.WithContext(ctx).Create(p)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresCallRepository) RemoveParticipant(ctx context.Context, callID, userID uuid.UUID) error {
	now := time.Now()
	res := r.db.WithContext(ctx).
		Model(&call.CallParticipant{}).
		Where("call_id = ? AND user_id = ?", callID, userID).
		Updates(map[string]interface{}{
			"status":  "LEFT",
			"left_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresCallRepository) GetCallParticipants(ctx context.Context, callID uuid.UUID) ([]call.CallParticipant, error) {
	var participants []call.CallParticipant
	err := r.db.WithContext(ctx).
		Where("call_id = ?", callID).
		Find(&participants).Error
	if err != nil {
		return nil, err
	}
	return participants, nil
}

func (r *PostgresCallRepository) IsCallParticipant(ctx context.Context, callID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&call.CallParticipant{}).
		Where("call_id = ? AND user_id = ?", callID, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

var (
	StatusJoined = "JOINED"
	StatusLeft   = "LEFT"
)

func (r *PostgresCallRepository) UpdateParticipantStatus(ctx context.Context, callID, userID uuid.UUID, status string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	switch status {
	case StatusJoined:
		updates["joined_at"] = time.Now()
	case StatusLeft:
		updates["left_at"] = time.Now()
	}

	res := r.db.WithContext(ctx).
		Model(&call.CallParticipant{}).
		Where("call_id = ? AND user_id = ?", callID, userID).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresCallRepository) UpdateParticipantMuteStatus(ctx context.Context, callID, userID uuid.UUID, audioMuted, videoMuted bool) error {
	res := r.db.WithContext(ctx).
		Model(&call.CallParticipant{}).
		Where("call_id = ? AND user_id = ?", callID, userID).
		Updates(map[string]interface{}{
			"muted_audio": audioMuted,
			"muted_video": videoMuted,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresCallRepository) GetActiveParticipantCount(ctx context.Context, callID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&call.CallParticipant{}).
		Where("call_id = ? AND status = 'JOINED'", callID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *PostgresCallRepository) RecordQualityMetric(ctx context.Context, m *call.CallQualityMetric) error {
	res := r.db.WithContext(ctx).Create(m)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *PostgresCallRepository) GetCallQualityMetrics(ctx context.Context, callID uuid.UUID) ([]call.CallQualityMetric, error) {
	var metrics []call.CallQualityMetric
	err := r.db.WithContext(ctx).
		Where("call_id = ?", callID).
		Order("recorded_at ASC").
		Find(&metrics).Error
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func (r *PostgresCallRepository) GetUserCallQualityMetrics(ctx context.Context, callID, userID uuid.UUID) ([]call.CallQualityMetric, error) {
	var metrics []call.CallQualityMetric
	err := r.db.WithContext(ctx).
		Where("call_id = ? AND user_id = ?", callID, userID).
		Order("recorded_at ASC").
		Find(&metrics).Error
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func (r *PostgresCallRepository) GetAverageCallQuality(ctx context.Context, callID uuid.UUID) (float64, error) {
	var result struct {
		AvgJitter float64
	}
	err := r.db.WithContext(ctx).
		Model(&call.CallQualityMetric{}).
		Select("AVG(jitter_ms) as avg_jitter").
		Where("call_id = ?", callID).
		Scan(&result).Error
	if err != nil {
		return 0, err
	}
	return result.AvgJitter, nil
}

func (r *PostgresCallRepository) CreateTurnCredential(ctx context.Context, tc *call.TurnCredential) error {
	res := r.db.WithContext(ctx).Create(tc)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *PostgresCallRepository) GetActiveTurnCredentials(ctx context.Context, userID uuid.UUID) ([]call.TurnCredential, error) {
	var creds []call.TurnCredential
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND expires_at > NOW()", userID).
		Find(&creds).Error
	if err != nil {
		return nil, err
	}
	return creds, nil
}

func (r *PostgresCallRepository) DeleteExpiredTurnCredentials(ctx context.Context) (int64, error) {
	res := r.db.WithContext(ctx).
		Delete(&call.TurnCredential{}, "expires_at < NOW()")
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

func (r *PostgresCallRepository) CreateSFUServer(ctx context.Context, s *call.SFUServer) error {
	res := r.db.WithContext(ctx).Create(s)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresCallRepository) GetSFUServerByID(ctx context.Context, id uuid.UUID) (call.SFUServer, error) {
	var s call.SFUServer
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return call.SFUServer{}, sentinal_errors.ErrNotFound
		}
		return call.SFUServer{}, err
	}
	return s, nil
}

func (r *PostgresCallRepository) GetHealthySFUServers(ctx context.Context, region string) ([]call.SFUServer, error) {
	var servers []call.SFUServer
	q := r.db.WithContext(ctx).Where("is_healthy = true")
	if region != "" {
		q = q.Where("region = ?", region)
	}
	err := q.Find(&servers).Error
	if err != nil {
		return nil, err
	}
	return servers, nil
}

func (r *PostgresCallRepository) GetLeastLoadedServer(ctx context.Context, region string) (call.SFUServer, error) {
	var s call.SFUServer
	q := r.db.WithContext(ctx).
		Where("is_healthy = true AND current_load < capacity")
	if region != "" {
		q = q.Where("region = ?", region)
	}
	err := q.Order("(CAST(current_load AS FLOAT) / capacity) ASC").First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return call.SFUServer{}, sentinal_errors.ErrNotFound
		}
		return call.SFUServer{}, err
	}
	return s, nil
}

func (r *PostgresCallRepository) UpdateServerLoad(ctx context.Context, serverID uuid.UUID, load int) error {
	res := r.db.WithContext(ctx).
		Model(&call.SFUServer{}).
		Where("id = ?", serverID).
		Update("current_load", load)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresCallRepository) UpdateServerHealth(ctx context.Context, serverID uuid.UUID, isHealthy bool) error {
	res := r.db.WithContext(ctx).
		Model(&call.SFUServer{}).
		Where("id = ?", serverID).
		Update("is_healthy", isHealthy)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresCallRepository) UpdateServerHeartbeat(ctx context.Context, serverID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&call.SFUServer{}).
		Where("id = ?", serverID).
		Update("last_heartbeat", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresCallRepository) AssignCallToServer(ctx context.Context, a *call.CallServerAssignment) error {
	res := r.db.WithContext(ctx).Create(a)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresCallRepository) GetCallServerAssignments(ctx context.Context, callID uuid.UUID) ([]call.CallServerAssignment, error) {
	var assignments []call.CallServerAssignment
	err := r.db.WithContext(ctx).
		Where("call_id = ?", callID).
		Find(&assignments).Error
	if err != nil {
		return nil, err
	}
	return assignments, nil
}

func (r *PostgresCallRepository) RemoveCallServerAssignment(ctx context.Context, callID, serverID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Delete(&call.CallServerAssignment{}, "call_id = ? AND sfu_server_id = ?", callID, serverID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}
