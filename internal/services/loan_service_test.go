package services_test

import (
	"testing"

	"github.com/komiga092-glitch/pwams/internal/services"
	"github.com/shopspring/decimal"
)

func TestCalculateLoanInstallment(t *testing.T) {
	tests := []struct {
		name           string
		amount         decimal.Decimal
		interestRate   decimal.Decimal
		durationMonths int
		expected       decimal.Decimal
	}{
		{
			name:           "no interest, 12 months",
			amount:         decimal.NewFromInt(12000),
			interestRate:   decimal.Zero,
			durationMonths: 12,
			expected:       decimal.NewFromInt(1000),
		},
		{
			name:           "10% interest, 10 months",
			amount:         decimal.NewFromInt(10000),
			interestRate:   decimal.NewFromInt(10),
			durationMonths: 10,
			expected:       decimal.NewFromInt(1100),
		},
		{
			name:           "5% interest, 6 months",
			amount:         decimal.NewFromInt(6000),
			interestRate:   decimal.NewFromInt(5),
			durationMonths: 6,
			expected:       decimal.NewFromInt(1050),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := services.CalculateLoanInstallmentPublic(tt.amount, tt.interestRate, tt.durationMonths)
			if !result.Equal(tt.expected) {
				t.Errorf("CalculateLoanInstallment = %v, want %v", result, tt.expected)
			}
		})
	}
}
