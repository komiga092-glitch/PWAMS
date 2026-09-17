package services_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// newLoanLifecyclePerson creates an active person using a dedicated Staff
// creator so the CreatedByID foreign key is satisfied.
func newLoanLifecyclePerson(t *testing.T, db *gorm.DB, suffix string) *models.Person {
	t.Helper()

	var staffRole models.Role
	if err := db.Where("name = ?", models.RoleStaff).First(&staffRole).Error; err != nil {
		t.Fatalf("resolve Staff role: %v", err)
	}
	creator := &models.User{
		Username:     "llc-" + suffix,
		Email:        "llc-" + suffix + "@pwams.local",
		FullName:     "LL Creator " + suffix,
		RoleID:       staffRole.ID,
		Status:       models.UserStatusActive,
		PasswordHash: "$2a$10$dummyhashdummyhashdummyhashdummyhashdummyhashdummyhashdum",
	}
	if err := db.Create(creator).Error; err != nil {
		t.Fatalf("create ll-creator: %v", err)
	}

	personRepo := repository.NewPersonRepository(db)
	svc := services.NewPersonService(personRepo)

	nic := "LL" + suffix
	if len(nic) > 20 {
		nic = nic[:20]
	}
	person, err := svc.CreatePerson(models.CreatePersonRequest{
		FullName:      "LL Person " + suffix,
		NICPassport:   nic,
		Gender:        "Male",
		Phone:         "0771234567",
		Email:         "ll-" + suffix + "@pwams.local",
		Address:       "123 LL Street",
		Occupation:    "Tester",
		MonthlyIncome: decimal.NewFromInt(50000),
	}, creator.ID)
	if err != nil {
		t.Fatalf("create ll-person: %v", err)
	}
	return person
}

func newLoanService(t *testing.T, db *gorm.DB) (*services.LoanService, *services.LoanRepaymentService) {
	t.Helper()
	loanRepo := repository.NewLoanRepository(db)
	repaymentRepo := repository.NewLoanRepaymentRepository(db)
	personRepo := repository.NewPersonRepository(db)
	notif := services.NewNotificationService(repository.NewNotificationRepository(db))
	loanSvc := services.NewLoanServiceWithNotifications(loanRepo, repaymentRepo, personRepo, db, notif)
	repaySvc := services.NewLoanRepaymentServiceWithPerson(repaymentRepo, loanRepo, personRepo, db)
	return loanSvc, repaySvc
}

func reloadLoan(t *testing.T, db *gorm.DB, id uuid.UUID) models.Loan {
	t.Helper()
	var loan models.Loan
	if err := db.First(&loan, "id = ?", id).Error; err != nil {
		t.Fatalf("reload loan: %v", err)
	}
	return loan
}

func sumScheduleInvariants(t *testing.T, db *gorm.DB, loanID uuid.UUID, wantPrincipal, wantInterest, wantRepayable decimal.Decimal) {
	t.Helper()
	var rows []models.LoanRepayment
	if err := db.Where("loan_id = ? AND is_deleted = FALSE", loanID).Order("installment_number ASC").Find(&rows).Error; err != nil {
		t.Fatalf("load schedule: %v", err)
	}
	var sumAmount, sumPrincipal, sumInterest decimal.Decimal
	for i := range rows {
		sumAmount = sumAmount.Add(rows[i].Amount)
		sumPrincipal = sumPrincipal.Add(rows[i].PrincipalAmount)
		sumInterest = sumInterest.Add(rows[i].InterestAmount)
	}
	if !sumPrincipal.Equal(wantPrincipal) {
		t.Errorf("SUM(principal) = %s, want %s", sumPrincipal.String(), wantPrincipal.String())
	}
	if !sumInterest.Equal(wantInterest) {
		t.Errorf("SUM(interest) = %s, want %s", sumInterest.String(), wantInterest.String())
	}
	if !sumAmount.Equal(wantRepayable) {
		t.Errorf("SUM(installment amounts) = %s, want %s", sumAmount.String(), wantRepayable.String())
	}
}

