package services

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrPersonSyncConflict    = errors.New("person sync conflict")
	ErrPersonSyncUnsupported = errors.New("unsupported person sync operation")
	ErrPersonSyncUser        = errors.New("invalid sync user id")
	ErrPersonSyncRecordID    = errors.New("invalid person record id")
)

type PersonSyncService struct {
	personRepo *repository.PersonRepository
}

func NewPersonSyncService(
	personRepo *repository.PersonRepository,
) *PersonSyncService {
	return &PersonSyncService{
		personRepo: personRepo,
	}
}

func (s *PersonSyncService) Apply(
	operation models.SyncOperation,
	tenantID *uuid.UUID,
) (*models.SyncResult, error) {
	entityType := strings.ToLower(
		strings.TrimSpace(operation.EntityType),
	)

	if entityType != "person" && entityType != "persons" {
		return nil, ErrPersonSyncUnsupported
	}

	switch strings.ToUpper(strings.TrimSpace(operation.Operation)) {
	case models.SyncOperationCreate:
		return s.create(operation, tenantID)

	case models.SyncOperationUpdate:
		return s.update(operation, tenantID)

	case models.SyncOperationDelete:
		return s.delete(operation, tenantID)

	default:
		return nil, ErrPersonSyncUnsupported
	}
}

func (s *PersonSyncService) create(
	operation models.SyncOperation,
	tenantID *uuid.UUID,
) (*models.SyncResult, error) {
	if operation.ID == uuid.Nil {
		return nil, errors.New("invalid sync operation id")
	}

	if operation.RecordID == uuid.Nil {
		return nil, ErrPersonSyncRecordID
	}

	if operation.UserID == uuid.Nil {
		return nil, ErrPersonSyncUser
	}

	payload, err := json.Marshal(operation.Payload)
	if err != nil {
		return nil, err
	}

	var person models.Person

	if err := json.Unmarshal(payload, &person); err != nil {
		return nil, err
	}

	// The server trusts the operation record ID,
	// not an ID supplied inside the client payload.
	person.ID = operation.RecordID

	person.CreatedByID = operation.UserID
	person.UpdatedBy = &operation.UserID
	person.Version = 1
	person.IsDeleted = false
	// The tenant comes from the authenticated principal, never from
	// the client payload. A principal without a tenant (single-tenant
	// deployment) records a NULL tenant.
	person.TenantID = tenantID

	if err := s.personRepo.Create(&person); err != nil {
		return nil, err
	}

	return &models.SyncResult{
		OperationID:   operation.ID,
		EntityType:    "person",
		RecordID:      person.ID,
		Success:       true,
		ServerVersion: person.Version,
		ClientVersion: operation.ClientVersion,
	}, nil
}

func (s *PersonSyncService) update(
	operation models.SyncOperation,
	tenantID *uuid.UUID,
) (*models.SyncResult, error) {
	if operation.ID == uuid.Nil {
		return nil, errors.New("invalid sync operation id")
	}

	if operation.RecordID == uuid.Nil {
		return nil, ErrPersonSyncRecordID
	}

	if operation.UserID == uuid.Nil {
		return nil, ErrPersonSyncUser
	}

	person, err := s.personRepo.FindByID(
		operation.RecordID.String(),
	)
	if err != nil {
		return nil, err
	}

	// Tenant isolation: a tenant-bound principal may only mutate
	// records that belong to its own tenant.
	if err := ensureSyncTenant(person, tenantID); err != nil {
		return nil, err
	}

	if person.IsDeleted {
		return nil, errors.New("person record was deleted on the server")
	}

	// Optimistic locking:
	// the client must update the version it originally read.
	if person.Version != operation.ClientVersion {
		return &models.SyncResult{
			OperationID:   operation.ID,
			EntityType:    "person",
			RecordID:      operation.RecordID,
			Success:       false,
			Code:          models.SyncErrorConflict,
			Message:       "Person record was modified on the server",
			ServerVersion: person.Version,
			ClientVersion: operation.ClientVersion,
		}, ErrPersonSyncConflict
	}

	payload, err := json.Marshal(operation.Payload)
	if err != nil {
		return nil, err
	}

	var incoming models.Person

	if err := json.Unmarshal(payload, &incoming); err != nil {
		return nil, err
	}

	// Server-controlled fields must never come from the
	// offline client payload.
	incoming.ID = person.ID
	incoming.CreatedByID = person.CreatedByID
	incoming.UpdatedBy = &operation.UserID
	incoming.Version = person.Version + 1
	incoming.IsDeleted = false
	// The tenant of an existing record is immutable for clients.
	incoming.TenantID = person.TenantID

	if err := s.personRepo.Update(&incoming); err != nil {
		return nil, err
	}

	return &models.SyncResult{
		OperationID:   operation.ID,
		EntityType:    "person",
		RecordID:      person.ID,
		Success:       true,
		ServerVersion: incoming.Version,
		ClientVersion: operation.ClientVersion,
	}, nil
}

func (s *PersonSyncService) delete(
	operation models.SyncOperation,
	tenantID *uuid.UUID,
) (*models.SyncResult, error) {
	if operation.ID == uuid.Nil {
		return nil, errors.New("invalid sync operation id")
	}

	if operation.RecordID == uuid.Nil {
		return nil, ErrPersonSyncRecordID
	}

	if operation.UserID == uuid.Nil {
		return nil, ErrPersonSyncUser
	}

	person, err := s.personRepo.FindByID(
		operation.RecordID.String(),
	)
	if err != nil {
		return nil, err
	}

	// Tenant isolation: a tenant-bound principal may only delete
	// records that belong to its own tenant.
	if err := ensureSyncTenant(person, tenantID); err != nil {
		return nil, err
	}

	// Optimistic locking before soft delete.
	if person.Version != operation.ClientVersion {
		return &models.SyncResult{
			OperationID:   operation.ID,
			EntityType:    "person",
			RecordID:      operation.RecordID,
			Success:       false,
			Code:          models.SyncErrorConflict,
			Message:       "Person record was modified on the server",
			ServerVersion: person.Version,
			ClientVersion: operation.ClientVersion,
		}, ErrPersonSyncConflict
	}

	person.IsDeleted = true
	person.UpdatedBy = &operation.UserID
	person.Version++

	if err := s.personRepo.Update(person); err != nil {
		return nil, err
	}

	return &models.SyncResult{
		OperationID:   operation.ID,
		EntityType:    "person",
		RecordID:      person.ID,
		Success:       true,
		ServerVersion: person.Version,
		ClientVersion: operation.ClientVersion,
	}, nil
}
