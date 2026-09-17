package services_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// ---------------------------------------------------------------------------
// OPTIMISTIC LOCKING
// ---------------------------------------------------------------------------
//
// These tests exercise the atomic compare-and-swap primitive
// (repository.applyOptimisticUpdate) end to end through the service layer.
// They are skipped when no PostgreSQL database is available, matching the
// Step 5 data-integrity test harness.

// newOptimisticLockPerson creates a person via the service layer using a
// dedicated creator so the CreatedByID foreign key is satisfied.
func newOptimisticLockPerson(t *testing.T, db *gorm.DB, suffix string) *models.Person {
	t.Helper()

	roleRepo := repository.NewRoleRepository(db)
	var staffRole models.Role
	if err := db.Where("name = ?", models.RoleStaff).First(&staffRole).Error; err != nil {
		t.Fatalf("resolve Staff role: %v", err)
	}
	creator := &models.User{
		Username:     "olc-" + suffix,
		Email:        "olc-" + suffix + "@pwams.local",
		FullName:     "OL Creator",
		RoleID:       staffRole.ID,
		Status:       models.UserStatusActive,
		PasswordHash: "$2a$10$dummyhashdummyhashdummyhashdummyhashdummyhashdummyhashdum",
	}
	if err := db.Create(creator).Error; err != nil {
		t.Fatalf("create ol-creator: %v", err)
	}
	_ = roleRepo

	personRepo := repository.NewPersonRepository(db)
	svc := services.NewPersonService(personRepo)

	// NIC/Passport is varchar(30); keep the value short enough. We only need
	// uniqueness across test runs, so a truncated suffix suffices.
	nic := "OL" + suffix
	if len(nic) > 20 {
		nic = nic[:20]
	}

	person, err := svc.CreatePerson(models.CreatePersonRequest{
		FullName:      "OL Person " + suffix,
		NICPassport:   nic,
		Gender:        "Male",
		Phone:         "0771234567",
		Email:         "ol-" + suffix + "@pwams.local",
		Address:       "123 OL Street",
		Occupation:    "Tester",
		MonthlyIncome: decimal.NewFromInt(50000),
	}, creator.ID)
	if err != nil {
		t.Fatalf("create ol-person: %v", err)
	}
	return person
}

// TestOptimisticLock_StaleVersionRejected verifies that an update carrying
// a stale version is rejected with ErrOptimisticLockConflict and the
// surviving row still holds the *other* writer's value — no lost update.
func TestOptimisticLock_StaleVersionRejected(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newOptimisticLockPerson(t, db, suffix)

	// Both readers observe version 1.
	readerA := *person
	readerB := *person

	// Writer B applies first, moving the stored version to 2.
	svc := services.NewPersonService(repository.NewPersonRepository(db))
	_, err := svc.UpdatePerson(person.ID.String(), models.UpdatePersonRequest{
		FullName:      "Writer B Update",
		NICPassport:   readerB.NICPassport,
		Gender:        readerB.Gender,
		Phone:         readerB.Phone,
		Email:         readerB.Email,
		Address:       readerB.Address,
		Occupation:    readerB.Occupation,
		MonthlyIncome: readerB.MonthlyIncome,
		Status:        readerB.Status,
		Version:       readerB.Version, // 1
	})
	if err != nil {
		t.Fatalf("writer B update must succeed: %v", err)
	}

	// Writer A still holds version 1 — must be rejected.
	_, err = svc.UpdatePerson(person.ID.String(), models.UpdatePersonRequest{
		FullName:      "Writer A Update",
		NICPassport:   readerA.NICPassport,
		Gender:        readerA.Gender,
		Phone:         readerA.Phone,
		Email:         readerA.Email,
		Address:       readerA.Address,
		Occupation:    readerA.Occupation,
		MonthlyIncome: readerA.MonthlyIncome,
		Status:        readerA.Status,
		Version:       readerA.Version, // 1 (stale)
	})
	if err == nil {
		t.Fatal("stale-version update must be rejected")
	}
	if !errors.Is(err, services.ErrOptimisticLockConflict) {
		t.Fatalf("expected ErrOptimisticLockConflict, got: %v", err)
	}

	// The row must hold Writer B's value, not Writer A's.
	var persisted models.Person
	if err := db.First(&persisted, "id = ?", person.ID).Error; err != nil {
		t.Fatalf("reload person: %v", err)
	}
	if persisted.FullName != "Writer B Update" {
		t.Fatalf("lost update: full_name = %q, want %q", persisted.FullName, "Writer B Update")
	}
	if persisted.Version != 2 {
		t.Fatalf("version = %d, want 2", persisted.Version)
	}
}

