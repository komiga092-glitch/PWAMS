package services_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// auditEnv bundles the shared live-DB dependencies for the behavioral audit
// tests. Every test in this file drives the REAL handler -> service ->
// repository -> PostgreSQL path through gin TestMode and then asserts the
// audit rows those mutations actually produced (or, for denied mutations,
// that no audit row was written).
type auditEnv struct {
	db    *gorm.DB
	fx    *testFixture
	roles seedRoles
	audit *services.AuditLogService
}

func newAuditEnv(t *testing.T) auditEnv {
	t.Helper()
	db := AcquireTestDB(t)
	fx := newFixture(db)
	roles := loadSeedRoles(t, db)
	gin.SetMode(gin.TestMode)
	auditSvc := services.NewAuditLogService(
		repository.NewAuditLogRepository(db),
	)
	return auditEnv{db: db, fx: fx, roles: roles, audit: auditSvc}
}

// routeAs builds a fresh router that injects user as the current_user and
// registers handler at path. This mirrors the auth middleware wiring in
// cmd/server/main.go (it stores *models.User in the context).
func routeAs(handler func(c *gin.Context), user *models.User, path, method string) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("current_user", user)
		c.Next()
	})
	switch method {
	case "POST":
		router.POST(path, handler)
	case "DELETE":
		router.DELETE(path, handler)
	}
	return router
}

