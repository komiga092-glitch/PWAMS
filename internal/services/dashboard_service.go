package services

import (
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

type DashboardService struct {
	userRepo *repository.UserRepository
}

func NewDashboardService(
	userRepo *repository.UserRepository,
) *DashboardService {
	return &DashboardService{
		userRepo: userRepo,
	}
}

func (s *DashboardService) GetStats() (
	*models.DashboardStats,
	error,
) {
	totalUsers, err := s.userRepo.CountAll()
	if err != nil {
		return nil, err
	}

	activeUsers, err := s.userRepo.CountByStatus(
		models.UserStatusActive,
	)
	if err != nil {
		return nil, err
	}

	disabledUsers, err := s.userRepo.CountByStatus(
		models.UserStatusDisabled,
	)
	if err != nil {
		return nil, err
	}

	lockedUsers, err := s.userRepo.CountByStatus(
		models.UserStatusLocked,
	)
	if err != nil {
		return nil, err
	}

	pendingUsers, err := s.userRepo.CountByStatus(
		models.UserStatusPending,
	)
	if err != nil {
		return nil, err
	}

	usersByRole, err := s.userRepo.CountByRole()
	if err != nil {
		return nil, err
	}

	return &models.DashboardStats{
		TotalUsers:    totalUsers,
		ActiveUsers:   activeUsers,
		DisabledUsers: disabledUsers,
		LockedUsers:   lockedUsers,
		PendingUsers:  pendingUsers,
		UsersByRole:   usersByRole,
	}, nil
}
