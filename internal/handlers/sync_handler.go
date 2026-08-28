package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// normalizedEntity applies the same singularisation the sync service
// uses so failure results report consistent entity names.
func normalizedEntity(entityType string) string {
	name := strings.ToLower(strings.TrimSpace(entityType))
	return strings.TrimSuffix(name, "s")
}

// truncatedReason keeps failure messages short and free of internals
// that would otherwise leak database or stack detail to clients.
func truncatedReason(err error) string {
	const maxLen = 160
	message := err.Error()
	if len(message) > maxLen {
		message = message[:maxLen]
	}
	return message
}

type SyncHandler struct {
	service *services.SyncService
	db      *gorm.DB
}

func NewSyncHandler(service *services.SyncService, db *gorm.DB) *SyncHandler {
	return &SyncHandler{
		service: service,
		db:      db,
	}
}

func (h *SyncHandler) Push(c *gin.Context) {
	var request models.SyncPushRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    models.SyncErrorValidation,
			"message": "Invalid sync request",
		})
		return
	}

	if err := h.service.ValidatePushRequest(request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    models.SyncErrorValidation,
			"message": err.Error(),
		})
		return
	}

	idempotencyKey := strings.TrimSpace(
		c.GetHeader("Idempotency-Key"),
	)

	if idempotencyKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "IDEMPOTENCY_KEY_REQUIRED",
			"message": "Idempotency-Key header is required",
		})
		return
	}
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	for i := range request.Operations {
		request.Operations[i].UserID = currentUser.ID
	}

	existing, err := h.service.GetIdempotencyRecord(
		idempotencyKey,
		request,
	)
	if err != nil {
		if errors.Is(err, services.ErrIdempotencyMismatch) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"code":    models.SyncErrorIdempotencyMismatch,
				"message": "Idempotency key was already used with a different payload",
			})
			return
		}

		if errors.Is(err, services.ErrMissingIdempotencyKey) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"code":    "IDEMPOTENCY_KEY_REQUIRED",
				"message": "Idempotency-Key header is required",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to process synchronization request",
		})
		return
	}

	if existing != nil {
		c.Data(
			existing.StatusCode,
			"application/json",
			[]byte(existing.ResponseBody),
		)
		return
	}

	entityOrder := map[string]int{
		"person": 0, "persons": 0,
		"student": 1, "students": 1,
		"aid_request": 2, "aid_requests": 2,
		"care_provided": 3,
		"loan":          4, "loans": 4,
		"loan_repayment": 5, "loan_repayments": 5,
		"donor": 6, "donors": 6,
	}
	sort.SliceStable(request.Operations, func(left, right int) bool {
		return entityOrder[strings.ToLower(strings.TrimSpace(request.Operations[left].EntityType))] < entityOrder[strings.ToLower(strings.TrimSpace(request.Operations[right].EntityType))]
	})

	results := make([]models.SyncResult, 0, len(request.Operations))
	hasConflict := false
	hasFailure := false

	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		for idx, operation := range request.Operations {
			// Per-operation savepoint (spec §5): a structural/validation
			// failure rolls back only that entity's work so independent
			// operations in the batch still commit.
			savepoint := fmt.Sprintf("pwams_sync_op_%d", idx)

			if err := tx.SavePoint(savepoint).Error; err != nil {
				return err
			}

			result, err := h.service.ApplyOperation(operation)

			if result != nil {
				results = append(results, *result)
			}

			if err != nil {
				switch {
				case errors.Is(err, services.ErrPersonSyncConflict),
					errors.Is(err, services.ErrSyncConflict):
					// Server record untouched for conflicts; other
					// operations proceed and overall status becomes 409.
					hasConflict = true

				default:
					if rollbackErr := tx.RollbackTo(savepoint).Error; rollbackErr != nil {
						return rollbackErr
					}

					failed := models.SyncResult{
						OperationID:   operation.ID,
						EntityType:    normalizedEntity(operation.EntityType),
						RecordID:      operation.RecordID,
						Success:       false,
						Code:          models.SyncErrorValidation,
						ClientVersion: operation.ClientVersion,
						Message:       truncatedReason(err),
					}
					results = append(results, failed)
					hasFailure = true
				}
			}
		}
		return nil
	})

	if txErr != nil && !hasConflict {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Unable to process synchronization operation",
			"results": results,
		})
		return
	}

	statusCode := http.StatusOK

	if hasConflict {
		statusCode = http.StatusConflict
	}

	response := models.SyncPushResponse{
		Success: !hasConflict && !hasFailure,
		Results: results,
	}

	record, err := h.service.SaveIdempotencyResponse(
		idempotencyKey,
		request,
		response,
		statusCode,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to store synchronization result",
		})
		return
	}

	c.Header("X-Sync-Idempotency-Record", record.ID.String())

	c.JSON(statusCode, response)
}

func (h *SyncHandler) Pull(c *gin.Context) {
	cursorValue := strings.TrimSpace(c.Query("cursor"))
	limitValue := strings.TrimSpace(c.Query("limit"))

	var cursor time.Time

	if cursorValue != "" {
		parsed, err := time.Parse(time.RFC3339, cursorValue)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid cursor",
			})
			return
		}

		cursor = parsed
	}

	limit := 500

	if limitValue != "" {
		parsed, err := strconv.Atoi(limitValue)
		if err != nil || parsed < 1 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid limit",
			})
			return
		}

		limit = parsed
	}

	records, nextCursor, hasMore, err := h.service.Pull(
		cursor,
		limit,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve synchronization records",
		})
		return
	}

	response := models.SyncPullResponse{
		Success: true,
		Cursor:  nextCursor.UTC().Format(time.RFC3339),
		HasMore: hasMore,
		Records: records,
	}

	c.JSON(http.StatusOK, response)
}
