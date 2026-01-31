package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/example/ec-event-driven/internal/api/middleware"
	"github.com/example/ec-event-driven/internal/auth"
	"github.com/example/ec-event-driven/internal/domain/user"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/readmodel"
	"github.com/google/uuid"
)

// hashToken creates a SHA-256 hash of the token for secure storage
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// AuthHandlers handles authentication-related HTTP requests
type AuthHandlers struct {
	userService *user.Service
	jwtService  *auth.JWTService
	readStore   *store.PostgresReadStore
}

// NewAuthHandlers creates a new AuthHandlers instance
func NewAuthHandlers(userService *user.Service, jwtService *auth.JWTService, readStore *store.PostgresReadStore) *AuthHandlers {
	return &AuthHandlers{
		userService: userService,
		jwtService:  jwtService,
		readStore:   readStore,
	}
}

// RegisterRequest represents the registration request body
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	User    UserResponse `json:"user"`
	Message string       `json:"message,omitempty"`
}

// UserResponse represents user data in responses
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Register handles user registration
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if email already exists
	if _, exists := h.readStore.GetUserByEmail(req.Email); exists {
		respondJSONError(w, "Email already registered", http.StatusConflict)
		return
	}

	// Create user
	newUser, err := h.userService.Register(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		if err == auth.ErrPasswordTooShort {
			respondJSONError(w, "Password must be at least 8 characters", http.StatusBadRequest)
			return
		}
		respondJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate tokens and set cookies
	h.setAuthCookies(w, newUser.ID, newUser.Email, newUser.Role, r)

	respondJSON(w, http.StatusCreated, AuthResponse{
		User: UserResponse{
			ID:        newUser.ID,
			Email:     newUser.Email,
			Name:      newUser.Name,
			Role:      newUser.Role,
			CreatedAt: newUser.CreatedAt,
		},
		Message: "Registration successful",
	})
}

// Login handles user login
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Find user by email
	userModel, exists := h.readStore.GetUserByEmail(req.Email)
	if !exists {
		respondJSONError(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Check if user is active
	if !userModel.IsActive {
		respondJSONError(w, "Account is deactivated", http.StatusForbidden)
		return
	}

	// Verify password
	if !auth.CheckPassword(req.Password, userModel.PasswordHash) {
		respondJSONError(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Generate tokens and set cookies
	h.setAuthCookies(w, userModel.ID, userModel.Email, userModel.Role, r)

	// Record login event (best-effort, don't fail login on error)
	sessionID := uuid.New().String()
	ipAddress := r.RemoteAddr
	userAgent := r.UserAgent()
	_ = h.userService.RecordLogin(r.Context(), userModel.ID, sessionID, ipAddress, userAgent)

	respondJSON(w, http.StatusOK, AuthResponse{
		User: UserResponse{
			ID:        userModel.ID,
			Email:     userModel.Email,
			Name:      userModel.Name,
			Role:      userModel.Role,
			CreatedAt: userModel.CreatedAt,
		},
		Message: "Login successful",
	})
}

// Logout handles user logout
func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if ok {
		// Record logout event (best-effort)
		sessionID := ""
		if cookie, err := r.Cookie("session_id"); err == nil {
			sessionID = cookie.Value
		}
		_ = h.userService.RecordLogout(r.Context(), claims.UserID, sessionID)

		// Delete user sessions (best-effort)
		_ = h.readStore.DeleteSessionsByUserID(claims.UserID)
	}

	// Clear cookies
	h.clearAuthCookies(w)

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Logout successful",
	})
}

