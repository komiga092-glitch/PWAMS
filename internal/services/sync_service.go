package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrMissingIdempotencyKey = errors.New("missing idempotency key")
	ErrInvalidSyncRequest    = errors.New("invalid sync request")
	ErrIdempotencyMismatch   = errors.New("idempotency key payload mismatch")
)

type SyncService struct {
	repo       *repository.SyncRepository
	personSync *PersonSyncService
	entities   map[string]*EntitySyncService
}

func NewSyncService(repo *repository.SyncRepository, personSync *PersonSyncService, entities ...map[string]*EntitySyncService) *SyncService {
	entityServices := make(map[string]*EntitySyncService)
	if len(entities) > 0 && entities[0] != nil {
		entityServices = entities[0]
	}
	return &SyncService{
		repo:       repo,
		personSync: personSync,
		entities:   entityServices,
	}
}

func (s *SyncService) ApplyOperation(
	operation models.SyncOperation,
) (*models.SyncResult, error) {
	switch strings.ToLower(strings.TrimSpace(operation.EntityType)) {
	case "person", "persons":
		return s.personSync.Apply(operation)

	default:
		entityName := strings.ToLower(strings.TrimSpace(operation.EntityType))
		if strings.HasSuffix(entityName, "s") {
			entityName = strings.TrimSuffix(entityName, "s")
		}
		entity, ok := s.entities[entityName]
		if !ok {
			return nil, errors.New("unsupported sync entity type")
		}
		return entity.Apply(operation)
	}
}

func (s *SyncService) ValidatePushRequest(
	request models.SyncPushRequest,
) error {
	if len(request.Operations) == 0 {
		return ErrInvalidSyncRequest
	}

	for _, operation := range request.Operations {
		if operation.ID == uuid.Nil {
			return ErrInvalidSyncRequest
		}

		if strings.TrimSpace(operation.EntityType) == "" {
			return ErrInvalidSyncRequest
		}

		if strings.TrimSpace(operation.Operation) == "" {
			return ErrInvalidSyncRequest
		}

		if operation.RecordID == uuid.Nil {
			return ErrInvalidSyncRequest
		}
		if operation.ClientVersion < 1 {
			return ErrInvalidSyncRequest
		}
		switch strings.ToUpper(strings.TrimSpace(operation.Operation)) {
		case models.SyncOperationCreate, models.SyncOperationUpdate, models.SyncOperationDelete:
		default:
			return ErrInvalidSyncRequest
		}
	}

	return nil
}

func (s *SyncService) RequestHash(
	request models.SyncPushRequest,
) (string, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)

	return hex.EncodeToString(hash[:]), nil
}

func (s *SyncService) GetIdempotencyRecord(
	key string,
	request models.SyncPushRequest,
) (*models.SyncIdempotencyRecord, error) {
	key = strings.TrimSpace(key)

	if key == "" {
		return nil, ErrMissingIdempotencyKey
	}

	hash, err := s.RequestHash(request)
	if err != nil {
		return nil, err
	}

	record, err := s.repo.FindIdempotencyRecord(key)
	if err != nil {
		if errors.Is(err, repository.ErrIdempotencyRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	if record.RequestHash != hash {
		return nil, ErrIdempotencyMismatch
	}

	return record, nil
}

func (s *SyncService) SaveIdempotencyResponse(
	key string,
	request models.SyncPushRequest,
	response models.SyncPushResponse,
	statusCode int,
) (*models.SyncIdempotencyRecord, error) {
	key = strings.TrimSpace(key)

	if key == "" {
		return nil, ErrMissingIdempotencyKey
	}

	hash, err := s.RequestHash(request)
	if err != nil {
		return nil, err
	}

	return s.repo.CreateIdempotencyRecord(
		key,
		hash,
		response,
		statusCode,
	)
}
func (s *SyncService) PullPersons(
	cursor time.Time,
	limit int,
) ([]models.SyncPullRecord, time.Time, bool, error) {
	return s.repo.PullPersons(cursor, limit)
}

func (s *SyncService) Pull(
	cursor time.Time,
	limit int,
) ([]models.SyncPullRecord, time.Time, bool, error) {
	return s.repo.PullEntities(cursor, limit)
}
