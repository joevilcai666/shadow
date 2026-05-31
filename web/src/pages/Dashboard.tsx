import { useEffect, useState } from 'react';
import { api, type DashboardData } from '../lib/api';
import { FileText, CheckCircle, Clock, Zap } from 'lucide-react';

export default function Dashboard() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getDashboard()
      .then(setData)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-spin text-purple-400">⠋</div>
      </div>
    );
  }

  const stats = [
    { label: 'Total Rules', value: data?.total_rules ?? 0, icon: FileText, color: 'text-blue-400' },
    { label: 'Active', value: data?.active_rules ?? 0, icon: CheckCircle, color: 'text-green-400' },
    { label: 'Candidates', value: data?.candidate_rules ?? 0, icon: Clock, color: 'text-yellow-400' },
    { label: 'Agents', value: 2, icon: Zap, color: 'text-purple-400' },
  ];

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold mb-6">Dashboard</h1>

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
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <h2 className="text-lg font-semibold mb-4">Connected Agents</h2>
          <div className="space-y-3">
            {['Claude Code', 'Cursor'].map(agent => (
              <div key={agent} className="flex items-center justify-between py-2">
                <div className="flex items-center gap-3">
                  <div className="w-2 h-2 rounded-full bg-green-500" />
                  <span>{agent}</span>
                </div>
                <span className="text-xs text-gray-500 bg-gray-800 px-2 py-1 rounded">Connected</span>
              </div>
            ))}
          </div>
        </div>

        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <h2 className="text-lg font-semibold mb-4">Trust & Privacy</h2>
          <div className="space-y-3">
            {[
              { label: 'Data Location', value: 'Local only' },
              { label: 'Keys Stored', value: 'None' },
              { label: 'Last Sync', value: 'N/A (local mode)' },
              { label: 'Rules with Sensitive Data', value: '0 (blocked)' },
            ].map(({ label, value }) => (
              <div key={label} className="flex items-center justify-between py-1">
                <span className="text-sm text-gray-400">{label}</span>
                <span className="text-sm">{value}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
