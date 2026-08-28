package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type SyncManager struct {
	service *SyncHandler
}

func NewSyncManager(service *SyncHandler) *SyncManager {
	return &SyncManager{service: service}
}

func (m *SyncManager) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"sync": gin.H{
			"push_endpoint": "/api/v1/sync/push",
			"pull_endpoint": "/api/v1/sync/pull",
			"max_batch":     500,
			"timestamp":     time.Now().UTC().Format(time.RFC3339),
		},
	})
}
