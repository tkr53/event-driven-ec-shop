package store

import (
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/example/ec-event-driven/internal/readmodel"
)

// PostgresReadStore implements ReadStoreInterface using PostgreSQL
type PostgresReadStore struct {
	db *sql.DB
	mu sync.RWMutex // for thread-safe operations
}

// NewPostgresReadStore creates a new PostgreSQL-based read store
func NewPostgresReadStore(db *sql.DB) *PostgresReadStore {
	return &PostgresReadStore{db: db}
}

// Set stores a read model
func (rs *PostgresReadStore) Set(collection, id string, data any) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	switch collection {
	case "products":
		rs.setProduct(id, data.(*readmodel.ProductReadModel))
	case "carts":
		rs.setCart(id, data.(*readmodel.CartReadModel))
	case "orders":
		rs.setOrder(id, data.(*readmodel.OrderReadModel))
	case "inventory":
		rs.setInventory(id, data.(*readmodel.InventoryReadModel))
	}
}

// Get retrieves a read model by id
func (rs *PostgresReadStore) Get(collection, id string) (any, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	switch collection {
	case "products":
		return rs.getProduct(id)
	case "carts":
		return rs.getCart(id)
	case "orders":
		return rs.getOrder(id)
	case "inventory":
		return rs.getInventory(id)
	}
	return nil, false
}

// GetAll retrieves all items in a collection
func (rs *PostgresReadStore) GetAll(collection string) []any {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	switch collection {
	case "products":
		return rs.getAllProducts()
	case "carts":
		return rs.getAllCarts()
	case "orders":
		return rs.getAllOrders()
	case "inventory":
		return rs.getAllInventory()
	}
	return nil
}

// Delete removes a read model
func (rs *PostgresReadStore) Delete(collection, id string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	var tableName string
	switch collection {
	case "products":
		tableName = "read_products"
	case "carts":
		tableName = "read_carts"
	case "orders":
		tableName = "read_orders"
	case "inventory":
		tableName = "read_inventory"
	default:
		return
	}

	_, err := rs.db.Exec("DELETE FROM "+tableName+" WHERE id = $1", id)
	if err != nil {
		log.Printf("[PostgresReadStore] Error deleting from %s: %v", collection, err)
	}
}

// Update modifies a read model using an update function
func (rs *PostgresReadStore) Update(collection, id string, updateFn func(current any) any) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Get current value
	var current any
	var found bool

	switch collection {
	case "products":
		current, found = rs.getProductUnsafe(id)
	case "carts":
		current, found = rs.getCartUnsafe(id)
	case "orders":
		current, found = rs.getOrderUnsafe(id)
	case "inventory":
		current, found = rs.getInventoryUnsafe(id)
	}

	if !found {
		return false
	}

	// Apply update function
	updated := updateFn(current)

	// Save updated value
	switch collection {
	case "products":
		rs.setProductUnsafe(id, updated.(*readmodel.ProductReadModel))
	case "carts":
		rs.setCartUnsafe(id, updated.(*readmodel.CartReadModel))
	case "orders":
		rs.setOrderUnsafe(id, updated.(*readmodel.OrderReadModel))
	case "inventory":
		rs.setInventoryUnsafe(id, updated.(*readmodel.InventoryReadModel))
	}

	return true
}

// Product operations
func (rs *PostgresReadStore) setProduct(id string, p *readmodel.ProductReadModel) {
	rs.setProductUnsafe(id, p)
}

func (rs *PostgresReadStore) setProductUnsafe(id string, p *readmodel.ProductReadModel) {
	_, err := rs.db.Exec(`
		INSERT INTO read_products (id, name, description, price, stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			price = EXCLUDED.price,
			stock = EXCLUDED.stock,
			updated_at = EXCLUDED.updated_at
	`, p.ID, p.Name, p.Description, p.Price, p.Stock, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		log.Printf("[PostgresReadStore] Error setting product: %v", err)
	}
}

