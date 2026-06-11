import { useCallback, useEffect, useState } from 'react';
import { api, type Rule } from '../lib/api';
import { Search, Trash2, CheckCircle, XCircle } from 'lucide-react';
import { Checkbox, Input } from '@heroui/react';
import { IconButton, LoadingState, ShadowButton, ShadowCard, StatusChip, TagChip } from '../components/ui';
import { useRealtimeRefresh } from '../lib/realtime';

export default function Rules() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [filterStatus, setFilterStatus] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const loadRules = useCallback(() => {
    const params: Record<string, string> = {};
    if (search) params.q = search;
    if (filterStatus) params.status = filterStatus;

    api.listRules(params)
      .then(setRules)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [filterStatus, search]);

  useEffect(() => { loadRules(); }, [loadRules]);
  useRealtimeRefresh(loadRules);

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
    await api.updateRule(rule.id, { status: newStatus });
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
          <Input
            type="text"
            placeholder="Search rules..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="w-full rounded-lg border border-gray-800 bg-gray-900 py-2 pl-10 pr-4 text-sm text-gray-100"
          />
        </div>

        <div className="flex items-center gap-1 rounded-lg border border-gray-800 bg-gray-900 p-1">
          {['', 'active', 'candidate', 'disabled'].map(status => (
            <ShadowButton
              key={status}
              onClick={() => setFilterStatus(status)}
              tone={filterStatus === status ? 'primary' : 'subtle'}
              className={`h-8 min-h-8 px-3 text-xs ${
                filterStatus === status
                  ? 'bg-purple-500/20 text-purple-300 hover:bg-purple-500/25'
                  : ''
              }`}
            >
              {status || 'All'}
            </ShadowButton>
          ))}
        </div>
      </div>

      {/* Batch Actions */}
      {selected.size > 0 && (
        <ShadowCard className="mb-4 flex items-center gap-3 p-3">
          <span className="text-sm text-gray-400">{selected.size} selected</span>
          <ShadowButton onClick={() => batchAction('activate')} tone="success" className="h-8 min-h-8 text-xs">
            Activate
          </ShadowButton>
          <ShadowButton onClick={() => batchAction('disable')} tone="neutral" className="h-8 min-h-8 text-xs">
            Disable
          </ShadowButton>
          <ShadowButton onClick={() => batchAction('delete')} tone="danger" className="h-8 min-h-8 text-xs">
            Delete
          </ShadowButton>
        </ShadowCard>
      )}

      {/* Rules List */}
      {loading ? (
        <LoadingState label="Loading rules..." />
      ) : rules.length === 0 ? (
        <div className="text-center py-12 text-gray-500">
          No rules yet. Start coding with your agents and Shadow will capture your corrections.
        </div>
      ) : (
        <div className="space-y-2">
          {rules.map(rule => {
            return (
              <ShadowCard key={rule.id} className="p-4 transition-colors hover:border-gray-700">
                <div className="flex items-start gap-3">
                  <Checkbox
                    isSelected={selected.has(rule.id)}
                    onChange={() => toggleSelect(rule.id)}
                    aria-label={`Select rule ${rule.content}`}
                    className="mt-1"
                  />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <StatusChip status={rule.status} />
                      <span className="text-xs text-gray-500">{rule.scope}</span>
                      {rule.category && (
                        <TagChip>{rule.category}</TagChip>
                      )}
                    </div>
                    <p className="text-sm leading-relaxed">{rule.content}</p>
                    <div className="flex items-center gap-3 mt-2 text-xs text-gray-500">
                      <span>v{rule.version}</span>
                      <span>confidence: {(rule.confidence * 100).toFixed(0)}%</span>
                      {rule.tags?.map(tag => (
                        <TagChip key={tag}>{tag}</TagChip>
                      ))}
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    <IconButton
                      onClick={() => toggleStatus(rule)}
                      tone="subtle"
                      label={rule.status === 'active' ? 'Disable' : 'Activate'}
                    >
                      {rule.status === 'active' ? <XCircle size={16} /> : <CheckCircle size={16} />}
                    </IconButton>
                    <IconButton
                      onClick={() => deleteRule(rule.id)}
                      tone="subtle"
                      label="Delete"
                      className="hover:text-red-300"
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </div>
                </div>
              </ShadowCard>
            );
          })}
        </div>
      )}
    </div>
  );
}
