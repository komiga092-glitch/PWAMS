package services_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// STEP 15.6.3: Loan and Loan Repayment authorization tests.
// Ownership model: Loan.CreatedByID (existing column, unchanged);
// LoanRepayment ownership is inherited through the parent Loan's
// CreatedByID. List scoping is applied at the database query level
// (identical predicate for COUNT and the row query).

func newLoanService(db *gorm.DB) *services.LoanService {
	return services.NewLoanService(
		repository.NewLoanRepository(db),
		repository.NewPersonRepository(db),
	)
}

func newRepaymentService(db *gorm.DB) *services.LoanRepaymentService {
	return services.NewLoanRepaymentService(
		repository.NewLoanRepaymentRepository(db),
		repository.NewLoanRepository(db),
		db,
	)
}

// TestLoanListObjectLevelAuthorization verifies DB-level list scoping,
// search/filter/pagination ownership enforcement and count isolation.
func TestLoanListObjectLevelAuthorization(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	loanSvc := newLoanService(db)

	owner := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "owner")
	beneficiaryIntruder := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "intruder")
	studentIntruder := makeUser(t, db, fx, roles.StudentID, models.RoleStudent, "intruder")
	donorIntruder := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "intruder")
	admin := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "admin")
	staff := makeUser(t, db, fx, roles.StaffID, models.RoleStaff, "staff")

	_, adminBaseline, _, _, err := loanSvc.ListLoans(
		models.LoanListQuery{Page: 1, PageSize: 1},
		services.Actor{ID: admin.ID, Role: models.RoleAdmin},
	)
	if err != nil {
		t.Fatalf("admin baseline: unexpected error: %v", err)
	}

	ownerPerson := makePerson(t, db, fx, owner.ID, "L1")
	intruderPerson := makePerson(t, db, fx, beneficiaryIntruder.ID, "L2")

	loanOwned1 := makeLoan(t, db, fx, owner.ID, ownerPerson.ID, "a")
	makeLoan(t, db, fx, owner.ID, ownerPerson.ID, "b")
	makeLoan(t, db, fx, beneficiaryIntruder.ID, intruderPerson.ID, "c")

	ownerActor := services.Actor{ID: owner.ID, Role: models.RoleBeneficiary}
	beneficiaryActor := services.Actor{ID: beneficiaryIntruder.ID, Role: models.RoleBeneficiary}
	studentActor := services.Actor{ID: studentIntruder.ID, Role: models.RoleStudent}
	donorActor := services.Actor{ID: donorIntruder.ID, Role: models.RoleDonor}
	adminActor := services.Actor{ID: admin.ID, Role: models.RoleAdmin}
	staffActor := services.Actor{ID: staff.ID, Role: models.RoleStaff}

	// 1. Owner sees exactly its own loans (2), never another user's.
	loans, total, _, _, err := loanSvc.ListLoans(
		models.LoanListQuery{Page: 1, PageSize: 50}, ownerActor)
	if err != nil {
		t.Fatalf("owner list: unexpected error: %v", err)
	}
	if total != 2 || len(loans) != 2 {
		t.Fatalf("owner list: expected 2 rows/total 2, got rows=%d total=%d", len(loans), total)
	}
	for _, l := range loans {
		if l.CreatedByID != owner.ID {
			t.Fatalf("owner list: returned loan not owned by owner: created_by=%s", l.CreatedByID)
		}
	}

	// 2. Another Beneficiary sees only its own loan (1).
	loans, total, _, _, err = loanSvc.ListLoans(
		models.LoanListQuery{Page: 1, PageSize: 50}, beneficiaryActor)
	if err != nil {
		t.Fatalf("beneficiary intruder list: unexpected error: %v", err)
	}
	if total != 1 || len(loans) != 1 {
		t.Fatalf("beneficiary intruder list: expected 1 row/total 1, got rows=%d total=%d", len(loans), total)
	}
	if loans[0].ID == loanOwned1.ID || loans[0].CreatedByID == owner.ID {
		t.Fatalf("beneficiary intruder list: exposed another user's loan")
	}

	// 3. Student and Donor with no loans of their own see nothing.
	for _, actor := range []services.Actor{studentActor, donorActor} {
		_, t2, _, _, err := loanSvc.ListLoans(
			models.LoanListQuery{Page: 1, PageSize: 50}, actor)
		if err != nil {
			t.Fatalf("%s list: unexpected error: %v", actor.Role, err)
		}
		if t2 != 0 {
			t.Fatalf("%s list: expected total 0, got %d (leak)", actor.Role, t2)
		}
	}

	// 4. Privileged actors (Admin, Staff) see every loan.
	for _, actor := range []services.Actor{adminActor, staffActor} {
		_, t2, _, _, err := loanSvc.ListLoans(
			models.LoanListQuery{Page: 1, PageSize: 50}, actor)
		if err != nil {
			t.Fatalf("%s list: unexpected error: %v", actor.Role, err)
		}
		if want := adminBaseline + 3; t2 != want {
			t.Fatalf("%s list: expected total %d, got %d", actor.Role, want, t2)
		}
	}

	// 5. nil actor fails closed.
	if _, _, _, _, err := loanSvc.ListLoans(
		models.LoanListQuery{Page: 1, PageSize: 50}, services.Actor{}); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("nil actor list: expected ErrRecordAccessDenied, got %v", err)
	}

	// 6. Pagination cannot bypass ownership: sweeping all pages with
	//    pageSize=1 returns only owned rows at a stable total.
	for p := 1; p <= 2; p++ {
		rows, t2, _, _, err := loanSvc.ListLoans(
			models.LoanListQuery{Page: p, PageSize: 1}, ownerActor)
		if err != nil {
			t.Fatalf("owner pagination page %d: unexpected error: %v", p, err)
		}
		if t2 != 2 {
			t.Fatalf("owner pagination page %d: expected total 2, got %d", p, t2)
		}
		for _, row := range rows {
			if row.CreatedByID != owner.ID {
				t.Fatalf("owner pagination page %d: unauthorized row leaked", p)
			}
		}
	}
	// An out-of-range page yields no rows but a correct total.
	_, t3, _, _, err := loanSvc.ListLoans(
		models.LoanListQuery{Page: 9, PageSize: 1}, ownerActor)
	if err != nil {
		t.Fatalf("owner out-of-range page: unexpected error: %v", err)
	}
	if t3 != 2 {
		t.Fatalf("owner out-of-range page: expected total 2, got %d (count leak)", t3)
	}

	// 7. Search cannot bypass ownership: the owner's unique purpose
	//    token yields nothing for intruder roles.
	for _, actor := range []services.Actor{beneficiaryActor, studentActor, donorActor} {
		rows, t2, _, _, err := loanSvc.ListLoans(
			models.LoanListQuery{Search: loanOwned1.Purpose, Page: 1, PageSize: 50}, actor)
		if err != nil {
			t.Fatalf("%s search: unexpected error: %v", actor.Role, err)
		}
		if t2 != 0 || len(rows) != 0 {
			t.Fatalf("%s search: expected 0 results, got rows=%d total=%d (search bypass)", actor.Role, len(rows), t2)
		}
	}
	// Search still works for the owner.
	rows, t2, _, _, err := loanSvc.ListLoans(
		models.LoanListQuery{Search: loanOwned1.Purpose, Page: 1, PageSize: 50}, ownerActor)
	if err != nil {
		t.Fatalf("owner search: unexpected error: %v", err)
	}
	if t2 != 1 || len(rows) != 1 || rows[0].ID != loanOwned1.ID {
		t.Fatalf("owner search: expected 1 result, got rows=%d total=%d", len(rows), t2)
	}

	// 8. person_id filter cannot bypass ownership.
	for _, actor := range []services.Actor{beneficiaryActor, studentActor} {
		rows, t2, _, _, err := loanSvc.ListLoans(
			models.LoanListQuery{PersonID: ownerPerson.ID.String(), Page: 1, PageSize: 50}, actor)
		if err != nil {
			t.Fatalf("%s person filter: unexpected error: %v", actor.Role, err)
		}
		if t2 != 0 || len(rows) != 0 {
			t.Fatalf("%s person filter: expected 0 results, got rows=%d total=%d (filter bypass)", actor.Role, len(rows), t2)
		}
	}

	// 9. Status filter cannot bypass ownership.
	rows, t2, _, _, err = loanSvc.ListLoans(
		models.LoanListQuery{Status: models.LoanStatusPending, Page: 1, PageSize: 50}, studentActor)
	if err != nil {
		t.Fatalf("student status filter: unexpected error: %v", err)
	}
	if t2 != 0 || len(rows) != 0 {
		t.Fatalf("student status filter: expected 0 results, got rows=%d total=%d (filter bypass)", len(rows), t2)
	}

	// 10. Invalid person_id is rejected safely.
	if _, _, _, _, err := loanSvc.ListLoans(
		models.LoanListQuery{PersonID: "not-a-uuid", Page: 1, PageSize: 10}, adminActor); !errors.Is(err, services.ErrInvalidPersonID) {
		t.Fatalf("invalid person_id: expected ErrInvalidPersonID, got %v", err)
	}
}

