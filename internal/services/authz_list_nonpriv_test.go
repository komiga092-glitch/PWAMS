package services_test

import (
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// TestListAuthorization_NonPrivilegedRolesAreScoped verifies that
// non-privileged roles (Donor, Beneficiary, Student) are correctly
// scoped to their own records and cannot enumerate others' records.
func TestListAuthorization_NonPrivilegedRolesAreScoped(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	personRepo := repository.NewPersonRepository(db)
	personSvc := services.NewPersonService(personRepo)

	// Staff owner creates persons.
	owner := makeUser(t, db, fx, roles.StaffID, models.RoleStaff, "owner")
	makePerson(t, db, fx, owner.ID, "owner1")
	makePerson(t, db, fx, owner.ID, "owner2")

	// Non-privileged users.
	donor := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "donor")
	beneficiary := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "beneficiary")
	student := makeUser(t, db, fx, roles.StudentID, models.RoleStudent, "student")

	nonPrivilegedActors := []services.Actor{
		{ID: donor.ID, Role: models.RoleDonor},
		{ID: beneficiary.ID, Role: models.RoleBeneficiary},
		{ID: student.ID, Role: models.RoleStudent},
	}

	for _, actor := range nonPrivilegedActors {
		persons, total, _, _, err := personSvc.ListPersons(models.PersonListQuery{Page: 1, PageSize: 50}, actor)
		if err != nil {
			t.Fatalf("non-privileged %s list: unexpected error: %v", actor.Role, err)
		}
		if total != 0 {
			t.Fatalf("non-privileged %s list: expected total 0, got %d (leak)", actor.Role, total)
		}
		if len(persons) != 0 {
			t.Fatalf("non-privileged %s list: expected 0 persons, got %d (bypass)", actor.Role, len(persons))
		}
	}
}

// TestListAuthorization_OwnerSeesOwnRecordsOnly verifies that a
// non-privileged owner sees only their own records, not other users'
// records, even when they own some records.
func TestListAuthorization_OwnerSeesOwnRecordsOnly(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	personRepo := repository.NewPersonRepository(db)
	personSvc := services.NewPersonService(personRepo)

	// Two non-privileged owners each create a person.
	ownerA := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "ownerA")
	ownerB := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "ownerB")

	makePerson(t, db, fx, ownerA.ID, "ownerA")
	makePerson(t, db, fx, ownerB.ID, "ownerB")

	ownerAActor := services.Actor{ID: ownerA.ID, Role: models.RoleDonor}
	ownerBActor := services.Actor{ID: ownerB.ID, Role: models.RoleBeneficiary}

	// Owner A sees only their own record.
	persons, total, _, _, err := personSvc.ListPersons(models.PersonListQuery{Page: 1, PageSize: 50}, ownerAActor)
	if err != nil {
		t.Fatalf("ownerA list: unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("ownerA list: expected total 1, got %d", total)
	}
	if len(persons) != 1 {
		t.Fatalf("ownerA list: expected 1 person, got %d", len(persons))
	}
	if persons[0].CreatedByID != ownerA.ID {
		t.Fatalf("ownerA list: returned record not owned by ownerA: created_by=%s", persons[0].CreatedByID)
	}

	// Owner B sees only their own record.
	persons, total, _, _, err = personSvc.ListPersons(models.PersonListQuery{Page: 1, PageSize: 50}, ownerBActor)
	if err != nil {
		t.Fatalf("ownerB list: unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("ownerB list: expected total 1, got %d", total)
	}
	if len(persons) != 1 {
		t.Fatalf("ownerB list: expected 1 person, got %d", len(persons))
	}
	if persons[0].CreatedByID != ownerB.ID {
		t.Fatalf("ownerB list: returned record not owned by ownerB: created_by=%s", persons[0].CreatedByID)
	}
}