// TestOptimisticLock_UpdateAfterSoftDelete verifies that a stale update
// submitted against a now-deleted record is rejected as a conflict (never
// resurrecting the row).
func TestOptimisticLock_UpdateAfterSoftDelete(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newOptimisticLockPerson(t, db, suffix)

	reader := *person // version 1

	// Another actor soft-deletes the record first.
	svc := services.NewPersonService(repository.NewPersonRepository(db))
	if err := svc.DeletePerson(person.ID.String()); err != nil {
		t.Fatalf("delete must succeed: %v", err)
	}

	// A stale update carrying the pre-delete version must be rejected.
	// The service's FindByID runs first against a soft-deleted record, so the
	// error surfaces as ErrPersonNotFound — the update is still refused.
	_, err := svc.UpdatePerson(person.ID.String(), models.UpdatePersonRequest{
		FullName:      "Zombie Update",
		NICPassport:   reader.NICPassport,
		Gender:        reader.Gender,
		Phone:         reader.Phone,
		Email:         reader.Email,
		Address:       reader.Address,
		Occupation:    reader.Occupation,
		MonthlyIncome: reader.MonthlyIncome,
		Status:        reader.Status,
		Version:       reader.Version, // 1
	})
	if err == nil {
		t.Fatal("update after soft-delete must be rejected")
	}
	if !errors.Is(err, services.ErrOptimisticLockConflict) && !errors.Is(err, repository.ErrPersonNotFound) {
		t.Fatalf("expected ErrOptimisticLockConflict or ErrPersonNotFound, got: %v", err)
	}

	// The row must remain soft-deleted (no resurrection).
	var persisted models.Person
	if err := db.Unscoped().First(&persisted, "id = ?", person.ID).Error; err != nil {
		t.Fatalf("unscoped reload: %v", err)
	}
	if !persisted.DeletedAt.Valid {
		t.Fatal("soft-deleted row must stay deleted")
	}
}

// TestOptimisticLock_MissingVersionRejected verifies that an update with no
// usable expected version (zero) is rejected with ErrInvalidRecordVersion —
// a client contract violation, distinct from a conflict.
func TestOptimisticLock_MissingVersionRejected(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newOptimisticLockPerson(t, db, suffix)

	svc := services.NewPersonService(repository.NewPersonRepository(db))
	_, err := svc.UpdatePerson(person.ID.String(), models.UpdatePersonRequest{
		FullName:      "No Version Update",
		NICPassport:   person.NICPassport,
		Gender:        person.Gender,
		Phone:         person.Phone,
		Email:         person.Email,
		Address:       person.Address,
		Occupation:    person.Occupation,
		MonthlyIncome: person.MonthlyIncome,
		Status:        person.Status,
		Version:       0, // missing
	})
	if err == nil {
		t.Fatal("missing-version update must be rejected")
	}
	if !errors.Is(err, services.ErrInvalidRecordVersion) {
		t.Fatalf("expected ErrInvalidRecordVersion, got: %v", err)
	}
}

