// 浮动 HUD — 统计 + 搜索 + 筛选 + 关联密度 + 成长进度条
// 毛玻璃 + 顶部居中

import { Input, Dropdown } from '@heroui/react';
import { Search, X, Filter } from 'lucide-react';
import type { MapStats, MapFilters, Category } from './types';
import { ShadowButton } from '../components/ui';

// 密度滑块标签：把 0-1 数值映射成人类可读的层级名
function densityLabel(d: number): string {
  if (d < 0.05) return '隐藏';
  if (d < 0.2) return '仅信号';
  if (d < 0.5) return '结构';
  if (d < 0.6) return '结构全';
  if (d < 0.9) return '低语';
  return '全部';
}

const CATEGORIES: Array<{ key: Category | 'all'; label: string; cls: string }> = [
  { key: 'all', label: '全部', cls: '' },
  { key: 'code', label: 'Code', cls: 'mm-chip-dot--code' },
  { key: 'architecture', label: 'Arch', cls: 'mm-chip-dot--architecture' },
  { key: 'practice', label: 'Practice', cls: 'mm-chip-dot--practice' },
];

const STATUS_OPTIONS: Array<{ key: 'all' | 'active' | 'conflicted' | 'other'; label: string }> = [
  { key: 'all', label: '全部状态' },
  { key: 'active', label: 'Active' },
  { key: 'conflicted', label: 'Conflicted' },
  { key: 'other', label: '其他' },
];

interface Props {
  stats: MapStats;
  filters: MapFilters;
  searchQuery: string;
  searchMatchCount: number;
  onSearchChange: (q: string) => void;
  onFilterChange: (f: MapFilters) => void;
}

export function Hud({
  stats,
  filters,
  searchQuery,
  searchMatchCount,
  onSearchChange,
  onFilterChange,
}: Props) {
  const activeRate = stats.total > 0 ? (stats.active / stats.total * 100).toFixed(0) : '0';

  return (
    <div className="mm-hud" role="region" aria-label="记忆地图顶部面板">
      {/* 统计行 */}
      <div className="mm-hud-stats">
        <div className="mm-hud-stat">
          <span className="mm-hud-stat-value">{stats.total}</span>
          <span>条记忆</span>
        </div>
        <div className="mm-hud-divider" />
        <div className="mm-hud-stat">
          <span className="mm-hud-stat-value mm-hud-stat-value--accent">{activeRate}%</span>
          <span>活跃</span>
        </div>
        <div className="mm-hud-divider" />
        <div className="mm-hud-stat">
          <span className="mm-hud-stat-value">+{stats.thisMonth}</span>
          <span>本月</span>
        </div>
        {stats.conflicted > 0 && (
          <>
            <div className="mm-hud-divider" />
            <div className="mm-hud-stat">
              <span className="mm-hud-stat-value mm-hud-stat-value--warn">{stats.conflicted}</span>
              <span>冲突</span>
            </div>
          </>
        )}

        <div className="mm-hud-spacer" />

        {/* 成长进度条 */}
        <div className="mm-hud-growth">
          <span style={{ fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' }}>
            Lv.{stats.growth.level} · {stats.total}/{stats.growth.nextLevelAt}
          </span>
          <div className="mm-hud-growth-bar">
            <div
              className="mm-hud-growth-fill"
              style={{ width: `${stats.growth.progress * 100}%` }}
            />
          </div>
        </div>

        <div className="mm-hud-achievements" aria-label="成就">
          {stats.growth.achievements.map(a => (
            <span
              key={a.id}
              className={`mm-hud-achievement ${a.unlocked ? 'mm-hud-achievement--unlocked' : ''}`}
              title={a.label}
              aria-label={`${a.label} ${a.unlocked ? '已解锁' : '未解锁'}`}
            >
              {a.icon}
            </span>
          ))}
        </div>
      </div>

      {/* 搜索行 */}
      <div className="mm-hud-search-row">
        <div className="mm-hud-search">
          <Search size={14} className="mm-hud-search-icon" />
          <Input
            className="mm-hud-search-input"
            placeholder="搜索记忆 · 标签 · 项目..."
            value={searchQuery}
            onChange={e => onSearchChange(e.target.value)}
            aria-label="搜索记忆"
          />
          {searchQuery && (
            <ShadowButton
              onClick={() => onSearchChange('')}
              tone="subtle"
              className="h-6 min-h-6 w-6 min-w-6 p-0 text-[var(--text-tertiary)]"
              aria-label="清除搜索"
            >
              <X size={12} />
            </ShadowButton>
          )}
          {searchQuery && (
            <span className="mm-hud-search-count">
              {searchMatchCount} 匹配
            </span>
          )}
        </div>

        <div className="mm-hud-chips">
          {CATEGORIES.map(c => (
            <ShadowButton
              key={c.key}
              className={`mm-chip ${filters.category === c.key ? 'mm-chip--active' : ''}`}
              onClick={() => onFilterChange({ ...filters, category: c.key })}
              tone="subtle"
            >
              {c.cls && <span className={`mm-chip-dot ${c.cls}`} />}
              {c.label}
            </ShadowButton>
          ))}
        </div>

        <Dropdown>
          <Dropdown.Trigger>
            <ShadowButton
            className={`mm-chip ${filters.status !== 'all' ? 'mm-chip--active' : ''}`}
            tone="subtle"
          >
            <Filter size={11} />
            {STATUS_OPTIONS.find(s => s.key === filters.status)?.label}
          </ShadowButton>
          </Dropdown.Trigger>
          <Dropdown.Popover className="z-50 min-w-32 rounded-lg border border-[var(--border)] bg-[var(--surface-strong)] p-1 shadow-2xl backdrop-blur">
            <Dropdown.Menu onAction={(key) => onFilterChange({ ...filters, status: key as MapFilters['status'] })}>
              {STATUS_OPTIONS.map(s => (
                <Dropdown.Item key={s.key} id={s.key} className="mm-chip w-full justify-start">
                  {s.label}
                </Dropdown.Item>
              ))}
            </Dropdown.Menu>
          </Dropdown.Popover>
        </Dropdown>

        {/* 关联密度滑块（做减法核心交互：单一滑块控制渐进式披露）*/}
        <div className="mm-density" title="关联密度：拖动揭示更多关联线">
          <span className="mm-density-label">关联</span>
          <input
            type="range"
            min={0}
            max={1}
            step={0.05}
            value={filters.edgeDensity}
            onChange={e => onFilterChange({ ...filters, edgeDensity: parseFloat(e.target.value) })}
            className="mm-density-slider"
            aria-label="关联密度"
          />
          <span className="mm-density-value">{densityLabel(filters.edgeDensity)}</span>
        </div>
      </div>
    </div>
  );
}
