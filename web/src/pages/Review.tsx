import { useCallback, useEffect, useState } from 'react';
import { api, type Rule } from '../lib/api';
import { CheckCircle, XCircle, Eye, Globe2 } from 'lucide-react';
import { Checkbox } from '@heroui/react';
import { LoadingState, ShadowButton, ShadowCard, TagChip } from '../components/ui';
import { useRealtimeRefresh } from '../lib/realtime';

export default function Review() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [processing, setProcessing] = useState(false);

  const loadRules = useCallback(() => {
    api.listRules({ status: 'candidate' })
      .then(r => setRules(r || []))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { loadRules(); }, [loadRules]);
  useRealtimeRefresh(loadRules);

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

  const promoteToGlobal = async (rule: Rule) => {
    setProcessing(true);
    await api.updateRule(rule.id, { scope: 'global', project_path: '' });
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

  const highConfidence = rules.filter(r => r.confidence >= 0.8);
  const medConfidence = rules.filter(r => r.confidence >= 0.5 && r.confidence < 0.8);
  const lowConfidence = rules.filter(r => r.confidence < 0.5);

  const renderGroup = (title: string, subtitle: string, items: Rule[]) => {
    if (items.length === 0) return null;
    return (
      <div className="mb-6">
        <h3 className="text-sm font-semibold text-gray-400 uppercase tracking-wider mb-3">{title} ({items.length})</h3>
        <p className="text-xs text-gray-600 mb-3">{subtitle}</p>
        <div className="space-y-2">
          {items.map(rule => (
            <ShadowCard key={rule.id} className={`p-4 transition-colors ${
              selected.has(rule.id) ? 'border-purple-500/40' : 'hover:border-gray-700'
            }`}>
              <div className="flex items-start gap-3">
                <Checkbox
                  isSelected={selected.has(rule.id)}
                  onChange={() => toggleSelect(rule.id)}
                  aria-label={`Select rule ${rule.content}`}
                  className="mt-1"
                />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-xs text-gray-500">confidence: {(rule.confidence * 100).toFixed(0)}%</span>
                    {rule.category && <TagChip>{rule.category}</TagChip>}
                    <span className="text-xs text-gray-500">{rule.scope}</span>
                  </div>
                  <p className="text-sm leading-relaxed">{rule.content}</p>

                  {expanded.has(rule.id) && (
                    <div className="mt-3 pt-3 border-t border-gray-800 text-xs text-gray-500 space-y-1">
                      <p>
                        Why suggested: {rule.confidence >= 0.8
                          ? 'strong repeated signal and safe to review quickly'
                          : rule.confidence >= 0.5
                            ? 'enough signal to review, but needs human confirmation'
                            : 'weak signal; keep local unless the wording is obviously durable'}
                      </p>
                      <p>
                        Health: {rule.scope === 'project'
                          ? 'project-scoped; promote only if it should follow you across repos'
                          : 'global-scoped; will affect every connected agent context'}
                      </p>
                      {rule.trigger_context && <p>Trigger: {rule.trigger_context}</p>}
                      {rule.tags?.length > 0 && <p>Tags: {rule.tags.join(', ')}</p>}
                      <p>Created: {new Date(rule.created_at).toLocaleString()}</p>
                      <p className="text-gray-600 mt-1">
                        Will be written to: {rule.scope === 'global' ? 'CLAUDE.md, .cursorrules, AGENTS.md' : 'project-level context files'}
                      </p>
                    </div>
                  )}

                  <div className="flex items-center gap-2 mt-2">
                    <ShadowButton onClick={() => approve(rule.id)} isDisabled={processing}
                      tone="success" className="h-8 min-h-8 gap-1 text-xs">
                      <CheckCircle size={12} /> Approve
                    </ShadowButton>
                    <ShadowButton onClick={() => reject(rule.id)} isDisabled={processing}
                      tone="danger" className="h-8 min-h-8 gap-1 text-xs">
                      <XCircle size={12} /> Reject
                    </ShadowButton>
                    <ShadowButton onClick={() => toggleExpand(rule.id)} tone="subtle" className="h-8 min-h-8 gap-1 text-xs">
                      <Eye size={12} /> {expanded.has(rule.id) ? 'Less' : 'Details'}
                    </ShadowButton>
                    {rule.scope === 'project' && (
                      <ShadowButton onClick={() => promoteToGlobal(rule)} isDisabled={processing}
                        tone="subtle" className="h-8 min-h-8 gap-1 text-xs">
                        <Globe2 size={12} /> Promote Global
                      </ShadowButton>
                    )}
                  </div>
                </div>
              </div>
            </ShadowCard>
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
          <ShadowButton onClick={selectAll} tone="subtle" className="h-8 min-h-8 text-xs">
            {selected.size === rules.length ? 'Deselect All' : 'Select All'}
          </ShadowButton>
        </div>
      </div>

      {selected.size > 0 && (
        <ShadowCard className="mb-4 flex items-center gap-3 p-3">
          <span className="text-sm text-gray-400">{selected.size} selected</span>
          <ShadowButton onClick={() => batchAction('approve')} isDisabled={processing}
            tone="success" className="h-8 min-h-8 text-xs">
            Batch Approve
          </ShadowButton>
          <ShadowButton onClick={() => batchAction('reject')} isDisabled={processing}
            tone="danger" className="h-8 min-h-8 text-xs">
            Batch Reject
          </ShadowButton>
        </ShadowCard>
      )}

      {loading ? (
        <LoadingState label="Loading candidates..." />
      ) : rules.length === 0 ? (
        <div className="text-center py-16">
          <p className="text-gray-400">All caught up! New candidates will appear here.</p>
        </div>
      ) : (
        <>
          {renderGroup('High Confidence', '>= 80% - recommended to approve', highConfidence)}
          {renderGroup('Medium Confidence', '50-79% - review recommended', medConfidence)}
          {renderGroup('Low Confidence', '< 50% - needs careful review', lowConfidence)}
        </>
      )}
    </div>
  );
}
