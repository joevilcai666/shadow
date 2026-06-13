package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/joevilcai666/shadow/internal/adapter"
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
	s, db, _ := testEnvWithDir(t)
	return s, db
}

func testEnvWithDir(t *testing.T) (*Server, *sql.DB, string) {
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
		storage.NewEventRepo(db),
		storage.NewVersionRepo(db),
		storage.NewConfigRepo(db),
		storage.NewProjectRepo(db),
		cfgMgr,
		config.ServerConfig{Port: 7878, Bind: "127.0.0.1"},
		adapter.NewMCPServer(storage.NewRuleRepo(db)),
	)
	return s, db, dir
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

func TestPartialRuleUpdatePreservesFieldsAndTriggersSync(t *testing.T) {
	s, db := testEnv(t)
	syncs := 0
	s.SetControlHooks(nil, func() error {
		syncs++
		return nil
	})

	original := &storage.Rule{
		ID:          storage.NewID(),
		Content:     "Use pnpm not npm",
		Scope:       "project",
		ProjectPath: "/tmp/shadow-project",
		Tags:        []string{"toolchain"},
		Category:    "toolchain",
		Confidence:  0.9,
		Status:      "candidate",
		Version:     1,
		CreatedAt:   storage.Now(),
		UpdatedAt:   storage.Now(),
	}
	if err := storage.NewRuleRepo(db).Create(original); err != nil {
		t.Fatalf("seed rule: %v", err)
	}

	req := newLocalRequest("PUT", "/api/rules/"+original.ID, bytes.NewReader([]byte(`{"status":"active"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("update status: got %d, want %d. body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	got, err := storage.NewRuleRepo(db).GetByID(original.ID)
	if err != nil {
		t.Fatalf("load updated rule: %v", err)
	}
	if got.Content != original.Content {
		t.Errorf("content = %q, want preserved %q", got.Content, original.Content)
	}
	if got.ProjectPath != original.ProjectPath {
		t.Errorf("project path = %q, want %q", got.ProjectPath, original.ProjectPath)
	}
	if got.Status != "active" {
		t.Errorf("status = %q, want active", got.Status)
	}
	if syncs != 1 {
		t.Errorf("adapter sync calls = %d, want 1", syncs)
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

func TestDashboardIncludesEffectivenessMetrics(t *testing.T) {
	s, db := testEnv(t)
	ruleRepo := storage.NewRuleRepo(db)
	eventRepo := storage.NewEventRepo(db)

	rule := &storage.Rule{
		ID: storage.NewID(), Content: "Use pnpm not npm", Scope: "global",
		Tags: []string{"toolchain"}, Status: "active", Version: 1,
		CreatedAt: storage.Now(), UpdatedAt: storage.Now(),
	}
	if err := ruleRepo.Create(rule); err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	for _, event := range []*storage.Event{
		{
			ID: storage.NewID(), RuleID: rule.ID, EventType: "rule_hit",
			AgentName: "codex", ProjectPath: "/tmp/app", TargetPath: "AGENTS.md",
			Details: "Aha demo memory hit", Timestamp: storage.Now(),
		},
		{
			ID: storage.NewID(), EventType: "sync_success",
			AgentName: "codex", ProjectPath: "/tmp/app", TargetPath: "AGENTS.md",
			Details: "wrote 1 active rule", Timestamp: storage.Now(),
		},
	} {
		if err := eventRepo.Create(event); err != nil {
			t.Fatalf("seed event: %v", err)
		}
	}

	req := newLocalRequest("GET", "/api/dashboard", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("dashboard status: %d", w.Code)
	}

	var data map[string]any
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	if data["total_rule_hits"].(float64) != 1 {
		t.Fatalf("total_rule_hits = %v, want 1", data["total_rule_hits"])
	}
	coverage := data["agent_coverage"].(map[string]any)
	if coverage["codex"].(float64) != 1 {
		t.Fatalf("codex coverage = %v, want 1", coverage["codex"])
	}
	sync := data["adapter_sync"].(map[string]any)
	if _, ok := sync["codex"]; !ok {
		t.Fatalf("adapter_sync = %#v, want codex latest sync", sync)
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

func TestUpdateConfigPersistsActualConfig(t *testing.T) {
	s, _, dir := testEnvWithDir(t)

	req := newLocalRequest("PUT", "/api/config", bytes.NewReader([]byte(`{"capture_enabled":false,"cursor_enabled":false,"distill_threshold":"high"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("config update status: got %d, want %d. body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if s.configMgr.Get().Capture.Enabled {
		t.Error("capture enabled should be false in live config")
	}
	if s.configMgr.Get().Adapters.Cursor.Enabled {
		t.Error("cursor adapter should be false in live config")
	}

	reloaded := config.NewManager(dir)
	if err := reloaded.LoadGlobal(); err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.Get().Capture.Enabled {
		t.Error("capture enabled should persist as false")
	}
	if reloaded.Get().Adapters.Cursor.Enabled {
		t.Error("cursor adapter should persist as false")
	}
	if reloaded.Get().Distill.Threshold != "high" {
		t.Errorf("threshold = %q, want high", reloaded.Get().Distill.Threshold)
	}
}

func TestToggleAdapterPersistsAndTriggersSync(t *testing.T) {
	s, _, dir := testEnvWithDir(t)
	syncs := 0
	s.SetControlHooks(nil, func() error {
		syncs++
		return nil
	})

	req := newLocalRequest("POST", "/api/adapters/cursor/toggle", bytes.NewReader([]byte(`{"enabled":false}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("adapter toggle status: got %d, want %d. body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if s.configMgr.Get().Adapters.Cursor.Enabled {
		t.Error("cursor adapter should be disabled in live config")
	}
	if syncs != 1 {
		t.Errorf("adapter sync calls = %d, want 1", syncs)
	}

	reloaded := config.NewManager(dir)
	if err := reloaded.LoadGlobal(); err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.Get().Adapters.Cursor.Enabled {
		t.Error("cursor adapter should persist as disabled")
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

func TestMCPRoutesMounted(t *testing.T) {
	s, _ := testEnv(t)

	// MCP tools list endpoint should be reachable under /mcp/tools.
	req := newLocalRequest("GET", "/mcp/tools", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("MCP tools status: %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	tools, ok := resp["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Error("MCP tools should return non-empty tools list")
	}
}

func TestGitSignalHandler(t *testing.T) {
	s, db := testEnv(t)

	// Checkout event.
	body := bytes.NewReader([]byte(`{"type":"git_checkout","pwd":"/tmp/repo","prev":"abc123","new":"def456"}`))
	req := newLocalRequest("POST", "/api/capture/git-signal", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("git-signal status: got %d, want %d. body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var stored storage.Source
	if err := json.NewDecoder(w.Body).Decode(&stored); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if stored.SignalType != "git_revert" {
		t.Errorf("SignalType = %q, want git_revert", stored.SignalType)
	}
	if stored.AgentName != "git" {
		t.Errorf("AgentName = %q, want git", stored.AgentName)
	}
	if stored.SignalStrength != "strong" {
		t.Errorf("SignalStrength = %q, want strong", stored.SignalStrength)
	}

	// Verify it actually landed in the DB.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sources WHERE agent_name='git'").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("db sources count = %d, want 1", count)
	}

	// Rewrite event.
	body2 := bytes.NewReader([]byte(`{"type":"git_rewrite","op":"rebase","pwd":"/tmp/repo"}`))
	req2 := newLocalRequest("POST", "/api/capture/git-signal", body2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	s.router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Errorf("rewrite status: got %d, want %d", w2.Code, http.StatusCreated)
	}

	// Bad JSON.
	req3 := newLocalRequest("POST", "/api/capture/git-signal", bytes.NewReader([]byte(`not json`)))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	s.router.ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Errorf("bad json status: got %d, want %d", w3.Code, http.StatusBadRequest)
	}
}

func TestDashboardMapEmpty(t *testing.T) {
	s, _ := testEnv(t)
	req := newLocalRequest("GET", "/api/dashboard/map", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["nodes"].([]any); !ok {
		t.Error("expected nodes array")
	}
	if _, ok := resp["edges"].([]any); !ok {
		t.Error("expected edges array")
	}
}

func TestDashboardMapIncludesRuleHitCountAndSourceSnippet(t *testing.T) {
	s, db := testEnv(t)
	ruleRepo := storage.NewRuleRepo(db)
	sourceRepo := storage.NewSourceRepo(db)
	eventRepo := storage.NewEventRepo(db)

	rule := &storage.Rule{
		ID: storage.NewID(), Content: "Use pnpm not npm", Scope: "project", ProjectPath: "/tmp/app",
		Tags: []string{"toolchain"}, Status: "active", Version: 1,
		CreatedAt: storage.Now(), UpdatedAt: storage.Now(),
	}
	if err := ruleRepo.Create(rule); err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	if err := sourceRepo.Create(&storage.Source{
		ID: storage.NewID(), RuleID: rule.ID, SignalType: "manual_edit",
		SignalStrength: "strong", AgentName: "codex", ProjectPath: "/tmp/app",
		RawSnippet: "Don't use npm, use pnpm", Timestamp: storage.Now(),
	}); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	if err := eventRepo.Create(&storage.Event{
		ID: storage.NewID(), RuleID: rule.ID, EventType: "rule_hit",
		AgentName: "codex", ProjectPath: "/tmp/app", TargetPath: "AGENTS.md",
		Details: "rule matched prompt", Timestamp: storage.Now(),
	}); err != nil {
		t.Fatalf("seed event: %v", err)
	}

	req := newLocalRequest("GET", "/api/dashboard/map", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("map status: %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode map: %v", err)
	}
	nodes := resp["nodes"].([]any)
	if len(nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(nodes))
	}
	node := nodes[0].(map[string]any)
	if node["hit_count"].(float64) != 1 {
		t.Fatalf("hit_count = %v, want 1", node["hit_count"])
	}
	if node["source_snippet"] != "Don't use npm, use pnpm" {
		t.Fatalf("source_snippet = %v", node["source_snippet"])
	}
	agents := node["agents"].([]any)
	if len(agents) != 1 || agents[0] != "codex" {
		t.Fatalf("agents = %#v, want codex from source/hit evidence", agents)
	}
}

func TestDashboardMapEdgesFromTagOverlap(t *testing.T) {
	s, _ := testEnv(t)

	// Two rules sharing the "pnpm" tag should produce a whisper edge
	// (1 shared tag, no project → never "strong"; 做减法 sieve).
	for i, content := range []string{"Use pnpm not npm", "Always commit lockfile"} {
		body, _ := json.Marshal(map[string]any{
			"content":    content,
			"scope":      "global",
			"tags":       []string{"pnpm"},
			"category":   "toolchain",
			"confidence": 0.9,
			"status":     "active",
		})
		_ = i
		req := newLocalRequest("POST", "/api/rules", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.router.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("seed rule %d status: %d", i, w.Code)
		}
	}

	// A third rule with no shared tags — should NOT be linked (hidden tier).
	body, _ := json.Marshal(map[string]any{
		"content":    "Use camelCase not snake_case",
		"scope":      "global",
		"tags":       []string{"naming"},
		"category":   "style",
		"confidence": 0.7,
		"status":     "active",
	})
	req := newLocalRequest("POST", "/api/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("seed third rule: %d", w.Code)
	}

	req = newLocalRequest("GET", "/api/dashboard/map", nil)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("map status: %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	edges := resp["edges"].([]any)
	if len(edges) != 1 {
		t.Errorf("edges = %d, want 1 (only the pnpm pair should be linked)", len(edges))
	}
	if len(edges) == 1 {
		e := edges[0].(map[string]any)
		data := e["data"].(map[string]any)
		if data["tier"] != "whisper" {
			t.Errorf("edge tier = %v, want whisper (1 shared tag, no project)", data["tier"])
		}
	}
	// edgeStats reports the sieve breakdown (做减法 transparency).
	if stats, ok := resp["edgeStats"].(map[string]any); ok {
		if stats["hidden"].(float64) < 1 {
			t.Errorf("edgeStats hidden = %v, want >=1 (the naming rule pairs)", stats["hidden"])
		}
	} else {
		t.Error("expected edgeStats in map response")
	}
}

func TestDashboardMapSieveTiers(t *testing.T) {
	s, _ := testEnv(t)

	// Two rules in the same project sharing 3 tags → structure tier.
	for _, content := range []string{"Rule A three tags", "Rule B three tags"} {
		body, _ := json.Marshal(map[string]any{
			"content":      content,
			"scope":        "project",
			"project_path": "/proj/x",
			"tags":         []string{"a", "b", "c"},
			"category":     "code-style",
			"confidence":   0.9,
			"status":       "active",
		})
		req := newLocalRequest("POST", "/api/rules", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.router.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("seed status: %d", w.Code)
		}
	}

	// A conflicted rule in the same project sharing tags → signal conflict edge.
	body, _ := json.Marshal(map[string]any{
		"content":      "Conflicting rule",
		"scope":        "project",
		"project_path": "/proj/x",
		"tags":         []string{"a"},
		"category":     "code-style",
		"confidence":   0.5,
		"status":       "conflicted",
	})
	req := newLocalRequest("POST", "/api/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("seed conflicted status: %d", w.Code)
	}

	req = newLocalRequest("GET", "/api/dashboard/map", nil)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("map status: %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	edges := resp["edges"].([]any)
	var gotSignal, gotStructure, gotWhisper bool
	for _, raw := range edges {
		e := raw.(map[string]any)
		data := e["data"].(map[string]any)
		switch data["tier"] {
		case "signal":
			gotSignal = true
			if data["signalType"] != "conflict" {
				t.Errorf("signalType = %v, want conflict", data["signalType"])
			}
		case "structure":
			gotStructure = true
		case "whisper":
			gotWhisper = true
		}
	}
	if !gotSignal {
		t.Error("expected at least one signal (conflict) edge")
	}
	if !gotStructure {
		t.Error("expected at least one structure edge (3 shared tags)")
	}
	if !gotWhisper {
		t.Error("expected at least one whisper edge (conflicted rule shares 1 tag with peers)")
	}
}

func TestConflictPairsEndpointReturnsRealRelation(t *testing.T) {
	s, db := testEnv(t)
	ruleRepo := storage.NewRuleRepo(db)
	active := &storage.Rule{
		ID: storage.NewID(), Content: "Use pnpm", Scope: "project", ProjectPath: "/proj/x",
		Tags: []string{"toolchain", "pnpm"}, Status: "active", Version: 1,
		CreatedAt: storage.Now(), UpdatedAt: storage.Now(),
	}
	conflicted := &storage.Rule{
		ID: storage.NewID(), Content: "Use npm", Scope: "project", ProjectPath: "/proj/x",
		Tags: []string{"toolchain", "pnpm"}, Status: "conflicted", Version: 1,
		CreatedAt: storage.Now(), UpdatedAt: storage.Now(),
	}
	for _, rule := range []*storage.Rule{active, conflicted} {
		if err := ruleRepo.Create(rule); err != nil {
			t.Fatalf("seed rule: %v", err)
		}
	}

	req := newLocalRequest("GET", "/api/conflicts", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("conflicts status: %d, body: %s", w.Code, w.Body.String())
	}
	var pairs []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&pairs); err != nil {
		t.Fatalf("decode conflicts: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("pairs = %d, want 1", len(pairs))
	}
	if pairs[0]["reason"] == "" {
		t.Fatalf("pair missing reason: %#v", pairs[0])
	}
	if pairs[0]["rule_a"].(map[string]any)["id"] != conflicted.ID {
		t.Fatalf("rule_a should be the conflicted rule: %#v", pairs[0]["rule_a"])
	}
	if pairs[0]["rule_b"].(map[string]any)["id"] != active.ID {
		t.Fatalf("rule_b should be the matching active rule: %#v", pairs[0]["rule_b"])
	}
}

func TestWebSocketBroadcastDelivers(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run(context.Background())

	// Simulate a registered client: a buffered channel feeding WritePump.
	client := &WSClient{send: make(chan []byte, 4)}
	hub.Register(client)

	hub.Broadcast(map[string]any{"event": "test", "n": 42})

	select {
	case got := <-client.send:
		var msg map[string]any
		if err := json.Unmarshal(got, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if msg["event"] != "test" {
			t.Errorf("event = %v, want test", msg["event"])
		}
		// JSON numbers come back as float64.
		if n, _ := msg["n"].(float64); n != 42 {
			t.Errorf("n = %v, want 42", msg["n"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for broadcast")
	}

	hub.Unregister(client)
}

func TestWebSocketBroadcastDropsSlowClient(t *testing.T) {
	hub := NewWebSocketHub()

	// Tiny buffer (1) + a client that never drains it. The second broadcast
	// should hit the 'default' branch in Broadcast and unregister the client.
	client := &WSClient{send: make(chan []byte, 1)}
	hub.Register(client)

	hub.Broadcast(map[string]any{"event": "first"})  // fills the buffer
	hub.Broadcast(map[string]any{"event": "second"}) // should be dropped

	// Unregister runs in a goroutine; give it a beat.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		hub.mu.RLock()
		_, still := hub.clients[client]
		hub.mu.RUnlock()
		if !still {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("slow client should have been unregistered after a dropped broadcast")
}