func newTestCreator(t *testing.T, db *gorm.DB, suffix string) *models.User {
	t.Helper()
	var staffRole models.Role
	if err := db.Where("name = ?", models.RoleStaff).First(&staffRole).Error; err != nil {
		t.Fatalf("resolve Staff role: %v", err)
	}
	creator := &models.User{
		Username:     "creator-" + suffix,
		Email:        "creator-" + suffix + "@pwams.local",
		FullName:     "Creator",
		RoleID:       staffRole.ID,
		Status:       models.UserStatusActive,
		PasswordHash: "$2a$10$dummyhashdummyhashdummyhashdummyhashdummyhashdummyhashdum",
	}
	if err := db.Create(creator).Error; err != nil {
		t.Fatalf("create creator: %v", err)
	}
	return creator
}

func createActiveLoan(t *testing.T, db *gorm.DB, loanSvc *services.LoanService, personID uuid.UUID, creatorID uuid.UUID) models.Loan {
	t.Helper()
	loan, err := loanSvc.CreateLoan(models.CreateLoanRequest{
		PersonID:       personID.String(),
		LoanAmount:     decimal.NewFromInt(12000),
		InterestRate:   decimal.NewFromInt(12),
		DurationMonths: 3,
		StartDate:      "2026-09-01",
		DueDay:         1,
		Purpose:        "Lifecycle test",
	}, creatorID)
	if err != nil {
		t.Fatalf("CreateLoan: %v", err)
	}
	for _, st := range []string{models.LoanStatusApproved, models.LoanStatusActive} {
		if _, err := loanSvc.ReviewLoan(loan.ID.String(), models.ReviewLoanRequest{Status: st}, creatorID); err != nil {
			t.Fatalf("ReviewLoan to %s: %v", st, err)
		}
	}
	return reloadLoan(t, db, loan.ID)
}

func TestLoanFinancialPlan_FlatInterest(t *testing.T) {
	tests := []struct {
		name          string
		amount        decimal.Decimal
		interestRate  decimal.Decimal
		months        int
		wantInterest  decimal.Decimal
		wantRepayable decimal.Decimal
		wantMonthly   decimal.Decimal
	}{
		{
			name:          "no interest",
			amount:        decimal.NewFromInt(12000),
			interestRate:  decimal.Zero,
			months:        12,
			wantInterest:  decimal.Zero,
			wantRepayable: decimal.NewFromInt(12000),
			wantMonthly:   decimal.NewFromInt(1000),
		},
		{
			name:          "10% annualized, 10 months",
			amount:        decimal.NewFromInt(10000),
			interestRate:  decimal.NewFromInt(10),
			months:        10,
			wantInterest:  decimal.RequireFromString("833.33"),
			wantRepayable: decimal.RequireFromString("10833.33"),
			wantMonthly:   decimal.RequireFromString("1083.33"),
		},
		{
			name:          "5% annualized, 6 months",
			amount:        decimal.NewFromInt(6000),
			interestRate:  decimal.NewFromInt(5),
			months:        6,
			wantInterest:  decimal.NewFromInt(150),
			wantRepayable: decimal.NewFromInt(6150),
			wantMonthly:   decimal.NewFromInt(1025),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := services.CalculateLoanFinancialPlan(tt.amount, tt.interestRate, tt.months)
			if !plan.TotalInterest.Equal(tt.wantInterest) {
				t.Errorf("TotalInterest = %s, want %s", plan.TotalInterest.String(), tt.wantInterest.String())
			}
			if !plan.TotalRepayable.Equal(tt.wantRepayable) {
				t.Errorf("TotalRepayable = %s, want %s", plan.TotalRepayable.String(), tt.wantRepayable.String())
			}
			if !plan.MonthlyInstallment.Equal(tt.wantMonthly) {
				t.Errorf("MonthlyInstallment = %s, want %s", plan.MonthlyInstallment.String(), tt.wantMonthly.String())
			}
		})
	}
}

