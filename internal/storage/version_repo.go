package storage

import (
	"database/sql"
	"fmt"
)

// VersionRepo handles rule version history operations.
type VersionRepo struct {
	db *sql.DB
}

// NewVersionRepo creates a new VersionRepo.
func NewVersionRepo(db *sql.DB) *VersionRepo {
	return &VersionRepo{db: db}
}

// ListByRuleID returns version history for a rule, newest first.
func (r *VersionRepo) ListByRuleID(ruleID string) ([]*Version, error) {
	rows, err := r.db.Query(`
		SELECT id, rule_id, version, content, COALESCE(diff,''), changed_by, COALESCE(change_reason,''), timestamp
		FROM versions WHERE rule_id = ?
		ORDER BY version DESC`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("query versions: %w", err)
	}
	defer rows.Close()

	var versions []*Version
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// GetVersion returns a specific version of a rule.
func (r *VersionRepo) GetVersion(ruleID string, version int) (*Version, error) {
	row := r.db.QueryRow(`
		SELECT id, rule_id, version, content, COALESCE(diff,''), changed_by, COALESCE(change_reason,''), timestamp
		FROM versions WHERE rule_id = ? AND version = ?`, ruleID, version)

	var v Version
	err := row.Scan(&v.ID, &v.RuleID, &v.Version, &v.Content, &v.Diff,
		&v.ChangedBy, &v.ChangeReason, &v.Timestamp)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// Rollback reverts a rule to a previous version.
func (r *VersionRepo) Rollback(ruleID string, targetVersion int, reason string) error {
	// Fetch the target version content.
	v, err := r.GetVersion(ruleID, targetVersion)
	if err != nil {
		return fmt.Errorf("fetch target version: %w", err)
	}
	if v == nil {
		return fmt.Errorf("version %d not found for rule %s", targetVersion, ruleID)
	}

	// Get current rule to compute new version number.
	var currentVersion int
	err = r.db.QueryRow("SELECT version FROM rules WHERE id = ?", ruleID).Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("fetch current rule: %w", err)
	}

	newVersion := currentVersion + 1
	now := Now()

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Update rule content and version.
	_, err = tx.Exec(`
		UPDATE rules SET content = ?, version = ?, updated_at = ? WHERE id = ?`,
		v.Content, newVersion, now, ruleID)
	if err != nil {
		return fmt.Errorf("update rule: %w", err)
	}

	// Record rollback as new version.
	diff := fmt.Sprintf("rollback to v%d", targetVersion)
	_, err = tx.Exec(`
		INSERT INTO versions (id, rule_id, version, content, diff, changed_by, change_reason, timestamp)
		VALUES (?, ?, ?, ?, ?, 'user', ?, ?)`,
		NewID(), ruleID, newVersion, v.Content, diff, reason, now)
	if err != nil {
		return fmt.Errorf("insert rollback version: %w", err)
	}

	return tx.Commit()
}

func scanVersion(rows *sql.Rows) (*Version, error) {
	var v Version
	err := rows.Scan(&v.ID, &v.RuleID, &v.Version, &v.Content, &v.Diff,
		&v.ChangedBy, &v.ChangeReason, &v.Timestamp)
	return &v, err
}
