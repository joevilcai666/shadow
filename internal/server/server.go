package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/joevilcai666/shadow/internal/adapter"
	"github.com/joevilcai666/shadow/internal/config"
	"github.com/joevilcai666/shadow/internal/storage"
)

//go:embed static/*
var staticAssets embed.FS

// Server is the HTTP API server for the Shadow web console.
type Server struct {
	ruleRepo    *storage.RuleRepo
	sourceRepo  *storage.SourceRepo
	versionRepo *storage.VersionRepo
	configRepo  *storage.ConfigRepo
	projectRepo *storage.ProjectRepo
	configMgr   *config.Manager
	router      *mux.Router
	wsHub       *WebSocketHub
	cfg         config.ServerConfig
	mcpServer   *adapter.MCPServer
}

// New creates a new HTTP server.
func New(
	ruleRepo *storage.RuleRepo,
	sourceRepo *storage.SourceRepo,
	versionRepo *storage.VersionRepo,
	configRepo *storage.ConfigRepo,
	projectRepo *storage.ProjectRepo,
	configMgr *config.Manager,
	cfg config.ServerConfig,
	mcpServer *adapter.MCPServer,
) *Server {
	s := &Server{
		ruleRepo:    ruleRepo,
		sourceRepo:  sourceRepo,
		versionRepo: versionRepo,
		configRepo:  configRepo,
		projectRepo: projectRepo,
		configMgr:   configMgr,
		router:      mux.NewRouter(),
		wsHub:       NewWebSocketHub(),
		cfg:         cfg,
		mcpServer:   mcpServer,
	}
	s.routes()
	return s
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Bind, s.cfg.Port)

	// Verify binding to localhost only.
	ip := net.ParseIP(s.cfg.Bind)
	if !ip.IsLoopback() && s.cfg.Bind != "127.0.0.1" && s.cfg.Bind != "localhost" {
		return fmt.Errorf("refusing to bind to non-localhost address %s (security)", s.cfg.Bind)
	}

	go s.wsHub.Run()

	slog.Info("starting HTTP server", "addr", addr)
	return http.ListenAndServe(addr, s.router)
}

// ServeHTTP implements http.Handler, delegating to the gorilla mux router.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	api := s.router.PathPrefix("/api").Subrouter()
	api.Use(localhostOnly)

	// Rules
	api.HandleFunc("/rules", s.listRules).Methods("GET")
	api.HandleFunc("/rules", s.createRule).Methods("POST")
	api.HandleFunc("/rules/{id}", s.getRule).Methods("GET")
	api.HandleFunc("/rules/{id}", s.updateRule).Methods("PUT")
	api.HandleFunc("/rules/{id}", s.deleteRule).Methods("DELETE")
	api.HandleFunc("/rules/{id}/timeline", s.getRuleTimeline).Methods("GET")
	api.HandleFunc("/rules/{id}/versions", s.getRuleVersions).Methods("GET")
	api.HandleFunc("/rules/{id}/versions/{v}/rollback", s.rollbackRule).Methods("PUT")
	api.HandleFunc("/rules/batch", s.batchRules).Methods("POST")

	// Projects
	api.HandleFunc("/projects", s.listProjects).Methods("GET")
	api.HandleFunc("/projects", s.createProject).Methods("POST")

	// Config
	api.HandleFunc("/config", s.getConfig).Methods("GET")
	api.HandleFunc("/config", s.updateConfig).Methods("PUT")

	// Dashboard
	api.HandleFunc("/dashboard", s.getDashboard).Methods("GET")
	api.HandleFunc("/dashboard/map", s.getDashboardMap).Methods("GET")
	api.HandleFunc("/stats", s.getStats).Methods("GET")

	// Capture
	api.HandleFunc("/capture/toggle", s.toggleCapture).Methods("POST")
	api.HandleFunc("/capture/status", s.captureStatus).Methods("GET")
	api.HandleFunc("/capture/git-signal", s.gitSignal).Methods("POST")

	// Adapters
	api.HandleFunc("/adapters", s.listAdapters).Methods("GET")
	api.HandleFunc("/adapters/{name}/toggle", s.toggleAdapter).Methods("POST")
	api.HandleFunc("/adapters/sync", s.syncAdapters).Methods("POST")

	// WebSocket
	s.router.HandleFunc("/ws", s.handleWebSocket)

	// MCP server routes — mount under /mcp prefix.
	if s.mcpServer != nil {
		mcp := s.router.PathPrefix("/mcp").Subrouter()
		mcp.PathPrefix("/").Handler(s.mcpServer.Handler())
	}

	// Static files (SPA) — must be last.
	s.router.PathPrefix("/").Handler(s.spaHandler())
}

