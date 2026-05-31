-- 001_init.sql: Create all core tables for Shadow

CREATE TABLE IF NOT EXISTS rules (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    scope TEXT NOT NULL CHECK(scope IN ('global', 'project')),
    project_path TEXT,
    tags TEXT DEFAULT '[]',
    category TEXT,
    trigger_context TEXT,
    confidence REAL DEFAULT 0.0,
    status TEXT NOT NULL DEFAULT 'candidate' CHECK(status IN ('candidate', 'active', 'disabled', 'conflicted')),
    version INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_rules_scope ON rules(scope);
CREATE INDEX IF NOT EXISTS idx_rules_status ON rules(status);
CREATE INDEX IF NOT EXISTS idx_rules_project ON rules(project_path);
CREATE INDEX IF NOT EXISTS idx_rules_category ON rules(category);

CREATE TABLE IF NOT EXISTS sources (
    id TEXT PRIMARY KEY,
    rule_id TEXT NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    signal_type TEXT NOT NULL CHECK(signal_type IN ('explicit_instruction', 'manual_edit', 'git_revert', 'manual_mark', 'import', 'repetition')),
    signal_strength TEXT NOT NULL CHECK(signal_strength IN ('strong', 'medium', 'weak')),
    agent_name TEXT,
    project_path TEXT,
    raw_snippet TEXT,
    timestamp DATETIME NOT NULL DEFAULT (datetime('now')),
    confidence_contribution REAL DEFAULT 0.0
);

CREATE INDEX IF NOT EXISTS idx_sources_rule_id ON sources(rule_id);
CREATE INDEX IF NOT EXISTS idx_sources_signal_type ON sources(signal_type);
CREATE INDEX IF NOT EXISTS idx_sources_timestamp ON sources(timestamp);

CREATE TABLE IF NOT EXISTS versions (
    id TEXT PRIMARY KEY,
    rule_id TEXT NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    content TEXT NOT NULL,
    diff TEXT,
    changed_by TEXT NOT NULL CHECK(changed_by IN ('user', 'auto')),
    change_reason TEXT,
    timestamp DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_versions_rule_id ON versions(rule_id);
CREATE INDEX IF NOT EXISTS idx_versions_rule_version ON versions(rule_id, version);

CREATE TABLE IF NOT EXISTS config (
    key TEXT NOT NULL,
    value TEXT,
    scope TEXT NOT NULL DEFAULT 'global',
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (key, scope)
);

CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    agents TEXT DEFAULT '[]',
    last_scan_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_projects_path ON projects(path);
