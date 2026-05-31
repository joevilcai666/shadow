import { useEffect, useState } from 'react';
import { api, type Rule } from '../lib/api';
import { Search, Trash2, CheckCircle, XCircle, Clock, AlertTriangle } from 'lucide-react';

const statusColors: Record<string, string> = {
  active: 'bg-green-500/10 text-green-400',
  candidate: 'bg-yellow-500/10 text-yellow-400',
  disabled: 'bg-gray-500/10 text-gray-400',
  conflicted: 'bg-red-500/10 text-red-400',
};

const statusIcons: Record<string, typeof CheckCircle> = {
  active: CheckCircle,
  candidate: Clock,
  disabled: XCircle,
  conflicted: AlertTriangle,
};

export default function Rules() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [filterStatus, setFilterStatus] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const loadRules = () => {
    const params: Record<string, string> = {};
    if (search) params.q = search;
    if (filterStatus) params.status = filterStatus;

    api.listRules(params)
      .then(setRules)
      .catch(console.error)
      .finally(() => setLoading(false));
  };

  useEffect(() => { loadRules(); }, [search, filterStatus]);

  const toggleSelect = (id: string) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id); else next.add(id);
    setSelected(next);
  };

  const batchAction = async (action: string) => {
    if (selected.size === 0) return;
    await api.batchRules(action, Array.from(selected));
    setSelected(new Set());
    loadRules();
  };

  const deleteRule = async (id: string) => {
    await api.deleteRule(id);
    loadRules();
  };

  const toggleStatus = async (rule: Rule) => {
    const newStatus = rule.status === 'active' ? 'disabled' : 'active';
    await api.updateRule(rule.id, { ...rule, status: newStatus });
    loadRules();
  };

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Rules</h1>
        <span className="text-sm text-gray-500">{rules.length} rules</span>
      </div>

      {/* Search & Filter Bar */}
      <div className="flex items-center gap-3 mb-6">
        <div className="relative flex-1 max-w-md">
          <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" />
          <input
            type="text"
            placeholder="Search rules..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="w-full bg-gray-900 border border-gray-800 rounded-lg pl-10 pr-4 py-2 text-sm focus:outline-none focus:border-purple-500"
          />
        </div>

        <div className="flex items-center gap-1 bg-gray-900 border border-gray-800 rounded-lg p-1">
          {['', 'active', 'candidate', 'disabled'].map(status => (
            <button
              key={status}
              onClick={() => setFilterStatus(status)}
              className={`px-3 py-1.5 rounded text-xs transition-colors ${
                filterStatus === status
                  ? 'bg-purple-500/20 text-purple-400'
                  : 'text-gray-400 hover:text-gray-200'
              }`}
            >
              {status || 'All'}
            </button>
          ))}
        </div>
      </div>

      {/* Batch Actions */}
      {selected.size > 0 && (
        <div className="flex items-center gap-3 mb-4 p-3 bg-gray-900 rounded-lg border border-gray-800">
          <span className="text-sm text-gray-400">{selected.size} selected</span>
          <button onClick={() => batchAction('activate')} className="px-3 py-1 text-xs bg-green-500/10 text-green-400 rounded hover:bg-green-500/20">
            Activate
          </button>
          <button onClick={() => batchAction('disable')} className="px-3 py-1 text-xs bg-gray-500/10 text-gray-400 rounded hover:bg-gray-500/20">
            Disable
          </button>
          <button onClick={() => batchAction('delete')} className="px-3 py-1 text-xs bg-red-500/10 text-red-400 rounded hover:bg-red-500/20">
            Delete
          </button>
        </div>
      )}

      {/* Rules List */}
      {loading ? (
        <div className="text-center py-12 text-gray-500">Loading rules...</div>
      ) : rules.length === 0 ? (
        <div className="text-center py-12 text-gray-500">
          No rules yet. Start coding with your agents and Shadow will capture your corrections.
        </div>
      ) : (
        <div className="space-y-2">
          {rules.map(rule => {
            const StatusIcon = statusIcons[rule.status] || Clock;
            return (
              <div key={rule.id} className="bg-gray-900 border border-gray-800 rounded-lg p-4 hover:border-gray-700 transition-colors">
                <div className="flex items-start gap-3">
                  <input
                    type="checkbox"
                    checked={selected.has(rule.id)}
                    onChange={() => toggleSelect(rule.id)}
                    className="mt-1 accent-purple-500"
                  />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs ${statusColors[rule.status]}`}>
                        <StatusIcon size={12} />
                        {rule.status}
                      </span>
                      <span className="text-xs text-gray-500">{rule.scope}</span>
                      {rule.category && (
                        <span className="text-xs bg-gray-800 text-gray-400 px-2 py-0.5 rounded">{rule.category}</span>
                      )}
                    </div>
                    <p className="text-sm leading-relaxed">{rule.content}</p>
                    <div className="flex items-center gap-3 mt-2 text-xs text-gray-500">
                      <span>v{rule.version}</span>
                      <span>confidence: {(rule.confidence * 100).toFixed(0)}%</span>
                      {rule.tags?.map(tag => (
                        <span key={tag} className="bg-gray-800 px-1.5 py-0.5 rounded">{tag}</span>
                      ))}
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => toggleStatus(rule)}
                      className="p-1.5 text-gray-500 hover:text-gray-300 rounded"
                      title={rule.status === 'active' ? 'Disable' : 'Activate'}
                    >
                      {rule.status === 'active' ? <XCircle size={16} /> : <CheckCircle size={16} />}
                    </button>
                    <button
                      onClick={() => deleteRule(rule.id)}
                      className="p-1.5 text-gray-500 hover:text-red-400 rounded"
                      title="Delete"
                    >
                      <Trash2 size={16} />
                    </button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
