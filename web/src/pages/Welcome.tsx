import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api, type Rule } from '../lib/api';
import { CheckCircle, XCircle, ArrowRight, SkipForward } from 'lucide-react';

type Step = 'review' | 'demo' | 'done';

export default function Welcome() {
  const navigate = useNavigate();
  const [step, setStep] = useState<Step>('review');
  const [rules, setRules] = useState<Rule[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.listRules({ status: 'candidate' })
      .then(r => {
        setRules(r || []);
        // Pre-select high confidence items
        const preSelected = new Set<string>();
        (r || []).forEach(rule => {
          if (rule.confidence >= 0.7) preSelected.add(rule.id);
        });
        setSelected(preSelected);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const toggleRule = (id: string) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id); else next.add(id);
    setSelected(next);
  };

  const activateSelected = async () => {
    if (selected.size > 0) {
      await api.batchRules('activate', Array.from(selected));
    }
    // Disable unselected candidates
    const unselected = rules.filter(r => !selected.has(r.id)).map(r => r.id);
    if (unselected.length > 0) {
      await api.batchRules('disable', unselected);
    }
    setStep('demo');
  };

  const skipAll = () => {
    setStep('demo');
  };

  const finishOnboarding = () => {
    navigate('/');
  };

  // Demo data — uses real rules if available
  const demoRules = rules.slice(0, 2);
  const demoExample = demoRules.length > 0 ? demoRules[0].content : 'use pnpm instead of npm';

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-950 text-gray-100 flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin text-4xl mb-4">⠋</div>
          <p className="text-gray-500">Loading your initial memories...</p>
        </div>
      </div>
    );
  }

  const progress = step === 'review' ? 1 : step === 'demo' ? 2 : 3;

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100">
      <div className="max-w-3xl mx-auto px-6 py-8">
        {/* Header */}
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-3">
            <span className="text-2xl">👻</span>
            <span className="text-lg font-bold text-purple-400">Shadow</span>
          </div>
          <div className="flex items-center gap-4">
            <div className="flex gap-2">
              {[1, 2, 3].map(i => (
                <div key={i} className={`w-8 h-1 rounded-full ${i <= progress ? 'bg-purple-500' : 'bg-gray-800'}`} />
              ))}
            </div>
            <button onClick={finishOnboarding} className="text-xs text-gray-500 hover:text-gray-300">
              Skip to Console →
            </button>
          </div>
        </div>

        {/* Step 1: Review */}
        {step === 'review' && (
          <div>
            <h1 className="text-2xl font-bold mb-2">Review Your Initial Memories</h1>
            <p className="text-gray-400 mb-6">
              {rules.length > 0
                ? `We found ${rules.length} candidate rules from your project. Select which ones to activate.`
                : "No candidate rules found yet. Start coding and Shadow will learn automatically!"}
            </p>

            {rules.length > 0 ? (
              <>
                {/* Global vs Project grouping */}
                {['global', 'project'].map(scope => {
                  const scopedRules = rules.filter(r => r.scope === scope);
                  if (scopedRules.length === 0) return null;
                  return (
                    <div key={scope} className="mb-6">
                      <h3 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-3">
                        {scope === 'global' ? '🌐 Global Rules' : '📁 Project Rules'} ({scopedRules.length})
                      </h3>
                      <div className="space-y-2">
                        {scopedRules.map(rule => (
                          <div
                            key={rule.id}
                            className={`bg-gray-900 border rounded-lg p-4 cursor-pointer transition-colors ${
                              selected.has(rule.id) ? 'border-purple-500/50 bg-purple-500/5' : 'border-gray-800 hover:border-gray-700'
                            }`}
                            onClick={() => toggleRule(rule.id)}
                          >
                            <div className="flex items-start gap-3">
                              {selected.has(rule.id) ? (
                                <CheckCircle size={18} className="text-purple-400 mt-0.5 shrink-0" />
                              ) : (
                                <XCircle size={18} className="text-gray-600 mt-0.5 shrink-0" />
                              )}
                              <div className="flex-1 min-w-0">
                                <p className="text-sm leading-relaxed">{rule.content}</p>
                                <div className="flex items-center gap-3 mt-2 text-xs text-gray-500">
                                  <span className="capitalize">{rule.category || 'general'}</span>
                                  <span>confidence: {(rule.confidence * 100).toFixed(0)}%</span>
                                  {rule.tags?.filter(t => !t.startsWith('import:') && t !== 'auto-generated').map(tag => (
                                    <span key={tag} className="bg-gray-800 px-1.5 py-0.5 rounded">{tag}</span>
                                  ))}
                                </div>
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  );
                })}

                <div className="flex items-center justify-between pt-4">
                  <button onClick={skipAll} className="flex items-center gap-2 text-sm text-gray-500 hover:text-gray-300">
                    <SkipForward size={16} /> Skip review
                  </button>
                  <button
                    onClick={activateSelected}
                    className="flex items-center gap-2 px-5 py-2.5 bg-purple-600 hover:bg-purple-700 rounded-lg text-sm font-medium transition-colors"
                  >
                    Activate Selected ({selected.size}) <ArrowRight size={16} />
                  </button>
                </div>
              </>
            ) : (
              <div className="text-center py-16">
                <div className="text-6xl mb-4">🌱</div>
                <p className="text-gray-400 mb-2">No initial memories yet</p>
                <p className="text-sm text-gray-500">Start coding with your agents and Shadow will capture your corrections automatically.</p>
                <button onClick={finishOnboarding} className="mt-6 px-5 py-2.5 bg-purple-600 hover:bg-purple-700 rounded-lg text-sm font-medium">
                  Enter Console →
                </button>
              </div>
            )}
          </div>
        )}

        {/* Step 2: Aha Demo */}
        {step === 'demo' && (
          <div>
            <h1 className="text-2xl font-bold mb-2">See the Difference</h1>
            <p className="text-gray-400 mb-8">Here's how Shadow helps your agents get it right the first time.</p>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-8">
              {/* Without Shadow */}
              <div className="bg-gray-900 border border-red-500/30 rounded-xl p-6">
                <div className="flex items-center gap-2 mb-4">
                  <XCircle size={18} className="text-red-400" />
                  <h3 className="font-semibold text-red-400">Without Shadow</h3>
                </div>
                <div className="bg-gray-950 rounded-lg p-4 font-mono text-sm">
                  <p className="text-red-400">$ npm install express</p>
                  <p className="text-gray-500 mt-2">✗ Used npm (you corrected this before)</p>
                  <p className="text-gray-500">✗ Didn't follow your project conventions</p>
                </div>
              </div>

              {/* With Shadow */}
              <div className="bg-gray-900 border border-green-500/30 rounded-xl p-6">
                <div className="flex items-center gap-2 mb-4">
                  <CheckCircle size={18} className="text-green-400" />
                  <h3 className="font-semibold text-green-400">With Shadow</h3>
                </div>
                <div className="bg-gray-950 rounded-lg p-4 font-mono text-sm">
                  <p className="text-green-400">$ pnpm add express</p>
                  <p className="text-gray-500 mt-2">✓ Automatically used pnpm</p>
                  <p className="text-gray-500">✓ Followed your project conventions</p>
                </div>
                <div className="mt-3 text-xs text-purple-400">
                  ✓ Memory hit: "{demoExample}"
                </div>
              </div>
            </div>

            <div className="text-center py-4">
              <p className="text-lg mb-6">✨ <span className="font-semibold">The same mistake, this time it got it right.</span></p>
              <button
                onClick={finishOnboarding}
                className="px-6 py-3 bg-purple-600 hover:bg-purple-700 rounded-lg font-medium transition-colors"
              >
                Enter Console →
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
