package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
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

	// Database migration.
	//
	// PHASE 4D: production startup performs NO schema writes. It verifies
	// the tracked migration state (schema_migrations) and fails fast when
	// migrations are pending or drifted; operators apply them explicitly
	// with `go run ./cmd/migrate up` (after a verified backup). Development
	// keeps the AutoMigrate convenience path, which never deletes data.
	if cfg.AppEnv == "production" {
		if err := database.VerifyUpToDate(db); err != nil {
			log.Fatalf("schema verification error: %v", err)
		}
	} else if err := database.Migrate(db); err != nil {
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
		auditLogService,
	)

	studentHandler := handlers.NewStudentHandler(
		studentService,
		auditLogService,
	)

	donorHandler := handlers.NewDonorHandler(
		donorService,
		auditLogService,
	)

	donationHandler := handlers.NewDonationHandler(
		donationService,
		donorService,
		auditLogService,
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
		auditLogService,
	)

	loanRepaymentHandler := handlers.NewLoanRepaymentHandler(
		loanRepaymentService,
		auditLogService,
	)

	revenueHandler := handlers.NewRevenueHandler(revenueService, auditLogService)

	careProvidedHandler := handlers.NewCareProvidedHandler(
		careProvidedService,
		auditLogService,
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

	// Trust only the configured reverse proxies (TRUSTED_PROXIES). With no
	// proxy configured Gin trusts no X-Forwarded-For / X-Real-IP header, so
	// c.ClientIP() always reflects the real TCP peer and IP based controls
	// (login / OTP / password reset rate limiting) cannot be bypassed by a
	// spoofed forwarding header.
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Fatalf("invalid TRUSTED_PROXIES configuration: %v", err)
	}

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

		The authoritative file list lives in internal/routes/template_registry.go
		so the server, cmd/tplcheck, and the tests all load exactly the same set.
		html/template resolves `{{ template "name" }}` references across the whole
		set, so a partial list makes base.html's dispatch chain unresolvable and
		every page silently renders an empty body.
	*/

	router.SetFuncMap(template.FuncMap(routes.TemplateFuncMap()))

	router.LoadHTMLFiles(routes.TemplateFiles()...)
	router.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/static/js/offline/service-worker.js" {
			c.Header("Service-Worker-Allowed", "/")
		}
		c.Next()
	})
	router.Static("/static", "web/static")
	router.StaticFile("/offline.html", "web/static/offline.html")
	// Browsers request /favicon.ico automatically; serve the org logo so pages
	// do not log a 404 console error on every visit.
	router.StaticFile("/favicon.ico", "web/static/images/org-logo.png")
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

	listener, port, err := chooseAvailableAddress(cfg.AppPort)
	if err != nil {
		log.Fatalf("server failed to allocate a port: %v", err)
	}
	defer listener.Close()

	log.Printf(
		"PWAMS server running at http://localhost:%s",
		port,
	)

	if err := router.RunListener(listener); err != nil {
		log.Fatalf(
			"server failed: %v",
			err,
		)
	}
}

func chooseAvailableAddress(requestedPort string) (net.Listener, string, error) {
	trimmed := strings.TrimSpace(requestedPort)
	if trimmed == "" {
		trimmed = "8080"
	}

	candidates := []string{trimmed}
	for start := 8081; start <= 8099; start++ {
		candidates = append(candidates, strconv.Itoa(start))
	}

	var bindErrors []string
	for _, port := range candidates {
		listener, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", port))
		if err == nil {
			return listener, port, nil
		}
		bindErrors = append(bindErrors, fmt.Sprintf("%s: %v", port, err))
	}

	return nil, "", fmt.Errorf(
		"no free port found for requested port %q (tried %s-8099): %s",
		trimmed, trimmed, strings.Join(bindErrors, "; "),
	)
}

func isSafeHTTPMethod(method string) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}
