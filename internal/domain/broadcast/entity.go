package broadcast

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// BroadcastList represents broadcast_lists
type BroadcastList struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	OwnerID     uuid.UUID `gorm:"type:uuid;not null"`
	Name        string    `gorm:"not null"`
	Description sql.NullString
	CreatedAt   time.Time `gorm:"default:now()"`
}

// BroadcastRecipient represents broadcast_recipients
type BroadcastRecipient struct {
	BroadcastID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	AddedAt     time.Time `gorm:"default:now()"`
}

func (BroadcastList) TableName() string {
	return "broadcast_lists"
}

func (BroadcastRecipient) TableName() string {
	return "broadcast_recipients"
}
