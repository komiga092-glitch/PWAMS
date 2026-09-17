package services_test

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// testFixture tracks records created during an integration test so they can be
// removed afterward, keeping the shared database clean between runs.
type testFixture struct {
	db         *gorm.DB
	users      []uuid.UUID
	persons    []uuid.UUID
	students   []uuid.UUID
	donations  []uuid.UUID
	donors     []uuid.UUID
	loans      []uuid.UUID
	repayments []uuid.UUID
	cleanup    sync.Once
}

func newFixture(db *gorm.DB) *testFixture { return &testFixture{db: db} }

func (f *testFixture) addUser(id uuid.UUID)      { f.users = append(f.users, id) }
func (f *testFixture) addPerson(id uuid.UUID)    { f.persons = append(f.persons, id) }
func (f *testFixture) addStudent(id uuid.UUID)   { f.students = append(f.students, id) }
func (f *testFixture) addDonation(id uuid.UUID)  { f.donations = append(f.donations, id) }
func (f *testFixture) addDonor(id uuid.UUID)     { f.donors = append(f.donors, id) }
func (f *testFixture) addLoan(id uuid.UUID)      { f.loans = append(f.loans, id) }
func (f *testFixture) addRepayment(id uuid.UUID) { f.repayments = append(f.repayments, id) }

func (f *testFixture) Cleanup() {
	f.cleanup.Do(func() {
		for _, id := range f.repayments {
			_ = f.db.Unscoped().Delete(&models.LoanRepayment{}, "id = ?", id).Error
		}
		for _, id := range f.loans {
			_ = f.db.Unscoped().Delete(&models.Loan{}, "id = ?", id).Error
		}
		for _, id := range f.donations {
			_ = f.db.Unscoped().Delete(&models.Donation{}, "id = ?", id).Error
		}
		for _, id := range f.students {
			_ = f.db.Unscoped().Delete(&models.Student{}, "id = ?", id).Error
		}
		for _, id := range f.donors {
			_ = f.db.Unscoped().Delete(&models.Donor{}, "id = ?", id).Error
		}
		for _, id := range f.persons {
			_ = f.db.Unscoped().Delete(&models.Person{}, "id = ?", id).Error
		}
		for _, id := range f.users {
			_ = f.db.Unscoped().Delete(&models.User{}, "id = ?", id).Error
		}
	})
}

type seedRoles struct {
	AdminID       uuid.UUID
	SuperAdminID  uuid.UUID
	StaffID       uuid.UUID
	PartnerID     uuid.UUID
	DonorID       uuid.UUID
	BeneficiaryID uuid.UUID
	StudentID     uuid.UUID
}

// existingPartnerID returns the ID of the single Partner/Manager user
// the business rules permit (enforced by uq_users_one_manager), or
// uuid.Nil if none exists yet. Tests reuse it instead of trying to
// create a second Partner, which would violate the unique index.
func existingPartnerID(db *gorm.DB) uuid.UUID {
	var userID string
	err := db.
		Model(&models.User{}).
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("roles.name = ?", models.RoleManager).
		Where("users.deleted_at IS NULL").
		Select("users.id").
		Take(&userID).Error
	if err != nil {
		return uuid.Nil
	}
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return uuid.Nil
	}
	return parsed
}

func loadSeedRoles(t *testing.T, db *gorm.DB) seedRoles {
	t.Helper()
	var roles []models.Role
	if err := db.Find(&roles).Error; err != nil {
		t.Fatalf("failed to load roles: %v", err)
	}

	var out seedRoles
	for _, r := range roles {
		switch r.Name {
		case models.RoleAdmin:
			out.AdminID = r.ID
		case models.RoleSuperAdmin:
			out.SuperAdminID = r.ID
		case models.RoleStaff:
			out.StaffID = r.ID
		case models.RoleManager:
			out.PartnerID = r.ID
		case models.RoleDonor:
			out.DonorID = r.ID
		case models.RoleBeneficiary:
			out.BeneficiaryID = r.ID
		case models.RoleStudent:
			out.StudentID = r.ID
		}
	}

	return out
}

func makeUser(t *testing.T, db *gorm.DB, fx *testFixture, roleID uuid.UUID, roleName, suffix string) models.User {
	t.Helper()
	id := uuid.New()
	username := "authz_" + roleName + "_" + suffix + "_" + id.String()[:8]
	email := username + "@pwams.test"
	user := models.User{
		ID:           id,
		Username:     username,
		Email:        email,
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKJKLMNOPQR",
		RoleID:       roleID,
		Role:         models.Role{ID: roleID, Name: roleName},
		Status:       models.UserStatusActive,
	}

	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	fx.addUser(id)
	return user
}