// TestLoanRepaymentListObjectLevelAuthorization verifies that repayment
// lists are scoped through the parent loan's owner at the database level.
func TestLoanRepaymentListObjectLevelAuthorization(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	repaymentSvc := newRepaymentService(db)

	owner := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "r_owner")
	beneficiaryIntruder := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "r_intruder")
	studentIntruder := makeUser(t, db, fx, roles.StudentID, models.RoleStudent, "r_intruder")
	admin := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "r_admin")

	_, adminBaseline, _, _, err := repaymentSvc.List(
		models.LoanRepaymentListQuery{Page: 1, PageSize: 1},
		services.Actor{ID: admin.ID, Role: models.RoleAdmin},
	)
	if err != nil {
		t.Fatalf("admin baseline: unexpected error: %v", err)
	}

	ownerPerson := makePerson(t, db, fx, owner.ID, "R1")
	intruderPerson := makePerson(t, db, fx, beneficiaryIntruder.ID, "R2")

	loanOwned := makeLoan(t, db, fx, owner.ID, ownerPerson.ID, "r1")
	loanIntruder := makeLoan(t, db, fx, beneficiaryIntruder.ID, intruderPerson.ID, "r2")

	rep1 := makeRepayment(t, db, fx, loanOwned.ID, 1)
	makeRepayment(t, db, fx, loanOwned.ID, 2)
	makeRepayment(t, db, fx, loanIntruder.ID, 1)

	ownerActor := services.Actor{ID: owner.ID, Role: models.RoleBeneficiary}
	beneficiaryActor := services.Actor{ID: beneficiaryIntruder.ID, Role: models.RoleBeneficiary}
	studentActor := services.Actor{ID: studentIntruder.ID, Role: models.RoleStudent}
	adminActor := services.Actor{ID: admin.ID, Role: models.RoleAdmin}

	// 1. Owner sees exactly its own repayments (2).
	rows, total, _, _, err := repaymentSvc.List(
		models.LoanRepaymentListQuery{Page: 1, PageSize: 50}, ownerActor)
	if err != nil {
		t.Fatalf("owner list: unexpected error: %v", err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("owner list: expected 2 rows/total 2, got rows=%d total=%d", len(rows), total)
	}
	for _, r := range rows {
		if r.LoanID != loanOwned.ID {
			t.Fatalf("owner list: returned repayment of another loan: loan_id=%s", r.LoanID)
		}
	}

	// 2. Another Beneficiary sees only the repayment of its own loan (1),
	//    never the owner's.
	rows, total, _, _, err = repaymentSvc.List(
		models.LoanRepaymentListQuery{Page: 1, PageSize: 50}, beneficiaryActor)
	if err != nil {
		t.Fatalf("beneficiary intruder list: unexpected error: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("beneficiary intruder list: expected 1 row/total 1, got rows=%d total=%d", len(rows), total)
	}
	if rows[0].ID == rep1.ID || rows[0].LoanID == loanOwned.ID {
		t.Fatalf("beneficiary intruder list: exposed another user's repayment")
	}

	// 3. Student with no loans sees nothing.
	_, t2, _, _, err := repaymentSvc.List(
		models.LoanRepaymentListQuery{Page: 1, PageSize: 50}, studentActor)
	if err != nil {
		t.Fatalf("student list: unexpected error: %v", err)
	}
	if t2 != 0 {
		t.Fatalf("student list: expected total 0, got %d (leak)", t2)
	}

	// 4. Admin sees everything.
	_, t2, _, _, err = repaymentSvc.List(
		models.LoanRepaymentListQuery{Page: 1, PageSize: 50}, adminActor)
	if err != nil {
		t.Fatalf("admin list: unexpected error: %v", err)
	}
	if want := adminBaseline + 3; t2 != want {
		t.Fatalf("admin list: expected total %d, got %d", want, t2)
	}

	// 5. nil actor fails closed.
	if _, _, _, _, err := repaymentSvc.List(
		models.LoanRepaymentListQuery{Page: 1, PageSize: 50}, services.Actor{}); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("nil actor list: expected ErrRecordAccessDenied, got %v", err)
	}

	// 6. loan_id filter cannot bypass ownership: the intruder passing
	//    the owner's loan ID gets zero rows and a zero count.
	rows, total, _, _, err = repaymentSvc.List(
		models.LoanRepaymentListQuery{LoanID: loanOwned.ID.String(), Page: 1, PageSize: 50}, beneficiaryActor)
	if err != nil {
		t.Fatalf("intruder loan_id filter: unexpected error: %v", err)
	}
	if total != 0 || len(rows) != 0 {
		t.Fatalf("intruder loan_id filter: expected 0 rows/total 0, got rows=%d total=%d (IDOR bypass)", len(rows), total)
	}
	// The owner still sees its repayments through the same filter.
	rows, total, _, _, err = repaymentSvc.List(
		models.LoanRepaymentListQuery{LoanID: loanOwned.ID.String(), Page: 1, PageSize: 50}, ownerActor)
	if err != nil {
		t.Fatalf("owner loan_id filter: unexpected error: %v", err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("owner loan_id filter: expected 2 rows/total 2, got rows=%d total=%d", len(rows), total)
	}

	// 7. Status filter cannot bypass ownership.
	rows, total, _, _, err = repaymentSvc.List(
		models.LoanRepaymentListQuery{Status: models.RepaymentStatusPending, Page: 1, PageSize: 50}, studentActor)
	if err != nil {
		t.Fatalf("student status filter: unexpected error: %v", err)
	}
	if total != 0 || len(rows) != 0 {
		t.Fatalf("student status filter: expected 0 rows/total 0, got rows=%d total=%d (filter bypass)", len(rows), total)
	}

	// 8. Invalid loan_id and invalid status are rejected safely.
	if _, _, _, _, err := repaymentSvc.List(
		models.LoanRepaymentListQuery{LoanID: "not-a-uuid", Page: 1, PageSize: 10}, adminActor); !errors.Is(err, services.ErrInvalidLoanIDFormat) {
		t.Fatalf("invalid loan_id: expected ErrInvalidLoanIDFormat, got %v", err)
	}
	if _, _, _, _, err := repaymentSvc.List(
		models.LoanRepaymentListQuery{Status: "Bogus", Page: 1, PageSize: 10}, adminActor); !errors.Is(err, services.ErrInvalidRepaymentStatus) {
		t.Fatalf("invalid status: expected ErrInvalidRepaymentStatus, got %v", err)
	}
}

// TestLoanSingleRecordAuthorization verifies object-level access on
// Get/Review paths, including nil-actor, invalid-UUID and unknown-ID
// fail-safe behavior.
func TestLoanSingleRecordAuthorization(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	loanSvc := newLoanService(db)

	owner := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "s_owner")
	beneficiaryIntruder := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "s_intruder")
	studentIntruder := makeUser(t, db, fx, roles.StudentID, models.RoleStudent, "s_intruder")
	donorIntruder := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "s_intruder")
	admin := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "s_admin")
	staff := makeUser(t, db, fx, roles.StaffID, models.RoleStaff, "s_staff")
	partnerID := existingPartnerID(db)

	ownerPerson := makePerson(t, db, fx, owner.ID, "S1")
	loan := makeLoan(t, db, fx, owner.ID, ownerPerson.ID, "s")

	ownerActor := services.Actor{ID: owner.ID, Role: models.RoleBeneficiary}
	adminActor := services.Actor{ID: admin.ID, Role: models.RoleAdmin}
	staffActor := services.Actor{ID: staff.ID, Role: models.RoleStaff}

	// A. Owner may read its own loan.
	if _, err := loanSvc.GetLoanByID(loan.ID.String(), ownerActor); err != nil {
		t.Fatalf("owner get: unexpected error: %v", err)
	}

	// B. Privileged roles may read another principal's loan.
	privileged := []services.Actor{adminActor, staffActor}
	// The business rules permit only one Partner (Manager) user
	// (uq_users_one_manager); reuse the existing one when present.
	if partnerID != uuid.Nil {
		privileged = append(privileged, services.Actor{ID: partnerID, Role: models.RoleManager})
	} else {
		partner := makeUser(t, db, fx, roles.PartnerID, models.RoleManager, "s_partner")
		privileged = append(privileged, services.Actor{ID: partner.ID, Role: models.RoleManager})
	}
	// Super Admin policy (synthetic actor; role governs the decision).
	privileged = append(privileged,
		services.Actor{ID: admin.ID, Role: models.RoleSuperAdmin},
	)
	for _, actor := range privileged {
		if _, err := loanSvc.GetLoanByID(loan.ID.String(), actor); err != nil {
			t.Fatalf("privileged %s get: unexpected error: %v", actor.Role, err)
		}
	}

	// C/E/F/D. Non-owner same-role and cross-role principals denied.
	for _, actor := range []services.Actor{
		{ID: beneficiaryIntruder.ID, Role: models.RoleBeneficiary},
		{ID: studentIntruder.ID, Role: models.RoleStudent},
		{ID: donorIntruder.ID, Role: models.RoleDonor},
	} {
		if _, err := loanSvc.GetLoanByID(loan.ID.String(), actor); !errors.Is(err, services.ErrRecordAccessDenied) {
			t.Fatalf("%s get: expected ErrRecordAccessDenied, got %v", actor.Role, err)
		}
	}

	// H. nil actor denied.
	if _, err := loanSvc.GetLoanByID(loan.ID.String(), services.Actor{}); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("nil actor get: expected ErrRecordAccessDenied, got %v", err)
	}

	// I. Invalid UUID is rejected safely (no panic, 400-class error).
	if _, err := loanSvc.GetLoanByID("not-a-uuid", ownerActor); !errors.Is(err, services.ErrInvalidLoanID) {
		t.Fatalf("invalid uuid: expected ErrInvalidLoanID, got %v", err)
	}

	// J. Unknown record ID maps to ErrLoanNotFound (404-class).
	if _, err := loanSvc.GetLoanByID(uuid.New().String(), ownerActor); !errors.Is(err, repository.ErrLoanNotFound) {
		t.Fatalf("unknown id: expected ErrLoanNotFound, got %v", err)
	}

	// Review authorization: a non-privileged actor cannot move another
	// principal's loan through the status machine.
	if _, err := loanSvc.ReviewLoan(loan.ID.String(), models.ReviewLoanRequest{
		Status: models.LoanStatusRejected,
	}, beneficiaryIntruder.ID, services.Actor{
		ID:   beneficiaryIntruder.ID,
		Role: models.RoleBeneficiary,
	}); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("intruder review: expected ErrRecordAccessDenied, got %v", err)
	}
}

