// 记忆地图主组件 — React Flow + 自定义节点/边 + HUD + 抽屉
// 拖动体验对齐 Obsidian：
// - 拖动中：被拖动节点周围其他节点实时软避让（onNodeDrag 每次都算 pushOffset）
// - 拖动停止：被推开节点通过 framer-motion spring 平滑回原位
// 整个过程无阶跃、连续动画

import { useMemo, useState, useCallback, useEffect } from 'react';
import {
  ReactFlow,
  Background,
  BackgroundVariant,
  Controls,
  MiniMap,
  type Node,
  type Edge,
  type NodeMouseHandler,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

import { RuleNode } from './RuleNode';
import { RelationEdge } from './RelationEdge';
import { Hud } from './Hud';
import { DetailDrawer } from './DetailDrawer';
import { MOCK_NODES, MOCK_RELATIONS, MOCK_STATS } from './mockData';
import { computeLayout } from './layout';
import type { MemoryNodeData, MapFilters, MapStats, RelationData } from './types';
import './memory-map.css';

const nodeTypes = { rule: RuleNode };
const edgeTypes = { relation: RelationEdge };

interface MemoryMapProps {
  onOpenInRules?: (id: string) => void;
  nodes?: MemoryNodeData[];
  relations?: { source: string; target: string; data: RelationData }[];
  stats?: MapStats;
}

// 波纹推开参数 — 微调这几个值就能微调手感
const RIPPLE_RADIUS = 240;     // 影响半径 (px)
const RIPPLE_FORCE = 75;       // 最大推开距离 (px)

export function MemoryMap({ onOpenInRules, nodes: propNodes, relations: propRelations, stats: propStats }: MemoryMapProps) {
  // If the caller passed real data, use it; otherwise fall back to mock
  // (mock keeps the canvas renderable during local dev when the daemon
  // is offline / a brand-new user has fewer than the threshold of rules).
  const hasRealData = (propNodes?.length ?? 0) > 0;
  const dataNodes = useMemo(() => hasRealData ? propNodes! : MOCK_NODES, [hasRealData, propNodes]);
  const dataRelations = useMemo(() => hasRealData ? (propRelations ?? []) : MOCK_RELATIONS, [hasRealData, propRelations]);
  const dataStats = useMemo(() => propStats ?? MOCK_STATS, [propStats]);
  const [searchQuery, setSearchQuery] = useState('');
  const [filters, setFilters] = useState<MapFilters>({
    category: 'all',
    status: 'all',
    agent: 'all',
    edgeDensity: 0.4,   // Default: signal + top structure edges; users can raise density.
  });
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [showHint, setShowHint] = useState(true);

  // 节点位置（持久化用户拖动）。
  // 布局计算在 Web Worker 中异步完成，不阻塞主线程渲染。
  const layoutKey = useMemo(
    () => `${dataNodes.map(n => n.id).join('|')}::${dataRelations.map(r => `${r.source}>${r.target}`).join('|')}`,
    [dataNodes, dataRelations]
  );
  const [layout, setLayout] = useState<{ key: string; positions: Record<string, { x: number; y: number }> }>({
    key: '',
    positions: {},
  });
  const positions = layout.positions;
  const layoutBusy = layout.key !== layoutKey;

  useEffect(() => {
    if (dataNodes.length === 0) return;
    const worker = new Worker(
      new URL('./layout.worker.ts', import.meta.url),
      { type: 'module' }
    );
    worker.onmessage = (e: MessageEvent<{ positions: Record<string, { x: number; y: number }> }>) => {
      setLayout({ key: layoutKey, positions: e.data.positions });
      worker.terminate();
    };
    worker.onerror = () => {
      // Fallback: compute synchronously if worker fails.
      const result = computeLayout(
        dataNodes,
        dataRelations.map(r => ({ source: r.source, target: r.target }))
      );
      setLayout({ key: layoutKey, positions: result.positions });
      worker.terminate();
    };
    worker.postMessage({
      nodes: dataNodes,
      relations: dataRelations.map(r => ({ source: r.source, target: r.target })),
    });
    return () => worker.terminate();
  }, [dataNodes, dataRelations, layoutKey]);

  // 推开偏移（拖动中实时更新，停止后归零）
  const [pushOffsets, setPushOffsets] = useState<Record<string, { x: number; y: number }>>({});

  // 搜索 + 筛选
  const matches = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    return new Set(
      dataNodes.filter(n => {
        if (filters.category !== 'all' && n.category !== filters.category) return false;
        if (filters.status !== 'all' && n.status !== filters.status) return false;
        if (q) {
          const blob = `${n.title} ${n.content} ${n.tags.join(' ')} ${n.project_path}`.toLowerCase();
          if (!blob.includes(q)) return false;
        }
        return true;
      }).map(n => n.id)
    );
  }, [dataNodes, searchQuery, filters]);

  const isSearchingOrFiltering = searchQuery.trim() !== '' || filters.category !== 'all' || filters.status !== 'all';

  // === 做减法筛子：基于 edgeDensity 决定哪些边可见 ===
  // signal → 永远显示；structure → 默认每节点 top-3；whisper → 按需
  const visibleEdgeIndices = useMemo(() => {
    const density = filters.edgeDensity;
    const signalVisible = density >= 0.05;
    const structureVisible = density >= 0.2;
    const structureAll = density >= 0.5;
    const whisperVisible = density >= 0.6;
    const whisperAll = density >= 0.9;

    const result = new Set<number>();
    // 待 top-N 过滤的边（按节点收集）
    const byNode = new Map<string, Array<{ idx: number; score: number; tier: string }>>();

    dataRelations.forEach((r, idx) => {
      // tier 解析（兼容旧 kind 字段）
      const tier = r.data.tier ?? (r.data.kind === 'weak' ? 'whisper' : 'structure');
      const score = r.data.score ?? (tier === 'structure' ? 0.5 : 0.2);

      // 无 top-N 限制直接放行
      if (tier === 'signal' && signalVisible) { result.add(idx); return; }
      if (tier === 'structure' && structureAll) { result.add(idx); return; }
      if (tier === 'whisper' && whisperAll) { result.add(idx); return; }

      // 需要 top-N 过滤的
      if ((tier === 'structure' && structureVisible) || (tier === 'whisper' && whisperVisible)) {
        for (const nodeId of [r.source, r.target]) {
          if (!byNode.has(nodeId)) byNode.set(nodeId, []);
          byNode.get(nodeId)!.push({ idx, score, tier });
        }
      }
    });

    // 每节点取 top-N（score 降序）
    const STRUCT_LIMIT = 3;
    const WHISPER_LIMIT = 2;
    for (const nodeEdges of byNode.values()) {
      nodeEdges.sort((a, b) => b.score - a.score);
      let structCount = 0;
      let whisperCount = 0;
      for (const e of nodeEdges) {
        if (e.tier === 'structure' && structCount < STRUCT_LIMIT) {
          result.add(e.idx); structCount++;
        } else if (e.tier === 'whisper' && whisperCount < WHISPER_LIMIT) {
          result.add(e.idx); whisperCount++;
        }
      }
    }
    return result;
  }, [dataRelations, filters.edgeDensity]);

  // === Focus Mode：选中节点的全部关联（跨所有 tier，忽略密度）===
  const connectedToSelected = useMemo(() => {
    if (selectedId === null) return null;
    const set = new Set<string>([selectedId]);
    for (const r of dataRelations) {
      if (r.source === selectedId) set.add(r.target);
      if (r.target === selectedId) set.add(r.source);
    }
    return set;
  }, [selectedId, dataRelations]);

  // 拖动状态：用于 CSS 全局反馈
  const [isAnyDragging, setIsAnyDragging] = useState(false);

  // 推开的工具函数：给定被拖动节点位置，计算所有其他节点的 pushOffset
  const computePushOffsets = useCallback(
    (center: { x: number; y: number }, allPositions: Record<string, { x: number; y: number }>) => {
      const out: Record<string, { x: number; y: number }> = {};
      for (const [id, pos] of Object.entries(allPositions)) {
        const dx = pos.x - center.x;
        const dy = pos.y - center.y;
        const dist = Math.sqrt(dx * dx + dy * dy);
        if (dist < RIPPLE_RADIUS && dist > 1) {
          // 1/r² 衰减：近的推得多，远的推得少（更自然）
          const falloff = 1 - dist / RIPPLE_RADIUS;
          const k = falloff * falloff * RIPPLE_FORCE;
          out[id] = { x: (dx / dist) * k, y: (dy / dist) * k };
        }
      }
      return out;
    },
    []
  );

  // 构建 React Flow nodes
  const nodes: Node<MemoryNodeData & { pushOffsetX?: number; pushOffsetY?: number }>[] = useMemo(() => {
    return dataNodes.map(n => {
      const base = positions[n.id] ?? { x: 0, y: 0 };
      const off = pushOffsets[n.id];
      const matched = matches.has(n.id);
      // Focus Mode 优先于搜索：未与选中节点关联的节点淡出
      const dimmed = connectedToSelected !== null
        ? !connectedToSelected.has(n.id)
        : (isSearchingOrFiltering && !matched);
      return {
        id: n.id,
        type: 'rule',
        position: base,
        data: {
          ...n,
          pushOffsetX: off?.x ?? 0,
          pushOffsetY: off?.y ?? 0,
        },
        selected: n.id === selectedId,
        className: dimmed ? 'mm-node--dimmed' : '',
        draggable: true,
      } as Node<MemoryNodeData & { pushOffsetX?: number; pushOffsetY?: number }>;
    });
  }, [dataNodes, positions, pushOffsets, matches, isSearchingOrFiltering, selectedId, connectedToSelected]);

  // 构建 React Flow edges
  const edges: Edge<RelationData>[] = useMemo(() => {
    const result: Edge<RelationData>[] = [];
    dataRelations.forEach((r, i) => {
      const sourceInMatch = matches.has(r.source);
      const targetInMatch = matches.has(r.target);

      // 可见性：Focus Mode 只渲染选中节点的关联；否则用密度筛子
      let visible: boolean;
      if (connectedToSelected !== null) {
        visible = r.source === selectedId || r.target === selectedId;
      } else {
        visible = visibleEdgeIndices.has(i);
      }
      if (!visible) return;

      const dimmed = isSearchingOrFiltering && !(sourceInMatch && targetInMatch);
      result.push({
        id: `e${i}-${r.source}-${r.target}`,
        source: r.source,
        target: r.target,
        type: 'relation',
        data: r.data,
        className: dimmed ? 'mm-edge--dimmed' : '',
        selected: false,
      } as Edge<RelationData>);
    });
    return result;
  }, [matches, isSearchingOrFiltering, visibleEdgeIndices, connectedToSelected, selectedId, dataRelations]);

  // 节点点击
  const handleNodeClick: NodeMouseHandler = useCallback((_e, node) => {
    setSelectedId(node.id);
  }, []);

  // 画布点击关闭抽屉
  const handlePaneClick = useCallback(() => {
    setSelectedId(null);
  }, []);

  // 拖动开始：清零 pushOffsets（避免开始时残留），标记全局 dragging
  const handleNodeDragStart = useCallback(
    (_e: MouseEvent | TouchEvent, node: Node) => {
      setIsAnyDragging(true);
      setPushOffsets(computePushOffsets(node.position, positions));
    },
    [positions, computePushOffsets]
  );

  // 拖动中：每帧重算 pushOffsets（周围节点实时跟随避让）
  const handleNodeDrag = useCallback(
    (_e: MouseEvent | TouchEvent, node: Node) => {
      setPushOffsets(computePushOffsets(node.position, positions));
    },
    [positions, computePushOffsets]
  );

  // 拖动停止：持久化被拖动节点位置 + pushOffsets 归零（spring 自动回原位）
  const handleNodeDragStop = useCallback(
    (_e: MouseEvent | TouchEvent, node: Node) => {
      setIsAnyDragging(false);
      setLayout(prev => ({ ...prev, positions: { ...prev.positions, [node.id]: node.position } }));
      // 归零 → framer-motion spring 平滑回原位
      setPushOffsets({});
    },
    []
  );

  // Esc 关闭抽屉
  useEffect(() => {
    function handle(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        if (selectedId) {
          setSelectedId(null);
        } else if (searchQuery) {
          setSearchQuery('');
        }
      }
    }
    document.addEventListener('keydown', handle);
    return () => document.removeEventListener('keydown', handle);
  }, [selectedId, searchQuery]);

  // 5 秒后自动隐藏提示
  useEffect(() => {
    const t = setTimeout(() => setShowHint(false), 5000);
    return () => clearTimeout(t);
  }, []);

  const selectedNode = useMemo(
    () => dataNodes.find(n => n.id === selectedId) ?? null,
    [dataNodes, selectedId]
  );

  return (
    <div
      className={`mm-page ${isAnyDragging ? 'mm-page--dragging' : ''}`}
    >
      <div className="mm-canvas-bg" />
      <div className="mm-stardust" />

      {layoutBusy && (
        <div className="mm-layout-loading" role="status">
          <div className="mm-layout-loading-spinner" />
          <span>计算布局…</span>
        </div>
      )}

      <ReactFlow
        nodes={layoutBusy ? [] : nodes}
        edges={layoutBusy ? [] : edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        onNodeClick={handleNodeClick}
        onPaneClick={handlePaneClick}
        onNodeDragStart={handleNodeDragStart}
        onNodeDrag={handleNodeDrag}
        onNodeDragStop={handleNodeDragStop}
        minZoom={0.3}
        maxZoom={2.5}
        defaultViewport={{ x: 0, y: 0, zoom: 0.85 }}
        proOptions={{ hideAttribution: true }}
        nodesDraggable={true}
        nodesConnectable={false}
        elementsSelectable={true}
        nodesFocusable={false}
        fitView={!layoutBusy}
        fitViewOptions={{ padding: 0.25, maxZoom: 0.9 }}
        className="mm-flow"
      >
        <Background
          variant={BackgroundVariant.Dots}
          gap={32}
          size={1}
          color="rgba(255,255,255,0.04)"
        />
        <Controls
          showInteractive={false}
          position="bottom-right"
          style={{ marginBottom: 20, marginRight: 20 }}
        />
        <MiniMap
          position="bottom-right"
          pannable
          zoomable
          maskColor="rgba(10, 10, 15, 0.7)"
          nodeColor={(n) => {
            const data = n.data as MemoryNodeData;
            if (!data) return '#6B7280';
            return data.category === 'code' ? '#3B82F6' :
                   data.category === 'architecture' ? '#A78BFA' : '#10B981';
          }}
          nodeStrokeWidth={0}
          style={{
            marginBottom: 80,
            marginRight: 20,
            width: 160,
            height: 100,
          }}
        />
      </ReactFlow>

      <Hud
        stats={dataStats}
        filters={filters}
        searchQuery={searchQuery}
        searchMatchCount={matches.size}
        onSearchChange={setSearchQuery}
        onFilterChange={setFilters}
      />

      <Legend />

      {selectedNode && (
        <DetailDrawer
          node={selectedNode}
          onClose={() => setSelectedId(null)}
          onOpenInRules={(id) => onOpenInRules?.(id)}
        />
      )}

      {showHint && !selectedNode && (
        <div className="mm-empty-hint" role="status">
          <div className="mm-empty-hint-title">👻 这是你的记忆地图</div>
          <div>滚轮缩放 · 拖动节点 · 点击查看详情</div>
          <a className="mm-empty-hint-cta" onClick={() => setShowHint(false)}>我知道了</a>
        </div>
      )}
    </div>
  );
}

