package storage

import (
	"database/sql"
	"fmt"
)

// SessionMemoryRepo handles single-session context snapshots.
type SessionMemoryRepo struct {
	db *sql.DB
}

// NewSessionMemoryRepo creates a new SessionMemoryRepo.
func NewSessionMemoryRepo(db *sql.DB) *SessionMemoryRepo {
	return &SessionMemoryRepo{db: db}
}

// Create inserts a new session memory.
func (r *SessionMemoryRepo) Create(s *SessionMemory) error {
	_, err := r.db.Exec(`
		INSERT INTO session_memories (id, session_id, agent_name, project_path, context_dump, task_summary, created_at, ended_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.SessionID, s.AgentName, s.ProjectPath, s.ContextDump, s.TaskSummary,
		s.CreatedAt, s.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("insert session_memory: %w", err)
	}
	return nil
}

// GetBySessionID returns the most recent session memory for a session.
func (r *SessionMemoryRepo) GetBySessionID(sessionID string) (*SessionMemory, error) {
	row := r.db.QueryRow(`
		SELECT id, session_id, agent_name, project_path, COALESCE(context_dump,''),
		       COALESCE(task_summary,''), created_at, COALESCE(ended_at,'')
		FROM session_memories WHERE session_id = ? ORDER BY created_at DESC LIMIT 1`, sessionID)

	s, err := scanSessionMemory(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// GetActiveByProject returns the most recent active (no ended_at) session for a project.
func (r *SessionMemoryRepo) GetActiveByProject(projectPath, agentName string) (*SessionMemory, error) {
	row := r.db.QueryRow(`
		SELECT id, session_id, agent_name, project_path, COALESCE(context_dump,''),
		       COALESCE(task_summary,''), created_at, COALESCE(ended_at,'')
		FROM session_memories
		WHERE project_path = ? AND agent_name = ? AND ended_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, projectPath, agentName)

	s, err := scanSessionMemory(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// End marks a session as ended.
func (r *SessionMemoryRepo) End(id, endedAt string) error {
	_, err := r.db.Exec("UPDATE session_memories SET ended_at = ? WHERE id = ?", endedAt, id)
	return err
}

// ListByProject returns all sessions for a project, up to limit (0 = no limit).
func (r *SessionMemoryRepo) ListByProject(projectPath string, limit int) ([]*SessionMemory, error) {
	var query string
	var args []any
	if limit > 0 {
		query = `SELECT id, session_id, agent_name, project_path, COALESCE(context_dump,''),
		         COALESCE(task_summary,''), created_at, COALESCE(ended_at,'')
		         FROM session_memories WHERE project_path = ? ORDER BY created_at DESC LIMIT ?`
		args = []any{projectPath, limit}
	} else {
		query = `SELECT id, session_id, agent_name, project_path, COALESCE(context_dump,''),
		         COALESCE(task_summary,''), created_at, COALESCE(ended_at,'')
		         FROM session_memories WHERE project_path = ? ORDER BY created_at DESC`
		args = []any{projectPath}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query session_memories: %w", err)
	}
	defer rows.Close()

	var results []*SessionMemory
	for rows.Next() {
		s, err := scanSessionMemoryFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan session_memory: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

func scanSessionMemory(row *sql.Row) (*SessionMemory, error) {
	var s SessionMemory
	err := row.Scan(&s.ID, &s.SessionID, &s.AgentName, &s.ProjectPath,
		&s.ContextDump, &s.TaskSummary, &s.CreatedAt, &s.EndedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func scanSessionMemoryFromRows(rows *sql.Rows) (*SessionMemory, error) {
	var s SessionMemory
	err := rows.Scan(&s.ID, &s.SessionID, &s.AgentName, &s.ProjectPath,
		&s.ContextDump, &s.TaskSummary, &s.CreatedAt, &s.EndedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

