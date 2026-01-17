package readmodel

import "time"

// ProductReadModel is the read model for products
type ProductReadModel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       int       `json:"price"`
	Stock       int       `json:"stock"`
	ImageURL    string    `json:"image_url,omitempty"`
	CategoryIDs []string  `json:"category_ids,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CartItemReadModel represents an item in the cart
type CartItemReadModel struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Price     int    `json:"price"`
}

// CartReadModel is the read model for shopping cart
type CartReadModel struct {
	ID     string              `json:"id"`
	UserID string              `json:"user_id"`
	Items  []CartItemReadModel `json:"items"`
	Total  int                 `json:"total"`
}

// OrderItemReadModel represents an item in an order
type OrderItemReadModel struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Price     int    `json:"price"`
}

// OrderReadModel is the read model for orders
type OrderReadModel struct {
	ID        string               `json:"id"`
	UserID    string               `json:"user_id"`
	Items     []OrderItemReadModel `json:"items"`
	Total     int                  `json:"total"`
	Status    string               `json:"status"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// InventoryReadModel is the read model for inventory
type InventoryReadModel struct {
	ProductID      string `json:"product_id"`
	TotalStock     int    `json:"total_stock"`
	ReservedStock  int    `json:"reserved_stock"`
	AvailableStock int    `json:"available_stock"`
}

// UserReadModel is the read model for users
type UserReadModel struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Never expose in JSON
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SessionReadModel is the read model for user sessions
type SessionReadModel struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	RefreshTokenHash string    `json:"-"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
	IPAddress        string    `json:"ip_address"`
	UserAgent        string    `json:"user_agent"`
}

// CategoryReadModel is the read model for product categories
type CategoryReadModel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	ParentID    string    `json:"parent_id,omitempty"`
	SortOrder   int       `json:"sort_order"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProductCategoryReadModel represents the relationship between products and categories
type ProductCategoryReadModel struct {
	ProductID  string `json:"product_id"`
	CategoryID string `json:"category_id"`
}
