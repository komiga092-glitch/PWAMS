package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/routes"
	"github.com/komiga092-glitch/pwams/internal/services"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	// Connect database
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("database instance error: %v", err)
	}
	defer sqlDB.Close()

	// Database migration
	if err := database.Migrate(db); err != nil {
		log.Fatalf("migration error: %v", err)
	}

	// Seed default data
	if err := database.SeedDefaultRoles(db); err != nil {
		log.Fatalf("role seed error: %v", err)
	}

	if err := database.SeedSuperAdmin(db, cfg); err != nil {
		log.Fatalf("super admin seed error: %v", err)
	}

	// Repositories
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	roleRepository := repository.NewRoleRepository(db)

	// Services
	authService := services.NewAuthService(userRepo)
	sessionService := services.NewSessionService(sessionRepo)
	userService := services.NewUserService(userRepo, roleRepository)

	// Cookie configuration
	secureCookie := cfg.AppEnv == "production"

	// Handlers
	authHandler := handlers.NewAuthHandler(
		authService,
		sessionService,
		secureCookie,
	)
	userHandler := handlers.NewUserHandler(userService)

	// Middleware
	authMiddleware := middleware.NewAuthMiddleware(sessionService)

	// Router
	router := gin.Default()

	routes.Setup(router)

	routes.RegisterAuthRoutes(
		router,
		authHandler,
		authMiddleware,
	)

	routes.RegisterUserRoutes(
		router,
		userHandler,
		authMiddleware,
	)

	address := ":" + cfg.AppPort

	log.Printf("PWAMS server running at http://localhost%s", address)

	if err := router.Run(address); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
