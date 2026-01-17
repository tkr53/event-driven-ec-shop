package inventory

import "time"

const (
	EventStockAdded    = "StockAdded"
	EventStockReserved = "StockReserved"
	EventStockReleased = "StockReleased"
	EventStockDeducted = "StockDeducted"
)

type StockAdded struct {
	ProductID string    `json:"product_id"`
	Quantity  int       `json:"quantity"`
	AddedAt   time.Time `json:"added_at"`
}

type StockReserved struct {
	ProductID string    `json:"product_id"`
	OrderID   string    `json:"order_id"`
	Quantity  int       `json:"quantity"`
	ReservedAt time.Time `json:"reserved_at"`
}

type StockReleased struct {
	ProductID  string    `json:"product_id"`
	OrderID    string    `json:"order_id"`
	Quantity   int       `json:"quantity"`
	ReleasedAt time.Time `json:"released_at"`
}

type StockDeducted struct {
	ProductID  string    `json:"product_id"`
	OrderID    string    `json:"order_id"`
	Quantity   int       `json:"quantity"`
	DeductedAt time.Time `json:"deducted_at"`
}
