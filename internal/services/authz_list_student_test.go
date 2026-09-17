package services_test

import (
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// TestStudentListObjectLevelAuthorization verifies that ListStudents is
// scoped to the authenticated actor's ownership unless the actor holds
// a privileged role. Non-privileged owners make the exact assertions
// valid regardless of pre-existing rows in the shared DB.
func TestStudentListObjectLevelAuthorization(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	studentRepo := repository.NewStudentRepository(db)
	personRepo := repository.NewPersonRepository(db)
	studentSvc := services.NewStudentService(studentRepo, personRepo)

	owner := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "owner")
	intruder := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "intruder")
	emptyUser := makeUser(t, db, fx, roles.StudentID, models.RoleStudent, "empty")
	admin := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "admin")

	_, adminBaseline, _, _, err := studentSvc.ListStudents(
		models.StudentListQuery{Page: 1, PageSize: 1},
		services.Actor{ID: admin.ID, Role: models.RoleAdmin},
	)
	if err != nil {
		t.Fatalf("admin baseline: unexpected error: %v", err)
	}

	personOwned := makePerson(t, db, fx, owner.ID, "owner")
	personIntruder := makePerson(t, db, fx, intruder.ID, "intruder")

	studentOwned1 := makeStudent(t, db, fx, owner.ID, personOwned.ID, "owner1")
	makeStudent(t, db, fx, owner.ID, personOwned.ID, "owner2")
	makeStudent(t, db, fx, intruder.ID, personIntruder.ID, "intruder1")

	ownerActor := services.Actor{ID: owner.ID, Role: models.RoleDonor}
	intruderActor := services.Actor{ID: intruder.ID, Role: models.RoleBeneficiary}
	emptyActor := services.Actor{ID: emptyUser.ID, Role: models.RoleStudent}
	adminActor := services.Actor{ID: admin.ID, Role: models.RoleAdmin}

	// 1. Owner sees own student records (exactly 2).
	students, total, _, _, err := studentSvc.ListStudents(
		models.StudentListQuery{Page: 1, PageSize: 50}, ownerActor)
	if err != nil {
		t.Fatalf("owner list: unexpected error: %v", err)
	}
	if total != 2 || len(students) != 2 {
		t.Fatalf("owner list: expected 2 rows/total 2, got rows=%d total=%d", len(students), total)
	}
	for _, s := range students {
		if s.CreatedByID != owner.ID {
			t.Fatalf("owner list: returned record not owned by owner: created_by=%s", s.CreatedByID)
		}
	}

	// 2. Non-owner cannot enumerate the owner's student records; it
	//    sees only its own.
	students, total, _, _, err = studentSvc.ListStudents(
		models.StudentListQuery{Page: 1, PageSize: 50}, intruderActor)
	if err != nil {
		t.Fatalf("intruder list: unexpected error: %v", err)
	}
	if total != 1 || len(students) != 1 {
		t.Fatalf("intruder list: expected 1 row/total 1 (own only), got rows=%d total=%d", len(students), total)
	}
	if students[0].ID == studentOwned1.ID || students[0].CreatedByID == owner.ID {
		t.Fatalf("intruder list: exposed another user's record")
	}

	// 3. Empty result when no authorized records exist.
	students, total, _, _, err = studentSvc.ListStudents(
		models.StudentListQuery{Page: 1, PageSize: 50}, emptyActor)
	if err != nil {
		t.Fatalf("empty list: unexpected error: %v", err)
	}
	if total != 0 || len(students) != 0 {
		t.Fatalf("empty list: expected 0 results, got rows=%d total=%d (authorization bypass)", len(students), total)
	}

	// 4. Privileged Admin retains access (delta over baseline).
	students, total, _, _, err = studentSvc.ListStudents(
		models.StudentListQuery{Page: 1, PageSize: 50}, adminActor)
	if err != nil {
		t.Fatalf("admin list: unexpected error: %v", err)
	}
	if total != adminBaseline+3 {
		t.Fatalf("admin list: expected total %d (baseline %d + 3), got %d", adminBaseline+3, adminBaseline, total)
	}
	foundOwned := false
	for _, s := range students {
		if s.ID == studentOwned1.ID {
			foundOwned = true
		}
	}
	if !foundOwned {
		t.Fatalf("admin list: created record not visible to privileged role")
	}

	// 5. Pagination is correctly scoped: page 1 of the owner's result
	//    holds exactly one owned record with total reflecting only owned rows.
	students, total, page, pageSize, err := studentSvc.ListStudents(
		models.StudentListQuery{Page: 1, PageSize: 1}, ownerActor)
	if err != nil {
		t.Fatalf("owner pagination: unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("owner pagination: expected total 2, got %d (count leak)", total)
	}
	if len(students) != 1 || page != 1 || pageSize != 1 {
		t.Fatalf("owner pagination: expected 1 row on page 1/1, got rows=%d page=%d size=%d", len(students), page, pageSize)
	}
	if students[0].CreatedByID != owner.ID {
		t.Fatalf("owner pagination: unauthorized row leaked")
	}

	// 6. Count is correctly scoped for an actor with no authorized rows.
	_, total, _, _, err = studentSvc.ListStudents(
		models.StudentListQuery{Page: 1, PageSize: 10}, emptyActor)
	if err != nil {
		t.Fatalf("empty count: unexpected error: %v", err)
	}
	if total != 0 {
		t.Fatalf("empty count: expected total 0, got %d (count leak)", total)
	}

	// 7. Search cannot bypass ownership.
	students, total, _, _, err = studentSvc.ListStudents(
		models.StudentListQuery{Search: studentOwned1.StudentCode, Page: 1, PageSize: 50}, emptyActor)
	if err != nil {
		t.Fatalf("empty search: unexpected error: %v", err)
	}
	if total != 0 || len(students) != 0 {
		t.Fatalf("empty search: expected 0 results, got rows=%d total=%d (search bypass)", len(students), total)
	}
	students, total, _, _, err = studentSvc.ListStudents(
		models.StudentListQuery{Search: studentOwned1.StudentCode, Page: 1, PageSize: 50}, intruderActor)
	if err != nil {
		t.Fatalf("intruder search: unexpected error: %v", err)
	}
	if total != 0 || len(students) != 0 {
		t.Fatalf("intruder search: expected 0 results, got rows=%d total=%d (search bypass)", len(students), total)
	}

	// 8. Search still works for the owner.
	students, total, _, _, err = studentSvc.ListStudents(
		models.StudentListQuery{Search: studentOwned1.StudentCode, Page: 1, PageSize: 50}, ownerActor)
	if err != nil {
		t.Fatalf("owner search: unexpected error: %v", err)
	}
	if total != 1 || len(students) != 1 {
		t.Fatalf("owner search: expected 1 result, got rows=%d total=%d", len(students), total)
	}

	// 9. Grade filter cannot bypass ownership.
	students, total, _, _, err = studentSvc.ListStudents(
		models.StudentListQuery{Grade: "5", Page: 1, PageSize: 50}, emptyActor)
	if err != nil {
		t.Fatalf("empty filter: unexpected error: %v", err)
	}
	if total != 0 || len(students) != 0 {
		t.Fatalf("empty filter: expected 0 results, got rows=%d total=%d (filter bypass)", len(students), total)
	}

	// 10. Sort cannot bypass ownership: ordering is fixed server-side,
	//     so sweeping all pages yields owned records only.
	for p := 1; p <= 2; p++ {
		rows, t2, _, _, err := studentSvc.ListStudents(
			models.StudentListQuery{Page: p, PageSize: 1}, ownerActor)
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

	// 11. Object-level ID checks.
	if _, err := studentSvc.GetStudentByID(studentOwned1.ID.String(), ownerActor); err != nil {
		t.Fatalf("owner get: unexpected error: %v", err)
	}
	if _, err := studentSvc.GetStudentByID(studentOwned1.ID.String(), emptyActor); err != services.ErrRecordAccessDenied {
		t.Fatalf("empty get: expected ErrRecordAccessDenied, got: %v", err)
	}
	if _, err := studentSvc.GetStudentByID(studentOwned1.ID.String(), intruderActor); err != services.ErrRecordAccessDenied {
		t.Fatalf("intruder get: expected ErrRecordAccessDenied, got: %v", err)
	}
}
