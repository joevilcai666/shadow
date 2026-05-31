package adapter

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/joevilcai666/shadow/internal/storage"
)

// MCPServer provides an MCP-compatible interface for generic agent access.
type MCPServer struct {
	ruleDB *storage.RuleRepo
	router *mux.Router
}

// NewMCPServer creates a new MCP server.
func NewMCPServer(ruleDB *storage.RuleRepo) *MCPServer {
	s := &MCPServer{ruleDB: ruleDB, router: mux.NewRouter()}
	s.routes()
	return s
}

// Handler returns the HTTP handler for the MCP server.
func (s *MCPServer) Handler() http.Handler { return s.router }

func (s *MCPServer) routes() {
	// MCP Resources: list available rules.
	s.router.HandleFunc("/mcp/resources", s.listResources).Methods("GET")
	s.router.HandleFunc("/mcp/resources/{id}", s.getResource).Methods("GET")

	// MCP Tools: callable actions.
	s.router.HandleFunc("/mcp/tools", s.listTools).Methods("GET")
	s.router.HandleFunc("/mcp/tools/search_rules", s.searchRules).Methods("POST")
	s.router.HandleFunc("/mcp/tools/get_active_rules", s.getActiveRules).Methods("POST")

	// MCP config snippet for agent setup.
	s.router.HandleFunc("/mcp/config", s.getConfig).Methods("GET")
}

func (s *MCPServer) listResources(w http.ResponseWriter, r *http.Request) {
	rules, _ := s.ruleDB.List(storage.RuleFilter{Status: "active"})
	resources := make([]map[string]string, 0, len(rules))
	for _, rule := range rules {
		resources = append(resources, map[string]string{
			"uri":  fmt.Sprintf("shadow://rule/%s", rule.ID),
			"name": truncate(rule.Content, 50),
			"type": "text",
		})
	}
	writeMCPJSON(w, map[string]any{"resources": resources})
}

func (s *MCPServer) getResource(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	rule, err := s.ruleDB.GetByID(id)
	if err != nil || rule == nil {
		writeMCPError(w, "rule not found")
		return
	}
	writeMCPJSON(w, map[string]any{
		"uri":     fmt.Sprintf("shadow://rule/%s", rule.ID),
		"content": rule.Content,
		"scope":   rule.Scope,
		"tags":    rule.Tags,
	})
}

func (s *MCPServer) listTools(w http.ResponseWriter, r *http.Request) {
	tools := []map[string]any{
		{
			"name":        "search_rules",
			"description": "Search Shadow rules by keyword",
			"parameters":  map[string]any{"query": "string"},
		},
		{
			"name":        "get_active_rules",
			"description": "Get all active rules, optionally filtered by scope",
			"parameters":  map[string]any{"scope": "string (global|project)"},
		},
	}
	writeMCPJSON(w, map[string]any{"tools": tools})
}

func (s *MCPServer) searchRules(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	rules, _ := s.ruleDB.List(storage.RuleFilter{Status: "active", Search: req.Query})
	results := make([]map[string]string, 0, len(rules))
	for _, rule := range rules {
		results = append(results, map[string]string{
			"id":      rule.ID,
			"content": rule.Content,
			"scope":   rule.Scope,
		})
	}
	writeMCPJSON(w, map[string]any{"results": results})
}

func (s *MCPServer) getActiveRules(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope string `json:"scope"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	rules, _ := s.ruleDB.List(storage.RuleFilter{Status: "active", Scope: req.Scope})
	writeMCPJSON(w, map[string]any{"rules": rules})
}

func (s *MCPServer) getConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]any{
		"mcpServers": map[string]any{
			"shadow": map[string]string{
				"command": "shadow",
				"args":    "mcp",
			},
		},
	}
	writeMCPJSON(w, config)
}

func writeMCPJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeMCPError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
