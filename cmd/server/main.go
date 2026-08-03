package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/routes"
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

	routes.Setup(router)

	address := ":" + cfg.AppPort

	log.Printf("PWAMS server running at http://localhost%s", address)

	if err := database.Migrate(db); err != nil {
		log.Fatalf("database migration error: %v", err)
	}

	log.Println("Database migration completed successfully")

	if err := router.Run(address); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
