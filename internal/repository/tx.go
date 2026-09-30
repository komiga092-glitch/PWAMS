package repository

import (
	"gorm.io/gorm"
)

// Transaction-scoped repository helpers.
//
// PWAMS must be able to commit a business write and its mandatory audit log
// row atomically. Every repository in this project owns its own *gorm.DB
// handle, so the safest way to share a caller's transaction without changing
// existing method signatures or duplicating query logic is a shallow copy
// bound to the transaction:
//
//	err := db.Transaction(func(tx *gorm.DB) error {
//		person, err := personRepo.WithTx(tx).Create(person)
//		if err != nil {
//			return err
//		}
//		return auditRepo.WithTx(tx).Create(auditLog)
//	})
//
// The returned repository performs exactly the same operations as the
// original; only the underlying connection/transaction differs.
//
// WithTx deliberately tolerates a nil receiver (it returns a copy bound to
// tx) so a misconfigured dependency can never cause a nil pointer panic.

func (r *PersonRepository) WithTx(tx *gorm.DB) *PersonRepository {
	if r == nil {
		return &PersonRepository{db: tx}
	}
	return &PersonRepository{db: tx}
}

func (r *StudentRepository) WithTx(tx *gorm.DB) *StudentRepository {
	if r == nil {
		return &StudentRepository{db: tx}
	}
	return &StudentRepository{db: tx}
}

func (r *DonorRepository) WithTx(tx *gorm.DB) *DonorRepository {
	if r == nil {
		return &DonorRepository{db: tx}
	}
	return &DonorRepository{db: tx}
}

func (r *DonationRepository) WithTx(tx *gorm.DB) *DonationRepository {
	if r == nil {
		return &DonationRepository{db: tx}
	}
	return &DonationRepository{db: tx}
}

func (r *LoanRepository) WithTx(tx *gorm.DB) *LoanRepository {
	if r == nil {
		return &LoanRepository{db: tx}
	}
	return &LoanRepository{db: tx}
}

func (r *LoanRepaymentRepository) WithTx(tx *gorm.DB) *LoanRepaymentRepository {
	if r == nil {
		return &LoanRepaymentRepository{db: tx}
	}
	return &LoanRepaymentRepository{db: tx}
}

func (r *CareProvidedRepository) WithTx(tx *gorm.DB) *CareProvidedRepository {
	if r == nil {
		return &CareProvidedRepository{db: tx}
	}
	return &CareProvidedRepository{db: tx}
}

func (r *AidRequestRepository) WithTx(tx *gorm.DB) *AidRequestRepository {
	if r == nil {
		return &AidRequestRepository{db: tx}
	}
	return &AidRequestRepository{db: tx}
}

func (r *RevenueRepository) WithTx(tx *gorm.DB) *RevenueRepository {
	if r == nil {
		return &RevenueRepository{db: tx}
	}
	return &RevenueRepository{db: tx}
}

func (r *UserRepository) WithTx(tx *gorm.DB) *UserRepository {
	if r == nil {
		return &UserRepository{db: tx}
	}
	return &UserRepository{db: tx}
}

func (r *RoleRepository) WithTx(tx *gorm.DB) *RoleRepository {
	if r == nil {
		return &RoleRepository{db: tx}
	}
	return &RoleRepository{db: tx}
}

func (r *SessionRepository) WithTx(tx *gorm.DB) *SessionRepository {
	if r == nil {
		return &SessionRepository{db: tx}
	}
	return &SessionRepository{db: tx}
}

func (r *AuditLogRepository) WithTx(tx *gorm.DB) *AuditLogRepository {
	if r == nil {
		return &AuditLogRepository{db: tx}
	}
	return &AuditLogRepository{db: tx}
}

// DB exposes the underlying handle. The audit service uses it to open the
// single transaction that carries both the business write and the audit row.
func (r *AuditLogRepository) DB() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.db
}