func postJSON(router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func deleteRequest(router *gin.Engine, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodDelete, path, strings.NewReader(""))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

// countAuditRows counts audit rows for a specific actor/entity/action. Each
// test uses a freshly created actor (unique UUID), so == 0 truly means the
// denied path wrote nothing, and >= 1 means the mutation's audit landed.
func countAuditRows(t *testing.T, db *gorm.DB, entity, action string, userID uuid.UUID) int64 {
	t.Helper()
	var count int64
	err := db.Model(&models.AuditLog{}).
		Where("entity = ? AND action = ? AND user_id = ?", entity, action, userID).
		Count(&count).Error
	if err != nil {
		t.Fatalf("audit count failed: %v", err)
	}
	return count
}

//  1. User management: creating a user through the real handler writes a
//     users/CREATE audit row attributed to the acting admin.
func TestAuditWrittenForUserCreation(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	actor := makeUser(t, env.db, env.fx, env.roles.AdminID, models.RoleAdmin, "uactor")
	userSvc := services.NewUserService(
		repository.NewUserRepository(env.db),
		repository.NewRoleRepository(env.db),
		repository.NewSessionRepository(env.db),
		repository.NewAdminDeletionRequestRepository(env.db),
	)
	handler := handlers.NewUserHandler(userSvc, env.audit)
	router := routeAs(handler.Create, &actor, "/users", "POST")

	suffix := actor.ID.String()[:8]
	// The user is created through the handler and is not tracked by the
	// shared fixture; remove it by its unique username so user-created
	// cleanup does not leave orphaned rows (FK RESTRICT on created_by).
	defer func() {
		_ = env.db.Unscoped().Delete(&models.User{}, "username = ?", "audit_u_"+suffix).Error
	}()
	resp := postJSON(router, "/users", fmt.Sprintf(`{"username":"audit_u_%s","email":"audit_u_%s@pwams.test","password":"StrongPass123","role":"Staff"}`, suffix, suffix))
	if resp.Code != http.StatusCreated {
		t.Fatalf("user create status = %d, want %d", resp.Code, http.StatusCreated)
	}
	if n := countAuditRows(t, env.db, "users", "CREATE", actor.ID); n < 1 {
		t.Fatalf("user create wrote %d audit rows, want >= 1", n)
	}
}

//  2. User management negative: an Admin cannot create a Super Admin. The
//     authorization guard runs BEFORE service/audit, so no audit row may be
//     written for the denied attempt.
func TestNoAuditForForbiddenSuperAdminCreation(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	actor := makeUser(t, env.db, env.fx, env.roles.AdminID, models.RoleAdmin, "blocked")
	userSvc := services.NewUserService(
		repository.NewUserRepository(env.db),
		repository.NewRoleRepository(env.db),
		repository.NewSessionRepository(env.db),
		repository.NewAdminDeletionRequestRepository(env.db),
	)
	handler := handlers.NewUserHandler(userSvc, env.audit)
	router := routeAs(handler.Create, &actor, "/users", "POST")

	suffix := actor.ID.String()[:8]
	resp := postJSON(router, "/users", fmt.Sprintf(`{"username":"audit_sa_%s","email":"audit_sa_%s@pwams.test","password":"StrongPass123","role":"Super Admin"}`, suffix, suffix))
	if resp.Code != http.StatusForbidden {
		t.Fatalf("admin creating Super Admin status = %d, want %d", resp.Code, http.StatusForbidden)
	}
	if n := countAuditRows(t, env.db, "users", "CREATE", actor.ID); n != 0 {
		t.Fatalf("denied user create wrote %d audit rows, want 0 (authorization must precede audit)", n)
	}
}

//  3. Person: creating a person through the real handler writes a
//     persons/CREATE audit row for the acting staff member.
func TestAuditWrittenForPersonCreation(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	actor := makeUser(t, env.db, env.fx, env.roles.StaffID, models.RoleStaff, "pactor")
	personSvc := services.NewPersonService(repository.NewPersonRepository(env.db))
	handler := handlers.NewPersonHandler(personSvc, env.audit)
	router := routeAs(handler.Create, &actor, "/persons", "POST")

	suffix := actor.ID.String()[:8]
	defer func() {
		_ = env.db.Unscoped().Delete(&models.Person{}, "nic_passport = ?", "NIC-"+suffix).Error
	}()
	resp := postJSON(router, "/persons", fmt.Sprintf(`{"full_name":"Audit Person %s","nic_passport":"NIC-%s","gender":"Female","monthly_income":0}`, suffix, suffix))
	if resp.Code != http.StatusCreated {
		t.Fatalf("person create status = %d, want %d", resp.Code, http.StatusCreated)
	}
	if n := countAuditRows(t, env.db, "persons", "CREATE", actor.ID); n < 1 {
		t.Fatalf("person create wrote %d audit rows, want >= 1", n)
	}
}

//  4. Person negative: a non-privileged donor attempting to update another
//     staff member's person record is refused by the service (IDOR guard) and
//     the denied path must NOT write an audit row.
func TestNoAuditForObjectLevelAuthorizationDenial(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	owner := makeUser(t, env.db, env.fx, env.roles.StaffID, models.RoleStaff, "owner")
	intruder := makeUser(t, env.db, env.fx, env.roles.DonorID, models.RoleDonor, "intruder")
	person := makePerson(t, env.db, env.fx, owner.ID, "owned")

	personSvc := services.NewPersonService(repository.NewPersonRepository(env.db))
	handler := handlers.NewPersonHandler(personSvc, env.audit)
	router := routeAs(handler.Update, &intruder, "/persons/"+person.ID.String(), "POST")

	resp := postJSON(router, "/persons/"+person.ID.String(), fmt.Sprintf(`{"full_name":"Hacked","nic_passport":"%s","phone":"+94770000000","status":"Active"}`, person.NICPassport))
	if resp.Code < 400 {
		t.Fatalf("cross-user person update status = %d, want >= 400", resp.Code)
	}
	if n := countAuditRows(t, env.db, "persons", "UPDATE", intruder.ID); n != 0 {
		t.Fatalf("denied person update wrote %d audit rows, want 0", n)
	}
}

//  5. Student: creating a student through the real handler writes a
//     students/CREATE audit row.
func TestAuditWrittenForStudentCreation(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	actor := makeUser(t, env.db, env.fx, env.roles.StaffID, models.RoleStaff, "sactor")
	person := makePerson(t, env.db, env.fx, actor.ID, "student-parent")

	studentSvc := services.NewStudentService(
		repository.NewStudentRepository(env.db),
		repository.NewPersonRepository(env.db),
	)
	handler := handlers.NewStudentHandler(studentSvc, env.audit)
	router := routeAs(handler.Create, &actor, "/students", "POST")

	suffix := actor.ID.String()[:8]
	code := "SC-" + suffix
	defer func() {
		_ = env.db.Unscoped().Delete(&models.Student{}, "student_code = ?", code).Error
	}()
	resp := postJSON(router, "/students", fmt.Sprintf(`{"person_id":"%s","full_name":"Audit Student %s","school_name":"School %s","grade":"5","academic_year":2026,"student_code":"%s","guardian_name":"Guardian %s","guardian_phone":"+94771234567"}`, person.ID.String(), suffix, suffix, code, suffix))
	if resp.Code != http.StatusCreated {
		t.Fatalf("student create status = %d, want %d", resp.Code, http.StatusCreated)
	}
	if n := countAuditRows(t, env.db, "students", "CREATE", actor.ID); n < 1 {
		t.Fatalf("student create wrote %d audit rows, want >= 1", n)
	}
}

//  6. Donor: creating a donor through the real handler writes a
//     donors/CREATE audit row.
func TestAuditWrittenForDonorCreation(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	actor := makeUser(t, env.db, env.fx, env.roles.StaffID, models.RoleStaff, "dactor")
	donorSvc := services.NewDonorService(repository.NewDonorRepository(env.db))
	handler := handlers.NewDonorHandler(donorSvc, env.audit)
	router := routeAs(handler.Create, &actor, "/donors", "POST")

	suffix := actor.ID.String()[:8]
	defer func() {
		_ = env.db.Unscoped().Delete(&models.Donor{}, "email = ?", fmt.Sprintf("donor_%s@pwams.test", suffix)).Error
	}()
	resp := postJSON(router, "/donors", fmt.Sprintf(`{"name":"Audit Donor %s","donor_type":"Individual","nic_passport":"DN-%s","phone":"+94771234567","email":"donor_%s@pwams.test"}`, suffix, suffix, suffix))
	if resp.Code != http.StatusCreated {
		t.Fatalf("donor create status = %d, want %d", resp.Code, http.StatusCreated)
	}
	if n := countAuditRows(t, env.db, "donors", "CREATE", actor.ID); n < 1 {
		t.Fatalf("donor create wrote %d audit rows, want >= 1", n)
	}
}

//  7. Donation: creating a donation through the real handler writes a
//     donations/CREATE audit row.
func TestAuditWrittenForDonationCreation(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	actor := makeUser(t, env.db, env.fx, env.roles.StaffID, models.RoleStaff, "donactor")
	donor := makeDonorActive(t, env.db, env.fx, actor.ID, "audit")

	donorSvc := services.NewDonorService(repository.NewDonorRepository(env.db))
	donationSvc := services.NewDonationService(
		repository.NewDonationRepository(env.db),
		repository.NewDonorRepository(env.db),
		repository.NewPersonRepository(env.db),
	)
	handler := handlers.NewDonationHandler(donationSvc, donorSvc, env.audit)
	router := routeAs(handler.Create, &actor, "/donations", "POST")

	suffix := actor.ID.String()[:8]
	defer func() {
		_ = env.db.Unscoped().Delete(&models.Donation{}, "reference_no = ?", "AUDREF-"+suffix).Error
	}()
	resp := postJSON(router, "/donations", fmt.Sprintf(`{"donor_id":"%s","donation_type":"Cash","amount":1000,"currency":"LKR","quantity":1,"unit":"","item_name":"Rice pack","donation_date":"2026-09-01","reference_no":"AUDREF-%s"}`, donor.ID.String(), suffix))
	if resp.Code != http.StatusCreated {
		t.Fatalf("donation create status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
	}
	if n := countAuditRows(t, env.db, "donations", "CREATE", actor.ID); n < 1 {
		t.Fatalf("donation create wrote %d audit rows, want >= 1", n)
	}
}

//  8. Aid Request: creating an aid request through the real handler writes an
//     aid_requests/CREATE audit row.
func TestAuditWrittenForAidRequestCreation(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	actor := makeUser(t, env.db, env.fx, env.roles.StaffID, models.RoleStaff, "aactor")
	person := makePerson(t, env.db, env.fx, actor.ID, "aid-beneficiary")
	// aid_requests are not tracked by the shared fixture; clear by unique title.
	marker := "Audit aid " + actor.ID.String()[:8]
	defer func() {
		_ = env.db.Unscoped().Delete(&models.AidRequest{}, "title = ?", marker).Error
	}()

	notificationSvc := services.NewNotificationService(repository.NewNotificationRepository(env.db))
	aidSvc := services.NewAidRequestService(
		repository.NewAidRequestRepository(env.db),
		repository.NewPersonRepository(env.db),
		notificationSvc,
	)
	handler := handlers.NewAidRequestHandler(aidSvc, env.audit)
	router := routeAs(handler.Create, &actor, "/aid-requests", "POST")

	resp := postJSON(router, "/aid-requests", fmt.Sprintf(`{"person_id":"%s","aid_type":"Medical","priority":"High","title":"%s","description":"Needs financial support for treatment","requested_amount":1000,"currency":"LKR","request_date":"2026-09-14"}`, person.ID.String(), marker))
	if resp.Code != http.StatusCreated {
		t.Fatalf("aid request create status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
	}
	if n := countAuditRows(t, env.db, "aid_requests", "CREATE", actor.ID); n < 1 {
		t.Fatalf("aid request create wrote %d audit rows, want >= 1", n)
	}
}

//  9. Care Provided: creating a care-provided record through the real handler
//     writes a care_provided/CREATE audit row.
func TestAuditWrittenForCareProvidedCreation(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	actor := makeUser(t, env.db, env.fx, env.roles.StaffID, models.RoleStaff, "cactor")
	person := makePerson(t, env.db, env.fx, actor.ID, "care-beneficiary")
	arMarker := "Audit care ar " + actor.ID.String()[:8]
	careMarker := "audit-care-" + actor.ID.String()[:8]
	defer func() {
		_ = env.db.Unscoped().Delete(&models.CareProvided{}, "description = ?", careMarker).Error
		_ = env.db.Unscoped().Delete(&models.AidRequest{}, "title = ?", arMarker).Error
	}()

	// The aid request was created through the real service above; the
	// aid_requests/CREATE handler flow is covered in its own test.
	notificationSvc := services.NewNotificationService(repository.NewNotificationRepository(env.db))
	aidSvc := services.NewAidRequestService(
		repository.NewAidRequestRepository(env.db),
		repository.NewPersonRepository(env.db),
		notificationSvc,
	)
	if _, err := aidSvc.CreateAidRequest(models.CreateAidRequest{
		PersonID:        person.ID.String(),
		AidType:         "Medical",
		Priority:        "High",
		Title:           arMarker,
		Description:     "Supporting aid request for care test",
		RequestedAmount: decimal.NewFromInt(1000),
		Currency:        "LKR",
	}, actor.ID); err != nil {
		t.Fatalf("failed to create supporting aid request: %v", err)
	}
	var arID string
	if err := env.db.Model(&models.AidRequest{}).Select("id").Where("title = ?", arMarker).Take(&arID).Error; err != nil {
		t.Fatalf("failed to reload supporting aid request: %v", err)
	}

	careSvc := services.NewCareProvidedService(repository.NewCareProvidedRepository(env.db))
	handler := handlers.NewCareProvidedHandler(careSvc, env.audit)
	router := routeAs(handler.Create, &actor, "/care-provided", "POST")

	resp := postJSON(router, "/care-provided", fmt.Sprintf(`{"aid_request_id":"%s","person_id":"%s","amount":150.5,"description":"%s","care_type":"medical","provided_by":"Health Unit","provided_at":"2026-09-13T00:00:00Z"}`, arID, person.ID.String(), careMarker))
	if resp.Code != http.StatusCreated {
		t.Fatalf("care provided create status = %d, want %d", resp.Code, http.StatusCreated)
	}
	if n := countAuditRows(t, env.db, "care_provided", "CREATE", actor.ID); n < 1 {
		t.Fatalf("care provided create wrote %d audit rows, want >= 1", n)
	}
}

//  10. Loan: creating a loan through the real handler writes a loans/CREATE
//     audit row.
func TestAuditWrittenForLoanCreation(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	actor := makeUser(t, env.db, env.fx, env.roles.StaffID, models.RoleStaff, "lactor")
	person := makePerson(t, env.db, env.fx, actor.ID, "loan-borrower")

	loanSvc := services.NewLoanService(
		repository.NewLoanRepository(env.db),
		repository.NewPersonRepository(env.db),
	)
	handler := handlers.NewLoanHandler(loanSvc, env.audit)
	router := routeAs(handler.Create, &actor, "/loans", "POST")

	suffix := actor.ID.String()[:8]
	loanMarker := "audit-loan-" + suffix
	defer func() {
		_ = env.db.Unscoped().Delete(&models.Loan{}, "purpose = ?", loanMarker).Error
	}()
	resp := postJSON(router, "/loans", fmt.Sprintf(`{"person_id":"%s","loan_amount":12000,"interest_rate":0,"duration_months":12,"purpose":"%s"}`, person.ID.String(), loanMarker))
	if resp.Code != http.StatusCreated {
		t.Fatalf("loan create status = %d, want %d", resp.Code, http.StatusCreated)
	}
	if n := countAuditRows(t, env.db, "loans", "CREATE", actor.ID); n < 1 {
		t.Fatalf("loan create wrote %d audit rows, want >= 1", n)
	}
}

//  11. Loan Repayment: creating a repayment through the real handler writes a
//     loan_repayments/CREATE audit row.
func TestAuditWrittenForLoanRepaymentCreation(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	actor := makeUser(t, env.db, env.fx, env.roles.StaffID, models.RoleStaff, "rpayactor")
	person := makePerson(t, env.db, env.fx, actor.ID, "repay-borrower")
	loanMarker := "audit-repay-loan-" + actor.ID.String()[:8]

	// The loan is created through the real service; the loans/CREATE handler
	// flow is covered in its own test. This one focuses on the repayment.
	loanSvc := services.NewLoanService(
		repository.NewLoanRepository(env.db),
		repository.NewPersonRepository(env.db),
	)
	loan, err := loanSvc.CreateLoan(models.CreateLoanRequest{
		PersonID:       person.ID.String(),
		LoanAmount:     decimal.NewFromInt(12000),
		InterestRate:   decimal.Zero,
		DurationMonths: 12,
		Purpose:        loanMarker,
	}, actor.ID)
	if err != nil {
		t.Fatalf("failed to create supporting loan: %v", err)
	}
	// Track the supporting loan in the shared fixture and remove the
	// handler-created repayment first (untracked by the fixture).
	env.fx.addLoan(loan.ID)
	defer func() {
		_ = env.db.Unscoped().Delete(&models.LoanRepayment{}, "loan_id = ?", loan.ID).Error
	}()

	repaySvc := services.NewLoanRepaymentService(
		repository.NewLoanRepaymentRepository(env.db),
		repository.NewLoanRepository(env.db),
		env.db,
	)
	handler := handlers.NewLoanRepaymentHandler(repaySvc, env.audit)
	router := routeAs(handler.Create, &actor, "/loan-repayments", "POST")

	resp := postJSON(router, "/loan-repayments", fmt.Sprintf(`{"loan_id":"%s","installment_number":1,"due_date":"2026-09-13","amount":1000,"notes":"audit repayment"}`, loan.ID.String()))
	if resp.Code != http.StatusCreated {
		t.Fatalf("repayment create status = %d, want %d", resp.Code, http.StatusCreated)
	}
	if n := countAuditRows(t, env.db, "loan_repayments", "CREATE", actor.ID); n < 1 {
		t.Fatalf("repayment create wrote %d audit rows, want >= 1", n)
	}
}

//  12. Revenue: creating a revenue record through the real handler writes a
//     revenue_records/CREATE audit row.
func TestAuditWrittenForRevenueCreation(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	actor := makeUser(t, env.db, env.fx, env.roles.AdminID, models.RoleAdmin, "revactor")
	// revenue_records are not tracked by the shared fixture; clear by reference.
	ref := "AUDREV-" + actor.ID.String()[:8]
	defer func() {
		_ = env.db.Unscoped().Delete(&models.RevenueRecord{}, "reference_no = ?", ref).Error
	}()

	revSvc := services.NewRevenueService(repository.NewRevenueRepository(env.db))
	handler := handlers.NewRevenueHandler(revSvc, env.audit)
	router := routeAs(handler.Create, &actor, "/revenue", "POST")

	resp := postJSON(router, "/revenue", fmt.Sprintf(`{"record_type":"income","category":"Donations","amount":500,"currency":"LKR","record_date":"2020-01-01T00:00:00Z","description":"audit revenue","reference_no":"%s"}`, ref))
	if resp.Code != http.StatusCreated {
		t.Fatalf("revenue create status = %d, want %d", resp.Code, http.StatusCreated)
	}
	if n := countAuditRows(t, env.db, "revenue_records", "CREATE", actor.ID); n < 1 {
		t.Fatalf("revenue create wrote %d audit rows, want >= 1", n)
	}
}

//  13. Files: deleting a file through the real handler writes a
//     file_uploads/DELETE audit row.
func TestAuditWrittenForFileDeletion(t *testing.T) {
	SkipUnlessForceIntegration(t)
	env := newAuditEnv(t)
	defer env.fx.Cleanup()

	actor := makeUser(t, env.db, env.fx, env.roles.StaffID, models.RoleStaff, "filedel")
	// file_uploads are not tracked by the shared fixture; clear by stored name.
	stored := "audit-store-" + actor.ID.String()[:8]
	defer func() {
		_ = env.db.Unscoped().Delete(&models.FileUpload{}, "stored_name = ?", stored).Error
	}()

	fileRepo := repository.NewFileUploadRepository(env.db)
	file := models.FileUpload{
		ID:           uuid.New(),
		UserID:       actor.ID,
		OriginalName: "audit.bin",
		StoredName:   stored,
		Path:         "storage/uploads/" + stored,
		ContentType:  "application/octet-stream",
		Size:         8,
		SHA256:       strings.Repeat("a", 64),
	}
	if err := fileRepo.Create(&file); err != nil {
		t.Fatalf("failed to create test file row: %v", err)
	}

	fileSvc := services.NewFileUploadService(fileRepo)
	handler := handlers.NewFileUploadHandler(fileSvc, env.audit)
	router := routeAs(handler.Delete, &actor, "/files/:id", "DELETE")

	resp := deleteRequest(router, "/files/"+file.ID.String())
	if resp.Code != http.StatusOK {
		t.Fatalf("file delete status = %d, want %d", resp.Code, http.StatusOK)
	}
	if n := countAuditRows(t, env.db, "file_uploads", "DELETE", actor.ID); n < 1 {
		t.Fatalf("file delete wrote %d audit rows, want >= 1", n)
	}
}
