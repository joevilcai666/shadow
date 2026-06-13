import { useEffect, useState } from 'react';
import { api, type Config, type Adapter } from '../lib/api';
import { Shield, Eye, Zap, RefreshCw } from 'lucide-react';
import { Dropdown, Switch, toast } from '@heroui/react';
import { LoadingState, SectionCard, ShadowButton, ShadowCard, TagChip } from '../components/ui';

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
      if (config) {
        const newConfig = {
          ...config,
          capture: { ...config.capture },
          distill: { ...config.distill },
        };
        if (key === 'capture_enabled') newConfig.capture.enabled = value as boolean;
        if (key === 'batch_mode') newConfig.distill.batch_mode = value as boolean;
        if (key === 'distill_threshold') newConfig.distill.threshold = value as string;
        setConfig(newConfig);
      }
    } catch (err) {
      console.error('Failed to update config:', err);
      toast.danger('Failed to update settings');
    }
    setSaving(false);
  };

  const toggleCapture = async () => {
    if (!config) return;
    setSaving(true);
    try {
      await api.toggleCapture();
      setConfig({
        ...config,
        capture: { ...config.capture, enabled: !config.capture.enabled },
      });
    } catch (err) {
      console.error('Failed to toggle capture:', err);
      toast.danger('Capture toggle failed');
    }
    setSaving(false);
  };

  const toggleAdapter = async (name: string, enabled: boolean) => {
    try {
      await api.toggleAdapter(name, !enabled);
      setAdapters(prev => prev.map(a => a.name === name ? { ...a, enabled: !enabled } : a));
    } catch (err) {
      console.error('Failed to toggle adapter:', err);
      toast.danger('Adapter update failed');
    }
  };

  if (loading) return <div className="p-8"><LoadingState label="Loading settings..." /></div>;

  const captureEnabled = config?.capture.enabled ?? true;
  const threshold = config?.distill.threshold ?? 'medium';
  const batchMode = config?.distill.batch_mode ?? false;
  const thresholdLabel = threshold === 'low' ? 'Low (1 signal)' : threshold === 'high' ? 'High (5 signals)' : 'Medium (2 signals)';
  const formatTime = (value: string) => value ? new Date(value).toLocaleString() : 'Never';

  return (
    <div className="p-8 max-w-3xl">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Settings</h1>
        {saving && <span className="text-xs text-gray-500">Saving...</span>}
      </div>

      <div className="space-y-6">
        <SectionCard
          title="Capture"
          description="Control how Shadow captures your corrections."
          icon={<Eye size={20} />}
        >
          <div className="space-y-4">
            <div className="flex items-center justify-between py-2 border-t border-gray-800">
              <div>
                <span className="text-sm">Capture Enabled</span>
                <p className="text-xs text-gray-500">Watch agent logs for correction signals</p>
              </div>
              <Switch isSelected={captureEnabled} onChange={toggleCapture} />
            </div>
            <div className="flex items-center justify-between py-2 border-t border-gray-800">
              <span className="text-sm">Batch Mode</span>
              <Switch isSelected={batchMode} onChange={() => updateSetting('batch_mode', !batchMode)} />
            </div>
          </div>
        </SectionCard>

        <SectionCard
          title="Distillation"
          description="How corrections become reviewable candidate rules."
          icon={<Zap size={20} />}
        >
          <div className="space-y-4">
            <div className="flex items-center justify-between py-2 border-t border-gray-800">
              <span className="text-sm">Threshold</span>
              <Dropdown>
                <Dropdown.Trigger>
                  <ShadowButton className="min-w-40 justify-between">{thresholdLabel}</ShadowButton>
                </Dropdown.Trigger>
                <Dropdown.Popover className="rounded-lg border border-gray-800 bg-gray-900 p-1 text-gray-100 shadow-xl">
                  <Dropdown.Menu onAction={(key) => updateSetting('distill_threshold', String(key))}>
                    <Dropdown.Item id="low" className="rounded px-3 py-2 text-sm hover:bg-gray-800">Low (1 signal)</Dropdown.Item>
                    <Dropdown.Item id="medium" className="rounded px-3 py-2 text-sm hover:bg-gray-800">Medium (2 signals)</Dropdown.Item>
                    <Dropdown.Item id="high" className="rounded px-3 py-2 text-sm hover:bg-gray-800">High (5 signals)</Dropdown.Item>
                  </Dropdown.Menu>
                </Dropdown.Popover>
              </Dropdown>
            </div>
            <div className="flex items-center justify-between py-2 border-t border-gray-800">
              <div>
                <span className="text-sm">Activation Policy</span>
                <p className="text-xs text-gray-500">New memories stay candidates until you approve them</p>
              </div>
              <TagChip>Manual approval</TagChip>
            </div>
          </div>
        </SectionCard>

        <SectionCard
          title="Adapters"
          description="Connected coding agents."
          icon={<RefreshCw size={20} />}
        >
          <div className="space-y-3">
            {adapters.map(adapter => (
              <div key={adapter.name} className="flex items-center justify-between py-3 border-t border-gray-800">
                <div>
                  <span className="text-sm">{adapter.label}</span>
                  <p className="text-xs text-gray-600 font-mono mt-0.5">{adapter.target_path}</p>
                  <div className="mt-2 grid gap-1 text-xs text-gray-500 sm:grid-cols-2">
                    <span>Last sync: <span className="text-gray-300">{formatTime(adapter.last_sync_at)}</span></span>
                    <span>Hits: <span className="text-gray-300">{adapter.hit_count}</span></span>
                    <span>Managed block: <span className="text-gray-300">{adapter.managed_block_status}</span></span>
                    {adapter.last_error && (
                      <span className="text-red-300 sm:col-span-2">Last error: {adapter.last_error}</span>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  {adapter.installed ? (
                    <>
                      <Switch isSelected={adapter.enabled} onChange={() => toggleAdapter(adapter.name, adapter.enabled)} />
                      <TagChip className={adapter.enabled ? 'text-green-300' : 'text-gray-500'}>
                        {adapter.enabled ? 'Active' : 'Paused'}
                      </TagChip>
                    </>
                  ) : (
                    <TagChip className="text-gray-500">Not Detected</TagChip>
                  )}
                </div>
              </div>
            ))}
          </div>
          <ShadowButton
            onClick={() => api.syncAdapters().then(() => toast.success('Sync triggered')).catch(() => toast.danger('Sync failed'))}
            className="mt-4 gap-2"
          >
            Sync All Now
          </ShadowButton>
        </SectionCard>

        <SectionCard
          title="Privacy & Trust"
          description="What Shadow never stores."
          icon={<Shield size={20} />}
        >
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
                  <TagChip className="text-green-300">{value}</TagChip>
                ) : (
                  <span className="text-xs text-gray-500">{value}</span>
                )}
              </div>
            ))}
          </div>
        </SectionCard>

        <ShadowCard className="p-4 text-center">
          <p className="text-sm text-gray-500">Shadow v0.1.0 - Local-first, your data stays yours.</p>
          <p className="text-xs text-gray-600 mt-1">Run <code className="bg-gray-800 px-1 py-0.5 rounded">shadow uninstall --clean-blocks</code> to remove all traces.</p>
        </ShadowCard>
      </div>
    </div>
  );
}
