package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/joevilcai666/shadow/internal/config"
	"github.com/joevilcai666/shadow/internal/storage"
)

// newLocalRequest creates a request with localhost Host header.
func newLocalRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Host = "localhost:7878"
	return req
}

// testEnv sets up a full test environment with DB, repos, and config.
func testEnv(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfgMgr := config.NewManager(dir)
	cfgMgr.LoadGlobal()

	s := New(
		storage.NewRuleRepo(db),
		storage.NewSourceRepo(db),
		storage.NewVersionRepo(db),
		storage.NewConfigRepo(db),
		storage.NewProjectRepo(db),
		cfgMgr,
		config.ServerConfig{Port: 7878, Bind: "127.0.0.1"},
	)
	return s, db
}

func TestListRulesEmpty(t *testing.T) {
	s, _ := testEnv(t)
	req := newLocalRequest("GET", "/api/rules", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var rules []*storage.Rule
	json.NewDecoder(w.Body).Decode(&rules)
	if rules == nil {
		t.Error("should return empty array, not null")
	}
}

func TestCreateAndGetRule(t *testing.T) {
	s, _ := testEnv(t)

	rule := map[string]any{
		"content":    "Use pnpm not npm",
		"scope":      "global",
		"tags":       []string{"tooling"},
		"category":   "toolchain",
		"confidence": 0.9,
		"status":     "candidate",
	}
	body, _ := json.Marshal(rule)

	// Create.
	req := newLocalRequest("POST", "/api/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create status: got %d, want %d. body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var created storage.Rule
	json.NewDecoder(w.Body).Decode(&created)
	if created.ID == "" {
		t.Fatal("created rule should have ID")
	}
	if created.Content != "Use pnpm not npm" {
		t.Errorf("content mismatch: %q", created.Content)
	}

	// Get.
	req = newLocalRequest("GET", "/api/rules/"+created.ID, nil)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("get status: %d", w.Code)
	}

	var got storage.Rule
	json.NewDecoder(w.Body).Decode(&got)
	if got.Content != created.Content {
		t.Errorf("get content: got %q, want %q", got.Content, created.Content)
	}
}

func TestDeleteRule(t *testing.T) {
	s, _ := testEnv(t)

	rule := map[string]any{"content": "test", "scope": "global", "status": "active"}
	body, _ := json.Marshal(rule)
	req := newLocalRequest("POST", "/api/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	var created storage.Rule
	json.NewDecoder(w.Body).Decode(&created)

	// Delete.
	req = newLocalRequest("DELETE", "/api/rules/"+created.ID, nil)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("delete status: %d", w.Code)
	}

	// Verify gone.
	req = newLocalRequest("GET", "/api/rules/"+created.ID, nil)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("should be 404 after delete, got %d", w.Code)
	}
}

func TestSensitiveDataRejection(t *testing.T) {
	s, _ := testEnv(t)

	rule := map[string]any{
		"content": "The API key is sk-abc123def456ghi789jkl012mno345pqr",
		"scope":   "global",
		"status":  "candidate",
	}
	body, _ := json.Marshal(rule)

	req := newLocalRequest("POST", "/api/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("should reject sensitive data, got status %d", w.Code)
	}
}

func TestRuleTimeline(t *testing.T) {
	s, db := testEnv(t)

	rule := &storage.Rule{
		ID: storage.NewID(), Content: "Test", Scope: "global",
		Tags: []string{}, Status: "active", Version: 1,
		CreatedAt: storage.Now(), UpdatedAt: storage.Now(),
	}
	storage.NewRuleRepo(db).Create(rule)

	sourceRepo := storage.NewSourceRepo(db)
	sourceRepo.Create(&storage.Source{
		ID: storage.NewID(), RuleID: rule.ID, SignalType: "explicit_instruction",
		SignalStrength: "strong", AgentName: "claude-code", Timestamp: storage.Now(),
	})
	sourceRepo.Create(&storage.Source{
		ID: storage.NewID(), RuleID: rule.ID, SignalType: "manual_edit",
		SignalStrength: "medium", AgentName: "cursor", Timestamp: storage.Now(),
	})

	r := mux.NewRouter()
	r.HandleFunc("/api/rules/{id}/timeline", s.getRuleTimeline).Methods("GET")
	req := newLocalRequest("GET", "/api/rules/"+rule.ID+"/timeline", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("timeline status: %d", w.Code)
	}
}

func TestDashboard(t *testing.T) {
	s, _ := testEnv(t)

	req := newLocalRequest("GET", "/api/dashboard", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("dashboard status: %d", w.Code)
	}

	var data map[string]any
	json.NewDecoder(w.Body).Decode(&data)
	if data["total_rules"] == nil {
		t.Error("dashboard should include total_rules")
	}
}

func TestCreateProject(t *testing.T) {
	s, _ := testEnv(t)

	project := map[string]any{
		"path":   "/tmp/test-project",
		"name":   "test-project",
		"agents": []string{"claude-code"},
	}
	body, _ := json.Marshal(project)

	req := newLocalRequest("POST", "/api/projects", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("create project status: %d, body: %s", w.Code, w.Body.String())
	}
}

func TestLocalhostOnly(t *testing.T) {
	s, _ := testEnv(t)

	req := httptest.NewRequest("GET", "/api/rules", nil)
	req.Host = "evil.com:7878"
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("non-localhost should be 403, got %d", w.Code)
	}
}
