package storage

import "time"

// Rule represents a distilled rule in Shadow's memory layer.
type Rule struct {
	ID             string   `json:"id"`
	Content        string   `json:"content"`
	Scope          string   `json:"scope"` // "global" | "project"
	ProjectPath    string   `json:"project_path,omitempty"`
	Tags           []string `json:"tags"`
	Category       string   `json:"category,omitempty"`
	TriggerContext string   `json:"trigger_context,omitempty"`
	Confidence     float64  `json:"confidence"`
	Status         string   `json:"status"` // "candidate" | "active" | "disabled" | "conflicted"
	Version        int      `json:"version"`
	// 借鉴 Shadow Brain：Source-backed memory + 置信度
	Importance  float64  `json:"importance"`   // 0-1, 用户指定或系统计算
	DecayScore  float64  `json:"decay_score"`  // 30天半衰期衰减后的当前置信度
	LastHitAt   string   `json:"last_hit_at"`  // 上次命中时间，用于计算衰减
	SourcePaths []string `json:"source_paths"` // 证据文件/路径，反幻觉锚点
	Author      string   `json:"author"`       // "user" | "agent" | "system"
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// Source traces the origin of a rule — which signal generated it.
type Source struct {
	ID                     string  `json:"id"`
	RuleID                 string  `json:"rule_id"`
	SignalType             string  `json:"signal_type"`
	SignalStrength         string  `json:"signal_strength"` // "strong" | "medium" | "weak"
	AgentName              string  `json:"agent_name,omitempty"`
	ProjectPath            string  `json:"project_path,omitempty"`
	RawSnippet             string  `json:"raw_snippet,omitempty"`
	Timestamp              string  `json:"timestamp"`
	ConfidenceContribution float64 `json:"confidence_contribution"`
}

// Event records whether a memory actually propagated or fired.
type Event struct {
	ID          string `json:"id"`
	RuleID      string `json:"rule_id,omitempty"`
	EventType   string `json:"event_type"` // "rule_hit" | "sync_success" | "sync_failure"
	AgentName   string `json:"agent_name,omitempty"`
	ProjectPath string `json:"project_path,omitempty"`
	TargetPath  string `json:"target_path,omitempty"`
	Details     string `json:"details,omitempty"`
	Timestamp   string `json:"timestamp"`
}

// Version stores a snapshot of a rule at a point in time.
type Version struct {
	ID           string `json:"id"`
	RuleID       string `json:"rule_id"`
	Version      int    `json:"version"`
	Content      string `json:"content"`
	Diff         string `json:"diff,omitempty"`
	ChangedBy    string `json:"changed_by"` // "user" | "auto"
	ChangeReason string `json:"change_reason,omitempty"`
	Timestamp    string `json:"timestamp"`
}

// ConfigEntry represents a key-value configuration entry.
type ConfigEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Scope     string `json:"scope"` // "global" | project path
	UpdatedAt string `json:"updated_at"`
}

// Project represents a registered project in Shadow.
type Project struct {
	ID         string   `json:"id"`
	Path       string   `json:"path"`
	Name       string   `json:"name"`
	Agents     []string `json:"agents"`
	LastScanAt *string  `json:"last_scan_at,omitempty"`
	CreatedAt  string   `json:"created_at"`
}

// 借鉴 EverOS：Two-track memory split
// UserMemory — 用户个人上下文（跨 agent 共享）
type UserMemory struct {
	ID          string   `json:"id"`
	UserID      string   `json:"user_id"`  // 跨 agent 共享
	Content     string   `json:"content"`  // markdown 格式
	Category    string   `json:"category"` // "preference" | "convention" | "context"
	ProjectPath string   `json:"project_path,omitempty"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// AgentMemory — agent procedural memory（从 usage 提取）
type AgentMemory struct {
	ID            string `json:"id"`
	AgentName     string `json:"agent_name"` // "codex" | "claude-code" | "cursor" | "openclaw"
	Content       string `json:"content"`
	ExtractedFrom string `json:"extracted_from"` // event_id 或 task_id
	CreatedAt     string `json:"created_at"`
}

// SessionMemory — 单次 coding session 的上下文快照
type SessionMemory struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	AgentName   string `json:"agent_name"`
	ProjectPath string `json:"project_path"`
	ContextDump string `json:"context_dump"` // 该 session 的完整 context 快照
	TaskSummary string `json:"task_summary"` // 用户描述的任务概要
	CreatedAt   string `json:"created_at"`
	EndedAt     string `json:"ended_at,omitempty"`
}

// MemoryCard — 记忆卡（用于 UI 展示）
type MemoryCard struct {
	Rule           Rule    `json:"rule"`
	HitCount       int     `json:"hit_count"`
	DecayScore     float64 `json:"decay_score"`
	ConfirmedBy    int     `json:"confirmed_by"`     // 有多少个不同 agent 命中过
	NextBestAction string  `json:"next_best_action"` // "keep" | "demote" | "delete" | "merge"
}

// RuleFilter defines query parameters for filtering rules.
type RuleFilter struct {
	Scope       string   `json:"scope,omitempty"`
	Status      string   `json:"status,omitempty"`
	ProjectPath string   `json:"project_path,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Search      string   `json:"search,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	Offset      int      `json:"offset,omitempty"`
	OrderBy     string   `json:"order_by,omitempty"` // "confidence" | "updated_at" | "created_at"
}

// Storage provides all repository interfaces.
type Storage struct {
	db DB
}

// DB abstracts database operations for testability.
type DB interface {
	Exec(query string, args ...any) (Result, error)
	Query(query string, args ...any) (Rows, error)
	QueryRow(query string, args ...any) Row
}

// Result abstracts sql.Result.
type Result interface {
	LastInsertId() (int64, error)
	RowsAffected() (int64, error)
}

// Rows abstracts sql.Rows.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
}

// Row abstracts sql.Row.
type Row interface {
	Scan(dest ...any) error
}

// Now returns current UTC timestamp string.
func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
