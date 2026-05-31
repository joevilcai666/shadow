package storage

import "time"

// Rule represents a distilled rule in Shadow's memory layer.
type Rule struct {
	ID              string   `json:"id"`
	Content         string   `json:"content"`
	Scope           string   `json:"scope"` // "global" | "project"
	ProjectPath     string   `json:"project_path,omitempty"`
	Tags            []string `json:"tags"`
	Category        string   `json:"category,omitempty"`
	TriggerContext  string   `json:"trigger_context,omitempty"`
	Confidence      float64  `json:"confidence"`
	Status          string   `json:"status"` // "candidate" | "active" | "disabled" | "conflicted"
	Version         int      `json:"version"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// Source traces the origin of a rule — which signal generated it.
type Source struct {
	ID                    string  `json:"id"`
	RuleID                string  `json:"rule_id"`
	SignalType            string  `json:"signal_type"`
	SignalStrength        string  `json:"signal_strength"` // "strong" | "medium" | "weak"
	AgentName             string  `json:"agent_name,omitempty"`
	ProjectPath           string  `json:"project_path,omitempty"`
	RawSnippet            string  `json:"raw_snippet,omitempty"`
	Timestamp             string  `json:"timestamp"`
	ConfidenceContribution float64 `json:"confidence_contribution"`
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
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Agents     []string  `json:"agents"`
	LastScanAt *string   `json:"last_scan_at,omitempty"`
	CreatedAt  string    `json:"created_at"`
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
