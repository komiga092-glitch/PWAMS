package models

import (
	"time"

	"github.com/google/uuid"
)

type SyncIdempotencyRecord struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	IdempotencyKey string    `gorm:"size:100;uniqueIndex;not null" json:"idempotency_key"`

	RequestHash string `gorm:"type:text;not null" json:"request_hash"`

	ResponseBody string `gorm:"type:text;not null" json:"response_body"`

	StatusCode int `gorm:"not null" json:"status_code"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SyncIdempotencyRecord) TableName() string {
	return "sync_idempotency_records"
}
