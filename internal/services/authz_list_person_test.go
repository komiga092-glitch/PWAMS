package services_test

import (
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// TestPersonListObjectLevelAuthorization verifies that ListPersons is
// scoped to the authenticated actor's ownership unless the actor holds
// a privileged role. Owner and intruders are non-privileged roles so
// exact assertions hold regardless of pre-existing rows.
func TestPersonListObjectLevelAuthorization(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	personRepo := repository.NewPersonRepository(db)
	personSvc := services.NewPersonService(personRepo)

	owner := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "owner")
	intruder := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "intruder")
	emptyUser := makeUser(t, db, fx, roles.StudentID, models.RoleStudent, "empty")
	admin := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "admin")

	_, adminBaseline, _, _, err := personSvc.ListPersons(
		models.PersonListQuery{Page: 1, PageSize: 1},
		services.Actor{ID: admin.ID, Role: models.RoleAdmin},
	)
	if err != nil {
		t.Fatalf("admin baseline: unexpected error: %v", err)
	}

	personOwned1 := makePerson(t, db, fx, owner.ID, "owner1")
	makePerson(t, db, fx, owner.ID, "owner2")
	makePerson(t, db, fx, intruder.ID, "intruder1")

	ownerActor := services.Actor{ID: owner.ID, Role: models.RoleDonor}
	intruderActor := services.Actor{ID: intruder.ID, Role: models.RoleBeneficiary}
	emptyActor := services.Actor{ID: emptyUser.ID, Role: models.RoleStudent}
	adminActor := services.Actor{ID: admin.ID, Role: models.RoleAdmin}

	// 1. Owner sees own records (exactly 2), never another user's.
	persons, total, _, _, err := personSvc.ListPersons(
		models.PersonListQuery{Page: 1, PageSize: 50}, ownerActor)
	if err != nil {
		t.Fatalf("owner list: unexpected error: %v", err)
	}
	if total != 2 || len(persons) != 2 {
		t.Fatalf("owner list: expected 2 rows/total 2, got rows=%d total=%d", len(persons), total)
	}
	for _, p := range persons {
		if p.CreatedByID != owner.ID {
			t.Fatalf("owner list: returned record not owned by owner: created_by=%s", p.CreatedByID)
		}
	}

	// 2. Non-owner can only see its own record, never the owner's.
	persons, total, _, _, err = personSvc.ListPersons(
		models.PersonListQuery{Page: 1, PageSize: 50}, intruderActor)
	if err != nil {
		t.Fatalf("intruder list: unexpected error: %v", err)
	}
	if total != 1 || len(persons) != 1 {
		t.Fatalf("intruder list: expected 1 row/total 1 (own only), got rows=%d total=%d", len(persons), total)
	}
	if persons[0].ID == personOwned1.ID || persons[0].CreatedByID == owner.ID {
		t.Fatalf("intruder list: exposed another user's record")
	}

	// 3. Empty result when no authorized records exist.
	persons, total, _, _, err = personSvc.ListPersons(
		models.PersonListQuery{Page: 1, PageSize: 50}, emptyActor)
	if err != nil {
		t.Fatalf("empty list: unexpected error: %v", err)
	}
	if total != 0 || len(persons) != 0 {
		t.Fatalf("empty list: expected 0 results, got rows=%d total=%d (authorization bypass)", len(persons), total)
	}

	// 4. Privileged Admin sees all records (delta over baseline).
	persons, total, _, _, err = personSvc.ListPersons(
		models.PersonListQuery{Page: 1, PageSize: 50}, adminActor)
	if err != nil {
		t.Fatalf("admin list: unexpected error: %v", err)
	}
	if total != adminBaseline+3 {
		t.Fatalf("admin list: expected total %d (baseline %d + 3), got %d", adminBaseline+3, adminBaseline, total)
	}
	foundOwned := false
	for _, p := range persons {
		if p.ID == personOwned1.ID {
			foundOwned = true
		}
	}
	if !foundOwned {
		t.Fatalf("admin list: created record not visible to privileged role")
	}

	// 5. Pagination does not leak unauthorized rows: page 1 of the
	// owner's scoped result must contain exactly one owned record.
	persons, total, page, pageSize, err := personSvc.ListPersons(
		models.PersonListQuery{Page: 1, PageSize: 1}, ownerActor)
	if err != nil {
		t.Fatalf("owner pagination: unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("owner pagination: expected total 2, got %d (count leak)", total)
	}
	if len(persons) != 1 || page != 1 || pageSize != 1 {
		t.Fatalf("owner pagination: expected 1 row on page 1/1, got rows=%d page=%d size=%d", len(persons), page, pageSize)
	}
	if persons[0].CreatedByID != owner.ID {
		t.Fatalf("owner pagination: unauthorized row leaked")
	}

	// 6. Count does not leak unauthorized rows.
	_, total, _, _, err = personSvc.ListPersons(
		models.PersonListQuery{Page: 1, PageSize: 10}, emptyActor)
	if err != nil {
		t.Fatalf("empty count: unexpected error: %v", err)
	}
	if total != 0 {
		t.Fatalf("empty count: expected total 0, got %d (count leak)", total)
	}

	// 7. Search cannot bypass ownership: the owner's unique NIC yields
	// nothing for actors with no authorized records.
	persons, total, _, _, err = personSvc.ListPersons(
		models.PersonListQuery{Search: personOwned1.NICPassport, Page: 1, PageSize: 50}, emptyActor)
	if err != nil {
		t.Fatalf("empty search: unexpected error: %v", err)
	}
	if total != 0 || len(persons) != 0 {
		t.Fatalf("empty search: expected 0 results, got rows=%d total=%d (search bypass)", len(persons), total)
	}
	persons, total, _, _, err = personSvc.ListPersons(
		models.PersonListQuery{Search: personOwned1.NICPassport, Page: 1, PageSize: 50}, intruderActor)
	if err != nil {
		t.Fatalf("intruder search: unexpected error: %v", err)
	}
	if total != 0 || len(persons) != 0 {
		t.Fatalf("intruder search: expected 0 results, got rows=%d total=%d (search bypass)", len(persons), total)
	}

	// 8. Search still works for the owner.
	persons, total, _, _, err = personSvc.ListPersons(
		models.PersonListQuery{Search: personOwned1.NICPassport, Page: 1, PageSize: 50}, ownerActor)
	if err != nil {
		t.Fatalf("owner search: unexpected error: %v", err)
	}
	if total != 1 || len(persons) != 1 {
		t.Fatalf("owner search: expected 1 result, got rows=%d total=%d", len(persons), total)
	}

	// 9. Status filter cannot bypass ownership.
	persons, total, _, _, err = personSvc.ListPersons(
		models.PersonListQuery{Status: models.PersonStatusActive, Page: 1, PageSize: 50}, emptyActor)
	if err != nil {
		t.Fatalf("empty filter: unexpected error: %v", err)
	}
	if total != 0 || len(persons) != 0 {
		t.Fatalf("empty filter: expected 0 results, got rows=%d total=%d (filter bypass)", len(persons), total)
	}

	// 10. Sort cannot bypass ownership: ordering is fixed server-side
	// (created_at DESC), so walking every page of the scoped result must
	// only ever yield owned records.
	for p := 1; p <= 2; p++ {
		rows, t2, _, _, err := personSvc.ListPersons(
			models.PersonListQuery{Page: p, PageSize: 1}, ownerActor)
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

	// 11. Object-level ID checks: owner can read; non-owners cannot.
	if _, err := personSvc.GetPersonByID(personOwned1.ID.String(), ownerActor); err != nil {
		t.Fatalf("owner get: unexpected error: %v", err)
	}
	if _, err := personSvc.GetPersonByID(personOwned1.ID.String(), emptyActor); err != services.ErrRecordAccessDenied {
		t.Fatalf("empty get: expected ErrRecordAccessDenied, got: %v", err)
	}
	if _, err := personSvc.GetPersonByID(personOwned1.ID.String(), intruderActor); err != services.ErrRecordAccessDenied {
		t.Fatalf("intruder get: expected ErrRecordAccessDenied, got: %v", err)
	}
}
