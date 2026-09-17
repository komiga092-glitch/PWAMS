package handlers

import (
	"bytes"
	"html/template"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/komiga092-glitch/pwams/internal/i18n"
	"github.com/komiga092-glitch/pwams/internal/models"
)

// renderLoanDetails parses and renders the REAL loan_details.html template so
// a structural regression ("unexpected {{end}}", a missing data key, a nil
// pointer on a nullable field) fails the test suite instead of a route 500.
func renderLoanDetails(t *testing.T, data TemplateData) string {
	t.Helper()

	const path = "../../web/templates/loan_details.html"
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	tmpl, err := template.New("loan_details_test_root").Parse(string(content))
	if err != nil {
		t.Fatalf("loan_details.html failed to parse: %v (template parse panic)", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Lookup("loan_details_content").Execute(&buf, data); err != nil {
		t.Fatalf("failed to render loan_details_content: %v", err)
	}
	return buf.String()
}

func loanDetailsTemplateData() TemplateData {
	person := models.Person{
		ID:          uuid.New(),
		FullName:    "Test Borrower",
		NICPassport: "LL123",
	}
	loan := &models.Loan{
		ID:                   uuid.New(),
		PersonID:             person.ID,
		Person:               person,
		LoanAmount:           decimal.NewFromInt(12000),
		InterestRate:         decimal.NewFromInt(10),
		DurationMonths:       12,
		InstallmentAmount:    decimal.RequireFromString("1100.00"),
		TotalInterest:        decimal.NewFromInt(1200),
		TotalRepayableAmount: decimal.NewFromInt(13200),
		TotalPaidAmount:      decimal.Zero,
		OutstandingAmount:    decimal.NewFromInt(13200),
		StartDate:            ptrTime(time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)),
		DueDay:               15,
		Status:               models.LoanStatusActive,
	}

	now := time.Now().UTC()
	schedule := make([]models.LoanRepayment, 0, 12)
	for i := 1; i <= 12; i++ {
		schedule = append(schedule, models.LoanRepayment{
			ID:                uuid.New(),
			LoanID:            loan.ID,
			InstallmentNumber: i,
			DueDate:           time.Date(2026, time.Month(i), 15, 0, 0, 0, 0, time.UTC),
			Amount:            decimal.RequireFromString("1100.00"),
			PrincipalAmount:   decimal.NewFromInt(1000),
			InterestAmount:    decimal.NewFromInt(100),
			OutstandingAmount: decimal.RequireFromString("1100.00"),
			PaidAmount:        decimal.Zero,
			Status:            models.RepaymentStatusPending,
		})
	}

	// Mirror the handler exactly: payability comes from the same
	// isRepayable business-state check ViewPage uses, so the template test
	// exercises the production visibility rule (active loan + payable
	// status + outstanding balance), not a hand-rolled approximation.
	handler := &LoanHandler{}
	payable := make(map[string]bool, len(schedule))
	for i := range schedule {
		payable[schedule[i].ID.String()] = handler.isRepayable(loan, &schedule[i])
	}

	return TemplateData{
		"current_language":  i18n.DefaultLanguage,
		"loan":              loan,
		"schedule":          schedule,
		"nextPaymentAmount": decimal.RequireFromString("1100.00"),
		"nextDueDate":       &now,
		"installmentAmount": decimal.RequireFromString("1100.00"),
		"payable":           payable,
		"user_permissions": map[string]bool{
			"repayment.create": true,
		},
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

// TestLoanDetailsTemplate_RendersFinancialSummary verifies the financial
// summary block renders the principal, interest, repayable, paid and
// outstanding figures.
func TestLoanDetailsTemplate_RendersFinancialSummary(t *testing.T) {
	html := renderLoanDetails(t, loanDetailsTemplateData())

	for _, want := range []string{
		"12000.00", // principal / loan amount
		"1200.00",  // total interest
		"13200.00", // total repayable / outstanding
		"0.00",     // total paid
		"1100.00",  // monthly installment
	} {
		if !strings.Contains(html, want) {
			t.Errorf("financial summary should render %q", want)
		}
	}
}

// TestLoanDetailsTemplate_RendersRepaymentSchedule verifies every schedule
// row including installment number, amounts and status appears.
func TestLoanDetailsTemplate_RendersRepaymentSchedule(t *testing.T) {
	html := renderLoanDetails(t, loanDetailsTemplateData())

	for _, want := range []string{
		"#1", "#12", "2026-01-15", "2026-12-15",
		"1000.00", // principal per row
		"100.00",  // interest per row
	} {
		if !strings.Contains(html, want) {
			t.Errorf("repayment schedule should render %q", want)
		}
	}
	if !strings.Contains(html, `data-schedule-body`) {
		t.Error("repayment schedule tbody should carry data-schedule-body")
	}
}

// TestLoanDetailsTemplate_RepayButtonAuthorizedOnly verifies the Repay action
// only renders when the user holds repayment.create AND the installment is
// payable.
func TestLoanDetailsTemplate_RepayButtonAuthorizedOnly(t *testing.T) {
	base := loanDetailsTemplateData()

	t.Run("authorized and payable", func(t *testing.T) {
		html := renderLoanDetails(t, base)
		if !strings.Contains(html, `data-repay-open`) {
			t.Error("repay button should render for an authorized, payable installment")
		}
	})

	t.Run("no repayment.create permission", func(t *testing.T) {
		noPerm := TemplateData(base)
		noPerm["user_permissions"] = map[string]bool{}
		html := renderLoanDetails(t, noPerm)
		if strings.Contains(html, `data-repay-open`) {
			t.Error("repay button must not render without repayment.create")
		}
	})

	t.Run("installment not payable", func(t *testing.T) {
		notPayable := TemplateData(base)
		notPayable["payable"] = map[string]bool{}
		html := renderLoanDetails(t, notPayable)
		if strings.Contains(html, `data-repay-open`) {
			t.Error("repay button must not render for a non-payable installment")
		}
	})
}

// TestLoanDetailsTemplate_FullyPaidLoanDisplaysFullyPaid verifies a completed
// loan renders the localized Fully Paid status instead of the raw English
// internal status string.
func TestLoanDetailsTemplate_FullyPaidLoanDisplaysFullyPaid(t *testing.T) {
	data := loanDetailsTemplateData()
	loan := data["loan"].(*models.Loan)
	loan.Status = models.LoanStatusCompleted
	loan.OutstandingAmount = decimal.Zero
	loan.TotalPaidAmount = decimal.NewFromInt(13200)

	// ViewPage recomputes payability on every render; mirror that so the
	// completed-loan state (not the fixture's original active state)
	// drives button visibility exactly as production does.
	handler := &LoanHandler{}
	schedule := data["schedule"].([]models.LoanRepayment)
	recomputed := make(map[string]bool, len(schedule))
	for i := range schedule {
		recomputed[schedule[i].ID.String()] = handler.isRepayable(loan, &schedule[i])
	}
	data["payable"] = recomputed

	html := renderLoanDetails(t, data)

	if !strings.Contains(html, "Fully Paid") {
		t.Error("completed loan should render the localized Fully Paid label")
	}
	if !strings.Contains(html, "badge badge-success") {
		t.Error("completed loan should render a success badge")
	}
	if strings.Contains(html, `data-repay-open`) {
		t.Error("completed loan must not render repay buttons")
	}
}
