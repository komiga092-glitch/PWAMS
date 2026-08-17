package utils

import (
	"crypto/rand"
	"fmt"
)

func GenerateOTP() (string, error) {
	var number [1]byte

	_, err := rand.Read(number[:])
	if err != nil {
		return "", err
	}

	otpNumber := int(number[0]) % 1000000

	return fmt.Sprintf("%06d", otpNumber), nil
}
