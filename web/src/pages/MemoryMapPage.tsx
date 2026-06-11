// 记忆地图页面入口
// 拉取后端 /api/dashboard/map 的真数据，传给 MemoryMap 组件；
// 当数据 < 3 条时，混入 mock 让首次打开就有视觉冲击（PRD §5.7 示例数据模式）。

import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { MemoryMap } from '../memory-map/MemoryMap';
import { MOCK_NODES } from '../memory-map/mockData';
import type { DashboardMapEdge } from '../lib/api';
import { api } from '../lib/api';
import { useRealtimeRefresh } from '../lib/realtime';
import type { MemoryNodeData, MapStats, RelationData } from '../memory-map/types';

const SEED_THRESHOLD = 3; // 真数据少于这个数 → 补 mock

export default function MemoryMapPage() {
  const navigate = useNavigate();
  const [nodes, setNodes] = useState<MemoryNodeData[]>([]);
  const [relations, setRelations] = useState<{ source: string; target: string; data: RelationData }[]>([]);
  const [stats, setStats] = useState<MapStats | undefined>(undefined);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadMap = useCallback(async () => {
    try {
      const [map, dash] = await Promise.all([
        api.getDashboardMap(),
        api.getDashboard(),
      ]);

      // Backfill with mock seeds when the user has fewer than 3 rules,
      // so the canvas has something to render on first open (PRD §5.7).
      let realNodes = (map.nodes ?? []) as MemoryNodeData[];
      if (realNodes.length < SEED_THRESHOLD) {
        const existing = new Set(realNodes.map(n => n.id));
        const seeded = MOCK_NODES.filter(n => !existing.has(n.id)).slice(
          0,
          SEED_THRESHOLD - realNodes.length,
        );
        realNodes = [...realNodes, ...seeded];
      }

      // Translate backend edge shape (source/target + data) into the
      // shape MemoryMap's relations prop expects.
      const rels = (map.edges ?? []).map((e: DashboardMapEdge) => ({
        source: e.source,
        target: e.target,
        data: e.data as RelationData,
      }));

      setNodes(realNodes);
      setRelations(rels);
      setStats({
        total: dash.total_rules,
        active: dash.active_rules,
        conflicted: dash.conflicted_rules,
        other: dash.disabled_rules,
        thisMonth: 0,
        growth: { level: 1, progress: 0, nextLevelAt: 5, achievements: [] },
        byCategory: [],
      });
      setError(null);
      setLoading(false);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '加载失败');
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void Promise.resolve().then(loadMap);
  }, [loadMap]);
  useRealtimeRefresh(() => { void loadMap(); });

  return (
    <div className="mm-page-wrapper">
      {loading && <div className="mm-loading">载入记忆地图…</div>}
      {error && <div className="mm-error">{error}</div>}
      {!loading && !error && (
        <MemoryMap
          onOpenInRules={(id) => navigate(`/rules/${id}`)}
          nodes={nodes}
          relations={relations}
          stats={stats}
        />
      )}
    </div>
  );
}
