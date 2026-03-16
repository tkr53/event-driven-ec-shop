package aggregate

import (
	"errors"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/persistence"
	actorpersistence "github.com/example/ec-event-driven/internal/actor/persistence"
	pb "github.com/example/ec-event-driven/proto/domain/inventorypb"
	"google.golang.org/protobuf/proto"
)

var (
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrInvalidQuantity   = errors.New("quantity must be positive")
)

func init() {
	actorpersistence.RegisterEventType("StockAdded", func() proto.Message { return &pb.StockAddedEvent{} })
	actorpersistence.RegisterEventType("StockReserved", func() proto.Message { return &pb.StockReservedEvent{} })
	actorpersistence.RegisterEventType("StockReleased", func() proto.Message { return &pb.StockReleasedEvent{} })
	actorpersistence.RegisterEventType("StockDeducted", func() proto.Message { return &pb.StockDeductedEvent{} })
	actorpersistence.RegisterAggregateType("Inventory", "Inventory")
	actorpersistence.RegisterSnapshotFactory("Inventory", func() proto.Message { return &pb.InventorySnapshot{} })
}

type InventoryActor struct {
	persistence.Mixin
	state  *pb.InventorySnapshot
	system ActorSystemRef
}

func NewInventoryActor(system ActorSystemRef) *InventoryActor {
	return &InventoryActor{system: system}
}

func (a *InventoryActor) Receive(ctx actor.Context) {
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

	// Snapshot/event recovery
	case *pb.InventorySnapshot:
		a.state = msg
	case *pb.StockAddedEvent:
		a.applyStockAdded(msg)
	case *pb.StockReservedEvent:
		a.applyStockReserved(msg)
	case *pb.StockReleasedEvent:
		a.applyStockReleased(msg)
	case *pb.StockDeductedEvent:
		a.applyStockDeducted(msg)

	// Commands
	case *pb.AddStockCommand:
		a.handleAddStock(ctx, msg)
	case *pb.ReserveStockCommand:
		a.handleReserveStock(ctx, msg)
	case *pb.ReleaseStockCommand:
		a.handleReleaseStock(ctx, msg)
	case *pb.DeductStockCommand:
		a.handleDeductStock(ctx, msg)
	}
}

func (a *InventoryActor) ensureState(productID string) {
	if a.state == nil {
		a.state = &pb.InventorySnapshot{ProductId: productID}
	}
}

func (a *InventoryActor) handleAddStock(ctx actor.Context, cmd *pb.AddStockCommand) {
	sender := ctx.Sender()
	if cmd.Quantity <= 0 {
		respond(ctx, sender, newErrorResponse(ErrInvalidQuantity))
		return
	}

	event := &pb.StockAddedEvent{
		ProductId: cmd.ProductId,
		Quantity:  cmd.Quantity,
		AddedAt:   time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyStockAdded(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *InventoryActor) handleReserveStock(ctx actor.Context, cmd *pb.ReserveStockCommand) {
	sender := ctx.Sender()
	if cmd.Quantity <= 0 {
		respond(ctx, sender, newErrorResponse(ErrInvalidQuantity))
		return
	}

	a.ensureState(cmd.ProductId)
	available := a.state.TotalStock - a.state.ReservedStock
	if available < cmd.Quantity {
		respond(ctx, sender, newErrorResponse(ErrInsufficientStock))
		return
	}

	event := &pb.StockReservedEvent{
		ProductId:  cmd.ProductId,
		OrderId:    cmd.OrderId,
		Quantity:   cmd.Quantity,
		ReservedAt: time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyStockReserved(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *InventoryActor) handleReleaseStock(ctx actor.Context, cmd *pb.ReleaseStockCommand) {
	sender := ctx.Sender()
	if cmd.Quantity <= 0 {
		respond(ctx, sender, newErrorResponse(ErrInvalidQuantity))
		return
	}

	event := &pb.StockReleasedEvent{
		ProductId:  cmd.ProductId,
		OrderId:    cmd.OrderId,
		Quantity:   cmd.Quantity,
		ReleasedAt: time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyStockReleased(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *InventoryActor) handleDeductStock(ctx actor.Context, cmd *pb.DeductStockCommand) {
	sender := ctx.Sender()
	if cmd.Quantity <= 0 {
		respond(ctx, sender, newErrorResponse(ErrInvalidQuantity))
		return
	}

	event := &pb.StockDeductedEvent{
		ProductId:  cmd.ProductId,
		OrderId:    cmd.OrderId,
		Quantity:   cmd.Quantity,
		DeductedAt: time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyStockDeducted(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *InventoryActor) applyStockAdded(event *pb.StockAddedEvent) {
	a.ensureState(event.ProductId)
	a.state.TotalStock += event.Quantity
}

func (a *InventoryActor) applyStockReserved(event *pb.StockReservedEvent) {
	a.ensureState(event.ProductId)
	a.state.ReservedStock += event.Quantity
}

func (a *InventoryActor) applyStockReleased(event *pb.StockReleasedEvent) {
	a.ensureState(event.ProductId)
	a.state.ReservedStock -= event.Quantity
	if a.state.ReservedStock < 0 {
		a.state.ReservedStock = 0
	}
}

func (a *InventoryActor) applyStockDeducted(event *pb.StockDeductedEvent) {
	a.ensureState(event.ProductId)
	a.state.TotalStock -= event.Quantity
	a.state.ReservedStock -= event.Quantity
	if a.state.TotalStock < 0 {
		a.state.TotalStock = 0
	}
	if a.state.ReservedStock < 0 {
		a.state.ReservedStock = 0
	}
}
