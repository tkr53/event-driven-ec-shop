package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/example/ec-event-driven/internal/readmodel"
)

// PostgresReadStore implements ReadStoreInterface using PostgreSQL
type PostgresReadStore struct {
	db *sql.DB
}

// NewPostgresReadStore creates a new PostgreSQL-based read store
func NewPostgresReadStore(db *sql.DB) *PostgresReadStore {
	return &PostgresReadStore{db: db}
}

// Set stores a read model
func (rs *PostgresReadStore) Set(collection, id string, data any) error {
	switch collection {
	case "products":
		return rs.setProduct(id, data.(*readmodel.ProductReadModel))
	case "carts":
		return rs.setCart(id, data.(*readmodel.CartReadModel))
	case "orders":
		return rs.setOrder(id, data.(*readmodel.OrderReadModel))
	case "inventory":
		return rs.setInventory(id, data.(*readmodel.InventoryReadModel))
	case "users":
		return rs.setUser(id, data.(*readmodel.UserReadModel))
	case "sessions":
		return rs.setSession(id, data.(*readmodel.SessionReadModel))
	case "categories":
		return rs.setCategory(id, data.(*readmodel.CategoryReadModel))
	}
	return fmt.Errorf("unknown collection: %s", collection)
}

// Get retrieves a read model by id
func (rs *PostgresReadStore) Get(collection, id string) (any, bool, error) {
	switch collection {
	case "products":
		return rs.getProduct(id)
	case "carts":
		return rs.getCart(id)
	case "orders":
		return rs.getOrder(id)
	case "inventory":
		return rs.getInventory(id)
	case "users":
		return rs.getUser(id)
	case "sessions":
		return rs.getSession(id)
	case "categories":
		return rs.getCategory(id)
	}
	return nil, false, fmt.Errorf("unknown collection: %s", collection)
}

// GetAll retrieves all items in a collection
func (rs *PostgresReadStore) GetAll(collection string) ([]any, error) {
	switch collection {
	case "products":
		return rs.getAllProducts()
	case "carts":
		return rs.getAllCarts()
	case "orders":
		return rs.getAllOrders()
	case "inventory":
		return rs.getAllInventory()
	case "users":
		return rs.getAllUsers()
	case "sessions":
		return rs.getAllSessions()
	case "categories":
		return rs.getAllCategories()
	}
	return nil, fmt.Errorf("unknown collection: %s", collection)
}

// Delete removes a read model
func (rs *PostgresReadStore) Delete(collection, id string) error {
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
	case "users":
		tableName = "read_users"
	case "sessions":
		tableName = "user_sessions"
	case "categories":
		tableName = "read_categories"
	default:
		return fmt.Errorf("unknown collection: %s", collection)
	}

	_, err := rs.db.Exec("DELETE FROM "+tableName+" WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete from %s: %w", collection, err)
	}
	return nil
}

// Update modifies a read model using an update function
func (rs *PostgresReadStore) Update(collection, id string, updateFn func(current any) any) (bool, error) {
	// Get current value
	var current any
	var found bool
	var err error

	switch collection {
	case "products":
		current, found, err = rs.getProduct(id)
	case "carts":
		current, found, err = rs.getCart(id)
	case "orders":
		current, found, err = rs.getOrder(id)
	case "inventory":
		current, found, err = rs.getInventory(id)
	case "users":
		current, found, err = rs.getUser(id)
	case "sessions":
		current, found, err = rs.getSession(id)
	case "categories":
		current, found, err = rs.getCategory(id)
	default:
		return false, fmt.Errorf("unknown collection: %s", collection)
	}

	if err != nil {
		return false, fmt.Errorf("get %s: %w", collection, err)
	}

	if !found {
		return false, nil
	}

	// Apply update function
	updated := updateFn(current)

	// Save updated value
	switch collection {
	case "products":
		err = rs.setProduct(id, updated.(*readmodel.ProductReadModel))
	case "carts":
		err = rs.setCart(id, updated.(*readmodel.CartReadModel))
	case "orders":
		err = rs.setOrder(id, updated.(*readmodel.OrderReadModel))
	case "inventory":
		err = rs.setInventory(id, updated.(*readmodel.InventoryReadModel))
	case "users":
		err = rs.setUser(id, updated.(*readmodel.UserReadModel))
	case "sessions":
		err = rs.setSession(id, updated.(*readmodel.SessionReadModel))
	case "categories":
		err = rs.setCategory(id, updated.(*readmodel.CategoryReadModel))
	}

	if err != nil {
		return false, fmt.Errorf("set %s: %w", collection, err)
	}

	return true, nil
}

