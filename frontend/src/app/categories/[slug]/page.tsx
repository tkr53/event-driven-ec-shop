'use client';

import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import api from '@/lib/api';
import type { Category, Product } from '@/types';

export default function CategoryPage() {
  const params = useParams();
  const slug = params.slug as string;

  const [category, setCategory] = useState<Category | null>(null);
  const [products, setProducts] = useState<Product[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [categoryData, productsData] = await Promise.all([
          api.getCategory(slug),
          api.getProductsByCategory(slug),
        ]);
        setCategory(categoryData);
        setProducts(productsData || []);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'データの取得に失敗しました');
      } finally {
        setIsLoading(false);
      }
    };

    fetchData();
  }, [slug]);

  const formatPrice = (price: number) => {
    return new Intl.NumberFormat('ja-JP', {
      style: 'currency',
      currency: 'JPY',
    }).format(price);
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

  if (error) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-400 px-4 py-3 rounded">
          {error}
        </div>
        <Link
          href="/categories"
          className="mt-4 inline-block text-blue-600 dark:text-blue-400 hover:text-blue-500"
        >
          カテゴリ一覧に戻る
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
      <Link
        href="/categories"
        className="mb-6 text-blue-600 dark:text-blue-400 hover:text-blue-500 flex items-center gap-1 inline-flex"
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
        </svg>
        カテゴリ一覧
      </Link>

      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900 dark:text-white">{category?.name}</h1>
        {category?.description && (
          <p className="mt-2 text-gray-600 dark:text-gray-400">{category.description}</p>
        )}
      </div>

      {products.length === 0 ? (
        <div className="text-center py-12 text-gray-500 dark:text-gray-400">
          このカテゴリには商品がありません
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          {products.map((product) => (
            <Link
              key={product.id}
              href={`/products/${product.id}`}
              className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden hover:shadow-lg transition-shadow"
            >
              <div className="aspect-square bg-gray-100 dark:bg-gray-700 flex items-center justify-center">
                {product.image_url ? (
                  <img
                    src={product.image_url}
                    alt={product.name}
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <svg className="w-16 h-16 text-gray-300 dark:text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                  </svg>
                )}
              </div>
              <div className="p-4">
                <h3 className="font-semibold text-gray-900 dark:text-white mb-1">{product.name}</h3>
                <p className="text-sm text-gray-500 dark:text-gray-400 mb-2 line-clamp-2">
                  {product.description}
                </p>
                <div className="flex justify-between items-center">
                  <span className="text-lg font-bold text-blue-600 dark:text-blue-400">
                    {formatPrice(product.price)}
                  </span>
                  <span className={`text-sm ${product.stock > 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>
                    {product.stock > 0 ? `在庫: ${product.stock}` : '在庫切れ'}
                  </span>
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
