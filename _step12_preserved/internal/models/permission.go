package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Permission represents a single atomic capability in the system.
// Permissions follow the "resource.action" convention, e.g.:
//   - users.create
//   - donor.delete
//   - aid.approve
type Permission struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Category    string    `gorm:"size:50;index" json:"category"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (permission *Permission) BeforeCreate(_ *gorm.DB) error {
	if permission.ID == uuid.Nil {
		permission.ID = uuid.New()
	}

	return nil
}

// RolePermission maps a role to a permission (many-to-many).
type RolePermission struct {
	RoleID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserPermission grants (or explicitly revokes) a single permission
// for a specific user, overriding the role default.
// Granted = false means the permission is explicitly denied for this user.
type UserPermission struct {
	UserID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	Granted      bool      `gorm:"not null;default:true" json:"granted"`
	CreatedByID  uuid.UUID `gorm:"type:uuid;not null" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}
