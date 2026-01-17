package query

// Re-export read models from readmodel package for backward compatibility
import "github.com/example/ec-event-driven/internal/readmodel"

type ProductReadModel = readmodel.ProductReadModel
type CartItemReadModel = readmodel.CartItemReadModel
type CartReadModel = readmodel.CartReadModel
type OrderItemReadModel = readmodel.OrderItemReadModel
type OrderReadModel = readmodel.OrderReadModel
type InventoryReadModel = readmodel.InventoryReadModel
