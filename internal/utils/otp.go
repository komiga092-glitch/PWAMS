package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

func GenerateOTP() (string, error) {
	number, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", number.Int64()), nil
}