func (rs *PostgresReadStore) getProduct(id string) (any, bool) {
	return rs.getProductUnsafe(id)
}

func (rs *PostgresReadStore) getProductUnsafe(id string) (*readmodel.ProductReadModel, bool) {
	var p readmodel.ProductReadModel
	err := rs.db.QueryRow(`
		SELECT id, name, description, price, stock, created_at, updated_at
		FROM read_products WHERE id = $1
	`, id).Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("[PostgresReadStore] Error getting product: %v", err)
		}
		return nil, false
	}
	return &p, true
}

func (rs *PostgresReadStore) getAllProducts() []any {
	rows, err := rs.db.Query(`
		SELECT id, name, description, price, stock, created_at, updated_at
		FROM read_products ORDER BY created_at DESC
	`)
	if err != nil {
		log.Printf("[PostgresReadStore] Error getting all products: %v", err)
		return nil
	}
	defer rows.Close()

	var products []any
	for rows.Next() {
		var p readmodel.ProductReadModel
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.CreatedAt, &p.UpdatedAt); err != nil {
			log.Printf("[PostgresReadStore] Error scanning product: %v", err)
			continue
		}
		products = append(products, &p)
	}
	return products
}

// Cart operations
func (rs *PostgresReadStore) setCart(id string, c *readmodel.CartReadModel) {
	rs.setCartUnsafe(id, c)
}

func (rs *PostgresReadStore) setCartUnsafe(id string, c *readmodel.CartReadModel) {
	itemsJSON, _ := json.Marshal(c.Items)
	_, err := rs.db.Exec(`
		INSERT INTO read_carts (id, user_id, items, total, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			items = EXCLUDED.items,
			total = EXCLUDED.total,
			updated_at = EXCLUDED.updated_at
	`, c.ID, c.UserID, itemsJSON, c.Total, time.Now())
	if err != nil {
		log.Printf("[PostgresReadStore] Error setting cart: %v", err)
	}
}

func (rs *PostgresReadStore) getCart(id string) (any, bool) {
	return rs.getCartUnsafe(id)
}

func (rs *PostgresReadStore) getCartUnsafe(id string) (*readmodel.CartReadModel, bool) {
	var c readmodel.CartReadModel
	var itemsJSON []byte
	err := rs.db.QueryRow(`
		SELECT id, user_id, items, total FROM read_carts WHERE id = $1
	`, id).Scan(&c.ID, &c.UserID, &itemsJSON, &c.Total)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("[PostgresReadStore] Error getting cart: %v", err)
		}
		return nil, false
	}
	json.Unmarshal(itemsJSON, &c.Items)
	return &c, true
}

func (rs *PostgresReadStore) getAllCarts() []any {
	rows, err := rs.db.Query(`SELECT id, user_id, items, total FROM read_carts`)
	if err != nil {
		log.Printf("[PostgresReadStore] Error getting all carts: %v", err)
		return nil
	}
	defer rows.Close()

	var carts []any
	for rows.Next() {
		var c readmodel.CartReadModel
		var itemsJSON []byte
		if err := rows.Scan(&c.ID, &c.UserID, &itemsJSON, &c.Total); err != nil {
			log.Printf("[PostgresReadStore] Error scanning cart: %v", err)
			continue
		}
		json.Unmarshal(itemsJSON, &c.Items)
		carts = append(carts, &c)
	}
	return carts
}

// Order operations
func (rs *PostgresReadStore) setOrder(id string, o *readmodel.OrderReadModel) {
	rs.setOrderUnsafe(id, o)
}

func (rs *PostgresReadStore) setOrderUnsafe(id string, o *readmodel.OrderReadModel) {
	itemsJSON, _ := json.Marshal(o.Items)
	_, err := rs.db.Exec(`
		INSERT INTO read_orders (id, user_id, items, total, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			items = EXCLUDED.items,
			total = EXCLUDED.total,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`, o.ID, o.UserID, itemsJSON, o.Total, o.Status, o.CreatedAt, o.UpdatedAt)
	if err != nil {
		log.Printf("[PostgresReadStore] Error setting order: %v", err)
	}
}