// localhostOnly middleware rejects non-localhost requests.
func localhostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.Host)
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			http.Error(w, "Forbidden: localhost only", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) spaHandler() http.Handler {
	sub, err := fs.Sub(staticAssets, "static")
	if err != nil {
		slog.Error("static assets not found", "error", err)
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file; if not found, serve index.html (SPA routing).
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		f, err := staticAssets.Open("static" + path)
		if err != nil {
			// SPA fallback.
			r.URL.Path = "/"
		} else {
			f.Close()
		}
		fileServer.ServeHTTP(w, r)
	})
}

// --- Handlers ---

func (s *Server) listRules(w http.ResponseWriter, r *http.Request) {
	filter := storage.RuleFilter{
		Scope:   r.URL.Query().Get("scope"),
		Status:  r.URL.Query().Get("status"),
		Search:  r.URL.Query().Get("q"),
		OrderBy: r.URL.Query().Get("sort"),
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		filter.Limit, _ = strconv.Atoi(limitStr)
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		filter.Offset, _ = strconv.Atoi(offsetStr)
	}
	if tagsStr := r.URL.Query().Get("tags"); tagsStr != "" {
		// Simple comma-separated.
		// Production would use proper parsing.
	}

	rules, err := s.ruleRepo.List(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rules == nil {
		rules = []*storage.Rule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) createRule(w http.ResponseWriter, r *http.Request) {
	var rule storage.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	rule.ID = storage.NewID()
	rule.CreatedAt = storage.Now()
	rule.UpdatedAt = storage.Now()
	if rule.Version == 0 {
		rule.Version = 1
	}
	if rule.Status == "" {
		rule.Status = "candidate"
	}
	if rule.Tags == nil {
		rule.Tags = []string{}
	}

	// Privacy check.
	if found, pattern := s.configMgr.ContainsSensitiveData(rule.Content); found {
		writeError(w, http.StatusBadRequest, "rule contains sensitive data matching pattern: "+pattern)
		return
	}

	if err := s.ruleRepo.Create(&rule); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.wsHub.Broadcast(map[string]any{"event": "rule.created", "rule_id": rule.ID})
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) getRule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	rule, err := s.ruleRepo.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rule == nil {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) updateRule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var rule storage.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	rule.ID = id
	rule.UpdatedAt = storage.Now()

	// Privacy check.
	if found, pattern := s.configMgr.ContainsSensitiveData(rule.Content); found {
		writeError(w, http.StatusBadRequest, "rule contains sensitive data matching pattern: "+pattern)
		return
	}

	if err := s.ruleRepo.Update(&rule, "user", "updated via web"); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.wsHub.Broadcast(map[string]any{"event": "rule.updated", "rule_id": id})
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := s.ruleRepo.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) getRuleTimeline(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	sources, err := s.sourceRepo.ListByRuleID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sources == nil {
		sources = []*storage.Source{}
	}
	writeJSON(w, http.StatusOK, sources)
}

func (s *Server) getRuleVersions(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	versions, err := s.versionRepo.ListByRuleID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if versions == nil {
		versions = []*storage.Version{}
	}
	writeJSON(w, http.StatusOK, versions)
}

func (s *Server) rollbackRule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	v, _ := strconv.Atoi(mux.Vars(r)["v"])
	if err := s.versionRepo.Rollback(id, v, "rollback via web"); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rolled back"})
}

