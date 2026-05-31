import { Shield, Eye, Zap, Bell } from 'lucide-react';

export default function Settings() {

  const sections = [
    {
      title: 'Capture',
      icon: Eye,
      description: 'Control how Shadow captures your corrections.',
      settings: [
        { label: 'Capture Enabled', value: 'Yes', type: 'toggle' },
        { label: 'Batch Mode', value: 'No', type: 'toggle' },
      ],
    },
    {
      title: 'Distillation',
      icon: Zap,
      description: 'How corrections become rules.',
      settings: [
        { label: 'Threshold', value: 'Medium', type: 'select' },
        { label: 'Auto-activate Low Risk', value: 'Yes', type: 'toggle' },
      ],
    },
    {
      title: 'Privacy',
      icon: Shield,
      description: 'What Shadow never stores.',
      settings: [
        { label: 'Block Keys/Tokens', value: 'Yes (always on)', type: 'readonly' },
        { label: 'Store Raw Conversations', value: 'No (rules only)', type: 'readonly' },
        { label: 'Exclude Patterns', value: '6 patterns configured', type: 'info' },
      ],
    },
    {
      title: 'Adapters',
      icon: Bell,
      description: 'Connected coding agents.',
      settings: [
        { label: 'Claude Code', value: 'Connected', type: 'status' },
        { label: 'Cursor', value: 'Connected', type: 'status' },
        { label: 'GitHub Copilot', value: 'Not detected', type: 'status' },
      ],
    },
  ];

  return (
    <div className="p-8 max-w-3xl">
      <h1 className="text-2xl font-bold mb-6">Settings</h1>

      <div className="space-y-6">
        {sections.map(({ title, icon: Icon, description, settings }) => (
          <div key={title} className="bg-gray-900 border border-gray-800 rounded-xl p-6">
            <div className="flex items-center gap-3 mb-2">
              <Icon size={20} className="text-purple-400" />
              <h2 className="text-lg font-semibold">{title}</h2>
            </div>
            <p className="text-sm text-gray-500 mb-4">{description}</p>

            <div className="space-y-3">
              {settings.map(({ label, value, type }) => (
                <div key={label} className="flex items-center justify-between py-2 border-t border-gray-800">
                  <span className="text-sm">{label}</span>
                  {type === 'toggle' && value === 'Yes' ? (
                    <div className="w-10 h-5 bg-purple-500 rounded-full relative">
                      <div className="absolute right-0.5 top-0.5 w-4 h-4 bg-white rounded-full" />
                    </div>
                  ) : type === 'status' ? (
                    <span className={`text-xs px-2 py-0.5 rounded ${
                      value === 'Connected' ? 'bg-green-500/10 text-green-400' : 'bg-gray-800 text-gray-500'
                    }`}>{value}</span>
                  ) : type === 'readonly' ? (
                    <span className="text-xs text-gray-500">{value}</span>
                  ) : (
                    <span className="text-sm text-gray-400">{value}</span>
                  )}
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>

      <div className="mt-8 p-4 bg-gray-900/50 border border-gray-800 rounded-xl text-center">
        <p className="text-sm text-gray-500">Shadow v0.1.0 — Local-first, your data stays yours.</p>
      </div>
    </div>
  );
}
