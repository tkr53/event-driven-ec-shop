package category

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/google/uuid"
)

const AggregateType = "Category"

var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrInvalidName      = errors.New("name is required")
	ErrInvalidSlug      = errors.New("invalid slug format")
)

// slugRegex validates slug format (lowercase letters, numbers, hyphens)
var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Category represents a product category
type Category struct {
	ID          string
	Name        string
	Slug        string
	Description string
	ParentID    string
	SortOrder   int
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Service handles category domain operations
type Service struct {
	eventStore store.EventStoreInterface
}

// NewService creates a new category service
func NewService(es store.EventStoreInterface) *Service {
	return &Service{eventStore: es}
}

// Create creates a new category
func (s *Service) Create(ctx context.Context, name, slug, description, parentID string, sortOrder int) (*Category, error) {
	if name == "" {
		return nil, ErrInvalidName
	}

	// Generate slug from name if not provided
	if slug == "" {
		slug = generateSlug(name)
	}

	if !slugRegex.MatchString(slug) {
		return nil, ErrInvalidSlug
	}

	categoryID := uuid.New().String()
	now := time.Now()

	event := CategoryCreated{
		CategoryID:  categoryID,
		Name:        name,
		Slug:        slug,
		Description: description,
		ParentID:    parentID,
		SortOrder:   sortOrder,
		CreatedAt:   now,
	}

	_, err := s.eventStore.Append(ctx, categoryID, AggregateType, EventCategoryCreated, event)
	if err != nil {
		return nil, err
	}

	return &Category{
		ID:          categoryID,
		Name:        name,
		Slug:        slug,
		Description: description,
		ParentID:    parentID,
		SortOrder:   sortOrder,
		IsActive:    true,
		CreatedAt:   now,
	}, nil
}

// Update updates an existing category
func (s *Service) Update(ctx context.Context, categoryID, name, slug, description, parentID string, sortOrder int) error {
	if name == "" {
		return ErrInvalidName
	}

	events := s.eventStore.GetEvents(categoryID)
	if len(events) == 0 {
		return ErrCategoryNotFound
	}

	if slug == "" {
		slug = generateSlug(name)
	}

	if !slugRegex.MatchString(slug) {
		return ErrInvalidSlug
	}

	event := CategoryUpdated{
		CategoryID:  categoryID,
		Name:        name,
		Slug:        slug,
		Description: description,
		ParentID:    parentID,
		SortOrder:   sortOrder,
		UpdatedAt:   time.Now(),
	}

	_, err := s.eventStore.Append(ctx, categoryID, AggregateType, EventCategoryUpdated, event)
	return err
}

// Delete deletes a category
func (s *Service) Delete(ctx context.Context, categoryID string) error {
	events := s.eventStore.GetEvents(categoryID)
	if len(events) == 0 {
		return ErrCategoryNotFound
	}

	event := CategoryDeleted{
		CategoryID: categoryID,
		DeletedAt:  time.Now(),
	}

	_, err := s.eventStore.Append(ctx, categoryID, AggregateType, EventCategoryDeleted, event)
	return err
}

// generateSlug creates a URL-friendly slug from a name
func generateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)
	// Replace spaces and underscores with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	// Remove any characters that aren't alphanumeric or hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	slug = reg.ReplaceAllString(slug, "")
	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")
	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")
	return slug
}
