package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
)

const (
	bcryptCost = 12
	minPasswordLength = 8
)

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	if len(password) < minPasswordLength {
		return "", ErrPasswordTooShort
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// CheckPassword compares a password with its hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
