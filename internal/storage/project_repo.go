package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// ProjectRepo handles project registration operations.
type ProjectRepo struct {
	db *sql.DB
}

// NewProjectRepo creates a new ProjectRepo.
func NewProjectRepo(db *sql.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

// Create inserts a new project.
func (r *ProjectRepo) Create(project *Project) error {
	agentsJSON, err := json.Marshal(project.Agents)
	if err != nil {
		return fmt.Errorf("marshal agents: %w", err)
	}

	_, err = r.db.Exec(`
		INSERT INTO projects (id, path, name, agents, last_scan_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		project.ID, project.Path, project.Name, string(agentsJSON),
		project.LastScanAt, project.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

// GetByID fetches a project by its ID.
func (r *ProjectRepo) GetByID(id string) (*Project, error) {
	row := r.db.QueryRow(`
		SELECT id, path, name, agents, last_scan_at, created_at
		FROM projects WHERE id = ?`, id)

	return scanProject(row)
}

// GetByPath fetches a project by its filesystem path.
func (r *ProjectRepo) GetByPath(path string) (*Project, error) {
	row := r.db.QueryRow(`
		SELECT id, path, name, agents, last_scan_at, created_at
		FROM projects WHERE path = ?`, path)

	return scanProject(row)
}

// List returns all registered projects.
func (r *ProjectRepo) List() ([]*Project, error) {
	rows, err := r.db.Query(`
		SELECT id, path, name, agents, last_scan_at, created_at
		FROM projects ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		p, err := scanProjectFromRows(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// Update modifies an existing project.
func (r *ProjectRepo) Update(project *Project) error {
	agentsJSON, err := json.Marshal(project.Agents)
	if err != nil {
		return fmt.Errorf("marshal agents: %w", err)
	}

	_, err = r.db.Exec(`
		UPDATE projects SET name = ?, agents = ?, last_scan_at = ?
		WHERE id = ?`,
		project.Name, string(agentsJSON), project.LastScanAt, project.ID)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

// Delete removes a project by ID.
func (r *ProjectRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

func scanProject(row *sql.Row) (*Project, error) {
	var p Project
	var agentsJSON string
	var lastScan sql.NullString

	err := row.Scan(&p.ID, &p.Path, &p.Name, &agentsJSON, &lastScan, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(agentsJSON), &p.Agents)
	if p.Agents == nil {
		p.Agents = []string{}
	}
	if lastScan.Valid {
		p.LastScanAt = &lastScan.String
	}
	return &p, nil
}

func scanProjectFromRows(rows *sql.Rows) (*Project, error) {
	var p Project
	var agentsJSON string
	var lastScan sql.NullString

	err := rows.Scan(&p.ID, &p.Path, &p.Name, &agentsJSON, &lastScan, &p.CreatedAt)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(agentsJSON), &p.Agents)
	if p.Agents == nil {
		p.Agents = []string{}
	}
	if lastScan.Valid {
		p.LastScanAt = &lastScan.String
	}
	return &p, nil
}
