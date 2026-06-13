// 自定义关联边 — 4 层筛子的视觉编码（做减法）
//
// signal · conflict:  2.5px 实线 红 + 红光呼吸（警报，唯一带动画的线）
// signal · evolution: 2.5px 实线 紫（规则演进）
// signal · override:  2.5px 实线 琥珀（上下文覆盖）
// structure:          1.5px 实线 灰（强关联，默认可见）
// whisper:            0.8px 虚线 浅灰（弱关联，按需显示）
//
// 每一个视觉变量都承载语义——没有装饰性的差异。

import { memo } from 'react';
import {
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  type EdgeProps,
} from '@xyflow/react';
import type { RelationData, EdgeTier, SignalType, RelationKind } from './types';

interface Props extends EdgeProps {
  data?: RelationData;
}

interface TierStyle {
  stroke: string;
  strokeWidth: number;
  strokeDasharray: string | undefined;
  opacity: number;
}

// 信号线子样式（永远显示，0.85 高透明度）
const SIGNAL_STYLES: Record<SignalType, TierStyle> = {
  conflict: { stroke: '#EF4444', strokeWidth: 2.5, strokeDasharray: undefined, opacity: 0.85 },
  evolution: { stroke: 'var(--rel-strong)', strokeWidth: 2.5, strokeDasharray: undefined, opacity: 0.85 },
  override: { stroke: '#F59E0B', strokeWidth: 2.5, strokeDasharray: undefined, opacity: 0.85 },
};

// 结构线 / 低语线
const TIER_STYLES: Record<Exclude<EdgeTier, 'signal'>, TierStyle> = {
  structure: { stroke: 'var(--rel-medium)', strokeWidth: 1.5, strokeDasharray: undefined, opacity: 0.55 },
  whisper: { stroke: 'var(--rel-weak)', strokeWidth: 0.8, strokeDasharray: '4 4', opacity: 0.25 },
};

// 旧 kind → tier 降级映射（mock / 旧 API 向后兼容）
function kindToTier(kind: RelationKind): Exclude<EdgeTier, 'signal'> {
  // strong（同项目）/ medium（共享标签）→ structure；weak（语义）→ whisper
  return kind === 'weak' ? 'whisper' : 'structure';
}

// 解析 data → 视觉样式 + 信号子类型
function resolveStyle(data?: RelationData): { style: TierStyle; signalType?: SignalType } {
  if (!data) return { style: TIER_STYLES.structure };

  // 优先用新 tier 字段
  if (data.tier) {
    if (data.tier === 'signal') {
      const signalType: SignalType = data.signalType ?? 'conflict';
      return { style: SIGNAL_STYLES[signalType], signalType };
    }
    return { style: TIER_STYLES[data.tier] };
  }

  // 降级：旧 kind 字段
  if (data.kind) {
    const tier = kindToTier(data.kind);
    return { style: TIER_STYLES[tier] };
  }

  return { style: TIER_STYLES.structure };
}

function RelationEdgeImpl({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
  selected,
}: Props) {
  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
  });

  const { style, signalType } = resolveStyle(data);
  const isConflict = signalType === 'conflict';

  return (
    <>
      <BaseEdge
        id={id}
        path={edgePath}
        style={{
          stroke: style.stroke,
          strokeWidth: selected ? style.strokeWidth * 1.5 : style.strokeWidth,
          strokeDasharray: style.strokeDasharray,
          opacity: style.opacity,
          // 仅 transition stroke-width，避免与冲突脉冲的 opacity 动画打架
          transition: 'stroke-width 200ms cubic-bezier(0.16, 1, 0.3, 1)',
          // 冲突线呼吸脉冲：唯一带动画的线 = "我在这里，请处理我"
          animation: isConflict ? 'mm-edge-conflict-pulse 2s ease-in-out infinite' : undefined,
        }}
      />
      {selected && data?.reason && (
        <EdgeLabelRenderer>
          <div
            className={`mm-edge-label ${signalType ? `mm-edge-label--signal` : ''}`}
            style={{
              position: 'absolute',
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
              background: 'var(--surface-strong)',
              border: `1px solid ${signalType === 'conflict' ? '#EF4444' : 'var(--border-strong)'}`,
              borderRadius: 4,
              padding: '2px 6px',
              fontSize: 10,
              color: signalType === 'conflict' ? '#FCA5A5' : 'var(--text-secondary)',
              fontFamily: 'JetBrains Mono, Menlo, monospace',
              pointerEvents: 'none',
              whiteSpace: 'nowrap',
            }}
          >
            {data.reason}
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  );
}

export const RelationEdge = memo(RelationEdgeImpl);
