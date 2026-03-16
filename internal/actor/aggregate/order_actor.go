package aggregate

import (
	"errors"
	"fmt"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/persistence"
	actorpersistence "github.com/example/ec-event-driven/internal/actor/persistence"
	pb "github.com/example/ec-event-driven/proto/domain/orderpb"
	"google.golang.org/protobuf/proto"
)

var (
	ErrOrderNotFound    = errors.New("order not found")
	ErrEmptyOrder       = errors.New("order must have at least one item")
	ErrInvalidTransition = errors.New("invalid order status transition")
	ErrOrderAlreadyPaid = errors.New("order is already paid")
	ErrOrderNotPaid     = errors.New("order must be paid before shipping")
	ErrOrderShipped     = errors.New("cannot cancel shipped order")
	ErrOrderCancelled   = errors.New("order is already cancelled")
)

func init() {
	actorpersistence.RegisterEventType("OrderPlaced", func() proto.Message { return &pb.OrderPlacedEvent{} })
	actorpersistence.RegisterEventType("OrderPaid", func() proto.Message { return &pb.OrderPaidEvent{} })
	actorpersistence.RegisterEventType("OrderShipped", func() proto.Message { return &pb.OrderShippedEvent{} })
	actorpersistence.RegisterEventType("OrderCancelled", func() proto.Message { return &pb.OrderCancelledEvent{} })
	actorpersistence.RegisterAggregateType("Order", "Order")
	actorpersistence.RegisterSnapshotFactory("Order", func() proto.Message { return &pb.OrderSnapshot{} })
}

type OrderActor struct {
	persistence.Mixin
	state  *pb.OrderSnapshot
	system ActorSystemRef
}

func NewOrderActor(system ActorSystemRef) *OrderActor {
	return &OrderActor{system: system}
}

func (a *OrderActor) Receive(ctx actor.Context) {
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
	case *pb.OrderSnapshot:
		a.state = msg
	case *pb.OrderPlacedEvent:
		a.applyOrderPlaced(msg)
	case *pb.OrderPaidEvent:
		a.applyOrderPaid(msg)
	case *pb.OrderShippedEvent:
		a.applyOrderShipped(msg)
	case *pb.OrderCancelledEvent:
		a.applyOrderCancelled(msg)

	// Commands
	case *pb.PlaceOrderCommand:
		a.handlePlaceOrder(ctx, msg)
	case *pb.PayOrderCommand:
		a.handlePayOrder(ctx, msg)
	case *pb.ShipOrderCommand:
		a.handleShipOrder(ctx, msg)
	case *pb.CancelOrderCommand:
		a.handleCancelOrder(ctx, msg)
	}
}

var validTransitions = map[string][]string{
	"pending":   {"paid", "cancelled"},
	"paid":      {"shipped", "cancelled"},
	"shipped":   {},
	"cancelled": {},
}

func (a *OrderActor) canTransitionTo(target string) bool {
	if a.state == nil {
		return false
	}
	allowed, exists := validTransitions[a.state.Status]
	if !exists {
		return false
	}
	for _, s := range allowed {
		if s == target {
			return true
		}
	}
	return false
}

func (a *OrderActor) transitionError(target string) error {
	if a.state == nil {
		return ErrOrderNotFound
	}
	switch {
	case a.state.Status == "cancelled":
		return ErrOrderCancelled
	case a.state.Status == "shipped" && target == "cancelled":
		return ErrOrderShipped
	case (a.state.Status == "paid" || a.state.Status == "shipped") && target == "paid":
		return ErrOrderAlreadyPaid
	case a.state.Status == "pending" && target == "shipped":
		return ErrOrderNotPaid
	default:
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidTransition, a.state.Status, target)
	}
}

func (a *OrderActor) handlePlaceOrder(ctx actor.Context, cmd *pb.PlaceOrderCommand) {
	sender := ctx.Sender()
	if len(cmd.Items) == 0 {
		respond(ctx, sender, newErrorResponse(ErrEmptyOrder))
		return
	}
	if a.state != nil {
		respond(ctx, sender, newErrorResponse(errors.New("order already exists")))
		return
	}

	var total int32
	for _, item := range cmd.Items {
		total += item.Price * item.Quantity
	}

	// Extract order ID from the actor name
	orderID := a.Name()
	if _, id := actorpersistence.ParseActorName(orderID); id != "" {
		orderID = id
	}

	event := &pb.OrderPlacedEvent{
		OrderId:  orderID,
		UserId:   cmd.UserId,
		Items:    cmd.Items,
		Total:    total,
		PlacedAt: time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyOrderPlaced(event)

	respond(ctx, sender, a.state)
}

func (a *OrderActor) handlePayOrder(ctx actor.Context, cmd *pb.PayOrderCommand) {
	sender := ctx.Sender()
	if a.state == nil {
		respond(ctx, sender, newErrorResponse(ErrOrderNotFound))
		return
	}
	if !a.canTransitionTo("paid") {
		respond(ctx, sender, newErrorResponse(a.transitionError("paid")))
		return
	}

	event := &pb.OrderPaidEvent{
		OrderId: cmd.OrderId,
		PaidAt:  time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyOrderPaid(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *OrderActor) handleShipOrder(ctx actor.Context, cmd *pb.ShipOrderCommand) {
	sender := ctx.Sender()
	if a.state == nil {
		respond(ctx, sender, newErrorResponse(ErrOrderNotFound))
		return
	}
	if !a.canTransitionTo("shipped") {
		respond(ctx, sender, newErrorResponse(a.transitionError("shipped")))
		return
	}

	event := &pb.OrderShippedEvent{
		OrderId:   cmd.OrderId,
		ShippedAt: time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyOrderShipped(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *OrderActor) handleCancelOrder(ctx actor.Context, cmd *pb.CancelOrderCommand) {
	sender := ctx.Sender()
	if a.state == nil {
		respond(ctx, sender, newErrorResponse(ErrOrderNotFound))
		return
	}
	if !a.canTransitionTo("cancelled") {
		respond(ctx, sender, newErrorResponse(a.transitionError("cancelled")))
		return
	}

	event := &pb.OrderCancelledEvent{
		OrderId:     cmd.OrderId,
		Reason:      cmd.Reason,
		CancelledAt: time.Now().Format(time.RFC3339Nano),
	}
	a.PersistReceive(event)
	a.applyOrderCancelled(event)
	respond(ctx, sender, &CommandSuccess{})
}

func (a *OrderActor) applyOrderPlaced(event *pb.OrderPlacedEvent) {
	a.state = &pb.OrderSnapshot{
		Id:        event.OrderId,
		UserId:    event.UserId,
		Items:     event.Items,
		Total:     event.Total,
		Status:    "pending",
		CreatedAt: event.PlacedAt,
		UpdatedAt: event.PlacedAt,
	}
}

func (a *OrderActor) applyOrderPaid(event *pb.OrderPaidEvent) {
	if a.state == nil {
		return
	}
	a.state.Status = "paid"
	a.state.UpdatedAt = event.PaidAt
}

func (a *OrderActor) applyOrderShipped(event *pb.OrderShippedEvent) {
	if a.state == nil {
		return
	}
	a.state.Status = "shipped"
	a.state.UpdatedAt = event.ShippedAt
}

func (a *OrderActor) applyOrderCancelled(event *pb.OrderCancelledEvent) {
	if a.state == nil {
		return
	}
	a.state.Status = "cancelled"
	a.state.UpdatedAt = event.CancelledAt
}
