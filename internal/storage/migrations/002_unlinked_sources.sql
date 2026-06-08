-- 002_unlinked_sources.sql: Allow sources to exist before a rule is created.
--
-- Sources are captured from agent logs before the distill engine crystallizes
-- them into a rule. The original schema required rule_id NOT NULL with a FK,
-- which caused every captured signal to fail with FOREIGN KEY constraint failed
-- on a fresh database. This migration makes rule_id nullable so the capture
-- path can persist signals immediately; the distill engine later calls
-- SourceRepo.UpdateRuleID to link them.

-- SQLite doesn't support ALTER COLUMN ... DROP NOT NULL directly. Recreate
-- the table with the new shape and copy data over.
PRAGMA foreign_keys=OFF;

CREATE TABLE IF NOT EXISTS sources_new (
    id TEXT PRIMARY KEY,
    rule_id TEXT REFERENCES rules(id) ON DELETE CASCADE,
    signal_type TEXT NOT NULL CHECK(signal_type IN ('explicit_instruction', 'manual_edit', 'git_revert', 'manual_mark', 'import', 'repetition')),
    signal_strength TEXT NOT NULL CHECK(signal_strength IN ('strong', 'medium', 'weak')),
    agent_name TEXT,
    project_path TEXT,
    raw_snippet TEXT,
    timestamp DATETIME NOT NULL DEFAULT (datetime('now')),
    confidence_contribution REAL DEFAULT 0.0
);

INSERT INTO sources_new (id, rule_id, signal_type, signal_strength, agent_name,
                          project_path, raw_snippet, timestamp, confidence_contribution)
SELECT id, NULLIF(rule_id, ''), signal_type, signal_strength, agent_name,
       project_path, raw_snippet, timestamp, confidence_contribution
FROM sources;

DROP TABLE sources;

ALTER TABLE sources_new RENAME TO sources;

CREATE INDEX IF NOT EXISTS idx_sources_rule_id ON sources(rule_id);
CREATE INDEX IF NOT EXISTS idx_sources_signal_type ON sources(signal_type);
CREATE INDEX IF NOT EXISTS idx_sources_timestamp ON sources(timestamp);

PRAGMA foreign_keys=ON;
