package services

import (
	"testing"
	"time"

	"github.com/komiga092-glitch/pwams/internal/constants"
)

func TestParseDateValue(t *testing.T) {
	parsed, err := parseDateValue("2024-01-02")
	if err != nil {
		t.Fatalf("parseDateValue returned unexpected error: %v", err)
	}

	want := time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC)
	if !parsed.Equal(want) {
		t.Fatalf("parseDateValue() = %v, want %v", parsed, want)
	}
}

func TestParseDateValueUsesDefaultLayout(t *testing.T) {
	parsed, err := parseDateValue("2024-01-02")
	if err != nil {
		t.Fatalf("parseDateValue returned unexpected error: %v", err)
	}

	if parsed.Format(constants.DateLayout) != "2024-01-02" {
		t.Fatalf("parseDateValue() produced %q, want date in layout %q", parsed.Format(constants.DateLayout), constants.DateLayout)
	}
}
