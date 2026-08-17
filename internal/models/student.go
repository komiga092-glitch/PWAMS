package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	StudentStatusActive   = "Active"
	StudentStatusInactive = "Inactive"
	StudentStatusPending  = "Pending"
)

type Student struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	PersonID uuid.UUID `gorm:"type:uuid;not null;index" json:"person_id"`
	Person   Person    `gorm:"foreignKey:PersonID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"person"`

	FullName    string `gorm:"size:150;not null;index" json:"full_name"`
	SchoolName  string `gorm:"size:150;not null;index" json:"school_name"`
	Grade       string `gorm:"size:30;not null" json:"grade"`
	StudentCode string `gorm:"size:50;uniqueIndex" json:"student_code"`

	DateOfBirth *time.Time `json:"date_of_birth,omitempty"`
	Gender      string     `gorm:"size:20" json:"gender"`

	GuardianName  string `gorm:"size:150" json:"guardian_name"`
	GuardianPhone string `gorm:"size:20" json:"guardian_phone"`

	AcademicYear int    `gorm:"not null" json:"academic_year"`
	Remarks      string `gorm:"type:text" json:"remarks"`

	Status string `gorm:"size:20;not null;default:Active;index" json:"status"`

	CreatedByID uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by_id"`
	CreatedBy   User      `gorm:"foreignKey:CreatedByID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"created_by"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (student *Student) BeforeCreate(_ *gorm.DB) error {
	if student.ID == uuid.Nil {
		student.ID = uuid.New()
	}

	if student.Status == "" {
		student.Status = StudentStatusActive
	}

	return nil
}
func (Student) TableName() string {
	return "students"
}
