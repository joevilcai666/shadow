package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// RuleRepo handles CRUD operations for rules.
type RuleRepo struct {
	db *sql.DB
}

// NewRuleRepo creates a new RuleRepo.
func NewRuleRepo(db *sql.DB) *RuleRepo {
	return &RuleRepo{db: db}
}

// Create inserts a new rule and records its first version.
func (r *RuleRepo) Create(rule *Rule) error {
	tagsJSON, err := json.Marshal(rule.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO rules (id, content, scope, project_path, tags, category, trigger_context, confidence, status, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.Content, rule.Scope, rule.ProjectPath, string(tagsJSON),
		rule.Category, rule.TriggerContext, rule.Confidence, rule.Status,
		rule.Version, rule.CreatedAt, rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert rule: %w", err)
	}

	// Record initial version.
	_, err = tx.Exec(`
		INSERT INTO versions (id, rule_id, version, content, changed_by, change_reason, timestamp)
		VALUES (?, ?, ?, ?, 'auto', 'initial creation', ?)`,
		NewID(), rule.ID, rule.Version, rule.Content, rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert version: %w", err)
	}

	return tx.Commit()
}

// GetByID fetches a rule by its ID.
func (r *RuleRepo) GetByID(id string) (*Rule, error) {
	row := r.db.QueryRow(`
		SELECT id, content, scope, COALESCE(project_path,''), tags, COALESCE(category,''),
		       COALESCE(trigger_context,''), confidence, status, version, created_at, updated_at
		FROM rules WHERE id = ?`, id)

	rule, err := scanRule(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return rule, err
}

// Update modifies an existing rule, incrementing its version.
func (r *RuleRepo) Update(rule *Rule, changedBy, reason string) error {
	oldRule, err := r.GetByID(rule.ID)
	if err != nil {
		return fmt.Errorf("fetch old rule: %w", err)
	}
	if oldRule == nil {
		return fmt.Errorf("rule %s not found", rule.ID)
	}

	tagsJSON, err := json.Marshal(rule.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	newVersion := oldRule.Version + 1
	rule.Version = newVersion

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE rules SET content=?, scope=?, project_path=?, tags=?, category=?,
		                 trigger_context=?, confidence=?, status=?, version=?,
		                 updated_at=?
		WHERE id = ?`,
		rule.Content, rule.Scope, rule.ProjectPath, string(tagsJSON),
		rule.Category, rule.TriggerContext, rule.Confidence, rule.Status,
		newVersion, rule.UpdatedAt, rule.ID,
	)
	if err != nil {
		return fmt.Errorf("update rule: %w", err)
	}

	diff := computeDiff(oldRule.Content, rule.Content)
	_, err = tx.Exec(`
		INSERT INTO versions (id, rule_id, version, content, diff, changed_by, change_reason, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		NewID(), rule.ID, newVersion, rule.Content, diff, changedBy, reason, rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert version: %w", err)
	}

	return tx.Commit()
}

// Delete removes a rule by ID (cascades to sources and versions).
func (r *RuleRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM rules WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}
	return nil
}

// List returns rules matching the given filter.
func (r *RuleRepo) List(filter RuleFilter) ([]*Rule, error) {
	query, args := buildRuleQuery(filter)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	var rules []*Rule
	for rows.Next() {
		rule, err := scanRuleFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// Count returns the total number of rules matching a filter.
func (r *RuleRepo) Count(filter RuleFilter) (int, error) {
	query := "SELECT COUNT(*) FROM rules WHERE 1=1"
	var args []any

	if filter.Scope != "" {
		query += " AND scope = ?"
		args = append(args, filter.Scope)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.ProjectPath != "" {
		query += " AND project_path = ?"
		args = append(args, filter.ProjectPath)
	}
	if filter.Category != "" {
		query += " AND category = ?"
		args = append(args, filter.Category)
	}

	var count int
	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

func buildRuleQuery(f RuleFilter) (string, []any) {
	q := `SELECT id, content, scope, COALESCE(project_path,''), tags, COALESCE(category,''),
	              COALESCE(trigger_context,''), confidence, status, version, created_at, updated_at
	      FROM rules WHERE 1=1`
	var args []any

	if f.Scope != "" {
		q += " AND scope = ?"
		args = append(args, f.Scope)
	}
	if f.Status != "" {
		q += " AND status = ?"
		args = append(args, f.Status)
	}
	if f.ProjectPath != "" {
		q += " AND project_path = ?"
		args = append(args, f.ProjectPath)
	}
	if f.Category != "" {
		q += " AND category = ?"
		args = append(args, f.Category)
	}
	if f.Search != "" {
		q += " AND (content LIKE ? OR trigger_context LIKE ?)"
		pattern := "%" + f.Search + "%"
		args = append(args, pattern, pattern)
	}
	for _, tag := range f.Tags {
		q += " AND tags LIKE ?"
		args = append(args, `%"`+tag+`"%`)
	}

	switch f.OrderBy {
	case "confidence":
		q += " ORDER BY confidence DESC"
	case "created_at":
		q += " ORDER BY created_at DESC"
	default:
		q += " ORDER BY updated_at DESC"
	}

	if f.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, f.Limit)
	}
	if f.Offset > 0 {
		q += " OFFSET ?"
		args = append(args, f.Offset)
	}

	return q, args
}

func scanRule(row *sql.Row) (*Rule, error) {
	var rule Rule
	var tagsJSON string
	err := row.Scan(
		&rule.ID, &rule.Content, &rule.Scope, &rule.ProjectPath,
		&tagsJSON, &rule.Category, &rule.TriggerContext,
		&rule.Confidence, &rule.Status, &rule.Version,
		&rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &rule.Tags)
	if rule.Tags == nil {
		rule.Tags = []string{}
	}
	return &rule, nil
}

func scanRuleFromRows(rows *sql.Rows) (*Rule, error) {
	var rule Rule
	var tagsJSON string
	err := rows.Scan(
		&rule.ID, &rule.Content, &rule.Scope, &rule.ProjectPath,
		&tagsJSON, &rule.Category, &rule.TriggerContext,
		&rule.Confidence, &rule.Status, &rule.Version,
		&rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &rule.Tags)
	if rule.Tags == nil {
		rule.Tags = []string{}
	}
	return &rule, nil
}

// computeDiff produces a simple diff string between old and new content.
func computeDiff(old, new string) string {
	if old == new {
		return ""
	}
	return fmt.Sprintf("-%s\n+%s", old, new)
}
