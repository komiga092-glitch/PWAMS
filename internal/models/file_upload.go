package models

import (
	"time"

	"github.com/google/uuid"
)

type FileUpload struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID  `gorm:"type:uuid;not null;index"`
	OriginalName string     `gorm:"type:varchar(255);not null"`
	StoredName   string     `gorm:"type:varchar(255);not null;uniqueIndex"`
	Path         string     `gorm:"type:text;not null"`
	ContentType  string     `gorm:"type:varchar(100);not null"`
	Size         int64      `gorm:"not null"`
	CreatedAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP"`
	DeletedAt    *time.Time `gorm:"index"`
}

func (FileUpload) TableName() string {
	return "file_uploads"
}
