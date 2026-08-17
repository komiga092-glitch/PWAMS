package services

import (
	"errors"
	"strings"

	"github.com/google/uuid"

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
)

type AuditLogService struct {
	auditLogRepo *repository.AuditLogRepository
}

func NewAuditLogService(
	auditLogRepo *repository.AuditLogRepository,
) *AuditLogService {
	return &AuditLogService{
		auditLogRepo: auditLogRepo,
	}
}

func (s *AuditLogService) Create(
	userID string,
	action string,
	entity string,
	entityID string,
	details string,
	ipAddress string,
) error {
	action = strings.TrimSpace(action)
	entity = strings.TrimSpace(entity)
	details = strings.TrimSpace(details)
	ipAddress = strings.TrimSpace(ipAddress)

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
		ID:        uuid.New(),
		UserID:    parsedUserID,
		Action:    action,
		Entity:    entity,
		EntityID:  parsedEntityID,
		Details:   details,
		IPAddress: ipAddress,
	}

	return s.auditLogRepo.Create(auditLog)
}

func (s *AuditLogService) GetByID(
	id string,
) (*models.AuditLog, error) {
	parsedID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, ErrInvalidAuditLogID
	}

	return s.auditLogRepo.FindByID(parsedID)
}

func (s *AuditLogService) List(
	query models.AuditLogListQuery,
) ([]models.AuditLog, int64, int, int, error) {
	return s.auditLogRepo.List(query)
}

func (s *AuditLogService) Delete(
	id string,
) error {
	parsedID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return ErrInvalidAuditLogID
	}

	return s.auditLogRepo.Delete(parsedID)
}
