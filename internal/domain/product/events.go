package product

import "time"

const (
	EventProductCreated = "ProductCreated"
	EventProductUpdated = "ProductUpdated"
	EventProductDeleted = "ProductDeleted"
)

type ProductCreated struct {
	ProductID   string    `json:"product_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       int       `json:"price"`
	Stock       int       `json:"stock"`
	CreatedAt   time.Time `json:"created_at"`
}

type ProductUpdated struct {
	ProductID   string    `json:"product_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       int       `json:"price"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProductDeleted struct {
	ProductID string    `json:"product_id"`
	DeletedAt time.Time `json:"deleted_at"`
}
