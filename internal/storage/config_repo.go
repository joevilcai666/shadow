package storage

import (
	"database/sql"
	"fmt"
)

// ConfigRepo handles key-value configuration storage.
type ConfigRepo struct {
	db *sql.DB
}

// NewConfigRepo creates a new ConfigRepo.
func NewConfigRepo(db *sql.DB) *ConfigRepo {
	return &ConfigRepo{db: db}
}

// Get retrieves a config value by key and scope. Falls back to global if not found in project scope.
func (r *ConfigRepo) Get(key, scope string) (*ConfigEntry, error) {
	// Try exact scope first.
	row := r.db.QueryRow(`
		SELECT key, value, scope, updated_at FROM config
		WHERE key = ? AND scope = ?`, key, scope)

	var entry ConfigEntry
	err := row.Scan(&entry.Key, &entry.Value, &entry.Scope, &entry.UpdatedAt)
	if err == sql.ErrNoRows {
		// Fall back to global scope.
		if scope != "global" {
			return r.Get(key, "global")
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// Set upserts a config value.
func (r *ConfigRepo) Set(key, value, scope string) error {
	now := Now()
	_, err := r.db.Exec(`
		INSERT INTO config (key, value, scope, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(key, scope) DO UPDATE SET value = ?, updated_at = ?`,
		key, value, scope, now, value, now)
	if err != nil {
		return fmt.Errorf("set config: %w", err)
	}
	return nil
}

// ListByScope returns all config entries for a scope.
func (r *ConfigRepo) ListByScope(scope string) ([]*ConfigEntry, error) {
	rows, err := r.db.Query(`
		SELECT key, value, scope, updated_at FROM config WHERE scope = ?
		ORDER BY key`, scope)
	if err != nil {
		return nil, fmt.Errorf("list config: %w", err)
	}
	defer rows.Close()

	var entries []*ConfigEntry
	for rows.Next() {
		var e ConfigEntry
		if err := rows.Scan(&e.Key, &e.Value, &e.Scope, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

// Delete removes a config entry.
func (r *ConfigRepo) Delete(key, scope string) error {
	_, err := r.db.Exec("DELETE FROM config WHERE key = ? AND scope = ?", key, scope)
	if err != nil {
		return fmt.Errorf("delete config: %w", err)
	}
	return nil
}
