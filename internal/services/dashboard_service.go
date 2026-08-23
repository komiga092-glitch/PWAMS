package services

import (
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

type DashboardService struct {
	userRepo      *repository.UserRepository
	dashboardRepo *repository.DashboardRepository
}

func NewDashboardService(
	userRepo *repository.UserRepository,
	dashboardRepos ...*repository.DashboardRepository,
) *DashboardService {
	var dashboardRepo *repository.DashboardRepository
	if len(dashboardRepos) > 0 {
		dashboardRepo = dashboardRepos[0]
	}
	return &DashboardService{
		userRepo:      userRepo,
		dashboardRepo: dashboardRepo,
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

	activeUsers, err := s.userRepo.CountByStatus(models.UserStatusActive)
	if err != nil {
		return nil, err
	}

	disabledUsers, err := s.userRepo.CountByStatus(models.UserStatusDisabled)
	if err != nil {
		return nil, err
	}

	lockedUsers, err := s.userRepo.CountByStatus(models.UserStatusLocked)
	if err != nil {
		return nil, err
	}

	pendingUsers, err := s.userRepo.CountByStatus(models.UserStatusPending)
	if err != nil {
		return nil, err
	}
	usersByRole, err := s.userRepo.CountByRole()
	if err != nil {
		return nil, err
	}

	stats := &models.DashboardStats{
		TotalUsers:    totalUsers,
		ActiveUsers:   activeUsers,
		DisabledUsers: disabledUsers,
		LockedUsers:   lockedUsers,
		PendingUsers:  pendingUsers,
		UsersByRole:   usersByRole,
	}
	if s.dashboardRepo == nil {
		return stats, nil
	}

	if stats.TotalBeneficiaries, err = s.dashboardRepo.Count("persons", ""); err != nil {
		return nil, err
	}
	if stats.TotalStudents, err = s.dashboardRepo.Count("students", ""); err != nil {
		return nil, err
	}
	if stats.TotalDonors, err = s.dashboardRepo.Count("donors", ""); err != nil {
		return nil, err
	}
	if stats.ActiveLoans, err = s.dashboardRepo.Count("loans", "status = ?", models.LoanStatusActive); err != nil {
		return nil, err
	}
	if stats.PendingAidRequests, err = s.dashboardRepo.Count("aid_requests", "status = ?", models.AidStatusPending); err != nil {
		return nil, err
	}
	if stats.RevenueSummary, err = s.dashboardRepo.NetRevenue(); err != nil {
		return nil, err
	}
	if stats.UnreadNotifications, err = s.dashboardRepo.Count("notifications", "is_read = ?", false); err != nil {
		return nil, err
	}

	return stats, nil
}
