package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// UserMemoryRepo handles CRUD for user personal context (cross-agent shared).
type UserMemoryRepo struct {
	db *sql.DB
}

// NewUserMemoryRepo creates a new UserMemoryRepo.
func NewUserMemoryRepo(db *sql.DB) *UserMemoryRepo {
	return &UserMemoryRepo{db: db}
}

// Create inserts a new user memory.
func (r *UserMemoryRepo) Create(m *UserMemory) error {
	tagsJSON, err := json.Marshal(m.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	_, err = r.db.Exec(`
		INSERT INTO user_memories (id, user_id, content, category, project_path, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.UserID, m.Content, m.Category, m.ProjectPath, string(tagsJSON),
		m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert user_memory: %w", err)
	}
	return nil
}

// GetByID fetches a user memory by ID.
func (r *UserMemoryRepo) GetByID(id string) (*UserMemory, error) {
	row := r.db.QueryRow(`
		SELECT id, user_id, content, category, COALESCE(project_path,''), tags, created_at, updated_at
		FROM user_memories WHERE id = ?`, id)

	m, err := scanUserMemory(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

// List returns all user memories, optionally filtered by userID or projectPath.
func (r *UserMemoryRepo) List(userID, projectPath string) ([]*UserMemory, error) {
	query := `SELECT id, user_id, content, category, COALESCE(project_path,''), tags, created_at, updated_at
	          FROM user_memories WHERE 1=1`
	var args []any
	if userID != "" {
		query += " AND user_id = ?"
		args = append(args, userID)
	}
	if projectPath != "" {
		query += " AND (project_path = ? OR project_path = '')"
		args = append(args, projectPath)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query user_memories: %w", err)
	}
	defer rows.Close()

	var results []*UserMemory
	for rows.Next() {
		var m UserMemory
		var tagsJSON string
		err := rows.Scan(&m.ID, &m.UserID, &m.Content, &m.Category, &m.ProjectPath,
			&tagsJSON, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan user_memory: %w", err)
		}
		_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
		if m.Tags == nil {
			m.Tags = []string{}
		}
		results = append(results, &m)
	}
	return results, rows.Err()
}

// Delete removes a user memory by ID.
func (r *UserMemoryRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM user_memories WHERE id = ?", id)
	return err
}

func scanUserMemory(row *sql.Row) (*UserMemory, error) {
	var m UserMemory
	var tagsJSON string
	err := row.Scan(&m.ID, &m.UserID, &m.Content, &m.Category, &m.ProjectPath,
		&tagsJSON, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
	if m.Tags == nil {
		m.Tags = []string{}
	}
	return &m, nil
}