import { X, Edit2, Eye, GitBranch, Clock } from 'lucide-react';
import type { MemoryNodeData, Category } from './types';
import { IconButton, ShadowButton, StatusChip, TagChip } from '../components/ui';

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

export function DetailDrawer({ node, onClose, onOpenInRules }: Props) {
  if (!node) return null;

  return (
    <div className="mm-drawer" role="dialog" aria-labelledby="drawer-title">
      <div className="mm-drawer-header">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <TagChip className={`mm-drawer-cat-badge mm-drawer-cat-badge--${node.category}`}>
              {CATEGORY_LABELS[node.category]}
            </TagChip>
            <StatusChip status={node.status === 'other' ? 'candidate' : node.status} />
            <TagChip>v{node.version}</TagChip>
          </div>
          <h2 id="drawer-title" className="mm-drawer-title">
            {node.title}
          </h2>
        </div>
        <IconButton className="mm-drawer-close" onClick={onClose} label="Close" tone="subtle">
          <X size={18} />
        </IconButton>
      </div>

      <div className="mm-drawer-body">
        <section>
          <div className="mm-drawer-section-label">Rule</div>
          <div className="mm-drawer-content">{node.content}</div>
        </section>

        <section>
          <div className="mm-drawer-section-label">Confidence</div>
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

        <section>
          <div className="mm-drawer-section-label">Metadata</div>
          <div className="mm-drawer-meta">
            <span className="mm-drawer-meta-label">Trigger</span>
            <span className="mm-drawer-meta-value">{node.trigger_context}</span>
            <span className="mm-drawer-meta-label">Project</span>
            <span className="mm-drawer-meta-value" style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 11 }}>
              {node.project_path}
            </span>
            <span className="mm-drawer-meta-label">Hits</span>
            <span className="mm-drawer-meta-value">{node.hit_count} times</span>
            <span className="mm-drawer-meta-label">Created</span>
            <span className="mm-drawer-meta-value">{node.created_at}</span>
            <span className="mm-drawer-meta-label">Updated</span>
            <span className="mm-drawer-meta-value">{node.updated_at}</span>
          </div>
        </section>

        {node.tags.length > 0 && (
          <section>
            <div className="mm-drawer-section-label">Tags</div>
            <div className="mm-tag-list">
              {node.tags.map(t => (
                <TagChip key={t} className="mm-tag">#{t}</TagChip>
              ))}
            </div>
          </section>
        )}

        {node.agents.length > 0 && (
          <section>
            <div className="mm-drawer-section-label">Sources</div>
            <div className="mm-agents">
              {node.agents.map(a => (
                <TagChip key={a} className="mm-agent-pill">{a}</TagChip>
              ))}
            </div>
          </section>
        )}

        {node.source_snippet && (
          <section>
            <div className="mm-drawer-section-label">Original correction</div>
            <div className="mm-source-card">
              <div className="mm-source-meta">
                <Clock size={11} />
                <span>{node.updated_at}</span>
                <span>-</span>
                <span style={{ color: 'var(--brand)' }}>{node.agents[0] ?? '-'}</span>
              </div>
              <div className="mm-source-snippet">{node.source_snippet}</div>
            </div>
          </section>
        )}

        <section>
          <div className="mm-drawer-section-label">Actions</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <ShadowButton
              className="mm-chip justify-start px-3 py-2"
              onClick={() => onOpenInRules(node.id)}
              tone="subtle"
            >
              <Edit2 size={12} />
              <span>Edit rule</span>
            </ShadowButton>
            <ShadowButton
              className="mm-chip justify-start px-3 py-2"
              onClick={() => onOpenInRules(node.id)}
              tone="subtle"
            >
              <GitBranch size={12} />
              <span>View version history</span>
            </ShadowButton>
            <ShadowButton
              className="mm-chip justify-start px-3 py-2"
              onClick={() => onOpenInRules(node.id)}
              tone="subtle"
            >
              <Eye size={12} />
              <span>Open in Rules -&gt;</span>
            </ShadowButton>
          </div>
        </section>
      </div>
    </div>
  );
}
