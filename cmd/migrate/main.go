// PHASE 4C/4D/4E — operator migration CLI.
//
// Safety contract:
//   - `status` and `verify` are completely read-only: they never create the
//     tracking table and never execute any migration or data statement.
//   - `up` applies tracked migrations ONLY when an operator invokes this
//     command. The server and the test suite never call it.
//   - `repair-loan-repayments` prints a read-only duplicate report first. It
//     only modifies data when the operator passes --confirm. No other code
//     path in the project can invoke the repair (see
//     internal/database/repair.go: the server, Migrate(), AutoMigrate and
//     all tests are delete-free).
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"gorm.io/gorm"
)

const usageText = `PWAMS migration CLI

Usage:
  go run ./cmd/migrate <command> [flags]

Commands (explicit operator actions only — nothing runs automatically):
  status                  Show tracked migration state. READ-ONLY.
  verify                  Exit 1 when migrations are pending or drifted.
                          READ-ONLY. This is what production startup runs.
  up                      Apply pending tracked migrations in order, each in
                          its own transaction with its tracking record.
                          Take a verified backup first.
                          Optional: --to N stages the run (versions <= N).
  repair-loan-repayments  Report duplicate (loan_id, installment_number)
                          loan_repayments rows. READ-ONLY unless --confirm is
                          passed; with --confirm it removes later duplicates
                          (keeping the earliest row) and enforces the
                          uk_loan_repayment_installment unique index.

Flags (up and repair-loan-repayments only):
  --to N                  (up) apply only migration versions <= N.
  --confirm               (repair-loan-repayments) REQUIRED before the
                          repair modifies data.
  --limit N               Cap the number of duplicate groups listed in the
                          report (default 200, max 500).
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}

	command := strings.TrimSpace(os.Args[1])
	if command == "-h" || command == "--help" || command == "help" {
		fmt.Print(usageText)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		exitWithError("configuration error: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		exitWithError("database connection error: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		exitWithError("database instance error: %v", err)
	}
	defer sqlDB.Close()

	switch command {
	case "status":
		runStatus(db)
	case "verify":
		runVerify(db)
	case "up":
		runUp(db, os.Args[2:])
	case "repair-loan-repayments":
		runRepairLoanRepayments(db, os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", command, usageText)
		os.Exit(2)
	}
}

// runStatus is completely read-only.
func runStatus(db *gorm.DB) {
	statuses, err := database.Status(db)
	if err != nil {
		exitWithError("status failed: %v", err)
	}

	fmt.Println("migration status (read-only):")
	fmt.Println("version  name                             applied                   checksum")
	for _, state := range statuses {
		checksum := "ok"
		if state.Applied && !state.ChecksumMatch {
			checksum = "DRIFTED"
		}
		applied := "no"
		if state.Applied && state.AppliedAt != nil {
			applied = state.AppliedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Printf("%06d    %-32s %-25s %s\n", state.Version, state.Name, applied, checksum)
	}
}

// runVerify is completely read-only; it is the same check production
// startup performs before serving traffic.
func runVerify(db *gorm.DB) {
	if err := database.VerifyUpToDate(db); err != nil {
		fmt.Fprintf(os.Stderr, "verify failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("verify ok: database schema is up to date (no pending, no drifted migrations)")
}

// runUp applies pending tracked migrations. Explicit operator action only —
// the server and the tests never call MigrateUp. An optional --to N stages
// the run: only versions <= N are applied (e.g. `up --to 6`), enabling the
// Phase 4P staged deployment; without --to every pending migration applies.
func runUp(db *gorm.DB, flags []string) {
	maxVersion := int64(math.MaxInt64)
	if raw := flagValue(flags, "--to"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			exitWithError("invalid --to value %q: %v", raw, err)
		}
		if parsed <= 0 {
			exitWithError("--to must be a positive migration version, got %d", parsed)
		}
		maxVersion = parsed
	}

	fmt.Println("applying pending tracked migrations (explicit operator action)")
	fmt.Println("REMINDER: make sure a verified database backup exists first")
	if maxVersion != math.MaxInt64 {
		fmt.Printf("staged run: applying versions <= %06d only\n", maxVersion)
	}
	if err := database.MigrateUpTo(db, maxVersion); err != nil {
		exitWithError("migration failed: %v", err)
	}
	fmt.Println("migrations applied successfully")
}

// runRepairLoanRepayments prints the read-only duplicate report first and
// only modifies data when --confirm is present.
func runRepairLoanRepayments(db *gorm.DB, flags []string) {
	confirm := hasFlag(flags, "--confirm")
	limit := 200
	if raw := flagValue(flags, "--limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			exitWithError("invalid --limit value %q: %v", raw, err)
		}
		limit = parsed
	}

	groups, affected, err := database.CountLoanRepaymentDuplicateGroups(db)
	if err != nil {
		exitWithError("duplicate count failed: %v", err)
	}

	fmt.Println("loan repayment duplicate report (read-only):")
	fmt.Printf("  duplicate groups: %d\n", groups)
	fmt.Printf("  rows the repair would remove: %d\n", affected)

	details, err := database.LoanRepaymentDuplicateGroups(db, limit)
	if err != nil {
		exitWithError("duplicate listing failed: %v", err)
	}
	for _, group := range details {
		fmt.Printf(
			"  loan=%s installment=%d rows=%d keep_id=%s\n",
			group.LoanID, group.InstallmentNumber, group.Total, group.KeepID,
		)
	}

	if !confirm {
		if groups > 0 {
			fmt.Println("confirmation missing (--confirm): NO DATA WAS MODIFIED")
			os.Exit(1)
		}
		fmt.Println("nothing to repair")
		return
	}

	removed, _, err := database.RepairLoanRepaymentDuplicates(db)
	if err != nil {
		exitWithError("repair failed: %v", err)
	}
	fmt.Printf("repair complete: %d rows removed, unique index enforced\n", removed)
}

func hasFlag(flags []string, name string) bool {
	for _, flag := range flags {
		if flag == name || strings.HasPrefix(flag, name+"=") {
			return true
		}
	}
	return false
}

func flagValue(flags []string, name string) string {
	for i, flag := range flags {
		if strings.HasPrefix(flag, name+"=") {
			return strings.TrimPrefix(flag, name+"=")
		}
		if flag == name && i+1 < len(flags) {
			return flags[i+1]
		}
	}
	return ""
}

func exitWithError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "migrate: "+format+"\n", args...)
	os.Exit(1)
}
