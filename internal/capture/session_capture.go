package capture

import (
	"fmt"
	"sync"

	"github.com/joevilcai666/shadow/internal/storage"
)

// SessionCapture manages SessionMemory entries for active coding sessions.
type SessionCapture struct {
	repo       *storage.SessionMemoryRepo
	currentID  string
	currentSes *storage.SessionMemory
	mu         sync.RWMutex
}

// NewSessionCapture creates a new SessionCapture.
func NewSessionCapture(repo *storage.SessionMemoryRepo) *SessionCapture {
	return &SessionCapture{repo: repo}
}

// StartSession begins a new session for the given agent/project.
// If an existing session is active for this project+agent, it ends it first.
func (sc *SessionCapture) StartSession(agentName, projectPath, taskSummary string) (*storage.SessionMemory, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// End any existing active session for this project+agent.
	if sc.currentSes != nil && sc.currentSes.ProjectPath == projectPath && sc.currentSes.AgentName == agentName {
		if err := sc.endSessionLocked(sc.currentSes.ID); err != nil {
			// Non-fatal: log and continue.
		}
	}

	ses := &storage.SessionMemory{
		ID:          storage.NewID(),
		SessionID:   storage.NewID(),
		AgentName:   agentName,
		ProjectPath: projectPath,
		TaskSummary: taskSummary,
		CreatedAt:   storage.Now(),
	}

	if err := sc.repo.Create(ses); err != nil {
		return nil, fmt.Errorf("create session memory: %w", err)
	}

	sc.currentID = ses.ID
	sc.currentSes = ses
	return ses, nil
}

// GetCurrentSession returns the currently active session, if any.
func (sc *SessionCapture) GetCurrentSession() *storage.SessionMemory {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.currentSes
}

// UpdateContextDump updates the context dump for the current session.
func (sc *SessionCapture) UpdateContextDump(ctx string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.currentSes == nil {
		return nil
	}
	// Context dump is updated in-memory; a full implementation would persist it.
	sc.currentSes.ContextDump = ctx
	return nil
}

// EndSession ends the currently active session.
func (sc *SessionCapture) EndSession() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.currentSes == nil {
		return nil
	}
	return sc.endSessionLocked(sc.currentSes.ID)
}

// endSessionLocked ends a session (caller must hold lock).
func (sc *SessionCapture) endSessionLocked(id string) error {
	if err := sc.repo.End(id, storage.Now()); err != nil {
		return fmt.Errorf("end session: %w", err)
	}
	sc.currentID = ""
	sc.currentSes = nil
	return nil
}

// GetRecentSessions returns recent sessions for a project.
func (sc *SessionCapture) GetRecentSessions(projectPath string, limit int) ([]*storage.SessionMemory, error) {
	return sc.repo.ListByProject(projectPath, limit)
}