package aggregate

import (
	"errors"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/persistence"
	actorpersistence "github.com/example/ec-event-driven/internal/actor/persistence"
	pb "github.com/example/ec-event-driven/proto/domain/productpb"
	"google.golang.org/protobuf/proto"
)

const productPassivationTimeout = 5 * time.Minute

var (
	ErrProductInvalidName  = errors.New("name is required")
	ErrProductInvalidPrice = errors.New("price must be positive")
	ErrProductNotFound     = errors.New("product not found")
	ErrProductDeleted      = errors.New("product is deleted")
)

func init() {
	actorpersistence.RegisterEventType("ProductCreated", func() proto.Message { return &pb.ProductCreatedEvent{} })
	actorpersistence.RegisterEventType("ProductUpdated", func() proto.Message { return &pb.ProductUpdatedEvent{} })
	actorpersistence.RegisterEventType("ProductDeleted", func() proto.Message { return &pb.ProductDeletedEvent{} })
	actorpersistence.RegisterAggregateType("Product", "Product")
	actorpersistence.RegisterSnapshotFactory("Product", func() proto.Message { return &pb.ProductSnapshot{} })
}

// ProductActor is an event-sourced actor for the Product aggregate.
type ProductActor struct {
	persistence.Mixin
	state  *pb.ProductSnapshot
	system ActorSystemRef
}

// ActorSystemRef allows actors to notify the system of passivation.
type ActorSystemRef interface {
	RemoveActor(actorName string)
}

func NewProductActor(system ActorSystemRef) *ProductActor {
	return &ProductActor{
		system: system,
	}
}

func (a *ProductActor) Receive(ctx actor.Context) {
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

	// Snapshot recovery
	case *pb.ProductSnapshot:
		a.state = msg

	// Event replay during recovery
	case *pb.ProductCreatedEvent:
		a.applyProductCreated(msg)
	case *pb.ProductUpdatedEvent:
		a.applyProductUpdated(msg)
	case *pb.ProductDeletedEvent:
		a.applyProductDeleted(msg)

	// Commands (only during normal operation)
	case *pb.CreateProductCommand:
		a.handleCreate(ctx, msg)
	case *pb.UpdateProductCommand:
		a.handleUpdate(ctx, msg)
	case *pb.DeleteProductCommand:
		a.handleDelete(ctx, msg)
	}
}

// respond sends a message back to the original command sender.
// PersistReceive changes the context, so we capture sender beforehand.
func respond(ctx actor.Context, sender *actor.PID, msg interface{}) {
	if sender != nil {
		ctx.Send(sender, msg)
	}
}

func (a *ProductActor) handleCreate(ctx actor.Context, cmd *pb.CreateProductCommand) {
	sender := ctx.Sender()

	if cmd.Name == "" {
		respond(ctx, sender, newErrorResponse(ErrProductInvalidName))
		return
	}
	if cmd.Price <= 0 {
		respond(ctx, sender, newErrorResponse(ErrProductInvalidPrice))
		return
	}
	if a.state != nil {
		respond(ctx, sender, newErrorResponse(errors.New("product already exists")))
		return
	}

	event := &pb.ProductCreatedEvent{
		ProductId:   cmd.ProductId,
		Name:        cmd.Name,
		Description: cmd.Description,
		Price:       cmd.Price,
		Stock:       cmd.Stock,
		CreatedAt:   time.Now().Format(time.RFC3339Nano),
	}

	a.PersistReceive(event)
	a.applyProductCreated(event)

	respond(ctx, sender, &pb.ProductResponse{
		Id:          a.state.Id,
		Name:        a.state.Name,
		Description: a.state.Description,
		Price:       a.state.Price,
		Stock:       a.state.Stock,
		CreatedAt:   a.state.CreatedAt,
	})
}

func (a *ProductActor) handleUpdate(ctx actor.Context, cmd *pb.UpdateProductCommand) {
	sender := ctx.Sender()

	if a.state == nil {
		respond(ctx, sender, newErrorResponse(ErrProductNotFound))
		return
	}
	if a.state.IsDeleted {
		respond(ctx, sender, newErrorResponse(ErrProductDeleted))
		return
	}
	if cmd.Name == "" {
		respond(ctx, sender, newErrorResponse(ErrProductInvalidName))
		return
	}
	if cmd.Price <= 0 {
		respond(ctx, sender, newErrorResponse(ErrProductInvalidPrice))
		return
	}

	event := &pb.ProductUpdatedEvent{
		ProductId:   cmd.ProductId,
		Name:        cmd.Name,
		Description: cmd.Description,
		Price:       cmd.Price,
		UpdatedAt:   time.Now().Format(time.RFC3339Nano),
	}

	a.PersistReceive(event)
	a.applyProductUpdated(event)

	respond(ctx, sender, &CommandSuccess{})
}

func (a *ProductActor) handleDelete(ctx actor.Context, cmd *pb.DeleteProductCommand) {
	sender := ctx.Sender()

	if a.state == nil {
		respond(ctx, sender, newErrorResponse(ErrProductNotFound))
		return
	}
	if a.state.IsDeleted {
		respond(ctx, sender, newErrorResponse(ErrProductDeleted))
		return
	}

	event := &pb.ProductDeletedEvent{
		ProductId: cmd.ProductId,
		DeletedAt: time.Now().Format(time.RFC3339Nano),
	}

	a.PersistReceive(event)
	a.applyProductDeleted(event)

	respond(ctx, sender, &CommandSuccess{})
}

func (a *ProductActor) applyProductCreated(event *pb.ProductCreatedEvent) {
	a.state = &pb.ProductSnapshot{
		Id:          event.ProductId,
		Name:        event.Name,
		Description: event.Description,
		Price:       event.Price,
		Stock:       event.Stock,
		CreatedAt:   event.CreatedAt,
		UpdatedAt:   event.CreatedAt,
	}
}

func (a *ProductActor) applyProductUpdated(event *pb.ProductUpdatedEvent) {
	if a.state == nil {
		return
	}
	a.state.Name = event.Name
	a.state.Description = event.Description
	a.state.Price = event.Price
	a.state.UpdatedAt = event.UpdatedAt
}

func (a *ProductActor) applyProductDeleted(event *pb.ProductDeletedEvent) {
	if a.state == nil {
		return
	}
	a.state.IsDeleted = true
	a.state.UpdatedAt = event.DeletedAt
}

// CommandSuccess indicates a command was processed successfully without a specific return value.
type CommandSuccess struct{}

// ErrorResponse wraps an error for actor responses.
type ErrorResponse struct {
	Err error
}

func newErrorResponse(err error) *ErrorResponse {
	return &ErrorResponse{Err: err}
}
