package utils

import (
	"strconv"
	"testing"
)

func TestGenerateOTPProducesSixDigitValues(t *testing.T) {
	for range 100 {
		otp, err := GenerateOTP()
		if err != nil {
			t.Fatalf("GenerateOTP() error = %v", err)
		}

		if len(otp) != 6 {
			t.Fatalf("GenerateOTP() = %q, want six digits", otp)
		}

		value, err := strconv.Atoi(otp)
		if err != nil || value < 0 || value > 999999 {
			t.Fatalf("GenerateOTP() = %q, want a value from 000000 to 999999", otp)
		}
	}
}
