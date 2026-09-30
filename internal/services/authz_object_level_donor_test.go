package services_test

import (
	"errors"
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// Phase 2 §11 object-level authorization (live database).
//
// A Donor may read its OWN donation history, but must never read, mutate or
// delete another user's donation. The rule is enforced in the service layer
// (ownership.go / CanAccessRecord) below the HTTP RBAC middleware, so it holds
// even if a route is misconfigured.
func TestObjectLevel_DonorSeesOwnDonationButNotAnotherUsers(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)

	staff := makeUser(t, db, fx, roles.StaffID, models.RoleStaff, "donation-owner")
	donorUser := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "selfdonor")

	staffDonorRecord := makeDonorActive(t, db, fx, staff.ID, "objlevel")
	ownDonorRecord := makeDonorActive(t, db, fx, staff.ID, "objlevel-own")

	// One donation owned by the Staff member, one owned by the Donor user.
	foreignDonation := makeDonation(t, db, fx, staff.ID, staffDonorRecord.ID)
	ownDonation := makeDonation(t, db, fx, donorUser.ID, ownDonorRecord.ID)

	svc := services.NewDonationService(
		repository.NewDonationRepository(db),
		repository.NewDonorRepository(db),
		repository.NewPersonRepository(db),
	)

	donorActor := services.Actor{ID: donorUser.ID, Role: models.RoleDonor}

	// Own data: allowed.
	got, err := svc.GetDonationByID(ownDonation.ID.String(), donorActor)
	if err != nil {
		t.Fatalf("Donor must be able to read its own donation, got: %v", err)
	}
	if got.ID != ownDonation.ID {
		t.Fatalf("returned donation %s, want %s", got.ID, ownDonation.ID)
	}

	// Another user's donation: denied with no data leakage.
	if _, err := svc.GetDonationByID(foreignDonation.ID.String(), donorActor); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("Donor reading another user's donation: err = %v, want ErrRecordAccessDenied", err)
	}

	// Mutations are denied too.
	if err := svc.DeleteDonation(foreignDonation.ID.String(), donorActor); !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("Donor deleting another user's donation: err = %v, want ErrRecordAccessDenied", err)
	}

	// The list is scoped to the Donor's own records.
	donations, _, _, _, err := svc.ListDonations(models.DonationListQuery{Page: 1, PageSize: 50}, donorActor)
	if err != nil {
		t.Fatalf("Donor listing donations failed: %v", err)
	}

	foundOwn := false
	for _, donation := range donations {
		if donation.ID == foreignDonation.ID {
			t.Fatal("Donor list leaked another user's donation")
		}
		if donation.ID == ownDonation.ID {
			foundOwn = true
		}
	}
	if !foundOwn {
		t.Fatal("Donor list must include the Donor's own donation")
	}
}

// The same fail-closed contract for an unknown/anonymous actor: no identity
// means no records, never unrestricted access.
func TestObjectLevel_UnknownActorCannotListDonations(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)

	svc := services.NewDonationService(
		repository.NewDonationRepository(db),
		repository.NewDonorRepository(db),
		repository.NewPersonRepository(db),
	)

	_, _, _, _, err := svc.ListDonations(models.DonationListQuery{Page: 1, PageSize: 10}, services.Actor{})
	if !errors.Is(err, services.ErrRecordAccessDenied) {
		t.Fatalf("anonymous actor listing donations: err = %v, want ErrRecordAccessDenied", err)
	}
}