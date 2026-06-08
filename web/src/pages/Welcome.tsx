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

  // Demo data — cycles through the user's real rules if available,
  // otherwise falls back to a built-in roster of examples.
  const SEED_EXAMPLES: { task: string; before: string[]; after: string[]; memory: string }[] = [
    {
      task: 'Add a dependency installation script',
      before: ['$ npm install express', '✗ Used npm (you corrected this before)', '✗ Did not follow your project conventions'],
      after:  ['$ pnpm add express',         '✓ Automatically used pnpm',           '✓ Followed your project conventions'],
      memory: '本项目使用 pnpm，不要用 npm/yarn',
    },
    {
      task: 'Name a new utility file',
      before: ['src/string_utils.ts',         '✗ Used snake_case (you said camelCase)', '✗ Did not respect the project naming convention'],
      after:  ['src/stringUtils.ts',          '✓ Used camelCase',                       '✓ Matches the project naming rule'],
      memory: '本项目使用 camelCase 命名，禁止 snake_case',
    },
    {
      task: 'Write a test for a new function',
      before: ['import { test } from "jest"',  '✗ Used jest (project uses vitest)',   '✗ Has to be rewritten before CI runs'],
      after:  ['import { test } from "vitest"', '✓ Switched to vitest',                '✓ Matches the project testing stack'],
      memory: '测试用 vitest，不用 jest',
    },
    {
      task: 'Handle an error in a fetch call',
      before: ['throw new Error("network failed")', '✗ Throws across the data layer', '✗ Violates your error-handling convention'],
      after:  ['return Err("network failed")',       '✓ Returns Result type',         '✓ Matches your architecture rule'],
      memory: '错误处理优先使用 Result 类型而非 throw',
    },
  ];

  const userExamples = rules.slice(0, 4).map(r => ({
    task: 'Your captured rule',
    before: ['agent does the wrong thing', `✗ Violates "${r.content}"`],
    after:  ['agent follows the rule',     `✓ Honors "${r.content.slice(0, 32)}…"`],
    memory: r.content,
  }));
  const examples = userExamples.length > 0 ? userExamples : SEED_EXAMPLES;
  const [demoIndex, setDemoIndex] = useState(0);
  const demoExample = examples[demoIndex % examples.length];

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
            <p className="text-gray-400 mb-2">Same task, two agents. The one on the right has Shadow.</p>
            <p className="text-xs text-gray-500 mb-8">
              Task: <span className="text-gray-300">{demoExample.task}</span>
              {examples.length > 1 && (
                <button
                  onClick={() => setDemoIndex((demoIndex + 1) % examples.length)}
                  className="ml-3 underline hover:text-gray-300"
                >
                  重新演示 ({demoIndex + 1}/{examples.length})
                </button>
              )}
            </p>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-8">
              {/* Without Shadow */}
              <div className="bg-gray-900 border border-red-500/30 rounded-xl p-6">
                <div className="flex items-center gap-2 mb-4">
                  <XCircle size={18} className="text-red-400" />
                  <h3 className="font-semibold text-red-400">Without Shadow</h3>
                </div>
                <div className="bg-gray-950 rounded-lg p-4 font-mono text-sm space-y-1">
                  {demoExample.before.map((line, i) => (
                    <p key={i} className={i === 0 ? 'text-red-300' : 'text-gray-500'}>{line}</p>
                  ))}
                </div>
              </div>

              {/* With Shadow */}
              <div className="bg-gray-900 border border-green-500/30 rounded-xl p-6">
                <div className="flex items-center gap-2 mb-4">
                  <CheckCircle size={18} className="text-green-400" />
                  <h3 className="font-semibold text-green-400">With Shadow</h3>
                </div>
                <div className="bg-gray-950 rounded-lg p-4 font-mono text-sm space-y-1">
                  {demoExample.after.map((line, i) => (
                    <p key={i} className={i === 0 ? 'text-green-300' : 'text-gray-500'}>{line}</p>
                  ))}
                </div>
                <div className="mt-3 text-xs text-purple-400">
                  ✓ Memory hit: "{demoExample.memory}"
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
