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
import type { MemoryNodeData, MapFilters, RelationData } from './types';
import './memory-map.css';

const nodeTypes = { rule: RuleNode };
const edgeTypes = { relation: RelationEdge };

interface MemoryMapProps {
  onOpenInRules?: (id: string) => void;
}

// 波纹推开参数 — 微调这几个值就能微调手感
const RIPPLE_RADIUS = 240;     // 影响半径 (px)
const RIPPLE_FORCE = 75;       // 最大推开距离 (px)

export function MemoryMap({ onOpenInRules }: MemoryMapProps) {
  const [searchQuery, setSearchQuery] = useState('');
  const [filters, setFilters] = useState<MapFilters>({
    category: 'all',
    status: 'all',
    agent: 'all',
  });
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [showHint, setShowHint] = useState(true);

  // 节点位置（持久化用户拖动）
  const initialLayout = useMemo(
    () => computeLayout(MOCK_NODES, MOCK_RELATIONS.map(r => ({ source: r.source, target: r.target }))),
    []
  );
  const [positions, setPositions] = useState<Record<string, { x: number; y: number }>>(initialLayout.positions);

  // 推开偏移（拖动中实时更新，停止后归零）
  const [pushOffsets, setPushOffsets] = useState<Record<string, { x: number; y: number }>>({});

  // 搜索 + 筛选
  const matches = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    return new Set(
      MOCK_NODES.filter(n => {
        if (filters.category !== 'all' && n.category !== filters.category) return false;
        if (filters.status !== 'all' && n.status !== filters.status) return false;
        if (q) {
          const blob = `${n.title} ${n.content} ${n.tags.join(' ')} ${n.project_path}`.toLowerCase();
          if (!blob.includes(q)) return false;
        }
        return true;
      }).map(n => n.id)
    );
  }, [searchQuery, filters]);

  const isSearchingOrFiltering = searchQuery.trim() !== '' || filters.category !== 'all' || filters.status !== 'all';

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
    return MOCK_NODES.map(n => {
      const base = positions[n.id] ?? { x: 0, y: 0 };
      const off = pushOffsets[n.id];
      const matched = matches.has(n.id);
      const dimmed = isSearchingOrFiltering && !matched;
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
  }, [positions, pushOffsets, matches, isSearchingOrFiltering, selectedId]);

  // 构建 React Flow edges
  const edges: Edge<RelationData>[] = useMemo(() => {
    return MOCK_RELATIONS.map((r, i) => {
      const sourceInMatch = matches.has(r.source);
      const targetInMatch = matches.has(r.target);
      const dimmed = isSearchingOrFiltering && !(sourceInMatch && targetInMatch);
      return {
        id: `e${i}-${r.source}-${r.target}`,
        source: r.source,
        target: r.target,
        type: 'relation',
        data: r.data,
        className: dimmed ? 'mm-edge--dimmed' : '',
        selected: false,
      } as Edge<RelationData>;
    });
  }, [matches, isSearchingOrFiltering]);

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
      setPositions(prev => ({ ...prev, [node.id]: node.position }));
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
    () => MOCK_NODES.find(n => n.id === selectedId) ?? null,
    [selectedId]
  );

  return (
    <div
      className={`mm-page ${isAnyDragging ? 'mm-page--dragging' : ''}`}
    >
      <div className="mm-canvas-bg" />
      <div className="mm-stardust" />

      <ReactFlow
        nodes={nodes}
        edges={edges}
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
        fitView
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
        stats={MOCK_STATS}
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
          <div className="mm-legend-line mm-legend-line--strong" />
          <span>同项目</span>
        </div>
        <div className="mm-legend-row">
          <div className="mm-legend-line mm-legend-line--medium" />
          <span>共享标签</span>
        </div>
        <div className="mm-legend-row">
          <div className="mm-legend-line mm-legend-line--weak" />
          <span>语义相似</span>
        </div>
      </div>
    </div>
  );
}

export default MemoryMap;
