import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api, type Rule, type Source, type Version } from '../lib/api';
import { ArrowLeft, Edit3, RotateCcw, CheckCircle, XCircle, Clock, AlertTriangle, Save, X } from 'lucide-react';

const statusConfig: Record<string, { color: string; icon: typeof CheckCircle; label: string }> = {
  active: { color: 'text-green-400 bg-green-500/10', icon: CheckCircle, label: 'Active' },
  candidate: { color: 'text-yellow-400 bg-yellow-500/10', icon: Clock, label: 'Candidate' },
  disabled: { color: 'text-gray-400 bg-gray-500/10', icon: XCircle, label: 'Disabled' },
  conflicted: { color: 'text-red-400 bg-red-500/10', icon: AlertTriangle, label: 'Conflicted' },
};

export default function RuleDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [rule, setRule] = useState<Rule | null>(null);
  const [sources, setSources] = useState<Source[]>([]);
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
      api.getVersions(id).catch(() => []),
    ]).then(([r, s, v]) => {
      setRule(r);
      setSources(s || []);
      setVersions(v || []);
      setEditContent(r.content);
    }).finally(() => setLoading(false));
  }, [id]);

  const saveEdit = async () => {
    if (!rule || !editContent.trim()) return;
    const updated = await api.updateRule(rule.id, { ...rule, content: editContent });
    setRule(updated);
    setEditing(false);
  };

  const toggleStatus = async (newStatus: string) => {
    if (!rule) return;
    const updated = await api.updateRule(rule.id, { ...rule, status: newStatus as Rule['status'] });
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
    return <div className="p-8 text-center text-gray-500">Loading rule...</div>;
  }

  if (!rule) {
    return <div className="p-8 text-center text-gray-500">Rule not found.</div>;
  }

  const status = statusConfig[rule.status] || statusConfig.candidate;
  const StatusIcon = status.icon;

  return (
    <div className="p-8 max-w-4xl">
      <button onClick={() => navigate('/rules')} className="flex items-center gap-2 text-sm text-gray-500 hover:text-gray-300 mb-6">
        <ArrowLeft size={16} /> Back to Rules
      </button>

      {/* Rule Header */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 mb-6">
        <div className="flex items-start justify-between mb-4">
          <div className="flex items-center gap-3">
            <span className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-sm ${status.color}`}>
              <StatusIcon size={14} /> {status.label}
            </span>
            <span className="text-xs bg-gray-800 text-gray-400 px-2 py-1 rounded">{rule.scope}</span>
            {rule.category && <span className="text-xs bg-gray-800 text-gray-400 px-2 py-1 rounded">{rule.category}</span>}
          </div>
          <div className="flex items-center gap-2">
            {rule.status !== 'active' && (
              <button onClick={() => toggleStatus('active')} className="px-3 py-1.5 text-xs bg-green-500/10 text-green-400 rounded hover:bg-green-500/20">
                Activate
              </button>
            )}
            {rule.status === 'active' && (
              <button onClick={() => toggleStatus('disabled')} className="px-3 py-1.5 text-xs bg-gray-500/10 text-gray-400 rounded hover:bg-gray-500/20">
                Disable
              </button>
            )}
            <button onClick={deleteRule} className="px-3 py-1.5 text-xs bg-red-500/10 text-red-400 rounded hover:bg-red-500/20">
              Delete
            </button>
          </div>
        </div>

        {editing ? (
          <div className="mb-4">
            <textarea
              value={editContent}
              onChange={e => setEditContent(e.target.value)}
              className="w-full bg-gray-950 border border-gray-700 rounded-lg p-4 text-sm font-mono focus:outline-none focus:border-purple-500 min-h-[100px]"
              autoFocus
            />
            <div className="flex items-center gap-2 mt-2">
              <button onClick={saveEdit} className="flex items-center gap-1 px-3 py-1.5 bg-purple-600 text-sm rounded hover:bg-purple-700">
                <Save size={14} /> Save
              </button>
              <button onClick={() => { setEditing(false); setEditContent(rule.content); }} className="flex items-center gap-1 px-3 py-1.5 bg-gray-800 text-sm rounded hover:bg-gray-700">
                <X size={14} /> Cancel
              </button>
            </div>
          </div>
        ) : (
          <p className="text-lg leading-relaxed mb-4">{rule.content}</p>
        )}

        {!editing && (
          <button onClick={() => setEditing(true)} className="flex items-center gap-1 text-xs text-gray-500 hover:text-gray-300 mb-4">
            <Edit3 size={12} /> Edit
          </button>
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
                <span key={tag} className="text-xs bg-gray-800 px-1.5 py-0.5 rounded">{tag}</span>
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
      </div>

      {/* Source Timeline */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 mb-6">
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
                    <span className="text-xs bg-gray-800 px-2 py-0.5 rounded">{source.signal_type}</span>
                    <span className={`text-xs px-2 py-0.5 rounded ${
                      source.signal_strength === 'strong' ? 'bg-green-500/10 text-green-400' :
                      source.signal_strength === 'medium' ? 'bg-yellow-500/10 text-yellow-400' :
                      'bg-gray-500/10 text-gray-400'
                    }`}>{source.signal_strength}</span>
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
      </div>

      {/* Version History */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Version History</h2>
          <button onClick={() => setShowVersions(!showVersions)} className="text-xs text-purple-400 hover:text-purple-300">
            {showVersions ? 'Hide' : `Show ${versions.length} versions`}
          </button>
        </div>
        {showVersions && versions.length > 0 ? (
          <div className="space-y-3">
            {versions.sort((a, b) => b.version - a.version).map(v => (
              <div key={v.id} className="flex items-start justify-between bg-gray-950 rounded-lg p-4">
                <div className="flex-1">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-xs font-mono text-purple-400">v{v.version}</span>
                    <span className={`text-xs px-2 py-0.5 rounded ${v.changed_by === 'user' ? 'bg-blue-500/10 text-blue-400' : 'bg-gray-800 text-gray-400'}`}>
                      {v.changed_by}
                    </span>
                    <span className="text-xs text-gray-500">{new Date(v.timestamp).toLocaleDateString()}</span>
                  </div>
                  <p className="text-sm text-gray-300">{v.content}</p>
                  {v.change_reason && <p className="text-xs text-gray-500 mt-1">Reason: {v.change_reason}</p>}
                </div>
                {v.version < (versions.sort((a, b) => b.version - a.version)[0]?.version || 0) && (
                  <button onClick={() => rollback(v.version)} className="flex items-center gap-1 text-xs text-gray-500 hover:text-purple-400 ml-4 shrink-0">
                    <RotateCcw size={12} /> Rollback
                  </button>
                )}
              </div>
            ))}
          </div>
        ) : showVersions ? (
          <p className="text-sm text-gray-500">No version history available.</p>
        ) : null}
      </div>
    </div>
  );
}
