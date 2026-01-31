package call

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// SFUServer represents sfu_servers
type SFUServer struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Hostname      string    `gorm:"not null"`
	Region        string    `gorm:"not null"`
	Capacity      int       `gorm:"not null"`
	CurrentLoad   int       `gorm:"default:0"`
	IsHealthy     bool      `gorm:"default:true"`
	LastHeartbeat sql.NullTime
	CreatedAt     time.Time `gorm:"default:now()"`
}

// CallServerAssignment represents call_server_assignments
type CallServerAssignment struct {
	CallID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	SFUServerID uuid.UUID `gorm:"type:uuid;primaryKey"`
	AssignedAt  time.Time `gorm:"default:now()"`
}

func (SFUServer) TableName() string {
	return "sfu_servers"
}

func (CallServerAssignment) TableName() string {
	return "call_server_assignments"
}