// TestLoanRepaymentMutationSafety verifies that unauthorized repayment
// operations (Get/Pay/Cancel/Create) cause no database change at all.
func TestLoanRepaymentMutationSafety(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	repaymentSvc := newRepaymentService(db)
	loanSvc := newLoanService(db)

	owner := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "m_owner")
	intruder := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "m_intruder")

	ownerPerson := makePerson(t, db, fx, owner.ID, "M1")

	loan := makeLoan(t, db, fx, owner.ID, ownerPerson.ID, "m")
	repayment := makeRepayment(t, db, fx, loan.ID, 1)

	intruderActor := services.Actor{ID: intruder.ID, Role: models.RoleBeneficiary}
	ownerActor := services.Actor{ID: owner.ID, Role: models.RoleBeneficiary}

	// 1. Get: intruder cannot view the repayment (IDOR denied).
	if _, err := repaymentSvc.GetByID(repayment.ID.String(), intruderActor); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("intruder repayment get: expected ErrRecordAccessDenied, got %v", err)
	}
	// Owner can view.
	if _, err := repaymentSvc.GetByID(repayment.ID.String(), ownerActor); err != nil {
		t.Fatalf("owner repayment get: unexpected error: %v", err)
	}

	// 2. Pay: intruder cannot record a payment; no DB change occurs.
	before := loanAmountsOf(t, db, repayment.ID)
	if _, err := repaymentSvc.Pay(repayment.ID.String(), models.PayLoanRepaymentRequest{
		PaidAmount: decimalOf(100),
	}, intruderActor); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("intruder pay: expected ErrRecordAccessDenied, got %v", err)
	}
	after := loanAmountsOf(t, db, repayment.ID)
	if before != after {
		t.Fatalf("intruder pay mutated the repayment: before=%+v after=%+v", before, after)
	}

	// 3. Cancel: intruder cannot cancel; status unchanged.
	if err := repaymentSvc.Cancel(repayment.ID.String(), intruderActor); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("intruder cancel: expected ErrRecordAccessDenied, got %v", err)
	}
	if got := loanAmountsOf(t, db, repayment.ID); got != after {
		t.Fatalf("intruder cancel mutated the repayment: before=%+v after=%+v", after, got)
	}

	// 4. Create: intruder cannot attach repayments to the owner's loan.
	if _, err := repaymentSvc.Create(models.CreateLoanRepaymentRequest{
		LoanID:            loan.ID.String(),
		InstallmentNumber: 2,
		DueDate:           "2030-01-01",
		Amount:            decimalOf(500),
	}, intruderActor); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("intruder create: expected ErrRecordAccessDenied, got %v", err)
	}
	var count int64
	if err := db.Model(&models.LoanRepayment{}).
		Where("loan_id = ?", loan.ID).Count(&count).Error; err != nil {
		t.Fatalf("count repayments: %v", err)
	}
	if count != 1 {
		t.Fatalf("intruder create: expected 1 repayment for the loan, got %d (unauthorized insert)", count)
	}

	// 5. Invalid and unknown identifiers fail safe.
	if _, err := repaymentSvc.GetByID("not-a-uuid", ownerActor); !errors.Is(err, services.ErrInvalidLoanRepaymentID) {
		t.Fatalf("invalid repayment uuid: expected ErrInvalidLoanRepaymentID, got %v", err)
	}
	if _, err := repaymentSvc.GetByID(uuid.New().String(), ownerActor); !errors.Is(err, repository.ErrLoanRepaymentNotFound) {
		t.Fatalf("unknown repayment id: expected ErrLoanRepaymentNotFound, got %v", err)
	}
	if err := repaymentSvc.Cancel("not-a-uuid", ownerActor); !errors.Is(err, services.ErrInvalidLoanRepaymentID) {
		t.Fatalf("invalid cancel uuid: expected ErrInvalidLoanRepaymentID, got %v", err)
	}

	// 6. Unauthorized loan mutation (review) leaves status/version intact.
	loanBefore := loanRowOf(t, db, loan.ID)
	if _, err := loanSvc.ReviewLoan(loan.ID.String(), models.ReviewLoanRequest{
		Status: models.LoanStatusRejected,
	}, intruder.ID, intruderActor); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("intruder review: expected ErrRecordAccessDenied, got %v", err)
	}
	loanAfter := loanRowOf(t, db, loan.ID)
	if loanBefore.Status != loanAfter.Status || loanBefore.Version != loanAfter.Version {
		t.Fatalf("unauthorized review mutated the loan: before=%+v after=%+v", loanBefore, loanAfter)
	}
}