func (s *Server) batchRules(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string   `json:"action"` // "activate", "disable", "delete"
		IDs    []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	for _, id := range req.IDs {
		switch req.Action {
		case "delete":
			s.ruleRepo.Delete(id)
		case "activate", "disable":
			rule, _ := s.ruleRepo.GetByID(id)
			if rule != nil {
				if req.Action == "activate" {
					rule.Status = "active"
				} else {
					rule.Status = "disabled"
				}
				rule.UpdatedAt = storage.Now()
				s.ruleRepo.Update(rule, "user", "batch "+req.Action)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "batch complete"})
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.projectRepo.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if projects == nil {
		projects = []*storage.Project{}
	}
	writeJSON(w, http.StatusOK, projects)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var project storage.Project
	if err := json.NewDecoder(r.Body).Decode(&project); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	project.ID = storage.NewID()
	project.CreatedAt = storage.Now()
	if project.Agents == nil {
		project.Agents = []string{}
	}
	if err := s.projectRepo.Create(&project); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.configMgr.Get()
	// Transform to camelCase JSON matching frontend expectations.
	writeJSON(w, http.StatusOK, map[string]any{
		"capture": map[string]any{
			"enabled":  cfg.Capture.Enabled,
			"projects": cfg.Capture.Projects,
		},
		"privacy": map[string]any{
			"exclude_patterns": cfg.Privacy.ExcludePatterns,
			"deny_patterns":    cfg.Privacy.DenyPatterns,
		},
		"distill": map[string]any{
			"threshold":              cfg.Distill.Threshold,
			"auto_activate_low_risk": cfg.Distill.AutoActivateLowRisk,
			"batch_mode":             cfg.Distill.BatchMode,
			"llm_model":              cfg.Distill.LLMModel,
		},
		"adapters": map[string]any{
			"claude_code": map[string]any{
				"enabled":     cfg.Adapters.ClaudeCode.Enabled,
				"global_path": cfg.Adapters.ClaudeCode.GlobalPath,
			},
			"cursor": map[string]any{
				"enabled":     cfg.Adapters.Cursor.Enabled,
				"global_path": cfg.Adapters.Cursor.GlobalPath,
			},
			"codex": map[string]any{
				"enabled":     cfg.Adapters.Codex.Enabled,
				"global_path": cfg.Adapters.Codex.GlobalPath,
			},
		},
		"server": map[string]any{
			"port": cfg.Server.Port,
			"bind": cfg.Server.Bind,
		},
	})
}

func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	// Apply individual config updates.
	cfg := s.configMgr.Get()
	for key, val := range updates {
		switch key {
		case "capture_enabled":
			if b, ok := val.(bool); ok {
				cfg.Capture.Enabled = b
			}
		case "distill_threshold":
			if s, ok := val.(string); ok {
				cfg.Distill.Threshold = s
			}
		case "auto_activate_low_risk":
			if b, ok := val.(bool); ok {
				cfg.Distill.AutoActivateLowRisk = b
			}
		case "batch_mode":
			if b, ok := val.(bool); ok {
				cfg.Distill.BatchMode = b
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) getDashboard(w http.ResponseWriter, r *http.Request) {
	total, _ := s.ruleRepo.Count(storage.RuleFilter{})
	active, _ := s.ruleRepo.Count(storage.RuleFilter{Status: "active"})
	candidate, _ := s.ruleRepo.Count(storage.RuleFilter{Status: "candidate"})
	disabled, _ := s.ruleRepo.Count(storage.RuleFilter{Status: "disabled"})
	conflicted, _ := s.ruleRepo.Count(storage.RuleFilter{Status: "conflicted"})

	// Get source stats.
	sourceCount, _ := s.sourceRepo.CountTotal()
	agentStats, _ := s.sourceRepo.CountByAgent()

	// Get project count.
	projects, _ := s.projectRepo.List()
	projectCount := 0
	if projects != nil {
		projectCount = len(projects)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total_rules":     total,
		"active_rules":    active,
		"candidate_rules": candidate,
		"disabled_rules":  disabled,
		"conflicted_rules": conflicted,
		"total_sources":   sourceCount,
		"project_count":   projectCount,
		"agent_stats":     agentStats,
	})
}

// getDashboardMap powers the Memory Map canvas (web/src/pages/MemoryMapPage).
//
// It returns every rule translated into the React-Flow-shaped node the
// frontend expects (MemoryNodeData) plus a list of edges derived from
// tag overlap (>=1 shared tag = "medium" relation, >=2 = "strong").
// The frontend's category mapping is intentionally loose — we map our
// internal category vocabulary to the 3 frontend buckets (code /
// architecture / practice) by simple keyword match; otherwise the rule
// falls into 'practice' as a safe default.
func (s *Server) getDashboardMap(w http.ResponseWriter, r *http.Request) {
	rules, err := s.ruleRepo.List(storage.RuleFilter{Limit: 500})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build nodes.
	nodes := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		nodes = append(nodes, map[string]any{
			"id":              rule.ID,
			"title":           firstLine(rule.Content, 40),
			"content":         rule.Content,
			"category":        mapCategory(rule.Category, rule.Tags),
			"status":          mapStatus(rule.Status),
			"confidence":      rule.Confidence,
			"version":         rule.Version,
			"tags":            rule.Tags,
			"trigger_context": rule.TriggerContext,
			"project_path":    rule.ProjectPath,
			"agents":          ruleAgents(rule.ProjectPath), // best-effort
			"hit_count":       0,                            // filled later if you wire a counter
			"created_at":      rule.CreatedAt,
			"updated_at":      rule.UpdatedAt,
		})
	}

	// Build edges from tag overlap. O(n^2) over tag sets is fine at
	// v1 scale (<500 rules). Each pair appears at most once.
	edges := []map[string]any{}
	for i := 0; i < len(rules); i++ {
		for j := i + 1; j < len(rules); j++ {
			shared := tagOverlap(rules[i].Tags, rules[j].Tags)
			if shared == 0 {
				continue
			}
			kind := "weak"
			switch {
			case shared >= 2:
				kind = "strong"
			case shared == 1:
				kind = "medium"
			}
			edges = append(edges, map[string]any{
				"source": rules[i].ID,
				"target": rules[j].ID,
				"data": map[string]any{
					"kind":   kind,
					"reason": "shared " + itoa(shared) + " tag(s)",
				},
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"nodes":     nodes,
		"edges":     edges,
		"generated": len(nodes),
	})
}

// --- dashboard/map helpers ---

// mapCategory collapses our internal category vocabulary into the 3
// frontend buckets. Unknown categories fall into 'practice'.
func mapCategory(category string, tags []string) string {
	c := category
	for _, t := range tags {
		if t != "" {
			c = t
			break
		}
	}
	switch c {
	case "toolchain", "tooling", "code-style", "style":
		return "code"
	case "arch", "architecture":
		return "architecture"
	}
	return "practice"
}

// mapStatus collapses 4 internal statuses into the 3 frontend buckets.
func mapStatus(status string) string {
	switch status {
	case "conflicted":
		return "conflicted"
	case "active":
		return "active"
	}
	return "other"
}

// tagOverlap returns how many strings the two slices share.
func tagOverlap(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, len(a))
	for _, t := range a {
		seen[t] = struct{}{}
	}
	n := 0
	for _, t := range b {
		if _, ok := seen[t]; ok {
			n++
		}
	}
	return n
}

// ruleAgents returns the project-path-derived "agent" label for a rule.
// Without a per-rule "agents" array on disk (we don't store one yet),
// the only signal we have is the scope: project-scoped rules are
// attributed to all known agents; global-scoped rules are too. The
// frontend treats agents as display labels, so a coarse list is fine.
func ruleAgents(_ string) []string {
	return []string{"Claude Code", "Cursor", "Codex"}
}

// firstLine trims a string to its first line and shortens if longer
// than maxLen (suffix "...").
func firstLine(s string, maxLen int) string {
	for i, r := range s {
		if r == '\n' {
			s = s[:i]
			break
		}
	}
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// itoa is a small int-to-string helper to avoid pulling strconv just
// for edge reason labels.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

func (s *Server) getStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version": "0.1.0",
		"status":  "running",
	})
}

