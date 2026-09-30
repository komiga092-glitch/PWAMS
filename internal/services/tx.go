package services

import (
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/repository"
)

// Transaction-scoped service helpers.
//
// A service copy bound to a transaction exposes exactly the same business
// methods as the original (no logic is duplicated); only the repositories it
// delegates to are scoped to the caller's transaction. This lets handlers run
//
//	business write + mandatory audit write
//
// inside one database transaction without restructuring the service layer:
//
//	err := auditService.Transaction(func(tx *gorm.DB) error {
//		person, err := personService.WithTx(tx).CreatePerson(request, actorID)
//		if err != nil {
//			return err
//		}
//		return auditService.WithTx(tx).Create(actorID.String(), "CREATE", "persons", person.ID.String(), "…")
//	})
//
// Nil receivers are tolerated deliberately: a misconfigured dependency fails
// cleanly (a nil-pointer dereference on the repository copy is impossible
// because repository.WithTx itself tolerates a nil receiver) instead of
// panicking.

func (s *PersonService) WithTx(tx *gorm.DB) *PersonService {
	service := &PersonService{}
	if s == nil {
		service.personRepo = (*repository.PersonRepository)(nil).WithTx(tx)
		return service
	}
	service.personRepo = s.personRepo.WithTx(tx)
	return service
}

func (s *StudentService) WithTx(tx *gorm.DB) *StudentService {
	service := &StudentService{}
	if s == nil {
		service.studentRepo = (*repository.StudentRepository)(nil).WithTx(tx)
		service.personRepo = (*repository.PersonRepository)(nil).WithTx(tx)
		return service
	}
	service.studentRepo = s.studentRepo.WithTx(tx)
	service.personRepo = s.personRepo.WithTx(tx)
	return service
}

func (s *DonorService) WithTx(tx *gorm.DB) *DonorService {
	service := &DonorService{}
	if s == nil {
		service.donorRepo = (*repository.DonorRepository)(nil).WithTx(tx)
		return service
	}
	service.donorRepo = s.donorRepo.WithTx(tx)
	return service
}

func (s *DonationService) WithTx(tx *gorm.DB) *DonationService {
	service := &DonationService{}
	if s == nil {
		service.donationRepo = (*repository.DonationRepository)(nil).WithTx(tx)
		service.donorRepo = (*repository.DonorRepository)(nil).WithTx(tx)
		service.personRepo = (*repository.PersonRepository)(nil).WithTx(tx)
		return service
	}
	service.donationRepo = s.donationRepo.WithTx(tx)
	service.donorRepo = s.donorRepo.WithTx(tx)
	service.personRepo = s.personRepo.WithTx(tx)
	return service
}

func (s *LoanService) WithTx(tx *gorm.DB) *LoanService {
	service := &LoanService{}
	if s == nil {
		service.loanRepo = (*repository.LoanRepository)(nil).WithTx(tx)
		service.personRepo = (*repository.PersonRepository)(nil).WithTx(tx)
		return service
	}
	service.loanRepo = s.loanRepo.WithTx(tx)
	service.personRepo = s.personRepo.WithTx(tx)
	return service
}

// WithTx rewires the service's own database handle as well, so nested
// s.db.Transaction calls become savepoints on the caller's transaction
// instead of opening a second, independent connection.
func (s *LoanRepaymentService) WithTx(tx *gorm.DB) *LoanRepaymentService {
	service := &LoanRepaymentService{db: tx}
	if s == nil {
		service.repaymentRepo = (*repository.LoanRepaymentRepository)(nil).WithTx(tx)
		service.loanRepo = (*repository.LoanRepository)(nil).WithTx(tx)
		return service
	}
	service.repaymentRepo = s.repaymentRepo.WithTx(tx)
	service.loanRepo = s.loanRepo.WithTx(tx)
	return service
}

func (s *CareProvidedService) WithTx(tx *gorm.DB) *CareProvidedService {
	service := &CareProvidedService{}
	if s == nil {
		service.careProvidedRepo = (*repository.CareProvidedRepository)(nil).WithTx(tx)
		return service
	}
	service.careProvidedRepo = s.careProvidedRepo.WithTx(tx)
	return service
}

func (s *AidRequestService) WithTx(tx *gorm.DB) *AidRequestService {
	service := &AidRequestService{}
	if s == nil {
		service.aidRequestRepo = (*repository.AidRequestRepository)(nil).WithTx(tx)
		service.personRepo = (*repository.PersonRepository)(nil).WithTx(tx)
		return service
	}
	service.aidRequestRepo = s.aidRequestRepo.WithTx(tx)
	service.personRepo = s.personRepo.WithTx(tx)
	service.notificationService = s.notificationService
	return service
}

func (s *RevenueService) WithTx(tx *gorm.DB) *RevenueService {
	service := &RevenueService{}
	if s == nil {
		service.repo = (*repository.RevenueRepository)(nil).WithTx(tx)
		return service
	}
	service.repo = s.repo.WithTx(tx)
	return service
}

func (s *UserService) WithTx(tx *gorm.DB) *UserService {
	service := &UserService{}
	if s == nil {
		service.userRepo = (*repository.UserRepository)(nil).WithTx(tx)
		service.roleRepo = (*repository.RoleRepository)(nil).WithTx(tx)
		service.sessionRepo = (*repository.SessionRepository)(nil).WithTx(tx)
		return service
	}
	service.userRepo = s.userRepo.WithTx(tx)
	service.roleRepo = s.roleRepo.WithTx(tx)
	service.sessionRepo = s.sessionRepo.WithTx(tx)
	service.adminDeletionRepo = s.adminDeletionRepo
	return service
}
