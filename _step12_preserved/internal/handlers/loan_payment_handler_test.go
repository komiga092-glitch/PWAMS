package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// randSuffix returns a short random string for unique fixture names.
func randSuffix() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
}

// loanPaymentTestDB brings the configured PostgreSQL database to the canonical
// state (migrations + role seed + permission seed). Tests are skipped when no
// database is reachable so `go test ./...` still succeeds in plain
// environments.
func loanPaymentTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	_ = godotenv.Load("../../.env")
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("loan payment handler test skipped (configuration unavailable): %v", err)
	}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Skipf("loan payment handler test skipped (database unavailable): %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("loan payment handler test migration failed: %v", err)
	}
	if err := database.SeedDefaultRoles(db); err != nil {
		t.Fatalf("loan payment handler test role seed failed: %v", err)
	}
	permSvc := services.NewPermissionService(
		repository.NewPermissionRepository(db),
		repository.NewRoleRepository(db),
		repository.NewAuditLogRepository(db),
	)
	if err := permSvc.SeedDefaults(); err != nil {
		t.Fatalf("loan payment handler test permission seed failed: %v", err)
	}
	return db
}

// TestLoanPaymentHandler_PayRecordsAuditAndAuthorizes drives the REAL Pay
// handler behind the REAL production role/permission middleware chain and
// verifies:
//
//  1. an authorized Staff user can submit a repayment (LOAN_PAYMENT stays
//     working end to end),
//  2. the handler records the mandatory LOAN_PAYMENT audit event attributed to
//     the session user (never a client-supplied identity),
//  3. a user outside the Super Admin/Admin/Staff role gate cannot submit a
//     repayment at all (HTTP 403, no audit row, no state change).
//
// RequireAuth itself is replaced by injecting the authenticated user into the
// gin context — exactly the value RequireAuth produces — so the authorization
// chain exercised here is the production one.
func TestLoanPaymentHandler_PayRecordsAuditAndAuthorizes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := loanPaymentTestDB(t)

	permSvc := services.NewPermissionService(
		repository.NewPermissionRepository(db),
		repository.NewRoleRepository(db),
		repository.NewAuditLogRepository(db),
	)

	// --- Fixtures: a Staff (authorized) and a Beneficiary (rejected) user ---
	var staffRole, beneficiaryRole models.Role
	if err := db.Where("name = ?", models.RoleStaff).First(&staffRole).Error; err != nil {
		t.Fatalf("resolve Staff role: %v", err)
	}
	if err := db.Where("name = ?", models.RoleBeneficiary).First(&beneficiaryRole).Error; err != nil {
		t.Fatalf("resolve Beneficiary role: %v", err)
	}

	dummyHash := "$2a$10$dummyhashdummyhashdummyhashdummyhashdummyhashdummyhashdum"
	staff := &models.User{
		Username:     "pay-h-staff-" + randSuffix(),
		Email:        "pay-h-staff-" + randSuffix() + "@pwams.local",
		FullName:     "Pay Handler Staff",
		RoleID:       staffRole.ID,
		Status:       models.UserStatusActive,
		PasswordHash: dummyHash,
	}
	if err := db.Create(staff).Error; err != nil {
		t.Fatalf("create staff fixture: %v", err)
	}
	beneficiary := &models.User{
		Username:     "pay-h-ben-" + randSuffix(),
		Email:        "pay-h-ben-" + randSuffix() + "@pwams.local",
		FullName:     "Pay Handler Beneficiary",
		RoleID:       beneficiaryRole.ID,
		Status:       models.UserStatusActive,
		PasswordHash: dummyHash,
	}
	if err := db.Create(beneficiary).Error; err != nil {
		t.Fatalf("create beneficiary fixture: %v", err)
	}

	// Load the Role association the RBAC middleware reads.
	if err := db.Preload("Role").First(staff, staff.ID).Error; err != nil {
		t.Fatalf("reload staff fixture: %v", err)
	}
	if err := db.Preload("Role").First(beneficiary, beneficiary.ID).Error; err != nil {
		t.Fatalf("reload beneficiary fixture: %v", err)
	}

	// --- Business fixtures: person + active loan with a payable installment ---
	personRepo := repository.NewPersonRepository(db)
	personSvc := services.NewPersonService(personRepo)
	person, err := personSvc.CreatePerson(models.CreatePersonRequest{
		FullName:      "Pay Handler Person",
		NICPassport:   "PH" + randSuffix(),
		Gender:        "Male",
		Phone:         "0771234567",
		Email:         "pay-h-person-" + randSuffix() + "@pwams.local",
		Address:       "1 Handler Way",
		Occupation:    "Tester",
		MonthlyIncome: decimal.NewFromInt(40000),
	}, staff.ID)
	if err != nil {
		t.Fatalf("create person fixture: %v", err)
	}

	loanRepo := repository.NewLoanRepository(db)
	repayRepo := repository.NewLoanRepaymentRepository(db)
	notifSvc := services.NewNotificationService(repository.NewNotificationRepository(db))
	auditSvc := services.NewAuditLogService(repository.NewAuditLogRepository(db))
	loanSvc := services.NewLoanServiceWithNotifications(loanRepo, repayRepo, personRepo, db, notifSvc)
	repaySvc := services.NewLoanRepaymentServiceWithPerson(repayRepo, loanRepo, personRepo, db)

	loan, err := loanSvc.CreateLoan(models.CreateLoanRequest{
		PersonID:       person.ID.String(),
		LoanAmount:     decimal.NewFromInt(5000),
		InterestRate:   decimal.NewFromInt(0),
		DurationMonths: 2,
		StartDate:      "2026-09-01",
		DueDay:         1,
		Purpose:        "Pay handler audit test",
	}, staff.ID)
	if err != nil {
		t.Fatalf("create loan fixture: %v", err)
	}
	for _, st := range []string{models.LoanStatusApproved, models.LoanStatusActive} {
		if _, err := loanSvc.ReviewLoan(loan.ID.String(), models.ReviewLoanRequest{Status: st}, staff.ID); err != nil {
			t.Fatalf("review loan to %s: %v", st, err)
		}
	}

	var installment models.LoanRepayment
	if err := db.Where("loan_id = ? AND is_deleted = FALSE", loan.ID).
		Order("installment_number ASC").First(&installment).Error; err != nil {
		t.Fatalf("load first installment: %v", err)
	}

	repaymentHandler := NewLoanRepaymentHandler(repaySvc, auditSvc)
	setUser := func(u *models.User) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set("current_user", u)
			c.Next()
		}
	}

	// Production middleware chain for PATCH /loan-repayments/:id/pay.
	buildRouter := func(u *models.User) *gin.Engine {
		router := gin.New()
		router.PATCH("/loan-repayments/:id/pay",
			setUser(u),
			middleware.RequireAnyRole(models.RoleSuperAdmin, models.RoleAdmin, models.RoleStaff),
			middleware.LoadPermissions(permSvc),
			middleware.RequirePermission(permSvc, "repayment.create"),
			repaymentHandler.Pay,
		)
		return router
	}

	auditCount := func(entityID string) int64 {
		var count int64
		if err := db.Model(&models.AuditLog{}).
			Where("action = ? AND entity_id = ?", "LOAN_PAYMENT", entityID).
			Count(&count).Error; err != nil {
			t.Fatalf("count LOAN_PAYMENT audit rows: %v", err)
		}
		return count
	}

	// --- Authorized payment succeeds and is audited ---
	payload := `{"paid_amount":` + installment.OutstandingAmount.StringFixed(2) +
		`,"payment_method":"Cash","payment_reference":"PAY-HANDLER-1","notes":"handler audit test"}`
	req := httptest.NewRequest(http.MethodPatch,
		"/loan-repayments/"+installment.ID.String()+"/pay", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	buildRouter(staff).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("authorized payment status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var payResponse struct {
		Success   bool `json:"success"`
		Repayment struct {
			ID            string `json:"id"`
			Status        string `json:"status"`
			PaidAmount    string `json:"paid_amount"`
			InstallmentNo int    `json:"installment_number"`
		} `json:"repayment"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payResponse); err != nil {
		t.Fatalf("decode pay response: %v", err)
	}
	if !payResponse.Success || payResponse.Repayment.Status != models.RepaymentStatusPaid {
		t.Errorf("pay response success=%v status=%q, want success=true status=%q",
			payResponse.Success, payResponse.Repayment.Status, models.RepaymentStatusPaid)
	}

	if got := auditCount(installment.ID.String()); got != 1 {
		t.Errorf("LOAN_PAYMENT audit count = %d, want 1", got)
	}

	var auditEntry models.AuditLog
	if err := db.Where("action = ? AND entity_id = ?", "LOAN_PAYMENT", installment.ID).
		First(&auditEntry).Error; err != nil {
		t.Fatalf("load LOAN_PAYMENT audit row: %v", err)
	}
	if auditEntry.UserID == nil || *auditEntry.UserID != staff.ID {
		t.Errorf("LOAN_PAYMENT audit user = %v, want paying user %s", auditEntry.UserID, staff.ID)
	}
	if !strings.Contains(auditEntry.Details, "installment=1") {
		t.Errorf("LOAN_PAYMENT audit details = %q, want installment number", auditEntry.Details)
	}

	// --- Unauthorized role is rejected before the handler runs ---
	forbidden := httptest.NewRequest(http.MethodPatch,
		"/loan-repayments/"+installment.ID.String()+"/pay", strings.NewReader(payload))
	forbidden.Header.Set("Content-Type", "application/json")
	forbiddenRec := httptest.NewRecorder()
	buildRouter(beneficiary).ServeHTTP(forbiddenRec, forbidden)

	if forbiddenRec.Code != http.StatusForbidden {
		t.Errorf("unauthorized payment status = %d, want 403 (body: %s)",
			forbiddenRec.Code, forbiddenRec.Body.String())
	}
	if got := auditCount(installment.ID.String()); got != 1 {
		t.Errorf("LOAN_PAYMENT audit count after forbidden attempt = %d, want 1 (unchanged)", got)
	}
}