// Product operations
func (rs *PostgresReadStore) setProduct(id string, p *readmodel.ProductReadModel) error {
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
	return err
}

func (rs *PostgresReadStore) getProduct(id string) (*readmodel.ProductReadModel, bool, error) {
	var p readmodel.ProductReadModel
	err := rs.db.QueryRow(`
		SELECT id, name, description, price, stock, created_at, updated_at
		FROM read_products WHERE id = $1
	`, id).Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &p, true, nil
}

func (rs *PostgresReadStore) getAllProducts() ([]any, error) {
	rows, err := rs.db.Query(`
		SELECT id, name, description, price, stock, created_at, updated_at
		FROM read_products ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var products []any
	for rows.Next() {
		var p readmodel.ProductReadModel
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		products = append(products, &p)
	}
	return products, rows.Err()
}

// Cart operations
func (rs *PostgresReadStore) setCart(id string, c *readmodel.CartReadModel) error {
	itemsJSON, err := json.Marshal(c.Items)
	if err != nil {
		return err
	}
	_, err = rs.db.Exec(`
		INSERT INTO read_carts (id, user_id, items, total, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			items = EXCLUDED.items,
			total = EXCLUDED.total,
			updated_at = EXCLUDED.updated_at
	`, c.ID, c.UserID, itemsJSON, c.Total, time.Now())
	return err
}

func (rs *PostgresReadStore) getCart(id string) (*readmodel.CartReadModel, bool, error) {
	var c readmodel.CartReadModel
	var itemsJSON []byte
	err := rs.db.QueryRow(`
		SELECT id, user_id, items, total FROM read_carts WHERE id = $1
	`, id).Scan(&c.ID, &c.UserID, &itemsJSON, &c.Total)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	if err := json.Unmarshal(itemsJSON, &c.Items); err != nil {
		return nil, false, err
	}
	return &c, true, nil
}

func (rs *PostgresReadStore) getAllCarts() ([]any, error) {
	rows, err := rs.db.Query(`SELECT id, user_id, items, total FROM read_carts`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var carts []any
	for rows.Next() {
		var c readmodel.CartReadModel
		var itemsJSON []byte
		if err := rows.Scan(&c.ID, &c.UserID, &itemsJSON, &c.Total); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(itemsJSON, &c.Items); err != nil {
			return nil, err
		}
		carts = append(carts, &c)
	}
	return carts, rows.Err()
}

// Order operations
func (rs *PostgresReadStore) setOrder(id string, o *readmodel.OrderReadModel) error {
	itemsJSON, err := json.Marshal(o.Items)
	if err != nil {
		return err
	}
	_, err = rs.db.Exec(`
		INSERT INTO read_orders (id, user_id, items, total, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			items = EXCLUDED.items,
			total = EXCLUDED.total,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`, o.ID, o.UserID, itemsJSON, o.Total, o.Status, o.CreatedAt, o.UpdatedAt)
	return err
}

func (rs *PostgresReadStore) getOrder(id string) (*readmodel.OrderReadModel, bool, error) {
	var o readmodel.OrderReadModel
	var itemsJSON []byte
	err := rs.db.QueryRow(`
		SELECT id, user_id, items, total, status, created_at, updated_at
		FROM read_orders WHERE id = $1
	`, id).Scan(&o.ID, &o.UserID, &itemsJSON, &o.Total, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	if err := json.Unmarshal(itemsJSON, &o.Items); err != nil {
		return nil, false, err
	}
	return &o, true, nil
}

func (rs *PostgresReadStore) getAllOrders() ([]any, error) {
	rows, err := rs.db.Query(`
		SELECT id, user_id, items, total, status, created_at, updated_at
		FROM read_orders ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var orders []any
	for rows.Next() {
		var o readmodel.OrderReadModel
		var itemsJSON []byte
		if err := rows.Scan(&o.ID, &o.UserID, &itemsJSON, &o.Total, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(itemsJSON, &o.Items); err != nil {
			return nil, err
		}
		orders = append(orders, &o)
	}
	return orders, rows.Err()
}

// Inventory operations
func (rs *PostgresReadStore) setInventory(id string, inv *readmodel.InventoryReadModel) error {
	_, err := rs.db.Exec(`
		INSERT INTO read_inventory (product_id, total_stock, reserved_stock, available_stock, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (product_id) DO UPDATE SET
			total_stock = EXCLUDED.total_stock,
			reserved_stock = EXCLUDED.reserved_stock,
			available_stock = EXCLUDED.available_stock,
			updated_at = EXCLUDED.updated_at
	`, inv.ProductID, inv.TotalStock, inv.ReservedStock, inv.AvailableStock, time.Now())
	return err
}

func (rs *PostgresReadStore) getInventory(id string) (*readmodel.InventoryReadModel, bool, error) {
	var inv readmodel.InventoryReadModel
	err := rs.db.QueryRow(`
		SELECT product_id, total_stock, reserved_stock, available_stock
		FROM read_inventory WHERE product_id = $1
	`, id).Scan(&inv.ProductID, &inv.TotalStock, &inv.ReservedStock, &inv.AvailableStock)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &inv, true, nil
}

func (rs *PostgresReadStore) getAllInventory() ([]any, error) {
	rows, err := rs.db.Query(`
		SELECT product_id, total_stock, reserved_stock, available_stock FROM read_inventory
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var inventory []any
	for rows.Next() {
		var inv readmodel.InventoryReadModel
		if err := rows.Scan(&inv.ProductID, &inv.TotalStock, &inv.ReservedStock, &inv.AvailableStock); err != nil {
			return nil, err
		}
		inventory = append(inventory, &inv)
	}
	return inventory, rows.Err()
}

// User operations
func (rs *PostgresReadStore) setUser(id string, u *readmodel.UserReadModel) error {
	_, err := rs.db.Exec(`
		INSERT INTO read_users (id, email, password_hash, name, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			email = EXCLUDED.email,
			password_hash = EXCLUDED.password_hash,
			name = EXCLUDED.name,
			role = EXCLUDED.role,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at
	`, u.ID, u.Email, u.PasswordHash, u.Name, u.Role, u.IsActive, u.CreatedAt, u.UpdatedAt)
	return err
}

func (rs *PostgresReadStore) getUser(id string) (*readmodel.UserReadModel, bool, error) {
	var u readmodel.UserReadModel
	err := rs.db.QueryRow(`
		SELECT id, email, password_hash, name, role, is_active, created_at, updated_at
		FROM read_users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &u, true, nil
}

// GetUserByEmail retrieves a user by email
func (rs *PostgresReadStore) GetUserByEmail(email string) (*readmodel.UserReadModel, bool) {
	var u readmodel.UserReadModel
	err := rs.db.QueryRow(`
		SELECT id, email, password_hash, name, role, is_active, created_at, updated_at
		FROM read_users WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, false
	}
	return &u, true
}

func (rs *PostgresReadStore) getAllUsers() ([]any, error) {
	rows, err := rs.db.Query(`
		SELECT id, email, password_hash, name, role, is_active, created_at, updated_at
		FROM read_users ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var users []any
	for rows.Next() {
		var u readmodel.UserReadModel
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, rows.Err()
}

// Session operations
func (rs *PostgresReadStore) setSession(id string, s *readmodel.SessionReadModel) error {
	_, err := rs.db.Exec(`
		INSERT INTO user_sessions (id, user_id, refresh_token_hash, expires_at, created_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			refresh_token_hash = EXCLUDED.refresh_token_hash,
			expires_at = EXCLUDED.expires_at
	`, s.ID, s.UserID, s.RefreshTokenHash, s.ExpiresAt, s.CreatedAt, s.IPAddress, s.UserAgent)
	return err
}

func (rs *PostgresReadStore) getSession(id string) (*readmodel.SessionReadModel, bool, error) {
	var s readmodel.SessionReadModel
	err := rs.db.QueryRow(`
		SELECT id, user_id, refresh_token_hash, expires_at, created_at, ip_address, user_agent
		FROM user_sessions WHERE id = $1
	`, id).Scan(&s.ID, &s.UserID, &s.RefreshTokenHash, &s.ExpiresAt, &s.CreatedAt, &s.IPAddress, &s.UserAgent)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &s, true, nil
}

// GetSessionByUserID retrieves a session by user ID
func (rs *PostgresReadStore) GetSessionByUserID(userID string) (*readmodel.SessionReadModel, bool) {
	var s readmodel.SessionReadModel
	err := rs.db.QueryRow(`
		SELECT id, user_id, refresh_token_hash, expires_at, created_at, ip_address, user_agent
		FROM user_sessions WHERE user_id = $1 AND expires_at > NOW()
		ORDER BY created_at DESC LIMIT 1
	`, userID).Scan(&s.ID, &s.UserID, &s.RefreshTokenHash, &s.ExpiresAt, &s.CreatedAt, &s.IPAddress, &s.UserAgent)
	if err != nil {
		return nil, false
	}
	return &s, true
}

// DeleteSessionsByUserID deletes all sessions for a user
func (rs *PostgresReadStore) DeleteSessionsByUserID(userID string) error {
	_, err := rs.db.Exec(`DELETE FROM user_sessions WHERE user_id = $1`, userID)
	return err
}

// DeleteExpiredSessions removes expired sessions
func (rs *PostgresReadStore) DeleteExpiredSessions() error {
	_, err := rs.db.Exec(`DELETE FROM user_sessions WHERE expires_at < NOW()`)
	return err
}

func (rs *PostgresReadStore) getAllSessions() ([]any, error) {
	rows, err := rs.db.Query(`
		SELECT id, user_id, refresh_token_hash, expires_at, created_at, ip_address, user_agent
		FROM user_sessions ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sessions []any
	for rows.Next() {
		var s readmodel.SessionReadModel
		if err := rows.Scan(&s.ID, &s.UserID, &s.RefreshTokenHash, &s.ExpiresAt, &s.CreatedAt, &s.IPAddress, &s.UserAgent); err != nil {
			return nil, err
		}
		sessions = append(sessions, &s)
	}
	return sessions, rows.Err()
}

// Category operations
func (rs *PostgresReadStore) setCategory(id string, c *readmodel.CategoryReadModel) error {
	_, err := rs.db.Exec(`
		INSERT INTO read_categories (id, name, slug, description, parent_id, sort_order, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			slug = EXCLUDED.slug,
			description = EXCLUDED.description,
			parent_id = EXCLUDED.parent_id,
			sort_order = EXCLUDED.sort_order,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at
	`, c.ID, c.Name, c.Slug, c.Description, nullString(c.ParentID), c.SortOrder, c.IsActive, c.CreatedAt, c.UpdatedAt)
	return err
}

func (rs *PostgresReadStore) getCategory(id string) (*readmodel.CategoryReadModel, bool, error) {
	var c readmodel.CategoryReadModel
	var parentID sql.NullString
	err := rs.db.QueryRow(`
		SELECT id, name, slug, description, parent_id, sort_order, is_active, created_at, updated_at
		FROM read_categories WHERE id = $1
	`, id).Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &parentID, &c.SortOrder, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	c.ParentID = parentID.String
	return &c, true, nil
}

// GetCategoryBySlug retrieves a category by its slug
func (rs *PostgresReadStore) GetCategoryBySlug(slug string) (*readmodel.CategoryReadModel, bool) {
	var c readmodel.CategoryReadModel
	var parentID sql.NullString
	err := rs.db.QueryRow(`
		SELECT id, name, slug, description, parent_id, sort_order, is_active, created_at, updated_at
		FROM read_categories WHERE slug = $1 AND is_active = true
	`, slug).Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &parentID, &c.SortOrder, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, false
	}
	c.ParentID = parentID.String
	return &c, true
}

func (rs *PostgresReadStore) getAllCategories() ([]any, error) {
	rows, err := rs.db.Query(`
		SELECT id, name, slug, description, parent_id, sort_order, is_active, created_at, updated_at
		FROM read_categories WHERE is_active = true ORDER BY sort_order, name
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var categories []any
	for rows.Next() {
		var c readmodel.CategoryReadModel
		var parentID sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &parentID, &c.SortOrder, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.ParentID = parentID.String
		categories = append(categories, &c)
	}
	return categories, rows.Err()
}

// Product-Category relationship operations

// AddProductCategory adds a category to a product
func (rs *PostgresReadStore) AddProductCategory(productID, categoryID string) {
	_, err := rs.db.Exec(`
		INSERT INTO product_categories (product_id, category_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, productID, categoryID)
	if err != nil {
		log.Printf("[PostgresReadStore] Error adding product category: %v", err)
	}
}

// RemoveProductCategory removes a category from a product
func (rs *PostgresReadStore) RemoveProductCategory(productID, categoryID string) {
	_, err := rs.db.Exec(`DELETE FROM product_categories WHERE product_id = $1 AND category_id = $2`, productID, categoryID)
	if err != nil {
		log.Printf("[PostgresReadStore] Error removing product category: %v", err)
	}
}

// GetProductCategories returns all category IDs for a product
func (rs *PostgresReadStore) GetProductCategories(productID string) []string {
	rows, err := rs.db.Query(`SELECT category_id FROM product_categories WHERE product_id = $1`, productID)
	if err != nil {
		log.Printf("[PostgresReadStore] Error getting product categories: %v", err)
		return nil
	}
	defer func() { _ = rows.Close() }()

	var categoryIDs []string
	for rows.Next() {
		var categoryID string
		if err := rows.Scan(&categoryID); err != nil {
			continue
		}
		categoryIDs = append(categoryIDs, categoryID)
	}
	return categoryIDs
}

// Product search and filtering

// SearchProductsParams contains parameters for product search
type SearchProductsParams struct {
	Query      string
	CategoryID string
	MinPrice   int
	MaxPrice   int
	Limit      int
	Offset     int
}

// SearchProducts searches for products with various filters
func (rs *PostgresReadStore) SearchProducts(params SearchProductsParams) []*readmodel.ProductReadModel {
	query := `
		SELECT DISTINCT p.id, p.name, p.description, p.price, p.stock, p.image_url, p.created_at, p.updated_at
		FROM read_products p
	`
	var args []any
	argNum := 1
	var conditions []string

	// Join with product_categories if filtering by category
	if params.CategoryID != "" {
		query += ` INNER JOIN product_categories pc ON p.id = pc.product_id`
		conditions = append(conditions, "pc.category_id = $"+fmt.Sprintf("%d", argNum))
		args = append(args, params.CategoryID)
		argNum++
	}

	// Full-text search
	if params.Query != "" {
		conditions = append(conditions, "p.search_vector @@ plainto_tsquery('english', $"+fmt.Sprintf("%d", argNum)+")")
		args = append(args, params.Query)
		argNum++
	}

	// Price range filters
	if params.MinPrice > 0 {
		conditions = append(conditions, "p.price >= $"+fmt.Sprintf("%d", argNum))
		args = append(args, params.MinPrice)
		argNum++
	}
	if params.MaxPrice > 0 {
		conditions = append(conditions, "p.price <= $"+fmt.Sprintf("%d", argNum))
		args = append(args, params.MaxPrice)
		argNum++
	}

	// Stock filter (only show in-stock products)
	conditions = append(conditions, "p.stock > 0")

	if len(conditions) > 0 {
		query += " WHERE "
		for i, cond := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += cond
		}
	}

	// Order by relevance if searching, otherwise by created_at
	if params.Query != "" {
		query += " ORDER BY ts_rank(p.search_vector, plainto_tsquery('english', $1)) DESC"
	} else {
		query += " ORDER BY p.created_at DESC"
	}

	// Pagination
	if params.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, params.Limit)
		argNum++
	}
	if params.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, params.Offset)
		argNum++
	}
	_ = argNum // suppress unused variable warning

	rows, err := rs.db.Query(query, args...)
	if err != nil {
		log.Printf("[PostgresReadStore] Error searching products: %v", err)
		return nil
	}
	defer func() { _ = rows.Close() }()

	var products []*readmodel.ProductReadModel
	for rows.Next() {
		var p readmodel.ProductReadModel
		var imageURL sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &imageURL, &p.CreatedAt, &p.UpdatedAt); err != nil {
			log.Printf("[PostgresReadStore] Error scanning product: %v", err)
			continue
		}
		p.ImageURL = imageURL.String
		p.CategoryIDs = rs.getProductCategoriesUnsafe(p.ID)
		products = append(products, &p)
	}
	return products
}

// GetProductsByCategory returns all products in a category
func (rs *PostgresReadStore) GetProductsByCategory(categoryID string) []*readmodel.ProductReadModel {
	return rs.SearchProducts(SearchProductsParams{CategoryID: categoryID})
}

func (rs *PostgresReadStore) getProductCategoriesUnsafe(productID string) []string {
	rows, err := rs.db.Query(`SELECT category_id FROM product_categories WHERE product_id = $1`, productID)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var categoryIDs []string
	for rows.Next() {
		var categoryID string
		if err := rows.Scan(&categoryID); err != nil {
			continue
		}
		categoryIDs = append(categoryIDs, categoryID)
	}
	return categoryIDs
}

// Helper function
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
