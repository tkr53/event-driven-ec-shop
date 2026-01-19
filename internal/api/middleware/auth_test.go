package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/ec-event-driven/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestJWTService() *auth.JWTService {
	return auth.NewJWTService("test-secret-key", 15*time.Minute, 7*24*time.Hour)
}

func TestAuthMiddleware_ValidToken_Header(t *testing.T) {
	jwtService := newTestJWTService()
	middleware := AuthMiddleware(jwtService)

	// Generate a valid token
	token, _, err := jwtService.GenerateAccessToken("user-123", "test@example.com", "customer")
	require.NoError(t, err)

	// Create test handler that captures the context
	var capturedClaims *auth.Claims
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(UserContextKey).(*auth.Claims)
		if ok {
			capturedClaims = claims
		}
		w.WriteHeader(http.StatusOK)
	})

	// Create request with Authorization header
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, capturedClaims)
	assert.Equal(t, "user-123", capturedClaims.UserID)
	assert.Equal(t, "test@example.com", capturedClaims.Email)
	assert.Equal(t, "customer", capturedClaims.Role)
}

func TestAuthMiddleware_ValidToken_Cookie(t *testing.T) {
	jwtService := newTestJWTService()
	middleware := AuthMiddleware(jwtService)

	token, _, err := jwtService.GenerateAccessToken("user-456", "cookie@example.com", "admin")
	require.NoError(t, err)

	var capturedClaims *auth.Claims
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(UserContextKey).(*auth.Claims)
		if ok {
			capturedClaims = claims
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, capturedClaims)
	assert.Equal(t, "user-456", capturedClaims.UserID)
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	jwtService := newTestJWTService()
	middleware := AuthMiddleware(jwtService)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthorized")
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	jwtService := newTestJWTService()
	middleware := AuthMiddleware(jwtService)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid token")
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	// Create service with very short expiry
	jwtService := auth.NewJWTService("test-secret", 1*time.Millisecond, 7*24*time.Hour)
	middleware := AuthMiddleware(jwtService)

	token, _, err := jwtService.GenerateAccessToken("user-123", "test@example.com", "customer")
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_WrongSignature(t *testing.T) {
	jwtService1 := auth.NewJWTService("secret-1", 15*time.Minute, 7*24*time.Hour)
	jwtService2 := auth.NewJWTService("secret-2", 15*time.Minute, 7*24*time.Hour)

	// Generate token with service1
	token, _, err := jwtService1.GenerateAccessToken("user-123", "test@example.com", "customer")
	require.NoError(t, err)

	// Validate with service2
	middleware := AuthMiddleware(jwtService2)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_CookieTakesPrecedence(t *testing.T) {
	jwtService := newTestJWTService()
	middleware := AuthMiddleware(jwtService)

	// Generate two different tokens
	cookieToken, _, _ := jwtService.GenerateAccessToken("cookie-user", "cookie@example.com", "customer")
	headerToken, _, _ := jwtService.GenerateAccessToken("header-user", "header@example.com", "admin")

	var capturedClaims *auth.Claims
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(UserContextKey).(*auth.Claims)
		if ok {
			capturedClaims = claims
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: cookieToken})
	req.Header.Set("Authorization", "Bearer "+headerToken)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, capturedClaims)
	// Cookie should take precedence
	assert.Equal(t, "cookie-user", capturedClaims.UserID)
}

// ============================================
// Optional Auth Middleware Tests
// ============================================

func TestOptionalAuthMiddleware_ValidToken(t *testing.T) {
	jwtService := newTestJWTService()
	middleware := OptionalAuthMiddleware(jwtService)

	token, _, _ := jwtService.GenerateAccessToken("user-123", "test@example.com", "customer")

	var capturedClaims *auth.Claims
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(UserContextKey).(*auth.Claims)
		if ok {
			capturedClaims = claims
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/optional", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, capturedClaims)
	assert.Equal(t, "user-123", capturedClaims.UserID)
}

func TestOptionalAuthMiddleware_NoToken(t *testing.T) {
	jwtService := newTestJWTService()
	middleware := OptionalAuthMiddleware(jwtService)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		// Should have no claims
		_, ok := r.Context().Value(UserContextKey).(*auth.Claims)
		assert.False(t, ok)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/optional", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, handlerCalled)
}

func TestOptionalAuthMiddleware_InvalidToken(t *testing.T) {
	jwtService := newTestJWTService()
	middleware := OptionalAuthMiddleware(jwtService)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		// Should have no claims (invalid token is ignored)
		_, ok := r.Context().Value(UserContextKey).(*auth.Claims)
		assert.False(t, ok)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/optional", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, handlerCalled)
}

// ============================================
// Require Role Middleware Tests
// ============================================

func TestRequireRole_HasRole(t *testing.T) {
	middleware := RequireRole("admin", "moderator")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create context with admin claims
	claims := &auth.Claims{
		UserID: "user-123",
		Email:  "admin@example.com",
		Role:   "admin",
	}
	ctx := context.WithValue(context.Background(), UserContextKey, claims)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRole_HasAlternateRole(t *testing.T) {
	middleware := RequireRole("admin", "moderator")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	claims := &auth.Claims{
		UserID: "user-123",
		Role:   "moderator",
	}
	ctx := context.WithValue(context.Background(), UserContextKey, claims)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRole_NoRole(t *testing.T) {
	middleware := RequireRole("admin")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	claims := &auth.Claims{
		UserID: "user-123",
		Role:   "customer",
	}
	ctx := context.WithValue(context.Background(), UserContextKey, claims)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "forbidden")
}

func TestRequireRole_NoClaims(t *testing.T) {
	middleware := RequireRole("admin")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ============================================
// Helper Functions Tests
// ============================================

func TestGetUserFromContext_WithClaims(t *testing.T) {
	claims := &auth.Claims{
		UserID: "user-123",
		Email:  "test@example.com",
		Role:   "customer",
	}
	ctx := context.WithValue(context.Background(), UserContextKey, claims)

	result, ok := GetUserFromContext(ctx)

	assert.True(t, ok)
	assert.Equal(t, claims, result)
}

func TestGetUserFromContext_NoClaims(t *testing.T) {
	ctx := context.Background()

	result, ok := GetUserFromContext(ctx)

	assert.False(t, ok)
	assert.Nil(t, result)
}

func TestGetUserID_WithClaims(t *testing.T) {
	claims := &auth.Claims{
		UserID: "user-123",
	}
	ctx := context.WithValue(context.Background(), UserContextKey, claims)

	result := GetUserID(ctx)

	assert.Equal(t, "user-123", result)
}

func TestGetUserID_NoClaims(t *testing.T) {
	ctx := context.Background()

	result := GetUserID(ctx)

	assert.Empty(t, result)
}
