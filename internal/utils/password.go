package utils

import "golang.org/x/crypto/bcrypt"

// HashPassword converts a plain password into a bcrypt hash.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)

	return string(bytes), err
}

// CheckPassword compares a bcrypt hash with a plain password.
func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword(
		[]byte(hash),
		[]byte(password),
	)
}
