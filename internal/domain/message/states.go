package message

import (
	"database/sql"

	"github.com/google/uuid"
)

// MessageUserState represents message_user_states
type MessageUserState struct {
	MessageID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	IsDeleted bool      `gorm:"default:false"`
	DeletedAt sql.NullTime
	IsStarred bool `gorm:"default:false"`
	IsPinned  bool `gorm:"default:false"`
}

func (MessageUserState) TableName() string {
	return "message_user_states"
}
