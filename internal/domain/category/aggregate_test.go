package category

import (
	"context"
	"testing"

	"github.com/example/ec-event-driven/internal/infrastructure/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCategoryService() (*Service, *mocks.MockEventStore) {
	eventStore := mocks.NewMockEventStore()
	service := NewService(eventStore)
	return service, eventStore
}

// ============================================
// Slug Generation Tests
// ============================================

func TestGenerateSlug_Various(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedSlug string
	}{
		{"simple name", "Electronics", "electronics"},
		{"with spaces", "Home & Garden", "home-garden"},
		{"with underscores", "Sports_Equipment", "sports-equipment"},
		{"multiple spaces", "Men's   Clothing", "mens-clothing"},
		{"with numbers", "Category 123", "category-123"},
		{"special characters", "Books & Movies!", "books-movies"},
		{"leading/trailing spaces", "  Toys  ", "toys"},
		{"unicode characters", "日本語", ""},
		{"mixed unicode and ascii", "カテゴリー Category", "category"},
		{"multiple hyphens", "Multi---Hyphen", "multi-hyphen"},
		{"uppercase", "UPPERCASE", "uppercase"},
		{"already lowercase", "lowercase", "lowercase"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateSlug(tt.input)
			assert.Equal(t, tt.expectedSlug, result)
		})
	}
}

// ============================================
// Create Category Tests
// ============================================

func TestService_Create_ValidCategory(t *testing.T) {
	service, eventStore := newTestCategoryService()
	ctx := context.Background()

	category, err := service.Create(ctx, "Electronics", "electronics", "Electronic devices", "", 1)

	require.NoError(t, err)
	assert.NotEmpty(t, category.ID)
	assert.Equal(t, "Electronics", category.Name)
	assert.Equal(t, "electronics", category.Slug)
	assert.Equal(t, "Electronic devices", category.Description)
	assert.Empty(t, category.ParentID)
	assert.Equal(t, 1, category.SortOrder)
	assert.True(t, category.IsActive)

	// Verify event was stored
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventCategoryCreated, eventStore.AppendCalls[0].EventType)
	assert.Equal(t, AggregateType, eventStore.AppendCalls[0].AggregateType)
}

func TestService_Create_WithParentID(t *testing.T) {
	service, _ := newTestCategoryService()
	ctx := context.Background()

	category, err := service.Create(ctx, "Smartphones", "smartphones", "Mobile phones", "parent-123", 1)

	require.NoError(t, err)
	assert.Equal(t, "parent-123", category.ParentID)
}

func TestService_Create_AutoGenerateSlug(t *testing.T) {
	service, _ := newTestCategoryService()
	ctx := context.Background()

	// Empty slug should be auto-generated
	category, err := service.Create(ctx, "Home & Garden", "", "Description", "", 1)

	require.NoError(t, err)
	assert.Equal(t, "home-garden", category.Slug)
}

