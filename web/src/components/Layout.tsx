import { type ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Activity, FileText, FolderOpen, Settings, Zap } from 'lucide-react';

const navItems = [
  { path: '/', label: 'Dashboard', icon: Activity },
  { path: '/rules', label: 'Rules', icon: FileText },
  { path: '/projects', label: 'Projects', icon: FolderOpen },
  { path: '/settings', label: 'Settings', icon: Settings },
];

export default function Layout({ children }: { children: ReactNode }) {
  const location = useLocation();

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100 flex">
      {/* Sidebar */}
      <aside className="w-56 bg-gray-900 border-r border-gray-800 flex flex-col">
        <div className="p-5 border-b border-gray-800">
          <Link to="/" className="flex items-center gap-2 text-lg font-bold">
            <span className="text-purple-400 text-xl">👻</span>
            <span className="text-purple-400">Shadow</span>
          </Link>
          <p className="text-xs text-gray-500 mt-1">AI Agent Memory Layer</p>
        </div>

        <nav className="flex-1 p-3 space-y-1">
          {navItems.map(({ path, label, icon: Icon }) => {
            const active = location.pathname === path || (path !== '/' && location.pathname.startsWith(path));
            return (
              <Link
                key={path}
                to={path}
                className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                  active
                    ? 'bg-purple-500/10 text-purple-400'
                    : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800'
                }`}
              >
                <Icon size={18} />
                {label}
              </Link>
            );
          })}
        </nav>

        <div className="p-4 border-t border-gray-800">
          <div className="flex items-center gap-2 text-xs text-gray-500">
            <Zap size={14} className="text-green-500" />
            Daemon Running
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        {children}
      </main>
    </div>
  );
}
