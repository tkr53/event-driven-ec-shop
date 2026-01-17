package user

import (
	"context"
	"errors"
	"time"

	"github.com/example/ec-event-driven/internal/auth"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/google/uuid"
)

const AggregateType = "User"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidEmail       = errors.New("email is required")
	ErrInvalidName        = errors.New("name is required")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserDeactivated    = errors.New("user account is deactivated")
)

// User represents a user aggregate
type User struct {
	ID           string
	Email        string
	PasswordHash string
	Name         string
	Role         string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Service handles user domain operations
type Service struct {
	eventStore store.EventStoreInterface
}

// NewService creates a new user service
func NewService(es store.EventStoreInterface) *Service {
	return &Service{eventStore: es}
}

// Register creates a new user
func (s *Service) Register(ctx context.Context, email, password, name string) (*User, error) {
	return s.RegisterWithRole(ctx, email, password, name, "customer")
}

// RegisterAdmin creates a new admin user
func (s *Service) RegisterAdmin(ctx context.Context, email, password, name string) (*User, error) {
	return s.RegisterWithRole(ctx, email, password, name, "admin")
}

// RegisterWithRole creates a new user with a specific role
func (s *Service) RegisterWithRole(ctx context.Context, email, password, name, role string) (*User, error) {
	if email == "" {
		return nil, ErrInvalidEmail
	}
	if name == "" {
		return nil, ErrInvalidName
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}

	userID := uuid.New().String()
	now := time.Now()

	event := UserCreated{
		UserID:       userID,
		Email:        email,
		PasswordHash: passwordHash,
		Name:         name,
		Role:         role,
		CreatedAt:    now,
	}

	_, err = s.eventStore.Append(ctx, userID, AggregateType, EventUserCreated, event)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:        userID,
		Email:     email,
		Name:      name,
		Role:      role,
		IsActive:  true,
		CreatedAt: now,
	}, nil
}

// RecordLogin records a user login event
func (s *Service) RecordLogin(ctx context.Context, userID, sessionID, ipAddress, userAgent string) error {
	event := UserLoggedIn{
		UserID:    userID,
		SessionID: sessionID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		LoggedAt:  time.Now(),
	}

	_, err := s.eventStore.Append(ctx, userID, AggregateType, EventUserLoggedIn, event)
	return err
}

// RecordLogout records a user logout event
func (s *Service) RecordLogout(ctx context.Context, userID, sessionID string) error {
	event := UserLoggedOut{
		UserID:    userID,
		SessionID: sessionID,
		LoggedAt:  time.Now(),
	}

	_, err := s.eventStore.Append(ctx, userID, AggregateType, EventUserLoggedOut, event)
	return err
}

// UpdateProfile updates user profile information
func (s *Service) UpdateProfile(ctx context.Context, userID, name string) error {
	if name == "" {
		return ErrInvalidName
	}

	events := s.eventStore.GetEvents(userID)
	if len(events) == 0 {
		return ErrUserNotFound
	}

	event := UserUpdated{
		UserID:    userID,
		Name:      name,
		UpdatedAt: time.Now(),
	}

	_, err := s.eventStore.Append(ctx, userID, AggregateType, EventUserUpdated, event)
	return err
}

// ChangePassword changes user password
func (s *Service) ChangePassword(ctx context.Context, userID, newPassword string) error {
	events := s.eventStore.GetEvents(userID)
	if len(events) == 0 {
		return ErrUserNotFound
	}

	passwordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}

	event := UserPasswordChanged{
		UserID:       userID,
		PasswordHash: passwordHash,
		ChangedAt:    time.Now(),
	}

	_, err = s.eventStore.Append(ctx, userID, AggregateType, EventUserPasswordChanged, event)
	return err
}

// Deactivate deactivates a user account
func (s *Service) Deactivate(ctx context.Context, userID string) error {
	events := s.eventStore.GetEvents(userID)
	if len(events) == 0 {
		return ErrUserNotFound
	}

	event := UserDeactivated{
		UserID:        userID,
		DeactivatedAt: time.Now(),
	}

	_, err := s.eventStore.Append(ctx, userID, AggregateType, EventUserDeactivated, event)
	return err
}

// Activate activates a user account
func (s *Service) Activate(ctx context.Context, userID string) error {
	events := s.eventStore.GetEvents(userID)
	if len(events) == 0 {
		return ErrUserNotFound
	}

	event := UserActivated{
		UserID:      userID,
		ActivatedAt: time.Now(),
	}

	_, err := s.eventStore.Append(ctx, userID, AggregateType, EventUserActivated, event)
	return err
}
