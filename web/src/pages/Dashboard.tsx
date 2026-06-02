import { useEffect, useState } from 'react';
import { api, type DashboardData, type Adapter } from '../lib/api';
import { FileText, CheckCircle, Clock, AlertTriangle, Activity, Shield, RefreshCw } from 'lucide-react';

export default function Dashboard() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [adapters, setAdapters] = useState<Adapter[]>([]);
  const [loading, setLoading] = useState(true);

  const loadData = () => {
    setLoading(true);
    Promise.all([
      api.getDashboard().catch(() => null),
      api.listAdapters().catch(() => []),
    ]).then(([d, a]) => {
      setData(d);
      setAdapters(a || []);
    }).finally(() => setLoading(false));
  };

  useEffect(() => { loadData(); }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-spin text-purple-400">⠋</div>
      </div>
    );
  }

  const total = data?.total_rules ?? 0;
  const active = data?.active_rules ?? 0;
  const candidate = data?.candidate_rules ?? 0;
  const conflicted = data?.conflicted_rules ?? 0;
  const disabled = data?.disabled_rules ?? 0;
  const totalSources = data?.total_sources ?? 0;
  const projectCount = data?.project_count ?? 0;

  const stats = [
    { label: 'Total Rules', value: total, icon: FileText, color: 'text-blue-400' },
    { label: 'Active', value: active, icon: CheckCircle, color: 'text-green-400' },
    { label: 'Candidates', value: candidate, icon: Clock, color: 'text-yellow-400' },
    { label: 'Conflicts', value: conflicted, icon: AlertTriangle, color: 'text-red-400' },
  ];

  const installedAdapters = adapters.filter(a => a.installed);

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <button onClick={loadData} className="flex items-center gap-1 text-xs text-gray-500 hover:text-gray-300">
          <RefreshCw size={14} /> Refresh
        </button>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {stats.map(({ label, value, icon: Icon, color }) => (
          <div key={label} className="bg-gray-900 rounded-xl p-5 border border-gray-800">
            <div className="flex items-center justify-between mb-3">
              <span className="text-sm text-gray-400">{label}</span>
              <Icon size={20} className={color} />
            </div>
            <div className="text-3xl font-bold">{value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Connected Agents */}
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold">Connected Agents</h2>
            <span className="text-xs text-gray-500">{installedAdapters.length} connected</span>
          </div>
          <div className="space-y-3">
            {installedAdapters.length > 0 ? installedAdapters.map(adapter => (
              <div key={adapter.name} className="flex items-center justify-between py-2">
                <div className="flex items-center gap-3">
                  <div className={`w-2 h-2 rounded-full ${adapter.enabled ? 'bg-green-500' : 'bg-gray-600'}`} />
                  <div>
                    <span className="text-sm">{adapter.label}</span>
                    <p className="text-xs text-gray-600 mt-0.5">{adapter.target_path}</p>
                  </div>
                </div>
                <span className={`text-xs px-2 py-1 rounded ${
                  adapter.enabled ? 'bg-green-500/10 text-green-400' : 'bg-gray-800 text-gray-500'
                }`}>
                  {adapter.enabled ? 'Active' : 'Paused'}
                </span>
              </div>
            )) : (
              <p className="text-sm text-gray-500">No agents detected. Run <code className="bg-gray-800 px-1 py-0.5 rounded text-xs">shadow start</code> to detect and connect agents.</p>
            )}
          </div>
        </div>

        {/* System Overview */}
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <h2 className="text-lg font-semibold mb-4">System Overview</h2>
          <div className="space-y-3">
            <div className="flex items-center justify-between py-2 border-b border-gray-800">
              <span className="text-sm text-gray-400 flex items-center gap-2"><Activity size={14} /> Sources Captured</span>
              <span className="text-sm font-medium">{totalSources}</span>
            </div>
            <div className="flex items-center justify-between py-2 border-b border-gray-800">
              <span className="text-sm text-gray-400 flex items-center gap-2"><FileText size={14} /> Disabled Rules</span>
              <span className="text-sm font-medium">{disabled}</span>
            </div>
            <div className="flex items-center justify-between py-2 border-b border-gray-800">
              <span className="text-sm text-gray-400">Projects</span>
              <span className="text-sm font-medium">{projectCount}</span>
            </div>

            {/* Agent stats */}
            {data?.agent_stats && Object.keys(data.agent_stats).length > 0 && (
              <div className="pt-2">
                <span className="text-sm text-gray-400">Signals by Agent</span>
                <div className="mt-2 space-y-2">
                  {Object.entries(data.agent_stats).map(([agent, count]) => (
                    <div key={agent} className="flex items-center justify-between text-sm">
                      <span className="text-gray-300">{agent}</span>
                      <div className="flex items-center gap-2">
                        <div className="w-20 h-2 bg-gray-800 rounded-full overflow-hidden">
                          <div className="h-full bg-purple-500 rounded-full" style={{ width: `${Math.min(100, (count / Math.max(totalSources, 1)) * 100)}%` }} />
                        </div>
                        <span className="text-gray-500 text-xs">{count}</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Trust & Privacy */}
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <Shield size={18} className="text-purple-400" /> Trust & Privacy
          </h2>
          <div className="space-y-3">
            {[
              { label: 'Data Location', value: 'Local only (~/.shadow/)', good: true },
              { label: 'Keys Stored', value: 'None (blocked)', good: true },
              { label: 'Raw Conversations', value: 'Never stored', good: true },
              { label: 'Only Distilled Rules', value: 'Yes', good: true },
              { label: 'Cloud Sync', value: 'Disabled (local mode)', good: true },
            ].map(({ label, value, good }) => (
              <div key={label} className="flex items-center justify-between py-1">
                <span className="text-sm text-gray-400">{label}</span>
                <span className={`text-sm ${good ? 'text-green-400' : 'text-yellow-400'}`}>{value}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Quick Actions */}
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <h2 className="text-lg font-semibold mb-4">Quick Actions</h2>
          <div className="space-y-2">
            {candidate > 0 && (
              <a href="/review" className="block p-3 rounded-lg bg-yellow-500/5 border border-yellow-500/20 hover:bg-yellow-500/10 transition-colors">
                <span className="text-sm text-yellow-400">{candidate} candidate rules awaiting review</span>
                <p className="text-xs text-gray-500 mt-1">Click to review and activate</p>
              </a>
            )}
            {conflicted > 0 && (
              <a href="/conflicts" className="block p-3 rounded-lg bg-red-500/5 border border-red-500/20 hover:bg-red-500/10 transition-colors">
                <span className="text-sm text-red-400">{conflicted} conflicting rules need resolution</span>
                <p className="text-xs text-gray-500 mt-1">Click to resolve conflicts</p>
              </a>
            )}
            <button
              onClick={() => api.syncAdapters().then(() => alert('Adapter sync triggered'))}
              className="w-full text-left p-3 rounded-lg bg-gray-800 hover:bg-gray-750 transition-colors"
            >
              <span className="text-sm text-gray-300">Sync rules to all agents</span>
              <p className="text-xs text-gray-500 mt-1">Force write active rules to agent context files</p>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
