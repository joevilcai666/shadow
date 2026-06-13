package storage

import (
	"database/sql"
	"fmt"
)

// AgentMemoryRepo handles agent procedural memory (extracted from usage).
type AgentMemoryRepo struct {
	db *sql.DB
}

// NewAgentMemoryRepo creates a new AgentMemoryRepo.
func NewAgentMemoryRepo(db *sql.DB) *AgentMemoryRepo {
	return &AgentMemoryRepo{db: db}
}

// Create inserts a new agent memory.
func (r *AgentMemoryRepo) Create(m *AgentMemory) error {
	_, err := r.db.Exec(`
		INSERT INTO agent_memories (id, agent_name, content, extracted_from, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		m.ID, m.AgentName, m.Content, m.ExtractedFrom, m.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert agent_memory: %w", err)
	}
	return nil
}

// ListByAgent returns all agent memories for a given agent name.
func (r *AgentMemoryRepo) ListByAgent(agentName string) ([]*AgentMemory, error) {
	rows, err := r.db.Query(`
		SELECT id, agent_name, content, COALESCE(extracted_from,''), created_at
		FROM agent_memories WHERE agent_name = ? ORDER BY created_at DESC`, agentName)
	if err != nil {
		return nil, fmt.Errorf("query agent_memories: %w", err)
	}
	defer rows.Close()

	var results []*AgentMemory
	for rows.Next() {
		var m AgentMemory
		err := rows.Scan(&m.ID, &m.AgentName, &m.Content, &m.ExtractedFrom, &m.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan agent_memory: %w", err)
		}
		results = append(results, &m)
	}
	return results, rows.Err()
}

// Delete removes an agent memory by ID.
func (r *AgentMemoryRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM agent_memories WHERE id = ?", id)
	return err
}