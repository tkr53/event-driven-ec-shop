'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import api from '@/lib/api';
import type { Category } from '@/types';

export default function CategoriesPage() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const fetchCategories = async () => {
      try {
        const data = await api.getCategories();
        setCategories(data || []);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'カテゴリの取得に失敗しました');
      } finally {
        setIsLoading(false);
      }
    };

    fetchCategories();
  }, []);

  const renderCategory = (category: Category, level: number = 0) => (
    <div key={category.id} style={{ marginLeft: level * 24 }}>
      <Link
        href={`/categories/${category.slug}`}
        className="block p-4 bg-white border rounded-lg mb-2 hover:shadow-md transition-shadow"
      >
        <h3 className="font-semibold text-gray-900">{category.name}</h3>
        {category.description && (
          <p className="text-sm text-gray-500 mt-1">{category.description}</p>
        )}
      </Link>
      {category.children?.map((child) => renderCategory(child, level + 1))}
    </div>
  );

  if (isLoading) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="flex justify-center py-12">
          <div className="w-8 h-8 border-2 border-gray-300 border-t-blue-600 rounded-full animate-spin" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
          {error}
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
      <h1 className="text-3xl font-bold text-gray-900 mb-8">カテゴリ</h1>

      {categories.length === 0 ? (
        <div className="text-center py-12 text-gray-500">
          カテゴリがありません
        </div>
      ) : (
        <div className="space-y-2">
          {categories.map((category) => renderCategory(category))}
        </div>
      )}
    </div>
  );
}
