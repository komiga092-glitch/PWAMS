package models

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID       uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID   *uuid.UUID `gorm:"type:uuid" json:"user_id"`
	Action   string     `gorm:"size:100;not null" json:"action"`
	Entity   string     `gorm:"size:100;not null" json:"entity"`
	EntityID *uuid.UUID `gorm:"type:uuid" json:"entity_id"`
	Details  string     `gorm:"type:text" json:"details"`
	OldValue string     `gorm:"type:text" json:"old_value"`
	NewValue string     `gorm:"type:text" json:"new_value"`
	// Audit logs must never store client IP addresses (PII). The
	// ip_address column is deliberately dropped by migration
	// 000002_drop_audit_ip; do not reintroduce an IP field here or
	// AutoMigrate will silently re-create the dropped column.
	RequestID string     `gorm:"size:100;index" json:"request_id"`
	TenantID  *uuid.UUID `gorm:"type:uuid;index" json:"tenant_id"`
	CreatedAt time.Time  `json:"created_at"`
}

type AuditLogListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Action   string `form:"action"`
	Entity   string `form:"entity"`
	UserID   string `form:"user_id"`
}
