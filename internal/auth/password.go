package auth

import (
	"errors"
	"unicode/utf8"
)

func ValidatePassword(password string) error {
	if !utf8.ValidString(password) {
		return errors.New("password is not valid UTF-8")
	}
	if len([]rune(password)) < 10 {
		return errors.New("password must contain at least 10 characters")
	}
	// bcrypt only accepts passwords up to 72 bytes. Rejecting longer values
	// avoids ambiguous truncation behavior across clients and implementations.
	if len([]byte(password)) > 72 {
		return errors.New("password must not exceed 72 bytes")
	}
	return nil
}