func TestService_Create_EmptyName(t *testing.T) {
	service, eventStore := newTestCategoryService()
	ctx := context.Background()

	category, err := service.Create(ctx, "", "slug", "Description", "", 1)

	assert.ErrorIs(t, err, ErrInvalidName)
	assert.Nil(t, category)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_Create_InvalidSlug(t *testing.T) {
	service, eventStore := newTestCategoryService()
	ctx := context.Background()

	// Slug with invalid format
	category, err := service.Create(ctx, "Name", "Invalid Slug!", "Description", "", 1)

	assert.ErrorIs(t, err, ErrInvalidSlug)
	assert.Nil(t, category)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_Create_ValidSlugFormats(t *testing.T) {
	service, _ := newTestCategoryService()
	ctx := context.Background()

	validSlugs := []string{
		"electronics",
		"home-garden",
		"category-123",
		"a",
		"abc-def-ghi",
	}

	for _, slug := range validSlugs {
		t.Run(slug, func(t *testing.T) {
			category, err := service.Create(ctx, "Name", slug, "Description", "", 1)
			require.NoError(t, err)
			assert.Equal(t, slug, category.Slug)
		})
	}
}

func TestService_Create_EmptyDescription(t *testing.T) {
	service, _ := newTestCategoryService()
	ctx := context.Background()

	category, err := service.Create(ctx, "Name", "slug", "", "", 1)

	require.NoError(t, err)
	assert.Empty(t, category.Description)
}

func TestService_Create_ZeroSortOrder(t *testing.T) {
	service, _ := newTestCategoryService()
	ctx := context.Background()

	category, err := service.Create(ctx, "Name", "slug", "Description", "", 0)

	require.NoError(t, err)
	assert.Equal(t, 0, category.SortOrder)
}

// ============================================
// Update Category Tests
// ============================================

func TestService_Update_Success(t *testing.T) {
	service, eventStore := newTestCategoryService()
	ctx := context.Background()

	categoryID := "cat-123"
	_ = eventStore.AddEvent(categoryID, AggregateType, EventCategoryCreated, CategoryCreated{CategoryID: categoryID})

	err := service.Update(ctx, categoryID, "Updated Name", "updated-slug", "Updated description", "parent-456", 2)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventCategoryUpdated, eventStore.AppendCalls[0].EventType)

	// Verify event data
	data := eventStore.AppendCalls[0].Data.(CategoryUpdated)
	assert.Equal(t, "Updated Name", data.Name)
	assert.Equal(t, "updated-slug", data.Slug)
	assert.Equal(t, "Updated description", data.Description)
	assert.Equal(t, "parent-456", data.ParentID)
	assert.Equal(t, 2, data.SortOrder)
}

func TestService_Update_AutoGenerateSlug(t *testing.T) {
	service, eventStore := newTestCategoryService()
	ctx := context.Background()

	categoryID := "cat-123"
	_ = eventStore.AddEvent(categoryID, AggregateType, EventCategoryCreated, CategoryCreated{CategoryID: categoryID})

	// Empty slug should be auto-generated
	err := service.Update(ctx, categoryID, "New Name", "", "Description", "", 1)

	require.NoError(t, err)
	data := eventStore.AppendCalls[0].Data.(CategoryUpdated)
	assert.Equal(t, "new-name", data.Slug)
}

func TestService_Update_NotFound(t *testing.T) {
	service, _ := newTestCategoryService()
	ctx := context.Background()

	err := service.Update(ctx, "non-existent", "Name", "slug", "Description", "", 1)

	assert.ErrorIs(t, err, ErrCategoryNotFound)
}

func TestService_Update_EmptyName(t *testing.T) {
	service, eventStore := newTestCategoryService()
	ctx := context.Background()

	categoryID := "cat-123"
	_ = eventStore.AddEvent(categoryID, AggregateType, EventCategoryCreated, CategoryCreated{CategoryID: categoryID})

	err := service.Update(ctx, categoryID, "", "slug", "Description", "", 1)

	assert.ErrorIs(t, err, ErrInvalidName)
}

func TestService_Update_InvalidSlug(t *testing.T) {
	service, eventStore := newTestCategoryService()
	ctx := context.Background()

	categoryID := "cat-123"
	_ = eventStore.AddEvent(categoryID, AggregateType, EventCategoryCreated, CategoryCreated{CategoryID: categoryID})

	err := service.Update(ctx, categoryID, "Name", "Invalid Slug!", "Description", "", 1)

	assert.ErrorIs(t, err, ErrInvalidSlug)
}

// ============================================
// Delete Category Tests
// ============================================

func TestService_Delete_Success(t *testing.T) {
	service, eventStore := newTestCategoryService()
	ctx := context.Background()

	categoryID := "cat-123"
	_ = eventStore.AddEvent(categoryID, AggregateType, EventCategoryCreated, CategoryCreated{CategoryID: categoryID})

	err := service.Delete(ctx, categoryID)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventCategoryDeleted, eventStore.AppendCalls[0].EventType)
}

func TestService_Delete_NotFound(t *testing.T) {
	service, _ := newTestCategoryService()
	ctx := context.Background()

	err := service.Delete(ctx, "non-existent")

	assert.ErrorIs(t, err, ErrCategoryNotFound)
}

// ============================================
// Slug Regex Tests
// ============================================

func TestSlugRegex(t *testing.T) {
	validSlugs := []string{
		"a",
		"abc",
		"abc-def",
		"a1b2c3",
		"test-123",
		"multi-word-slug",
	}

	invalidSlugs := []string{
		"",
		"-",
		"-abc",
		"abc-",
		"--abc",
		"abc--def",
		"ABC",
		"abc def",
		"abc_def",
		"abc.def",
	}

	for _, slug := range validSlugs {
		t.Run("valid: "+slug, func(t *testing.T) {
			assert.True(t, slugRegex.MatchString(slug), "Expected %s to be valid", slug)
		})
	}

	for _, slug := range invalidSlugs {
		t.Run("invalid: "+slug, func(t *testing.T) {
			assert.False(t, slugRegex.MatchString(slug), "Expected %s to be invalid", slug)
		})
	}
}
