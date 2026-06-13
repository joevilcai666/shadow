package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joevilcai666/shadow/internal/storage"
)

func TestRunSyncDryRunReportsChangesWithoutWritingFiles(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	projectPath := filepath.Join(dir, "repo")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	dbPath := filepath.Join(home, ".shadow", "shadow.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := storage.NewProjectRepo(db).Create(&storage.Project{
		ID:        storage.NewID(),
		Path:      projectPath,
		Name:      "repo",
		Agents:    []string{"Codex"},
		CreatedAt: storage.Now(),
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := storage.NewRuleRepo(db).Create(&storage.Rule{
		ID:          storage.NewID(),
		Content:     "Use pnpm instead of npm",
		Scope:       "project",
		ProjectPath: projectPath,
		Tags:        []string{"node"},
		Confidence:  0.9,
		Status:      "active",
		Version:     1,
		CreatedAt:   storage.Now(),
		UpdatedAt:   storage.Now(),
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	var out bytes.Buffer
	if err := runSync(syncOptions{
		homeDir: home,
		dbPath:  dbPath,
		dryRun:  true,
		out:     &out,
	}); err != nil {
		t.Fatalf("run dry sync: %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectPath, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write AGENTS.md, stat err = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "DRY-RUN") || !strings.Contains(got, "AGENTS.md") {
		t.Fatalf("output = %q, want dry-run target report", got)
	}
}

func TestStoreMemoryRejectsInvalidScope(t *testing.T) {
	cmd := storeMemoryCmd
	if err := cmd.Flags().Set("scope", "workspace"); err != nil {
		t.Fatalf("set scope: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Flags().Set("scope", "global")
	})

	err := cmd.RunE(cmd, []string{"remember this"})
	if err == nil {
		t.Fatal("RunE returned nil, want invalid scope error")
	}
	if !strings.Contains(err.Error(), "invalid --scope") {
		t.Fatalf("error = %q, want invalid --scope", err.Error())
	}
}

func TestRunSyncWritesOnlyRegisteredProjectAgents(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	projectPath := filepath.Join(dir, "repo")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	dbPath := filepath.Join(home, ".shadow", "shadow.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := storage.NewProjectRepo(db).Create(&storage.Project{
		ID:        storage.NewID(),
		Path:      projectPath,
		Name:      "repo",
		Agents:    []string{"Codex"},
		CreatedAt: storage.Now(),
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := storage.NewRuleRepo(db).Create(&storage.Rule{
		ID:          storage.NewID(),
		Content:     "Use table-driven tests",
		Scope:       "project",
		ProjectPath: projectPath,
		Tags:        []string{"go"},
		Confidence:  0.8,
		Status:      "active",
		Version:     1,
		CreatedAt:   storage.Now(),
		UpdatedAt:   storage.Now(),
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	var out bytes.Buffer
	if err := runSync(syncOptions{
		homeDir: home,
		dbPath:  dbPath,
		out:     &out,
	}); err != nil {
		t.Fatalf("run sync: %v", err)
	}

	agentsPath := filepath.Join(projectPath, "AGENTS.md")
	content, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(content), "Use table-driven tests") {
		t.Fatalf("AGENTS.md = %q, want synced rule", string(content))
	}
	if _, err := os.Stat(filepath.Join(projectPath, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("sync should not write unregistered Claude target, stat err = %v", err)
	}
}

func TestRunSyncWritesUserMemoriesToRegisteredAgentContext(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	projectPath := filepath.Join(dir, "repo")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	dbPath := filepath.Join(home, ".shadow", "shadow.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := storage.NewProjectRepo(db).Create(&storage.Project{
		ID:        storage.NewID(),
		Path:      projectPath,
		Name:      "repo",
		Agents:    []string{"Codex"},
		CreatedAt: storage.Now(),
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := storage.NewUserMemoryRepo(db).Create(&storage.UserMemory{
		ID:        storage.NewID(),
		UserID:    "local",
		Content:   "Always use Conventional Commits",
		Category:  "convention",
		Tags:      []string{"git"},
		CreatedAt: storage.Now(),
		UpdatedAt: storage.Now(),
	}); err != nil {
		t.Fatalf("create memory: %v", err)
	}

	var out bytes.Buffer
	if err := runSync(syncOptions{
		homeDir: home,
		dbPath:  dbPath,
		out:     &out,
	}); err != nil {
		t.Fatalf("run sync: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(projectPath, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "Always use Conventional Commits") {
		t.Fatalf("AGENTS.md = %q, want user memory content", text)
	}
	if !strings.Contains(text, "Category: convention") {
		t.Fatalf("AGENTS.md = %q, want memory category metadata", text)
	}
}
