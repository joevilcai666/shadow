// 自定义记忆节点 — 统一圆形
// 拖动体验：framer-motion 同时控制 scale + x/y 推开位移
// - 推开位移：spring，拖动中实时更新
// - scale：spring，根据 selected/dragging/hover 切换
//   整个节点动画统一交给 framer-motion，避免 CSS transform 冲突

import { memo, useEffect, useState } from 'react';
import { Handle, Position, type NodeProps } from '@xyflow/react';
import { motion, useMotionValue, useSpring, useTransform } from 'framer-motion';
import type { MemoryNodeData, Category } from './types';

const CATEGORY_COLORS: Record<Category, string> = {
  code: 'var(--cat-code)',
  architecture: 'var(--cat-architecture)',
  practice: 'var(--cat-practice)',
};

const CATEGORY_LABELS: Record<Category, string> = {
  code: 'CODE',
  architecture: 'ARCH',
  practice: 'PRAC',
};

const CONFIDENCE_RADIUS: Record<'high' | 'mid' | 'low', number> = {
  high: 22,
  mid: 15,
  low: 9,
};

function confidenceBucket(c: number): 'high' | 'mid' | 'low' {
  if (c >= 0.9) return 'high';
  if (c >= 0.6) return 'mid';
  return 'low';
}

// Spring 配置
// 推开位移：硬一些、阻尼小一些（被推开后想回去的感觉）
const PUSH_SPRING = { stiffness: 220, damping: 22, mass: 0.8 };
// scale 切换：更软一些（hover/select 反馈更柔和）
const SCALE_SPRING = { stiffness: 300, damping: 28, mass: 0.6 };

interface Props extends NodeProps {
  data: MemoryNodeData & {
    pushOffsetX?: number;
    pushOffsetY?: number;
  };
}

function RuleNodeImpl({ data, selected, dragging }: Props) {
  const r = CONFIDENCE_RADIUS[confidenceBucket(data.confidence)];
  const fillOpacity = data.status === 'other' ? 0.45 : 1;
  const stroke = data.status === 'conflicted'
    ? 'var(--status-conflicted)'
    : selected
      ? 'var(--brand)'
      : 'transparent';
  const strokeWidth = data.status === 'conflicted' ? 2 : selected ? 1.5 : 0;
  const fill = CATEGORY_COLORS[data.category];

  // 推开位移（spring 平滑）
  const pushX = useMotionValue(0);
  const pushY = useMotionValue(0);
  const springX = useSpring(pushX, PUSH_SPRING);
  const springY = useSpring(pushY, PUSH_SPRING);

  // scale 反馈（拖动 1.08、选中 1.04、hover 1.06、默认 1.0）
  const [isHovered, setIsHovered] = useState(false);
  const targetScale = dragging ? 1.08 : selected ? 1.04 : isHovered ? 1.06 : 1.0;
  const scale = useSpring(useTransform(useMotionValue(targetScale), (v) => v), SCALE_SPRING);

  useEffect(() => {
    scale.set(targetScale);
  }, [targetScale, scale]);

  // 每次 pushOffset 变化（拖动中或停止），spring 自动平滑到目标值
  useEffect(() => {
    pushX.set(data.pushOffsetX ?? 0);
    pushY.set(data.pushOffsetY ?? 0);
  }, [data.pushOffsetX, data.pushOffsetY, pushX, pushY]);

  return (
    <motion.div
      className={[
        'mm-node',
        selected ? 'mm-node--selected' : '',
        dragging ? 'mm-node--dragging' : '',
      ].filter(Boolean).join(' ')}
      style={{
        x: springX,
        y: springY,
        scale,
        width: r * 2.4,
        height: r * 2.4,
      }}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      title={`${data.title} · ${(data.confidence * 100).toFixed(0)}%`}
    >
      <Handle
        type="target"
        position={Position.Top}
        style={{ background: 'transparent', border: 'none', width: 0, height: 0 }}
      />

      <svg
        width={r * 2.4}
        height={r * 2.4}
        className="mm-node-shape"
        style={{ overflow: 'visible' }}
      >
        <defs>
          <filter id={`glow-${data.id}`} x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="2" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        <circle
          cx={r * 1.2}
          cy={r * 1.2}
          r={r}
          fill={fill}
          fillOpacity={fillOpacity}
          stroke={stroke}
          strokeWidth={strokeWidth}
          vectorEffect="non-scaling-stroke"
          filter={data.status === 'active' || data.status === 'conflicted' ? `url(#glow-${data.id})` : undefined}
        />
      </svg>

      <div
        className={`mm-node-status mm-node-status--${data.status}`}
        aria-label={`status: ${data.status}`}
      />

      {r >= 14 && (
        <div
          style={{
            position: 'absolute',
            fontSize: 7.5,
            fontWeight: 700,
            color: 'rgba(255,255,255,0.92)',
            letterSpacing: '0.06em',
            textShadow: '0 0 4px rgba(0,0,0,0.7), 0 1px 2px rgba(0,0,0,0.5)',
            pointerEvents: 'none',
          }}
        >
          {CATEGORY_LABELS[data.category]}
        </div>
      )}

      <div className="mm-node-label">{data.title}</div>

      <Handle
        type="source"
        position={Position.Bottom}
        style={{ background: 'transparent', border: 'none', width: 0, height: 0 }}
      />
    </motion.div>
  );
}

export const RuleNode = memo(RuleNodeImpl);
