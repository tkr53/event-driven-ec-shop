package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword_ValidPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"8 characters", "password"},
		{"long password", "this-is-a-very-long-password-123!@#"},
		{"with special chars", "p@ssw0rd!"},
		{"with unicode", "パスワード12345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			require.NoError(t, err)
			assert.NotEmpty(t, hash)
			assert.NotEqual(t, tt.password, hash)

			// Verify the hash is valid bcrypt format
			assert.True(t, len(hash) >= 60, "bcrypt hash should be at least 60 chars")
		})
	}
}

func TestHashPassword_ShortPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"7 characters", "1234567"},
		{"empty", ""},
		{"1 character", "a"},
		{"spaces only", "       "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			assert.ErrorIs(t, err, ErrPasswordTooShort)
			assert.Empty(t, hash)
		})
	}
}

func TestHashPassword_DifferentHashesForSamePassword(t *testing.T) {
	password := "testpassword123"

	hash1, err := HashPassword(password)
	require.NoError(t, err)

	hash2, err := HashPassword(password)
	require.NoError(t, err)

	// bcrypt generates different hashes due to random salt
	assert.NotEqual(t, hash1, hash2)
}

func TestCheckPassword_CorrectPassword(t *testing.T) {
	password := "correctpassword"

	hash, err := HashPassword(password)
	require.NoError(t, err)

	result := CheckPassword(password, hash)
	assert.True(t, result)
}

func TestCheckPassword_WrongPassword(t *testing.T) {
	password := "correctpassword"
	wrongPassword := "wrongpassword"

	hash, err := HashPassword(password)
	require.NoError(t, err)

	result := CheckPassword(wrongPassword, hash)
	assert.False(t, result)
}

func TestCheckPassword_EmptyPassword(t *testing.T) {
	hash, err := HashPassword("validpassword")
	require.NoError(t, err)

	result := CheckPassword("", hash)
	assert.False(t, result)
}

func TestCheckPassword_InvalidHash(t *testing.T) {
	result := CheckPassword("password", "invalid-hash")
	assert.False(t, result)
}

func TestCheckPassword_EmptyHash(t *testing.T) {
	result := CheckPassword("password", "")
	assert.False(t, result)
}

func TestCheckPassword_CaseSensitive(t *testing.T) {
	password := "Password123"

	hash, err := HashPassword(password)
	require.NoError(t, err)

	// Verify case sensitivity
	assert.True(t, CheckPassword("Password123", hash))
	assert.False(t, CheckPassword("password123", hash))
	assert.False(t, CheckPassword("PASSWORD123", hash))
}
