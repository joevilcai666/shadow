package storage

import (
	"database/sql"
	"fmt"
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