function Legend() {
  return (
    <div className="mm-legend" aria-label="图例">
      <div className="mm-legend-section">
        <div className="mm-legend-title">类别</div>
        <div className="mm-legend-row">
          <svg width="12" height="12" viewBox="0 0 1 1" className="mm-legend-swatch">
            <circle cx="0.5" cy="0.5" r="0.45" fill="var(--cat-code)" />
          </svg>
          <span>Code</span>
        </div>
        <div className="mm-legend-row">
          <svg width="12" height="12" viewBox="0 0 1 1" className="mm-legend-swatch">
            <circle cx="0.5" cy="0.5" r="0.45" fill="var(--cat-architecture)" />
          </svg>
          <span>Architecture</span>
        </div>
        <div className="mm-legend-row">
          <svg width="12" height="12" viewBox="0 0 1 1" className="mm-legend-swatch">
            <circle cx="0.5" cy="0.5" r="0.45" fill="var(--cat-practice)" />
          </svg>
          <span>Practice</span>
        </div>
      </div>
      <div className="mm-legend-section">
        <div className="mm-legend-title">关联</div>
        <div className="mm-legend-row">
          <div className="mm-legend-line mm-legend-line--signal" />
          <span>⚡ 冲突</span>
        </div>
        <div className="mm-legend-row">
          <div className="mm-legend-line mm-legend-line--structure" />
          <span>🔗 结构</span>
        </div>
        <div className="mm-legend-row">
          <div className="mm-legend-line mm-legend-line--whisper" />
          <span>💬 低语</span>
        </div>
      </div>
    </div>
  );
}

export default MemoryMap;
