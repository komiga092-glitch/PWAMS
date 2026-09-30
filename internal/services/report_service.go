package services

import (
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

// ReportService is the thin service boundary for the Reports module: every
// report area (summary) and every paginated detail view delegates to a
// focused ReportRepository method. No SQL and no HTTP concerns live here.
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

// WithFilter returns a service view whose summary reports honour filter, so a
// report's summary cards describe exactly the rows its detail table shows. An
// empty filter (or a repository already scoped that way) returns the shared
// service unchanged, so unfiltered reports keep their original query plan.
func (s *ReportService) WithFilter(filter models.ReportFilter) (*ReportService, error) {
	reportRepo, err := s.reportRepo.WithFilter(filter)
	if err != nil {
		return nil, err
	}

	if reportRepo == s.reportRepo {
		return s, nil
	}

	return &ReportService{reportRepo: reportRepo}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Summary reports (one per report area)
// ─────────────────────────────────────────────────────────────────────────────

func (s *ReportService) GetDashboardReport() (*models.DashboardReport, error) {
	return s.reportRepo.GetDashboardReport()
}

func (s *ReportService) GetUsersReport() (*models.UsersReport, error) {
	return s.reportRepo.GetUsersReport()
}

func (s *ReportService) GetAccountStatusReport() (*models.AccountStatusReport, error) {
	return s.reportRepo.GetAccountStatusReport()
}

func (s *ReportService) GetPersonReport() (*models.PersonReport, error) {
	return s.reportRepo.GetPersonReport()
}

func (s *ReportService) GetStudentReport() (*models.StudentReport, error) {
	return s.reportRepo.GetStudentReport()
}

func (s *ReportService) GetDonorReport() (*models.DonorReport, error) {
	return s.reportRepo.GetDonorReport()
}

func (s *ReportService) GetDonationReport() (*models.DonationReport, error) {
	return s.reportRepo.GetDonationReport()
}

func (s *ReportService) GetAidRequestReport() (*models.AidRequestReport, error) {
	return s.reportRepo.GetAidRequestReport()
}

func (s *ReportService) GetCareProvidedReport() (*models.CareProvidedReport, error) {
	return s.reportRepo.GetCareProvidedReport()
}

func (s *ReportService) GetLoanReport() (*models.LoanReport, error) {
	return s.reportRepo.GetLoanReport()
}

func (s *ReportService) GetLoanRepaymentReport() (*models.LoanRepaymentReport, error) {
	return s.reportRepo.GetLoanRepaymentReport()
}

func (s *ReportService) GetRevenueReport() (*models.RevenueReport, error) {
	return s.reportRepo.GetRevenueReport()
}

func (s *ReportService) GetAuditLogReport() (*models.AuditLogReport, error) {
	return s.reportRepo.GetAuditLogReport()
}

func (s *ReportService) GetSystemAlertReport() (*models.SystemAlertReport, error) {
	return s.reportRepo.GetSystemAlertReport()
}

// ─────────────────────────────────────────────────────────────────────────────
// Row-level detail views (filter + pagination)
// ─────────────────────────────────────────────────────────────────────────────

func (s *ReportService) GetUserReportRows(filter models.ReportFilter) ([]models.UserReportRow, int64, error) {
	return s.reportRepo.GetUserReportRows(filter)
}

func (s *ReportService) GetPersonReportRows(filter models.ReportFilter) ([]models.PersonReportRow, int64, error) {
	return s.reportRepo.GetPersonReportRows(filter)
}

func (s *ReportService) GetStudentReportRows(filter models.ReportFilter) ([]models.StudentReportRow, int64, error) {
	return s.reportRepo.GetStudentReportRows(filter)
}

func (s *ReportService) GetDonorReportRows(filter models.ReportFilter) ([]models.DonorReportRow, int64, error) {
	return s.reportRepo.GetDonorReportRows(filter)
}

func (s *ReportService) GetDonationReportRows(filter models.ReportFilter) ([]models.DonationReportRow, int64, error) {
	return s.reportRepo.GetDonationReportRows(filter)
}

func (s *ReportService) GetAidRequestReportRows(filter models.ReportFilter) ([]models.AidRequestReportRow, int64, error) {
	return s.reportRepo.GetAidRequestReportRows(filter)
}

func (s *ReportService) GetCareProvidedReportRows(filter models.ReportFilter) ([]models.CareProvidedReportRow, int64, error) {
	return s.reportRepo.GetCareProvidedReportRows(filter)
}

func (s *ReportService) GetLoanReportRows(filter models.ReportFilter) ([]models.LoanReportRow, int64, error) {
	return s.reportRepo.GetLoanReportRows(filter)
}

func (s *ReportService) GetLoanRepaymentReportRows(filter models.ReportFilter) ([]models.LoanRepaymentReportRow, int64, error) {
	return s.reportRepo.GetLoanRepaymentReportRows(filter)
}

func (s *ReportService) GetRevenueReportRows(filter models.ReportFilter) ([]models.RevenueReportRow, int64, error) {
	return s.reportRepo.GetRevenueReportRows(filter)
}

func (s *ReportService) GetAuditLogReportRows(filter models.ReportFilter) ([]models.AuditLogReportRow, int64, error) {
	return s.reportRepo.GetAuditLogReportRows(filter)
}
