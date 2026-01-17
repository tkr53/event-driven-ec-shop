// User types
export interface User {
  id: string;
  email: string;
  name: string;
  role: 'customer' | 'admin';
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  user: User;
  access_token: string;
  expires_at: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
  name: string;
}

export interface ChangePasswordRequest {
  current_password: string;
  new_password: string;
}

// Product types
export interface Product {
  id: string;
  name: string;
  description: string;
  price: number;
  stock: number;
  image_url?: string;
  category_ids?: string[];
  created_at: string;
  updated_at: string;
}

export interface CreateProductRequest {
  name: string;
  description: string;
  price: number;
  stock: number;
}

export interface UpdateProductRequest {
  name: string;
  description: string;
  price: number;
}

// Category types
export interface Category {
  id: string;
  name: string;
  slug: string;
  description: string;
  parent_id?: string;
  sort_order: number;
  children?: Category[];
}

export interface CreateCategoryRequest {
  name: string;
  slug?: string;
  description?: string;
  parent_id?: string;
  sort_order?: number;
}

// Cart types
export interface CartItem {
  product_id: string;
  name: string;
  quantity: number;
  price: number;
}

export interface Cart {
  id: string;
  user_id: string;
  items: CartItem[];
  total: number;
}

export interface AddToCartRequest {
  product_id: string;
  quantity: number;
}

// Order types
export interface OrderItem {
  product_id: string;
  quantity: number;
  price: number;
}

export interface Order {
  id: string;
  user_id: string;
  items: OrderItem[];
  total: number;
  status: 'pending' | 'paid' | 'shipped' | 'cancelled';
  created_at: string;
  updated_at: string;
}

// Search types
export interface SearchProductsParams {
  q?: string;
  category?: string;
  min_price?: number;
  max_price?: number;
  limit?: number;
  offset?: number;
}

// API Response types
export interface ApiError {
  error: string;
}

export interface MessageResponse {
  message: string;
}
