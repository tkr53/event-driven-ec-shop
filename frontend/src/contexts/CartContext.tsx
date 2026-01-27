'use client';

import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react';
import type { Cart, Product } from '@/types';
import api from '@/lib/api';
import { useAuth } from './AuthContext';

interface CartContextType {
  cart: Cart | null;
  isLoading: boolean;
  itemCount: number;
  addToCart: (product: Product, quantity: number) => Promise<void>;
  removeFromCart: (productId: string) => Promise<void>;
  refreshCart: () => Promise<void>;
  clearCart: () => void;
}

const CartContext = createContext<CartContextType | undefined>(undefined);

const CART_STORAGE_KEY = 'optimistic_cart';

export function CartProvider({ children }: { children: ReactNode }) {
  const { user, isAuthenticated } = useAuth();
  const [cart, setCart] = useState<Cart | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Calculate item count from cart
  const itemCount = cart?.items?.reduce((sum, item) => sum + item.quantity, 0) ?? 0;

  // Load cart from sessionStorage on mount
  const loadOptimisticCart = useCallback(() => {
    if (typeof window === 'undefined') return null;
    const stored = sessionStorage.getItem(CART_STORAGE_KEY);
    if (stored) {
      try {
        return JSON.parse(stored) as Cart;
      } catch {
        sessionStorage.removeItem(CART_STORAGE_KEY);
      }
    }
    return null;
  }, []);

  // Save cart to sessionStorage
  const saveOptimisticCart = useCallback((cartData: Cart | null) => {
    if (typeof window === 'undefined') return;
    if (cartData) {
      sessionStorage.setItem(CART_STORAGE_KEY, JSON.stringify(cartData));
    } else {
      sessionStorage.removeItem(CART_STORAGE_KEY);
    }
  }, []);

  // Fetch cart from API
  const fetchCart = useCallback(async () => {
    if (!isAuthenticated) {
      setCart(null);
      setIsLoading(false);
      return;
    }

    try {
      const data = await api.getCart();
      setCart(data);
      saveOptimisticCart(data);
    } catch {
      // Cart might not exist yet
      setCart(null);
      saveOptimisticCart(null);
    } finally {
      setIsLoading(false);
    }
  }, [isAuthenticated, saveOptimisticCart]);

  // Initialize cart on mount and auth change
  useEffect(() => {
    if (!isAuthenticated) {
      setCart(null);
      setIsLoading(false);
      sessionStorage.removeItem(CART_STORAGE_KEY);
      return;
    }

    // Load optimistic cart first for instant UI
    const optimisticCart = loadOptimisticCart();
    if (optimisticCart && optimisticCart.user_id === user?.id) {
      setCart(optimisticCart);
      setIsLoading(false);
    }

    // Then fetch real cart
    fetchCart();
  }, [isAuthenticated, user?.id, fetchCart, loadOptimisticCart]);

  // Add item to cart with optimistic update
  const addToCart = useCallback(async (product: Product, quantity: number) => {
    if (!user) return;

    // Optimistic update
    const previousCart = cart;
    const newItem = {
      product_id: product.id,
      name: product.name,
      quantity,
      price: product.price,
    };

    const optimisticCart: Cart = cart
      ? {
          ...cart,
          items: (() => {
            const existingIndex = cart.items.findIndex(
              (item) => item.product_id === product.id
            );
            if (existingIndex >= 0) {
              const updatedItems = [...cart.items];
              updatedItems[existingIndex] = {
                ...updatedItems[existingIndex],
                quantity: updatedItems[existingIndex].quantity + quantity,
              };
              return updatedItems;
            }
            return [...cart.items, newItem];
          })(),
          total: cart.total + product.price * quantity,
        }
      : {
          id: 'optimistic',
          user_id: user.id,
          items: [newItem],
          total: product.price * quantity,
        };

    setCart(optimisticCart);
    saveOptimisticCart(optimisticCart);

    try {
      // API call
      await api.addToCart({
        product_id: product.id,
        quantity,
      });

      // Sync with server after a short delay
      setTimeout(async () => {
        try {
          const serverCart = await api.getCartWithRetry();
          setCart(serverCart);
          saveOptimisticCart(serverCart);
        } catch {
          // Keep optimistic state if sync fails
        }
      }, 500);
    } catch (error) {
      // Rollback on error
      setCart(previousCart);
      saveOptimisticCart(previousCart);
      throw error;
    }
  }, [cart, user, saveOptimisticCart]);

  // Remove item from cart with optimistic update
  const removeFromCart = useCallback(async (productId: string) => {
    if (!cart) return;

    // Optimistic update
    const previousCart = cart;
    const removedItem = cart.items.find((item) => item.product_id === productId);
    if (!removedItem) return;

    const optimisticCart: Cart = {
      ...cart,
      items: cart.items.filter((item) => item.product_id !== productId),
      total: cart.total - removedItem.price * removedItem.quantity,
    };

    setCart(optimisticCart);
    saveOptimisticCart(optimisticCart);

    try {
      // API call
      await api.removeFromCart(productId);

      // Sync with server after a short delay
      setTimeout(async () => {
        try {
          const serverCart = await api.getCartWithRetry();
          setCart(serverCart);
          saveOptimisticCart(serverCart);
        } catch {
          // Keep optimistic state if sync fails
        }
      }, 500);
    } catch (error) {
      // Rollback on error
      setCart(previousCart);
      saveOptimisticCart(previousCart);
      throw error;
    }
  }, [cart, saveOptimisticCart]);

  // Refresh cart from server
  const refreshCart = useCallback(async () => {
    await fetchCart();
  }, [fetchCart]);

  // Clear cart (used after placing an order)
  const clearCart = useCallback(() => {
    setCart(null);
    saveOptimisticCart(null);
  }, [saveOptimisticCart]);

  return (
    <CartContext.Provider
      value={{
        cart,
        isLoading,
        itemCount,
        addToCart,
        removeFromCart,
        refreshCart,
        clearCart,
      }}
    >
      {children}
    </CartContext.Provider>
  );
}

export function useCart() {
  const context = useContext(CartContext);
  if (context === undefined) {
    throw new Error('useCart must be used within a CartProvider');
  }
  return context;
}
