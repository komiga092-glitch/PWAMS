package services

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrInvalidRevenueID       = errors.New("invalid revenue record id")
	ErrInvalidRevenueType     = errors.New("invalid revenue record type")
	ErrInvalidRevenueCategory = errors.New("invalid revenue category")
	ErrInvalidRevenueAmount   = errors.New("revenue amount must be greater than zero")
	ErrInvalidRevenueDate     = errors.New("invalid revenue record date")
	ErrRevenueReferenceExists = errors.New("revenue reference number already exists")
)

func validRevenueType(value string) bool {
	return value == models.RevenueTypeIncome || value == models.RevenueTypeExpense
}

func validRevenueCategory(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))

	switch normalized {
	case strings.ToLower(models.RevenueCategoryDonations),
		strings.ToLower(models.RevenueCategoryLoanRepayments),
		strings.ToLower(models.RevenueCategoryGrants),
		strings.ToLower(models.RevenueCategoryAdministrativeExpense),
		strings.ToLower(models.RevenueCategoryWelfareExpense):
		return true
	default:
		return false
	}
}

type RevenueService struct{ repo *repository.RevenueRepository }

func NewRevenueService(repo *repository.RevenueRepository) *RevenueService {
	return &RevenueService{repo: repo}
}

func parseRevenueDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}

func (s *RevenueService) Create(request models.CreateRevenueRecordRequest, createdByID uuid.UUID) (*models.RevenueRecord, error) {
	record, err := s.buildRecord(request.RecordType, request.Category, request.Amount, request.Currency, request.RecordDate, request.Description, request.ReferenceNo, createdByID)
	if err != nil {
		return nil, err
	}
	exists, err := s.repo.ReferenceExists(record.ReferenceNo, "")
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrRevenueReferenceExists
	}
	if err := s.repo.Create(record); err != nil {
		return nil, err
	}
	return s.repo.FindByID(record.ID.String())
}

func (s *RevenueService) buildRecord(recordType, category string, amount decimal.Decimal, currency, recordDate, description, reference string, createdByID uuid.UUID) (*models.RevenueRecord, error) {
	recordType = strings.ToLower(strings.TrimSpace(recordType))
	category = strings.TrimSpace(category)
	if !validRevenueType(recordType) {
		return nil, ErrInvalidRevenueType
	}
	if !validRevenueCategory(category) {
		return nil, ErrInvalidRevenueCategory
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidRevenueAmount
	}
	date, err := parseRevenueDate(recordDate)
	if err != nil || date.After(time.Now().UTC()) {
		return nil, ErrInvalidRevenueDate
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "LKR"
	}
	return &models.RevenueRecord{ID: uuid.New(), RecordType: recordType, Category: category, Amount: amount, Currency: strings.ToUpper(strings.TrimSpace(currency)), RecordDate: date, Description: strings.TrimSpace(description), ReferenceNo: strings.TrimSpace(reference), CreatedByID: createdByID}, nil
}

func (s *RevenueService) GetByID(id string) (*models.RevenueRecord, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return nil, ErrInvalidRevenueID
	}
	return s.repo.FindByID(id)
}

func (s *RevenueService) Update(id string, request models.UpdateRevenueRecordRequest) (*models.RevenueRecord, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return nil, ErrInvalidRevenueID
	}
	record, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	updated, err := s.buildRecord(request.RecordType, request.Category, request.Amount, request.Currency, request.RecordDate, request.Description, request.ReferenceNo, record.CreatedByID)
	if err != nil {
		return nil, err
	}
	record.RecordType, record.Category, record.Amount, record.Currency = updated.RecordType, updated.Category, updated.Amount, updated.Currency
	record.RecordDate, record.Description, record.ReferenceNo = updated.RecordDate, updated.Description, updated.ReferenceNo
	exists, err := s.repo.ReferenceExists(record.ReferenceNo, record.ID.String())
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrRevenueReferenceExists
	}
	if err := s.repo.Update(record); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *RevenueService) Delete(id string) error {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return ErrInvalidRevenueID
	}
	return s.repo.Delete(id)
}

func (s *RevenueService) List(query models.RevenueListQuery) ([]models.RevenueRecord, int64, int, int, error) {
	page, pageSize := query.Page, query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if query.RecordType != "" && !validRevenueType(strings.ToLower(strings.TrimSpace(query.RecordType))) {
		return nil, 0, page, pageSize, ErrInvalidRevenueType
	}
	if query.Category != "" && !validRevenueCategory(strings.TrimSpace(query.Category)) {
		return nil, 0, page, pageSize, ErrInvalidRevenueCategory
	}
	query.Page, query.PageSize = page, pageSize
	fromDate, toDate, err := parseRevenueRange(query.FromDate, query.ToDate)
	if err != nil {
		return nil, 0, page, pageSize, ErrInvalidRevenueDate
	}
	if !fromDate.IsZero() {
		query.FromDate = fromDate.Format(time.RFC3339)
	}
	if !toDate.IsZero() {
		query.ToDate = toDate.Format(time.RFC3339)
	}
	records, total, err := s.repo.List(query)
	return records, total, page, pageSize, err
}

func parseRevenueRange(from, to string) (time.Time, time.Time, error) {
	start := time.Time{}
	end := time.Time{}
	var err error
	if strings.TrimSpace(from) != "" {
		start, err = parseRevenueDate(from)
		if err != nil {
			return start, end, err
		}
	}
	if strings.TrimSpace(to) != "" {
		end, err = parseRevenueDate(to)
		if err != nil {
			return start, end, err
		}
		end = end.AddDate(0, 0, 1)
	}
	if !start.IsZero() && !end.IsZero() && end.Before(start) {
		return start, end, errors.New("invalid revenue date range")
	}
	return start, end, nil
}

func (s *RevenueService) Summaries(now time.Time) ([]models.RevenueSummary, error) {
	now = now.UTC()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	year := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	periods := []struct {
		name       string
		start, end time.Time
	}{{"daily", day, day.AddDate(0, 0, 1)}, {"monthly", month, month.AddDate(0, 1, 0)}, {"annual", year, year.AddDate(1, 0, 0)}}
	result := make([]models.RevenueSummary, 0, len(periods))
	for _, period := range periods {
		summary, err := s.repo.Summary(period.name, period.start, period.end)
		if err != nil {
			return nil, err
		}
		result = append(result, *summary)
	}
	return result, nil
}
