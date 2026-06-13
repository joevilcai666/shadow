-- 004_memory_models.sql: Shadow Brain fields + Two-track memory tables
--
-- Adds:
--   - Rule new columns: importance, decay_score, last_hit_at, source_paths, author
--   - user_memories: user personal context (跨 agent 共享)
--   - agent_memories: agent procedural memory (从 usage 提取)
--   - session_memories: single-session context snapshot
--   - events index on rule_id for hit rate queries

-- Add new columns to rules table (SQLite doesn't support ADD COLUMN for some cases, use rebuild)
PRAGMA foreign_keys=OFF;

CREATE TABLE IF NOT EXISTS rules_new (
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
    importance REAL DEFAULT 0.5,
    decay_score REAL DEFAULT 0.0,
    last_hit_at TEXT,
    source_paths TEXT DEFAULT '[]',
    author TEXT DEFAULT 'agent' CHECK(author IN ('user', 'agent', 'system')),
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Ensure empty author defaults to 'agent' so existing rows pass the CHECK constraint
CREATE TRIGGER IF NOT EXISTS rules_default_author
BEFORE INSERT ON rules_new
WHEN (new.author IS NULL OR new.author = '')
BEGIN
    UPDATE rules_new SET author = 'agent' WHERE id = new.id;
END;

INSERT INTO rules_new (id, content, scope, project_path, tags, category, trigger_context,
                      confidence, status, version, created_at, updated_at)
SELECT id, content, scope, project_path, tags, category, trigger_context,
       confidence, status, version, created_at, updated_at
FROM rules;

DROP TABLE rules;
ALTER TABLE rules_new RENAME TO rules;

CREATE INDEX IF NOT EXISTS idx_rules_scope ON rules(scope);
CREATE INDEX IF NOT EXISTS idx_rules_status ON rules(status);
CREATE INDEX IF NOT EXISTS idx_rules_project ON rules(project_path);
CREATE INDEX IF NOT EXISTS idx_rules_category ON rules(category);
CREATE INDEX IF NOT EXISTS idx_rules_decay ON rules(decay_score);

-- UserMemory: user personal context (跨 agent 共享)
CREATE TABLE IF NOT EXISTS user_memories (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    content TEXT NOT NULL,
    category TEXT NOT NULL CHECK(category IN ('preference', 'convention', 'context')),
    project_path TEXT,
    tags TEXT DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_user_memories_user_id ON user_memories(user_id);
CREATE INDEX IF NOT EXISTS idx_user_memories_project ON user_memories(project_path);

-- AgentMemory: agent procedural memory (从 usage 提取)
CREATE TABLE IF NOT EXISTS agent_memories (
    id TEXT PRIMARY KEY,
    agent_name TEXT NOT NULL,
    content TEXT NOT NULL,
    extracted_from TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_agent_memories_agent ON agent_memories(agent_name);

-- SessionMemory: single-session context snapshot
CREATE TABLE IF NOT EXISTS session_memories (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    agent_name TEXT NOT NULL,
    project_path TEXT NOT NULL,
    context_dump TEXT NOT NULL DEFAULT '',
    task_summary TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    ended_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_session_memories_session ON session_memories(session_id);
CREATE INDEX IF NOT EXISTS idx_session_memories_project ON session_memories(project_path);
CREATE INDEX IF NOT EXISTS idx_session_memories_agent ON session_memories(agent_name);

PRAGMA foreign_keys=ON;