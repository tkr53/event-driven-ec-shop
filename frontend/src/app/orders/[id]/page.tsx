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
    const fetchOrder = async () => {
      try {
        const data = await api.getOrder(orderId);
        setOrder(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : '注文の取得に失敗しました');
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
      // Refresh order data
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
      pending: 'bg-yellow-100 text-yellow-800',
      paid: 'bg-blue-100 text-blue-800',
      shipped: 'bg-green-100 text-green-800',
      cancelled: 'bg-red-100 text-red-800',
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
          <div className="w-8 h-8 border-2 border-gray-300 border-t-blue-600 rounded-full animate-spin" />
        </div>
      </div>
    );
  }

  if (error && !order) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
          {error}
        </div>
        <button
          onClick={() => router.back()}
          className="mt-4 text-blue-600 hover:text-blue-500"
        >
          戻る
        </button>
      </div>
    );
  }

  if (!order) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="text-center py-12 text-gray-500">
          注文が見つかりませんでした
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
      <Link
        href="/orders"
        className="mb-6 text-blue-600 hover:text-blue-500 flex items-center gap-1 inline-flex"
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
        </svg>
        注文履歴に戻る
      </Link>

      <div className="bg-white border rounded-lg overflow-hidden">
        {/* Order Header */}
        <div className="p-6 border-b">
          <div className="flex justify-between items-start">
            <div>
              <h1 className="text-2xl font-bold text-gray-900 mb-2">注文詳細</h1>
              <div className="text-sm text-gray-500">
                注文番号: <span className="font-mono">{order.id}</span>
              </div>
              <div className="text-sm text-gray-500">
                注文日: {formatDate(order.created_at)}
              </div>
            </div>
            <div className="text-right">
              {getStatusBadge(order.status)}
              {order.status === 'pending' && (
                <button
                  onClick={handleCancelOrder}
                  disabled={isCancelling}
                  className="mt-2 block text-sm text-red-600 hover:text-red-500 disabled:opacity-50"
                >
                  {isCancelling ? 'キャンセル中...' : '注文をキャンセル'}
                </button>
              )}
            </div>
          </div>
        </div>

        {error && (
          <div className="p-4 bg-red-50 border-b border-red-200 text-red-700">
            {error}
          </div>
        )}

        {/* Order Items */}
        <div className="p-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">注文商品</h2>
          <div className="space-y-4">
            {order.items.map((item, index) => (
              <div
                key={index}
                className="flex justify-between items-center py-2 border-b last:border-0"
              >
                <div>
                  <Link
                    href={`/products/${item.product_id}`}
                    className="text-gray-900 hover:text-blue-600"
                  >
                    商品ID: {item.product_id}
                  </Link>
                  <div className="text-sm text-gray-500">
                    {formatPrice(item.price)} × {item.quantity}
                  </div>
                </div>
                <div className="font-semibold text-gray-900">
                  {formatPrice(item.price * item.quantity)}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Order Summary */}
        <div className="p-6 bg-gray-50 border-t">
          <div className="flex justify-between items-center">
            <span className="text-lg font-semibold text-gray-900">合計</span>
            <span className="text-2xl font-bold text-gray-900">
              {formatPrice(order.total)}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
