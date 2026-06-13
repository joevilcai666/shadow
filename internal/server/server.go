package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

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
	ruleRepo       *storage.RuleRepo
	sourceRepo     *storage.SourceRepo
	eventRepo      *storage.EventRepo
	versionRepo    *storage.VersionRepo
	configRepo     *storage.ConfigRepo
	projectRepo    *storage.ProjectRepo
	userMemoryRepo *storage.UserMemoryRepo
	configMgr      *config.Manager
	router         *mux.Router
	wsHub          *WebSocketHub
	cfg            config.ServerConfig
	mcpServer      *adapter.MCPServer

	// Optional callback hooks. The daemon wires these in after New() so
	// the HTTP layer doesn't need to know about the capture engine or
	// the adapter loop. Nil-safe — if not wired the endpoints return
	// 503 with a clear "daemon-only" message.
	onToggleCapture func() error
	onSyncAdapters  func() error
}

// SetControlHooks wires daemon-side callbacks into the HTTP server. Must
// be called after New() before the server is started.
func (s *Server) SetControlHooks(toggleCapture, syncAdapters func() error) {
	s.onToggleCapture = toggleCapture
	s.onSyncAdapters = syncAdapters
}

// New creates a new HTTP server.
func New(
	ruleRepo *storage.RuleRepo,
	sourceRepo *storage.SourceRepo,
	eventRepo *storage.EventRepo,
	versionRepo *storage.VersionRepo,
	configRepo *storage.ConfigRepo,
	projectRepo *storage.ProjectRepo,
	userMemoryRepo *storage.UserMemoryRepo,
	configMgr *config.Manager,
	cfg config.ServerConfig,
	mcpServer *adapter.MCPServer,
) *Server {
	s := &Server{
		ruleRepo:       ruleRepo,
		sourceRepo:     sourceRepo,
		eventRepo:      eventRepo,
		versionRepo:    versionRepo,
		configRepo:     configRepo,
		projectRepo:    projectRepo,
		userMemoryRepo: userMemoryRepo,
		configMgr:      configMgr,
		router:         mux.NewRouter(),
		wsHub:          NewWebSocketHub(),
		cfg:            cfg,
		mcpServer:      mcpServer,
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

	go s.wsHub.Run(context.Background())

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
	api.HandleFunc("/rules/{id}/events", s.getRuleEvents).Methods("GET")
	api.HandleFunc("/rules/{id}/versions", s.getRuleVersions).Methods("GET")
	api.HandleFunc("/rules/{id}/versions/{v}/rollback", s.rollbackRule).Methods("PUT")
	api.HandleFunc("/rules/{id}/hit", s.recordRuleHit).Methods("POST")
	api.HandleFunc("/rules/batch", s.batchRules).Methods("POST")

	// User memories (cross-agent personal context — SHADOW-038)
	api.HandleFunc("/memories", s.listMemories).Methods("GET")
	api.HandleFunc("/memories", s.createMemory).Methods("POST")
	api.HandleFunc("/memories/{id}", s.deleteMemory).Methods("DELETE")
	api.HandleFunc("/export", s.exportPackage).Methods("GET")

	// Projects
	api.HandleFunc("/projects", s.listProjects).Methods("GET")
	api.HandleFunc("/projects", s.createProject).Methods("POST")

	// Config
	api.HandleFunc("/config", s.getConfig).Methods("GET")
	api.HandleFunc("/config", s.updateConfig).Methods("PUT")

	// Dashboard
	api.HandleFunc("/dashboard", s.getDashboard).Methods("GET")
	api.HandleFunc("/dashboard/map", s.getDashboardMap).Methods("GET")
	api.HandleFunc("/conflicts", s.listConflicts).Methods("GET")
	api.HandleFunc("/stats", s.getStats).Methods("GET")
	api.HandleFunc("/stats/hit-rate", s.getHitRate).Methods("GET")

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
		// Comma-separated tag filter — matches rules containing ALL listed tags.
		var tags []string
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
		filter.Tags = tags
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
	if rule.Status == "active" {
		s.triggerAdapterSync()
	}
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
	existing, err := s.ruleRepo.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}

	var patch map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	rule := *existing
	if err := applyRulePatch(&rule, patch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
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
	if existing.Status == "active" || rule.Status == "active" {
		s.triggerAdapterSync()
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	existing, err := s.ruleRepo.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.ruleRepo.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing != nil && existing.Status == "active" {
		s.triggerAdapterSync()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// MARK: - User memories (SHADOW-038 /store_memory)

// validMemoryCategories is the allow-list for UserMemory.Category.
var validMemoryCategories = map[string]bool{
	"preference": true,
	"convention": true,
	"context":    true,
}

// createMemory stores a user-authored, cross-agent personal memory.
// UserMemory has no status/decay (unlike Rule) — it is always-active context
// the user explicitly chose to persist.
func (s *Server) createMemory(w http.ResponseWriter, r *http.Request) {
	var m storage.UserMemory
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(m.Content) == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	if !validMemoryCategories[m.Category] {
		writeError(w, http.StatusBadRequest,
			"category must be one of: preference, convention, context")
		return
	}
	// Single-user, local-first product — no auth. Default to "local" unless
	// the caller (e.g. --user flag) supplies a different id.
	if m.UserID == "" {
		m.UserID = "local"
	}
	if m.Tags == nil {
		m.Tags = []string{}
	}
	m.ID = storage.NewID()
	m.CreatedAt = storage.Now()
	m.UpdatedAt = storage.Now()

	// Privacy check — same guard as createRule.
	if found, pattern := s.configMgr.ContainsSensitiveData(m.Content); found {
		writeError(w, http.StatusBadRequest, "memory contains sensitive data matching pattern: "+pattern)
		return
	}

	if err := s.userMemoryRepo.Create(&m); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.wsHub.Broadcast(map[string]any{"event": "memory.created", "memory_id": m.ID})
	s.triggerAdapterSync()
	writeJSON(w, http.StatusCreated, m)
}

// listMemories returns user memories, optionally filtered by ?user= and ?project=.
func (s *Server) listMemories(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	project := r.URL.Query().Get("project")
	memories, err := s.userMemoryRepo.List(user, project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if memories == nil {
		memories = []*storage.UserMemory{}
	}
	writeJSON(w, http.StatusOK, memories)
}

// deleteMemory removes a user memory by id.
func (s *Server) deleteMemory(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	existing, err := s.userMemoryRepo.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.userMemoryRepo.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing != nil {
		s.triggerAdapterSync()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type exportPackage struct {
	SchemaVersion string                `json:"schema_version"`
	ExportedAt    string                `json:"exported_at"`
	RuleCount     int                   `json:"rule_count"`
	MemoryCount   int                   `json:"memory_count"`
	Rules         []*storage.Rule       `json:"rules"`
	Memories      []*storage.UserMemory `json:"memories"`
}

func (s *Server) exportPackage(w http.ResponseWriter, r *http.Request) {
	rules, err := s.ruleRepo.List(storage.RuleFilter{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export rules: "+err.Error())
		return
	}
	memories, err := s.userMemoryRepo.List("", "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export memories: "+err.Error())
		return
	}
	if rules == nil {
		rules = []*storage.Rule{}
	}
	if memories == nil {
		memories = []*storage.UserMemory{}
	}

	w.Header().Set("Content-Disposition", `attachment; filename="shadow-export.json"`)
	writeJSON(w, http.StatusOK, exportPackage{
		SchemaVersion: "shadow.export.v1",
		ExportedAt:    storage.Now(),
		RuleCount:     len(rules),
		MemoryCount:   len(memories),
		Rules:         rules,
		Memories:      memories,
	})
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

func (s *Server) getRuleEvents(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	events, err := s.eventRepo.ListByRuleID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []*storage.Event{}
	}
	writeJSON(w, http.StatusOK, events)
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
	existing, err := s.ruleRepo.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.versionRepo.Rollback(id, v, "rollback via web"); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing != nil && existing.Status == "active" {
		s.triggerAdapterSync()
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

	shouldSync := false
	for _, id := range req.IDs {
		switch req.Action {
		case "delete":
			rule, _ := s.ruleRepo.GetByID(id)
			if rule != nil && rule.Status == "active" {
				shouldSync = true
			}
			s.ruleRepo.Delete(id)
		case "activate", "disable":
			rule, _ := s.ruleRepo.GetByID(id)
			if rule != nil {
				wasActive := rule.Status == "active"
				if req.Action == "activate" {
					rule.Status = "active"
				} else {
					rule.Status = "disabled"
				}
				rule.UpdatedAt = storage.Now()
				s.ruleRepo.Update(rule, "user", "batch "+req.Action)
				if wasActive || rule.Status == "active" {
					shouldSync = true
				}
			}
		}
	}
	if shouldSync {
		s.triggerAdapterSync()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "batch complete"})
}

func (s *Server) recordRuleHit(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		AgentName   string `json:"agent_name"`
		ProjectPath string `json:"project_path"`
		TargetPath  string `json:"target_path"`
		Details     string `json:"details"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.AgentName == "" {
		req.AgentName = "unknown"
	}
	if req.ProjectPath == "" {
		req.ProjectPath = rule.ProjectPath
	}
	if req.Details == "" {
		req.Details = "rule surfaced via shadow task"
	}

	now := storage.Now()
	event := &storage.Event{
		ID:          storage.NewID(),
		RuleID:      id,
		EventType:   "rule_hit",
		AgentName:   req.AgentName,
		ProjectPath: req.ProjectPath,
		TargetPath:  req.TargetPath,
		Details:     req.Details,
		Timestamp:   now,
	}
	if err := s.eventRepo.Create(event); err != nil {
		writeError(w, http.StatusInternalServerError, "record hit: "+err.Error())
		return
	}
	// A hit refreshes decay: the rule just proved useful, so restore it to
	// near-full confidence (30-day half-life from now). (SHADOW-041)
	decayScore := storage.ComputeDecayScore(rule.Confidence, now)
	if err := s.ruleRepo.TouchHit(id, now, decayScore); err != nil {
		// Non-fatal — the event was recorded; just log the decay refresh failure.
		slog.Warn("refresh decay after hit", "rule_id", id, "err", err)
	}
	s.wsHub.Broadcast(map[string]any{"event": "rule.hit", "rule_id": id, "agent": event.AgentName})
	writeJSON(w, http.StatusCreated, event)
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
			"openclaw": map[string]any{
				"enabled":     cfg.Adapters.OpenClaw.Enabled,
				"global_path": cfg.Adapters.OpenClaw.GlobalPath,
			},
			"copilot": map[string]any{
				"enabled":     cfg.Adapters.Copilot.Enabled,
				"global_path": cfg.Adapters.Copilot.GlobalPath,
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
	adapterChanged := false
	if err := s.configMgr.UpdateGlobal(func(cfg *config.Config) {
		adapterChanged = applyConfigUpdates(cfg, updates)
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "save config: "+err.Error())
		return
	}
	if adapterChanged {
		s.triggerAdapterSync()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) getDashboard(w http.ResponseWriter, r *http.Request) {
	statusCounts, err := s.ruleRepo.StatusCounts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get source stats.
	sourceCount, _ := s.sourceRepo.CountTotal()
	agentStats, _ := s.sourceRepo.CountByAgent()

	// Get project count.
	projects, _ := s.projectRepo.List()
	projectCount := 0
	if projects != nil {
		projectCount = len(projects)
	}

	hitCounts, _ := s.eventRepo.CountRuleHits()
	totalHits := 0
	for _, count := range hitCounts {
		totalHits += count
	}
	agentCoverage, _ := s.eventRepo.CountRuleHitsByAgent()
	adapterSync := map[string]any{}
	for _, name := range []string{"claude_code", "cursor", "codex", "openclaw", "copilot"} {
		if latest, err := s.eventRepo.LatestSyncByAgent(name); err == nil && latest != nil {
			adapterSync[name] = latest
		}
	}
	health := []map[string]string{}
	if statusCounts.Active == 0 {
		health = append(health, map[string]string{
			"level": "warning", "message": "No active rules yet; review candidates to complete the memory loop.",
		})
	}
	if totalHits == 0 && statusCounts.Active > 0 {
		health = append(health, map[string]string{
			"level": "info", "message": "Active rules have synced, but no hit events have been observed yet.",
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total_rules":      statusCounts.Total,
		"active_rules":     statusCounts.Active,
		"candidate_rules":  statusCounts.Candidate,
		"disabled_rules":   statusCounts.Disabled,
		"conflicted_rules": statusCounts.Conflicted,
		"total_sources":    sourceCount,
		"project_count":    projectCount,
		"agent_stats":      agentStats,
		"total_rule_hits":  totalHits,
		"agent_coverage":   agentCoverage,
		"adapter_sync":     adapterSync,
		"health":           health,
		"hit_rate":         s.computeHitRate(statusCounts.Active),
	})
}

// computeHitRate builds the hit-rate summary used by /api/stats/hit-rate and
// embedded in the dashboard. activeRules is passed in to avoid a second count
// query when the caller already has it. (SHADOW-041)
//
// hit_rate_pct = distinct active rules hit this week / total active rules.
// This is a Type-A proxy (rules actually surfaced/used) for the PRD's
// aspirational Type-C (user-perceived) metric, which is not directly measurable.
func (s *Server) computeHitRate(activeRules int) map[string]any {
	distinct7d, _ := s.eventRepo.DistinctHitRulesLastDays(7)
	hitsThisWeek, _ := s.eventRepo.CountRuleHitsLastDays(7)
	hitsLast14, _ := s.eventRepo.CountRuleHitsLastDays(14)
	// last-week = hits in [7,14) days = (last 14) − (last 7).
	hitsLastWeek := hitsLast14 - hitsThisWeek
	if hitsLastWeek < 0 {
		hitsLastWeek = 0
	}

	ratePct := 0
	if activeRules > 0 {
		ratePct = distinct7d * 100 / activeRules
	}

	trend := "equal"
	switch {
	case hitsThisWeek > hitsLastWeek:
		trend = "up"
	case hitsThisWeek < hitsLastWeek:
		trend = "down"
	}

	// low-hit = active rules not surfaced at all in the last week.
	lowHit := 0
	if activeRules > 0 {
		lowHit = activeRules - distinct7d
		if lowHit < 0 {
			lowHit = 0
		}
	}

	lastHit := map[string]any(nil)
	if latest, err := s.eventRepo.LatestRuleHit(); err == nil && latest != nil {
		entry := map[string]any{
			"rule_id":    latest.RuleID,
			"agent_name": latest.AgentName,
			"timestamp":  latest.Timestamp,
		}
		if rule, err := s.ruleRepo.GetByID(latest.RuleID); err == nil && rule != nil {
			entry["content"] = rule.Content
		}
		lastHit = entry
	}

	return map[string]any{
		"active_rules":          activeRules,
		"distinct_hit_rules_7d": distinct7d,
		"hit_rate_pct":          ratePct,
		"hits_this_week":        hitsThisWeek,
		"hits_last_week":        hitsLastWeek,
		"trend":                 trend,
		"low_hit_count":         lowHit,
		"last_hit":              lastHit,
	}
}

// getHitRate returns the hit-rate summary (SHADOW-041).
func (s *Server) getHitRate(w http.ResponseWriter, r *http.Request) {
	active, _ := s.ruleRepo.Count(storage.RuleFilter{Status: "active"})
	writeJSON(w, http.StatusOK, s.computeHitRate(active))
}

// getDashboardMap powers the Memory Map canvas (web/src/pages/MemoryMapPage).
//
// It returns every rule translated into the React-Flow-shaped node the
// frontend expects (MemoryNodeData) plus a list of edges classified into
// a 3-tier "sieve" (做减法 philosophy):
//
//   - signal:    conflict edges — always rendered (alerts, not info)
//   - structure: strong relations — shown by default (top-N per node)
//   - whisper:   weak relations — shown on demand
//   - hidden:    no tag overlap + no conflict — never generated
//
// The frontend's category mapping is intentionally loose — we map our
// internal category vocabulary to the 3 frontend buckets (code /
// architecture / practice) by simple keyword match; otherwise the rule
// falls into 'practice' as a safe default.
func (s *Server) getDashboardMap(w http.ResponseWriter, r *http.Request) {
	rules, err := s.ruleRepo.List(storage.RuleFilter{Limit: 300})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	hitCounts, _ := s.eventRepo.CountRuleHits()
	ruleIDs := make([]string, 0, len(rules))
	tagSets := make([]map[string]struct{}, len(rules))
	for i, rule := range rules {
		ruleIDs = append(ruleIDs, rule.ID)
		tagSets[i] = newTagSet(rule.Tags)
	}
	sourceEvidence, _ := s.sourceRepo.EvidenceByRuleIDs(ruleIDs)
	eventAgents, _ := s.eventRepo.AgentsByRuleIDs(ruleIDs)

	// Build nodes.
	nodes := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		sourceSnippet, agents := mergeRuleEvidence(rule.ID, sourceEvidence, eventAgents)
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
			"agents":          agents,
			"hit_count":       hitCounts[rule.ID],
			"source_snippet":  sourceSnippet,
			"created_at":      rule.CreatedAt,
			"updated_at":      rule.UpdatedAt,
		})
	}

	edges := []map[string]any{}
	stats := map[string]int{"signal": 0, "structure": 0, "whisper": 0, "hidden": 0}

	// Pass 1 — conflict signal edges. For each conflicted rule, link it to
	// its single most-similar peer in the same project. One conflict edge
	// per conflicted rule keeps the "sieve" honest (做减法: a conflict is an
	// alert, not a graph explosion).
	for i := range rules {
		if rules[i].Status != "conflicted" {
			continue
		}
		bestJ, bestShared := -1, 0
		for j := range rules {
			if i == j {
				continue
			}
			if rules[i].ProjectPath == "" || rules[i].ProjectPath != rules[j].ProjectPath {
				continue
			}
			if s := tagOverlapSet(tagSets[i], rules[j].Tags); s > bestShared {
				bestShared, bestJ = s, j
			}
		}
		if bestJ >= 0 && bestShared > 0 {
			edges = append(edges, map[string]any{
				"source": rules[i].ID,
				"target": rules[bestJ].ID,
				"data": map[string]any{
					"tier":       "signal",
					"signalType": "conflict",
					"score":      1.0,
					"reason":     "冲突：同项目矛盾规则",
				},
			})
			stats["signal"]++
		}
	}

	// Pass 2 — tag-overlap scan. Collect candidates per node instead of
	// emitting every pair. After the scan, each node keeps only its best
	// structure/whisper edges (de-duplicated A↔B), capping at 8 structure
	// and 4 whisper edges per node. This follows the "做减法" philosophy:
	// without the cap, 500 rules with a shared tag produce ~124K whisper
	// edges (~11 MB of JSON) and make the Memory Map unusable.
	//
	// Candidates are collected into per-node buckets during the O(n²) sweep
	// and then trimmed. The node-edge budget constants live here so the
	// entire sieve is visible in one place.
	const maxStructurePerNode = 8
	const maxWhisperPerNode = 4
	type edgeCandidate struct {
		source, target string
		tier           string
		score          float64
		reason         string
	}
	nodeEdges := make(map[string][]edgeCandidate, len(rules))

	for i := 0; i < len(rules); i++ {
		for j := i + 1; j < len(rules); j++ {
			shared := tagOverlapSet(tagSets[i], rules[j].Tags)
			if shared == 0 {
				stats["hidden"]++
				continue
			}
			sameProject := rules[i].ProjectPath != "" && rules[i].ProjectPath == rules[j].ProjectPath

			tier, score, reason := classifyEdge(shared, len(rules[i].Tags), len(rules[j].Tags), sameProject)
			if tier == "hidden" {
				stats["hidden"]++
				continue
			}
			ec := edgeCandidate{
				source: rules[i].ID, target: rules[j].ID,
				tier: tier, score: score, reason: reason,
			}
			nodeEdges[rules[i].ID] = append(nodeEdges[rules[i].ID], ec)
			// Record reverse direction so the other node's budget sees it too.
			rev := ec
			rev.source, rev.target = ec.target, ec.source
			nodeEdges[rules[j].ID] = append(nodeEdges[rules[j].ID], rev)
		}
	}

	// Trim: keep per-node budget, de-duplicate A↔B.
	seen := make(map[string]struct{}, len(rules)*4)
	for _, ecs := range nodeEdges {
		structCount := 0
		whispCount := 0
		for _, ec := range ecs {
			key := edgeKey(ec.source, ec.target)
			if _, ok := seen[key]; ok {
				continue
			}
			switch ec.tier {
			case "structure":
				if structCount >= maxStructurePerNode {
					continue
				}
				structCount++
			case "whisper":
				if whispCount >= maxWhisperPerNode {
					continue
				}
				whispCount++
			default:
				continue // signal edges were emitted in Pass 1
			}
			seen[key] = struct{}{}
			edges = append(edges, map[string]any{
				"source": ec.source,
				"target": ec.target,
				"data": map[string]any{
					"tier":   ec.tier,
					"score":  ec.score,
					"reason": ec.reason,
				},
			})
			stats[ec.tier]++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"nodes":     nodes,
		"edges":     edges,
		"generated": len(nodes),
		"edgeStats": map[string]int{
			"signal":    stats["signal"],
			"structure": stats["structure"],
			"whisper":   stats["whisper"],
			"hidden":    stats["hidden"],
		},
	})
}

func (s *Server) listConflicts(w http.ResponseWriter, r *http.Request) {
	rules, err := s.ruleRepo.List(storage.RuleFilter{Limit: 500})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, buildConflictPairs(rules))
}

func buildConflictPairs(rules []*storage.Rule) []map[string]any {
	pairs := []map[string]any{}
	for i := range rules {
		if rules[i].Status != "conflicted" {
			continue
		}
		bestJ, bestShared := -1, 0
		for j := range rules {
			if i == j {
				continue
			}
			if rules[i].ProjectPath == "" || rules[i].ProjectPath != rules[j].ProjectPath {
				continue
			}
			if shared := tagOverlap(rules[i].Tags, rules[j].Tags); shared > bestShared {
				bestShared, bestJ = shared, j
			}
		}
		if bestJ < 0 || bestShared == 0 {
			continue
		}
		score := 0.7 + float64(bestShared)*0.1
		if score > 1 {
			score = 1
		}
		pairs = append(pairs, map[string]any{
			"rule_a": rules[i],
			"rule_b": rules[bestJ],
			"score":  score,
			"reason": "同项目共享 " + itoa(bestShared) + " 个标签，且其中一条规则处于 conflicted 状态",
		})
	}
	return pairs
}

// classifyEdge maps a tag-overlap pair onto a sieve tier.
//
// Rule (逻辑是底线 — each branch is verifiable):
//   - shared >= 3 tags           → structure (strong tag cluster)
//   - shared == 2 + same project → structure
//   - shared == 2 (cross-project)→ whisper
//   - shared == 1                → whisper (a single shared tag is never "strong")
//
// score is used by the frontend to rank edges within a tier (top-N).
func classifyEdge(shared, tagsA, tagsB int, sameProject bool) (tier string, score float64, reason string) {
	maxTags := tagsA
	if tagsB > maxTags {
		maxTags = tagsB
	}
	if maxTags == 0 {
		maxTags = 1
	}
	score = float64(shared) / float64(maxTags)
	if sameProject {
		score += 0.2
	}
	reason = "shared " + itoa(shared) + " tag(s)"
	if sameProject {
		reason = "same project · " + reason
	}

	switch {
	case shared >= 3:
		return "structure", score, reason
	case shared == 2:
		if sameProject {
			return "structure", score, reason
		}
		return "whisper", score, reason
	default: // shared == 1
		return "whisper", score, reason
	}
}

// --- dashboard/map helpers ---

func (s *Server) ruleEvidence(ruleID string) (string, []string) {
	agentSet := map[string]struct{}{}
	sourceSnippet := ""
	sources, err := s.sourceRepo.ListByRuleID(ruleID)
	if err == nil {
		for _, source := range sources {
			if sourceSnippet == "" && source.RawSnippet != "" {
				sourceSnippet = source.RawSnippet
			}
			if source.AgentName != "" {
				agentSet[source.AgentName] = struct{}{}
			}
		}
	}
	eventAgents, err := s.eventRepo.AgentsForRule(ruleID)
	if err == nil {
		for _, agent := range eventAgents {
			agentSet[agent] = struct{}{}
		}
	}

	agents := make([]string, 0, len(agentSet))
	for agent := range agentSet {
		agents = append(agents, agent)
	}
	if len(agents) == 0 {
		agents = ruleAgents("")
	}
	return sourceSnippet, agents
}

func mergeRuleEvidence(ruleID string, sourceEvidence map[string]storage.SourceEvidence, eventAgents map[string]map[string]bool) (string, []string) {
	agentSet := map[string]struct{}{}
	sourceSnippet := ""
	if ev, ok := sourceEvidence[ruleID]; ok {
		sourceSnippet = ev.FirstSnippet
		for agent := range ev.Agents {
			agentSet[agent] = struct{}{}
		}
	}
	for agent := range eventAgents[ruleID] {
		agentSet[agent] = struct{}{}
	}

	agents := make([]string, 0, len(agentSet))
	for agent := range agentSet {
		agents = append(agents, agent)
	}
	if len(agents) == 0 {
		agents = ruleAgents("")
	}
	return sourceSnippet, agents
}

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

// edgeKey produces a canonical string key for an unordered edge so we can
// de-duplicate A→B and B→A as the same undirected relation.
func edgeKey(a, b string) string {
	if a < b {
		return a + "\x00" + b
	}
	return b + "\x00" + a
}

// tagOverlap returns how many strings the two slices share.
func tagOverlap(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	seen := newTagSet(a)
	return tagOverlapSet(seen, b)
}

func newTagSet(tags []string) map[string]struct{} {
	seen := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		if t != "" {
			seen[t] = struct{}{}
		}
	}
	return seen
}

func tagOverlapSet(seen map[string]struct{}, b []string) int {
	if len(seen) == 0 || len(b) == 0 {
		return 0
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
	return []string{"Claude Code", "Cursor", "Codex", "OpenClaw"}
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
	if s.onToggleCapture == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "capture toggle not wired (daemon not running in-process)",
		})
		return
	}
	if err := s.onToggleCapture(); err != nil {
		writeError(w, http.StatusInternalServerError, "toggle: "+err.Error())
		return
	}
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
		ID:                     storage.NewID(),
		SignalType:             "git_revert",
		SignalStrength:         "strong",
		AgentName:              "git",
		ProjectPath:            payload.Pwd,
		RawSnippet:             snippet,
		Timestamp:              storage.Now(),
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
		s.adapterStatus(map[string]any{
			"name":        "claude_code",
			"label":       "Claude Code",
			"installed":   adapter.NewClaudeCodeAdapter(backupDir).IsInstalled(),
			"enabled":     s.configMgr.Get().Adapters.ClaudeCode.Enabled,
			"target_path": "CLAUDE.md (project) + ~/.claude/CLAUDE.md (global)",
		}),
		s.adapterStatus(map[string]any{
			"name":        "cursor",
			"label":       "Cursor",
			"installed":   adapter.NewCursorAdapter(backupDir).IsInstalled(),
			"enabled":     s.configMgr.Get().Adapters.Cursor.Enabled,
			"target_path": ".cursorrules (project) + ~/.cursorrules (global)",
		}),
		s.adapterStatus(map[string]any{
			"name":        "codex",
			"label":       "Codex",
			"installed":   adapter.NewCodexAdapter(backupDir).IsInstalled(),
			"enabled":     s.configMgr.Get().Adapters.Codex.Enabled,
			"target_path": "AGENTS.md (project) + ~/AGENTS.md (global)",
		}),
		s.adapterStatus(map[string]any{
			"name":        "openclaw",
			"label":       "OpenClaw",
			"installed":   adapter.NewOpenClawAdapter(backupDir).IsInstalled(),
			"enabled":     s.configMgr.Get().Adapters.OpenClaw.Enabled,
			"target_path": "OPENCLAW.md (project) + ~/OPENCLAW.md (global)",
		}),
		s.adapterStatus(map[string]any{
			"name":        "copilot",
			"label":       "GitHub Copilot",
			"installed":   adapter.NewCopilotAdapter(backupDir).IsInstalled(),
			"enabled":     s.configMgr.Get().Adapters.Copilot.Enabled,
			"target_path": ".github/copilot-instructions.md (project) + ~/.copilot/instructions.md (global)",
		}),
	}

	writeJSON(w, http.StatusOK, adapters)
}

func (s *Server) adapterStatus(base map[string]any) map[string]any {
	name, _ := base["name"].(string)
	base["last_sync_at"] = ""
	base["last_error"] = ""
	base["hit_count"] = 0
	base["managed_block_status"] = "unknown"

	if count, err := s.eventRepo.CountRuleHitsByAgentName(name); err == nil {
		base["hit_count"] = count
	}
	if latest, err := s.eventRepo.LatestSyncByAgent(name); err == nil && latest != nil {
		base["last_sync_at"] = latest.Timestamp
		base["managed_block_status"] = "synced"
		if latest.EventType == "sync_failure" {
			base["last_error"] = latest.Details
			base["managed_block_status"] = "error"
		}
	}
	return base
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

	if !isKnownAdapter(name) {
		writeError(w, http.StatusNotFound, "adapter not found: "+name)
		return
	}
	if err := s.configMgr.UpdateGlobal(func(cfg *config.Config) {
		setAdapterEnabled(cfg, name, req.Enabled)
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "save config: "+err.Error())
		return
	}

	s.triggerAdapterSync()
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) syncAdapters(w http.ResponseWriter, r *http.Request) {
	if s.onSyncAdapters == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "adapter sync not wired (daemon not running in-process)",
		})
		return
	}
	if err := s.onSyncAdapters(); err != nil {
		writeError(w, http.StatusInternalServerError, "sync: "+err.Error())
		return
	}
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

func (s *Server) triggerAdapterSync() {
	if s.onSyncAdapters == nil {
		return
	}
	if err := s.onSyncAdapters(); err != nil {
		slog.Warn("adapter sync hook failed", "error", err)
	}
}

func applyRulePatch(rule *storage.Rule, patch map[string]json.RawMessage) error {
	for key, raw := range patch {
		switch key {
		case "id", "version", "created_at", "updated_at":
			continue
		case "content":
			if err := json.Unmarshal(raw, &rule.Content); err != nil {
				return fmt.Errorf("invalid content")
			}
		case "scope":
			if err := json.Unmarshal(raw, &rule.Scope); err != nil {
				return fmt.Errorf("invalid scope")
			}
		case "project_path":
			if err := json.Unmarshal(raw, &rule.ProjectPath); err != nil {
				return fmt.Errorf("invalid project_path")
			}
		case "tags":
			if err := json.Unmarshal(raw, &rule.Tags); err != nil {
				return fmt.Errorf("invalid tags")
			}
			if rule.Tags == nil {
				rule.Tags = []string{}
			}
		case "category":
			if err := json.Unmarshal(raw, &rule.Category); err != nil {
				return fmt.Errorf("invalid category")
			}
		case "trigger_context":
			if err := json.Unmarshal(raw, &rule.TriggerContext); err != nil {
				return fmt.Errorf("invalid trigger_context")
			}
		case "confidence":
			if err := json.Unmarshal(raw, &rule.Confidence); err != nil {
				return fmt.Errorf("invalid confidence")
			}
		case "status":
			if err := json.Unmarshal(raw, &rule.Status); err != nil {
				return fmt.Errorf("invalid status")
			}
		}
	}
	return nil
}

func applyConfigUpdates(cfg *config.Config, updates map[string]any) bool {
	adapterChanged := false
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
		case "llm_api_key":
			if str, ok := val.(string); ok {
				cfg.Distill.LLMAPIKey = str
			}
		case "llm_model":
			if str, ok := val.(string); ok {
				cfg.Distill.LLMModel = str
			}
		case "deny_patterns":
			if arr, ok := val.([]any); ok {
				cfg.Privacy.DenyPatterns = stringsFromJSONValues(arr)
			}
		case "exclude_patterns":
			if arr, ok := val.([]any); ok {
				cfg.Privacy.ExcludePatterns = stringsFromJSONValues(arr)
			}
		case "claude_code_enabled":
			if b, ok := val.(bool); ok {
				cfg.Adapters.ClaudeCode.Enabled = b
				adapterChanged = true
			}
		case "cursor_enabled":
			if b, ok := val.(bool); ok {
				cfg.Adapters.Cursor.Enabled = b
				adapterChanged = true
			}
		case "codex_enabled":
			if b, ok := val.(bool); ok {
				cfg.Adapters.Codex.Enabled = b
				adapterChanged = true
			}
		case "openclaw_enabled":
			if b, ok := val.(bool); ok {
				cfg.Adapters.OpenClaw.Enabled = b
				adapterChanged = true
			}
		case "copilot_enabled":
			if b, ok := val.(bool); ok {
				cfg.Adapters.Copilot.Enabled = b
				adapterChanged = true
			}
		case "server_port":
			if n, ok := val.(float64); ok {
				cfg.Server.Port = int(n)
			}
		}
	}
	return adapterChanged
}

func stringsFromJSONValues(values []any) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func isKnownAdapter(name string) bool {
	switch name {
	case "claude_code", "cursor", "codex", "openclaw", "copilot":
		return true
	default:
		return false
	}
}

func setAdapterEnabled(cfg *config.Config, name string, enabled bool) {
	switch name {
	case "claude_code":
		cfg.Adapters.ClaudeCode.Enabled = enabled
	case "cursor":
		cfg.Adapters.Cursor.Enabled = enabled
	case "codex":
		cfg.Adapters.Codex.Enabled = enabled
	case "openclaw":
		cfg.Adapters.OpenClaw.Enabled = enabled
	case "copilot":
		cfg.Adapters.Copilot.Enabled = enabled
	}
}
