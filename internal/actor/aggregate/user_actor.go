package aggregate

import (
	"errors"
	"regexp"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/persistence"
	"github.com/example/ec-event-driven/internal/auth"
	actorpersistence "github.com/example/ec-event-driven/internal/actor/persistence"
	pb "github.com/example/ec-event-driven/proto/domain/userpb"
	"google.golang.org/protobuf/proto"
)

var userEmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserInvalidEmail   = errors.New("invalid email format")
	ErrUserInvalidName    = errors.New("name is required")
	ErrUserDeactivated    = errors.New("user account is deactivated")
)

func init() {
	actorpersistence.RegisterEventType("UserCreated", func() proto.Message { return &pb.UserCreatedEvent{} })
	actorpersistence.RegisterEventType("UserUpdated", func() proto.Message { return &pb.UserUpdatedEvent{} })
	actorpersistence.RegisterEventType("UserPasswordChanged", func() proto.Message { return &pb.UserPasswordChangedEvent{} })
	actorpersistence.RegisterEventType("UserLoggedIn", func() proto.Message { return &pb.UserLoggedInEvent{} })
	actorpersistence.RegisterEventType("UserLoggedOut", func() proto.Message { return &pb.UserLoggedOutEvent{} })
	actorpersistence.RegisterEventType("UserDeactivated", func() proto.Message { return &pb.UserDeactivatedEvent{} })
	actorpersistence.RegisterEventType("UserActivated", func() proto.Message { return &pb.UserActivatedEvent{} })
	actorpersistence.RegisterAggregateType("User", "User")
	actorpersistence.RegisterSnapshotFactory("User", func() proto.Message { return &pb.UserSnapshot{} })
}

type UserActor struct {
	persistence.Mixin
	state  *pb.UserSnapshot
	system ActorSystemRef
}

func NewUserActor(system ActorSystemRef) *UserActor {
	return &UserActor{system: system}
}

// UserCreatedResponse is returned after successful user registration.
type UserCreatedResponse struct {
	UserID string
	Email  string
	Name   string
	Role   string
}

func (a *UserActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		ctx.SetReceiveTimeout(productPassivationTimeout)
	case *actor.ReceiveTimeout:
		if a.system != nil {
			a.system.RemoveActor(ctx.Self().GetId())
		}
		ctx.Poison(ctx.Self())
	case *persistence.RequestSnapshot:
		if a.state != nil {
			a.PersistSnapshot(a.state)
		}

	// Recovery
	case *pb.UserSnapshot:
		a.state = msg
	case *pb.UserCreatedEvent:
		a.applyUserCreated(msg)
	case *pb.UserUpdatedEvent:
		a.applyUserUpdated(msg)
	case *pb.UserPasswordChangedEvent:
		a.applyPasswordChanged(msg)
	case *pb.UserLoggedInEvent:
		// No state change needed
	case *pb.UserLoggedOutEvent:
		// No state change needed
	case *pb.UserDeactivatedEvent:
		a.applyDeactivated(msg)
	case *pb.UserActivatedEvent:
		a.applyActivated(msg)

	// Commands
	case *pb.RegisterUserCommand:
		a.handleRegister(ctx, msg)
	case *pb.UpdateProfileCommand:
		a.handleUpdateProfile(ctx, msg)
	case *pb.ChangePasswordCommand:
		a.handleChangePassword(ctx, msg)
	case *pb.RecordLoginCommand:
		a.handleRecordLogin(ctx, msg)
	case *pb.RecordLogoutCommand:
		a.handleRecordLogout(ctx, msg)
	case *pb.DeactivateUserCommand:
		a.handleDeactivate(ctx, msg)
	case *pb.ActivateUserCommand:
		a.handleActivate(ctx, msg)
	}
}

