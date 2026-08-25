package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Message struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	SenderID uuid.UUID `gorm:"type:uuid;not null;index:idx_messages_sender_created_at,priority:1" json:"sender_id"`
	Sender   User      `gorm:"foreignKey:SenderID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"sender"`

	RecipientID uuid.UUID `gorm:"type:uuid;not null;index:idx_messages_recipient_created_at,priority:1;index:idx_messages_recipient_unread,priority:1" json:"recipient_id"`
	Recipient   User      `gorm:"foreignKey:RecipientID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"recipient"`

	Subject string `gorm:"size:200;not null" json:"subject"`
	Body    string `gorm:"type:text;not null" json:"body"`

	IsRead bool       `gorm:"not null;default:false;index:idx_messages_recipient_unread,priority:2" json:"is_read"`
	ReadAt *time.Time `json:"read_at,omitempty"`

	IsDeleted bool           `gorm:"not null;default:false;index" json:"is_deleted"`
	CreatedAt time.Time      `gorm:"index:idx_messages_recipient_created_at,priority:2,sort:desc;index:idx_messages_sender_created_at,priority:2,sort:desc;index:idx_messages_created_at,sort:desc" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (message *Message) BeforeCreate(_ *gorm.DB) error {
	if message.ID == uuid.Nil {
		message.ID = uuid.New()
	}

	return nil
}

func (Message) TableName() string {
	return "messages"
}
