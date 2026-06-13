-- 003_events.sql: Effectiveness evidence events for PMF proof chain

CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    rule_id TEXT REFERENCES rules(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL CHECK(event_type IN ('rule_hit', 'sync_success', 'sync_failure')),
    agent_name TEXT,
    project_path TEXT,
    target_path TEXT,
    details TEXT,
    timestamp DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_events_rule_id ON events(rule_id);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_agent_type ON events(agent_name, event_type);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
