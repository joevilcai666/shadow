import { useEffect, useState } from 'react';
import { api, type Rule } from '../lib/api';
import { SkipForward } from 'lucide-react';

interface ConflictPair {
  ruleA: Rule;
  ruleB: Rule;
}

export default function Conflicts() {
  const [conflicts, setConflicts] = useState<ConflictPair[]>([]);
  const [loading, setLoading] = useState(true);
  const [currentIdx, setCurrentIdx] = useState(0);
  const [resolution, setResolution] = useState<string>('');
  const [processing, setProcessing] = useState(false);

  useEffect(() => {
    api.listRules({ status: 'conflicted' })
      .then(rules => {
        const conflicted = rules || [];
        // Pair up conflicts (simple: sequential pairs)
        const pairs: ConflictPair[] = [];
        for (let i = 0; i < conflicted.length; i += 2) {
          if (i + 1 < conflicted.length) {
            pairs.push({ ruleA: conflicted[i], ruleB: conflicted[i + 1] });
          }
        }
        setConflicts(pairs);
      })
      .finally(() => setLoading(false));
  }, []);

  const resolve = async () => {
    if (!resolution || currentIdx >= conflicts.length) return;
    setProcessing(true);
    const pair = conflicts[currentIdx];

    switch (resolution) {
      case 'keep_a':
        await api.updateRule(pair.ruleA.id, { ...pair.ruleA, status: 'active' });
        await api.updateRule(pair.ruleB.id, { ...pair.ruleB, status: 'disabled' });
        break;
      case 'keep_b':
        await api.updateRule(pair.ruleB.id, { ...pair.ruleB, status: 'active' });
        await api.updateRule(pair.ruleA.id, { ...pair.ruleA, status: 'disabled' });
        break;
      case 'keep_both':
        // Keep both but differentiate scope
        await api.updateRule(pair.ruleA.id, { ...pair.ruleA, status: 'active' });
        await api.updateRule(pair.ruleB.id, { ...pair.ruleB, status: 'active' });
        break;
      case 'disable_both':
        await api.updateRule(pair.ruleA.id, { ...pair.ruleA, status: 'disabled' });
        await api.updateRule(pair.ruleB.id, { ...pair.ruleB, status: 'disabled' });
        break;
    }

    setResolution('');
    setCurrentIdx(prev => prev + 1);
    setProcessing(false);
  };

  if (loading) return <div className="p-8 text-center text-gray-500">Loading conflicts...</div>;

  if (conflicts.length === 0 || currentIdx >= conflicts.length) {
    return (
      <div className="p-8">
        <h1 className="text-2xl font-bold mb-6">Conflict Resolution</h1>
        <div className="text-center py-16">
          <div className="text-4xl mb-4">✓</div>
          <p className="text-gray-400">
            {conflicts.length === 0 ? 'No conflicts found. All rules are consistent.' : 'All conflicts resolved!'}
          </p>
        </div>
      </div>
    );
  }

  const pair = conflicts[currentIdx];

  return (
    <div className="p-8 max-w-4xl">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Resolve Conflicts</h1>
        <span className="text-sm text-gray-500">Conflict {currentIdx + 1} of {conflicts.length}</span>
      </div>

      <p className="text-gray-400 mb-6">These two rules contradict each other. Choose how to resolve:</p>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-8">
        {/* Rule A */}
        <div className={`bg-gray-900 border rounded-xl p-6 ${resolution === 'keep_a' ? 'border-green-500/50' : 'border-gray-800'}`}>
          <h3 className="text-sm font-semibold text-gray-500 mb-3">Rule A</h3>
          <p className="text-sm leading-relaxed mb-4">{pair.ruleA.content}</p>
          <div className="space-y-2 text-xs text-gray-500">
            <div className="flex justify-between"><span>Scope</span><span className="text-gray-300 capitalize">{pair.ruleA.scope}</span></div>
            <div className="flex justify-between"><span>Confidence</span><span className="text-gray-300">{(pair.ruleA.confidence * 100).toFixed(0)}%</span></div>
            <div className="flex justify-between"><span>Category</span><span className="text-gray-300">{pair.ruleA.category || 'general'}</span></div>
            <div className="flex justify-between"><span>Version</span><span className="text-gray-300">v{pair.ruleA.version}</span></div>
          </div>
        </div>

        {/* Rule B */}
        <div className={`bg-gray-900 border rounded-xl p-6 ${resolution === 'keep_b' ? 'border-green-500/50' : 'border-gray-800'}`}>
          <h3 className="text-sm font-semibold text-gray-500 mb-3">Rule B</h3>
          <p className="text-sm leading-relaxed mb-4">{pair.ruleB.content}</p>
          <div className="space-y-2 text-xs text-gray-500">
            <div className="flex justify-between"><span>Scope</span><span className="text-gray-300 capitalize">{pair.ruleB.scope}</span></div>
            <div className="flex justify-between"><span>Confidence</span><span className="text-gray-300">{(pair.ruleB.confidence * 100).toFixed(0)}%</span></div>
            <div className="flex justify-between"><span>Category</span><span className="text-gray-300">{pair.ruleB.category || 'general'}</span></div>
            <div className="flex justify-between"><span>Version</span><span className="text-gray-300">v{pair.ruleB.version}</span></div>
          </div>
        </div>
      </div>

      {/* System suggestion */}
      {pair.ruleB.confidence > pair.ruleA.confidence && (
        <div className="bg-purple-500/10 border border-purple-500/30 rounded-lg p-4 mb-6">
          <p className="text-sm text-purple-300">
            💡 Suggestion: Keep Rule B — it has higher confidence ({(pair.ruleB.confidence * 100).toFixed(0)}% vs {(pair.ruleA.confidence * 100).toFixed(0)}%)
            {pair.ruleB.version > pair.ruleA.version && ' and is more recent'}
          </p>
        </div>
      )}

      {/* Resolution options */}
      <div className="space-y-2 mb-6">
        {[
          { value: 'keep_a', label: 'Keep A, disable B', desc: `Keep "${pair.ruleA.content.slice(0, 40)}..."` },
          { value: 'keep_b', label: 'Keep B, disable A', desc: `Keep "${pair.ruleB.content.slice(0, 40)}..."` },
          { value: 'keep_both', label: 'Keep both (different scopes)', desc: 'Both active, may cause issues' },
          { value: 'disable_both', label: 'Disable both', desc: 'Neither rule will apply' },
        ].map(opt => (
          <label key={opt.value} className={`flex items-start gap-3 p-3 rounded-lg cursor-pointer transition-colors ${
            resolution === opt.value ? 'bg-gray-800 border border-gray-700' : 'hover:bg-gray-900'
          }`}>
            <input type="radio" name="resolution" value={opt.value} checked={resolution === opt.value}
              onChange={() => setResolution(opt.value)} className="mt-1 accent-purple-500" />
            <div>
              <span className="text-sm">{opt.label}</span>
              <p className="text-xs text-gray-500">{opt.desc}</p>
            </div>
          </label>
        ))}
      </div>

      <div className="flex items-center justify-between">
        <button onClick={() => setCurrentIdx(prev => prev + 1)}
          className="flex items-center gap-2 text-sm text-gray-500 hover:text-gray-300">
          <SkipForward size={16} /> Skip, decide later
        </button>
        <button onClick={resolve} disabled={!resolution || processing}
          className="px-5 py-2.5 bg-purple-600 hover:bg-purple-700 rounded-lg text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed">
          Apply & Next →
        </button>
      </div>
    </div>
  );
}
