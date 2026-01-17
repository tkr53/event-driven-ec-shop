package api

import (
	"encoding/json"
	"net/http"
	"strings"

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

// Cart Handlers

func (h *Handlers) AddToCart(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default-user"
	}

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
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default-user"
	}

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
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default-user"
	}

	cart, _ := h.queryHandler.GetCart(userID)
	respondJSON(w, http.StatusOK, cart)
}

// Order Handlers

func (h *Handlers) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default-user"
	}

	cmd := command.PlaceOrder{UserID: userID}
	order, err := h.cmdHandler.PlaceOrder(r.Context(), cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusCreated, order)
}

func (h *Handlers) GetOrders(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default-user"
	}

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
	respondJSON(w, http.StatusOK, order)
}

func (h *Handlers) CancelOrder(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/orders/")
	id := strings.TrimSuffix(path, "/cancel")

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

// Helper functions

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func extractPathParam(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}
