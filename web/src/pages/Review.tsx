import { useEffect, useState } from 'react';
import { api, type Rule } from '../lib/api';
import { CheckCircle, XCircle, Eye } from 'lucide-react';

export default function Review() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [processing, setProcessing] = useState(false);

  const loadRules = () => {
    api.listRules({ status: 'candidate' })
      .then(r => setRules(r || []))
      .finally(() => setLoading(false));
  };

  useEffect(() => { loadRules(); }, []);

  const toggleSelect = (id: string) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id); else next.add(id);
    setSelected(next);
  };

  const selectAll = () => {
    if (selected.size === rules.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(rules.map(r => r.id)));
    }
  };

  const toggleExpand = (id: string) => {
    const next = new Set(expanded);
    if (next.has(id)) next.delete(id); else next.add(id);
    setExpanded(next);
  };

  const approve = async (id: string) => {
    setProcessing(true);
    await api.updateRule(id, { status: 'active' } as Partial<Rule>);
    loadRules();
    setProcessing(false);
  };

  const reject = async (id: string) => {
    setProcessing(true);
    await api.updateRule(id, { status: 'disabled' } as Partial<Rule>);
    loadRules();
    setProcessing(false);
  };

  const batchAction = async (action: string) => {
    if (selected.size === 0) return;
    setProcessing(true);
    await api.batchRules(action === 'approve' ? 'activate' : 'disable', Array.from(selected));
    setSelected(new Set());
    loadRules();
    setProcessing(false);
  };

  // Group by confidence
  const highConfidence = rules.filter(r => r.confidence >= 0.8);
  const medConfidence = rules.filter(r => r.confidence >= 0.5 && r.confidence < 0.8);
  const lowConfidence = rules.filter(r => r.confidence < 0.5);

  const Group = ({ title, subtitle, items }: { title: string; subtitle: string; items: Rule[] }) => {
    if (items.length === 0) return null;
    return (
      <div className="mb-6">
        <h3 className="text-sm font-semibold text-gray-400 uppercase tracking-wider mb-3">{title} ({items.length})</h3>
        <p className="text-xs text-gray-600 mb-3">{subtitle}</p>
        <div className="space-y-2">
          {items.map(rule => (
            <div key={rule.id} className={`bg-gray-900 border rounded-lg p-4 transition-colors ${
              selected.has(rule.id) ? 'border-purple-500/30' : 'border-gray-800 hover:border-gray-700'
            }`}>
              <div className="flex items-start gap-3">
                <input
                  type="checkbox"
                  checked={selected.has(rule.id)}
                  onChange={() => toggleSelect(rule.id)}
                  className="mt-1 accent-purple-500"
                />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-xs text-gray-500">confidence: {(rule.confidence * 100).toFixed(0)}%</span>
                    {rule.category && <span className="text-xs bg-gray-800 text-gray-400 px-2 py-0.5 rounded">{rule.category}</span>}
                    <span className="text-xs text-gray-500">{rule.scope}</span>
                  </div>
                  <p className="text-sm leading-relaxed">{rule.content}</p>

                  {expanded.has(rule.id) && (
                    <div className="mt-3 pt-3 border-t border-gray-800 text-xs text-gray-500 space-y-1">
                      {rule.trigger_context && <p>Trigger: {rule.trigger_context}</p>}
                      {rule.tags?.length > 0 && <p>Tags: {rule.tags.join(', ')}</p>}
                      <p>Created: {new Date(rule.created_at).toLocaleString()}</p>
                      <p className="text-gray-600 mt-1">
                        Will be written to: {rule.scope === 'global' ? 'CLAUDE.md, .cursorrules, AGENTS.md' : 'project-level context files'}
                      </p>
                    </div>
                  )}

                  <div className="flex items-center gap-2 mt-2">
                    <button onClick={() => approve(rule.id)} disabled={processing}
                      className="flex items-center gap-1 px-2.5 py-1 text-xs bg-green-500/10 text-green-400 rounded hover:bg-green-500/20 disabled:opacity-50">
                      <CheckCircle size={12} /> Approve
                    </button>
                    <button onClick={() => reject(rule.id)} disabled={processing}
                      className="flex items-center gap-1 px-2.5 py-1 text-xs bg-red-500/10 text-red-400 rounded hover:bg-red-500/20 disabled:opacity-50">
                      <XCircle size={12} /> Reject
                    </button>
                    <button onClick={() => toggleExpand(rule.id)} className="text-xs text-gray-500 hover:text-gray-300 flex items-center gap-1">
                      <Eye size={12} /> {expanded.has(rule.id) ? 'Less' : 'Details'}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  };

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Review Queue</h1>
          <p className="text-sm text-gray-500 mt-1">{rules.length} candidate rules awaiting review</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={selectAll} className="text-xs text-gray-500 hover:text-gray-300">
            {selected.size === rules.length ? 'Deselect All' : 'Select All'}
          </button>
        </div>
      </div>

      {selected.size > 0 && (
        <div className="flex items-center gap-3 mb-4 p-3 bg-gray-900 rounded-lg border border-gray-800">
          <span className="text-sm text-gray-400">{selected.size} selected</span>
          <button onClick={() => batchAction('approve')} disabled={processing}
            className="px-3 py-1 text-xs bg-green-500/10 text-green-400 rounded hover:bg-green-500/20">
            Batch Approve
          </button>
          <button onClick={() => batchAction('reject')} disabled={processing}
            className="px-3 py-1 text-xs bg-red-500/10 text-red-400 rounded hover:bg-red-500/20">
            Batch Reject
          </button>
        </div>
      )}

      {loading ? (
        <div className="text-center py-12 text-gray-500">Loading candidates...</div>
      ) : rules.length === 0 ? (
        <div className="text-center py-16">
          <div className="text-4xl mb-4">🎉</div>
          <p className="text-gray-400">All caught up! New candidates will appear here.</p>
        </div>
      ) : (
        <>
          <Group title="High Confidence" subtitle="≥ 80% — recommended to approve" items={highConfidence} />
          <Group title="Medium Confidence" subtitle="50–80% — review recommended" items={medConfidence} />
          <Group title="Low Confidence" subtitle="< 50% — needs careful review" items={lowConfidence} />
        </>
      )}
    </div>
  );
}
