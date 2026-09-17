package services_test

import (
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// TestDonationListObjectLevelAuthorization verifies that ListDonations
// is scoped to the authenticated actor's ownership unless the actor
// holds a privileged role. The owner is a non-privileged Donor so exact
// counts hold regardless of pre-existing rows in the shared DB.
func TestDonationListObjectLevelAuthorization(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	donationRepo := repository.NewDonationRepository(db)
	donorRepo := repository.NewDonorRepository(db)
	personRepo := repository.NewPersonRepository(db)
	donationSvc := services.NewDonationService(donationRepo, donorRepo, personRepo)

	owner := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "owner")
	donorIntruder := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "intruder")
	beneficiaryIntruder := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "intruder")
	studentIntruder := makeUser(t, db, fx, roles.StudentID, models.RoleStudent, "intruder")
	admin := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "admin")

	_, adminBaseline, _, _, err := donationSvc.ListDonations(
		models.DonationListQuery{Page: 1, PageSize: 1},
		services.Actor{ID: admin.ID, Role: models.RoleAdmin},
	)
	if err != nil {
		t.Fatalf("admin baseline: unexpected error: %v", err)
	}

	donorOwned := makeDonorActive(t, db, fx, owner.ID, "owner")
	donorIntruderAccount := makeDonorActive(t, db, fx, donorIntruder.ID, "intruder")

	donationOwned1 := makeDonation(t, db, fx, owner.ID, donorOwned.ID)
	makeDonation(t, db, fx, owner.ID, donorOwned.ID)
	makeDonation(t, db, fx, donorIntruder.ID, donorIntruderAccount.ID)

	ownerActor := services.Actor{ID: owner.ID, Role: models.RoleDonor}
	donorActor := services.Actor{ID: donorIntruder.ID, Role: models.RoleDonor}
	beneficiaryActor := services.Actor{ID: beneficiaryIntruder.ID, Role: models.RoleBeneficiary}
	studentActor := services.Actor{ID: studentIntruder.ID, Role: models.RoleStudent}
	adminActor := services.Actor{ID: admin.ID, Role: models.RoleAdmin}

	// 1. Owner sees own donations (exactly 2), never another user's.
	donations, total, _, _, err := donationSvc.ListDonations(
		models.DonationListQuery{Page: 1, PageSize: 50}, ownerActor)
	if err != nil {
		t.Fatalf("owner list: unexpected error: %v", err)
	}
	if total != 2 || len(donations) != 2 {
		t.Fatalf("owner list: expected 2 rows/total 2, got rows=%d total=%d", len(donations), total)
	}
	for _, d := range donations {
		if d.CreatedByID != owner.ID {
			t.Fatalf("owner list: returned record not owned by owner: created_by=%s", d.CreatedByID)
		}
	}

	// 2. Another Donor cannot enumerate the owner's donations; it sees
	//    only its own record.
	donations, total, _, _, err = donationSvc.ListDonations(
		models.DonationListQuery{Page: 1, PageSize: 50}, donorActor)
	if err != nil {
		t.Fatalf("donor intruder list: unexpected error: %v", err)
	}
	if total != 1 || len(donations) != 1 {
		t.Fatalf("donor intruder list: expected 1 row/total 1 (own only), got rows=%d total=%d", len(donations), total)
	}
	if donations[0].ID == donationOwned1.ID || donations[0].CreatedByID == owner.ID {
		t.Fatalf("donor intruder list: exposed another user's donation")
	}

	// 3. Beneficiary cannot enumerate the owner's donations.
	donations, total, _, _, err = donationSvc.ListDonations(
		models.DonationListQuery{Page: 1, PageSize: 50}, beneficiaryActor)
	if err != nil {
		t.Fatalf("beneficiary list: unexpected error: %v", err)
	}
	if total != 0 || len(donations) != 0 {
		t.Fatalf("beneficiary list: expected 0 results, got rows=%d total=%d (authorization bypass)", len(donations), total)
	}

	// 4. Student cannot enumerate the owner's donations.
	donations, total, _, _, err = donationSvc.ListDonations(
		models.DonationListQuery{Page: 1, PageSize: 50}, studentActor)
	if err != nil {
		t.Fatalf("student list: unexpected error: %v", err)
	}
	if total != 0 || len(donations) != 0 {
		t.Fatalf("student list: expected 0 results, got rows=%d total=%d (authorization bypass)", len(donations), total)
	}

	// 5. Privileged Admin retains access (delta over baseline).
	donations, total, _, _, err = donationSvc.ListDonations(
		models.DonationListQuery{Page: 1, PageSize: 50}, adminActor)
	if err != nil {
		t.Fatalf("admin list: unexpected error: %v", err)
	}
	if total != adminBaseline+3 {
		t.Fatalf("admin list: expected total %d (baseline %d + 3), got %d", adminBaseline+3, adminBaseline, total)
	}
	adminHas := false
	for _, d := range donations {
		if d.ID == donationOwned1.ID {
			adminHas = true
		}
	}
	if !adminHas {
		t.Fatalf("admin list: created donation not visible to privileged role")
	}

	// 6. Pagination is correctly scoped for the owner.
	donations, total, page, pageSize, err := donationSvc.ListDonations(
		models.DonationListQuery{Page: 1, PageSize: 1}, ownerActor)
	if err != nil {
		t.Fatalf("owner pagination: unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("owner pagination: expected total 2, got %d (count leak)", total)
	}
	if len(donations) != 1 || page != 1 || pageSize != 1 {
		t.Fatalf("owner pagination: expected 1 row on page 1/1, got rows=%d page=%d size=%d", len(donations), page, pageSize)
	}
	if donations[0].CreatedByID != owner.ID {
		t.Fatalf("owner pagination: unauthorized row leaked")
	}

	// 7. Count is correctly scoped for actors with no authorized rows.
	for _, actor := range []services.Actor{beneficiaryActor, studentActor} {
		_, t2, _, _, err := donationSvc.ListDonations(
			models.DonationListQuery{Page: 1, PageSize: 10}, actor)
		if err != nil {
			t.Fatalf("scoped count (%s): unexpected error: %v", actor.Role, err)
		}
		if t2 != 0 {
			t.Fatalf("scoped count (%s): expected total 0, got %d (count leak)", actor.Role, t2)
		}
	}

	// 8. Search cannot bypass ownership: the owner's unique reference
	//    yields nothing for Beneficiary or Student actors.
	donations, total, _, _, err = donationSvc.ListDonations(
		models.DonationListQuery{Search: donationOwned1.ReferenceNo, Page: 1, PageSize: 50}, beneficiaryActor)
	if err != nil {
		t.Fatalf("beneficiary search: unexpected error: %v", err)
	}
	if total != 0 || len(donations) != 0 {
		t.Fatalf("beneficiary search: expected 0 results, got rows=%d total=%d (search bypass)", len(donations), total)
	}

	// 9. Search still works for the owner.
	donations, total, _, _, err = donationSvc.ListDonations(
		models.DonationListQuery{Search: donationOwned1.ReferenceNo, Page: 1, PageSize: 50}, ownerActor)
	if err != nil {
		t.Fatalf("owner search: unexpected error: %v", err)
	}
	if total != 1 || len(donations) != 1 {
		t.Fatalf("owner search: expected 1 result, got rows=%d total=%d", len(donations), total)
	}

	// 10. Type and status filters cannot bypass ownership.
	donations, total, _, _, err = donationSvc.ListDonations(
		models.DonationListQuery{Type: models.DonationTypeCash, Page: 1, PageSize: 50}, studentActor)
	if err != nil {
		t.Fatalf("student type filter: unexpected error: %v", err)
	}
	if total != 0 || len(donations) != 0 {
		t.Fatalf("student type filter: expected 0 results, got rows=%d total=%d (filter bypass)", len(donations), total)
	}
	donations, total, _, _, err = donationSvc.ListDonations(
		models.DonationListQuery{Status: models.DonationStatusPending, Page: 1, PageSize: 50}, beneficiaryActor)
	if err != nil {
		t.Fatalf("beneficiary status filter: unexpected error: %v", err)
	}
	if total != 0 || len(donations) != 0 {
		t.Fatalf("beneficiary status filter: expected 0 results, got rows=%d total=%d (filter bypass)", len(donations), total)
	}

	// 11. Sort cannot bypass ownership: ordering is fixed server-side
	//     (created_at DESC), so sweeping all pages yields owned rows only.
	for p := 1; p <= 2; p++ {
		rows, t2, _, _, err := donationSvc.ListDonations(
			models.DonationListQuery{Page: p, PageSize: 1}, ownerActor)
		if err != nil {
			t.Fatalf("owner sort sweep page %d: unexpected error: %v", p, err)
		}
		if t2 != 2 {
			t.Fatalf("owner sort sweep page %d: expected total 2, got %d", p, t2)
		}
		for _, row := range rows {
			if row.CreatedByID != owner.ID {
				t.Fatalf("owner sort sweep page %d: unauthorized row leaked", p)
			}
		}
	}

	// 12. Object-level ID checks: another Donor, Beneficiary and
	//     Student cannot read the owner's donation by ID.
	if _, err := donationSvc.GetDonationByID(donationOwned1.ID.String(), ownerActor); err != nil {
		t.Fatalf("owner get: unexpected error: %v", err)
	}
	for _, actor := range []services.Actor{donorActor, beneficiaryActor, studentActor} {
		if _, err := donationSvc.GetDonationByID(donationOwned1.ID.String(), actor); err != services.ErrRecordAccessDenied {
			t.Fatalf("%s get: expected ErrRecordAccessDenied, got: %v", actor.Role, err)
		}
	}
}
