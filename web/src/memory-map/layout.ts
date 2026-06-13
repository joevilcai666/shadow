// 极简 force-directed 布局 — 不引入 d3-force 包
// 3 类别吸引中心 + 同类散布 + 节点排斥
// 一次性预计算，输出稳定坐标

import type { MemoryNodeData } from './types';

const CATEGORY_CENTERS = {
  code: { x: -260, y: -120 },
  architecture: { x: 260, y: -120 },
  practice: { x: 0, y: 240 },
} as const;

interface LayoutResult {
  positions: Record<string, { x: number; y: number }>;
}

export function computeLayout(
  nodes: MemoryNodeData[],
  relations: Array<{ source: string; target: string }>
): LayoutResult {
  // 1. 按类别分组
  const byCategory: Record<string, MemoryNodeData[]> = {
    code: [],
    architecture: [],
    practice: [],
  };
  for (const n of nodes) byCategory[n.category].push(n);

  // 2. 初始化位置：每个节点在类别中心周围分布
  //    节点多时增大散布半径，避免初始状态太密导致排斥爆炸
  const pos: Record<string, { x: number; y: number }> = {};
  const totalNodes = nodes.length;
  const baseRadius = Math.max(240, Math.sqrt(totalNodes) * 28);
  for (const [cat, list] of Object.entries(byCategory)) {
    const center = CATEGORY_CENTERS[cat as keyof typeof CATEGORY_CENTERS];
    const n = list.length || 1;
    list.forEach((node, i) => {
      const angle = (i / n) * Math.PI * 2 + (cat === 'code' ? 0 : cat === 'architecture' ? 0.4 : 0.8);
      const r = baseRadius * (0.5 + 0.5 * (i / n));
      pos[node.id] = {
        x: center.x + Math.cos(angle) * r,
        y: center.y + Math.sin(angle) * r,
      };
    });
  }

  // 3. Force 迭代：节点排斥 + 关联吸引
  //    迭代数减少（大 n 时足够）、排斥力降低、中心引力增强
  //    坐标上限收紧，避免 SVG 数值溢出
  const iterations = 120;
  const repulsion = 1800;
  const centerGravity = 0.004;
  const attraction = 0.012;
  const damping = 0.65;
  const minDist = 70;
  const maxCoord = 2200;

  // 索引关联
  const adj: Record<string, string[]> = {};
  for (const r of relations) {
    (adj[r.source] ??= []).push(r.target);
    (adj[r.target] ??= []).push(r.source);
  }

  for (let it = 0; it < iterations; it++) {
    const forces: Record<string, { fx: number; fy: number }> = {};
    for (const n of nodes) forces[n.id] = { fx: 0, fy: 0 };

    // 排斥力
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        const a = nodes[i];
        const b = nodes[j];
        const pa = pos[a.id];
        const pb = pos[b.id];
        const dx = pa.x - pb.x;
        const dy = pa.y - pb.y;
        const d2 = dx * dx + dy * dy;
        if (d2 < 1) continue;
        const d = Math.sqrt(d2);
        const force = repulsion / d2;
        const fx = (dx / d) * force;
        const fy = (dy / d) * force;
        forces[a.id].fx += fx;
        forces[a.id].fy += fy;
        forces[b.id].fx -= fx;
        forces[b.id].fy -= fy;
      }
    }

    // 类别中心吸引
    for (const n of nodes) {
      const c = CATEGORY_CENTERS[n.category] ?? CATEGORY_CENTERS.practice;
      const p = pos[n.id];
      forces[n.id].fx += (c.x - p.x) * centerGravity;
      forces[n.id].fy += (c.y - p.y) * centerGravity;
    }

    // 关联吸引
    for (const [src, targets] of Object.entries(adj)) {
      for (const tgt of targets) {
        const ps = pos[src];
        const pt = pos[tgt];
        if (!ps || !pt) continue;
        const dx = pt.x - ps.x;
        const dy = pt.y - ps.y;
        forces[src].fx += dx * attraction;
        forces[src].fy += dy * attraction;
        forces[tgt].fx -= dx * attraction;
        forces[tgt].fy -= dy * attraction;
      }
    }

    // 应用 + 阻尼 + 边界裁剪
    for (const n of nodes) {
      const p = pos[n.id];
      const f = forces[n.id];
      p.x += f.fx * damping;
      p.y += f.fy * damping;
      // 裁剪到安全范围，防止 SVG 坐标溢出
      if (p.x > maxCoord) p.x = maxCoord;
      if (p.x < -maxCoord) p.x = -maxCoord;
      if (p.y > maxCoord) p.y = maxCoord;
      if (p.y < -maxCoord) p.y = -maxCoord;
    }
  }

  // 4. 保证最小间距（最后一遍扫描推开）
  for (let pass = 0; pass < 3; pass++) {
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        const a = nodes[i];
        const b = nodes[j];
        const pa = pos[a.id];
        const pb = pos[b.id];
        const dx = pa.x - pb.x;
        const dy = pa.y - pb.y;
        const d = Math.sqrt(dx * dx + dy * dy) || 0.01;
        if (d < minDist) {
          const push = (minDist - d) / 2;
          const ux = dx / d;
          const uy = dy / d;
          pa.x += ux * push;
          pa.y += uy * push;
          pb.x -= ux * push;
          pb.y -= uy * push;
        }
      }
    }
  }

  return { positions: pos };
}
