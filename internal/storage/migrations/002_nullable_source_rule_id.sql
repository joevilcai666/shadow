-- 002_nullable_source_rule_id.sql
--
-- Make sources.rule_id nullable so the capture engine can save a new
-- signal before the distill engine has linked it to a rule.
--
-- The original schema (001_init.sql) declared rule_id as NOT NULL with
-- a foreign key, but the capture -> distill flow expects new sources to
-- be created with no rule_id (the SourceRepo.LinkToRule call fills it
-- in later, and the distill query selects unlinked sources with
-- `WHERE rule_id = '' OR rule_id IS NULL`). The mismatch caused every
-- captured signal to fail with "FOREIGN KEY constraint failed" on a
-- fresh database (SQLite error 787).
--
-- SQLite has no `ALTER COLUMN ... DROP NOT NULL`, so we recreate the
-- table with the corrected schema. NULLIF() converts any pre-existing
-- empty-string rule_ids (the value the old capture path wrote) to NULL
-- so the distill query's IS NULL branch picks them up.

CREATE TABLE sources_new (
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

INSERT INTO sources_new (id, rule_id, signal_type, signal_strength, agent_name, project_path, raw_snippet, timestamp, confidence_contribution)
SELECT id, NULLIF(rule_id, ''), signal_type, signal_strength,
       agent_name, project_path, raw_snippet, timestamp, confidence_contribution
FROM sources;

DROP TABLE sources;
ALTER TABLE sources_new RENAME TO sources;

CREATE INDEX IF NOT EXISTS idx_sources_rule_id ON sources(rule_id);
CREATE INDEX IF NOT EXISTS idx_sources_signal_type ON sources(signal_type);
CREATE INDEX IF NOT EXISTS idx_sources_timestamp ON sources(timestamp);
