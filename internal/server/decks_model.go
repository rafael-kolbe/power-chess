package server

import (
	"time"
)

// userDeckModel is a persisted constructed deck. Primary key is ID; names may duplicate per user.
type userDeckModel struct {
	ID            uint64 `gorm:"primaryKey;autoIncrement"`
	UserID        uint64 `gorm:"index:idx_user_decks_user_id,not null"`
	Name          string `gorm:"size:128;not null"`
	CardIDsJSON   []byte `gorm:"type:jsonb;not null"`
	PlayerSkillID string `gorm:"size:64;not null"`
	SleeveColor   string `gorm:"size:16;not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (userDeckModel) TableName() string { return "user_decks" }