// Refresh handles token refresh
func (h *AuthHandlers) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshCookie, err := r.Cookie("refresh_token")
	if err != nil {
		respondJSONError(w, "No refresh token", http.StatusUnauthorized)
		return
	}

	sessionCookie, err := r.Cookie("session_id")
	if err != nil {
		h.clearAuthCookies(w)
		respondJSONError(w, "No session", http.StatusUnauthorized)
		return
	}

	// Validate JWT token
	userID, err := h.jwtService.ValidateRefreshToken(refreshCookie.Value)
	if err != nil {
		h.clearAuthCookies(w)
		respondJSONError(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	// Validate session exists and is not expired
	sessionData, exists, err := h.readStore.Get("sessions", sessionCookie.Value)
	if err != nil || !exists {
		h.clearAuthCookies(w)
		respondJSONError(w, "Session not found", http.StatusUnauthorized)
		return
	}

	session := sessionData.(*readmodel.SessionReadModel)

	// Check session expiration
	if time.Now().After(session.ExpiresAt) {
		_ = h.readStore.Delete("sessions", sessionCookie.Value)
		h.clearAuthCookies(w)
		respondJSONError(w, "Session expired", http.StatusUnauthorized)
		return
	}

	// Verify refresh token hash matches stored hash
	if hashToken(refreshCookie.Value) != session.RefreshTokenHash {
		h.clearAuthCookies(w)
		respondJSONError(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	// Get user
	userData, exists, err := h.readStore.Get("users", userID)
	if err != nil || !exists {
		h.clearAuthCookies(w)
		respondJSONError(w, "User not found", http.StatusUnauthorized)
		return
	}

	userModel := userData.(*readmodel.UserReadModel)
	if !userModel.IsActive {
		h.clearAuthCookies(w)
		respondJSONError(w, "Account is deactivated", http.StatusForbidden)
		return
	}

	// Delete old session
	_ = h.readStore.Delete("sessions", sessionCookie.Value)

	// Generate new tokens (this will create a new session)
	h.setAuthCookies(w, userModel.ID, userModel.Email, userModel.Role, r)

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Token refreshed",
	})
}

// Me returns the current authenticated user's information
func (h *AuthHandlers) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userData, exists, err := h.readStore.Get("users", claims.UserID)
	if err != nil || !exists {
		respondJSONError(w, "User not found", http.StatusNotFound)
		return
	}

	userModel := userData.(*readmodel.UserReadModel)

	respondJSON(w, http.StatusOK, UserResponse{
		ID:        userModel.ID,
		Email:     userModel.Email,
		Name:      userModel.Name,
		Role:      userModel.Role,
		CreatedAt: userModel.CreatedAt,
	})
}

// ChangePassword handles password change requests
func (h *AuthHandlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user and verify current password
	userData, exists, err := h.readStore.Get("users", claims.UserID)
	if err != nil || !exists {
		respondJSONError(w, "User not found", http.StatusNotFound)
		return
	}

	userModel := userData.(*readmodel.UserReadModel)
	if !auth.CheckPassword(req.CurrentPassword, userModel.PasswordHash) {
		respondJSONError(w, "Current password is incorrect", http.StatusBadRequest)
		return
	}

	// Change password
	if err := h.userService.ChangePassword(r.Context(), claims.UserID, req.NewPassword); err != nil {
		if err == auth.ErrPasswordTooShort {
			respondJSONError(w, "New password must be at least 8 characters", http.StatusBadRequest)
			return
		}
		respondJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Password changed successfully",
	})
}

// Helper methods

func (h *AuthHandlers) setAuthCookies(w http.ResponseWriter, userID, email, role string, r *http.Request) {
	// Generate access token
	accessToken, accessExpiry, _ := h.jwtService.GenerateAccessToken(userID, email, role)

	// Generate refresh token
	refreshToken, refreshExpiry, _ := h.jwtService.GenerateRefreshToken(userID)

	// Generate session ID
	sessionID := uuid.New().String()

	// Store session with hashed refresh token
	_ = h.readStore.Set("sessions", sessionID, &readmodel.SessionReadModel{
		ID:               sessionID,
		UserID:           userID,
		RefreshTokenHash: hashToken(refreshToken),
		ExpiresAt:        refreshExpiry,
		CreatedAt:        time.Now(),
		IPAddress:        r.RemoteAddr,
		UserAgent:        r.UserAgent(),
	})

	// Set cookies
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		Expires:  accessExpiry,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/api/auth/refresh",
		Expires:  refreshExpiry,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		Expires:  refreshExpiry,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *AuthHandlers) clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth/refresh",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

func respondJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
