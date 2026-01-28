package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/example/ec-event-driven/internal/domain/category"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/readmodel"
)

// CategoryHandlers handles category-related HTTP requests
type CategoryHandlers struct {
	categoryService *category.Service
	readStore       *store.PostgresReadStore
}

// NewCategoryHandlers creates a new CategoryHandlers instance
func NewCategoryHandlers(categoryService *category.Service, readStore *store.PostgresReadStore) *CategoryHandlers {
	return &CategoryHandlers{
		categoryService: categoryService,
		readStore:       readStore,
	}
}

// CreateCategoryRequest represents the request body for creating a category
type CreateCategoryRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug,omitempty"`
	Description string `json:"description,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
	SortOrder   int    `json:"sort_order,omitempty"`
}

// CategoryResponse represents a category in API responses
type CategoryResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Slug        string                 `json:"slug"`
	Description string                 `json:"description"`
	ParentID    string                 `json:"parent_id,omitempty"`
	SortOrder   int                    `json:"sort_order"`
	Children    []CategoryResponse     `json:"children,omitempty"`
}

// ListCategories returns all categories
func (h *CategoryHandlers) ListCategories(w http.ResponseWriter, r *http.Request) {
	allCategories, err := h.readStore.GetAll("categories")
	if err != nil {
		log.Printf("[API] Error getting categories: %v", err)
		respondJSONError(w, "Failed to fetch categories", http.StatusInternalServerError)
		return
	}

	// Build a tree structure
	categoryMap := make(map[string]*CategoryResponse)
	var rootCategories []CategoryResponse

	// First pass: create all category responses
	for _, c := range allCategories {
		cat := c.(*readmodel.CategoryReadModel)
		categoryMap[cat.ID] = &CategoryResponse{
			ID:          cat.ID,
			Name:        cat.Name,
			Slug:        cat.Slug,
			Description: cat.Description,
			ParentID:    cat.ParentID,
			SortOrder:   cat.SortOrder,
			Children:    []CategoryResponse{},
		}
	}

	// Second pass: build tree structure
	for _, c := range allCategories {
		cat := c.(*readmodel.CategoryReadModel)
		if cat.ParentID == "" {
			rootCategories = append(rootCategories, *categoryMap[cat.ID])
		} else if parent, exists := categoryMap[cat.ParentID]; exists {
			parent.Children = append(parent.Children, *categoryMap[cat.ID])
		}
	}

	respondJSON(w, http.StatusOK, rootCategories)
}

// GetCategory returns a single category by slug
func (h *CategoryHandlers) GetCategory(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/api/categories/")

	cat, exists := h.readStore.GetCategoryBySlug(slug)
	if !exists {
		respondJSONError(w, "Category not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, CategoryResponse{
		ID:          cat.ID,
		Name:        cat.Name,
		Slug:        cat.Slug,
		Description: cat.Description,
		ParentID:    cat.ParentID,
		SortOrder:   cat.SortOrder,
	})
}

// CreateCategory creates a new category (admin only)
func (h *CategoryHandlers) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cat, err := h.categoryService.Create(r.Context(), req.Name, req.Slug, req.Description, req.ParentID, req.SortOrder)
	if err != nil {
		respondJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusCreated, CategoryResponse{
		ID:          cat.ID,
		Name:        cat.Name,
		Slug:        cat.Slug,
		Description: cat.Description,
		ParentID:    cat.ParentID,
		SortOrder:   cat.SortOrder,
	})
}

// UpdateCategory updates an existing category (admin only)
func (h *CategoryHandlers) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	categoryID := strings.TrimPrefix(r.URL.Path, "/api/categories/")

	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.categoryService.Update(r.Context(), categoryID, req.Name, req.Slug, req.Description, req.ParentID, req.SortOrder)
	if err != nil {
		if err == category.ErrCategoryNotFound {
			respondJSONError(w, "Category not found", http.StatusNotFound)
			return
		}
		respondJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Category updated"})
}

// DeleteCategory deletes a category (admin only)
func (h *CategoryHandlers) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	categoryID := strings.TrimPrefix(r.URL.Path, "/api/categories/")

	err := h.categoryService.Delete(r.Context(), categoryID)
	if err != nil {
		if err == category.ErrCategoryNotFound {
			respondJSONError(w, "Category not found", http.StatusNotFound)
			return
		}
		respondJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Category deleted"})
}

// GetProductsByCategory returns products in a category
func (h *CategoryHandlers) GetProductsByCategory(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/api/products/category/")

	cat, exists := h.readStore.GetCategoryBySlug(slug)
	if !exists {
		respondJSONError(w, "Category not found", http.StatusNotFound)
		return
	}

	products := h.readStore.GetProductsByCategory(cat.ID)
	respondJSON(w, http.StatusOK, products)
}

// SearchProducts handles product search with filters
func (h *CategoryHandlers) SearchProducts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	params := store.SearchProductsParams{
		Query:      query.Get("q"),
		CategoryID: query.Get("category"),
	}

	if minPrice := query.Get("min_price"); minPrice != "" {
		if val, err := strconv.Atoi(minPrice); err == nil {
			params.MinPrice = val
		}
	}

	if maxPrice := query.Get("max_price"); maxPrice != "" {
		if val, err := strconv.Atoi(maxPrice); err == nil {
			params.MaxPrice = val
		}
	}

	if limit := query.Get("limit"); limit != "" {
		if val, err := strconv.Atoi(limit); err == nil {
			params.Limit = val
		}
	}

	if offset := query.Get("offset"); offset != "" {
		if val, err := strconv.Atoi(offset); err == nil {
			params.Offset = val
		}
	}

	products := h.readStore.SearchProducts(params)
	respondJSON(w, http.StatusOK, products)
}
