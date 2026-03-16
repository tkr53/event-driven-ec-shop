package saga

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	actorsystem "github.com/example/ec-event-driven/internal/actor"
	"github.com/example/ec-event-driven/internal/actor/aggregate"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/readmodel"
	inventorypb "github.com/example/ec-event-driven/proto/domain/inventorypb"
	orderpb "github.com/example/ec-event-driven/proto/domain/orderpb"
)

const sagaTimeout = 10 * time.Second

// StartPlaceOrderSaga is the message that initiates the saga.
type StartPlaceOrderSaga struct {
	OrderID string
	UserID  string
	Items   []*orderpb.OrderItem
}

// PlaceOrderResult is the response returned to the caller.
type PlaceOrderResult struct {
	OrderSnapshot *orderpb.OrderSnapshot
	Err           error
}

// PlaceOrderSaga orchestrates the order placement process:
// 1. Place order (Order Actor)
// 2. Reserve inventory for each item (Inventory Actors)
// 3. On failure: release reserved inventory + cancel order (compensation)
type PlaceOrderSaga struct {
	system    *actorsystem.System
	readStore store.ReadStoreInterface
}

func NewPlaceOrderSaga(system *actorsystem.System, readStore store.ReadStoreInterface) *PlaceOrderSaga {
	return &PlaceOrderSaga{system: system, readStore: readStore}
}

func (s *PlaceOrderSaga) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *StartPlaceOrderSaga:
		s.execute(ctx, msg)
	}
}

func (s *PlaceOrderSaga) execute(ctx actor.Context, msg *StartPlaceOrderSaga) {
	sender := ctx.Sender()

	// Validate stock availability from read store
	for _, item := range msg.Items {
		inv, ok, err := s.readStore.Get("inventory", item.ProductId)
		if err != nil || !ok {
			respond := &PlaceOrderResult{
				Err: fmt.Errorf("inventory not found for product %s", item.ProductId),
			}
			if sender != nil {
				ctx.Send(sender, respond)
			}
			ctx.Poison(ctx.Self())
			return
		}
		invModel := inv.(*readmodel.InventoryReadModel)
		if invModel.AvailableStock < int(item.Quantity) {
			resp := &PlaceOrderResult{
				Err: fmt.Errorf("insufficient stock: product %s has only %d available, requested %d",
					item.ProductId, invModel.AvailableStock, item.Quantity),
			}
			if sender != nil {
				ctx.Send(sender, resp)
			}
			ctx.Poison(ctx.Self())
			return
		}
	}

	// 1. Place order via Order Actor
	orderPid := s.system.GetOrSpawnOrder(msg.OrderID, func() actor.Actor {
		return aggregate.NewOrderActor(s.system)
	})

	placeCmd := &orderpb.PlaceOrderCommand{
		UserId: msg.UserID,
		Items:  msg.Items,
	}
	orderResult, err := s.system.Root().RequestFuture(orderPid, placeCmd, sagaTimeout).Result()
	if err != nil {
		if sender != nil {
			ctx.Send(sender, &PlaceOrderResult{Err: fmt.Errorf("order placement failed: %w", err)})
		}
		ctx.Poison(ctx.Self())
		return
	}
	if errResp, ok := orderResult.(*aggregate.ErrorResponse); ok {
		if sender != nil {
			ctx.Send(sender, &PlaceOrderResult{Err: errResp.Err})
		}
		ctx.Poison(ctx.Self())
		return
	}

	orderSnapshot, ok := orderResult.(*orderpb.OrderSnapshot)
	if !ok {
		if sender != nil {
			ctx.Send(sender, &PlaceOrderResult{Err: fmt.Errorf("unexpected order response: %T", orderResult)})
		}
		ctx.Poison(ctx.Self())
		return
	}

	// 2. Reserve inventory for each item
	var reservedItems []*orderpb.OrderItem
	for _, item := range msg.Items {
		invPid := s.system.GetOrSpawnInventory(item.ProductId, func() actor.Actor {
			return aggregate.NewInventoryActor(s.system)
		})

		reserveCmd := &inventorypb.ReserveStockCommand{
			ProductId: item.ProductId,
			OrderId:   msg.OrderID,
			Quantity:  item.Quantity,
		}
		reserveResult, err := s.system.Root().RequestFuture(invPid, reserveCmd, sagaTimeout).Result()
		if err != nil || isErrorResponse(reserveResult) {
			// Compensation: release already reserved items
			s.compensate(msg.OrderID, reservedItems)

			reserveErr := err
			if errResp, ok := reserveResult.(*aggregate.ErrorResponse); ok {
				reserveErr = errResp.Err
			}
			if sender != nil {
				ctx.Send(sender, &PlaceOrderResult{
					Err: fmt.Errorf("failed to reserve inventory for product %s: %w", item.ProductId, reserveErr),
				})
			}
			ctx.Poison(ctx.Self())
			return
		}
		reservedItems = append(reservedItems, item)
	}

	// Success
	if sender != nil {
		ctx.Send(sender, &PlaceOrderResult{OrderSnapshot: orderSnapshot})
	}
	ctx.Poison(ctx.Self())
}

// compensate releases reserved inventory and cancels the order.
func (s *PlaceOrderSaga) compensate(orderID string, reservedItems []*orderpb.OrderItem) {
	for _, item := range reservedItems {
		invPid := s.system.GetOrSpawnInventory(item.ProductId, func() actor.Actor {
			return aggregate.NewInventoryActor(s.system)
		})
		releaseCmd := &inventorypb.ReleaseStockCommand{
			ProductId: item.ProductId,
			OrderId:   orderID,
			Quantity:  item.Quantity,
		}
		result, err := s.system.Root().RequestFuture(invPid, releaseCmd, sagaTimeout).Result()
		if err != nil {
			slog.Error("failed to release inventory during compensation", "product_id", item.ProductId, "error", err)
		} else if errResp, ok := result.(*aggregate.ErrorResponse); ok {
			slog.Error("failed to release inventory during compensation", "product_id", item.ProductId, "error", errResp.Err)
		}
	}

	// Cancel the order
	orderPid := s.system.GetOrSpawnOrder(orderID, func() actor.Actor {
		return aggregate.NewOrderActor(s.system)
	})
	cancelCmd := &orderpb.CancelOrderCommand{
		OrderId: orderID,
		Reason:  "inventory reservation failed",
	}
	result, err := s.system.Root().RequestFuture(orderPid, cancelCmd, sagaTimeout).Result()
	if err != nil {
		slog.Error("failed to cancel order during compensation", "order_id", orderID, "error", err)
	} else if errResp, ok := result.(*aggregate.ErrorResponse); ok {
		slog.Error("failed to cancel order during compensation", "order_id", orderID, "error", errResp.Err)
	}
}

func isErrorResponse(result interface{}) bool {
	_, ok := result.(*aggregate.ErrorResponse)
	return ok
}
