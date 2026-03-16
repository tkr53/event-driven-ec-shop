package command

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	actorsystem "github.com/example/ec-event-driven/internal/actor"
	"github.com/example/ec-event-driven/internal/actor/aggregate"
	"github.com/example/ec-event-driven/internal/domain/cart"
	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/domain/product"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/readmodel"
	cartpb "github.com/example/ec-event-driven/proto/domain/cartpb"
	inventorypb "github.com/example/ec-event-driven/proto/domain/inventorypb"
	orderpb "github.com/example/ec-event-driven/proto/domain/orderpb"
	productpb "github.com/example/ec-event-driven/proto/domain/productpb"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
)

var actorTracer = otel.Tracer("ec-event-driven/command/actor")

const actorRequestTimeout = 5 * time.Second

// ActorHandler executes commands by sending messages to aggregate actors.
type ActorHandler struct {
	system    *actorsystem.System
	readStore store.ReadStoreInterface
}

func NewActorHandler(system *actorsystem.System, readStore store.ReadStoreInterface) *ActorHandler {
	return &ActorHandler{
		system:    system,
		readStore: readStore,
	}
}

// --- Product ---

func (h *ActorHandler) CreateProduct(ctx context.Context, cmd CreateProduct) (*product.Product, error) {
	ctx, span := actorTracer.Start(ctx, "actor.CreateProduct")
	defer span.End()

	productID := uuid.New().String()

	// 1. Product Actor に Create コマンド
	pid := h.system.GetOrSpawnProduct(productID, func() actor.Actor {
		return aggregate.NewProductActor(h.system)
	})
	if pid == nil {
		return nil, fmt.Errorf("failed to spawn product actor")
	}

	result, err := h.system.Root().RequestFuture(pid, &productpb.CreateProductCommand{
		ProductId:   productID,
		Name:        cmd.Name,
		Description: cmd.Description,
		Price:       int32(cmd.Price),
		Stock:       int32(cmd.Stock),
	}, actorRequestTimeout).Result()
	if err != nil {
		return nil, fmt.Errorf("product actor request failed: %w", err)
	}
	if errResp, ok := result.(*aggregate.ErrorResponse); ok {
		return nil, errResp.Err
	}
	resp, ok := result.(*productpb.ProductResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	// 2. Inventory Actor に AddStock コマンド
	invPid := h.system.GetOrSpawnInventory(productID, func() actor.Actor {
		return aggregate.NewInventoryActor(h.system)
	})
	if invPid == nil {
		return nil, fmt.Errorf("failed to spawn inventory actor")
	}

	invResult, err := h.system.Root().RequestFuture(invPid, &inventorypb.AddStockCommand{
		ProductId: productID,
		Quantity:  int32(cmd.Stock),
	}, actorRequestTimeout).Result()
	if err != nil {
		return nil, fmt.Errorf("inventory actor request failed: %w", err)
	}
	if errResp, ok := invResult.(*aggregate.ErrorResponse); ok {
		return nil, errResp.Err
	}

	createdAt, _ := time.Parse(time.RFC3339Nano, resp.CreatedAt)
	return &product.Product{
		ID:          resp.Id,
		Name:        resp.Name,
		Description: resp.Description,
		Price:       int(resp.Price),
		Stock:       int(resp.Stock),
		CreatedAt:   createdAt,
	}, nil
}

func (h *ActorHandler) UpdateProduct(ctx context.Context, cmd UpdateProduct) error {
	ctx, span := actorTracer.Start(ctx, "actor.UpdateProduct")
	defer span.End()

	pid := h.system.GetOrSpawnProduct(cmd.ProductID, func() actor.Actor {
		return aggregate.NewProductActor(h.system)
	})
	if pid == nil {
		return fmt.Errorf("failed to spawn product actor")
	}

	result, err := h.system.Root().RequestFuture(pid, &productpb.UpdateProductCommand{
		ProductId:   cmd.ProductID,
		Name:        cmd.Name,
		Description: cmd.Description,
		Price:       int32(cmd.Price),
	}, actorRequestTimeout).Result()
	if err != nil {
		return fmt.Errorf("product actor request failed: %w", err)
	}
	if errResp, ok := result.(*aggregate.ErrorResponse); ok {
		return errResp.Err
	}
	return nil
}

func (h *ActorHandler) DeleteProduct(ctx context.Context, cmd DeleteProduct) error {
	ctx, span := actorTracer.Start(ctx, "actor.DeleteProduct")
	defer span.End()

	pid := h.system.GetOrSpawnProduct(cmd.ProductID, func() actor.Actor {
		return aggregate.NewProductActor(h.system)
	})
	if pid == nil {
		return fmt.Errorf("failed to spawn product actor")
	}

	result, err := h.system.Root().RequestFuture(pid, &productpb.DeleteProductCommand{
		ProductId: cmd.ProductID,
	}, actorRequestTimeout).Result()
	if err != nil {
		return fmt.Errorf("product actor request failed: %w", err)
	}
	if errResp, ok := result.(*aggregate.ErrorResponse); ok {
		return errResp.Err
	}
	return nil
}

// --- Cart ---

func (h *ActorHandler) AddToCart(ctx context.Context, cmd AddToCart) error {
	ctx, span := actorTracer.Start(ctx, "actor.AddToCart")
	defer span.End()

	p, ok, err := h.readStore.Get("products", cmd.ProductID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get product", "product_id", cmd.ProductID, "error", err)
		return product.ErrProductNotFound
	}
	if !ok {
		return product.ErrProductNotFound
	}
	prod := p.(*readmodel.ProductReadModel)

	pid := h.system.GetOrSpawnCart(cmd.UserID, func() actor.Actor {
		return aggregate.NewCartActor(h.system)
	})
	if pid == nil {
		return fmt.Errorf("failed to spawn cart actor")
	}

	result, err := h.system.Root().RequestFuture(pid, &cartpb.AddItemCommand{
		UserId:    cmd.UserID,
		ProductId: cmd.ProductID,
		Quantity:  int32(cmd.Quantity),
		Price:     int32(prod.Price),
	}, actorRequestTimeout).Result()
	if err != nil {
		return fmt.Errorf("cart actor request failed: %w", err)
	}
	if errResp, ok := result.(*aggregate.ErrorResponse); ok {
		return errResp.Err
	}
	return nil
}

func (h *ActorHandler) RemoveFromCart(ctx context.Context, cmd RemoveFromCart) error {
	ctx, span := actorTracer.Start(ctx, "actor.RemoveFromCart")
	defer span.End()

	pid := h.system.GetOrSpawnCart(cmd.UserID, func() actor.Actor {
		return aggregate.NewCartActor(h.system)
	})
	if pid == nil {
		return fmt.Errorf("failed to spawn cart actor")
	}

	result, err := h.system.Root().RequestFuture(pid, &cartpb.RemoveItemCommand{
		UserId:    cmd.UserID,
		ProductId: cmd.ProductID,
	}, actorRequestTimeout).Result()
	if err != nil {
		return fmt.Errorf("cart actor request failed: %w", err)
	}
	if errResp, ok := result.(*aggregate.ErrorResponse); ok {
		return errResp.Err
	}
	return nil
}

func (h *ActorHandler) ClearCart(ctx context.Context, cmd ClearCart) error {
	pid := h.system.GetOrSpawnCart(cmd.UserID, func() actor.Actor {
		return aggregate.NewCartActor(h.system)
	})
	if pid == nil {
		return fmt.Errorf("failed to spawn cart actor")
	}

	result, err := h.system.Root().RequestFuture(pid, &cartpb.ClearCartCommand{
		UserId: cmd.UserID,
	}, actorRequestTimeout).Result()
	if err != nil {
		return fmt.Errorf("cart actor request failed: %w", err)
	}
	if errResp, ok := result.(*aggregate.ErrorResponse); ok {
		return errResp.Err
	}
	return nil
}

// --- Order ---

func (h *ActorHandler) PlaceOrder(ctx context.Context, cmd PlaceOrder) (*order.Order, error) {
	ctx, span := actorTracer.Start(ctx, "actor.PlaceOrder")
	defer span.End()

	// カート取得
	cartID := cart.GetCartID(cmd.UserID)
	c, ok, err := h.readStore.Get("carts", cartID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get cart", "cart_id", cartID, "error", err)
		return nil, order.ErrEmptyOrder
	}
	if !ok || len(c.(*readmodel.CartReadModel).Items) == 0 {
		return nil, order.ErrEmptyOrder
	}
	cartModel := c.(*readmodel.CartReadModel)

	var pbItems []*orderpb.OrderItem
	for _, item := range cartModel.Items {
		pbItems = append(pbItems, &orderpb.OrderItem{
			ProductId: item.ProductID,
			Name:      item.Name,
			Quantity:  int32(item.Quantity),
			Price:     int32(item.Price),
		})
	}

	// 在庫確認
	for _, item := range pbItems {
		inv, ok, err := h.readStore.Get("inventory", item.ProductId)
		if err != nil || !ok {
			return nil, fmt.Errorf("inventory not found for product %s", item.ProductId)
		}
		invModel := inv.(*readmodel.InventoryReadModel)
		if invModel.AvailableStock < int(item.Quantity) {
			return nil, fmt.Errorf("insufficient stock: product %s has only %d available, requested %d",
				item.ProductId, invModel.AvailableStock, item.Quantity)
		}
	}

	orderID := uuid.New().String()

	// 1. Order Actor に PlaceOrder
	orderPid := h.system.GetOrSpawnOrder(orderID, func() actor.Actor {
		return aggregate.NewOrderActor(h.system)
	})
	orderResult, err := h.system.Root().RequestFuture(orderPid, &orderpb.PlaceOrderCommand{
		UserId: cmd.UserID,
		Items:  pbItems,
	}, actorRequestTimeout).Result()
	if err != nil {
		return nil, fmt.Errorf("order actor request failed: %w", err)
	}
	if errResp, ok := orderResult.(*aggregate.ErrorResponse); ok {
		return nil, errResp.Err
	}
	orderSnapshot, ok := orderResult.(*orderpb.OrderSnapshot)
	if !ok {
		return nil, fmt.Errorf("unexpected order response: %T", orderResult)
	}

	// 2. 在庫予約（補償トランザクション付き）
	var reservedItems []*orderpb.OrderItem
	for _, item := range pbItems {
		invPid := h.system.GetOrSpawnInventory(item.ProductId, func() actor.Actor {
			return aggregate.NewInventoryActor(h.system)
		})
		reserveResult, err := h.system.Root().RequestFuture(invPid, &inventorypb.ReserveStockCommand{
			ProductId: item.ProductId,
			OrderId:   orderID,
			Quantity:  item.Quantity,
		}, actorRequestTimeout).Result()
		if err != nil || isActorError(reserveResult) {
			h.compensatePlaceOrder(orderID, reservedItems)
			reserveErr := err
			if errResp, ok := reserveResult.(*aggregate.ErrorResponse); ok {
				reserveErr = errResp.Err
			}
			return nil, fmt.Errorf("failed to reserve inventory for product %s: %w", item.ProductId, reserveErr)
		}
		reservedItems = append(reservedItems, item)
	}

	// 3. カートクリア
	cartPid := h.system.GetOrSpawnCart(cmd.UserID, func() actor.Actor {
		return aggregate.NewCartActor(h.system)
	})
	clearResult, err := h.system.Root().RequestFuture(cartPid, &cartpb.ClearCartCommand{
		UserId: cmd.UserID,
	}, actorRequestTimeout).Result()
	if err != nil {
		slog.WarnContext(ctx, "failed to clear cart after order", "user_id", cmd.UserID, "error", err)
	} else if errResp, ok := clearResult.(*aggregate.ErrorResponse); ok {
		slog.WarnContext(ctx, "failed to clear cart after order", "user_id", cmd.UserID, "error", errResp.Err)
	}

	// OrderSnapshot → order.Order 変換
	createdAt, _ := time.Parse(time.RFC3339Nano, orderSnapshot.CreatedAt)
	var items []order.OrderItem
	for _, item := range orderSnapshot.Items {
		items = append(items, order.OrderItem{
			ProductID: item.ProductId,
			Name:      item.Name,
			Quantity:  int(item.Quantity),
			Price:     int(item.Price),
		})
	}
	return &order.Order{
		ID:        orderSnapshot.Id,
		UserID:    orderSnapshot.UserId,
		Items:     items,
		Total:     int(orderSnapshot.Total),
		Status:    order.Status(orderSnapshot.Status),
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}, nil
}

func (h *ActorHandler) PayOrder(ctx context.Context, cmd PayOrder) error {
	ctx, span := actorTracer.Start(ctx, "actor.PayOrder")
	defer span.End()

	// 注文から items を取得（在庫確定用）
	o, ok, err := h.readStore.Get("orders", cmd.OrderID)
	if err != nil || !ok {
		return order.ErrOrderNotFound
	}
	orderModel := o.(*readmodel.OrderReadModel)

	// 在庫確定（Reserved → Sold）
	for _, item := range orderModel.Items {
		invPid := h.system.GetOrSpawnInventory(item.ProductID, func() actor.Actor {
			return aggregate.NewInventoryActor(h.system)
		})
		result, err := h.system.Root().RequestFuture(invPid, &inventorypb.DeductStockCommand{
			ProductId: item.ProductID,
			OrderId:   cmd.OrderID,
			Quantity:  int32(item.Quantity),
		}, actorRequestTimeout).Result()
		if err != nil {
			return fmt.Errorf("failed to deduct inventory for product %s: %w", item.ProductID, err)
		}
		if errResp, ok := result.(*aggregate.ErrorResponse); ok {
			return fmt.Errorf("failed to deduct inventory for product %s: %w", item.ProductID, errResp.Err)
		}
	}

	// 支払い処理
	orderPid := h.system.GetOrSpawnOrder(cmd.OrderID, func() actor.Actor {
		return aggregate.NewOrderActor(h.system)
	})
	result, err := h.system.Root().RequestFuture(orderPid, &orderpb.PayOrderCommand{
		OrderId: cmd.OrderID,
	}, actorRequestTimeout).Result()
	if err != nil {
		return fmt.Errorf("order actor request failed: %w", err)
	}
	if errResp, ok := result.(*aggregate.ErrorResponse); ok {
		return errResp.Err
	}
	return nil
}

func (h *ActorHandler) ShipOrder(ctx context.Context, cmd ShipOrder) error {
	ctx, span := actorTracer.Start(ctx, "actor.ShipOrder")
	defer span.End()

	pid := h.system.GetOrSpawnOrder(cmd.OrderID, func() actor.Actor {
		return aggregate.NewOrderActor(h.system)
	})
	result, err := h.system.Root().RequestFuture(pid, &orderpb.ShipOrderCommand{
		OrderId: cmd.OrderID,
	}, actorRequestTimeout).Result()
	if err != nil {
		return fmt.Errorf("order actor request failed: %w", err)
	}
	if errResp, ok := result.(*aggregate.ErrorResponse); ok {
		return errResp.Err
	}
	return nil
}

func (h *ActorHandler) CancelOrder(ctx context.Context, cmd CancelOrder) error {
	ctx, span := actorTracer.Start(ctx, "actor.CancelOrder")
	defer span.End()

	// 在庫解放
	o, ok, err := h.readStore.Get("orders", cmd.OrderID)
	if err != nil || !ok {
		return order.ErrOrderNotFound
	}
	orderModel := o.(*readmodel.OrderReadModel)

	for _, item := range orderModel.Items {
		invPid := h.system.GetOrSpawnInventory(item.ProductID, func() actor.Actor {
			return aggregate.NewInventoryActor(h.system)
		})
		result, err := h.system.Root().RequestFuture(invPid, &inventorypb.ReleaseStockCommand{
			ProductId: item.ProductID,
			OrderId:   cmd.OrderID,
			Quantity:  int32(item.Quantity),
		}, actorRequestTimeout).Result()
		if err != nil {
			return fmt.Errorf("failed to release inventory: %w", err)
		}
		if errResp, ok := result.(*aggregate.ErrorResponse); ok {
			return errResp.Err
		}
	}

	// 注文キャンセル
	orderPid := h.system.GetOrSpawnOrder(cmd.OrderID, func() actor.Actor {
		return aggregate.NewOrderActor(h.system)
	})
	result, err := h.system.Root().RequestFuture(orderPid, &orderpb.CancelOrderCommand{
		OrderId: cmd.OrderID,
		Reason:  cmd.Reason,
	}, actorRequestTimeout).Result()
	if err != nil {
		return fmt.Errorf("order actor request failed: %w", err)
	}
	if errResp, ok := result.(*aggregate.ErrorResponse); ok {
		return errResp.Err
	}
	return nil
}

// --- helpers ---

func (h *ActorHandler) compensatePlaceOrder(orderID string, reservedItems []*orderpb.OrderItem) {
	for _, item := range reservedItems {
		invPid := h.system.GetOrSpawnInventory(item.ProductId, func() actor.Actor {
			return aggregate.NewInventoryActor(h.system)
		})
		result, err := h.system.Root().RequestFuture(invPid, &inventorypb.ReleaseStockCommand{
			ProductId: item.ProductId,
			OrderId:   orderID,
			Quantity:  item.Quantity,
		}, actorRequestTimeout).Result()
		if err != nil {
			slog.Error("compensation: failed to release inventory", "product_id", item.ProductId, "error", err)
		} else if errResp, ok := result.(*aggregate.ErrorResponse); ok {
			slog.Error("compensation: failed to release inventory", "product_id", item.ProductId, "error", errResp.Err)
		}
	}

	orderPid := h.system.GetOrSpawnOrder(orderID, func() actor.Actor {
		return aggregate.NewOrderActor(h.system)
	})
	result, err := h.system.Root().RequestFuture(orderPid, &orderpb.CancelOrderCommand{
		OrderId: orderID,
		Reason:  "inventory reservation failed",
	}, actorRequestTimeout).Result()
	if err != nil {
		slog.Error("compensation: failed to cancel order", "order_id", orderID, "error", err)
	} else if errResp, ok := result.(*aggregate.ErrorResponse); ok {
		slog.Error("compensation: failed to cancel order", "order_id", orderID, "error", errResp.Err)
	}
}

func isActorError(result interface{}) bool {
	_, ok := result.(*aggregate.ErrorResponse)
	return ok
}
