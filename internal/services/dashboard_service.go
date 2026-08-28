package services

import (
	"github.com/google/uuid"
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

func (s *DashboardService) GetStats(userID uuid.UUID) (
	*models.DashboardStats,
	error,
) {
	return s.getStatsForRole(userID, models.RoleSuperAdmin)
}

func (s *DashboardService) getStatsForRole(userID uuid.UUID, role string) (
	*models.DashboardStats,
	error,
) {
	stats := &models.DashboardStats{}
	if role != models.RoleSuperAdmin && role != models.RoleAdmin && role != models.RoleStaff {
		if s.dashboardRepo == nil {
			return stats, nil
		}
		unread, err := s.dashboardRepo.CountWithoutDeleted(
			"notifications",
			"user_id = ? AND is_read = ?",
			userID,
			false,
		)
		if err != nil {
			return nil, err
		}
		stats.UnreadNotifications = unread
		return stats, nil
	}

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

	stats = &models.DashboardStats{
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

	// Records can be soft-deleted through two paths: online CRUD sets
	// gorm's deleted_at while offline sync sets is_deleted. Counts must
	// exclude both or synced deletions inflate the numbers.
	if stats.TotalBeneficiaries, err = s.dashboardRepo.Count("persons", "is_deleted = ?", false); err != nil {
		return nil, err
	}
	if stats.TotalStudents, err = s.dashboardRepo.Count("students", "is_deleted = ?", false); err != nil {
		return nil, err
	}
	if stats.TotalDonors, err = s.dashboardRepo.Count("donors", "is_deleted = ?", false); err != nil {
		return nil, err
	}
	if stats.ActiveLoans, err = s.dashboardRepo.CountWithoutDeleted("loans", "status = ? AND is_deleted = ?", models.LoanStatusActive, false); err != nil {
		return nil, err
	}
	if stats.PendingAidRequests, err = s.dashboardRepo.Count("aid_requests", "status = ? AND is_deleted = ?", models.AidStatusPending, false); err != nil {
		return nil, err
	}
	if stats.RevenueSummary, err = s.dashboardRepo.NetRevenue(); err != nil {
		return nil, err
	}
	if stats.UnreadNotifications, err = s.dashboardRepo.CountWithoutDeleted("notifications", "user_id = ? AND is_read = ?", userID, false); err != nil {
		return nil, err
	}

	return stats, nil
}

func (s *DashboardService) GetStatsForRole(
	userID uuid.UUID,
	role string,
) (*models.DashboardStats, error) {
	return s.getStatsForRole(userID, role)
}
