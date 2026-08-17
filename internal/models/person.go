package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	PersonStatusActive   = "Active"
	PersonStatusInactive = "Inactive"
	PersonStatusPending  = "Pending"
)

type Person struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	FullName    string `gorm:"size:150;not null;index" json:"full_name"`
	NICPassport string `gorm:"size:30;uniqueIndex;not null" json:"nic_passport"`

	DateOfBirth *time.Time `json:"date_of_birth,omitempty"`
	Gender      string     `gorm:"size:20" json:"gender"`

	Phone string `gorm:"size:20;index" json:"phone"`
	Email string `gorm:"size:100;index" json:"email"`

	Address       string  `gorm:"type:text" json:"address"`
	Occupation    string  `gorm:"size:100" json:"occupation"`
	MonthlyIncome float64 `gorm:"type:numeric(12,2);default:0" json:"monthly_income"`

	Status string `gorm:"size:20;not null;default:'Active';index" json:"status"`

	CreatedByID uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by_id"`
	CreatedBy   User      `gorm:"foreignKey:CreatedByID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"created_by"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (person *Person) BeforeCreate(_ *gorm.DB) error {
	if person.ID == uuid.Nil {
		person.ID = uuid.New()
	}

	if person.Status == "" {
		person.Status = PersonStatusActive
	}

	return nil
}

func (Person) TableName() string {
	return "persons"
}
