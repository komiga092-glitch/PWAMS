package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
)

var (
	ErrSyncOperationID = errors.New("invalid sync operation id")
	ErrSyncRecordID    = errors.New("invalid sync record id")
	ErrSyncUserID      = errors.New("invalid sync user id")
	ErrSyncOperation   = errors.New("unsupported sync operation")
	ErrSyncConflict    = errors.New("sync conflict")
	// ErrSyncTenantMismatch is returned when a tenant-bound principal
	// attempts to mutate a record belonging to a different tenant.
	ErrSyncTenantMismatch = errors.New("sync record belongs to another tenant")
)

type SyncEntityAdapter struct {
	Entity   string
	New      func() any
	Find     func(uuid.UUID) (any, error)
	Create   func(any) error
	Update   func(any) error
	Validate func(any) error
}

type EntitySyncService struct{ adapter SyncEntityAdapter }

func NewEntitySyncService(adapter SyncEntityAdapter) *EntitySyncService {
	return &EntitySyncService{adapter: adapter}
}

func (s *EntitySyncService) Apply(operation models.SyncOperation, tenantID *uuid.UUID) (*models.SyncResult, error) {
	if operation.ID == uuid.Nil {
		return nil, ErrSyncOperationID
	}
	if operation.RecordID == uuid.Nil {
		return nil, ErrSyncRecordID
	}
	if operation.UserID == uuid.Nil {
		return nil, ErrSyncUserID
	}

	result := func(success bool) *models.SyncResult {
		return &models.SyncResult{OperationID: operation.ID, EntityType: s.adapter.Entity, RecordID: operation.RecordID, Success: success, ClientVersion: operation.ClientVersion}
	}

	switch strings.ToUpper(strings.TrimSpace(operation.Operation)) {
	case models.SyncOperationCreate:
		record := s.adapter.New()
		if err := decodePayload(operation.Payload, record); err != nil {
			return nil, err
		}
		setSyncField(record, "ID", operation.RecordID)
		setSyncField(record, "CreatedByID", operation.UserID)
		setSyncField(record, "UpdatedBy", &operation.UserID)
		setSyncField(record, "Version", 1)
		setSyncField(record, "IsDeleted", false)
		// The tenant comes from the authenticated principal, never
		// from the client payload.
		if tenantID != nil {
			setSyncField(record, "TenantID", tenantID)
		} else {
			clearSyncField(record, "TenantID")
		}
		if err := s.adapter.Validate(record); err != nil {
			return nil, err
		}
		if err := s.adapter.Create(record); err != nil {
			return nil, err
		}
		created := result(true)
		created.ServerVersion = 1
		return created, nil

	case models.SyncOperationUpdate:
		current, err := s.adapter.Find(operation.RecordID)
		if err != nil {
			return nil, err
		}

		// Tenant isolation: a tenant-bound principal may only mutate
		// records that belong to its own tenant.
		if err := ensureSyncTenant(current, tenantID); err != nil {
			return nil, err
		}

		if syncBool(syncField(current, "IsDeleted")) {
			return nil, fmt.Errorf("%s record was deleted on the server", s.adapter.Entity)
		}

		serverVersion := syncVersion(current)
		if serverVersion != operation.ClientVersion {
			conflict := result(false)
			conflict.Code = models.SyncErrorConflict
			conflict.Message = s.adapter.Entity + " record was modified on the server"
			conflict.ServerVersion = serverVersion
			return conflict, ErrSyncConflict
		}
		incoming := s.adapter.New()
		if err := decodePayload(operation.Payload, incoming); err != nil {
			return nil, err
		}
		setSyncField(incoming, "ID", operation.RecordID)
		setSyncField(incoming, "CreatedByID", syncField(current, "CreatedByID"))
		setSyncField(incoming, "UpdatedBy", &operation.UserID)
		setSyncField(incoming, "Version", serverVersion+1)
		setSyncField(incoming, "IsDeleted", false)
		setSyncField(incoming, "TenantID", syncField(current, "TenantID"))
		if err := s.adapter.Validate(incoming); err != nil {
			return nil, err
		}
		if err := s.adapter.Update(incoming); err != nil {
			return nil, err
		}
		updated := result(true)
		updated.ServerVersion = serverVersion + 1
		return updated, nil

	case models.SyncOperationDelete:
		current, err := s.adapter.Find(operation.RecordID)
		if err != nil {
			return nil, err
		}

		// Tenant isolation: a tenant-bound principal may only delete
		// records that belong to its own tenant.
		if err := ensureSyncTenant(current, tenantID); err != nil {
			return nil, err
		}

		serverVersion := syncVersion(current)
		if serverVersion != operation.ClientVersion {
			conflict := result(false)
			conflict.Code = models.SyncErrorConflict
			conflict.Message = s.adapter.Entity + " record was modified on the server"
			conflict.ServerVersion = serverVersion
			return conflict, ErrSyncConflict
		}
		setSyncField(current, "IsDeleted", true)
		setSyncField(current, "UpdatedBy", &operation.UserID)
		setSyncField(current, "Version", serverVersion+1)
		if err := s.adapter.Update(current); err != nil {
			return nil, err
		}
		deleted := result(true)
		deleted.ServerVersion = serverVersion + 1
		return deleted, nil

	default:
		return nil, ErrSyncOperation
	}
}

// ensureSyncTenant enforces tenant isolation for server-authoritative
// sync mutations: a tenant-bound principal may only mutate records that
// belong to its own tenant. Principals without a tenant (single-tenant
// deployment) are unrestricted — the same contract as
// middleware.WithTenantScope. A record without a tenant is invisible to
// tenant-bound principals, matching the `tenant_id = ?` predicate.
func ensureSyncTenant(record any, tenantID *uuid.UUID) error {
	if tenantID == nil {
		return nil
	}

	recordTenant, _ := syncField(record, "TenantID").(*uuid.UUID)
	if recordTenant == nil || *recordTenant != *tenantID {
		return ErrSyncTenantMismatch
	}
	return nil
}

func decodePayload(payload map[string]interface{}, target any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func syncVersion(record any) int {
	value := syncField(record, "Version")
	if version, ok := value.(int); ok && version > 0 {
		return version
	}
	return 1
}

func syncBool(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func syncField(record any, name string) any {
	value := reflect.ValueOf(record)
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	field := value.FieldByName(name)
	if !field.IsValid() {
		return nil
	}
	return field.Interface()
}

func setSyncField(record any, name string, value any) {
	if value == nil {
		return
	}
	field := reflect.ValueOf(record)
	if field.Kind() == reflect.Pointer {
		field = field.Elem()
	}
	field = field.FieldByName(name)
	if !field.IsValid() || !field.CanSet() {
		return
	}
	incoming := reflect.ValueOf(value)
	if incoming.Type().AssignableTo(field.Type()) {
		field.Set(incoming)
	}
}

func clearSyncField(record any, name string) {
	field := reflect.ValueOf(record)
	if field.Kind() == reflect.Pointer {
		field = field.Elem()
	}
	field = field.FieldByName(name)
	if field.IsValid() && field.CanSet() {
		field.Set(reflect.Zero(field.Type()))
	}
}
