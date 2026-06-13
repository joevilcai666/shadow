package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// EventRepo stores effectiveness events: rule hits and adapter sync outcomes.
type EventRepo struct {
	db *sql.DB
}

// NewEventRepo creates a new EventRepo.
func NewEventRepo(db *sql.DB) *EventRepo {
	return &EventRepo{db: db}
}

// Create inserts an event. Empty optional strings are stored as NULL where useful.
func (r *EventRepo) Create(event *Event) error {
	if event.ID == "" {
		event.ID = NewID()
	}
	if event.Timestamp == "" {
		event.Timestamp = Now()
	}
	var ruleID sql.NullString
	if event.RuleID != "" {
		ruleID = sql.NullString{String: event.RuleID, Valid: true}
	}
	_, err := r.db.Exec(`
		INSERT INTO events (id, rule_id, event_type, agent_name, project_path, target_path, details, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, ruleID, event.EventType, event.AgentName, event.ProjectPath,
		event.TargetPath, event.Details, event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

// CountRuleHits returns hit counts keyed by rule_id.
func (r *EventRepo) CountRuleHits() (map[string]int, error) {
	rows, err := r.db.Query(`
		SELECT rule_id, COUNT(*)
		FROM events
		WHERE event_type = 'rule_hit' AND rule_id IS NOT NULL AND rule_id != ''
		GROUP BY rule_id`)
	if err != nil {
		return nil, fmt.Errorf("count rule hits: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var ruleID string
		var count int
		if err := rows.Scan(&ruleID, &count); err != nil {
			return nil, err
		}
		counts[ruleID] = count
	}
	return counts, rows.Err()
}

// CountRuleHitsByAgent returns rule-hit counts grouped by agent name.
func (r *EventRepo) CountRuleHitsByAgent() (map[string]int, error) {
	rows, err := r.db.Query(`
		SELECT COALESCE(agent_name, 'unknown'), COUNT(*)
		FROM events
		WHERE event_type = 'rule_hit'
		GROUP BY agent_name`)
	if err != nil {
		return nil, fmt.Errorf("count rule hits by agent: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var agent string
		var count int
		if err := rows.Scan(&agent, &count); err != nil {
			return nil, err
		}
		counts[agent] = count
	}
	return counts, rows.Err()
}

// CountRuleHitsByAgentName returns the number of hits attributed to one agent.
func (r *EventRepo) CountRuleHitsByAgentName(agentName string) (int, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM events
		WHERE event_type = 'rule_hit' AND agent_name = ?`, agentName).Scan(&count)
	return count, err
}

// LatestByAgentEvent returns the newest event of a type for one agent.
func (r *EventRepo) LatestByAgentEvent(agentName, eventType string) (*Event, error) {
	row := r.db.QueryRow(`
		SELECT id, rule_id, event_type, COALESCE(agent_name,''), COALESCE(project_path,''),
		       COALESCE(target_path,''), COALESCE(details,''), timestamp
		FROM events
		WHERE agent_name = ? AND event_type = ?
		ORDER BY timestamp DESC, id DESC
		LIMIT 1`, agentName, eventType)
	event, err := scanEvent(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return event, err
}

// MARK: - Hit-rate aggregation (SHADOW-041)
//
// These power the /api/stats/hit-rate endpoint. "Hit rate" is defined as the
// share of active rules that were surfaced/used at least once in the window —
// a Type-A proxy for the PRD's aspirational Type-C (user-perceived) metric.

// daysAgoRFC3339 returns an RFC3339 UTC timestamp N days in the past. Both
// stored event timestamps and these cutoffs use RFC3339, so SQL string
// comparisons are format-consistent (avoids SQLite datetime() vs RFC3339
// mismatches).
func daysAgoRFC3339(days int) string {
	return time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
}

// CountRuleHitsLastDays returns the number of rule_hit events in the last `days` days.
// Use subtraction for ranges: hits in [7,14) days = CountRuleHitsLastDays(14) - CountRuleHitsLastDays(7).
// (A single lower bound avoids RFC3339 same-second boundary issues that an
// exclusive upper bound would hit.)
func (r *EventRepo) CountRuleHitsLastDays(days int) (int, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM events
		WHERE event_type = 'rule_hit' AND datetime(timestamp) >= datetime(?)`,
		daysAgoRFC3339(days)).Scan(&count)
	return count, err
}

// DistinctHitRulesLastDays returns the count of distinct rule_ids hit in the last `days` days.
func (r *EventRepo) DistinctHitRulesLastDays(days int) (int, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(DISTINCT rule_id) FROM events
		WHERE event_type = 'rule_hit' AND rule_id IS NOT NULL AND rule_id != ''
		  AND datetime(timestamp) >= datetime(?)`,
		daysAgoRFC3339(days)).Scan(&count)
	return count, err
}

