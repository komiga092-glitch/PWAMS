package services

import (
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

type ReportService struct {
	reportRepo *repository.ReportRepository
}

func NewReportService(
	reportRepo *repository.ReportRepository,
) *ReportService {
	return &ReportService{
		reportRepo: reportRepo,
	}
}

func (s *ReportService) GetDashboardReport() (
	*models.DashboardReport,
	error,
) {
	return s.reportRepo.GetDashboardReport()
}

func (s *ReportService) GetDonationReport() (
	*models.DonationReport,
	error,
) {
	return s.reportRepo.GetDonationReport()
}

func (s *ReportService) GetAidRequestReport() (
	*models.AidRequestReport,
	error,
) {
	return s.reportRepo.GetAidRequestReport()
}
