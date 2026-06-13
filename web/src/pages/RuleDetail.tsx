import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api, type Event, type Rule, type Source, type Version } from '../lib/api';
import { ArrowLeft, Edit3, RotateCcw, Save, X } from 'lucide-react';
import { Card, TextArea } from '@heroui/react';
import { LoadingState, ShadowButton, ShadowCard, StatusChip, TagChip } from '../components/ui';

export default function RuleDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [rule, setRule] = useState<Rule | null>(null);
  const [sources, setSources] = useState<Source[]>([]);
  const [events, setEvents] = useState<Event[]>([]);
  const [versions, setVersions] = useState<Version[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);
  const [editContent, setEditContent] = useState('');
  const [showVersions, setShowVersions] = useState(false);

  useEffect(() => {
    if (!id) return;
    Promise.all([
      api.getRule(id),
      api.getTimeline(id).catch(() => []),
      api.getEvents(id).catch(() => []),
      api.getVersions(id).catch(() => []),
    ]).then(([r, s, e, v]) => {
      setRule(r);
      setSources(s || []);
      setEvents(e || []);
      setVersions(v || []);
      setEditContent(r.content);
    }).finally(() => setLoading(false));
  }, [id]);

  const saveEdit = async () => {
    if (!rule || !editContent.trim()) return;
    const updated = await api.updateRule(rule.id, { content: editContent });
    setRule(updated);
    setEditing(false);
  };

  const toggleStatus = async (newStatus: string) => {
    if (!rule) return;
    const updated = await api.updateRule(rule.id, { status: newStatus as Rule['status'] });
    setRule(updated);
  };

  const deleteRule = async () => {
    if (!rule || !confirm('Delete this rule? It will be removed from all agent context files.')) return;
    await api.deleteRule(rule.id);
    navigate('/rules');
  };

  const rollback = async (version: number) => {
    if (!rule) return;
    await api.rollback(rule.id, version);
    const updated = await api.getRule(rule.id);
    setRule(updated);
    const newVersions = await api.getVersions(rule.id).catch(() => []);
    setVersions(newVersions || []);
  };

  if (loading) {
    return <div className="p-8"><LoadingState label="Loading rule..." /></div>;
  }

  if (!rule) {
    return <div className="p-8 text-center text-gray-500">Rule not found.</div>;
  }

  return (
    <div className="p-8 max-w-4xl">
      <ShadowButton onClick={() => navigate('/rules')} tone="subtle" className="mb-6 gap-2">
        <ArrowLeft size={16} /> Back to Rules
      </ShadowButton>

      {/* Rule Header */}
      <ShadowCard className="mb-6 p-6">
        <div className="flex items-start justify-between mb-4">
          <div className="flex items-center gap-3">
            <StatusChip status={rule.status} />
            <TagChip>{rule.scope}</TagChip>
            {rule.category && <TagChip>{rule.category}</TagChip>}
          </div>
          <div className="flex items-center gap-2">
            {rule.status !== 'active' && (
              <ShadowButton onClick={() => toggleStatus('active')} tone="success" className="h-8 min-h-8 text-xs">
                Activate
              </ShadowButton>
            )}
            {rule.status === 'active' && (
              <ShadowButton onClick={() => toggleStatus('disabled')} tone="neutral" className="h-8 min-h-8 text-xs">
                Disable
              </ShadowButton>
            )}
            <ShadowButton onClick={deleteRule} tone="danger" className="h-8 min-h-8 text-xs">
              Delete
            </ShadowButton>
          </div>
        </div>

        {editing ? (
          <div className="mb-4">
            <TextArea
              value={editContent}
              onChange={e => setEditContent(e.target.value)}
              className="w-full min-h-[100px] rounded-lg border border-gray-700 bg-gray-950 p-4 font-mono text-sm text-gray-100"
              autoFocus
            />
            <div className="flex items-center gap-2 mt-2">
              <ShadowButton onClick={saveEdit} tone="primary" className="gap-1">
                <Save size={14} /> Save
              </ShadowButton>
              <ShadowButton onClick={() => { setEditing(false); setEditContent(rule.content); }} tone="neutral" className="gap-1">
                <X size={14} /> Cancel
              </ShadowButton>
            </div>
          </div>
        ) : (
          <p className="text-lg leading-relaxed mb-4">{rule.content}</p>
        )}

        {!editing && (
          <ShadowButton onClick={() => setEditing(true)} tone="subtle" className="mb-4 h-8 min-h-8 gap-1 text-xs">
            <Edit3 size={12} /> Edit
          </ShadowButton>
        )}

        {/* Metadata Grid */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
          <div>
            <span className="text-gray-500 block">Confidence</span>
            <div className="flex items-center gap-2 mt-1">
              <div className="flex-1 h-2 bg-gray-800 rounded-full overflow-hidden">
                <div className="h-full bg-purple-500 rounded-full" style={{ width: `${rule.confidence * 100}%` }} />
              </div>
              <span>{(rule.confidence * 100).toFixed(0)}%</span>
            </div>
          </div>
          <div>
            <span className="text-gray-500 block">Version</span>
            <span className="mt-1 block">v{rule.version}</span>
          </div>
          <div>
            <span className="text-gray-500 block">Scope</span>
            <span className="mt-1 block capitalize">{rule.scope}</span>
          </div>
          <div>
            <span className="text-gray-500 block">Tags</span>
            <div className="flex flex-wrap gap-1 mt-1">
              {rule.tags?.map(tag => (
                <TagChip key={tag}>{tag}</TagChip>
              ))}
            </div>
          </div>
        </div>

        {rule.trigger_context && (
          <div className="mt-4 pt-4 border-t border-gray-800">
            <span className="text-gray-500 text-sm">Trigger Context:</span>
            <p className="text-sm mt-1 text-gray-300">{rule.trigger_context}</p>
          </div>
        )}
      </ShadowCard>

      {/* Effectiveness Events */}
      <ShadowCard className="mb-6 p-6">
        <h2 className="text-lg font-semibold mb-4">Effectiveness</h2>
        {events.length === 0 ? (
          <p className="text-sm text-gray-500">No hit or sync events recorded for this rule yet.</p>
        ) : (
          <div className="space-y-3">
            {events.map(event => (
              <div key={event.id} className="rounded-lg border border-gray-800 bg-gray-950 p-3 text-sm">
                <div className="mb-1 flex flex-wrap items-center gap-2">
                  <TagChip className={event.event_type === 'rule_hit' ? 'text-green-300' : 'text-purple-300'}>
                    {event.event_type}
                  </TagChip>
                  {event.agent_name && <span className="text-xs text-gray-500">Agent: {event.agent_name}</span>}
                  <span className="text-xs text-gray-500">{new Date(event.timestamp).toLocaleString()}</span>
                </div>
                {event.target_path && <p className="font-mono text-xs text-gray-400">{event.target_path}</p>}
                {event.details && <p className="mt-1 text-xs text-gray-500">{event.details}</p>}
              </div>
            ))}
          </div>
        )}
      </ShadowCard>

      {/* Source Timeline */}
      <ShadowCard className="mb-6 p-6">
        <h2 className="text-lg font-semibold mb-4">Source Timeline</h2>
        {sources.length === 0 ? (
          <p className="text-sm text-gray-500">No source traceability data available.</p>
        ) : (
          <div className="space-y-4">
            {sources.map((source, i) => (
              <div key={source.id} className="flex gap-4">
                <div className="flex flex-col items-center">
                  <div className={`w-3 h-3 rounded-full ${i === 0 ? 'bg-purple-500' : 'bg-gray-600'}`} />
                  {i < sources.length - 1 && <div className="w-0.5 flex-1 bg-gray-800" />}
                </div>
                <div className="pb-4 flex-1">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-xs text-gray-500">{new Date(source.timestamp).toLocaleDateString()}</span>
                    <TagChip>{source.signal_type}</TagChip>
                    <TagChip className={`${
                      source.signal_strength === 'strong' ? 'bg-green-500/10 text-green-400' :
                      source.signal_strength === 'medium' ? 'bg-yellow-500/10 text-yellow-400' :
                      'bg-gray-500/10 text-gray-400'
                    }`}>{source.signal_strength}</TagChip>
                  </div>
                  {source.raw_snippet && (
                    <p className="text-sm text-gray-300 font-mono bg-gray-950 rounded p-2 mt-1">
                      {source.raw_snippet.length > 200 ? source.raw_snippet.slice(0, 200) + '...' : source.raw_snippet}
                    </p>
                  )}
                  <div className="flex items-center gap-3 mt-1 text-xs text-gray-500">
                    {source.agent_name && <span>Agent: {source.agent_name}</span>}
                    {source.project_path && <span>Project: {source.project_path}</span>}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </ShadowCard>

      {/* Version History */}
      <ShadowCard className="p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Version History</h2>
          <ShadowButton onClick={() => setShowVersions(!showVersions)} tone="subtle" className="h-8 min-h-8 text-xs text-purple-300">
            {showVersions ? 'Hide' : `Show ${versions.length} versions`}
          </ShadowButton>
        </div>
        {showVersions && versions.length > 0 ? (
          <div className="space-y-3">
            {versions.sort((a, b) => b.version - a.version).map(v => (
              <Card key={v.id} className="flex items-start justify-between rounded-lg bg-gray-950 p-4">
                <div className="flex-1">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-xs font-mono text-purple-400">v{v.version}</span>
                    <TagChip className={v.changed_by === 'user' ? 'text-blue-300' : 'text-gray-400'}>
                      {v.changed_by}
                    </TagChip>
                    <span className="text-xs text-gray-500">{new Date(v.timestamp).toLocaleDateString()}</span>
                  </div>
                  <p className="text-sm text-gray-300">{v.content}</p>
                  {v.change_reason && <p className="text-xs text-gray-500 mt-1">Reason: {v.change_reason}</p>}
                </div>
                {v.version < (versions.sort((a, b) => b.version - a.version)[0]?.version || 0) && (
                  <ShadowButton onClick={() => rollback(v.version)} tone="subtle" className="ml-4 h-8 min-h-8 shrink-0 gap-1 text-xs hover:text-purple-300">
                    <RotateCcw size={12} /> Rollback
                  </ShadowButton>
                )}
              </Card>
            ))}
          </div>
        ) : showVersions ? (
          <p className="text-sm text-gray-500">No version history available.</p>
        ) : null}
      </ShadowCard>
    </div>
  );
}
