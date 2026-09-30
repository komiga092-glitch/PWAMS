package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrInvalidAuditLog = errors.New(
		"invalid audit log information",
	)

	ErrInvalidAuditLogID = errors.New(
		"invalid audit log ID",
	)

	ErrInvalidAuditLogUserID = errors.New(
		"invalid audit log user ID",
	)

	ErrInvalidAuditLogEntityID = errors.New(
		"invalid audit log entity ID",
	)

	// ErrAuditLogUnavailable is returned when audit logging is not wired up
	// (nil service or nil repository). It is a hard failure: mandatory audit
	// events must never be silently dropped, so callers abort the surrounding
	// operation instead of ignoring it.
	ErrAuditLogUnavailable = errors.New(
		"audit logging is unavailable",
	)

	// ErrAuditLogTransactionUnavailable is returned when the audit service has
	// no database handle and therefore cannot open the transaction that makes
	// the business write and the audit write atomic.
	ErrAuditLogTransactionUnavailable = errors.New(
		"audit log transaction is unavailable",
	)

	// ErrAuditLogWriteFailed wraps every failure of a mandatory audit write so
	// handlers can tell an audit failure apart from a business failure and
	// return an infrastructure error instead of a misleading validation error.
	ErrAuditLogWriteFailed = errors.New(
		"audit log write failed",
	)
)

type AuditLogService struct {
	auditLogRepo *repository.AuditLogRepository
	// db is the handle shared with the business repositories. It is used to
	// commit a business mutation and its audit record atomically.
	db *gorm.DB
}

func NewAuditLogService(
	auditLogRepo *repository.AuditLogRepository,
) *AuditLogService {
	return &AuditLogService{
		auditLogRepo: auditLogRepo,
		db:           auditLogRepo.DB(),
	}
}

// WithTx returns a copy of the service bound to tx so the audit row is written
// inside the caller's transaction (both the business write and the audit write
// commit or roll back together).
func (s *AuditLogService) WithTx(tx *gorm.DB) *AuditLogService {
	if s == nil {
		return &AuditLogService{auditLogRepo: nil, db: tx}
	}
	return &AuditLogService{
		auditLogRepo: s.auditLogRepo.WithTx(tx),
		db:           tx,
	}
}

// Transaction runs fn inside a database transaction.
//
// A nil (unconfigured) audit service fails closed: the callback is never
// executed, ErrAuditLogUnavailable is returned and the caller therefore never
// performs the business write. This guarantees that a missing nil check cannot
// cause a panic, a 500 caused by a nil pointer dereference, or a committed
// business record without its audit entry.
func (s *AuditLogService) Transaction(
	fn func(tx *gorm.DB) error,
) error {
	if s == nil || s.db == nil {
		return ErrAuditLogUnavailable
	}
	if fn == nil {
		return ErrAuditLogTransactionUnavailable
	}
	return s.db.Transaction(fn)
}

// Create writes an audit log entry.
//
// Calling Create on a nil service returns ErrAuditLogUnavailable instead of
// panicking, so legacy call sites that do not guard against a missing
// dependency degrade into a reported failure rather than a crash.
func (s *AuditLogService) Create(
	userID string,
	action string,
	entity string,
	entityID string,
	details string,
) error {
	if s == nil || s.auditLogRepo == nil {
		return ErrAuditLogUnavailable
	}

	action = strings.TrimSpace(action)
	entity = strings.TrimSpace(entity)
	details = strings.TrimSpace(details)

	if action == "" || entity == "" {
		return ErrInvalidAuditLog
	}

	var parsedUserID *uuid.UUID

	if strings.TrimSpace(userID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(userID))
		if err != nil {
			return ErrInvalidAuditLogUserID
		}

		parsedUserID = &id
	}

	var parsedEntityID *uuid.UUID

	if strings.TrimSpace(entityID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(entityID))
		if err != nil {
			return ErrInvalidAuditLogEntityID
		}

		parsedEntityID = &id
	}

	auditLog := &models.AuditLog{
		ID:       uuid.New(),
		UserID:   parsedUserID,
		Action:   action,
		Entity:   entity,
		EntityID: parsedEntityID,
		Details:  details,
	}

	return s.auditLogRepo.Create(auditLog)
}

// Audit writes a mandatory audit row inside tx.
//
// It is the entry point used by handlers that commit a business write and its
// audit row in a single transaction. Any failure is wrapped with
// ErrAuditLogWriteFailed, which aborts (and therefore rolls back) that
// transaction: the business record is never committed without its audit event.
func (s *AuditLogService) Audit(
	tx *gorm.DB,
	userID string,
	action string,
	entity string,
	entityID string,
	details string,
) error {
	if err := s.WithTx(tx).Create(
		userID,
		action,
		entity,
		entityID,
		details,
	); err != nil {
		return fmt.Errorf("%w: %w", ErrAuditLogWriteFailed, err)
	}

	return nil
}

func (s *AuditLogService) GetByID(
	id string,
) (*models.AuditLog, error) {
	if s == nil || s.auditLogRepo == nil {
		return nil, ErrAuditLogUnavailable
	}

	parsedID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, ErrInvalidAuditLogID
	}

	return s.auditLogRepo.FindByID(parsedID)
}

func (s *AuditLogService) List(
	query models.AuditLogListQuery,
) ([]models.AuditLog, int64, int, int, error) {
	if s == nil || s.auditLogRepo == nil {
		return nil, 0, 0, 0, ErrAuditLogUnavailable
	}

	return s.auditLogRepo.List(query)
}

func (s *AuditLogService) Delete(
	id string,
) error {
	if s == nil || s.auditLogRepo == nil {
		return ErrAuditLogUnavailable
	}

	parsedID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return ErrInvalidAuditLogID
	}

	return s.auditLogRepo.Delete(parsedID)
}
