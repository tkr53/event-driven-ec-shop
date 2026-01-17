package command

// Product Commands
type CreateProduct struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int    `json:"price"`
	Stock       int    `json:"stock"`
}

type UpdateProduct struct {
	ProductID   string `json:"product_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int    `json:"price"`
}

type DeleteProduct struct {
	ProductID string `json:"product_id"`
}

// Cart Commands
type AddToCart struct {
	UserID    string `json:"user_id"`
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type RemoveFromCart struct {
	UserID    string `json:"user_id"`
	ProductID string `json:"product_id"`
}

type ClearCart struct {
	UserID string `json:"user_id"`
}

// Order Commands
type PlaceOrder struct {
	UserID string `json:"user_id"`
}

type CancelOrder struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}
