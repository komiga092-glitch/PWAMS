package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	SyncOperationCreate = "CREATE"
	SyncOperationUpdate = "UPDATE"
	SyncOperationDelete = "DELETE"

	SyncStatusPending  = "PENDING"
	SyncStatusSynced   = "SYNCED"
	SyncStatusConflict = "CONFLICT"
)

const (
	SyncErrorConflict            = "SYNC_CONFLICT"
	SyncErrorValidation          = "VALIDATION_FAILED"
	SyncErrorIdempotencyMismatch = "IDEMPOTENCY_MISMATCH"
	SyncErrorUnauthorized        = "AUTH_EXPIRED"
)

type SyncPushRequest struct {
	Operations []SyncOperation `json:"operations" binding:"required"`
}

type SyncOperation struct {
	ID            uuid.UUID              `json:"id"`
	EntityType    string                 `json:"entity_type" binding:"required"`
	Operation     string                 `json:"operation" binding:"required"`
	RecordID      uuid.UUID              `json:"record_id" binding:"required"`
	ClientVersion int                    `json:"client_version"`
	UserID        uuid.UUID              `json:"user_id"`
	Payload       map[string]interface{} `json:"payload"`
	CreatedAt     time.Time              `json:"created_at"`
}

type SyncPushResponse struct {
	Success    bool         `json:"success"`
	Results    []SyncResult `json:"results"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

type SyncResult struct {
	OperationID      uuid.UUID `json:"operation_id"`
	EntityType       string    `json:"entity_type"`
	RecordID         uuid.UUID `json:"record_id"`
	Success          bool      `json:"success"`
	Code             string    `json:"code,omitempty"`
	Message          string    `json:"message,omitempty"`
	ServerVersion    int       `json:"server_version,omitempty"`
	ClientVersion    int       `json:"client_version,omitempty"`
	ValidationFaults []string  `json:"validation_faults,omitempty"`
}

type SyncPullResponse struct {
	Success bool             `json:"success"`
	Cursor  string           `json:"cursor"`
	HasMore bool             `json:"has_more"`
	Records []SyncPullRecord `json:"records"`
}

type SyncPullRecord struct {
	EntityType string                 `json:"entity_type"`
	RecordID   uuid.UUID              `json:"record_id"`
	Version    int                    `json:"version"`
	UpdatedAt  time.Time              `json:"updated_at"`
	IsDeleted  bool                   `json:"is_deleted"`
	Payload    map[string]interface{} `json:"payload"`
}