// TestOptimisticLock_SuccessIncrementsVersion verifies the happy path: a
// correct update succeeds and the stored version advances by exactly one.
func TestOptimisticLock_SuccessIncrementsVersion(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newOptimisticLockPerson(t, db, suffix)

	if person.Version != 1 {
		t.Fatalf("initial version = %d, want 1", person.Version)
	}

	svc := services.NewPersonService(repository.NewPersonRepository(db))
	_, err := svc.UpdatePerson(person.ID.String(), models.UpdatePersonRequest{
		FullName:      "Version 2 Name",
		NICPassport:   person.NICPassport,
		Gender:        person.Gender,
		Phone:         person.Phone,
		Email:         person.Email,
		Address:       person.Address,
		Occupation:    person.Occupation,
		MonthlyIncome: person.MonthlyIncome,
		Status:        person.Status,
		Version:       1,
	})
	if err != nil {
		t.Fatalf("valid update must succeed: %v", err)
	}

	// The stored version must advance to 2 (re-read from the database — the
	// service returns the in-memory object whose Version is not rewritten).
	var persisted models.Person
	if err := db.First(&persisted, "id = ?", person.ID).Error; err != nil {
		t.Fatalf("reload person: %v", err)
	}
	if persisted.Version != 2 {
		t.Fatalf("version after update = %d, want 2", persisted.Version)
	}

	// Second correct update advances to 3.
	_, err = svc.UpdatePerson(person.ID.String(), models.UpdatePersonRequest{
		FullName:      "Version 3 Name",
		NICPassport:   person.NICPassport,
		Gender:        person.Gender,
		Phone:         person.Phone,
		Email:         person.Email,
		Address:       person.Address,
		Occupation:    person.Occupation,
		MonthlyIncome: person.MonthlyIncome,
		Status:        person.Status,
		Version:       2,
	})
	if err != nil {
		t.Fatalf("second valid update must succeed: %v", err)
	}
	if err := db.First(&persisted, "id = ?", person.ID).Error; err != nil {
		t.Fatalf("reload person: %v", err)
	}
	if persisted.Version != 3 {
		t.Fatalf("version after second update = %d, want 3", persisted.Version)
	}
}

// writers all starting from the same read, at most one wins and the stored
// version equals 1 + (number of successful serialised writes). No write is
// silently lost.
func TestOptimisticLock_ConcurrentUpdates(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newOptimisticLockPerson(t, db, suffix)

	const n = 10
	var wg sync.WaitGroup
	results := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each goroutine gets its own repo over the shared DB pool.
			repo := repository.NewPersonRepository(db)
			svc := services.NewPersonService(repo)
			_, err := svc.UpdatePerson(person.ID.String(), models.UpdatePersonRequest{
				FullName:      "Concurrent " + uuid.NewString(),
				NICPassport:   person.NICPassport,
				Gender:        person.Gender,
				Phone:         person.Phone,
				Email:         person.Email,
				Address:       person.Address,
				Occupation:    person.Occupation,
				MonthlyIncome: person.MonthlyIncome,
				Status:        person.Status,
				Version:       person.Version, // all read version 1
			})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent same-version updates: successes = %d, want exactly 1", successes)
	}

	var persisted models.Person
	if err := db.First(&persisted, "id = ?", person.ID).Error; err != nil {
		t.Fatalf("reload person: %v", err)
	}
	if persisted.Version != 2 {
		t.Fatalf("version = %d, want 2 (initial 1 + 1 successful write)", persisted.Version)
	}
}

// TestOptimisticLock_ConcurrentDeleteRace verifies that when two actors try
// to soft-delete the same record concurrently (both reading version 1), only
// one delete succeeds and the other observes a conflict — never a
// double-delete or a lost delete.
func TestOptimisticLock_ConcurrentDeleteRace(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	suffix := uuid.NewString()
	person := newOptimisticLockPerson(t, db, suffix)

	const n = 8
	var wg sync.WaitGroup
	results := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repo := repository.NewPersonRepository(db)
			svc := services.NewPersonService(repo)
			results <- svc.DeletePerson(person.ID.String())
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	conflicts := 0
	notFound := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case err == services.ErrOptimisticLockConflict:
			conflicts++
		case err == repository.ErrPersonNotFound:
			// A goroutine's FindByID ran after another goroutine had already
			// soft-deleted the record. This is a valid race outcome: the
			// delete did not happen, but it was not a lost update either.
			notFound++
		default:
			t.Fatalf("unexpected delete error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent deletes: successes = %d, want exactly 1", successes)
	}
	// Exactly one delete succeeded; the rest were either conflicts or
	// not-found outcomes. No other error is acceptable.
	if successes+conflicts+notFound != n {
		t.Fatalf("concurrent deletes: successes=%d conflicts=%d notFound=%d, want total %d", successes, conflicts, notFound, n)
	}

	var persisted models.Person
	if err := db.Unscoped().First(&persisted, "id = ?", person.ID).Error; err != nil {
		t.Fatalf("unscoped reload: %v", err)
	}
	if !persisted.DeletedAt.Valid {
		t.Fatal("record must be soft-deleted")
	}
	if persisted.Version != 2 {
		t.Fatalf("version = %d, want 2 (initial 1 + 1 successful delete)", persisted.Version)
	}
}
