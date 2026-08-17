package models

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    *uuid.UUID `gorm:"type:uuid" json:"user_id"`
	Action    string     `gorm:"size:100;not null" json:"action"`
	Entity    string     `gorm:"size:100;not null" json:"entity"`
	EntityID  *uuid.UUID `gorm:"type:uuid" json:"entity_id"`
	Details   string     `gorm:"type:text" json:"details"`
	IPAddress string     `gorm:"size:45" json:"ip_address"`
	CreatedAt time.Time  `json:"created_at"`
}

type AuditLogListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Action   string `form:"action"`
	Entity   string `form:"entity"`
	UserID   string `form:"user_id"`
}
