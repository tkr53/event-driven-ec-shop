package user

import (
	"context"
	"testing"

	"github.com/example/ec-event-driven/internal/auth"
	"github.com/example/ec-event-driven/internal/infrastructure/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestUserService() (*Service, *mocks.MockEventStore) {
	eventStore := mocks.NewMockEventStore()
	service := NewService(eventStore)
	return service, eventStore
}

// ============================================
// Email Validation Tests
// ============================================

func TestIsValidEmail_ValidEmails(t *testing.T) {
	validEmails := []string{
		"test@example.com",
		"user.name@domain.org",
		"user+tag@example.com",
		"user123@test.co.jp",
		"a@b.cd",
		"user_name@domain.com",
		"USER@EXAMPLE.COM",
		"test@subdomain.example.com",
	}

	for _, email := range validEmails {
		t.Run(email, func(t *testing.T) {
			assert.True(t, isValidEmail(email), "Expected %s to be valid", email)
		})
	}
}

func TestIsValidEmail_InvalidEmails(t *testing.T) {
	invalidEmails := []string{
		"",
		"notanemail",
		"@example.com",
		"user@",
		"user@.com",
		"user@domain",
		"user@domain.",
		"user space@example.com",
		"user@exam ple.com",
		// Too long email (>254 chars) - need 255+ characters total
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa@example.com",
	}

	for _, email := range invalidEmails {
		t.Run(email, func(t *testing.T) {
			assert.False(t, isValidEmail(email), "Expected %s to be invalid", email)
		})
	}
}

// ============================================
// Register Tests
// ============================================

func TestService_Register_Success(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	user, err := service.Register(ctx, "test@example.com", "password123", "Test User")

	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "customer", user.Role)
	assert.True(t, user.IsActive)

	// Verify event was stored
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventUserCreated, eventStore.AppendCalls[0].EventType)
}

func TestService_Register_InvalidEmail(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	user, err := service.Register(ctx, "invalid-email", "password123", "Test User")

	assert.ErrorIs(t, err, ErrInvalidEmail)
	assert.Nil(t, user)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_Register_EmptyName(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	user, err := service.Register(ctx, "test@example.com", "password123", "")

	assert.ErrorIs(t, err, ErrInvalidName)
	assert.Nil(t, user)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_Register_ShortPassword(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	user, err := service.Register(ctx, "test@example.com", "short", "Test User")

	assert.ErrorIs(t, err, auth.ErrPasswordTooShort)
	assert.Nil(t, user)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_RegisterAdmin_Success(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	user, err := service.RegisterAdmin(ctx, "admin@example.com", "password123", "Admin User")

	require.NoError(t, err)
	assert.Equal(t, "admin", user.Role)
	assert.Len(t, eventStore.AppendCalls, 1)
}

func TestService_RegisterWithRole_Success(t *testing.T) {
	service, _ := newTestUserService()
	ctx := context.Background()

	tests := []struct {
		name string
		role string
	}{
		{"customer role", "customer"},
		{"admin role", "admin"},
		{"custom role", "moderator"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := service.RegisterWithRole(ctx, "test@example.com", "password123", "Test", tt.role)
			require.NoError(t, err)
			assert.Equal(t, tt.role, user.Role)
		})
	}
}

// ============================================
// Update Profile Tests
// ============================================

func TestService_UpdateProfile_Success(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	userID := "user-123"
	_ = eventStore.AddEvent(userID, AggregateType, EventUserCreated, UserCreated{UserID: userID})

	err := service.UpdateProfile(ctx, userID, "New Name")

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventUserUpdated, eventStore.AppendCalls[0].EventType)
}

func TestService_UpdateProfile_EmptyName(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	userID := "user-123"
	_ = eventStore.AddEvent(userID, AggregateType, EventUserCreated, UserCreated{UserID: userID})

	err := service.UpdateProfile(ctx, userID, "")

	assert.ErrorIs(t, err, ErrInvalidName)
}

func TestService_UpdateProfile_UserNotFound(t *testing.T) {
	service, _ := newTestUserService()
	ctx := context.Background()

	err := service.UpdateProfile(ctx, "non-existent", "New Name")

	assert.ErrorIs(t, err, ErrUserNotFound)
}

// ============================================
// Change Password Tests
// ============================================

func TestService_ChangePassword_Success(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	userID := "user-123"
	_ = eventStore.AddEvent(userID, AggregateType, EventUserCreated, UserCreated{UserID: userID})

	err := service.ChangePassword(ctx, userID, "newpassword123")

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventUserPasswordChanged, eventStore.AppendCalls[0].EventType)
}

func TestService_ChangePassword_ShortPassword(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	userID := "user-123"
	_ = eventStore.AddEvent(userID, AggregateType, EventUserCreated, UserCreated{UserID: userID})

	err := service.ChangePassword(ctx, userID, "short")

	assert.ErrorIs(t, err, auth.ErrPasswordTooShort)
}

func TestService_ChangePassword_UserNotFound(t *testing.T) {
	service, _ := newTestUserService()
	ctx := context.Background()

	err := service.ChangePassword(ctx, "non-existent", "newpassword123")

	assert.ErrorIs(t, err, ErrUserNotFound)
}

// ============================================
// Login/Logout Recording Tests
// ============================================

func TestService_RecordLogin_Success(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	err := service.RecordLogin(ctx, "user-123", "session-456", "192.168.1.1", "Mozilla/5.0")

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventUserLoggedIn, eventStore.AppendCalls[0].EventType)

	// Verify event data
	data := eventStore.AppendCalls[0].Data.(UserLoggedIn)
	assert.Equal(t, "user-123", data.UserID)
	assert.Equal(t, "session-456", data.SessionID)
	assert.Equal(t, "192.168.1.1", data.IPAddress)
	assert.Equal(t, "Mozilla/5.0", data.UserAgent)
}

func TestService_RecordLogout_Success(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	err := service.RecordLogout(ctx, "user-123", "session-456")

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventUserLoggedOut, eventStore.AppendCalls[0].EventType)
}

// ============================================
// Deactivate/Activate Tests
// ============================================

func TestService_Deactivate_Success(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	userID := "user-123"
	_ = eventStore.AddEvent(userID, AggregateType, EventUserCreated, UserCreated{UserID: userID})

	err := service.Deactivate(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventUserDeactivated, eventStore.AppendCalls[0].EventType)
}

func TestService_Deactivate_UserNotFound(t *testing.T) {
	service, _ := newTestUserService()
	ctx := context.Background()

	err := service.Deactivate(ctx, "non-existent")

	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestService_Activate_Success(t *testing.T) {
	service, eventStore := newTestUserService()
	ctx := context.Background()

	userID := "user-123"
	_ = eventStore.AddEvent(userID, AggregateType, EventUserCreated, UserCreated{UserID: userID})
	_ = eventStore.AddEvent(userID, AggregateType, EventUserDeactivated, UserDeactivated{UserID: userID})

	err := service.Activate(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventUserActivated, eventStore.AppendCalls[0].EventType)
}

func TestService_Activate_UserNotFound(t *testing.T) {
	service, _ := newTestUserService()
	ctx := context.Background()

	err := service.Activate(ctx, "non-existent")

	assert.ErrorIs(t, err, ErrUserNotFound)
}
