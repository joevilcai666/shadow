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

  // 2. 初始化位置：每个节点在类别中心周围均匀分布
  const pos: Record<string, { x: number; y: number }> = {};
  for (const [cat, list] of Object.entries(byCategory)) {
    const center = CATEGORY_CENTERS[cat as keyof typeof CATEGORY_CENTERS];
    const radius = 140;
    const n = list.length;
    list.forEach((node, i) => {
      // 螺旋分布（避免重叠）
      const angle = (i / n) * Math.PI * 2 + (cat === 'code' ? 0 : cat === 'architecture' ? 0.4 : 0.8);
      const r = radius + (i % 3) * 24;
      pos[node.id] = {
        x: center.x + Math.cos(angle) * r,
        y: center.y + Math.sin(angle) * r,
      };
    });
  }

  // 3. 极简 force 迭代：节点排斥 + 关联吸引
  const iterations = 300;
  const repulsion = 4200;          // 排斥力（提高，避免重叠）
  const attraction = 0.010;        // 关联吸引（降低，让节点不被拉太近）
  const damping = 0.7;
  const minDist = 75;              // 最小间距（提高，容纳大节点）

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

    // 类别中心吸引（轻）
    for (const n of nodes) {
      const c = CATEGORY_CENTERS[n.category];
      const p = pos[n.id];
      forces[n.id].fx += (c.x - p.x) * 0.0008;
      forces[n.id].fy += (c.y - p.y) * 0.0008;
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

    // 应用 + 阻尼
    for (const n of nodes) {
      const p = pos[n.id];
      const f = forces[n.id];
      p.x += f.fx * damping;
      p.y += f.fy * damping;
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
