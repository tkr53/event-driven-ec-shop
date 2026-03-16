package aggregate

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/persistence"
	actorpersistence "github.com/example/ec-event-driven/internal/actor/persistence"
	pb "github.com/example/ec-event-driven/proto/domain/categorypb"
	"google.golang.org/protobuf/proto"
)

var categorySlugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrCategoryInvalidName = errors.New("name is required")
	ErrCategoryInvalidSlug = errors.New("invalid slug format")
	ErrCategoryDeleted     = errors.New("category is deleted")
)

func init() {
	actorpersistence.RegisterEventType("CategoryCreated", func() proto.Message { return &pb.CategoryCreatedEvent{} })
	actorpersistence.RegisterEventType("CategoryUpdated", func() proto.Message { return &pb.CategoryUpdatedEvent{} })
	actorpersistence.RegisterEventType("CategoryDeleted", func() proto.Message { return &pb.CategoryDeletedEvent{} })
	actorpersistence.RegisterAggregateType("Category", "Category")
	actorpersistence.RegisterSnapshotFactory("Category", func() proto.Message { return &pb.CategorySnapshot{} })
}

type CategoryActor struct {
	persistence.Mixin
	state  *pb.CategorySnapshot
	system ActorSystemRef
}

func NewCategoryActor(system ActorSystemRef) *CategoryActor {
	return &CategoryActor{system: system}
}

// CategoryCreatedResponse is returned after successful category creation.
type CategoryCreatedResponse struct {
	ID   string
	Name string
	Slug string
}

func (a *CategoryActor) Receive(ctx actor.Context) {
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
	case *pb.CategorySnapshot:
		a.state = msg
	case *pb.CategoryCreatedEvent:
		a.applyCategoryCreated(msg)
	case *pb.CategoryUpdatedEvent:
		a.applyCategoryUpdated(msg)
	case *pb.CategoryDeletedEvent:
		a.applyCategoryDeleted(msg)

	// Commands
	case *pb.CreateCategoryCommand:
		a.handleCreate(ctx, msg)
	case *pb.UpdateCategoryCommand:
		a.handleUpdate(ctx, msg)
	case *pb.DeleteCategoryCommand:
		a.handleDelete(ctx, msg)
	}
}

func (a *CategoryActor) handleCreate(ctx actor.Context, cmd *pb.CreateCategoryCommand) {
	sender := ctx.Sender()
	if cmd.Name == "" {
		respond(ctx, sender, newErrorResponse(ErrCategoryInvalidName))
		return
	}
	if a.state != nil {
		respond(ctx, sender, newErrorResponse(errors.New("category already exists")))
		return
	}

	slug := cmd.Slug
	if slug == "" {
		slug = generateSlug(cmd.Name)
	}
	if !categorySlugRegex.MatchString(slug) {
		respond(ctx, sender, newErrorResponse(ErrCategoryInvalidSlug))
		return
	}

	categoryID := a.Name()
	if _, id := actorpersistence.ParseActorName(categoryID); id != "" {
		categoryID = id
	}

	event := &pb.CategoryCreatedEvent{
		CategoryId:  categoryID,
		Name:        cmd.Name,
		Slug:        slug,
		Description: cmd.Description,
		ParentId:    cmd.ParentId,
		SortOrder:   cmd.SortOrder,
		CreatedAt:   time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyCategoryCreated(event)

	respond(ctx, sender, &CategoryCreatedResponse{
		ID:   a.state.Id,
		Name: a.state.Name,
		Slug: a.state.Slug,
	})
}

func (a *CategoryActor) handleUpdate(ctx actor.Context, cmd *pb.UpdateCategoryCommand) {
	sender := ctx.Sender()
	if a.state == nil {
		respond(ctx, sender, newErrorResponse(ErrCategoryNotFound))
		return
	}
	if !a.state.IsActive {
		respond(ctx, sender, newErrorResponse(ErrCategoryDeleted))
		return
	}
	if cmd.Name == "" {
		respond(ctx, sender, newErrorResponse(ErrCategoryInvalidName))
		return
	}

	slug := cmd.Slug
	if slug == "" {
		slug = generateSlug(cmd.Name)
	}
	if !categorySlugRegex.MatchString(slug) {
		respond(ctx, sender, newErrorResponse(ErrCategoryInvalidSlug))
		return
	}

	event := &pb.CategoryUpdatedEvent{
		CategoryId:  cmd.CategoryId,
		Name:        cmd.Name,
		Slug:        slug,
		Description: cmd.Description,
		ParentId:    cmd.ParentId,
		SortOrder:   cmd.SortOrder,
		UpdatedAt:   time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyCategoryUpdated(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *CategoryActor) handleDelete(ctx actor.Context, cmd *pb.DeleteCategoryCommand) {
	sender := ctx.Sender()
	if a.state == nil {
		respond(ctx, sender, newErrorResponse(ErrCategoryNotFound))
		return
	}
	event := &pb.CategoryDeletedEvent{
		CategoryId: cmd.CategoryId,
		DeletedAt:  time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyCategoryDeleted(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *CategoryActor) applyCategoryCreated(event *pb.CategoryCreatedEvent) {
	a.state = &pb.CategorySnapshot{
		Id:          event.CategoryId,
		Name:        event.Name,
		Slug:        event.Slug,
		Description: event.Description,
		ParentId:    event.ParentId,
		SortOrder:   event.SortOrder,
		IsActive:    true,
		CreatedAt:   event.CreatedAt,
		UpdatedAt:   event.CreatedAt,
	}
}

func (a *CategoryActor) applyCategoryUpdated(event *pb.CategoryUpdatedEvent) {
	if a.state == nil {
		return
	}
	a.state.Name = event.Name
	a.state.Slug = event.Slug
	a.state.Description = event.Description
	a.state.ParentId = event.ParentId
	a.state.SortOrder = event.SortOrder
	a.state.UpdatedAt = event.UpdatedAt
}

func (a *CategoryActor) applyCategoryDeleted(event *pb.CategoryDeletedEvent) {
	if a.state == nil {
		return
	}
	a.state.IsActive = false
	a.state.UpdatedAt = event.DeletedAt
}

func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	slug = reg.ReplaceAllString(slug, "")
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}
