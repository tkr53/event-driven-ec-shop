'use client';

import { useState, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import api from '@/lib/api';
import type { Order } from '@/types';

export default function OrderDetailPage() {
  const params = useParams();
  const router = useRouter();
  const orderId = params.id as string;

  const [order, setOrder] = useState<Order | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isCancelling, setIsCancelling] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    // 1. sessionStorageから初期データを取得（Optimistic UI）
    const cached = sessionStorage.getItem(`order_${orderId}`);
    if (cached) {
      try {
        setOrder(JSON.parse(cached));
        setIsLoading(false);
      } catch {
        // パースエラーは無視
      }
    }

    // 2. バックグラウンドでRead Storeからリトライ取得
    const fetchOrder = async () => {
      try {
        const data = await api.getOrderWithRetry(orderId);
        setOrder(data);
        sessionStorage.removeItem(`order_${orderId}`); // キャッシュ削除
      } catch (err) {
        // sessionStorageにデータがなければエラー表示
        if (!cached) {
          setError(err instanceof Error ? err.message : '注文の取得に失敗しました');
        }
      } finally {
        setIsLoading(false);
      }
    };

    fetchOrder();
  }, [orderId]);

  const handleCancelOrder = async () => {
    if (!confirm('この注文をキャンセルしますか？')) return;

    setIsCancelling(true);
    try {
      await api.cancelOrder(orderId);
      const data = await api.getOrder(orderId);
      setOrder(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : '注文のキャンセルに失敗しました');
    } finally {
      setIsCancelling(false);
    }
  };

  const formatPrice = (price: number) => {
    return new Intl.NumberFormat('ja-JP', {
      style: 'currency',
      currency: 'JPY',
    }).format(price);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('ja-JP', {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const getStatusBadge = (status: Order['status']) => {
    const styles = {
      pending: 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-800 dark:text-yellow-300',
      paid: 'bg-blue-100 dark:bg-blue-900/50 text-blue-800 dark:text-blue-300',
      shipped: 'bg-green-100 dark:bg-green-900/50 text-green-800 dark:text-green-300',
      cancelled: 'bg-red-100 dark:bg-red-900/50 text-red-800 dark:text-red-300',
    };
    const labels = {
      pending: '処理中',
      paid: '支払い済み',
      shipped: '発送済み',
      cancelled: 'キャンセル',
    };
    return (
      <span className={`px-3 py-1 text-sm font-medium rounded-full ${styles[status]}`}>
        {labels[status]}
      </span>
    );
  };

  if (isLoading) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="flex justify-center py-12">
          <div className="w-8 h-8 border-2 border-gray-300 dark:border-gray-600 border-t-blue-600 rounded-full animate-spin" />
        </div>
      </div>
    );
  }

  if (error && !order) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-400 px-4 py-3 rounded">
          {error}
        </div>
        <button
          onClick={() => router.back()}
          className="mt-4 text-blue-600 dark:text-blue-400 hover:text-blue-500"
        >
          戻る
        </button>
      </div>
    );
  }

  if (!order) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="text-center py-12 text-gray-500 dark:text-gray-400">
          注文が見つかりませんでした
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
      <Link
        href="/orders"
        className="mb-6 text-blue-600 dark:text-blue-400 hover:text-blue-500 flex items-center gap-1 inline-flex"
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
        </svg>
        注文履歴に戻る
      </Link>

      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
        {/* Order Header */}
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex justify-between items-start">
            <div>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-2">注文詳細</h1>
              <div className="text-sm text-gray-500 dark:text-gray-400">
                注文番号: <span className="font-mono">{order.id}</span>
              </div>
              <div className="text-sm text-gray-500 dark:text-gray-400">
                注文日: {formatDate(order.created_at)}
              </div>
            </div>
            <div className="text-right">
              {getStatusBadge(order.status)}
              {order.status === 'pending' && (
                <button
                  onClick={handleCancelOrder}
                  disabled={isCancelling}
                  className="mt-2 block text-sm text-red-600 dark:text-red-400 hover:text-red-500 disabled:opacity-50"
                >
                  {isCancelling ? 'キャンセル中...' : '注文をキャンセル'}
                </button>
              )}
            </div>
          </div>
        </div>

        {error && (
          <div className="p-4 bg-red-50 dark:bg-red-900/30 border-b border-red-200 dark:border-red-800 text-red-700 dark:text-red-400">
            {error}
          </div>
        )}

        {/* Order Items */}
        <div className="p-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">注文商品</h2>
          <div className="space-y-4">
            {order.items.map((item, index) => (
              <div
                key={index}
                className="flex justify-between items-center py-2 border-b border-gray-200 dark:border-gray-700 last:border-0"
              >
                <div>
                  <Link
                    href={`/products/${item.product_id}`}
                    className="text-gray-900 dark:text-white hover:text-blue-600 dark:hover:text-blue-400"
                  >
                    商品ID: {item.product_id}
                  </Link>
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    {formatPrice(item.price)} × {item.quantity}
                  </div>
                </div>
                <div className="font-semibold text-gray-900 dark:text-white">
                  {formatPrice(item.price * item.quantity)}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Order Summary */}
        <div className="p-6 bg-gray-50 dark:bg-gray-900 border-t border-gray-200 dark:border-gray-700">
          <div className="flex justify-between items-center">
            <span className="text-lg font-semibold text-gray-900 dark:text-white">合計</span>
            <span className="text-2xl font-bold text-gray-900 dark:text-white">
              {formatPrice(order.total)}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
