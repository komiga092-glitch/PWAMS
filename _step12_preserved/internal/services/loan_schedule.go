package services

import (
	"time"

	"github.com/shopspring/decimal"
)

// LoanFinancialPlan is the server-side decomposition of a flat-interest
// loan. Every amount is stored at the database monetary precision
// (NUMERIC(15,2)). Division by the tenure is performed with decimal
// arithmetic; any rounding remainder is absorbed by the final installments
// at schedule-build time (see BuildLoanSchedule) so the invariants
//
//	SUM(installment amounts)   == TotalRepayable
//	SUM(principal amounts)     == Loan Amount
//	SUM(interest amounts)      == TotalInterest
//
// hold exactly.
type LoanFinancialPlan struct {
	TotalInterest      decimal.Decimal
	TotalRepayable     decimal.Decimal
	MonthlyInstallment decimal.Decimal
	MonthlyPrincipal   decimal.Decimal
	MonthlyInterest    decimal.Decimal
}

// CalculateLoanFinancialPlan computes the flat-interest loan decomposition:
//
//	Total Interest   = Amount x (Rate / 100) x (Months / 12)
//	Total Repayable  = Amount + Total Interest
//	Monthly Installment = Total Repayable / Months
//	Monthly Principal   = Amount / Months
//	Monthly Interest    = Total Interest / Months
//
// All values are rounded to 2 decimal places (the stored monetary
// precision). The per-installment remainder handling in BuildLoanSchedule
// guarantees the sums above regardless of rounding.
func CalculateLoanFinancialPlan(
	amount decimal.Decimal,
	interestRate decimal.Decimal,
	durationMonths int,
) LoanFinancialPlan {
	if durationMonths < 1 {
		durationMonths = 1
	}

	months := decimal.NewFromInt(int64(durationMonths))

	totalInterest := amount.
		Mul(interestRate).
		Div(decimal.NewFromInt(100)).
		Mul(months).
		Div(decimal.NewFromInt(12)).
		Round(2)
	if totalInterest.LessThan(decimal.Zero) {
		totalInterest = decimal.Zero
	}

	totalRepayable := amount.Add(totalInterest).Round(2)
	monthlyInstallment := totalRepayable.Div(months).Round(2)
	monthlyPrincipal := amount.Div(months).Round(2)
	monthlyInterest := totalInterest.Div(months).Round(2)

	// The base monthly figures are derived figures; they never feed the
	// schedule invariant directly (the final installment absorbs any
	// remainder), but clamp to non-negative defensively.
	if monthlyPrincipal.LessThan(decimal.Zero) {
		monthlyPrincipal = decimal.Zero
	}
	if monthlyInterest.LessThan(decimal.Zero) {
		monthlyInterest = decimal.Zero
	}

	return LoanFinancialPlan{
		TotalInterest:      totalInterest,
		TotalRepayable:     totalRepayable,
		MonthlyInstallment: monthlyInstallment,
		MonthlyPrincipal:   monthlyPrincipal,
		MonthlyInterest:    monthlyInterest,
	}
}

// LoanScheduleEntry is one scheduled installment.
type LoanScheduleEntry struct {
	InstallmentNumber int
	DueDate           time.Time
	PrincipalAmount   decimal.Decimal
	InterestAmount    decimal.Decimal
	TotalAmount       decimal.Decimal
}

// GenerateLoanSchedule builds the full monthly repayment schedule for a
// flat-interest loan. startDate anchors installment 1's due date and dueDay
// (1-31) overrides the day-of-month; 0 keeps startDate's day. Subsequent
// due dates use calendar-month arithmetic (clamped to the last valid day of
// each month) — never a hardcoded 30-day span.
func GenerateLoanSchedule(
	amount decimal.Decimal,
	interestRate decimal.Decimal,
	durationMonths int,
	startDate time.Time,
	dueDay int,
) []LoanScheduleEntry {
	plan := CalculateLoanFinancialPlan(amount, interestRate, durationMonths)
	return BuildLoanSchedule(plan, amount, durationMonths, startDate, dueDay)
}

// BuildLoanSchedule converts a financial plan into concrete installments.
// The final installment absorbs the rounding remainder so the three sum
// invariants hold at exact NUMERIC(15,2) precision.
func BuildLoanSchedule(
	plan LoanFinancialPlan,
	amount decimal.Decimal,
	durationMonths int,
	startDate time.Time,
	dueDay int,
) []LoanScheduleEntry {
	if durationMonths < 1 {
		durationMonths = 1
	}

	months := decimal.NewFromInt(int64(durationMonths))
	basePrincipal := plan.MonthlyPrincipal
	baseInterest := plan.MonthlyInterest

	// Reconstruct the principal so the last installment absorbs the
	// division remainder without ever relying on the derived figure.
	principalTotal := plan.TotalRepayable.Sub(plan.TotalInterest)
	if principalTotal.LessThan(decimal.Zero) {
		principalTotal = amount
	}

	firstDue := firstDueDate(startDate, dueDay)

	entries := make([]LoanScheduleEntry, 0, durationMonths)
	for i := 0; i < durationMonths; i++ {
		principal := basePrincipal
		interest := baseInterest

		if i == durationMonths-1 {
			principal = principalTotal.Sub(
				basePrincipal.Mul(months.Sub(decimal.NewFromInt(1))),
			)
			interest = plan.TotalInterest.Sub(
				baseInterest.Mul(months.Sub(decimal.NewFromInt(1))),
			)
		}

		if principal.LessThan(decimal.Zero) {
			principal = decimal.Zero
		}
		if interest.LessThan(decimal.Zero) {
			interest = decimal.Zero
		}

		entries = append(entries, LoanScheduleEntry{
			InstallmentNumber: i + 1,
			DueDate:           AddCalendarMonths(firstDue, i),
			PrincipalAmount:   principal,
			InterestAmount:    interest,
			TotalAmount:       principal.Add(interest),
		})
	}

	return entries
}

// firstDueDate resolves the base date for installment 1. When dueDay is
// valid it is used as the day-of-month (clamped to the month length);
// otherwise the start date itself is used.
func firstDueDate(startDate time.Time, dueDay int) time.Time {
	if dueDay < 1 {
		return startDate
	}

	year, month, _ := startDate.Date()
	last := daysInMonth(year, month)
	day := dueDay
	if day > last {
		day = last
	}
	return time.Date(year, month, day, 0, 0, 0, 0, startDate.Location())
}

// AddCalendarMonths adds whole calendar months to t, clamping the day to
// the last valid day of the target month (e.g. Jan 31 + 1 month = Feb
// 28/29). The wall-clock time of day is preserved.
func AddCalendarMonths(t time.Time, months int) time.Time {
	year, month, day := t.Date()
	total := int(month) - 1 + months
	targetYear := year + total/12
	targetMonth := time.Month(total%12 + 1)

	last := daysInMonth(targetYear, targetMonth)
	if day > last {
		day = last
	}

	return time.Date(targetYear, targetMonth, day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// daysInMonth returns the number of days in the given month.
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
