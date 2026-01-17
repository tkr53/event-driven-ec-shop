package order

import "time"

const (
	EventOrderPlaced    = "OrderPlaced"
	EventOrderPaid      = "OrderPaid"
	EventOrderShipped   = "OrderShipped"
	EventOrderCancelled = "OrderCancelled"
)

type OrderItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Price     int    `json:"price"`
}

type OrderPlaced struct {
	OrderID   string      `json:"order_id"`
	UserID    string      `json:"user_id"`
	Items     []OrderItem `json:"items"`
	Total     int         `json:"total"`
	PlacedAt  time.Time   `json:"placed_at"`
}

type OrderPaid struct {
	OrderID string    `json:"order_id"`
	PaidAt  time.Time `json:"paid_at"`
}

type OrderShipped struct {
	OrderID   string    `json:"order_id"`
	ShippedAt time.Time `json:"shipped_at"`
}

type OrderCancelled struct {
	OrderID     string    `json:"order_id"`
	Reason      string    `json:"reason"`
	CancelledAt time.Time `json:"cancelled_at"`
}
