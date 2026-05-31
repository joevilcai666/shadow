package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Capture.Enabled {
		t.Error("capture should be enabled by default")
	}
	if cfg.Server.Port != 7878 {
		t.Errorf("default port: got %d, want 7878", cfg.Server.Port)
	}
	if cfg.Distill.Threshold != "medium" {
		t.Errorf("default threshold: got %q", cfg.Distill.Threshold)
	}
	if !cfg.Adapters.ClaudeCode.Enabled {
		t.Error("claude_code adapter should be enabled by default")
	}
}

func TestLoadGlobal(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `
capture:
  enabled: false
distill:
  threshold: high
server:
  port: 9090
`
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(cfgContent), 0644)

	m := NewManager(dir)
	if err := m.LoadGlobal(); err != nil {
		t.Fatalf("load global: %v", err)
	}

	cfg := m.Get()
	if cfg.Capture.Enabled {
		t.Error("capture should be disabled")
	}
	if cfg.Distill.Threshold != "high" {
		t.Errorf("threshold: got %q, want %q", cfg.Distill.Threshold, "high")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("port: got %d, want 9090", cfg.Server.Port)
	}
}

func TestProjectOverride(t *testing.T) {
	dir := t.TempDir()
	projectDir := t.TempDir()

	// Global config.
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(`
capture:
  enabled: true
distill:
  threshold: low
`), 0644)

	// Project config overrides.
	os.MkdirAll(filepath.Join(projectDir, ".shadow"), 0755)
	os.WriteFile(filepath.Join(projectDir, ".shadow", "config.yaml"), []byte(`
capture:
  enabled: false
`), 0644)

	m := NewManager(dir)
	m.LoadGlobal()

	projectCfg, err := m.LoadProject(projectDir)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if projectCfg.Capture.Enabled {
		t.Error("project should override capture to disabled")
	}
	if projectCfg.Distill.Threshold != "low" {
		t.Errorf("project should inherit global threshold: got %q", projectCfg.Distill.Threshold)
	}
}

func TestMissingConfigUsesDefaults(t *testing.T) {
	dir := t.TempDir() // empty dir, no config.yaml
	m := NewManager(dir)
	if err := m.LoadGlobal(); err != nil {
		t.Fatalf("load missing config: %v", err)
	}
	cfg := m.Get()
	if !cfg.Capture.Enabled {
		t.Error("should use defaults when no config file exists")
	}
}

func TestHotReload(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Start with defaults.
	m.LoadGlobal()
	if m.Get().Server.Port != 7878 {
		t.Error("should start with default port")
	}

	// Write new config.
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(`
server:
  port: 9999
`), 0644)

	if err := m.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if m.Get().Server.Port != 9999 {
		t.Errorf("after reload: got %d, want 9999", m.Get().Server.Port)
	}
}

func TestSensitiveDataDetection(t *testing.T) {
	m := NewManager(t.TempDir())
	m.LoadGlobal()
	m.compileDenyPatterns(m.Get())

	tests := []struct {
		name    string
		content string
		found   bool
	}{
		{"clean", "Use pnpm not npm", false},
		{"openai_key", "key is sk-abc123def456ghi789jkl012", true},
		{"github_token", "token: ghp_1234567890abcdefghijklmnopqrstuvwxyz1234", true},
		{"aws_key", "AWS_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, pattern := m.ContainsSensitiveData(tt.content)
			if found != tt.found {
				t.Errorf("ContainsSensitiveData(%q) = %v (pattern: %s), want %v", tt.content, found, pattern, tt.found)
			}
		})
	}
}

func TestPathExclusion(t *testing.T) {
	m := NewManager(t.TempDir())
	m.LoadGlobal()

	excluded := []string{".env.production", "secrets/key.pem", ".git/config"}
	for _, p := range excluded {
		if !m.IsPathExcluded(p) {
			t.Errorf("%q should be excluded", p)
		}
	}

	allowed := []string{"src/main.go", "package.json", "README.md"}
	for _, p := range allowed {
		if m.IsPathExcluded(p) {
			t.Errorf("%q should not be excluded", p)
		}
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{"valid", DefaultConfig(), false},
		{"bad_threshold", &Config{Distill: DistillConfig{Threshold: "invalid"}, Server: ServerConfig{Port: 8080}}, true},
		{"bad_port_zero", &Config{Distill: DistillConfig{Threshold: "medium"}, Server: ServerConfig{Port: 0}}, true},
		{"bad_port_high", &Config{Distill: DistillConfig{Threshold: "medium"}, Server: ServerConfig{Port: 99999}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
