package storage

import (
	"database/sql"
	"fmt"
	"strings"
)

// SourceRepo handles operations for rule source tracing.
type SourceRepo struct {
	db *sql.DB
}

// SourceEvidence is the compact source summary used by dashboard-style views.
type SourceEvidence struct {
	FirstSnippet string
	Agents       map[string]bool
}

// NewSourceRepo creates a new SourceRepo.
func NewSourceRepo(db *sql.DB) *SourceRepo {
	return &SourceRepo{db: db}
}

// Create inserts a new source record. An empty RuleID is stored as NULL so
// the capture path can persist signals before the distill engine links them
// to a crystallized rule (see migration 002).
func (r *SourceRepo) Create(source *Source) error {
	var ruleID sql.NullString
	if source.RuleID != "" {
		ruleID = sql.NullString{String: source.RuleID, Valid: true}
	}
	_, err := r.db.Exec(`
		INSERT INTO sources (id, rule_id, signal_type, signal_strength, agent_name,
		                     project_path, raw_snippet, timestamp, confidence_contribution)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		source.ID, ruleID, source.SignalType, source.SignalStrength,
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

// EvidenceByRuleIDs returns first source snippet and source agents for a batch
// of rules. It avoids one source query per map node.
func (r *SourceRepo) EvidenceByRuleIDs(ruleIDs []string) (map[string]SourceEvidence, error) {
	out := make(map[string]SourceEvidence, len(ruleIDs))
	if len(ruleIDs) == 0 {
		return out, nil
	}
	args := make([]any, len(ruleIDs))
	for i, id := range ruleIDs {
		args[i] = id
	}
	rows, err := r.db.Query(`
		SELECT rule_id, COALESCE(agent_name,''), COALESCE(raw_snippet,'')
		FROM sources
		WHERE rule_id IN (`+placeholders(len(ruleIDs))+`)
		ORDER BY timestamp ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("query source evidence: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ruleID, agent, snippet string
		if err := rows.Scan(&ruleID, &agent, &snippet); err != nil {
			return nil, err
		}
		ev := out[ruleID]
		if ev.Agents == nil {
			ev.Agents = map[string]bool{}
		}
		if ev.FirstSnippet == "" && snippet != "" {
			ev.FirstSnippet = snippet
		}
		if agent != "" {
			ev.Agents[agent] = true
		}
		out[ruleID] = ev
	}
	return out, rows.Err()
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
	var ruleID sql.NullString
	err := rows.Scan(
		&s.ID, &ruleID, &s.SignalType, &s.SignalStrength,
		&s.AgentName, &s.ProjectPath, &s.RawSnippet,
		&s.Timestamp, &s.ConfidenceContribution,
	)
	if err != nil {
		return &s, err
	}
	if ruleID.Valid {
		s.RuleID = ruleID.String
	}
	return &s, nil
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

// ListUnlinked returns sources that haven't been linked to any rule yet.
func (r *SourceRepo) ListUnlinked(limit int) ([]*Source, error) {
	query := `
		SELECT id, rule_id, signal_type, signal_strength, COALESCE(agent_name,''),
		       COALESCE(project_path,''), COALESCE(raw_snippet,''), timestamp, confidence_contribution
		FROM sources WHERE rule_id = '' OR rule_id IS NULL
		ORDER BY timestamp ASC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query unlinked sources: %w", err)
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

// LinkToRule updates a source to be linked to a specific rule.
func (r *SourceRepo) LinkToRule(sourceID, ruleID string) error {
	_, err := r.db.Exec("UPDATE sources SET rule_id = ? WHERE id = ?", ruleID, sourceID)
	if err != nil {
		return fmt.Errorf("link source to rule: %w", err)
	}
	return nil
}

// CountByAgent returns the count of sources grouped by agent_name.
func (r *SourceRepo) CountByAgent() (map[string]int, error) {
	rows, err := r.db.Query(`
		SELECT COALESCE(agent_name, 'unknown'), COUNT(*)
		FROM sources GROUP BY agent_name`)
	if err != nil {
		return nil, fmt.Errorf("count sources by agent: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var agent string
		var count int
		if err := rows.Scan(&agent, &count); err != nil {
			return nil, err
		}
		stats[agent] = count
	}
	return stats, rows.Err()
}

// CountTotal returns the total number of sources.
func (r *SourceRepo) CountTotal() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM sources").Scan(&count)
	return count, err
}