// decimalOf builds a decimal from an integer amount.
func decimalOf(value int64) decimal.Decimal {
	return decimal.NewFromInt(value)
}

// loanAmountsOf snapshots the payment-relevant state of a repayment.
func loanAmountsOf(t *testing.T, db *gorm.DB, repaymentID uuid.UUID) repaymentSnapshot {
	t.Helper()
	var row struct {
		Status   string
		PaidAmnt float64
		Version  int
	}
	if err := db.Model(&models.LoanRepayment{}).
		Select("status", "paid_amount AS paid_amnt", "version").
		Where("id = ?", repaymentID).
		Take(&row).Error; err != nil {
		t.Fatalf("load repayment snapshot: %v", err)
	}
	return repaymentSnapshot{Status: row.Status, PaidAmount: row.PaidAmnt, Version: row.Version}
}

type repaymentSnapshot struct {
	Status     string
	PaidAmount float64
	Version    int
}

// loanRowOf snapshots the mutation-relevant state of a loan.
func loanRowOf(t *testing.T, db *gorm.DB, loanID uuid.UUID) loanSnapshot {
	t.Helper()
	var row struct {
		Status  string
		Version int
	}
	if err := db.Model(&models.Loan{}).
		Select("status", "version").
		Where("id = ?", loanID).
		Take(&row).Error; err != nil {
		t.Fatalf("load loan snapshot: %v", err)
	}
	return loanSnapshot{Status: row.Status, Version: row.Version}
}

type loanSnapshot struct {
	Status  string
	Version int
}

// TestLoanListFailClosedWithoutDB proves the nil-actor denial happens in
// the service layer before any repository access (no DB needed).
func TestLoanListFailClosedWithoutDB(t *testing.T) {
	loanSvc := services.NewLoanService(nil, nil)
	if _, _, _, _, err := loanSvc.ListLoans(
		models.LoanListQuery{Page: 1, PageSize: 10}, services.Actor{}); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("nil actor ListLoans: expected ErrRecordAccessDenied, got %v", err)
	}

	repaymentSvc := services.NewLoanRepaymentService(nil, nil, nil)
	if _, _, _, _, err := repaymentSvc.List(
		models.LoanRepaymentListQuery{Page: 1, PageSize: 10}, services.Actor{}); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("nil actor List: expected ErrRecordAccessDenied, got %v", err)
	}
}
