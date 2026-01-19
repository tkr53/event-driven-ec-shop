package api

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/example/ec-event-driven/internal/api/middleware"
	"github.com/example/ec-event-driven/internal/auth"
)

// allowedOrigins is a map of allowed CORS origins for O(1) lookup
var allowedOrigins map[string]bool

func init() {
	// Initialize allowed origins from environment variable or use defaults
	originsEnv := os.Getenv("CORS_ALLOWED_ORIGINS")
	if originsEnv == "" {
		// Default allowed origins for development
		originsEnv = "http://localhost:3000,http://localhost:8080"
	}

	allowedOrigins = make(map[string]bool)
	for _, origin := range strings.Split(originsEnv, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowedOrigins[origin] = true
		}
	}
	log.Printf("[API] CORS allowed origins: %v", allowedOrigins)
}

// RouterConfig holds the configuration for the router
type RouterConfig struct {
	Handlers         *Handlers
	AuthHandlers     *AuthHandlers
	CategoryHandlers *CategoryHandlers
	JWTService       *auth.JWTService
}

func NewRouter(config RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// Authentication routes (no auth required)
	mux.HandleFunc("/api/auth/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			config.AuthHandlers.Register(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			config.AuthHandlers.Login(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			config.AuthHandlers.Refresh(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Auth routes requiring authentication
	mux.Handle("/api/auth/logout", middleware.AuthMiddleware(config.JWTService)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				config.AuthHandlers.Logout(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}),
	))

	mux.Handle("/api/auth/me", middleware.AuthMiddleware(config.JWTService)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				config.AuthHandlers.Me(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}),
	))

	mux.Handle("/api/auth/password", middleware.AuthMiddleware(config.JWTService)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				config.AuthHandlers.ChangePassword(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}),
	))

	// Products (public read, auth required for write)
	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			config.Handlers.GetProducts(w, r)
		case http.MethodPost:
			// Admin only for creating products
			middleware.AuthMiddleware(config.JWTService)(
				middleware.RequireRole("admin")(
					http.HandlerFunc(config.Handlers.CreateProduct),
				),
			).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/products/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			config.Handlers.GetProduct(w, r)
		case http.MethodPut:
			middleware.AuthMiddleware(config.JWTService)(
				middleware.RequireRole("admin")(
					http.HandlerFunc(config.Handlers.UpdateProduct),
				),
			).ServeHTTP(w, r)
		case http.MethodDelete:
			middleware.AuthMiddleware(config.JWTService)(
				middleware.RequireRole("admin")(
					http.HandlerFunc(config.Handlers.DeleteProduct),
				),
			).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Cart (optional auth - uses JWT user or X-User-ID header for backward compatibility)
	mux.Handle("/cart", middleware.OptionalAuthMiddleware(config.JWTService)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				config.Handlers.GetCart(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}),
	))

	mux.Handle("/cart/items", middleware.OptionalAuthMiddleware(config.JWTService)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				config.Handlers.AddToCart(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}),
	))

	mux.Handle("/cart/items/", middleware.OptionalAuthMiddleware(config.JWTService)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodDelete:
				config.Handlers.RemoveFromCart(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}),
	))

	// Orders (optional auth - uses JWT user or X-User-ID header for backward compatibility)
	mux.Handle("/orders", middleware.OptionalAuthMiddleware(config.JWTService)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				config.Handlers.GetOrders(w, r)
			case http.MethodPost:
				config.Handlers.PlaceOrder(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}),
	))

	mux.Handle("/orders/", middleware.OptionalAuthMiddleware(config.JWTService)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			switch {
			case strings.HasSuffix(path, "/cancel") && r.Method == http.MethodPost:
				config.Handlers.CancelOrder(w, r)
			case r.Method == http.MethodGet:
				config.Handlers.GetOrder(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}),
	))

	// Categories (public read, admin only for write)
	mux.HandleFunc("/api/categories", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			config.CategoryHandlers.ListCategories(w, r)
		case http.MethodPost:
			middleware.AuthMiddleware(config.JWTService)(
				middleware.RequireRole("admin")(
					http.HandlerFunc(config.CategoryHandlers.CreateCategory),
				),
			).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/categories/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			config.CategoryHandlers.GetCategory(w, r)
		case http.MethodPut:
			middleware.AuthMiddleware(config.JWTService)(
				middleware.RequireRole("admin")(
					http.HandlerFunc(config.CategoryHandlers.UpdateCategory),
				),
			).ServeHTTP(w, r)
		case http.MethodDelete:
			middleware.AuthMiddleware(config.JWTService)(
				middleware.RequireRole("admin")(
					http.HandlerFunc(config.CategoryHandlers.DeleteCategory),
				),
			).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Products by category
	mux.HandleFunc("/api/products/category/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			config.CategoryHandlers.GetProductsByCategory(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Product search
	mux.HandleFunc("/api/products/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			config.CategoryHandlers.SearchProducts(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Admin routes
	mux.Handle("/api/admin/orders", middleware.AuthMiddleware(config.JWTService)(
		middleware.RequireRole("admin")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					config.Handlers.GetAllOrders(w, r)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			}),
		),
	))

	return withCORS(withLogging(mux))
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		println("[API]", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Only set CORS headers if origin is in allowlist
		if origin != "" && allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID")
		}

		if r.Method == http.MethodOptions {
			if origin != "" && allowedOrigins[origin] {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusForbidden)
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}
