package storage

import (
	"database/sql"
	"fmt"
)

// SourceRepo handles operations for rule source tracing.
type SourceRepo struct {
	db *sql.DB
}

// NewSourceRepo creates a new SourceRepo.
func NewSourceRepo(db *sql.DB) *SourceRepo {
	return &SourceRepo{db: db}
}

// Create inserts a new source record.
func (r *SourceRepo) Create(source *Source) error {
	_, err := r.db.Exec(`
		INSERT INTO sources (id, rule_id, signal_type, signal_strength, agent_name,
		                     project_path, raw_snippet, timestamp, confidence_contribution)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		source.ID, source.RuleID, source.SignalType, source.SignalStrength,
		source.AgentName, source.ProjectPath, source.RawSnippet,
		source.Timestamp, source.ConfidenceContribution,
	)
	if err != nil {
		return fmt.Errorf("insert source: %w", err)
	}
	return nil
}

// ListByRuleID returns all sources for a rule, ordered by timestamp.
func (r *SourceRepo) ListByRuleID(ruleID string) ([]*Source, error) {
	rows, err := r.db.Query(`
		SELECT id, rule_id, signal_type, signal_strength, COALESCE(agent_name,''),
		       COALESCE(project_path,''), COALESCE(raw_snippet,''), timestamp, confidence_contribution
		FROM sources WHERE rule_id = ?
		ORDER BY timestamp ASC`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	var sources []*Source
	for rows.Next() {
		s, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

// StatsBySignalType returns count of sources grouped by signal_type for a rule.
func (r *SourceRepo) StatsBySignalType(ruleID string) (map[string]int, error) {
	rows, err := r.db.Query(`
		SELECT signal_type, COUNT(*) FROM sources
		WHERE rule_id = ?
		GROUP BY signal_type`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("query source stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var sigType string
		var count int
		if err := rows.Scan(&sigType, &count); err != nil {
			return nil, err
		}
		stats[sigType] = count
	}
	return stats, rows.Err()
}

func scanSource(rows *sql.Rows) (*Source, error) {
	var s Source
	err := rows.Scan(
		&s.ID, &s.RuleID, &s.SignalType, &s.SignalStrength,
		&s.AgentName, &s.ProjectPath, &s.RawSnippet,
		&s.Timestamp, &s.ConfidenceContribution,
	)
	return &s, err
}
