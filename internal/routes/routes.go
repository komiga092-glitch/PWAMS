package routes

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/handlers"
)

func Setup(router *gin.Engine, db *gorm.DB) {
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base", handlers.PageData(c, gin.H{
			"page_template": "home_content",
			"title":         "PWAMS",
		}))
	})

	router.GET("/health", func(c *gin.Context) {
		storageDir := "storage/uploads"
		storageOK := true
		if _, err := os.Stat(storageDir); os.IsNotExist(err) {
			storageOK = false
		}

		sqlDB, err := db.DB()
		dbOK := err == nil && sqlDB.Ping() == nil

		status := http.StatusOK
		if !dbOK || !storageOK {
			status = http.StatusServiceUnavailable
		}

		c.JSON(status, gin.H{
			"application": "PWAMS",
			"status":      healthStatus(dbOK && storageOK),
			"db":          healthStatus(dbOK),
			"storage":     healthStatus(storageOK),
		})
	})
}

func healthStatus(ok bool) string {
	if ok {
		return "healthy"
	}
	return "unhealthy"
}
