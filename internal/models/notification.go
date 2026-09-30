package models

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	Title     string     `gorm:"type:varchar(200);not null" json:"title"`
	Message   string     `gorm:"type:text;not null" json:"message"`
	Type      string     `gorm:"type:varchar(50);not null" json:"type"`
	IsRead    bool       `gorm:"not null;default:false;index" json:"is_read"`
	CreatedAt time.Time  `gorm:"not null" json:"created_at"`
	ReadAt    *time.Time `json:"read_at"`

	// The association is never preloaded by NotificationRepository, so it is
	// excluded from JSON instead of leaking an all-zero user object into the
	// API response (the frontend reads the notification's own fields only).
	User User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}