// CountRepeatedHitRulesLastDays returns distinct rule_ids with 2+ rule_hit
// events in the window. This is a recurrence proxy: the same remembered rule
// needed to surface more than once recently.
func (r *EventRepo) CountRepeatedHitRulesLastDays(days int) (int, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT rule_id
			FROM events
			WHERE event_type = 'rule_hit' AND rule_id IS NOT NULL AND rule_id != ''
			  AND datetime(timestamp) >= datetime(?)
			GROUP BY rule_id
			HAVING COUNT(*) >= 2
		)`,
		daysAgoRFC3339(days)).Scan(&count)
	return count, err
}

// LatestRuleHit returns the most recent rule_hit event across all agents, or nil if none.
func (r *EventRepo) LatestRuleHit() (*Event, error) {
	row := r.db.QueryRow(`
		SELECT id, rule_id, event_type, COALESCE(agent_name,''), COALESCE(project_path,''),
		       COALESCE(target_path,''), COALESCE(details,''), timestamp
		FROM events
		WHERE event_type = 'rule_hit'
		ORDER BY timestamp DESC, id DESC
		LIMIT 1`)
	event, err := scanEvent(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return event, err
}

// LatestSyncByAgent returns the latest sync success or failure for one agent.
func (r *EventRepo) LatestSyncByAgent(agentName string) (*Event, error) {
	row := r.db.QueryRow(`
		SELECT id, rule_id, event_type, COALESCE(agent_name,''), COALESCE(project_path,''),
		       COALESCE(target_path,''), COALESCE(details,''), timestamp
		FROM events
		WHERE agent_name = ? AND event_type IN ('sync_success', 'sync_failure')
		ORDER BY timestamp DESC, id DESC
		LIMIT 1`, agentName)
	event, err := scanEvent(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return event, err
}

// ListByRuleID returns events tied to one rule.
func (r *EventRepo) ListByRuleID(ruleID string) ([]*Event, error) {
	rows, err := r.db.Query(`
		SELECT id, rule_id, event_type, COALESCE(agent_name,''), COALESCE(project_path,''),
		       COALESCE(target_path,''), COALESCE(details,''), timestamp
		FROM events
		WHERE rule_id = ?
		ORDER BY timestamp ASC`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("query rule events: %w", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		event, err := scanEventRows(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// AgentsForRule returns distinct agents that produced source or hit evidence for a rule.
func (r *EventRepo) AgentsForRule(ruleID string) ([]string, error) {
	rows, err := r.db.Query(`
		SELECT DISTINCT agent_name
		FROM events
		WHERE rule_id = ? AND agent_name IS NOT NULL AND agent_name != ''
		ORDER BY agent_name`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("query event agents: %w", err)
	}
	defer rows.Close()

	var agents []string
	for rows.Next() {
		var agent string
		if err := rows.Scan(&agent); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

// AgentsByRuleIDs returns distinct event agents for a batch of rules.
func (r *EventRepo) AgentsByRuleIDs(ruleIDs []string) (map[string]map[string]bool, error) {
	out := make(map[string]map[string]bool, len(ruleIDs))
	if len(ruleIDs) == 0 {
		return out, nil
	}
	args := make([]any, len(ruleIDs))
	for i, id := range ruleIDs {
		args[i] = id
	}
	rows, err := r.db.Query(`
		SELECT DISTINCT rule_id, agent_name
		FROM events
		WHERE rule_id IN (`+eventPlaceholders(len(ruleIDs))+`)
		  AND agent_name IS NOT NULL AND agent_name != ''
		ORDER BY rule_id, agent_name`, args...)
	if err != nil {
		return nil, fmt.Errorf("query event agents: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ruleID, agent string
		if err := rows.Scan(&ruleID, &agent); err != nil {
			return nil, err
		}
		if out[ruleID] == nil {
			out[ruleID] = map[string]bool{}
		}
		out[ruleID][agent] = true
	}
	return out, rows.Err()
}

func scanEvent(row interface {
	Scan(dest ...any) error
}) (*Event, error) {
	var event Event
	var ruleID sql.NullString
	err := row.Scan(
		&event.ID, &ruleID, &event.EventType, &event.AgentName, &event.ProjectPath,
		&event.TargetPath, &event.Details, &event.Timestamp,
	)
	if err != nil {
		return &event, err
	}
	if ruleID.Valid {
		event.RuleID = ruleID.String
	}
	return &event, nil
}

func scanEventRows(rows *sql.Rows) (*Event, error) {
	return scanEvent(rows)
}

func eventPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}
