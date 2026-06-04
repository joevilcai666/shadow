// Memory Map 类型定义
// 设计原则：3 类别 / 3 状态 / 3 关联线 — Steve Jobs 「少即是多」

export type Category = 'code' | 'architecture' | 'practice';

export type RuleStatus = 'active' | 'conflicted' | 'other';

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
  kind: RelationKind;
  reason: string;            // 共享标签/项目/语义
  [key: string]: unknown;
}

export interface MapFilters {
  category: Category | 'all';
  status: RuleStatus | 'all';
  agent: string | 'all';
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
  growth: GrowthData;
  byCategory: ClusterStat[];
}
