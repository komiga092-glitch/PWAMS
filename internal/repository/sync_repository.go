package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
)

var (
	ErrIdempotencyRecordNotFound = errors.New("idempotency record not found")
)

type SyncRepository struct {
	db *gorm.DB
}

func NewSyncRepository(db *gorm.DB) *SyncRepository {
	return &SyncRepository{db: db}
}

func (r *SyncRepository) SaveIdempotencyRecord(
	record *models.SyncIdempotencyRecord,
) error {
	return r.db.Create(record).Error
}

func (r *SyncRepository) FindIdempotencyRecord(
	key string,
) (*models.SyncIdempotencyRecord, error) {
	var record models.SyncIdempotencyRecord

	err := r.db.
		Where("idempotency_key = ?", key).
		First(&record).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrIdempotencyRecordNotFound
	}

	if err != nil {
		return nil, err
	}

	return &record, nil
}

func (r *SyncRepository) CreateIdempotencyRecord(
	key string,
	requestHash string,
	response interface{},
	statusCode int,
) (*models.SyncIdempotencyRecord, error) {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	record := &models.SyncIdempotencyRecord{
		ID:             uuid.New(),
		IdempotencyKey: key,
		RequestHash:    requestHash,
		ResponseBody:   string(responseJSON),
		StatusCode:     statusCode,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := r.SaveIdempotencyRecord(record); err != nil {
		return nil, err
	}

	return record, nil
}

func (r *SyncRepository) PullPersons(
	cursor time.Time,
	limit int,
) ([]models.SyncPullRecord, time.Time, bool, error) {
	if limit <= 0 {
		limit = 500
	}

	if limit > 500 {
		limit = 500
	}

	var records []models.Person

	query := r.db.
		Where("updated_at > ?", cursor).
		Order("updated_at ASC").
		Limit(limit + 1)

	if err := query.Find(&records).Error; err != nil {
		return nil, cursor, false, err
	}

	hasMore := len(records) > limit

	if hasMore {
		records = records[:limit]
	}

	result := make([]models.SyncPullRecord, 0, len(records))

	nextCursor := cursor

	for _, person := range records {
		payload := map[string]interface{}{
			"id":         person.ID,
			"version":    person.Version,
			"is_deleted": person.IsDeleted,
		}

		result = append(result, models.SyncPullRecord{
			EntityType: "person",
			RecordID:   person.ID,
			Version:    person.Version,
			UpdatedAt:  person.UpdatedAt,
			IsDeleted:  person.IsDeleted,
			Payload:    payload,
		})

		if person.UpdatedAt.After(nextCursor) {
			nextCursor = person.UpdatedAt
		}
	}

	return result, nextCursor, hasMore, nil
}

func (r *SyncRepository) PullEntities(
	cursor time.Time,
	limit int,
) ([]models.SyncPullRecord, time.Time, bool, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}

	type entityTable struct {
		name  string
		label string
	}
	tables := []entityTable{
		{"persons", "person"},
		{"students", "student"},
		{"donors", "donor"},
		{"aid_requests", "aid_request"},
		{"care_provided", "care_provided"},
		{"loans", "loan"},
		{"loan_repayments", "loan_repayment"},
	}

	type syncRow struct {
		ID        uuid.UUID      `gorm:"column:id"`
		Version   int            `gorm:"column:version"`
		UpdatedAt time.Time      `gorm:"column:updated_at"`
		IsDeleted bool           `gorm:"column:is_deleted"`
		Payload   map[string]any `gorm:"-"`
	}

	result := make([]models.SyncPullRecord, 0)
	for _, table := range tables {
		var rows []map[string]any
		if err := r.db.Table(table.name).Unscoped().Where("updated_at > ?", cursor).Order("updated_at ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
			return nil, cursor, false, fmt.Errorf("failed to pull %s: %w", table.label, err)
		}
		for _, row := range rows {
			id, err := syncUUID(row["id"])
			if err != nil {
				return nil, cursor, false, err
			}
			version := syncInt(row["version"])
			updatedAt, err := syncTime(row["updated_at"])
			if err != nil {
				return nil, cursor, false, err
			}
			result = append(result, models.SyncPullRecord{EntityType: table.label, RecordID: id, Version: version, UpdatedAt: updatedAt, IsDeleted: syncBool(row["is_deleted"]), Payload: row})
		}
	}

	sort.SliceStable(result, func(left, right int) bool {
		if result[left].UpdatedAt.Equal(result[right].UpdatedAt) {
			if result[left].EntityType == result[right].EntityType {
				return result[left].RecordID.String() < result[right].RecordID.String()
			}
			return result[left].EntityType < result[right].EntityType
		}
		return result[left].UpdatedAt.Before(result[right].UpdatedAt)
	})
	hasMore := len(result) > limit
	if hasMore {
		result = result[:limit]
	}
	nextCursor := cursor
	for _, record := range result {
		if record.UpdatedAt.After(nextCursor) {
			nextCursor = record.UpdatedAt
		}
	}
	return result, nextCursor, hasMore, nil
}

func syncUUID(value any) (uuid.UUID, error) {
	switch typed := value.(type) {
	case uuid.UUID:
		return typed, nil
	case string:
		return uuid.Parse(typed)
	case []byte:
		if len(typed) == 16 {
			return uuid.FromBytes(typed)
		}
		return uuid.Parse(string(typed))
	default:
		return uuid.Nil, fmt.Errorf("invalid synchronized record id")
	}
}

func syncInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	default:
		return 1
	}
}
func syncBool(value any) bool { typed, ok := value.(bool); return ok && typed }
func syncTime(value any) (time.Time, error) {
	if typed, ok := value.(time.Time); ok {
		return typed, nil
	}
	if typed, ok := value.(string); ok {
		return time.Parse(time.RFC3339, typed)
	}
	return time.Time{}, fmt.Errorf("invalid synchronized updated_at")
}
