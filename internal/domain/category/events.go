package category

import "time"

const (
	EventCategoryCreated = "CategoryCreated"
	EventCategoryUpdated = "CategoryUpdated"
	EventCategoryDeleted = "CategoryDeleted"
)

// CategoryCreated is emitted when a new category is created
type CategoryCreated struct {
	CategoryID  string    `json:"category_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	ParentID    string    `json:"parent_id,omitempty"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
}

// CategoryUpdated is emitted when a category is updated
type CategoryUpdated struct {
	CategoryID  string    `json:"category_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	ParentID    string    `json:"parent_id,omitempty"`
	SortOrder   int       `json:"sort_order"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CategoryDeleted is emitted when a category is deleted
type CategoryDeleted struct {
	CategoryID string    `json:"category_id"`
	DeletedAt  time.Time `json:"deleted_at"`
}
