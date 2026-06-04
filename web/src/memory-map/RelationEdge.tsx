// 自定义关联边 — 3 种关联类型
// strong（同项目）：粗实线，品牌紫
// medium（共享标签）：细实线，灰
// weak（语义相似）：虚线，浅蓝

import { memo } from 'react';
import {
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  type EdgeProps,
} from '@xyflow/react';
import type { RelationData } from './types';

interface Props extends EdgeProps {
  data?: RelationData;
}

const RELATION_STYLES = {
  strong: {
    stroke: 'var(--rel-strong)',
    strokeWidth: 1.8,
    strokeDasharray: undefined as string | undefined,
    opacity: 0.7,
  },
  medium: {
    stroke: 'var(--rel-medium)',
    strokeWidth: 0.8,
    strokeDasharray: undefined,
    opacity: 0.45,
  },
  weak: {
    stroke: 'var(--rel-weak)',
    strokeWidth: 0.7,
    strokeDasharray: '3 3',
    opacity: 0.35,
  },
} as const;

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

  const kind = data?.kind ?? 'medium';
  const style = RELATION_STYLES[kind];

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
          transition: 'all 200ms cubic-bezier(0.16, 1, 0.3, 1)',
        }}
      />
      {selected && data?.reason && (
        <EdgeLabelRenderer>
          <div
            style={{
              position: 'absolute',
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
              background: 'var(--surface-strong)',
              border: '1px solid var(--border-strong)',
              borderRadius: 4,
              padding: '2px 6px',
              fontSize: 10,
              color: 'var(--text-secondary)',
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
