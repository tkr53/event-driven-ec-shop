'use client';

import { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import api from '@/lib/api';
import type { Product } from '@/types';

export default function AdminProductsPage() {
  const [products, setProducts] = useState<Product[]>([]);
  const [pendingProducts, setPendingProducts] = useState<Product[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [editingProduct, setEditingProduct] = useState<Product | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Load pending products from sessionStorage
  const loadPendingProducts = useCallback(() => {
    if (typeof window === 'undefined') return [];
    const pending: Product[] = [];
    for (let i = 0; i < sessionStorage.length; i++) {
      const key = sessionStorage.key(i);
      if (key?.startsWith('product_created_')) {
        try {
          const product = JSON.parse(sessionStorage.getItem(key) || '');
          pending.push(product);
        } catch {
          sessionStorage.removeItem(key!);
        }
      }
    }
    return pending;
  }, []);

  // Clear pending product from sessionStorage if it exists on server
  const clearConfirmedProducts = useCallback((serverProducts: Product[]) => {
    if (typeof window === 'undefined') return;
    const serverIds = new Set(serverProducts.map(p => p.id));
    for (let i = sessionStorage.length - 1; i >= 0; i--) {
      const key = sessionStorage.key(i);
      if (key?.startsWith('product_created_')) {
        const productId = key.replace('product_created_', '');
        if (serverIds.has(productId)) {
          sessionStorage.removeItem(key);
        }
      }
    }
  }, []);

  const fetchProducts = useCallback(async () => {
    try {
      const data = await api.getProducts();
      const serverProducts = data || [];
      setProducts(serverProducts);

      // Clear confirmed products from sessionStorage
      clearConfirmedProducts(serverProducts);

      // Update pending products
      const pending = loadPendingProducts();
      const serverIds = new Set(serverProducts.map(p => p.id));
      setPendingProducts(pending.filter(p => !serverIds.has(p.id)));
    } catch (err) {
      setError(err instanceof Error ? err.message : '商品の取得に失敗しました');
    } finally {
      setIsLoading(false);
    }
  }, [clearConfirmedProducts, loadPendingProducts]);

  useEffect(() => {
    // Load pending products immediately for instant display
    const pending = loadPendingProducts();
    setPendingProducts(pending);

    fetchProducts();
  }, [fetchProducts, loadPendingProducts]);

  const handleDelete = async (id: string, isPending = false) => {
    if (!confirm('この商品を削除しますか？')) return;

    if (isPending) {
      // Remove from pending products
      sessionStorage.removeItem(`product_created_${id}`);
      setPendingProducts(pendingProducts.filter((p) => p.id !== id));
      return;
    }

    // Optimistic update
    const previousProducts = products;
    setProducts(products.filter((p) => p.id !== id));

    try {
      await api.deleteProduct(id);
    } catch (err) {
      // Rollback on error
      setProducts(previousProducts);
      setError(err instanceof Error ? err.message : '商品の削除に失敗しました');
    }
  };

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingProduct) return;

    setIsSubmitting(true);
    setError('');

    // Optimistic update
    const previousProducts = products;
    setProducts(products.map(p =>
      p.id === editingProduct.id ? editingProduct : p
    ));
    setEditingProduct(null);

    try {
      await api.updateProduct(editingProduct.id, {
        name: editingProduct.name,
        description: editingProduct.description,
        price: editingProduct.price,
      });
      // Sync with server after delay
      setTimeout(() => fetchProducts(), 500);
    } catch (err) {
      // Rollback on error
      setProducts(previousProducts);
      setError(err instanceof Error ? err.message : '商品の更新に失敗しました');
    } finally {
      setIsSubmitting(false);
    }
  };

  const formatPrice = (price: number) => {
    return new Intl.NumberFormat('ja-JP', {
      style: 'currency',
      currency: 'JPY',
    }).format(price);
  };

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <div className="w-8 h-8 border-2 border-gray-300 dark:border-gray-600 border-t-blue-600 rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div>
      <div className="flex justify-between items-center mb-8">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">商品管理</h1>
        <Link
          href="/admin/products/new"
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
        >
          商品を追加
        </Link>
      </div>

      {error && (
        <div className="mb-6 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-400 px-4 py-3 rounded">
          {error}
        </div>
      )}

      {/* Edit Modal */}
      {editingProduct && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-4">商品を編集</h2>
            <form onSubmit={handleUpdate} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  商品名
                </label>
                <input
                  type="text"
                  value={editingProduct.name}
                  onChange={(e) => setEditingProduct({ ...editingProduct, name: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  説明
                </label>
                <textarea
                  value={editingProduct.description}
                  onChange={(e) => setEditingProduct({ ...editingProduct, description: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                  rows={3}
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  価格
                </label>
                <input
                  type="number"
                  value={editingProduct.price}
                  onChange={(e) => setEditingProduct({ ...editingProduct, price: parseInt(e.target.value) || 0 })}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                  required
                  min="1"
                />
              </div>
              <div className="flex gap-3 pt-4">
                <button
                  type="button"
                  onClick={() => setEditingProduct(null)}
                  className="flex-1 px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                >
                  キャンセル
                </button>
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50"
                >
                  {isSubmitting ? '更新中...' : '更新'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Products Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 dark:bg-gray-900">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">商品名</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">価格</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">在庫</th>
                <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {/* Pending products (optimistic, not yet on server) */}
              {pendingProducts.map((product) => (
                <tr key={`pending-${product.id}`} className="bg-yellow-50 dark:bg-yellow-900/20">
                  <td className="px-6 py-4">
                    <div>
                      <div className="font-medium text-gray-900 dark:text-white flex items-center gap-2">
                        {product.name}
                        <span className="text-xs px-2 py-0.5 bg-yellow-200 dark:bg-yellow-800 text-yellow-800 dark:text-yellow-200 rounded">
                          同期中...
                        </span>
                      </div>
                      <div className="text-sm text-gray-500 dark:text-gray-400 truncate max-w-xs">{product.description}</div>
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-gray-900 dark:text-white">
                    {formatPrice(product.price)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span className={`${product.stock > 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>
                      {product.stock}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right">
                    <button
                      onClick={() => handleDelete(product.id, true)}
                      className="text-red-600 dark:text-red-400 hover:text-red-500"
                    >
                      削除
                    </button>
                  </td>
                </tr>
              ))}
              {/* Confirmed products from server */}
              {products.map((product) => (
                <tr key={product.id}>
                  <td className="px-6 py-4">
                    <div>
                      <div className="font-medium text-gray-900 dark:text-white">{product.name}</div>
                      <div className="text-sm text-gray-500 dark:text-gray-400 truncate max-w-xs">{product.description}</div>
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-gray-900 dark:text-white">
                    {formatPrice(product.price)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span className={`${product.stock > 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>
                      {product.stock}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right">
                    <button
                      onClick={() => setEditingProduct(product)}
                      className="text-blue-600 dark:text-blue-400 hover:text-blue-500 mr-4"
                    >
                      編集
                    </button>
                    <button
                      onClick={() => handleDelete(product.id)}
                      className="text-red-600 dark:text-red-400 hover:text-red-500"
                    >
                      削除
                    </button>
                  </td>
                </tr>
              ))}
              {products.length === 0 && pendingProducts.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-6 py-8 text-center text-gray-500 dark:text-gray-400">
                    商品がありません
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
