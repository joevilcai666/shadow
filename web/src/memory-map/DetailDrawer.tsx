// 侧边详情抽屉 — 选中节点时显示
// 包含：标题、类别徽章、内容、置信度、触发情境、标签、来源 agent、纠正溯源、关联记忆

import { X, Edit2, Eye, GitBranch, Clock } from 'lucide-react';
import type { MemoryNodeData, Category } from './types';

interface Props {
  node: MemoryNodeData | null;
  onClose: () => void;
  onOpenInRules: (id: string) => void;
}

const CATEGORY_LABELS: Record<Category, string> = {
  code: 'CODE',
  architecture: 'ARCH',
  practice: 'PRACTICE',
};

const STATUS_LABELS = {
  active: 'Active',
  conflicted: 'Conflicted',
  other: 'Other',
};

export function DetailDrawer({ node, onClose, onOpenInRules }: Props) {
  if (!node) return null;

  return (
    <div className="mm-drawer" role="dialog" aria-labelledby="drawer-title">
      <div className="mm-drawer-header">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <span className={`mm-drawer-cat-badge mm-drawer-cat-badge--${node.category}`}>
              {CATEGORY_LABELS[node.category]}
            </span>
            <span style={{ fontSize: 10, color: 'var(--text-tertiary)' }}>
              · {STATUS_LABELS[node.status]} · v{node.version}
            </span>
          </div>
          <h2 id="drawer-title" className="mm-drawer-title">
            {node.title}
          </h2>
        </div>
        <button className="mm-drawer-close" onClick={onClose} aria-label="关闭">
          <X size={18} />
        </button>
      </div>

      <div className="mm-drawer-body">
        {/* 规则正文 */}
        <section>
          <div className="mm-drawer-section-label">规则</div>
          <div className="mm-drawer-content">{node.content}</div>
        </section>

        {/* 置信度 */}
        <section>
          <div className="mm-drawer-section-label">置信度</div>
          <div className="mm-confidence">
            <div className="mm-confidence-bar">
              <div
                className="mm-confidence-fill"
                style={{ width: `${node.confidence * 100}%` }}
              />
            </div>
            <span className="mm-confidence-value">{node.confidence.toFixed(2)}</span>
          </div>
        </section>

        {/* 元数据 */}
        <section>
          <div className="mm-drawer-section-label">元数据</div>
          <div className="mm-drawer-meta">
            <span className="mm-drawer-meta-label">触发情境</span>
            <span className="mm-drawer-meta-value">{node.trigger_context}</span>
            <span className="mm-drawer-meta-label">项目</span>
            <span className="mm-drawer-meta-value" style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 11 }}>
              {node.project_path}
            </span>
            <span className="mm-drawer-meta-label">命中</span>
            <span className="mm-drawer-meta-value">{node.hit_count} 次</span>
            <span className="mm-drawer-meta-label">创建</span>
            <span className="mm-drawer-meta-value">{node.created_at}</span>
            <span className="mm-drawer-meta-label">更新</span>
            <span className="mm-drawer-meta-value">{node.updated_at}</span>
          </div>
        </section>

        {/* 标签 */}
        {node.tags.length > 0 && (
          <section>
            <div className="mm-drawer-section-label">标签</div>
            <div className="mm-tag-list">
              {node.tags.map(t => (
                <span key={t} className="mm-tag">#{t}</span>
              ))}
            </div>
          </section>
        )}

        {/* 来源 agent */}
        {node.agents.length > 0 && (
          <section>
            <div className="mm-drawer-section-label">来源</div>
            <div className="mm-agents">
              {node.agents.map(a => (
                <span key={a} className="mm-agent-pill">{a}</span>
              ))}
            </div>
          </section>
        )}

        {/* 纠正溯源（合并 Level 4 内容到抽屉） */}
        {node.source_snippet && (
          <section>
            <div className="mm-drawer-section-label">原始纠正</div>
            <div className="mm-source-card">
              <div className="mm-source-meta">
                <Clock size={11} />
                <span>{node.updated_at}</span>
                <span>·</span>
                <span style={{ color: 'var(--brand)' }}>{node.agents[0] ?? '—'}</span>
              </div>
              <div className="mm-source-snippet">{node.source_snippet}</div>
            </div>
          </section>
        )}

        {/* 操作 */}
        <section>
          <div className="mm-drawer-section-label">操作</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <button
              className="mm-chip"
              style={{ justifyContent: 'flex-start', padding: '8px 12px' }}
              onClick={() => onOpenInRules(node.id)}
            >
              <Edit2 size={12} />
              <span>编辑规则</span>
            </button>
            <button
              className="mm-chip"
              style={{ justifyContent: 'flex-start', padding: '8px 12px' }}
              onClick={() => onOpenInRules(node.id)}
            >
              <GitBranch size={12} />
              <span>查看版本历史</span>
            </button>
            <button
              className="mm-chip"
              style={{ justifyContent: 'flex-start', padding: '8px 12px' }}
              onClick={() => onOpenInRules(node.id)}
            >
              <Eye size={12} />
              <span>在 Rules 页打开 →</span>
            </button>
          </div>
        </section>
      </div>
    </div>
  );
}
