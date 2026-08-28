package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

func TestRequestHash_Stability(t *testing.T) {
	syncService := services.NewSyncService(nil, nil)

	userID := uuid.New()
	recordID := uuid.New()
	opID := uuid.New()

	req1 := models.SyncPushRequest{
		Operations: []models.SyncOperation{
			{
				ID:            opID,
				UserID:        userID,
				EntityType:    "person",
				Operation:     "CREATE",
				RecordID:      recordID,
				ClientVersion: 1,
				Payload:       map[string]interface{}{"full_name": "Test User"},
			},
		},
	}

	hash1, err := syncService.RequestHash(req1)
	if err != nil {
		t.Fatalf("RequestHash failed: %v", err)
	}

	req2 := models.SyncPushRequest{
		Operations: []models.SyncOperation{
			{
				ID:            opID,
				UserID:        userID,
				EntityType:    "person",
				Operation:     "CREATE",
				RecordID:      recordID,
				ClientVersion: 1,
				Payload:       map[string]interface{}{"full_name": "Test User"},
			},
		},
	}

	hash2, err := syncService.RequestHash(req2)
	if err != nil {
		t.Fatalf("RequestHash failed: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("RequestHash should be stable, got %q and %q", hash1, hash2)
	}
}

func TestRequestHash_DifferentPayloadProducesDifferentHash(t *testing.T) {
	syncService := services.NewSyncService(nil, nil)

	userID := uuid.New()
	recordID := uuid.New()
	opID := uuid.New()

	req1 := models.SyncPushRequest{
		Operations: []models.SyncOperation{
			{
				ID:            opID,
				UserID:        userID,
				EntityType:    "person",
				Operation:     "CREATE",
				RecordID:      recordID,
				ClientVersion: 1,
				Payload:       map[string]interface{}{"full_name": "Alice"},
			},
		},
	}

	req2 := models.SyncPushRequest{
		Operations: []models.SyncOperation{
			{
				ID:            opID,
				UserID:        userID,
				EntityType:    "person",
				Operation:     "CREATE",
				RecordID:      recordID,
				ClientVersion: 1,
				Payload:       map[string]interface{}{"full_name": "Bob"},
			},
		},
	}

	hash1, _ := syncService.RequestHash(req1)
	hash2, _ := syncService.RequestHash(req2)

	if hash1 == hash2 {
		t.Error("Different payloads should produce different hashes")
	}
}