func (rs *PostgresReadStore) getOrder(id string) (any, bool) {
	return rs.getOrderUnsafe(id)
}

func (rs *PostgresReadStore) getOrderUnsafe(id string) (*readmodel.OrderReadModel, bool) {
	var o readmodel.OrderReadModel
	var itemsJSON []byte
	err := rs.db.QueryRow(`
		SELECT id, user_id, items, total, status, created_at, updated_at
		FROM read_orders WHERE id = $1
	`, id).Scan(&o.ID, &o.UserID, &itemsJSON, &o.Total, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("[PostgresReadStore] Error getting order: %v", err)
		}
		return nil, false
	}
	json.Unmarshal(itemsJSON, &o.Items)
	return &o, true
}

func (rs *PostgresReadStore) getAllOrders() []any {
	rows, err := rs.db.Query(`
		SELECT id, user_id, items, total, status, created_at, updated_at
		FROM read_orders ORDER BY created_at DESC
	`)
	if err != nil {
		log.Printf("[PostgresReadStore] Error getting all orders: %v", err)
		return nil
	}
	defer rows.Close()

	var orders []any
	for rows.Next() {
		var o readmodel.OrderReadModel
		var itemsJSON []byte
		if err := rows.Scan(&o.ID, &o.UserID, &itemsJSON, &o.Total, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
			log.Printf("[PostgresReadStore] Error scanning order: %v", err)
			continue
		}
		json.Unmarshal(itemsJSON, &o.Items)
		orders = append(orders, &o)
	}
	return orders
}

// Inventory operations
func (rs *PostgresReadStore) setInventory(id string, inv *readmodel.InventoryReadModel) {
	rs.setInventoryUnsafe(id, inv)
}

func (rs *PostgresReadStore) setInventoryUnsafe(id string, inv *readmodel.InventoryReadModel) {
	_, err := rs.db.Exec(`
		INSERT INTO read_inventory (product_id, total_stock, reserved_stock, available_stock, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (product_id) DO UPDATE SET
			total_stock = EXCLUDED.total_stock,
			reserved_stock = EXCLUDED.reserved_stock,
			available_stock = EXCLUDED.available_stock,
			updated_at = EXCLUDED.updated_at
	`, inv.ProductID, inv.TotalStock, inv.ReservedStock, inv.AvailableStock, time.Now())
	if err != nil {
		log.Printf("[PostgresReadStore] Error setting inventory: %v", err)
	}
}

func (rs *PostgresReadStore) getInventory(id string) (any, bool) {
	return rs.getInventoryUnsafe(id)
}

func (rs *PostgresReadStore) getInventoryUnsafe(id string) (*readmodel.InventoryReadModel, bool) {
	var inv readmodel.InventoryReadModel
	err := rs.db.QueryRow(`
		SELECT product_id, total_stock, reserved_stock, available_stock
		FROM read_inventory WHERE product_id = $1
	`, id).Scan(&inv.ProductID, &inv.TotalStock, &inv.ReservedStock, &inv.AvailableStock)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("[PostgresReadStore] Error getting inventory: %v", err)
		}
		return nil, false
	}
	return &inv, true
}

func (rs *PostgresReadStore) getAllInventory() []any {
	rows, err := rs.db.Query(`
		SELECT product_id, total_stock, reserved_stock, available_stock FROM read_inventory
	`)
	if err != nil {
		log.Printf("[PostgresReadStore] Error getting all inventory: %v", err)
		return nil
	}
	defer rows.Close()

	var inventory []any
	for rows.Next() {
		var inv readmodel.InventoryReadModel
		if err := rows.Scan(&inv.ProductID, &inv.TotalStock, &inv.ReservedStock, &inv.AvailableStock); err != nil {
			log.Printf("[PostgresReadStore] Error scanning inventory: %v", err)
			continue
		}
		inventory = append(inventory, &inv)
	}
	return inventory
}
