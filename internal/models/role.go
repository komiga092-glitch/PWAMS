package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	RoleSuperAdmin  = "Super Admin"
	RoleAdmin       = "Admin"
	RoleStaff       = "Staff"
	RoleVolunteer   = "Volunteer"
	RoleDonor       = "Donor"
	RoleBeneficiary = "Beneficiary"
)

type Role struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string         `gorm:"size:50;uniqueIndex;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (role *Role) BeforeCreate(_ *gorm.DB) error {
	if role.ID == uuid.Nil {
		role.ID = uuid.New()
	}

	return nil
}
