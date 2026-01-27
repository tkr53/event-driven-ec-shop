import type {
  User,
  AuthResponse,
  LoginRequest,
  RegisterRequest,
  ChangePasswordRequest,
  Product,
  CreateProductRequest,
  UpdateProductRequest,
  Category,
  CreateCategoryRequest,
  Cart,
  AddToCartRequest,
  Order,
  SearchProductsParams,
  MessageResponse,
} from '@/types';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

class ApiClient {
  private baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    const config: RequestInit = {
      ...options,
      credentials: 'include', // Include cookies for authentication
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    };

    const response = await fetch(url, config);

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'An error occurred' }));
      throw new Error(error.error || `HTTP error! status: ${response.status}`);
    }

    // Handle empty responses
    const text = await response.text();
    if (!text) {
      return {} as T;
    }

    return JSON.parse(text);
  }

  // Auth endpoints
  async register(data: RegisterRequest): Promise<AuthResponse> {
    return this.request<AuthResponse>('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async login(data: LoginRequest): Promise<AuthResponse> {
    return this.request<AuthResponse>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async logout(): Promise<MessageResponse> {
    return this.request<MessageResponse>('/api/auth/logout', {
      method: 'POST',
    });
  }

  async refreshToken(): Promise<AuthResponse> {
    return this.request<AuthResponse>('/api/auth/refresh', {
      method: 'POST',
    });
  }

  async getMe(): Promise<User> {
    return this.request<User>('/api/auth/me');
  }

  async changePassword(data: ChangePasswordRequest): Promise<MessageResponse> {
    return this.request<MessageResponse>('/api/auth/password', {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  // Product endpoints
  async getProducts(): Promise<Product[]> {
    return this.request<Product[]>('/products');
  }

  async getProduct(id: string): Promise<Product> {
    return this.request<Product>(`/products/${id}`);
  }

  async createProduct(data: CreateProductRequest): Promise<Product> {
    return this.request<Product>('/products', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateProduct(id: string, data: UpdateProductRequest): Promise<MessageResponse> {
    return this.request<MessageResponse>(`/products/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteProduct(id: string): Promise<MessageResponse> {
    return this.request<MessageResponse>(`/products/${id}`, {
      method: 'DELETE',
    });
  }

  async searchProducts(params: SearchProductsParams): Promise<Product[]> {
    const searchParams = new URLSearchParams();
    if (params.q) searchParams.set('q', params.q);
    if (params.category) searchParams.set('category', params.category);
    if (params.min_price !== undefined) searchParams.set('min_price', params.min_price.toString());
    if (params.max_price !== undefined) searchParams.set('max_price', params.max_price.toString());
    if (params.limit !== undefined) searchParams.set('limit', params.limit.toString());
    if (params.offset !== undefined) searchParams.set('offset', params.offset.toString());

    return this.request<Product[]>(`/api/products/search?${searchParams.toString()}`);
  }

  // Category endpoints
  async getCategories(): Promise<Category[]> {
    return this.request<Category[]>('/api/categories');
  }

  async getCategory(slug: string): Promise<Category> {
    return this.request<Category>(`/api/categories/${slug}`);
  }

  async createCategory(data: CreateCategoryRequest): Promise<Category> {
    return this.request<Category>('/api/categories', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateCategory(id: string, data: CreateCategoryRequest): Promise<MessageResponse> {
    return this.request<MessageResponse>(`/api/categories/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteCategory(id: string): Promise<MessageResponse> {
    return this.request<MessageResponse>(`/api/categories/${id}`, {
      method: 'DELETE',
    });
  }

  async getProductsByCategory(slug: string): Promise<Product[]> {
    return this.request<Product[]>(`/api/products/category/${slug}`);
  }

  // Cart endpoints
  async getCart(): Promise<Cart> {
    return this.request<Cart>('/cart');
  }

  async getCartWithRetry(maxRetries = 5): Promise<Cart> {
    for (let i = 0; i < maxRetries; i++) {
      try {
        return await this.getCart();
      } catch (error) {
        if (i === maxRetries - 1) throw error;
        await new Promise(r => setTimeout(r, 200 * Math.pow(2, i))); // 200ms, 400ms, 800ms...
      }
    }
    throw new Error('Cart not found');
  }

  async addToCart(data: AddToCartRequest): Promise<Cart> {
    return this.request<Cart>('/cart/items', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async removeFromCart(productId: string): Promise<Cart> {
    return this.request<Cart>(`/cart/items/${productId}`, {
      method: 'DELETE',
    });
  }

  // Order endpoints
  async getOrders(): Promise<Order[]> {
    return this.request<Order[]>('/orders');
  }

  async getOrder(id: string): Promise<Order> {
    return this.request<Order>(`/orders/${id}`);
  }

  async getOrderWithRetry(id: string, maxRetries = 5): Promise<Order> {
    for (let i = 0; i < maxRetries; i++) {
      try {
        return await this.getOrder(id);
      } catch (error) {
        if (i === maxRetries - 1) throw error;
        await new Promise(r => setTimeout(r, 200 * Math.pow(2, i))); // 200ms, 400ms, 800ms...
      }
    }
    throw new Error('Order not found');
  }

  async placeOrder(): Promise<Order> {
    return this.request<Order>('/orders', {
      method: 'POST',
    });
  }

  async cancelOrder(id: string): Promise<MessageResponse> {
    return this.request<MessageResponse>(`/orders/${id}/cancel`, {
      method: 'POST',
    });
  }

  // Admin endpoints
  async getAllOrders(): Promise<Order[]> {
    return this.request<Order[]>('/api/admin/orders');
  }
}

export const api = new ApiClient(API_BASE_URL);
export default api;
