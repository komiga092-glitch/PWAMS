package main

import (
	"log"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/komiga092-glitch/pwams/docs"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/i18n"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
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

	// Release mode for production: disables debug warnings and
	// request-level debug logging (secure-by-default).
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
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
	messageRepo := repository.NewMessageRepository(db)
	dashboardRepo := repository.NewDashboardRepository(db)
	fileUploadRepo := repository.NewFileUploadRepository(db)

	loanRepo := repository.NewLoanRepository(db)
	loanRepaymentRepo := repository.NewLoanRepaymentRepository(db)
	revenueRepo := repository.NewRevenueRepository(db)

	syncRepository := repository.NewSyncRepository(db)

	personSyncService := services.NewPersonSyncService(
		personRepository,
	)

	entitySyncServices := map[string]*services.EntitySyncService{
		"student": services.NewEntitySyncService(services.SyncEntityAdapter{
			Entity: "student", New: func() any { return &models.Student{} },
			Find:     func(id uuid.UUID) (any, error) { return studentRepository.FindByID(id.String()) },
			Create:   func(value any) error { return studentRepository.Create(value.(*models.Student)) },
			Update:   func(value any) error { return studentRepository.Update(value.(*models.Student)) },
			Validate: func(any) error { return nil },
		}),
		"donor": services.NewEntitySyncService(services.SyncEntityAdapter{
			Entity: "donor", New: func() any { return &models.Donor{} },
			Find:     func(id uuid.UUID) (any, error) { return donorRepository.FindByID(id.String()) },
			Create:   func(value any) error { return donorRepository.Create(value.(*models.Donor)) },
			Update:   func(value any) error { return donorRepository.Update(value.(*models.Donor)) },
			Validate: func(any) error { return nil },
		}),
		"aid_request": services.NewEntitySyncService(services.SyncEntityAdapter{
			Entity: "aid_request", New: func() any { return &models.AidRequest{} },
			Find:     func(id uuid.UUID) (any, error) { return aidRequestRepository.FindByID(id.String()) },
			Create:   func(value any) error { return aidRequestRepository.Create(value.(*models.AidRequest)) },
			Update:   func(value any) error { return aidRequestRepository.Update(value.(*models.AidRequest)) },
			Validate: func(any) error { return nil },
		}),
		"care_provided": services.NewEntitySyncService(services.SyncEntityAdapter{
			Entity: "care_provided", New: func() any { return &models.CareProvided{} },
			Find:     func(id uuid.UUID) (any, error) { return careProvidedRepo.FindByID(id) },
			Create:   func(value any) error { return careProvidedRepo.Create(value.(*models.CareProvided)) },
			Update:   func(value any) error { return careProvidedRepo.Update(value.(*models.CareProvided)) },
			Validate: func(any) error { return nil },
		}),
		"loan": services.NewEntitySyncService(services.SyncEntityAdapter{
			Entity: "loan", New: func() any { return &models.Loan{} },
			Find:     func(id uuid.UUID) (any, error) { return loanRepo.FindByID(id.String()) },
			Create:   func(value any) error { return loanRepo.Create(value.(*models.Loan)) },
			Update:   func(value any) error { return loanRepo.Update(value.(*models.Loan)) },
			Validate: func(any) error { return nil },
		}),
		"loan_repayment": services.NewEntitySyncService(services.SyncEntityAdapter{
			Entity: "loan_repayment", New: func() any { return &models.LoanRepayment{} },
			Find:     func(id uuid.UUID) (any, error) { return loanRepaymentRepo.FindByID(id.String()) },
			Create:   func(value any) error { return loanRepaymentRepo.Create(value.(*models.LoanRepayment)) },
			Update:   func(value any) error { return loanRepaymentRepo.Update(value.(*models.LoanRepayment)) },
			Validate: func(any) error { return nil },
		}),
	}

	syncService := services.NewSyncService(
		syncRepository,
		personSyncService,
		entitySyncServices,
	)

	syncHandler := handlers.NewSyncHandler(
		syncService,
		db,
	)
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
		sessionRepo,
	)

	userService := services.NewUserService(
		userRepo,
		roleRepository,
		sessionRepo,
	)

	dashboardService := services.NewDashboardService(
		userRepo,
		dashboardRepo,
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

	messageService := services.NewMessageService(
		messageRepo,
		userRepo,
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
		db,
	)

	revenueService := services.NewRevenueService(revenueRepo)

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
		auditLogService,
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
		donorService,
	)

	aidRequestHandler := handlers.NewAidRequestHandler(
		aidRequestService,
		auditLogService,
	)

	notificationHandler := handlers.NewNotificationHandler(
		notificationService,
	)

	messageHandler := handlers.NewMessageHandler(
		messageService,
	)

	messageRecipientHandler := handlers.NewMessageRecipientHandler(
		userRepo,
	)

	fileUploadHandler := handlers.NewFileUploadHandler(
		fileUploadService,
		auditLogService,
	)

	loanHandler := handlers.NewLoanHandler(
		loanService,
	)

	loanRepaymentHandler := handlers.NewLoanRepaymentHandler(
		loanRepaymentService,
		auditLogService,
	)

	revenueHandler := handlers.NewRevenueHandler(revenueService, auditLogService)

	careProvidedHandler := handlers.NewCareProvidedHandler(
		careProvidedService,
	)

	reportPDFService := services.NewReportPDFService()
	reportHandler := handlers.NewReportHandler(
		reportService,
		reportPDFService,
	)

	auditLogHandler := handlers.NewAuditLogHandler(
		auditLogService,
	)

	// =========================
	// Middleware
	// =========================

	authMiddleware := middleware.NewAuthMiddleware(
		sessionService,
		secureCookie,
	)

	// =========================
	// Router
	// =========================

	router := gin.Default()

	// Global middleware
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.RateLimitGeneric())

	// CSRF: issue token cookies on every response and validate the
	// double-submit token on all unsafe methods.
	router.Use(middleware.EnsureCSRF(secureCookie))

	// UI language resolution (English / Tamil / Sinhala, SRS section 32).
	router.Use(middleware.ResolveLocale(secureCookie))

	/*
		Template loading.

		Root templates:
			web/templates/home.html
			web/templates/login.html
			etc.

		Layout templates:
			web/templates/layouts/*.html
	*/

	// Template helper functions used by the report templates loaded below
	// (report_detail.html / report_page_content.html), e.g. pagination links.
	router.SetFuncMap(template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"t":   i18n.T,
	})

	router.LoadHTMLFiles(
		"web/templates/layouts/base.html",
		"web/templates/layouts/header.html",

		"web/templates/home.html",
		"web/templates/login.html",
		"web/templates/forgot_password.html",
		"web/templates/verify_reset_otp.html",
		"web/templates/reset_password.html",
		"web/templates/error.html",

		"web/templates/dashboard.html",
		"web/templates/users.html",
		"web/templates/profile.html",

		"web/templates/persons.html",
		"web/templates/person_form.html",
		"web/templates/person_view.html",
		"web/templates/person_edit.html",

		"web/templates/students.html",
		"web/templates/student_view.html",
		"web/templates/student_edit.html",

		"web/templates/donors.html",
		"web/templates/donor_view.html",
		"web/templates/donor_edit.html",
		"web/templates/donations.html",
		"web/templates/aid_requests.html",
		"web/templates/care_provided.html",
		"web/templates/loans.html",
		"web/templates/loan_repayments.html",
		"web/templates/revenue.html",
		"web/templates/notifications.html",
		"web/templates/messages.html",
		"web/templates/files.html",
		"web/templates/reports.html",
		"web/templates/report_detail.html",
		"web/templates/report_page_content.html",
		"web/templates/audit_logs.html",
	)
	router.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/static/js/offline/service-worker.js" {
			c.Header("Service-Worker-Allowed", "/")
		}
		c.Next()
	})
	router.Static("/static", "web/static")
	router.StaticFile("/offline.html", "web/static/offline.html")
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

	routes.Setup(router, db)

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

	routes.RegisterMessageRoutes(
		router,
		messageHandler,
		messageRecipientHandler,
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

	routes.RegisterSyncRoutes(
		router,
		syncHandler,
		authMiddleware,
	)

	routes.RegisterRevenueRoutes(
		router,
		revenueHandler,
		authMiddleware,
	)
	// Swagger UI is a development/documentation tool and must not be
	// exposed in production builds.
	if cfg.AppEnv != "production" {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
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

func isSafeHTTPMethod(method string) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}