func makePerson(t *testing.T, db *gorm.DB, fx *testFixture, createdBy uuid.UUID, nicSuffix string) models.Person {
	t.Helper()
	id := uuid.New()
	person := models.Person{
		ID:          id,
		FullName:    "Authz Person " + nicSuffix,
		NICPassport: "NIC-" + nicSuffix + "-" + id.String()[:8],
		Phone:       "+94771234567",
		Email:       "person_" + id.String()[:8] + "@pwams.test",
		CreatedByID: createdBy,
		Status:      models.PersonStatusActive,
	}

	if err := db.Create(&person).Error; err != nil {
		t.Fatalf("failed to create test person: %v", err)
	}

	fx.addPerson(id)
	return person
}

func makeDonorActive(t *testing.T, db *gorm.DB, fx *testFixture, createdBy uuid.UUID, suffix string) models.Donor {
	t.Helper()
	id := uuid.New()
	donor := models.Donor{
		ID:          id,
		Name:        "Donor " + suffix,
		DonorType:   models.DonorTypeIndividual,
		Phone:       "+94771234567",
		Status:      models.DonorStatusActive,
		CreatedByID: createdBy,
	}

	if err := db.Create(&donor).Error; err != nil {
		t.Fatalf("failed to create test donor: %v", err)
	}

	fx.addDonor(id)
	return donor
}

func makeDonation(t *testing.T, db *gorm.DB, fx *testFixture, createdBy, donorID uuid.UUID) models.Donation {
	t.Helper()
	id := uuid.New()
	donation := models.Donation{
		ID:           id,
		DonorID:      donorID,
		DonationType: models.DonationTypeCash,
		Amount:       decimal.NewFromInt(1000),
		Currency:     "LKR",
		DonationDate: time.Now().UTC(),
		ReferenceNo:  "REF-" + id.String()[:8],
		Status:       models.DonationStatusPending,
		CreatedByID:  createdBy,
	}

	if err := db.Create(&donation).Error; err != nil {
		t.Fatalf("failed to create test donation: %v", err)
	}

	fx.addDonation(id)
	return donation
}

func makeStudent(t *testing.T, db *gorm.DB, fx *testFixture, createdBy, personID uuid.UUID, codeSuffix string) models.Student {
	t.Helper()
	id := uuid.New()
	student := models.Student{
		ID:            id,
		PersonID:      personID,
		FullName:      "Student " + codeSuffix,
		SchoolName:    "School " + codeSuffix,
		Grade:         "5",
		StudentCode:   "SC-" + codeSuffix + "-" + id.String()[:8],
		GuardianName:  "Guardian " + codeSuffix,
		GuardianPhone: "+94771234567",
		AcademicYear:  time.Now().Year(),
		Status:        models.StudentStatusActive,
		CreatedByID:   createdBy,
	}

	if err := db.Create(&student).Error; err != nil {
		t.Fatalf("failed to create test student: %v", err)
	}

	fx.addStudent(id)
	return student
}

func makeLoan(t *testing.T, db *gorm.DB, fx *testFixture, createdBy, personID uuid.UUID, purposeSuffix string) models.Loan {
	t.Helper()
	id := uuid.New()
	loan := models.Loan{
		ID:                id,
		PersonID:          personID,
		LoanAmount:        decimal.NewFromInt(12000),
		InterestRate:      decimal.Zero,
		DurationMonths:    12,
		InstallmentAmount: decimal.NewFromInt(1000),
		Status:            models.LoanStatusPending,
		Purpose:           "authz-loan-" + purposeSuffix + "-" + id.String()[:8],
		CreatedByID:       createdBy,
	}

	if err := db.Create(&loan).Error; err != nil {
		t.Fatalf("failed to create test loan: %v", err)
	}

	fx.addLoan(id)
	return loan
}

func makeRepayment(t *testing.T, db *gorm.DB, fx *testFixture, loanID uuid.UUID, installment int) models.LoanRepayment {
	t.Helper()
	id := uuid.New()
	repayment := models.LoanRepayment{
		ID:                id,
		LoanID:            loanID,
		InstallmentNumber: installment,
		DueDate:           time.Now().UTC().AddDate(0, 1, 0),
		Amount:            decimal.NewFromInt(1000),
		PaidAmount:        decimal.Zero,
		Status:            models.RepaymentStatusPending,
	}

	if err := db.Create(&repayment).Error; err != nil {
		t.Fatalf("failed to create test repayment: %v", err)
	}

	fx.addRepayment(id)
	return repayment
}
