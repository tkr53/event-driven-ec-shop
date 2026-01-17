package product

import "time"

const (
	EventProductCreated          = "ProductCreated"
	EventProductUpdated          = "ProductUpdated"
	EventProductDeleted          = "ProductDeleted"
	EventProductCategoryAssigned = "ProductCategoryAssigned"
	EventProductCategoryRemoved  = "ProductCategoryRemoved"
	EventProductImageUpdated     = "ProductImageUpdated"
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

// ProductCategoryAssigned is emitted when a category is assigned to a product
type ProductCategoryAssigned struct {
	ProductID  string    `json:"product_id"`
	CategoryID string    `json:"category_id"`
	AssignedAt time.Time `json:"assigned_at"`
}

// ProductCategoryRemoved is emitted when a category is removed from a product
type ProductCategoryRemoved struct {
	ProductID string    `json:"product_id"`
	CategoryID string    `json:"category_id"`
	RemovedAt  time.Time `json:"removed_at"`
}

// ProductImageUpdated is emitted when product image is updated
type ProductImageUpdated struct {
	ProductID string    `json:"product_id"`
	ImageURL  string    `json:"image_url"`
	UpdatedAt time.Time `json:"updated_at"`
}
