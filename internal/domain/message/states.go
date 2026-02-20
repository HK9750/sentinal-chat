package message

import (
	"database/sql"

	"github.com/google/uuid"
)

// MessageUserState represents message_user_states
type MessageUserState struct {
	MessageID uuid.UUID
	UserID    uuid.UUID
	IsDeleted bool
	DeletedAt sql.NullTime
	IsStarred bool
	IsPinned  bool
}

func (MessageUserState) TableName() string {
	return "message_user_states"
}
