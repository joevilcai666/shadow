import { useEffect, useState } from 'react';
import { api, type Config, type Adapter } from '../lib/api';
import { Shield, Eye, Zap, RefreshCw } from 'lucide-react';

export default function Settings() {
  const [config, setConfig] = useState<Config | null>(null);
  const [adapters, setAdapters] = useState<Adapter[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    Promise.all([
      api.getConfig().catch(() => null),
      api.listAdapters().catch(() => []),
    ]).then(([c, a]) => {
      setConfig(c);
      setAdapters(a || []);
    }).finally(() => setLoading(false));
  }, []);

  const updateSetting = async (key: string, value: unknown) => {
    setSaving(true);
    try {
      await api.updateConfig({ [key]: value });
      // Optimistic update
      if (config) {
        const newConfig = { ...config };
        if (key === 'capture_enabled') newConfig.capture.enabled = value as boolean;
        if (key === 'auto_activate_low_risk') newConfig.distill.auto_activate_low_risk = value as boolean;
        if (key === 'batch_mode') newConfig.distill.batch_mode = value as boolean;
        if (key === 'distill_threshold') newConfig.distill.threshold = value as string;
        setConfig(newConfig);
      }
    } catch (err) {
      console.error('Failed to update config:', err);
    }
    setSaving(false);
  };

  const toggleAdapter = async (name: string, enabled: boolean) => {
    try {
      await api.toggleAdapter(name, !enabled);
      setAdapters(prev => prev.map(a => a.name === name ? { ...a, enabled: !enabled } : a));
    } catch (err) {
      console.error('Failed to toggle adapter:', err);
    }
  };

  // Toggle component
  const Toggle = ({ enabled, onChange }: { enabled: boolean; onChange: () => void }) => (
    <button onClick={onChange} className={`w-10 h-5 rounded-full relative transition-colors ${enabled ? 'bg-purple-500' : 'bg-gray-700'}`}>
      <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full transition-transform ${enabled ? 'right-0.5' : 'left-0.5'}`} />
    </button>
  );

  if (loading) return <div className="p-8 text-center text-gray-500">Loading settings...</div>;

  const captureEnabled = config?.capture.enabled ?? true;
  const threshold = config?.distill.threshold ?? 'medium';
  const autoActivate = config?.distill.auto_activate_low_risk ?? true;
  const batchMode = config?.distill.batch_mode ?? false;

  return (
    <div className="p-8 max-w-3xl">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Settings</h1>
        {saving && <span className="text-xs text-gray-500">Saving...</span>}
      </div>

      <div className="space-y-6">
        {/* Capture */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
          <div className="flex items-center gap-3 mb-2">
            <Eye size={20} className="text-purple-400" />
            <h2 className="text-lg font-semibold">Capture</h2>
          </div>
          <p className="text-sm text-gray-500 mb-4">Control how Shadow captures your corrections.</p>
          <div className="space-y-4">
            <div className="flex items-center justify-between py-2 border-t border-gray-800">
              <div>
                <span className="text-sm">Capture Enabled</span>
                <p className="text-xs text-gray-500">Watch agent logs for correction signals</p>
              </div>
              <Toggle enabled={captureEnabled} onChange={() => updateSetting('capture_enabled', !captureEnabled)} />
            </div>
            <div className="flex items-center justify-between py-2 border-t border-gray-800">
              <span className="text-sm">Batch Mode</span>
              <Toggle enabled={batchMode} onChange={() => updateSetting('batch_mode', !batchMode)} />
            </div>
          </div>
        </div>

        {/* Distillation */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
          <div className="flex items-center gap-3 mb-2">
            <Zap size={20} className="text-purple-400" />
            <h2 className="text-lg font-semibold">Distillation</h2>
          </div>
          <p className="text-sm text-gray-500 mb-4">How corrections become rules.</p>
          <div className="space-y-4">
            <div className="flex items-center justify-between py-2 border-t border-gray-800">
              <span className="text-sm">Threshold</span>
              <select value={threshold} onChange={e => updateSetting('distill_threshold', e.target.value)}
                className="bg-gray-800 border border-gray-700 rounded px-3 py-1 text-sm focus:outline-none focus:border-purple-500">
                <option value="low">Low (1 signal)</option>
                <option value="medium">Medium (2 signals)</option>
                <option value="high">High (5 signals)</option>
              </select>
            </div>
            <div className="flex items-center justify-between py-2 border-t border-gray-800">
              <div>
                <span className="text-sm">Auto-activate Low Risk</span>
                <p className="text-xs text-gray-500">Automatically activate high-confidence rules</p>
              </div>
              <Toggle enabled={autoActivate} onChange={() => updateSetting('auto_activate_low_risk', !autoActivate)} />
            </div>
          </div>
        </div>

        {/* Adapters */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
          <div className="flex items-center gap-3 mb-2">
            <RefreshCw size={20} className="text-purple-400" />
            <h2 className="text-lg font-semibold">Adapters</h2>
          </div>
          <p className="text-sm text-gray-500 mb-4">Connected coding agents.</p>
          <div className="space-y-3">
            {adapters.map(adapter => (
              <div key={adapter.name} className="flex items-center justify-between py-3 border-t border-gray-800">
                <div>
                  <span className="text-sm">{adapter.label}</span>
                  <p className="text-xs text-gray-600 font-mono mt-0.5">{adapter.target_path}</p>
                </div>
                <div className="flex items-center gap-3">
                  {adapter.installed ? (
                    <>
                      <Toggle enabled={adapter.enabled} onChange={() => toggleAdapter(adapter.name, adapter.enabled)} />
                      <span className={`text-xs px-2 py-1 rounded ${
                        adapter.enabled ? 'bg-green-500/10 text-green-400' : 'bg-gray-800 text-gray-500'
                      }`}>{adapter.enabled ? 'Active' : 'Paused'}</span>
                    </>
                  ) : (
                    <span className="text-xs bg-gray-800 text-gray-500 px-2 py-1 rounded">Not Detected</span>
                  )}
                </div>
              </div>
            ))}
          </div>
          <button
            onClick={() => api.syncAdapters().then(() => alert('Sync triggered'))}
            className="mt-4 px-4 py-2 bg-gray-800 hover:bg-gray-700 rounded-lg text-sm transition-colors"
          >
            Sync All Now
          </button>
        </div>

        {/* Privacy */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
          <div className="flex items-center gap-3 mb-2">
            <Shield size={20} className="text-purple-400" />
            <h2 className="text-lg font-semibold">Privacy & Trust</h2>
          </div>
          <p className="text-sm text-gray-500 mb-4">What Shadow never stores.</p>
          <div className="space-y-3">
            {[
              { label: 'Block Keys/Tokens', value: 'Always on', type: 'readonly' },
              { label: 'Store Raw Conversations', value: 'Never', type: 'readonly' },
              { label: 'Only Distilled Rules', value: 'Yes', type: 'readonly' },
              { label: 'Exclude Patterns', value: `${config?.privacy?.exclude_patterns?.length ?? 6} patterns configured`, type: 'info' },
              { label: 'Deny Patterns', value: `${config?.privacy?.deny_patterns?.length ?? 3} patterns active`, type: 'info' },
              { label: 'Data Location', value: '~/.shadow/ (local only)', type: 'readonly' },
            ].map(({ label, value, type }) => (
              <div key={label} className="flex items-center justify-between py-2 border-t border-gray-800">
                <span className="text-sm">{label}</span>
                {type === 'readonly' ? (
                  <span className="text-xs text-green-400">{value}</span>
                ) : (
                  <span className="text-xs text-gray-500">{value}</span>
                )}
              </div>
            ))}
          </div>
        </div>

        {/* About */}
        <div className="p-4 bg-gray-900/50 border border-gray-800 rounded-xl text-center">
          <p className="text-sm text-gray-500">Shadow v0.1.0 — Local-first, your data stays yours.</p>
          <p className="text-xs text-gray-600 mt-1">Run <code className="bg-gray-800 px-1 py-0.5 rounded">shadow uninstall --clean-blocks</code> to remove all traces.</p>
        </div>
      </div>
    </div>
  );
}