func TestGenerateLoanSchedule_Invariants(t *testing.T) {
	amount := decimal.NewFromInt(100000)
	rate := decimal.NewFromInt(12)
	months := 3
	startDate := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	plan := services.CalculateLoanFinancialPlan(amount, rate, months)
	schedule := services.GenerateLoanSchedule(amount, rate, months, startDate, 0)

	if len(schedule) != months {
		t.Fatalf("schedule length = %d, want %d", len(schedule), months)
	}
	var sumAmount, sumPrincipal, sumInterest decimal.Decimal
	for i := range schedule {
		sumAmount = sumAmount.Add(schedule[i].TotalAmount)
		sumPrincipal = sumPrincipal.Add(schedule[i].PrincipalAmount)
		sumInterest = sumInterest.Add(schedule[i].InterestAmount)
		if schedule[i].InstallmentNumber != i+1 {
			t.Errorf("installment %d number = %d, want %d", i, schedule[i].InstallmentNumber, i+1)
		}
	}
	if !sumPrincipal.Equal(amount) {
		t.Errorf("SUM(principal) = %s, want %s", sumPrincipal.String(), amount.String())
	}
	if !sumInterest.Equal(plan.TotalInterest) {
		t.Errorf("SUM(interest) = %s, want %s", sumInterest.String(), plan.TotalInterest.String())
	}
	if !sumAmount.Equal(plan.TotalRepayable) {
		t.Errorf("SUM(amounts) = %s, want %s", sumAmount.String(), plan.TotalRepayable.String())
	}
}

func TestGenerateLoanSchedule_RoundingRemainder(t *testing.T) {
	amount := decimal.NewFromInt(90000)
	rate := decimal.NewFromInt(12)
	months := 3
	startDate := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	plan := services.CalculateLoanFinancialPlan(amount, rate, months)
	schedule := services.GenerateLoanSchedule(amount, rate, months, startDate, 0)
	var sumAmount decimal.Decimal
	for i := range schedule {
		sumAmount = sumAmount.Add(schedule[i].TotalAmount)
	}
	if !sumAmount.Equal(plan.TotalRepayable) {
		t.Errorf("SUM = %s, want %s (remainder not absorbed)", sumAmount.String(), plan.TotalRepayable.String())
	}
}

func TestGenerateLoanSchedule_DueDates(t *testing.T) {
	schedule := services.GenerateLoanSchedule(decimal.NewFromInt(12000), decimal.Zero, 12,
		time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), 0)
	expected := []time.Time{
		time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	for i, exp := range expected {
		if i >= len(schedule) {
			break
		}
		if !schedule[i].DueDate.Equal(exp) {
			t.Errorf("installment %d due = %v, want %v", i+1, schedule[i].DueDate, exp)
		}
	}
}

func TestGenerateLoanSchedule_MonthEndClamping(t *testing.T) {
	schedule := services.GenerateLoanSchedule(decimal.NewFromInt(3000), decimal.Zero, 3,
		time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), 31)
	if len(schedule) != 3 {
		t.Fatalf("schedule length = %d, want 3", len(schedule))
	}
	if !schedule[1].DueDate.Equal(time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Feb due = %v, want 2026-02-28", schedule[1].DueDate)
	}
	if !schedule[2].DueDate.Equal(time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Mar due = %v, want 2026-03-31", schedule[2].DueDate)
	}
}

func TestLoanCreation_GeneratesSchedule(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newLoanLifecyclePerson(t, db, suffix)
	creator := newTestCreator(t, db, suffix)
	loanSvc, _ := newLoanService(t, db)

	loan, err := loanSvc.CreateLoan(models.CreateLoanRequest{
		PersonID:       person.ID.String(),
		LoanAmount:     decimal.NewFromInt(12000),
		InterestRate:   decimal.NewFromInt(12),
		DurationMonths: 12,
		StartDate:      "2026-09-01",
		DueDay:         1,
		Purpose:        "Test loan",
	}, creator.ID)
	if err != nil {
		t.Fatalf("CreateLoan: %v", err)
	}
	if loan.Status != models.LoanStatusPending {
		t.Errorf("status = %q, want Pending", loan.Status)
	}
	if loan.TotalRepayableAmount.LessThanOrEqual(decimal.Zero) {
		t.Errorf("TotalRepayableAmount = %s, want > 0", loan.TotalRepayableAmount.String())
	}
	if !loan.OutstandingAmount.Equal(loan.TotalRepayableAmount) {
		t.Errorf("Outstanding = %s, want %s", loan.OutstandingAmount.String(), loan.TotalRepayableAmount.String())
	}
	if loan.TotalPaidAmount.Sign() != 0 {
		t.Errorf("TotalPaidAmount = %s, want 0", loan.TotalPaidAmount.String())
	}
	wantInterest := decimal.NewFromInt(1440)
	wantRepayable := decimal.NewFromInt(13440)
	if !loan.TotalInterest.Equal(wantInterest) {
		t.Errorf("TotalInterest = %s, want %s", loan.TotalInterest.String(), wantInterest.String())
	}
	if !loan.TotalRepayableAmount.Equal(wantRepayable) {
		t.Errorf("TotalRepayableAmount = %s, want %s", loan.TotalRepayableAmount.String(), wantRepayable.String())
	}
	var count int64
	if err := db.Model(&models.LoanRepayment{}).Where("loan_id = ? AND is_deleted = FALSE", loan.ID).Count(&count).Error; err != nil {
		t.Fatalf("count schedule: %v", err)
	}
	if count != 12 {
		t.Errorf("schedule rows = %d, want 12", count)
	}
	sumScheduleInvariants(t, db, loan.ID, decimal.NewFromInt(12000), wantInterest, wantRepayable)
}

