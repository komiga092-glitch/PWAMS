package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// TestPersonObjectLevelAuthorization verifies the service-layer IDOR guard on
// Person endpoints: owners and privileged roles may read/update/delete, while
// unprivileged cross-users are refused with ErrRecordAccessDenied and cause
// no mutation.
func TestPersonObjectLevelAuthorization(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	personRepo := repository.NewPersonRepository(db)
	personSvc := services.NewPersonService(personRepo)

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

	// Donor is a non-privileged role: it must NOT access another user's
	// Person record (the real IDOR class).
	donor := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "intruder")
	beneficiary := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "intruder")

	person := makePerson(t, db, fx, owner.ID, "owner")

	// Authorized owner can read.
	if _, err := personSvc.GetPersonByID(person.ID.String(), services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != nil {
		t.Fatalf("owner read: unexpected error: %v", err)
	}

	// Privileged Admin can read.
	if _, err := personSvc.GetPersonByID(person.ID.String(), services.Actor{ID: admin.ID, Role: models.RoleAdmin}); err != nil {
		t.Fatalf("admin read: unexpected error: %v", err)
	}

	// Privileged Manager (Partner) can read.
	if _, err := personSvc.GetPersonByID(person.ID.String(), services.Actor{ID: manager.ID, Role: models.RoleManager}); err != nil {
		t.Fatalf("manager read: unexpected error: %v", err)
	}

	// Cross-user non-privileged Donor is denied.
	if _, err := personSvc.GetPersonByID(person.ID.String(), services.Actor{ID: donor.ID, Role: models.RoleDonor}); err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user Donor read: expected ErrRecordAccessDenied, got: %v", err)
	}

	// Cross-user non-privileged Beneficiary is denied.
	if _, err := personSvc.GetPersonByID(person.ID.String(), services.Actor{ID: beneficiary.ID, Role: models.RoleBeneficiary}); err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user Beneficiary read: expected ErrRecordAccessDenied, got: %v", err)
	}

	// Nil/unknown actor is denied.
	if _, err := personSvc.GetPersonByID(person.ID.String(), services.Actor{}); err != services.ErrRecordAccessDenied {
		t.Fatalf("nil actor read: expected ErrRecordAccessDenied, got: %v", err)
	}

	// Invalid record ID is rejected with ErrInvalidPersonID.
	if _, err := personSvc.GetPersonByID("not-a-uuid", services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != services.ErrInvalidPersonID {
		t.Fatalf("invalid id read: expected ErrInvalidPersonID, got: %v", err)
	}

	// Unknown record ID is rejected with ErrPersonNotFound.
	if _, err := personSvc.GetPersonByID(uuid.NewString(), services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != repository.ErrPersonNotFound {
		t.Fatalf("unknown id read: expected ErrPersonNotFound, got: %v", err)
	}

	// Cross-user Donor cannot update — no mutation occurs.
	origFullName := person.FullName
	origStatus := person.Status
	_, err := personSvc.UpdatePerson(person.ID.String(), models.UpdatePersonRequest{
		FullName:    "Hacked",
		NICPassport: person.NICPassport,
		Phone:       "+94771234567",
		Status:      models.PersonStatusActive,
	}, services.Actor{ID: donor.ID, Role: models.RoleDonor})
	if err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user update: expected ErrRecordAccessDenied, got: %v", err)
	}

	// Verify no mutation occurred.
	after, findErr := personRepo.FindByID(person.ID.String())
	if findErr != nil {
		t.Fatalf("verify no mutation: %v", findErr)
	}
	if after.FullName != origFullName || after.Status != origStatus {
		t.Fatalf("cross-user update mutated state")
	}

	// Cross-user Donor cannot update status — no mutation.
	if err := personSvc.UpdatePersonStatus(person.ID.String(), models.PersonStatusInactive, services.Actor{ID: donor.ID, Role: models.RoleDonor}); err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user status update: expected ErrRecordAccessDenied, got: %v", err)
	}
	after2, _ := personRepo.FindByID(person.ID.String())
	if after2.Status != origStatus {
		t.Fatalf("cross-user status update mutated state")
	}

	// Cross-user Donor cannot delete — no mutation.
	if err := personSvc.DeletePerson(person.ID.String(), services.Actor{ID: donor.ID, Role: models.RoleDonor}); err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user delete: expected ErrRecordAccessDenied, got: %v", err)
	}
	if _, findErr := personRepo.FindByID(person.ID.String()); findErr != nil {
		t.Fatalf("cross-user delete removed the record: %v", findErr)
	}

	// Owner can update.
	if _, err := personSvc.UpdatePerson(person.ID.String(), models.UpdatePersonRequest{
		FullName:    "Owner Updated",
		NICPassport: person.NICPassport,
		Phone:       "+94771234567",
		Status:      models.PersonStatusActive,
	}, services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != nil {
		t.Fatalf("owner update: unexpected error: %v", err)
	}
}
