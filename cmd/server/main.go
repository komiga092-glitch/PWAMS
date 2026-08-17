package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/routes"
	"github.com/komiga092-glitch/pwams/internal/services"
	"github.com/komiga092-glitch/pwams/internal/services/email"
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

	// =========================
	// Repositories
	// =========================

	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	passwordResetRepository := repository.NewPasswordResetRepository(db)
	roleRepository := repository.NewRoleRepository(db)

	personRepository := repository.NewPersonRepository(db)
	studentRepository := repository.NewStudentRepository(db)

	donorRepository := repository.NewDonorRepository(db)
	donationRepository := repository.NewDonationRepository(db)

	aidRequestRepository := repository.NewAidRequestRepository(db)
	accountActivationRepository := repository.NewAccountActivationRepository(db)

	careProvidedRepo := repository.NewCareProvidedRepository(db)
	reportRepo := repository.NewReportRepository(db)
	auditLogRepo := repository.NewAuditLogRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)
	fileUploadRepo := repository.NewFileUploadRepository(db)

	loanRepo := repository.NewLoanRepository(db)
	loanRepaymentRepo := repository.NewLoanRepaymentRepository(db)

	// =========================
	// Services
	// =========================

	auditLogService := services.NewAuditLogService(
		auditLogRepo,
	)

	reportService := services.NewReportService(
		reportRepo,
	)

	careProvidedService := services.NewCareProvidedService(
		careProvidedRepo,
	)

	authService := services.NewAuthService(
		userRepo,
	)

	sessionService := services.NewSessionService(
		sessionRepo,
	)

	emailService := email.NewEmailService(
		cfg,
	)

	passwordResetService := services.NewPasswordResetService(
		passwordResetRepository,
		userRepo,
		emailService,
	)

	userService := services.NewUserService(
		userRepo,
		roleRepository,
		sessionRepo,
	)

	dashboardService := services.NewDashboardService(
		userRepo,
	)

	personService := services.NewPersonService(
		personRepository,
	)

	studentService := services.NewStudentService(
		studentRepository,
		personRepository,
	)

	donorService := services.NewDonorService(
		donorRepository,
	)

	donationService := services.NewDonationService(
		donationRepository,
		donorRepository,
		personRepository,
	)

	notificationService := services.NewNotificationService(
		notificationRepo,
	)

	aidRequestService := services.NewAidRequestService(
		aidRequestRepository,
		personRepository,
		notificationService,
	)

	accountActivationService := services.NewAccountActivationService(
		accountActivationRepository,
		userRepo,
		emailService,
	)

	fileUploadService := services.NewFileUploadService(
		fileUploadRepo,
	)

	loanService := services.NewLoanService(
		loanRepo,
		personRepository,
	)

	loanRepaymentService := services.NewLoanRepaymentService(
		loanRepaymentRepo,
		loanRepo,
	)

	// =========================
	// Cookie configuration
	// =========================

	secureCookie := cfg.AppEnv == "production"

	// =========================
	// Handlers
	// =========================

	authHandler := handlers.NewAuthHandler(
		authService,
		sessionService,
		passwordResetService,
		secureCookie,
	)

	accountActivationHandler := handlers.NewAccountActivationHandler(
		accountActivationService,
	)

	userHandler := handlers.NewUserHandler(
		userService,
		auditLogService,
	)

	dashboardHandler := handlers.NewDashboardHandler(
		dashboardService,
	)

	personHandler := handlers.NewPersonHandler(
		personService,
	)

	studentHandler := handlers.NewStudentHandler(
		studentService,
	)

	donorHandler := handlers.NewDonorHandler(
		donorService,
	)

	donationHandler := handlers.NewDonationHandler(
		donationService,
	)

	aidRequestHandler := handlers.NewAidRequestHandler(
		aidRequestService,
	)

	notificationHandler := handlers.NewNotificationHandler(
		notificationService,
	)

	fileUploadHandler := handlers.NewFileUploadHandler(
		fileUploadService,
	)

	loanHandler := handlers.NewLoanHandler(
		loanService,
	)

	loanRepaymentHandler := handlers.NewLoanRepaymentHandler(
		loanRepaymentService,
	)

	careProvidedHandler := handlers.NewCareProvidedHandler(
		careProvidedService,
	)

	reportHandler := handlers.NewReportHandler(
		reportService,
	)

	auditLogHandler := handlers.NewAuditLogHandler(
		auditLogService,
	)

	// =========================
	// Middleware
	// =========================

	authMiddleware := middleware.NewAuthMiddleware(
		sessionService,
	)

	// =========================
	// Router
	// =========================

	router := gin.Default()

	/*
		Template loading.

		Root templates:
			web/templates/home.html
			web/templates/login.html
			etc.

		Layout templates:
			web/templates/layouts/*.html
	*/

	router.LoadHTMLFiles(
		"web/templates/layouts/base.html",
		"web/templates/layouts/header.html",
		"web/templates/home.html",
		"web/templates/login.html",
		"web/templates/forgot_password.html",
		"web/templates/verify_reset_otp.html",
"web/templates/reset_password.html",
		"web/templates/dashboard.html",
		"web/templates/users.html",
		"web/templates/persons.html",
		"web/templates/person_form.html",
		"web/templates/students.html",
		"web/templates/donors.html",
		"web/templates/donations.html",
		"web/templates/aid_requests.html",
		"web/templates/care_provided.html",
		"web/templates/loans.html",
		"web/templates/loan_repayments.html",
		"web/templates/notifications.html",
		"web/templates/reports.html",
		"web/templates/audit_logs.html",
		"web/templates/students.html",
		"web/templates/students_table.html",
	)
	router.Static("/static", "web/static")
	// =========================
	// Background Jobs
	// =========================

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			if err := loanRepaymentService.MarkOverdue(); err != nil {
				log.Printf(
					"failed to mark overdue repayments: %v",
					err,
				)
			}

			<-ticker.C
		}
	}()

	// =========================
	// Routes
	// =========================

	routes.Setup(router)

	routes.RegisterAuthRoutes(
		router,
		authHandler,
		dashboardHandler,
		authMiddleware,
	)

	routes.RegisterAccountActivationRoutes(
		router,
		accountActivationHandler,
	)

	routes.RegisterUserRoutes(
		router,
		userHandler,
		authMiddleware,
	)

	routes.RegisterDashboardRoutes(
		router,
		dashboardHandler,
		authMiddleware,
	)

	routes.RegisterPersonRoutes(
		router,
		personHandler,
		authMiddleware,
	)

	routes.RegisterStudentRoutes(
		router,
		studentHandler,
		authMiddleware,
	)

	routes.RegisterDonorRoutes(
		router,
		donorHandler,
		authMiddleware,
	)

	routes.RegisterDonationRoutes(
		router,
		donationHandler,
		authMiddleware,
	)

	routes.RegisterAidRequestRoutes(
		router,
		aidRequestHandler,
		authMiddleware,
	)

	routes.RegisterCareProvidedRoutes(
		router,
		careProvidedHandler,
		authMiddleware,
	)
	routes.RegisterReportRoutes(
		router,
		reportHandler,
		authMiddleware,
	)

	routes.RegisterAuditLogRoutes(
		router,
		auditLogHandler,
		authMiddleware,
	)

	routes.RegisterNotificationRoutes(
		router,
		notificationHandler,
		authMiddleware,
	)

	routes.RegisterFileUploadRoutes(
		router,
		fileUploadHandler,
		authMiddleware,
	)

	routes.RegisterLoanRoutes(
		router,
		loanHandler,
		authMiddleware,
	)

	routes.RegisterLoanRepaymentRoutes(
		router,
		loanRepaymentHandler,
		authMiddleware,
	)

	// =========================
	// Start Server
	// =========================

	address := ":" + cfg.AppPort

	log.Printf(
		"PWAMS server running at http://localhost%s",
		address,
	)

	if err := router.Run(address); err != nil {
		log.Fatalf(
			"server failed: %v",
			err,
		)
	}
}
