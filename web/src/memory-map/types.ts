// Memory Map 类型定义
// 设计原则：3 类别 / 3 状态 / 关联线筛子（做减法）
// 关联线不是"全都显示"，而是"智能沉默"——默认只显示真正有意义的连接

export type Category = 'code' | 'architecture' | 'practice';

export type RuleStatus = 'active' | 'conflicted' | 'other';

// 关联层级（4 层筛子的前 3 层；第 4 层"隐藏"不渲染）
// signal:    冲突/进化/覆盖 — 永远显示，这些是"警报"不是信息
// structure: 强关联 — 默认显示每节点 top-3，画面呼吸
// whisper:   弱关联 — 按需显示，用户主动"请求"才看见
export type EdgeTier = 'signal' | 'structure' | 'whisper';

// 信号线子类型（仅 signal tier 才有意义）
export type SignalType = 'conflict' | 'evolution' | 'override';

// 旧字段向后兼容（mock / 旧 API 可能仍返回 kind）
export type RelationKind = 'strong' | 'medium' | 'weak';

export interface MemoryNodeData {
  id: string;
  title: string;
  content: string;
  category: Category;
  status: RuleStatus;
  confidence: number;        // 0-1
  version: number;
  tags: string[];
  trigger_context: string;
  project_path: string;
  agents: string[];          // 来源 agent
  hit_count: number;         // 命中次数
  created_at: string;
  updated_at: string;
  source_snippet?: string;   // 原始纠正片段（level 4 详情用）
  [key: string]: unknown;    // React Flow 要求
}

export interface RelationData {
  tier?: EdgeTier;           // 关联层级（新系统主字段）
  signalType?: SignalType;   // 仅 signal tier
  score?: number;            // 关联分数 0-1，决定 tier 内排序
  reason: string;            // 共享标签/项目/冲突原因
  // 旧字段（mock / 旧 API 向后兼容，RelationEdge 会自动降级映射到 tier）
  kind?: RelationKind;
  [key: string]: unknown;
}

export interface MapFilters {
  category: Category | 'all';
  status: RuleStatus | 'all';
  agent: string | 'all';
  // 关联密度滑块 0-1，控制渐进式披露（做减法的核心交互）
  // 0.0   : 只显示信号线（最纯粹）
  // 0.4   : 信号 + 每节点 top-3 结构线（默认）
  // 0.7   : 信号 + 全部结构 + 每节点 top-2 低语线
  // 1.0   : 全部非隐藏线（完整视图）
  edgeDensity: number;
}

export interface GrowthData {
  level: number;             // 当前等级 1-6
  progress: number;          // 0-1 进度
  nextLevelAt: number;       // 下一级需要多少条
  achievements: Achievement[];
}

export interface Achievement {
  id: string;
  label: string;
  unlocked: boolean;
  icon: string;
}

export interface ClusterStat {
  category: Category;
  count: number;
  active: number;
  conflicted: number;
}

export interface MapStats {
  total: number;
  active: number;
  conflicted: number;
  other: number;
  thisMonth: number;
  hitRatePct?: number;
  recurrenceProxyPct?: number;
  growth: GrowthData;
  byCategory: ClusterStat[];
  edgeStats?: EdgeStats;   // 关联线分层统计（做减法透明度）
}

export interface EdgeStats {
  signal: number;          // 永远显示
  structure: number;       // 默认 top-3
  whisper: number;         // 按需显示
  hidden: number;          // 永不显示
}
