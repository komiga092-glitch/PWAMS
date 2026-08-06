package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DonorTypeIndividual   = "Individual"
	DonorTypeOrganization = "Organization"

	DonorStatusActive   = "Active"
	DonorStatusInactive = "Inactive"
	DonorStatusPending  = "Pending"
)

type Donor struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	Name      string `gorm:"size:150;not null;index" json:"name"`
	DonorType string `gorm:"size:30;not null;index" json:"donor_type"`

	NICPassport        string `gorm:"size:30;index" json:"nic_passport"`
	OrganizationName   string `gorm:"size:150;index" json:"organization_name"`
	RegistrationNumber string `gorm:"size:50;index" json:"registration_number"`

	Phone   string `gorm:"size:20;index" json:"phone"`
	Email   string `gorm:"size:100;index" json:"email"`
	Address string `gorm:"type:text" json:"address"`

	ContactPersonName  string `gorm:"size:150" json:"contact_person_name"`
	ContactPersonPhone string `gorm:"size:20" json:"contact_person_phone"`

	PreferredDonationType string `gorm:"size:50" json:"preferred_donation_type"`
	Notes                 string `gorm:"type:text" json:"notes"`

	Status string `gorm:"size:20;not null;default:Active;index" json:"status"`

	CreatedByID uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by_id"`
	CreatedBy   User      `gorm:"foreignKey:CreatedByID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"created_by"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (donor *Donor) BeforeCreate(_ *gorm.DB) error {
	if donor.ID == uuid.Nil {
		donor.ID = uuid.New()
	}

	if donor.Status == "" {
		donor.Status = DonorStatusActive
	}

	return nil
}

func (Donor) TableName() string {
	return "donors"
}