func TestRepayment_FirstSecondFinal(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newLoanLifecyclePerson(t, db, suffix)
	creator := newTestCreator(t, db, suffix)
	loanSvc, repaySvc := newLoanService(t, db)
	loan := createActiveLoan(t, db, loanSvc, person.ID, creator.ID)

	wantRepayable := decimal.NewFromInt(12360)
	wantMonthly := decimal.NewFromInt(4120)
	var installments []models.LoanRepayment
	if err := db.Where("loan_id = ? AND is_deleted = FALSE", loan.ID).Order("installment_number ASC").Find(&installments).Error; err != nil {
		t.Fatalf("load installments: %v", err)
	}
	if len(installments) != 3 {
		t.Fatalf("installments = %d, want 3", len(installments))
	}
	payerID := creator.ID

	if _, err := repaySvc.Pay(installments[0].ID.String(), models.PayLoanRepaymentRequest{PaidAmount: wantMonthly, PaymentMethod: "Cash", UpdatedBy: &payerID}); err != nil {
		t.Fatalf("first Pay: %v", err)
	}
	afterFirst := reloadLoan(t, db, loan.ID)
	if !afterFirst.TotalPaidAmount.Equal(wantMonthly) {
		t.Errorf("after first: TotalPaidAmount = %s, want %s", afterFirst.TotalPaidAmount.String(), wantMonthly.String())
	}
	if !afterFirst.OutstandingAmount.Equal(wantRepayable.Sub(wantMonthly)) {
		t.Errorf("after first: Outstanding = %s, want %s", afterFirst.OutstandingAmount.String(), wantRepayable.Sub(wantMonthly).String())
	}
	if afterFirst.Status != models.LoanStatusActive {
		t.Errorf("after first: status = %q, want Active", afterFirst.Status)
	}

	if _, err := repaySvc.Pay(installments[1].ID.String(), models.PayLoanRepaymentRequest{PaidAmount: wantMonthly, UpdatedBy: &payerID}); err != nil {
		t.Fatalf("second Pay: %v", err)
	}
	afterSecond := reloadLoan(t, db, loan.ID)
	if !afterSecond.TotalPaidAmount.Equal(wantMonthly.Mul(decimal.NewFromInt(2))) {
		t.Errorf("after second: TotalPaidAmount = %s, want %s", afterSecond.TotalPaidAmount.String(), wantMonthly.Mul(decimal.NewFromInt(2)).String())
	}

	if _, err := repaySvc.Pay(installments[2].ID.String(), models.PayLoanRepaymentRequest{PaidAmount: wantMonthly, UpdatedBy: &payerID}); err != nil {
		t.Fatalf("final Pay: %v", err)
	}
	afterFinal := reloadLoan(t, db, loan.ID)
	if afterFinal.Status != models.LoanStatusCompleted {
		t.Errorf("after final: status = %q, want Completed", afterFinal.Status)
	}
	if afterFinal.OutstandingAmount.Sign() != 0 {
		t.Errorf("after final: Outstanding = %s, want 0", afterFinal.OutstandingAmount.String())
	}
	if !afterFinal.TotalPaidAmount.Equal(wantRepayable) {
		t.Errorf("after final: TotalPaidAmount = %s, want %s", afterFinal.TotalPaidAmount.String(), wantRepayable.String())
	}
	if afterFinal.CompletedAt == nil {
		t.Error("after final: CompletedAt nil, want set")
	}
}

