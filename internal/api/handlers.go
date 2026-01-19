package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/example/ec-event-driven/internal/api/middleware"
	"github.com/example/ec-event-driven/internal/command"
	"github.com/example/ec-event-driven/internal/query"
)

type Handlers struct {
	cmdHandler   *command.Handler
	queryHandler *query.Handler
}

func NewHandlers(cmdHandler *command.Handler, queryHandler *query.Handler) *Handlers {
	return &Handlers{
		cmdHandler:   cmdHandler,
		queryHandler: queryHandler,
	}
}

// Product Handlers

func (h *Handlers) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var cmd command.CreateProduct
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	product, err := h.cmdHandler.CreateProduct(r.Context(), cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, product)
}

func (h *Handlers) GetProducts(w http.ResponseWriter, r *http.Request) {
	products := h.queryHandler.ListProducts()
	respondJSON(w, http.StatusOK, products)
}

func (h *Handlers) GetProduct(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r.URL.Path, "/products/")
	product, ok := h.queryHandler.GetProduct(id)
	if !ok {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, product)
}

func (h *Handlers) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r.URL.Path, "/products/")

	var cmd command.UpdateProduct
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cmd.ProductID = id

	if err := h.cmdHandler.UpdateProduct(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Product updated"})
}

func (h *Handlers) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r.URL.Path, "/products/")

	cmd := command.DeleteProduct{ProductID: id}
	if err := h.cmdHandler.DeleteProduct(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Product deleted"})
}

// Cart Handlers

func (h *Handlers) AddToCart(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	var req struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd := command.AddToCart{
		UserID:    userID,
		ProductID: req.ProductID,
		Quantity:  req.Quantity,
	}
	if err := h.cmdHandler.AddToCart(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	productID := extractPathParam(r.URL.Path, "/cart/items/")
	cmd := command.RemoveFromCart{
		UserID:    userID,
		ProductID: productID,
	}
	if err := h.cmdHandler.RemoveFromCart(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) GetCart(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	cart, _ := h.queryHandler.GetCart(userID)
	respondJSON(w, http.StatusOK, cart)
}

// Order Handlers

func (h *Handlers) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	cmd := command.PlaceOrder{UserID: userID}
	order, err := h.cmdHandler.PlaceOrder(r.Context(), cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusCreated, order)
}

func (h *Handlers) GetOrders(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	orders := h.queryHandler.ListOrdersByUser(userID)
	respondJSON(w, http.StatusOK, orders)
}

func (h *Handlers) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r.URL.Path, "/orders/")
	// Remove /cancel suffix if present
	id = strings.TrimSuffix(id, "/cancel")

	order, ok := h.queryHandler.GetOrder(id)
	if !ok {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	// Authorization check: user can only access their own orders (admins can access all)
	userID := getUserID(r)
	if order.UserID != userID && !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	respondJSON(w, http.StatusOK, order)
}

func (h *Handlers) CancelOrder(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/orders/")
	id := strings.TrimSuffix(path, "/cancel")

	// Authorization check: user can only cancel their own orders (admins can cancel all)
	order, ok := h.queryHandler.GetOrder(id)
	if !ok {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	userID := getUserID(r)
	if order.UserID != userID && !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	cmd := command.CancelOrder{
		OrderID: id,
		Reason:  req.Reason,
	}
	if err := h.cmdHandler.CancelOrder(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Admin Handlers

func (h *Handlers) GetAllOrders(w http.ResponseWriter, r *http.Request) {
	orders := h.queryHandler.ListAllOrders()
	respondJSON(w, http.StatusOK, orders)
}

// Helper functions

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func extractPathParam(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}

// getUserID extracts user ID from JWT context or falls back to X-User-ID header
func getUserID(r *http.Request) string {
	// First try to get from JWT context
	if userID := middleware.GetUserID(r.Context()); userID != "" {
		return userID
	}

	// Fall back to X-User-ID header for backward compatibility
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID
	}

	return "default-user"
}

// isAdmin checks if the current user has admin role
func isAdmin(r *http.Request) bool {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		return false
	}
	return claims.Role == "admin"
}
