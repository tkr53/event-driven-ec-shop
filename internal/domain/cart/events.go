package cart

import "time"

const (
	EventItemAdded   = "ItemAddedToCart"
	EventItemRemoved = "ItemRemovedFromCart"
	EventCartCleared = "CartCleared"
)

type ItemAddedToCart struct {
	CartID    string    `json:"cart_id"`
	UserID    string    `json:"user_id"`
	ProductID string    `json:"product_id"`
	Quantity  int       `json:"quantity"`
	Price     int       `json:"price"`
	AddedAt   time.Time `json:"added_at"`
}

type ItemRemovedFromCart struct {
	CartID    string    `json:"cart_id"`
	UserID    string    `json:"user_id"`
	ProductID string    `json:"product_id"`
	RemovedAt time.Time `json:"removed_at"`
}

type CartCleared struct {
	CartID    string    `json:"cart_id"`
	UserID    string    `json:"user_id"`
	ClearedAt time.Time `json:"cleared_at"`
}