func (a *UserActor) handleRegister(ctx actor.Context, cmd *pb.RegisterUserCommand) {
	sender := ctx.Sender()

	if !userEmailRegex.MatchString(cmd.Email) || len(cmd.Email) > 254 {
		respond(ctx, sender, newErrorResponse(ErrUserInvalidEmail))
		return
	}
	if cmd.Name == "" {
		respond(ctx, sender, newErrorResponse(ErrUserInvalidName))
		return
	}
	if a.state != nil {
		respond(ctx, sender, newErrorResponse(errors.New("user already exists")))
		return
	}

	passwordHash, err := auth.HashPassword(cmd.Password)
	if err != nil {
		respond(ctx, sender, newErrorResponse(err))
		return
	}

	userID := a.Name()
	if _, id := actorpersistence.ParseActorName(userID); id != "" {
		userID = id
	}

	role := cmd.Role
	if role == "" {
		role = "customer"
	}

	event := &pb.UserCreatedEvent{
		UserId:       userID,
		Email:        cmd.Email,
		PasswordHash: passwordHash,
		Name:         cmd.Name,
		Role:         role,
		CreatedAt:    time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyUserCreated(event)

	respond(ctx, sender, &UserCreatedResponse{
		UserID: a.state.Id,
		Email:  a.state.Email,
		Name:   a.state.Name,
		Role:   a.state.Role,
	})
}

func (a *UserActor) handleUpdateProfile(ctx actor.Context, cmd *pb.UpdateProfileCommand) {
	sender := ctx.Sender()
	if a.state == nil {
		respond(ctx, sender, newErrorResponse(ErrUserNotFound))
		return
	}
	if cmd.Name == "" {
		respond(ctx, sender, newErrorResponse(ErrUserInvalidName))
		return
	}
	event := &pb.UserUpdatedEvent{
		UserId:    cmd.UserId,
		Name:      cmd.Name,
		UpdatedAt: time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyUserUpdated(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *UserActor) handleChangePassword(ctx actor.Context, cmd *pb.ChangePasswordCommand) {
	sender := ctx.Sender()
	if a.state == nil {
		respond(ctx, sender, newErrorResponse(ErrUserNotFound))
		return
	}
	passwordHash, err := auth.HashPassword(cmd.NewPassword)
	if err != nil {
		respond(ctx, sender, newErrorResponse(err))
		return
	}
	event := &pb.UserPasswordChangedEvent{
		UserId:       cmd.UserId,
		PasswordHash: passwordHash,
		ChangedAt:    time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyPasswordChanged(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *UserActor) handleRecordLogin(ctx actor.Context, cmd *pb.RecordLoginCommand) {
	sender := ctx.Sender()
	event := &pb.UserLoggedInEvent{
		UserId:    cmd.UserId,
		SessionId: cmd.SessionId,
		IpAddress: cmd.IpAddress,
		UserAgent: cmd.UserAgent,
		LoggedAt:  time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *UserActor) handleRecordLogout(ctx actor.Context, cmd *pb.RecordLogoutCommand) {
	sender := ctx.Sender()
	event := &pb.UserLoggedOutEvent{
		UserId:    cmd.UserId,
		SessionId: cmd.SessionId,
		LoggedAt:  time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *UserActor) handleDeactivate(ctx actor.Context, cmd *pb.DeactivateUserCommand) {
	sender := ctx.Sender()
	if a.state == nil {
		respond(ctx, sender, newErrorResponse(ErrUserNotFound))
		return
	}
	event := &pb.UserDeactivatedEvent{
		UserId:        cmd.UserId,
		DeactivatedAt: time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyDeactivated(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *UserActor) handleActivate(ctx actor.Context, cmd *pb.ActivateUserCommand) {
	sender := ctx.Sender()
	if a.state == nil {
		respond(ctx, sender, newErrorResponse(ErrUserNotFound))
		return
	}
	event := &pb.UserActivatedEvent{
		UserId:      cmd.UserId,
		ActivatedAt: time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyActivated(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *UserActor) applyUserCreated(event *pb.UserCreatedEvent) {
	a.state = &pb.UserSnapshot{
		Id:           event.UserId,
		Email:        event.Email,
		PasswordHash: event.PasswordHash,
		Name:         event.Name,
		Role:         event.Role,
		IsActive:     true,
		CreatedAt:    event.CreatedAt,
		UpdatedAt:    event.CreatedAt,
	}
}

func (a *UserActor) applyUserUpdated(event *pb.UserUpdatedEvent) {
	if a.state == nil {
		return
	}
	a.state.Name = event.Name
	a.state.UpdatedAt = event.UpdatedAt
}

func (a *UserActor) applyPasswordChanged(event *pb.UserPasswordChangedEvent) {
	if a.state == nil {
		return
	}
	a.state.PasswordHash = event.PasswordHash
	a.state.UpdatedAt = event.ChangedAt
}

func (a *UserActor) applyDeactivated(event *pb.UserDeactivatedEvent) {
	if a.state == nil {
		return
	}
	a.state.IsActive = false
	a.state.UpdatedAt = event.DeactivatedAt
}

func (a *UserActor) applyActivated(event *pb.UserActivatedEvent) {
	if a.state == nil {
		return
	}
	a.state.IsActive = true
	a.state.UpdatedAt = event.ActivatedAt
}
