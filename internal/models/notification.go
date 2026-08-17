package models

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Title     string    `gorm:"type:varchar(200);not null"`
	Message   string    `gorm:"type:text;not null"`
	Type      string    `gorm:"type:varchar(50);not null"`
	IsRead    bool      `gorm:"not null;default:false;index"`
	CreatedAt time.Time `gorm:"not null"`
	ReadAt    *time.Time

	User User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
