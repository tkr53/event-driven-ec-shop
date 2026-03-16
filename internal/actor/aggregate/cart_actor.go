package aggregate

import (
	"errors"
	"fmt"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/persistence"
	actorpersistence "github.com/example/ec-event-driven/internal/actor/persistence"
	pb "github.com/example/ec-event-driven/proto/domain/cartpb"
	"google.golang.org/protobuf/proto"
)

var (
	ErrCartInvalidProduct  = errors.New("product_id is required")
	ErrCartInvalidQuantity = errors.New("quantity must be positive")
)

func init() {
	actorpersistence.RegisterEventType("ItemAddedToCart", func() proto.Message { return &pb.ItemAddedToCartEvent{} })
	actorpersistence.RegisterEventType("ItemRemovedFromCart", func() proto.Message { return &pb.ItemRemovedFromCartEvent{} })
	actorpersistence.RegisterEventType("CartCleared", func() proto.Message { return &pb.CartClearedEvent{} })
	actorpersistence.RegisterAggregateType("Cart", "Cart")
	actorpersistence.RegisterSnapshotFactory("Cart", func() proto.Message { return &pb.CartSnapshot{} })
}

type CartActor struct {
	persistence.Mixin
	state  *pb.CartSnapshot
	system ActorSystemRef
}

func NewCartActor(system ActorSystemRef) *CartActor {
	return &CartActor{system: system}
}

func (a *CartActor) Receive(ctx actor.Context) {
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
	case *pb.CartSnapshot:
		a.state = msg
	case *pb.ItemAddedToCartEvent:
		a.applyItemAdded(msg)
	case *pb.ItemRemovedFromCartEvent:
		a.applyItemRemoved(msg)
	case *pb.CartClearedEvent:
		a.applyCartCleared(msg)

	// Commands
	case *pb.AddItemCommand:
		a.handleAddItem(ctx, msg)
	case *pb.RemoveItemCommand:
		a.handleRemoveItem(ctx, msg)
	case *pb.ClearCartCommand:
		a.handleClearCart(ctx, msg)
	}
}

func (a *CartActor) ensureState(cartID, userID string) {
	if a.state == nil {
		a.state = &pb.CartSnapshot{
			Id:     cartID,
			UserId: userID,
			Items:  make(map[string]*pb.CartItem),
		}
	}
	if a.state.Items == nil {
		a.state.Items = make(map[string]*pb.CartItem)
	}
}

func (a *CartActor) handleAddItem(ctx actor.Context, cmd *pb.AddItemCommand) {
	sender := ctx.Sender()
	if cmd.ProductId == "" {
		respond(ctx, sender, newErrorResponse(ErrCartInvalidProduct))
		return
	}
	if cmd.Quantity <= 0 {
		respond(ctx, sender, newErrorResponse(ErrCartInvalidQuantity))
		return
	}

	cartID := fmt.Sprintf("cart-%s", cmd.UserId)
	event := &pb.ItemAddedToCartEvent{
		CartId:    cartID,
		UserId:    cmd.UserId,
		ProductId: cmd.ProductId,
		Quantity:  cmd.Quantity,
		Price:     cmd.Price,
		AddedAt:   time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyItemAdded(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *CartActor) handleRemoveItem(ctx actor.Context, cmd *pb.RemoveItemCommand) {
	sender := ctx.Sender()
	if cmd.ProductId == "" {
		respond(ctx, sender, newErrorResponse(ErrCartInvalidProduct))
		return
	}

	cartID := fmt.Sprintf("cart-%s", cmd.UserId)
	event := &pb.ItemRemovedFromCartEvent{
		CartId:    cartID,
		UserId:    cmd.UserId,
		ProductId: cmd.ProductId,
		RemovedAt: time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyItemRemoved(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *CartActor) handleClearCart(ctx actor.Context, cmd *pb.ClearCartCommand) {
	sender := ctx.Sender()
	cartID := fmt.Sprintf("cart-%s", cmd.UserId)
	event := &pb.CartClearedEvent{
		CartId:    cartID,
		UserId:    cmd.UserId,
		ClearedAt: time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyCartCleared(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *CartActor) applyItemAdded(event *pb.ItemAddedToCartEvent) {
	a.ensureState(event.CartId, event.UserId)
	if existing, ok := a.state.Items[event.ProductId]; ok {
		existing.Quantity += event.Quantity
		existing.Price = event.Price
	} else {
		a.state.Items[event.ProductId] = &pb.CartItem{
			ProductId: event.ProductId,
			Quantity:  event.Quantity,
			Price:     event.Price,
		}
	}
}

func (a *CartActor) applyItemRemoved(event *pb.ItemRemovedFromCartEvent) {
	a.ensureState(event.CartId, event.UserId)
	delete(a.state.Items, event.ProductId)
}

func (a *CartActor) applyCartCleared(event *pb.CartClearedEvent) {
	a.ensureState(event.CartId, event.UserId)
	a.state.Items = make(map[string]*pb.CartItem)
}
