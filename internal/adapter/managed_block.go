package adapter

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	blockBegin = "# >>> shadow managed >>>"
	blockEnd   = "# <<< shadow managed <<<"
)

// ManagedBlock handles safe reading and writing of managed blocks in agent context files.
type ManagedBlock struct {
	commentPrefix string // "#" for most files, "//" for some
	backupDir     string
}

// NewManagedBlock creates a new managed block writer.
func NewManagedBlock(backupDir string) *ManagedBlock {
	return &ManagedBlock{
		commentPrefix: "#",
		backupDir:     backupDir,
	}
}

// BlockContent represents the content to write in a managed block.
type BlockContent struct {
	Rules []RuleEntry
}

// RuleEntry is a single rule to write in the managed block.
type RuleEntry struct {
	Content     string
	Confidence  float64
}

// WriteResult contains the result of a write operation.
type WriteResult struct {
	FilePath     string
	BackupPath   string
	Handwritten  string // The user's original content (unchanged)
	ManagedBlock string // The new managed block
	Verified     bool
}

// Write writes rules to a file using the managed block mechanism.
// It guarantees user hand-written content is never modified.
func (mb *ManagedBlock) Write(filePath string, rules []RuleEntry) (*WriteResult, error) {
	// Sort rules by confidence descending.
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Confidence > rules[j].Confidence
	})

	// Build the new managed block content.
	newBlock := mb.formatBlock(rules)

	// Read existing file (or empty if not exists).
	existing, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read file: %w", err)
	}

	handwritten, oldBlock, err := mb.splitContent(string(existing))
	if err != nil {
		return nil, err
	}

	_ = oldBlock // We don't need the old block content.

	// Build new file content.
	var newContent string
	if strings.TrimSpace(handwritten) == "" && !strings.Contains(string(existing), blockBegin) {
		// No existing content — just the block.
		newContent = newBlock + "\n"
	} else {
		// Existing content — append/replace block.
		newContent = strings.TrimSpace(handwritten) + "\n\n" + newBlock + "\n"
	}

	// Backup before writing.
	backupPath := ""
	if len(existing) > 0 {
		backupPath, err = mb.backup(filePath, existing)
		if err != nil {
			return nil, fmt.Errorf("backup: %w", err)
		}
	}

	// Verify handwritten content MD5 before write.
	// Normalize: trim trailing whitespace for consistent comparison.
	handwrittenNormalized := strings.TrimSpace(handwritten)
	handwrittenMD5 := md5.Sum([]byte(handwrittenNormalized))

	// Atomic write.
	if err := mb.atomicWrite(filePath, []byte(newContent)); err != nil {
		// Restore from backup.
		if backupPath != "" {
			os.Rename(backupPath, filePath)
		}
		return nil, fmt.Errorf("atomic write: %w", err)
	}

	// Verify: re-read and check handwritten content unchanged.
	result := &WriteResult{
		FilePath:     filePath,
		BackupPath:   backupPath,
		Handwritten:  handwritten,
		ManagedBlock: newBlock,
	}

	written, err := os.ReadFile(filePath)
	if err != nil {
		return result, nil // Can't verify, but write succeeded.
	}

	writtenHandwritten, _, _ := mb.splitContent(string(written))
	writtenNormalized := strings.TrimSpace(writtenHandwritten)
	writtenMD5 := md5.Sum([]byte(writtenNormalized))
	result.Verified = handwrittenMD5 == writtenMD5

	if !result.Verified && backupPath != "" {
		// Restore backup.
		os.Rename(backupPath, filePath)
		return result, fmt.Errorf("verification failed: handwritten content changed, restored backup")
	}

	// Clean up old backups.
	mb.cleanBackups(filePath)

	return result, nil
}

// Remove removes the managed block from a file, preserving handwritten content.
func (mb *ManagedBlock) Remove(filePath string) error {
	existing, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read file: %w", err)
	}

	handwritten, _, err := mb.splitContent(string(existing))
	if err != nil {
		return err
	}

	if strings.TrimSpace(handwritten) == "" {
		// File only had the managed block — remove entirely.
		return os.Remove(filePath)
	}

	// Backup before removing block.
	mb.backup(filePath, existing)

	return mb.atomicWrite(filePath, []byte(handwritten))
}

// splitContent splits a file into handwritten content and managed block.
func (mb *ManagedBlock) splitContent(content string) (handwritten, block string, err error) {
	beginIdx := strings.Index(content, blockBegin)
	if beginIdx == -1 {
		// No managed block — everything is handwritten.
		return content, "", nil
	}

	endIdx := strings.Index(content, blockEnd)
	if endIdx == -1 {
		return "", "", fmt.Errorf("found block begin marker but no end marker")
	}

	if endIdx < beginIdx {
		return "", "", fmt.Errorf("block end marker appears before begin marker")
	}

	handwritten = content[:beginIdx]
	// Trim trailing whitespace from handwritten content.
	handwritten = strings.TrimRight(handwritten, " \t\n\r")

	block = content[beginIdx : endIdx+len(blockEnd)]

	return handwritten, block, nil
}

// formatBlock creates the managed block string from rules.
func (mb *ManagedBlock) formatBlock(rules []RuleEntry) string {
	var lines []string
	lines = append(lines, blockBegin)
	lines = append(lines, fmt.Sprintf("%s [Shadow auto-managed rules — do not edit between markers]", mb.commentPrefix))

	for _, r := range rules {
		lines = append(lines, fmt.Sprintf("%s %s", mb.commentPrefix, r.Content))
	}

	lines = append(lines, blockEnd)
	return strings.Join(lines, "\n")
}

// backup creates a timestamped backup of the file.
func (mb *ManagedBlock) backup(filePath string, content []byte) (string, error) {
	if err := os.MkdirAll(mb.backupDir, 0755); err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102-150405")
	base := filepath.Base(filePath)
	backupPath := filepath.Join(mb.backupDir, fmt.Sprintf("%s_%s", timestamp, base))

	return backupPath, os.WriteFile(backupPath, content, 0644)
}

// atomicWrite writes content to a temp file then atomically renames.
func (mb *ManagedBlock) atomicWrite(targetPath string, content []byte) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmpFile := filepath.Join(dir, ".shadow-tmp-"+filepath.Base(targetPath))

	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		return err
	}

	// fsync to ensure data is on disk.
	f, err := os.Open(tmpFile)
	if err != nil {
		return err
	}
	f.Sync()
	f.Close()

	return os.Rename(tmpFile, targetPath)
}

// cleanBackups keeps only the 10 most recent backups for a file.
func (mb *ManagedBlock) cleanBackups(filePath string) {
	base := filepath.Base(filePath)
	entries, err := os.ReadDir(mb.backupDir)
	if err != nil {
		return
	}

	var backups []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), base) {
			backups = append(backups, filepath.Join(mb.backupDir, e.Name()))
		}
	}

	// Remove oldest if more than 10.
	if len(backups) > 10 {
		sort.Strings(backups)
		for i := 0; i < len(backups)-10; i++ {
			os.Remove(backups[i])
		}
	}
}
