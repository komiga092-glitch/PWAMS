package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DonationTypeCash      = "Cash"
	DonationTypeFood      = "Food"
	DonationTypeMedical   = "Medical"
	DonationTypeEducation = "Education"
	DonationTypeClothing  = "Clothing"
	DonationTypeEquipment = "Equipment"
	DonationTypeOther     = "Other"

	DonationStatusPending   = "Pending"
	DonationStatusConfirmed = "Confirmed"
	DonationStatusCancelled = "Cancelled"
)

type Donation struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	DonorID uuid.UUID `gorm:"type:uuid;not null;index" json:"donor_id"`
	Donor   Donor     `gorm:"foreignKey:DonorID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"donor"`

	PersonID *uuid.UUID `gorm:"type:uuid;index" json:"person_id,omitempty"`
	Person   *Person    `gorm:"foreignKey:PersonID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"person,omitempty"`

	DonationType string `gorm:"size:50;not null;index" json:"donation_type"`

	Amount      float64 `gorm:"type:numeric(14,2);default:0" json:"amount"`
	Currency    string  `gorm:"size:10;default:LKR" json:"currency"`
	ItemName    string  `gorm:"size:150" json:"item_name"`
	Quantity    float64 `gorm:"type:numeric(12,2);default:0" json:"quantity"`
	Unit        string  `gorm:"size:30" json:"unit"`
	Description string  `gorm:"type:text" json:"description"`

	DonationDate time.Time `gorm:"not null;index" json:"donation_date"`
	ReferenceNo  string    `gorm:"size:100;uniqueIndex" json:"reference_no"`

	Status string `gorm:"size:20;not null;default:Pending;index" json:"status"`

	CreatedByID uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by_id"`
	CreatedBy   User      `gorm:"foreignKey:CreatedByID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"created_by"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (donation *Donation) BeforeCreate(_ *gorm.DB) error {
	if donation.ID == uuid.Nil {
		donation.ID = uuid.New()
	}

	if donation.Status == "" {
		donation.Status = DonationStatusPending
	}

	if donation.Currency == "" {
		donation.Currency = "LKR"
	}

	if donation.DonationDate.IsZero() {
		donation.DonationDate = time.Now().UTC()
	}

	return nil
}

func (Donation) TableName() string {
	return "donations"
}
