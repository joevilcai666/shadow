// 浮动 HUD — 统计 + 搜索 + 筛选 + 成长进度条
// 毛玻璃 + 顶部居中

import { useState, useRef, useEffect } from 'react';
import { Search, X, Filter } from 'lucide-react';
import type { MapStats, MapFilters, Category } from './types';

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
  const [openDropdown, setOpenDropdown] = useState<'status' | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!openDropdown) return;
    function handle(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpenDropdown(null);
      }
    }
    document.addEventListener('mousedown', handle);
    return () => document.removeEventListener('mousedown', handle);
  }, [openDropdown]);

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

        <div style={{ flex: 1 }} />

        {/* 成长进度条 */}
        <div className="mm-hud-growth" style={{ minWidth: 200, flex: '0 1 280px' }}>
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
          <input
            className="mm-hud-search-input"
            placeholder="搜索记忆 · 标签 · 项目..."
            value={searchQuery}
            onChange={e => onSearchChange(e.target.value)}
            aria-label="搜索记忆"
          />
          {searchQuery && (
            <button
              onClick={() => onSearchChange('')}
              style={{
                background: 'transparent',
                border: 'none',
                color: 'var(--text-tertiary)',
                cursor: 'pointer',
                padding: 2,
                display: 'flex',
              }}
              aria-label="清除搜索"
            >
              <X size={12} />
            </button>
          )}
          {searchQuery && (
            <span className="mm-hud-search-count">
              {searchMatchCount} 匹配
            </span>
          )}
        </div>

        <div className="mm-hud-chips">
          {CATEGORIES.map(c => (
            <button
              key={c.key}
              className={`mm-chip ${filters.category === c.key ? 'mm-chip--active' : ''}`}
              onClick={() => onFilterChange({ ...filters, category: c.key })}
            >
              {c.cls && <span className={`mm-chip-dot ${c.cls}`} />}
              {c.label}
            </button>
          ))}
        </div>

        <div ref={dropdownRef} style={{ position: 'relative' }}>
          <button
            className={`mm-chip ${filters.status !== 'all' ? 'mm-chip--active' : ''}`}
            onClick={() => setOpenDropdown(openDropdown === 'status' ? null : 'status')}
            aria-expanded={openDropdown === 'status'}
          >
            <Filter size={11} />
            {STATUS_OPTIONS.find(s => s.key === filters.status)?.label}
          </button>
          {openDropdown === 'status' && (
            <div
              style={{
                position: 'absolute',
                top: 'calc(100% + 6px)',
                right: 0,
                background: 'var(--surface-strong)',
                backdropFilter: 'blur(16px)',
                WebkitBackdropFilter: 'blur(16px)',
                border: '1px solid var(--border)',
                borderRadius: 8,
                padding: 4,
                display: 'flex',
                flexDirection: 'column',
                gap: 2,
                minWidth: 120,
                zIndex: 50,
                boxShadow: '0 8px 24px rgba(0,0,0,0.4)',
                animation: 'mm-hud-in 200ms cubic-bezier(0.16, 1, 0.3, 1) both',
              }}
            >
              {STATUS_OPTIONS.map(s => (
                <button
                  key={s.key}
                  className="mm-chip"
                  style={{
                    justifyContent: 'flex-start',
                    width: '100%',
                  }}
                  onClick={() => {
                    onFilterChange({ ...filters, status: s.key });
                    setOpenDropdown(null);
                  }}
                >
                  {s.label}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
