// 记忆地图页面入口
// 拉取后端 /api/dashboard/map 的真数据，传给 MemoryMap 组件；
// 没有真实数据时才展示示例；有任何真实规则时，证据链必须完全来自后端。

import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { MemoryMap } from '../memory-map/MemoryMap';
import { MOCK_NODES, MOCK_RELATIONS } from '../memory-map/mockData';
import type { DashboardMapEdge } from '../lib/api';
import { api } from '../lib/api';
import { useRealtimeRefresh } from '../lib/realtime';
import type { MemoryNodeData, MapStats, RelationData } from '../memory-map/types';

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

      const realNodes = (map.nodes ?? []) as MemoryNodeData[];
      const nodesForCanvas = realNodes.length > 0 ? realNodes : MOCK_NODES;

      // Translate backend edge shape (source/target + data) into the
      // shape MemoryMap's relations prop expects.
      const realRelations = (map.edges ?? []).map((e: DashboardMapEdge) => ({
        source: e.source,
        target: e.target,
        data: e.data as RelationData,
      }));
      const rels = realNodes.length > 0 ? realRelations : MOCK_RELATIONS;

      setNodes(nodesForCanvas);
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
