package services

import (
	"errors"
	"regexp"
)

var ErrInvalidPhone = errors.New("invalid phone number")

var phonePattern = regexp.MustCompile(`^\+?[0-9][0-9 ()-]{5,19}$`)

func isValidPhone(value string) bool {
	return value == "" || phonePattern.MatchString(value)
}
