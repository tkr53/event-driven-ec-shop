package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/example/ec-event-driven/internal/auth"
)

// respondError writes a JSON error response
func respondError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// ExtractToken extracts JWT token from cookie or Authorization header
func ExtractToken(r *http.Request) string {
	// Try cookie first (for browser)
	if cookie, err := r.Cookie("access_token"); err == nil {
		return cookie.Value
	}
	// Fall back to Authorization header (for API clients)
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

type contextKey string

const (
	UserContextKey contextKey = "user"
)

// AuthMiddleware validates JWT tokens and adds user claims to context
func AuthMiddleware(jwtService *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := ExtractToken(r)
			if tokenString == "" {
				respondError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := jwtService.ValidateAccessToken(tokenString)
			if err != nil {
				respondError(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuthMiddleware adds user claims to context if token is present, but doesn't require it
func OptionalAuthMiddleware(jwtService *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tokenString := ExtractToken(r); tokenString != "" {
				if claims, err := jwtService.ValidateAccessToken(tokenString); err == nil {
					ctx := context.WithValue(r.Context(), UserContextKey, claims)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole checks if the user has one of the required roles
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(UserContextKey).(*auth.Claims)
			if !ok {
				respondError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			for _, role := range roles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			respondError(w, "forbidden", http.StatusForbidden)
		})
	}
}

// GetUserFromContext retrieves user claims from the request context
func GetUserFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(UserContextKey).(*auth.Claims)
	return claims, ok
}

// GetUserID is a helper to get just the user ID from context
func GetUserID(ctx context.Context) string {
	claims, ok := GetUserFromContext(ctx)
	if !ok {
		return ""
	}
	return claims.UserID
}
