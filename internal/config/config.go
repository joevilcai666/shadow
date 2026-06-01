package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config holds all Shadow configuration.
type Config struct {
	Capture  CaptureConfig  `yaml:"capture"`
	Privacy  PrivacyConfig  `yaml:"privacy"`
	Distill  DistillConfig  `yaml:"distill"`
	Adapters AdaptersConfig `yaml:"adapters"`
	Server   ServerConfig   `yaml:"server"`
}

// CaptureConfig controls the capture engine.
type CaptureConfig struct {
	Enabled  bool                       `yaml:"enabled"`
	Projects map[string]ProjectCapture  `yaml:"projects"`
}

// ProjectCapture is per-project capture override.
type ProjectCapture struct {
	Enabled bool `yaml:"enabled"`
}

// PrivacyConfig defines privacy and security constraints.
type PrivacyConfig struct {
	ExcludePatterns []string `yaml:"exclude_patterns"`
	DenyPatterns    []string `yaml:"deny_patterns"`
}

// DistillConfig controls the rule distillation engine.
type DistillConfig struct {
	Threshold            string `yaml:"threshold"`
	AutoActivateLowRisk  bool   `yaml:"auto_activate_low_risk"`
	BatchMode            bool   `yaml:"batch_mode"`
	LLMAPIKey            string `yaml:"llm_api_key"`
	LLMModel             string `yaml:"llm_model"`
}

// AdaptersConfig configures which agent adapters are active.
type AdaptersConfig struct {
	ClaudeCode AdapterConfig `yaml:"claude_code"`
	Cursor     AdapterConfig `yaml:"cursor"`
	Codex      AdapterConfig `yaml:"codex"`
}

// AdapterConfig is per-adapter configuration.
type AdapterConfig struct {
	Enabled    bool   `yaml:"enabled"`
	GlobalPath string `yaml:"global_path,omitempty"`
}

// ServerConfig is the HTTP server configuration.
type ServerConfig struct {
	Port int    `yaml:"port"`
	Bind string `yaml:"bind"`
}

// Manager manages configuration loading, merging, and validation.
type Manager struct {
	mu       sync.RWMutex
	global   *Config
	homeDir  string
	denyRe   []*regexp.Regexp
}

// NewManager creates a new config manager.
func NewManager(homeDir string) *Manager {
	return &Manager{
		homeDir: homeDir,
		global:  DefaultConfig(),
	}
}

// DefaultConfig returns safe default configuration.
func DefaultConfig() *Config {
	return &Config{
		Capture: CaptureConfig{
			Enabled:  true,
			Projects: make(map[string]ProjectCapture),
		},
		Privacy: PrivacyConfig{
			ExcludePatterns: []string{
				".env*",
				".git/**",
				"node_modules/**",
				"*.key",
				"*.pem",
				"**/secrets/**",
			},
			DenyPatterns: []string{
				`sk-[a-zA-Z0-9]{20,}`,
				`ghp_[a-zA-Z0-9]{36}`,
				`AKIA[A-Z0-9]{16}`,
			},
		},
		Distill: DistillConfig{
			Threshold:          "medium",
			AutoActivateLowRisk: true,
			BatchMode:          false,
			LLMModel:           "claude-sonnet-4-20250514",
		},
		Adapters: AdaptersConfig{
			ClaudeCode: AdapterConfig{Enabled: true, GlobalPath: "~/.claude/CLAUDE.md"},
			Cursor:     AdapterConfig{Enabled: true},
			Codex:      AdapterConfig{Enabled: true},
		},
		Server: ServerConfig{
			Port: 7878,
			Bind: "127.0.0.1",
		},
	}
}

// LoadGlobal loads the global config from ~/.shadow/config.yaml.
func (m *Manager) LoadGlobal() error {
	path := filepath.Join(m.homeDir, "config.yaml")
	if err := m.loadFile(path, m.global); err != nil {
		return err
	}
	return m.compileDenyPatterns(m.global)
}

// LoadProject loads and merges a project-level config.
func (m *Manager) LoadProject(projectDir string) (*Config, error) {
	m.mu.RLock()
	base := *m.global
	m.mu.RUnlock()

	project := &base
	path := filepath.Join(projectDir, ".shadow", "config.yaml")
	if _, err := os.Stat(path); err == nil {
		if err := m.loadFile(path, project); err != nil {
			return nil, fmt.Errorf("load project config: %w", err)
		}
	}

	return project, nil
}

// Get returns the current global config (read-only copy).
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := *m.global
	return &cp
}

// Reload reloads the global config from disk (hot update).
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fresh := DefaultConfig()
	path := filepath.Join(m.homeDir, "config.yaml")
	if err := m.loadFile(path, fresh); err != nil {
		return err
	}

	if err := validate(fresh); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if err := m.compileDenyPatterns(fresh); err != nil {
		return fmt.Errorf("compile deny patterns: %w", err)
	}

	m.global = fresh
	return nil
}

// IsCaptureEnabled checks if capture is enabled for a given project path.
func (m *Manager) IsCaptureEnabled(projectPath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.global.Capture.Enabled {
		return false
	}
	if pc, ok := m.global.Capture.Projects[projectPath]; ok {
		return pc.Enabled
	}
	return true
}

// IsPathExcluded checks if a file path matches any exclude pattern.
func (m *Manager) IsPathExcluded(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, pattern := range m.global.Privacy.ExcludePatterns {
		if matchGlob(pattern, path) {
			return true
		}
	}
	return false
}

// matchGlob supports ** globs by converting to simple substring/pattern matching.
func matchGlob(pattern, path string) bool {
	// Handle ** glob patterns.
	if strings.Contains(pattern, "**") {
		// Convert "**/secrets/**" to check if "secrets" appears as a path component.
		parts := strings.Split(pattern, "**")
		for _, part := range parts {
			part = strings.Trim(part, "/")
			if part == "" {
				continue
			}
			if strings.Contains(path, part) {
				return true
			}
		}
		return false
	}

	matched, _ := filepath.Match(pattern, path)
	return matched
}

// ContainsSensitiveData checks if content contains any denied patterns (keys/tokens).
func (m *Manager) ContainsSensitiveData(content string) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, re := range m.denyRe {
		if re.MatchString(content) {
			return true, re.String()
		}
	}
	return false, ""
}

func (m *Manager) loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	return nil
}

func (m *Manager) compileDenyPatterns(cfg *Config) error {
	m.denyRe = make([]*regexp.Regexp, 0, len(cfg.Privacy.DenyPatterns))
	for _, p := range cfg.Privacy.DenyPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("compile deny pattern %q: %w", p, err)
		}
		m.denyRe = append(m.denyRe, re)
	}
	return nil
}

func validate(cfg *Config) error {
	validThresholds := map[string]bool{"low": true, "medium": true, "high": true}
	if !validThresholds[cfg.Distill.Threshold] {
		return fmt.Errorf("invalid distill threshold: %q (must be low/medium/high)", cfg.Distill.Threshold)
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}
	return nil
}
