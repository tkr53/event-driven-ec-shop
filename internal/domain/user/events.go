package user

import "time"

const (
	EventUserCreated         = "UserCreated"
	EventUserUpdated         = "UserUpdated"
	EventUserPasswordChanged = "UserPasswordChanged"
	EventUserLoggedIn        = "UserLoggedIn"
	EventUserLoggedOut       = "UserLoggedOut"
	EventUserDeactivated     = "UserDeactivated"
	EventUserActivated       = "UserActivated"
)

// UserCreated is emitted when a new user is registered
type UserCreated struct {
	UserID       string    `json:"user_id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserUpdated is emitted when user profile is updated
type UserUpdated struct {
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserPasswordChanged is emitted when user changes password
type UserPasswordChanged struct {
	UserID       string    `json:"user_id"`
	PasswordHash string    `json:"password_hash"`
	ChangedAt    time.Time `json:"changed_at"`
}

// UserLoggedIn is emitted when user successfully logs in
type UserLoggedIn struct {
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	LoggedAt  time.Time `json:"logged_at"`
}

// UserLoggedOut is emitted when user logs out
type UserLoggedOut struct {
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	LoggedAt  time.Time `json:"logged_at"`
}

// UserDeactivated is emitted when user account is deactivated
type UserDeactivated struct {
	UserID        string    `json:"user_id"`
	DeactivatedAt time.Time `json:"deactivated_at"`
}

// UserActivated is emitted when user account is reactivated
type UserActivated struct {
	UserID      string    `json:"user_id"`
	ActivatedAt time.Time `json:"activated_at"`
}
