package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("database instance error: %v", err)
	}

	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("database close error: %v", err)
		}
	}()

	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"application": "PWAMS",
			"message":     "PWAMS web server is running successfully",
		})
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":   "healthy",
			"database": "connected",
		})
	})

	address := ":" + cfg.AppPort

	log.Printf("PWAMS server running at http://localhost%s", address)

	if err := router.Run(address); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
