package services_test

import (
	"log"
	"os"
	"sync"
	"testing"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"gorm.io/gorm"
)

var (
	testDB     *gorm.DB
	testDBOnce sync.Once
	testDBErr  error
)

// AcquireTestDB returns a shared *gorm.DB connected to the integration test
// database, initialising it exactly once per test binary. The connection
// string is read from the same environment variables the application uses.
// Tests that require a live database call t.Skip if no database is reachable
// so the unit-test suite stays green in restricted CI environments.
func AcquireTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	testDBOnce.Do(func() {
		cfg, err := config.Load()
		if err != nil {
			testDBErr = err
			return
		}

		testDB, testDBErr = database.Connect(cfg)
		if testDBErr != nil {
			return
		}

		if testDBErr = database.Migrate(testDB); testDBErr != nil {
			log.Printf("test DB migration issue: %v", testDBErr)
		}

		// The migration only creates the schema. Seed the default roles so
		// integration tests (authz_*, audit_behavioral) can resolve role IDs
		// on a fresh database. Idempotent (FirstOrCreate).
		if seedErr := database.SeedDefaultRoles(testDB); seedErr != nil {
			log.Printf("test DB role seeding issue: %v", seedErr)
		}
	})

	if testDBErr != nil {
		t.Skipf("integration test DB unavailable: %v", testDBErr)
	}

	if testDB == nil {
		t.Skip("integration test DB unavailable: connection is nil")
	}

	return testDB
}

// SkipUnlessForceIntegration allows an operator to opt into the live-DB
// integration suite even when the default probe would skip (for example,
// when the DB requires a tunnel). It reads PWAMS_FORCE_INTEGRATION=1.
func SkipUnlessForceIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("PWAMS_FORCE_INTEGRATION") != "1" {
		t.Skip("set PWAMS_FORCE_INTEGRATION=1 to run the live-DB integration suite")
	}
}
