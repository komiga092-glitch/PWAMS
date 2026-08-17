package repository

import (
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
)

type ReportRepository struct {
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) *ReportRepository {
	return &ReportRepository{
		db: db,
	}
}

func (r *ReportRepository) GetDashboardReport() (*models.DashboardReport, error) {
	report := &models.DashboardReport{}

	queries := []struct {
		query string
		dest  *int64
	}{
		{
			query: `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`,
			dest:  &report.TotalUsers,
		},
		{
			query: `SELECT COUNT(*) FROM persons WHERE deleted_at IS NULL`,
			dest:  &report.TotalPersons,
		},
		{
			query: `SELECT COUNT(*) FROM students WHERE deleted_at IS NULL`,
			dest:  &report.TotalStudents,
		},
		{
			query: `SELECT COUNT(*) FROM donors WHERE deleted_at IS NULL`,
			dest:  &report.TotalDonors,
		},
		{
			query: `SELECT COUNT(*) FROM donations WHERE deleted_at IS NULL`,
			dest:  &report.TotalDonations,
		},
		{
			query: `SELECT COUNT(*) FROM aid_requests WHERE deleted_at IS NULL`,
			dest:  &report.TotalAidRequests,
		},
	}

	for _, item := range queries {
		if err := r.db.Raw(item.query).Scan(item.dest).Error; err != nil {
			return nil, err
		}
	}

	return report, nil
}

func (r *ReportRepository) GetDonationReport() (*models.DonationReport, error) {
	report := &models.DonationReport{}

	query := `
		SELECT
			COUNT(*) AS total_donations,
			COALESCE(SUM(amount), 0) AS total_amount
		FROM donations
		WHERE deleted_at IS NULL
	`

	if err := r.db.Raw(query).Scan(report).Error; err != nil {
		return nil, err
	}

	return report, nil
}

func (r *ReportRepository) GetAidRequestReport() (*models.AidRequestReport, error) {
	report := &models.AidRequestReport{}

	query := `
		SELECT
			COUNT(*) AS total_requests,
			COUNT(*) FILTER (
				WHERE status = 'Pending'
			) AS pending_requests,
			COUNT(*) FILTER (
				WHERE status = 'Approved'
			) AS approved_requests,
			COUNT(*) FILTER (
				WHERE status = 'Rejected'
			) AS rejected_requests,
			COUNT(*) FILTER (
				WHERE status = 'Cancelled'
			) AS cancelled_requests
		FROM aid_requests
		WHERE deleted_at IS NULL
	`

	if err := r.db.Raw(query).Scan(report).Error; err != nil {
		return nil, err
	}

	return report, nil
}
