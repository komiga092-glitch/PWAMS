package database

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PHASE 4E — the loan-repayment duplicate repair lives here and is ONLY
// reachable through the explicit `go run ./cmd/migrate repair-loan-repayments`
// command. The application (and its tests) never execute it automatically;
// startup refuses to continue instead of silently deleting business history.
//
// The keep-earliest rule is the documented project convention (migration
// 000004_loan_repayment_dedup): within a (loan_id, installment_number) group
// the row with the smallest created_at — ties broken by the smallest id — is
// authoritative; every later duplicate is removed.

const loanRepaymentDedupSQL = `
DELETE FROM loan_repayments a
USING loan_repayments b
WHERE a.loan_id = b.loan_id
  AND a.installment_number = b.installment_number
  AND (a.created_at > b.created_at
       OR (a.created_at = b.created_at AND a.id > b.id))`

// CountLoanRepaymentDuplicateGroups reports how many (loan_id,
// installment_number) groups contain more than one row and how many rows in
// total would be removed by the repair. Read-only.
func CountLoanRepaymentDuplicateGroups(db *gorm.DB) (groups int64, affectedRows int64, err error) {
	if !db.Migrator().HasTable("loan_repayments") {
		return 0, 0, nil
	}

	if err := db.Raw(`
		SELECT count(*) FROM (
			SELECT loan_id, installment_number
			FROM loan_repayments
			GROUP BY loan_id, installment_number
			HAVING count(*) > 1
		) d`).Scan(&groups).Error; err != nil {
		return 0, 0, fmt.Errorf("failed to count loan repayment duplicate groups: %w", err)
	}

	if groups == 0 {
		return 0, 0, nil
	}

	if err := db.Raw(`
		SELECT coalesce(sum(c - 1), 0) FROM (
			SELECT count(*) AS c
			FROM loan_repayments
			GROUP BY loan_id, installment_number
			HAVING count(*) > 1
		) d`).Scan(&affectedRows).Error; err != nil {
		return 0, 0, fmt.Errorf("failed to count duplicate loan repayment rows: %w", err)
	}

	return groups, affectedRows, nil
}

// LoanRepaymentDuplicateError is returned by the startup pre-check when
// duplicate repayments would prevent the composite unique index from being
// enforced. Startup never deletes the rows itself.
type LoanRepaymentDuplicateError struct {
	Groups int64
	Rows   int64
}

func (e *LoanRepaymentDuplicateError) Error() string {
	return fmt.Sprintf(
		"%d duplicate loan repayment group(s) (%d rows) prevent the (loan_id, installment_number) unique index; "+
			"run 'go run ./cmd/migrate repair-loan-repayments' (migration 000004) after taking a backup",
		e.Groups, e.Rows,
	)
}

// EnsureLoanRepaymentsRepairable is the fail-closed startup pre-check: it
// verifies that no duplicate repayments exist. It never modifies data.
func EnsureLoanRepaymentsRepairable(db *gorm.DB) error {
	groups, rows, err := CountLoanRepaymentDuplicateGroups(db)
	if err != nil {
		return err
	}
	if groups > 0 {
		return &LoanRepaymentDuplicateError{Groups: groups, Rows: rows}
	}
	return nil
}

// RepairLoanRepaymentDuplicates is the explicit, idempotent data repair:
// inside one transaction it removes the later duplicates of every
// (loan_id, installment_number) group (keeping the earliest row, per the
// documented 000004 rule) and then enforces the composite unique index. It
// returns the number of rows removed so the operator gets a report of
// exactly what was deleted.
func RepairLoanRepaymentDuplicates(db *gorm.DB) (removed int64, groups int64, err error) {
	groups, _, err = CountLoanRepaymentDuplicateGroups(db)
	if err != nil {
		return 0, 0, err
	}
	if groups == 0 {
		// Still make sure the unique index exists (idempotent).
		if err := ensureLoanRepaymentUniqueIndex(db); err != nil {
			return 0, 0, err
		}
		return 0, 0, nil
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(loanRepaymentDedupSQL)
		if result.Error != nil {
			return fmt.Errorf("loan repayment deduplication failed: %w", result.Error)
		}
		removed = result.RowsAffected

		return ensureLoanRepaymentUniqueIndex(tx)
	})
	if err != nil {
		return 0, groups, err
	}

	return removed, groups, nil
}

// ensureLoanRepaymentUniqueIndex creates the composite unique index when it
// is missing. Idempotent.
func ensureLoanRepaymentUniqueIndex(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uk_loan_repayment_installment
		ON loan_repayments (loan_id, installment_number)`).Error; err != nil {
		return fmt.Errorf("failed to ensure uk_loan_repayment_installment: %w", err)
	}
	return nil
}

// LoanRepaymentDuplicateGroup describes one duplicate (loan_id,
// installment_number) group and the row the repair would keep. Read-only
// reporting type: it is filled by SELECTs only.
type LoanRepaymentDuplicateGroup struct {
	LoanID            uuid.UUID `gorm:"column:loan_id"`
	InstallmentNumber int       `gorm:"column:installment_number"`
	Total             int64     `gorm:"column:total"`
	KeepID            uuid.UUID `gorm:"column:keep_id"`
}

// LoanRepaymentDuplicateGroups lists every duplicate group with the row the
// repair would keep (earliest created_at, then id — the documented 000004
// rule). Read-only. The listing is capped so the operator report stays
// bounded even on pathological data.
func LoanRepaymentDuplicateGroups(db *gorm.DB, limit int) ([]LoanRepaymentDuplicateGroup, error) {
	if !db.Migrator().HasTable("loan_repayments") {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	var groups []LoanRepaymentDuplicateGroup
	if err := db.Raw(`
		SELECT loan_id, installment_number, count(*) AS total,
		       (array_agg(id ORDER BY created_at ASC, id ASC))[1] AS keep_id
		FROM loan_repayments
		GROUP BY loan_id, installment_number
		HAVING count(*) > 1
		ORDER BY loan_id, installment_number
		LIMIT ?`, limit).Scan(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to list loan repayment duplicate groups: %w", err)
	}

	return groups, nil
}
