package broadcast

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// BroadcastList represents broadcast_lists
type BroadcastList struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Name        string
	Description sql.NullString
	CreatedAt   time.Time
}

// BroadcastRecipient represents broadcast_recipients
type BroadcastRecipient struct {
	BroadcastID uuid.UUID
	UserID      uuid.UUID
	AddedAt     time.Time
}

func (BroadcastList) TableName() string {
	return "broadcast_lists"
}

func (BroadcastRecipient) TableName() string {
	return "broadcast_recipients"
}
