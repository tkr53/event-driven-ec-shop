export function Footer() {
  return (
    <footer className="bg-gray-50 dark:bg-gray-900 border-t border-gray-200 dark:border-gray-700 mt-auto">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
          <div>
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wider">
              EC Shop
            </h3>
            <p className="mt-4 text-sm text-gray-600 dark:text-gray-400">
              イベント駆動型ECサイト
            </p>
          </div>
          <div>
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wider">
              サポート
            </h3>
            <ul className="mt-4 space-y-2">
              <li>
                <a href="#" className="text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white">
                  ヘルプ
                </a>
              </li>
              <li>
                <a href="#" className="text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white">
                  配送について
                </a>
              </li>
              <li>
                <a href="#" className="text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white">
                  返品について
                </a>
              </li>
            </ul>
          </div>
          <div>
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wider">
              お問い合わせ
            </h3>
            <ul className="mt-4 space-y-2">
              <li className="text-sm text-gray-600 dark:text-gray-400">
                メール: support@example.com
              </li>
              <li className="text-sm text-gray-600 dark:text-gray-400">
                電話: 03-1234-5678
              </li>
            </ul>
          </div>
        </div>
        <div className="mt-8 pt-8 border-t border-gray-200 dark:border-gray-700">
          <p className="text-sm text-gray-500 dark:text-gray-400 text-center">
            &copy; 2025 EC Shop. All rights reserved.
          </p>
        </div>
      </div>
    </footer>
  );
}