func (s *Server) toggleCapture(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
}

func (s *Server) captureStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": s.configMgr.Get().Capture.Enabled,
	})
}

// gitSignal ingests a signal posted by a Shadow-installed git hook (see
// internal/capture/git_hooks.go). The hook posts when the user does
// `git checkout` (reset/revert) or `git rebase/amend` (rewrite). These
// events are strong signals that the user is correcting prior agent output.
func (s *Server) gitSignal(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Type string `json:"type"`
		Pwd  string `json:"pwd"`
		Prev string `json:"prev"`
		New  string `json:"new"`
		Op   string `json:"op"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Build a human-readable snippet describing the git event.
	var snippet string
	switch payload.Type {
	case "git_checkout":
		snippet = fmt.Sprintf("git checkout from %s to %s in %s", payload.Prev, payload.New, payload.Pwd)
	case "git_rewrite":
		snippet = fmt.Sprintf("git %s (rewrite) in %s", payload.Op, payload.Pwd)
	default:
		snippet = fmt.Sprintf("git event %s in %s", payload.Type, payload.Pwd)
	}

	// Privacy check.
	if found, pattern := s.configMgr.ContainsSensitiveData(snippet); found {
		writeError(w, http.StatusBadRequest, "git signal contains sensitive data matching pattern: "+pattern)
		return
	}

	source := &storage.Source{
		ID:                    storage.NewID(),
		SignalType:            "git_revert",
		SignalStrength:        "strong",
		AgentName:             "git",
		ProjectPath:           payload.Pwd,
		RawSnippet:            snippet,
		Timestamp:             storage.Now(),
		ConfidenceContribution: 0.7,
	}
	if err := s.sourceRepo.Create(source); err != nil {
		writeError(w, http.StatusInternalServerError, "create source: "+err.Error())
		return
	}

	s.wsHub.Broadcast(map[string]any{"event": "source.created", "source_id": source.ID, "agent": "git"})
	writeJSON(w, http.StatusCreated, source)
}

// --- WebSocket ---

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade", "error", err)
		return
	}
	client := &WSClient{conn: conn, send: make(chan []byte, 64)}
	s.wsHub.Register(client)
	go client.WritePump()
}

// --- Adapter Handlers ---

func (s *Server) listAdapters(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	backupDir := home + "/.shadow/backups"

	adapters := []map[string]any{
		{
			"name":        "claude_code",
			"label":       "Claude Code",
			"installed":   adapter.NewClaudeCodeAdapter(backupDir).IsInstalled(),
			"enabled":     s.configMgr.Get().Adapters.ClaudeCode.Enabled,
			"target_path": "CLAUDE.md (project) + ~/.claude/CLAUDE.md (global)",
		},
		{
			"name":        "cursor",
			"label":       "Cursor",
			"installed":   adapter.NewCursorAdapter(backupDir).IsInstalled(),
			"enabled":     s.configMgr.Get().Adapters.Cursor.Enabled,
			"target_path": ".cursorrules (project) + ~/.cursorrules (global)",
		},
		{
			"name":        "codex",
			"label":       "Codex",
			"installed":   adapter.NewCodexAdapter(backupDir).IsInstalled(),
			"enabled":     s.configMgr.Get().Adapters.Codex.Enabled,
			"target_path": "AGENTS.md (project) + ~/AGENTS.md (global)",
		},
		{
			"name":        "copilot",
			"label":       "GitHub Copilot",
			"installed":   adapter.NewCopilotAdapter(backupDir).IsInstalled(),
			"enabled":     s.configMgr.Get().Adapters.Copilot.Enabled,
			"target_path": ".github/copilot-instructions.md (project) + ~/.copilot/instructions.md (global)",
		},
	}

	writeJSON(w, http.StatusOK, adapters)
}

func (s *Server) toggleAdapter(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	cfg := s.configMgr.Get()
	switch name {
	case "claude_code":
		cfg.Adapters.ClaudeCode.Enabled = req.Enabled
	case "cursor":
		cfg.Adapters.Cursor.Enabled = req.Enabled
	case "codex":
		cfg.Adapters.Codex.Enabled = req.Enabled
	case "copilot":
		cfg.Adapters.Copilot.Enabled = req.Enabled
	default:
		writeError(w, http.StatusNotFound, "adapter not found: "+name)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) syncAdapters(w http.ResponseWriter, r *http.Request) {
	// Trigger adapter sync. In a real implementation, this would signal the daemon.
	writeJSON(w, http.StatusOK, map[string]string{"status": "sync triggered"})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
