package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/routes"
	"github.com/komiga092-glitch/pwams/internal/services"
)

func main() {
	// 1. Load application configuration.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	// 2. Connect to PostgreSQL.
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

	// 3. Create/update database tables.
	if err := database.Migrate(db); err != nil {
		log.Fatalf("database migration error: %v", err)
	}

	log.Println("Database migration completed successfully")

	// 4. Insert default roles.
	if err := database.SeedDefaultRoles(db); err != nil {
		log.Fatalf("default role seeding error: %v", err)
	}

	log.Println("Default roles seeded successfully")

	// 5. Insert initial Super Admin account.
	if err := database.SeedSuperAdmin(db, cfg); err != nil {
		log.Fatalf("super admin seeding error: %v", err)
	}

	log.Println("Super Admin account seeded successfully")

	// 6. Create repository, service and handler.
	userRepository := repository.NewUserRepository(db)
	authService := services.NewAuthService(userRepository)
	authHandler := handlers.NewAuthHandler(authService)

	// 7. Create Gin router.
	router := gin.Default()

	// Basic routes: / and /health
	routes.Setup(router)

	// Authentication route: POST /login
	routes.RegisterAuthRoutes(router, authHandler)

	// 8. Start server.
	address := ":" + cfg.AppPort

	log.Printf("PWAMS server running at http://localhost%s", address)

	if err := router.Run(address); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
