package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// TestStudentObjectLevelAuthorization verifies the service-layer IDOR guard
// on Student endpoints: owners (the creating principal) and privileged roles
// may read/update/delete, while unprivileged cross-users (Donor,
// Beneficiary, Student) are refused with ErrRecordAccessDenied and cause
// no mutation.
func TestStudentObjectLevelAuthorization(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	studentRepo := repository.NewStudentRepository(db)
	personRepo := repository.NewPersonRepository(db)
	studentSvc := services.NewStudentService(studentRepo, personRepo)

	owner := makeUser(t, db, fx, roles.StaffID, models.RoleStaff, "owner")
	admin := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "admin")

	// The business rules permit at most one Partner/Manager account,
	// enforced by the uq_users_one_manager unique index. Reuse the
	// existing Partner when one is present.
	var manager models.User
	if partnerID := existingPartnerID(db); partnerID != uuid.Nil {
		if err := db.Preload("Role").First(&manager, "id = ?", partnerID).Error; err != nil {
			t.Fatalf("failed to load existing Partner: %v", err)
		}
	} else {
		manager = makeUser(t, db, fx, roles.PartnerID, models.RoleManager, "manager")
	}

	donor := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "intruder")
	beneficiary := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "intruder")
	studentUser := makeUser(t, db, fx, roles.StudentID, models.RoleStudent, "intruder")

	person := makePerson(t, db, fx, owner.ID, "owner")
	student := makeStudent(t, db, fx, owner.ID, person.ID, "owner")

	// Authorized owner can read.
	if _, err := studentSvc.GetStudentByID(student.ID.String(), services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != nil {
		t.Fatalf("owner read: unexpected error: %v", err)
	}

	// Privileged Admin can read.
	if _, err := studentSvc.GetStudentByID(student.ID.String(), services.Actor{ID: admin.ID, Role: models.RoleAdmin}); err != nil {
		t.Fatalf("admin read: unexpected error: %v", err)
	}

	// Privileged Manager (Partner) can read.
	if _, err := studentSvc.GetStudentByID(student.ID.String(), services.Actor{ID: manager.ID, Role: models.RoleManager}); err != nil {
		t.Fatalf("manager read: unexpected error: %v", err)
	}

	// Cross-user Donor is denied.
	if _, err := studentSvc.GetStudentByID(student.ID.String(), services.Actor{ID: donor.ID, Role: models.RoleDonor}); err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user Donor read: expected ErrRecordAccessDenied, got: %v", err)
	}

	// Cross-user Beneficiary is denied.
	if _, err := studentSvc.GetStudentByID(student.ID.String(), services.Actor{ID: beneficiary.ID, Role: models.RoleBeneficiary}); err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user Beneficiary read: expected ErrRecordAccessDenied, got: %v", err)
	}

	// Cross-user Student is denied.
	if _, err := studentSvc.GetStudentByID(student.ID.String(), services.Actor{ID: studentUser.ID, Role: models.RoleStudent}); err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user Student read: expected ErrRecordAccessDenied, got: %v", err)
	}

	// Nil/unknown actor is denied.
	if _, err := studentSvc.GetStudentByID(student.ID.String(), services.Actor{}); err != services.ErrRecordAccessDenied {
		t.Fatalf("nil actor read: expected ErrRecordAccessDenied, got: %v", err)
	}

	// Invalid record ID is rejected with ErrInvalidStudentID.
	if _, err := studentSvc.GetStudentByID("not-a-uuid", services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != services.ErrInvalidStudentID {
		t.Fatalf("invalid id read: expected ErrInvalidStudentID, got: %v", err)
	}

	// Unknown record ID is rejected with ErrStudentNotFound.
	if _, err := studentSvc.GetStudentByID(uuid.NewString(), services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != repository.ErrStudentNotFound {
		t.Fatalf("unknown id read: expected ErrStudentNotFound, got: %v", err)
	}

	// Cross-user cannot update; state must not mutate.
	before := student.Status
	if _, err := studentSvc.UpdateStudent(student.ID.String(), models.UpdateStudentRequest{
		PersonID:      person.ID.String(),
		FullName:      "Hacked Name",
		SchoolName:    student.SchoolName,
		Grade:         student.Grade,
		StudentCode:   student.StudentCode,
		GuardianPhone: "+94771234567",
		AcademicYear:  student.AcademicYear,
		Status:        models.StudentStatusActive,
	}, services.Actor{ID: donor.ID, Role: models.RoleDonor}); err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user update: expected ErrRecordAccessDenied, got: %v", err)
	}

	reloaded, err := studentRepo.FindByID(student.ID.String())
	if err != nil {
		t.Fatalf("reload student: %v", err)
	}
	if reloaded.Status != before || reloaded.FullName == "Hacked Name" {
		t.Fatalf("state mutated after denied update: status=%q name=%q", reloaded.Status, reloaded.FullName)
	}

	// Cross-user cannot delete; record must still exist.
	if err := studentSvc.DeleteStudent(student.ID.String(), services.Actor{ID: beneficiary.ID, Role: models.RoleBeneficiary}); err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user delete: expected ErrRecordAccessDenied, got: %v", err)
	}

	if _, err := studentRepo.FindByID(student.ID.String()); err != nil {
		t.Fatalf("record missing after denied delete: %v", err)
	}

	// Owner can update.
	updated, err := studentSvc.UpdateStudent(student.ID.String(), models.UpdateStudentRequest{
		PersonID:      person.ID.String(),
		FullName:      "Legit Update",
		SchoolName:    student.SchoolName,
		Grade:         student.Grade,
		StudentCode:   student.StudentCode,
		GuardianPhone: "+94771234567",
		AcademicYear:  student.AcademicYear,
		Status:        models.StudentStatusActive,
	}, services.Actor{ID: owner.ID, Role: models.RoleStaff})
	if err != nil {
		t.Fatalf("owner update: unexpected error: %v", err)
	}
	if updated.FullName != "Legit Update" {
		t.Fatalf("owner update: expected FullName 'Legit Update', got %q", updated.FullName)
	}

	// Owner can delete.
	if err := studentSvc.DeleteStudent(student.ID.String(), services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != nil {
		t.Fatalf("owner delete: unexpected error: %v", err)
	}
}
