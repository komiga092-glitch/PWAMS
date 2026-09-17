package i18n

import (
	"testing"
)

func TestT_English(t *testing.T) {
	got := T("en", "profile.title")
	want := "My Profile"
	if got != want {
		t.Errorf("T(en, profile.title) = %q, want %q", got, want)
	}
}

func TestT_Tamil(t *testing.T) {
	got := T("ta", "profile.title")
	want := "எனது சுயவிவரம்"
	if got != want {
		t.Errorf("T(ta, profile.title) = %q, want %q", got, want)
	}
}

func TestT_Sinhala(t *testing.T) {
	got := T("si", "profile.title")
	want := "මගේ පැතිකඩ"
	if got != want {
		t.Errorf("T(si, profile.title) = %q, want %q", got, want)
	}
}

func TestT_FallbackToEnglish(t *testing.T) {
	// Key exists in English but not in Tamil; should fall back to English.
	// We use a key that we know is missing from Tamil by temporarily checking.
	// Since all keys are present in all dictionaries, we test the fallback
	// path by using an unsupported language.
	got := T("xx", "profile.title")
	want := "My Profile"
	if got != want {
		t.Errorf("T(xx, profile.title) = %q, want %q (English fallback)", got, want)
	}
}

func TestT_KeyNotFound(t *testing.T) {
	// Missing key should return the key itself.
	got := T("en", "nonexistent.key")
	want := "nonexistent.key"
	if got != want {
		t.Errorf("T(en, nonexistent.key) = %q, want %q", got, want)
	}
}

func TestT_EmptyLanguage(t *testing.T) {
	// Empty language should fall back to English.
	got := T("", "profile.title")
	want := "My Profile"
	if got != want {
		t.Errorf("T('', profile.title) = %q, want %q", got, want)
	}
}

func TestT_CaseInsensitive(t *testing.T) {
	// Language codes should be case-insensitive.
	got := T("EN", "profile.title")
	want := "My Profile"
	if got != want {
		t.Errorf("T(EN, profile.title) = %q, want %q", got, want)
	}
}

func TestT_WhitespaceTrimmed(t *testing.T) {
	got := T("  en  ", "profile.title")
	want := "My Profile"
	if got != want {
		t.Errorf("T('  en  ', profile.title) = %q, want %q", got, want)
	}
}

func TestT_PartnerRoleTranslated(t *testing.T) {
	// "Partner" role should display as "Manager" in English.
	got := T("en", "role.partner")
	want := "Manager"
	if got != want {
		t.Errorf("T(en, role.partner) = %q, want %q", got, want)
	}
}

func TestT_PartnerRoleTamil(t *testing.T) {
	got := T("ta", "role.partner")
	want := "மேலாளர்"
	if got != want {
		t.Errorf("T(ta, role.partner) = %q, want %q", got, want)
	}
}

func TestT_PartnerRoleSinhala(t *testing.T) {
	got := T("si", "role.partner")
	want := "කළමනාකරු"
	if got != want {
		t.Errorf("T(si, role.partner) = %q, want %q", got, want)
	}
}

func TestSupportedLanguages(t *testing.T) {
	langs := SupportedLanguages()
	if len(langs) != 3 {
		t.Errorf("SupportedLanguages() returned %d languages, want 3", len(langs))
	}
	for _, code := range []string{"en", "ta", "si"} {
		if _, ok := langs[code]; !ok {
			t.Errorf("SupportedLanguages() missing language %q", code)
		}
	}
}

func TestIsSupportedLanguage(t *testing.T) {
	tests := []struct {
		lang     string
		expected bool
	}{
		{"en", true},
		{"ta", true},
		{"si", true},
		{"EN", true}, // case-insensitive
		{"xx", false},
		{"", false},
		{"fr", false},
	}
	for _, tt := range tests {
		got := IsSupportedLanguage(tt.lang)
		if got != tt.expected {
			t.Errorf("IsSupportedLanguage(%q) = %v, want %v", tt.lang, got, tt.expected)
		}
	}
}

func TestEffectiveLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"en", "en"},
		{"ta", "ta"},
		{"si", "si"},
		{"EN", "en"},     // normalized to lowercase
		{"  ta  ", "ta"}, // trimmed
		{"", "en"},       // empty → default
		{"xx", "en"},     // unsupported → default
		{"fr", "en"},     // unsupported → default
	}
	for _, tt := range tests {
		got := EffectiveLanguage(tt.input)
		if got != tt.expected {
			t.Errorf("EffectiveLanguage(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestAllKeysPresentInAllLanguages(t *testing.T) {
	// Every key in English must exist in Tamil and Sinhala.
	for key := range english {
		if _, ok := tamil[key]; !ok {
			t.Errorf("Tamil translation missing key %q", key)
		}
		if _, ok := sinhala[key]; !ok {
			t.Errorf("Sinhala translation missing key %q", key)
		}
	}
}

// TestLoanRepaymentKeysExistInAllLanguages pins the Loan Details / Repayment
// UI contract: every user-facing string used by loan_details.html,
// loan-details.js and the repay modal must resolve (non-fallback) in English,
// Tamil AND Sinhala. The generic TestAllKeysPresentInAllLanguages guards
// dictionary parity; this test guards that the specific keys the loan UI
// references were not dropped from any language.
func TestLoanRepaymentKeysExistInAllLanguages(t *testing.T) {
	loanKeys := []string{
		// Loan information panel.
		"loan.details_title", "loan.back_to_list", "loan.information",
		"loan.borrower", "loan.amount", "loan.interest_rate",
		"loan.tenure", "loan.months", "loan.start_date", "loan.due_day",
		"loan.status",
		// Financial summary.
		"loan.financial_summary", "loan.principal", "loan.total_interest",
		"loan.total_repayable", "loan.total_paid", "loan.outstanding_amount",
		"loan.monthly_installment", "loan.next_payment", "loan.next_due_date",
		// Repayment schedule.
		"loan.repayment_schedule", "loan.installment",
		"loan.installment_number", "loan.due_date", "loan.interest",
		"loan.amount_due", "loan.paid_amount", "loan.outstanding",
		"loan.action", "loan.no_repayments",
		// Repay modal.
		"loan.repay", "loan.repay_installment", "loan.payment_date",
		"loan.payment_amount", "loan.payment_method", "loan.reference",
		"loan.notes", "loan.submit_payment", "common.cancel",
		// Statuses.
		"status.pending", "status.partially_paid", "status.paid",
		"status.overdue", "loan.fully_paid",
		// Success / error feedback.
		"loan.payment_successful", "loan.invalid_payment_amount",
		"loan.installment_already_paid", "loan.loan_completed",
		// Payment method labels.
		"payment_method.cash", "payment_method.bank_transfer",
		"payment_method.cheque", "payment_method.card",
		"payment_method.other",
	}

	for _, lang := range []string{LangEnglish, LangTamil, LangSinhala} {
		for _, key := range loanKeys {
			if _, ok := translations[lang][key]; !ok {
				t.Errorf("language %q missing loan/repayment key %q", lang, key)
			}
		}
	}
}

func TestNoEmptyTranslations(t *testing.T) {
	for lang, dict := range translations {
		for key, val := range dict {
			if val == "" {
				t.Errorf("Empty translation for key %q in language %q", key, lang)
			}
		}
	}
}
