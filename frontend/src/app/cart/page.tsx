'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import api from '@/lib/api';
import { useAuth } from '@/contexts/AuthContext';
import type { Cart } from '@/types';

export default function CartPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();
  const [cart, setCart] = useState<Cart | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isPlacingOrder, setIsPlacingOrder] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    // Redirect to login if not authenticated
    if (!authLoading && !user) {
      router.push('/login?redirect=/cart');
      return;
    }

    const fetchCart = async () => {
      try {
        const data = await api.getCart();
        setCart(data);
      } catch {
        // Cart might not exist yet, which is fine
        setCart(null);
      } finally {
        setIsLoading(false);
      }
    };

    if (user) {
      fetchCart();
    }
  }, [user, authLoading, router]);

  const handleRemoveItem = async (productId: string) => {
    try {
      await api.removeFromCart(productId);
      // Refresh cart after removing item
      const data = await api.getCart();
      setCart(data);
    } catch {
      setError('商品の削除に失敗しました');
    }
  };

  const handlePlaceOrder = async () => {
    setIsPlacingOrder(true);
    setError('');

    try {
      const order = await api.placeOrder();
      // 注文データをsessionStorageに一時保存（Optimistic UI用）
      sessionStorage.setItem(`order_${order.id}`, JSON.stringify(order));
      router.push(`/orders/${order.id}`);
    } catch {
      setError('注文に失敗しました。在庫が不足している可能性があります。');
      setIsPlacingOrder(false);
    }
  };

  const formatPrice = (price: number) => {
    return new Intl.NumberFormat('ja-JP', {
      style: 'currency',
      currency: 'JPY',
    }).format(price);
  };

  if (authLoading || isLoading) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="flex justify-center py-12">
          <div className="w-8 h-8 border-2 border-gray-300 dark:border-gray-600 border-t-blue-600 rounded-full animate-spin" />
        </div>
      </div>
    );
  }

  // Show nothing while redirecting
  if (!user) {
    return null;
  }

  const hasItems = cart && cart.items && cart.items.length > 0;

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
      <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-8">ショッピングカート</h1>

      {error && (
        <div className="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-400 px-4 py-3 rounded mb-6">
          {error}
        </div>
      )}

      {!hasItems ? (
        <div className="text-center py-12">
          <svg className="w-16 h-16 text-gray-300 dark:text-gray-600 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 3h2l.4 2M7 13h10l4-8H5.4M7 13L5.4 5M7 13l-2.293 2.293c-.63.63-.184 1.707.707 1.707H17m0 0a2 2 0 100 4 2 2 0 000-4zm-8 2a2 2 0 11-4 0 2 2 0 014 0z" />
          </svg>
          <p className="text-gray-500 dark:text-gray-400 mb-4">カートは空です</p>
          <Link
            href="/products"
            className="text-blue-600 dark:text-blue-400 hover:text-blue-500"
          >
            商品を探す
          </Link>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Cart Items */}
          <div className="lg:col-span-2">
            <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
              {cart.items.map((item, index) => (
                <div
                  key={item.product_id}
                  className={`p-4 flex items-center gap-4 ${
                    index > 0 ? 'border-t border-gray-200 dark:border-gray-700' : ''
                  }`}
                >
                  <div className="w-20 h-20 bg-gray-100 dark:bg-gray-700 rounded flex items-center justify-center flex-shrink-0">
                    <svg className="w-8 h-8 text-gray-300 dark:text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                    </svg>
                  </div>
                  <div className="flex-1">
                    <Link
                      href={`/products/${item.product_id}`}
                      className="font-semibold text-gray-900 dark:text-white hover:text-blue-600 dark:hover:text-blue-400"
                    >
                      {item.name || item.product_id}
                    </Link>
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      {formatPrice(item.price)} × {item.quantity}
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="font-semibold text-gray-900 dark:text-white">
                      {formatPrice(item.price * item.quantity)}
                    </div>
                    <button
                      onClick={() => handleRemoveItem(item.product_id)}
                      className="text-sm text-red-600 dark:text-red-400 hover:text-red-500"
                    >
                      削除
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Order Summary */}
          <div>
            <div className="bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">注文サマリー</h2>
              <div className="space-y-2 mb-4">
                <div className="flex justify-between text-gray-600 dark:text-gray-400">
                  <span>小計</span>
                  <span>{formatPrice(cart.total)}</span>
                </div>
                <div className="flex justify-between text-gray-600 dark:text-gray-400">
                  <span>送料</span>
                  <span>無料</span>
                </div>
              </div>
              <div className="border-t border-gray-200 dark:border-gray-700 pt-4 mb-6">
                <div className="flex justify-between text-lg font-semibold text-gray-900 dark:text-white">
                  <span>合計</span>
                  <span>{formatPrice(cart.total)}</span>
                </div>
              </div>
              <button
                onClick={handlePlaceOrder}
                disabled={isPlacingOrder}
                className="w-full py-3 px-4 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isPlacingOrder ? '注文処理中...' : '注文を確定する'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
