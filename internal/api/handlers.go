package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/example/ec-event-driven/internal/api/middleware"
	"github.com/example/ec-event-driven/internal/command"
	"github.com/example/ec-event-driven/internal/query"
)

type Handlers struct {
	cmdHandler   command.CommandHandler
	queryHandler *query.Handler
}

func NewHandlers(cmdHandler command.CommandHandler, queryHandler *query.Handler) *Handlers {
	return &Handlers{
		cmdHandler:   cmdHandler,
		queryHandler: queryHandler,
	}
}

// Product Handlers

func (h *Handlers) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var cmd command.CreateProduct
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		respondJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	product, err := h.cmdHandler.CreateProduct(r.Context(), cmd)
	if err != nil {
		respondJSONError(w, "Failed to create product", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, product)
}

func (h *Handlers) GetProducts(w http.ResponseWriter, r *http.Request) {
	products := h.queryHandler.ListProducts(r.Context())
	respondJSON(w, http.StatusOK, products)
}

func (h *Handlers) GetProduct(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r.URL.Path, "/products/")
	product, ok := h.queryHandler.GetProduct(r.Context(), id)
	if !ok {
		respondJSONError(w, "Product not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, product)
}

func (h *Handlers) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r.URL.Path, "/products/")

	var cmd command.UpdateProduct
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		respondJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	cmd.ProductID = id

	if err := h.cmdHandler.UpdateProduct(r.Context(), cmd); err != nil {
		respondJSONError(w, "Failed to update product", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Product updated"})
}

func (h *Handlers) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r.URL.Path, "/products/")

	cmd := command.DeleteProduct{ProductID: id}
	if err := h.cmdHandler.DeleteProduct(r.Context(), cmd); err != nil {
		respondJSONError(w, "Failed to delete product", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Product deleted"})
}

// Cart Handlers

func (h *Handlers) AddToCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cmd := command.AddToCart{
		UserID:    userID,
		ProductID: req.ProductID,
		Quantity:  req.Quantity,
	}
	if err := h.cmdHandler.AddToCart(r.Context(), cmd); err != nil {
		slog.ErrorContext(r.Context(), "AddToCart failed", "error", err)
		respondJSONError(w, "Failed to add item to cart", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	productID := extractPathParam(r.URL.Path, "/cart/items/")
	cmd := command.RemoveFromCart{
		UserID:    userID,
		ProductID: productID,
	}
	if err := h.cmdHandler.RemoveFromCart(r.Context(), cmd); err != nil {
		respondJSONError(w, "Failed to remove item from cart", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) GetCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	cart, _ := h.queryHandler.GetCart(r.Context(), userID)
	respondJSON(w, http.StatusOK, cart)
}

// Order Handlers

func (h *Handlers) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	cmd := command.PlaceOrder{UserID: userID}
	order, err := h.cmdHandler.PlaceOrder(r.Context(), cmd)
	if err != nil {
		respondJSONError(w, "Failed to place order", http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusCreated, order)
}

func (h *Handlers) GetOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	orders := h.queryHandler.ListOrdersByUser(r.Context(), userID)
	respondJSON(w, http.StatusOK, orders)
}

func (h *Handlers) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r.URL.Path, "/orders/")
	// Remove /cancel suffix if present
	id = strings.TrimSuffix(id, "/cancel")

	order, ok := h.queryHandler.GetOrder(r.Context(), id)
	if !ok {
		respondJSONError(w, "Order not found", http.StatusNotFound)
		return
	}

	// Authorization check: user can only access their own orders (admins can access all)
	userID := getUserID(r)
	if order.UserID != userID && !isAdmin(r) {
		respondJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}

	respondJSON(w, http.StatusOK, order)
}

func (h *Handlers) CancelOrder(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/orders/")
	id := strings.TrimSuffix(path, "/cancel")

	// Authorization check: user can only cancel their own orders (admins can cancel all)
	order, ok := h.queryHandler.GetOrder(r.Context(), id)
	if !ok {
		respondJSONError(w, "Order not found", http.StatusNotFound)
		return
	}

	userID := getUserID(r)
	if order.UserID != userID && !isAdmin(r) {
		respondJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	cmd := command.CancelOrder{
		OrderID: id,
		Reason:  req.Reason,
	}
	if err := h.cmdHandler.CancelOrder(r.Context(), cmd); err != nil {
		respondJSONError(w, "Failed to cancel order", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Admin Order Actions

func (h *Handlers) PayOrder(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r.URL.Path, "/api/admin/orders/")
	id = strings.TrimSuffix(id, "/pay")

	cmd := command.PayOrder{OrderID: id}
	if err := h.cmdHandler.PayOrder(r.Context(), cmd); err != nil {
		slog.ErrorContext(r.Context(), "PayOrder failed", "order_id", id, "error", err)
		respondJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Order paid"})
}

func (h *Handlers) ShipOrder(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r.URL.Path, "/api/admin/orders/")
	id = strings.TrimSuffix(id, "/ship")

	cmd := command.ShipOrder{OrderID: id}
	if err := h.cmdHandler.ShipOrder(r.Context(), cmd); err != nil {
		slog.ErrorContext(r.Context(), "ShipOrder failed", "order_id", id, "error", err)
		respondJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Order shipped"})
}

// Admin Handlers

func (h *Handlers) GetAllOrders(w http.ResponseWriter, r *http.Request) {
	orders := h.queryHandler.ListAllOrders(r.Context())
	respondJSON(w, http.StatusOK, orders)
}

// Helper functions

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func extractPathParam(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}

// getUserID extracts user ID from JWT context or X-User-ID header for anonymous cart
func getUserID(r *http.Request) string {
	// First try to get from JWT context (authenticated user)
	if userID := middleware.GetUserID(r.Context()); userID != "" {
		return userID
	}

	// Fall back to X-User-ID header for anonymous cart functionality
	// Client should generate a UUID for anonymous users
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID
	}

	return ""
}

// requireUserID returns the user ID or sends an unauthorized response
func requireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID := getUserID(r)
	if userID == "" {
		respondJSONError(w, "Authentication required", http.StatusUnauthorized)
		return "", false
	}
	return userID, true
}

// isAdmin checks if the current user has admin role
func isAdmin(r *http.Request) bool {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		return false
	}
	return claims.Role == "admin"
}
