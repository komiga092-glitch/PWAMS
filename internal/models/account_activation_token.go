package models

import (
	"time"

	"github.com/google/uuid"
)

type AccountActivationToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	OTP       string     `gorm:"size:6;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"not null;index" json:"-"`
	Verified  bool       `gorm:"default:false" json:"-"`
	UsedAt    *time.Time `json:"-"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
}
