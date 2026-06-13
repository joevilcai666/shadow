import { useCallback, useEffect, useMemo, useState } from 'react';
import { Download, FolderOpen, Globe2, Search, Trash2 } from 'lucide-react';
import { Input } from '@heroui/react';
import { api, type UserMemory } from '../lib/api';
import { IconButton, LoadingState, ShadowButton, ShadowCard, TagChip } from '../components/ui';
import { useRealtimeRefresh } from '../lib/realtime';

const categoryLabels: Record<UserMemory['category'], string> = {
  preference: 'Preference',
  convention: 'Convention',
  context: 'Context',
};

export default function Memories() {
  const [memories, setMemories] = useState<UserMemory[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [exporting, setExporting] = useState(false);

  const loadMemories = useCallback(() => {
    api.listMemories()
      .then(setMemories)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { loadMemories(); }, [loadMemories]);
  useRealtimeRefresh(loadMemories);

  const visibleMemories = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return memories;
    return memories.filter(memory => (
      memory.content.toLowerCase().includes(q) ||
      memory.category.toLowerCase().includes(q) ||
      memory.tags?.some(tag => tag.toLowerCase().includes(q)) ||
      memory.project_path?.toLowerCase().includes(q)
    ));
  }, [memories, search]);

  const deleteMemory = async (id: string) => {
    await api.deleteMemory(id);
    loadMemories();
  };

  const exportData = async () => {
    setExporting(true);
    try {
      const pkg = await api.exportPackage();
      const blob = new Blob([JSON.stringify(pkg, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `shadow-export-${new Date().toISOString().slice(0, 10)}.json`;
      link.click();
      URL.revokeObjectURL(url);
    } finally {
      setExporting(false);
    }
  };

  const formatTime = (value: string) => value ? new Date(value).toLocaleString() : 'Unknown';

  return (
    <div className="p-8">
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold">Memories</h1>
          <p className="mt-1 text-sm text-gray-500">Personal context shared across local agents.</p>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-sm text-gray-500">{visibleMemories.length} shown</span>
          <ShadowButton onClick={exportData} isDisabled={exporting} className="gap-2">
            <Download size={16} />
            {exporting ? 'Exporting' : 'Export'}
          </ShadowButton>
        </div>
      </div>

      <div className="mb-6 max-w-md">
        <div className="relative">
          <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" />
          <Input
            type="text"
            placeholder="Search memories..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="w-full rounded-lg border border-gray-800 bg-gray-900 py-2 pl-10 pr-4 text-sm text-gray-100"
          />
        </div>
      </div>

      {loading ? (
        <LoadingState label="Loading memories..." />
      ) : visibleMemories.length === 0 ? (
        <div className="py-12 text-center text-gray-500">
          No memories found.
        </div>
      ) : (
        <div className="space-y-2">
          {visibleMemories.map(memory => (
            <ShadowCard key={memory.id} className="p-4 transition-colors hover:border-gray-700">
              <div className="flex items-start gap-3">
                <div className="mt-1 text-gray-500">
                  {memory.project_path ? <FolderOpen size={18} /> : <Globe2 size={18} />}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="mb-2 flex flex-wrap items-center gap-2">
                    <TagChip>{categoryLabels[memory.category] ?? memory.category}</TagChip>
                    <span className="text-xs text-gray-500">{memory.project_path ? 'project' : 'global'}</span>
                    <span className="text-xs text-gray-600">Updated {formatTime(memory.updated_at)}</span>
                  </div>
                  <p className="text-sm leading-relaxed text-gray-100">{memory.content}</p>
                  {memory.project_path && (
                    <p className="mt-2 truncate font-mono text-xs text-gray-600">{memory.project_path}</p>
                  )}
                  {memory.tags?.length > 0 && (
                    <div className="mt-3 flex flex-wrap gap-2">
                      {memory.tags.map(tag => <TagChip key={tag}>{tag}</TagChip>)}
                    </div>
                  )}
                </div>
                <IconButton
                  onClick={() => deleteMemory(memory.id)}
                  tone="subtle"
                  label="Delete memory"
                  className="hover:text-red-300"
                >
                  <Trash2 size={16} />
                </IconButton>
              </div>
            </ShadowCard>
          ))}
        </div>
      )}
    </div>
  );
}
