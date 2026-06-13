package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// RuleRepo handles CRUD operations for rules.
type RuleRepo struct {
	db *sql.DB
}

// RuleStatusCounts contains the dashboard aggregate counts for rules.
type RuleStatusCounts struct {
	Total      int
	Active     int
	Candidate  int
	Disabled   int
	Conflicted int
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
	sourcePathsJSON, err := json.Marshal(rule.SourcePaths)
	if err != nil {
		return fmt.Errorf("marshal source_paths: %w", err)
	}
	// Normalize author: empty string → "agent" (SQLite DEFAULT only fires on NULL)
	if rule.Author == "" {
		rule.Author = "agent"
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO rules (id, content, scope, project_path, tags, category, trigger_context, confidence, status, version, importance, decay_score, last_hit_at, source_paths, author, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.Content, rule.Scope, rule.ProjectPath, string(tagsJSON),
		rule.Category, rule.TriggerContext, rule.Confidence, rule.Status,
		rule.Version, rule.Importance, rule.DecayScore, rule.LastHitAt, string(sourcePathsJSON),
		rule.Author, rule.CreatedAt, rule.UpdatedAt,
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
		       COALESCE(trigger_context,''), confidence, status, version,
		       COALESCE(importance,0.5), COALESCE(decay_score,0), COALESCE(last_hit_at,''),
		       COALESCE(source_paths,'[]'), COALESCE(author,'agent'),
		       created_at, updated_at
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
	sourcePathsJSON, err := json.Marshal(rule.SourcePaths)
	if err != nil {
		return fmt.Errorf("marshal source_paths: %w", err)
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
		                 importance=?, decay_score=?, last_hit_at=?, source_paths=?, author=?,
		                 updated_at=?
		WHERE id = ?`,
		rule.Content, rule.Scope, rule.ProjectPath, string(tagsJSON),
		rule.Category, rule.TriggerContext, rule.Confidence, rule.Status,
		newVersion, rule.Importance, rule.DecayScore, rule.LastHitAt, string(sourcePathsJSON),
		rule.Author, rule.UpdatedAt, rule.ID,
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

// TouchHit refreshes a rule's last_hit_at and decay_score after a hit.
// It is intentionally lightweight: a hit is not an edit, so no version
// snapshot is created (avoids version-table churn). Callers compute the
// new decay score via ComputeDecayScore(confidence, lastHitAt). (SHADOW-041)
func (r *RuleRepo) TouchHit(id, lastHitAt string, decayScore float64) error {
	_, err := r.db.Exec(`
		UPDATE rules SET last_hit_at = ?, decay_score = ?, updated_at = datetime('now')
		WHERE id = ?`, lastHitAt, decayScore, id)
	if err != nil {
		return fmt.Errorf("touch hit: %w", err)
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

// StatusCounts returns all rule status totals with a single table scan.
func (r *RuleRepo) StatusCounts() (RuleStatusCounts, error) {
	var counts RuleStatusCounts
	rows, err := r.db.Query(`
		SELECT status, COUNT(*)
		FROM rules
		GROUP BY status`)
	if err != nil {
		return counts, fmt.Errorf("count rules by status: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return counts, err
		}
		counts.Total += count
		switch status {
		case "active":
			counts.Active = count
		case "candidate":
			counts.Candidate = count
		case "disabled":
			counts.Disabled = count
		case "conflicted":
			counts.Conflicted = count
		}
	}
	return counts, rows.Err()
}

// ActiveProjectRulesByPath returns all active project-scoped rules grouped by
// project path with one rules table scan. Adapter sync uses this to avoid
// querying once per project per adapter.
func (r *RuleRepo) ActiveProjectRulesByPath() (map[string][]*Rule, error) {
	rows, err := r.db.Query(`SELECT id, content, scope, COALESCE(project_path,''), tags, COALESCE(category,''),
	              COALESCE(trigger_context,''), confidence, status, version,
	              COALESCE(importance,0.5), COALESCE(decay_score,0), COALESCE(last_hit_at,''),
	              COALESCE(source_paths,'[]'), COALESCE(author,'agent'),
	              created_at, updated_at
	      FROM rules
	      WHERE status = 'active' AND scope = 'project'
	      ORDER BY project_path, updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query active project rules: %w", err)
	}
	defer rows.Close()

	byPath := map[string][]*Rule{}
	for rows.Next() {
		rule, err := scanRuleFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan active project rule: %w", err)
		}
		byPath[rule.ProjectPath] = append(byPath[rule.ProjectPath], rule)
	}
	return byPath, rows.Err()
}

func buildRuleQuery(f RuleFilter) (string, []any) {
	q := `SELECT id, content, scope, COALESCE(project_path,''), tags, COALESCE(category,''),
	              COALESCE(trigger_context,''), confidence, status, version,
	              COALESCE(importance,0.5), COALESCE(decay_score,0), COALESCE(last_hit_at,''),
	              COALESCE(source_paths,'[]'), COALESCE(author,'agent'),
	              created_at, updated_at
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
	var tagsJSON, sourcePathsJSON string
	err := row.Scan(
		&rule.ID, &rule.Content, &rule.Scope, &rule.ProjectPath,
		&tagsJSON, &rule.Category, &rule.TriggerContext,
		&rule.Confidence, &rule.Status, &rule.Version,
		&rule.Importance, &rule.DecayScore, &rule.LastHitAt,
		&sourcePathsJSON, &rule.Author,
		&rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &rule.Tags)
	if rule.Tags == nil {
		rule.Tags = []string{}
	}
	_ = json.Unmarshal([]byte(sourcePathsJSON), &rule.SourcePaths)
	if rule.SourcePaths == nil {
		rule.SourcePaths = []string{}
	}
	return &rule, nil
}

func scanRuleFromRows(rows *sql.Rows) (*Rule, error) {
	var rule Rule
	var tagsJSON, sourcePathsJSON string
	err := rows.Scan(
		&rule.ID, &rule.Content, &rule.Scope, &rule.ProjectPath,
		&tagsJSON, &rule.Category, &rule.TriggerContext,
		&rule.Confidence, &rule.Status, &rule.Version,
		&rule.Importance, &rule.DecayScore, &rule.LastHitAt,
		&sourcePathsJSON, &rule.Author,
		&rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &rule.Tags)
	if rule.Tags == nil {
		rule.Tags = []string{}
	}
	_ = json.Unmarshal([]byte(sourcePathsJSON), &rule.SourcePaths)
	if rule.SourcePaths == nil {
		rule.SourcePaths = []string{}
	}
	return &rule, nil
}

// computeDiff produces a unified-diff-style string between two rule
// contents. It splits both inputs by line and runs a simple LCS-based
// diff, emitting "-old" for removed lines and "+new" for added lines.
// Context lines (those present on both sides) are emitted unchanged so
// the result is readable in the rule-history UI. Returns an empty
// string when the two contents are identical.
func computeDiff(old, new string) string {
	if old == new {
		return ""
	}
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	// Build the LCS table.
	m, n := len(oldLines), len(newLines)
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else if lcs[i-1][j] >= lcs[i][j-1] {
				lcs[i][j] = lcs[i-1][j]
			} else {
				lcs[i][j] = lcs[i][j-1]
			}
		}
	}

	// Walk the table backwards to emit the diff.
	var sb strings.Builder
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && oldLines[i-1] == newLines[j-1]:
			sb.WriteString(" " + oldLines[i-1] + "\n")
			i--
			j--
		case j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]):
			sb.WriteString("+" + newLines[j-1] + "\n")
			j--
		default:
			sb.WriteString("-" + oldLines[i-1] + "\n")
			i--
		}
	}
	return sb.String()
}
