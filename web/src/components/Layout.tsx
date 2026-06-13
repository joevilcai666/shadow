import { type ReactNode, useCallback, useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Activity, Database, FileText, FolderOpen, Settings, Zap, CheckSquare, AlertTriangle } from 'lucide-react';
import { Chip } from '@heroui/react';
import { api } from '../lib/api';
import { ShadowCard } from './ui';
import { useRealtimeRefresh } from '../lib/realtime';

const navItems = [
  { path: '/', label: 'Dashboard', icon: Activity },
  { path: '/rules', label: 'Rules', icon: FileText },
  { path: '/memories', label: 'Memories', icon: Database },
  { path: '/review', label: 'Review', icon: CheckSquare },
  { path: '/conflicts', label: 'Conflicts', icon: AlertTriangle },
  { path: '/projects', label: 'Projects', icon: FolderOpen },
  { path: '/settings', label: 'Settings', icon: Settings },
];

export default function Layout({ children }: { children: ReactNode }) {
  const location = useLocation();
  const [candidateCount, setCandidateCount] = useState(0);
  const [conflictCount, setConflictCount] = useState(0);

  const loadCounts = useCallback(() => {
    api.listRules({ status: 'candidate' }).then(r => setCandidateCount(r?.length ?? 0)).catch(() => {});
    api.listRules({ status: 'conflicted' }).then(r => setConflictCount(r?.length ?? 0)).catch(() => {});
  }, []);

  useEffect(() => { loadCounts(); }, [loadCounts, location.pathname]);
  useRealtimeRefresh(loadCounts);

  return (
    <div className="flex min-h-screen flex-col bg-gray-950 text-gray-100 md:flex-row">
      <aside className="flex w-full shrink-0 flex-col border-b border-gray-800 bg-gray-900 md:w-56 md:border-b-0 md:border-r md:h-screen md:overflow-y-auto">
        <div className="border-b border-gray-800 p-4 md:p-5">
          <Link to="/" className="flex items-center gap-2 text-lg font-bold">
            <span className="text-purple-400">Shadow</span>
          </Link>
          <p className="text-xs text-gray-500 mt-1">AI Agent Memory Layer</p>
        </div>

        <nav className="flex gap-1 overflow-x-auto p-2 md:flex-1 md:flex-col md:space-y-1 md:p-3">
          {navItems.map(({ path, label, icon: Icon }) => {
            const active = location.pathname === path || (path !== '/' && location.pathname.startsWith(path));
            const badge = label === 'Review' ? candidateCount : label === 'Conflicts' ? conflictCount : 0;
            return (
              <Link
                key={path}
                to={path}
                className={`flex shrink-0 items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors ${
                  active
                    ? 'bg-purple-500/10 text-purple-400'
                    : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800'
                }`}
              >
                <Icon size={18} />
                <span className="flex-1">{label}</span>
                {badge > 0 && (
                  <Chip className={`rounded-full px-1.5 py-0.5 text-xs ${
                    label === 'Conflicts' ? 'bg-red-500/10 text-red-300' : 'bg-yellow-500/10 text-yellow-300'
                  }`}>{badge}</Chip>
                )}
              </Link>
            );
          })}
        </nav>

        <div className="hidden border-t border-gray-800 p-4 md:block">
          <ShadowCard className="border-gray-800 bg-gray-950/40 p-3">
            <div className="flex items-center gap-2 text-xs text-gray-400">
              <Zap size={14} className="text-green-400" />
              Daemon Running
            </div>
            <p className="mt-1 text-xs text-gray-600">localhost:7878 - local mode</p>
          </ShadowCard>
        </div>
      </aside>

      <main className="min-h-[calc(100vh-132px)] flex-1 overflow-auto md:h-screen md:min-h-screen">
        {children}
      </main>
    </div>
  );
}