func TestRepayment_PartialPayment(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newLoanLifecyclePerson(t, db, suffix)
	creator := newTestCreator(t, db, suffix)
	loanSvc, repaySvc := newLoanService(t, db)
	loan := createActiveLoan(t, db, loanSvc, person.ID, creator.ID)

	var inst models.LoanRepayment
	if err := db.Where("loan_id = ? AND is_deleted = FALSE", loan.ID).Order("installment_number ASC").First(&inst).Error; err != nil {
		t.Fatalf("load installment: %v", err)
	}
	payerID := creator.ID
	half := decimal.NewFromInt(2060)

	if _, err := repaySvc.Pay(inst.ID.String(), models.PayLoanRepaymentRequest{PaidAmount: half, UpdatedBy: &payerID}); err != nil {
		t.Fatalf("partial Pay: %v", err)
	}
	var afterPartial models.LoanRepayment
	if err := db.First(&afterPartial, "id = ?", inst.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if afterPartial.Status != models.RepaymentStatusPartiallyPaid {
		t.Errorf("after partial: status = %q, want Partially Paid", afterPartial.Status)
	}
	if !afterPartial.PaidAmount.Equal(half) {
		t.Errorf("after partial: PaidAmount = %s, want %s", afterPartial.PaidAmount.String(), half.String())
	}
	remain := decimal.NewFromInt(4120).Sub(half)
	if !afterPartial.OutstandingAmount.Equal(remain) {
		t.Errorf("after partial: Outstanding = %s, want %s", afterPartial.OutstandingAmount.String(), remain.String())
	}

	if _, err := repaySvc.Pay(inst.ID.String(), models.PayLoanRepaymentRequest{PaidAmount: remain, UpdatedBy: &payerID}); err != nil {
		t.Fatalf("final partial Pay: %v", err)
	}
	var afterFull models.LoanRepayment
	if err := db.First(&afterFull, "id = ?", inst.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if afterFull.Status != models.RepaymentStatusPaid {
		t.Errorf("after full: status = %q, want Paid", afterFull.Status)
	}
	if afterFull.OutstandingAmount.Sign() != 0 {
		t.Errorf("after full: Outstanding = %s, want 0", afterFull.OutstandingAmount.String())
	}
}

func TestRepayment_OverpaymentRejected(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newLoanLifecyclePerson(t, db, suffix)
	creator := newTestCreator(t, db, suffix)
	loanSvc, repaySvc := newLoanService(t, db)
	loan := createActiveLoan(t, db, loanSvc, person.ID, creator.ID)

	var inst models.LoanRepayment
	if err := db.Where("loan_id = ? AND is_deleted = FALSE", loan.ID).Order("installment_number ASC").First(&inst).Error; err != nil {
		t.Fatalf("load installment: %v", err)
	}
	payerID := creator.ID
	_, err := repaySvc.Pay(inst.ID.String(), models.PayLoanRepaymentRequest{PaidAmount: decimal.NewFromInt(99999), UpdatedBy: &payerID})
	if !errors.Is(err, services.ErrRepaymentAmountTooHigh) {
		t.Fatalf("overpayment: err = %v, want ErrRepaymentAmountTooHigh", err)
	}
}

func TestRepayment_ZeroAndNegativeRejected(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newLoanLifecyclePerson(t, db, suffix)
	creator := newTestCreator(t, db, suffix)
	loanSvc, repaySvc := newLoanService(t, db)
	loan := createActiveLoan(t, db, loanSvc, person.ID, creator.ID)

	var inst models.LoanRepayment
	if err := db.Where("loan_id = ? AND is_deleted = FALSE", loan.ID).Order("installment_number ASC").First(&inst).Error; err != nil {
		t.Fatalf("load installment: %v", err)
	}
	payerID := creator.ID
	if _, err := repaySvc.Pay(inst.ID.String(), models.PayLoanRepaymentRequest{PaidAmount: decimal.Zero, UpdatedBy: &payerID}); !errors.Is(err, services.ErrInvalidRepaymentAmount) {
		t.Errorf("zero payment: err = %v, want ErrInvalidRepaymentAmount", err)
	}
	if _, err := repaySvc.Pay(inst.ID.String(), models.PayLoanRepaymentRequest{PaidAmount: decimal.NewFromInt(-100), UpdatedBy: &payerID}); !errors.Is(err, services.ErrInvalidRepaymentAmount) {
		t.Errorf("negative payment: err = %v, want ErrInvalidRepaymentAmount", err)
	}
}

func TestRepayment_AlreadyPaidRejected(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newLoanLifecyclePerson(t, db, suffix)
	creator := newTestCreator(t, db, suffix)
	loanSvc, repaySvc := newLoanService(t, db)
	loan := createActiveLoan(t, db, loanSvc, person.ID, creator.ID)

	var inst models.LoanRepayment
	if err := db.Where("loan_id = ? AND is_deleted = FALSE", loan.ID).Order("installment_number ASC").First(&inst).Error; err != nil {
		t.Fatalf("load installment: %v", err)
	}
	payerID := creator.ID
	full := decimal.NewFromInt(4120)
	if _, err := repaySvc.Pay(inst.ID.String(), models.PayLoanRepaymentRequest{PaidAmount: full, UpdatedBy: &payerID}); err != nil {
		t.Fatalf("first Pay: %v", err)
	}
	if _, err := repaySvc.Pay(inst.ID.String(), models.PayLoanRepaymentRequest{PaidAmount: full, UpdatedBy: &payerID}); !errors.Is(err, services.ErrRepaymentAlreadyPaid) {
		t.Errorf("already-paid: err = %v, want ErrRepaymentAlreadyPaid", err)
	}
}

func TestRepayment_ConcurrentSameInstallment(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newLoanLifecyclePerson(t, db, suffix)
	creator := newTestCreator(t, db, suffix)
	loanSvc, repaySvc := newLoanService(t, db)
	loan := createActiveLoan(t, db, loanSvc, person.ID, creator.ID)

	var inst models.LoanRepayment
	if err := db.Where("loan_id = ? AND is_deleted = FALSE", loan.ID).Order("installment_number ASC").First(&inst).Error; err != nil {
		t.Fatalf("load installment: %v", err)
	}
	payerID := creator.ID
	full := decimal.NewFromInt(4120)

	const n = 10
	var wg sync.WaitGroup
	results := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repaySvc.Pay(inst.ID.String(), models.PayLoanRepaymentRequest{PaidAmount: full, UpdatedBy: &payerID})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, services.ErrRepaymentModified), errors.Is(err, services.ErrRepaymentAlreadyPaid):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent: successes = %d, want exactly 1", successes)
	}
	if successes+conflicts != n {
		t.Fatalf("concurrent: successes=%d conflicts=%d, want total %d", successes, conflicts, n)
	}
	var after models.LoanRepayment
	if err := db.First(&after, "id = ?", inst.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.Status != models.RepaymentStatusPaid {
		t.Errorf("after concurrent: status = %q, want Paid", after.Status)
	}
	if !after.PaidAmount.Equal(full) {
		t.Errorf("after concurrent: PaidAmount = %s, want %s (no double deduction)", after.PaidAmount.String(), full.String())
	}
}

func TestLoanReport_InterestAwareOutstanding(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newLoanLifecyclePerson(t, db, suffix)
	creator := newTestCreator(t, db, suffix)
	loanSvc, _ := newLoanService(t, db)
	createActiveLoan(t, db, loanSvc, person.ID, creator.ID)

	reportRepo := repository.NewReportRepository(db)
	report, err := reportRepo.GetLoanReportList(models.LoanReportQuery{})
	if err != nil {
		t.Fatalf("GetLoanReportList: %v", err)
	}
	if report.TotalInterest < 360 {
		t.Errorf("TotalInterest = %f, want >= 360", report.TotalInterest)
	}
	if report.TotalRepayable < 12360 {
		t.Errorf("TotalRepayable = %f, want >= 12360", report.TotalRepayable)
	}
	if len(report.Rows) == 0 {
		t.Fatal("report has no rows")
	}
	if report.Rows[0].Outstanding < 0 {
		t.Errorf("outstanding = %f, want >= 0", report.Rows[0].Outstanding)
	}
}
