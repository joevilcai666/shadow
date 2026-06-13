//go:build darwin

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
        <string>serve</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.LogDir}}/daemon.log</string>
    <key>StandardErrorPath</key>
    <string>{{.LogDir}}/daemon-error.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>SHADOW_HOME</key>
        <string>{{.HomeDir}}</string>
    </dict>
</dict>
</plist>
`

// LaunchdConfig holds values for the plist template.
type LaunchdConfig struct {
	Label      string
	BinaryPath string
	LogDir     string
	HomeDir    string
}

// InstallLaunchd generates and installs the launchd plist.
func InstallLaunchd(cfg LaunchdConfig) error {
	agentsDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	plistPath := filepath.Join(agentsDir, cfg.Label+".plist")

	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return fmt.Errorf("parse plist template: %w", err)
	}

	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("create plist: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, cfg); err != nil {
		return fmt.Errorf("execute plist template: %w", err)
	}

	return nil
}

// UninstallLaunchd removes the launchd plist.
func UninstallLaunchd(label string) error {
	plistPath := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", label+".plist")
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	return nil
}

// LaunchdPlistPath returns the expected plist file path.
func LaunchdPlistPath(label string) string {
	return filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", label+".plist")
}

// LoadLaunchd loads and starts the daemon via launchctl.
func LoadLaunchd(label string) error {
	plistPath := LaunchdPlistPath(label)

	// Unload first in case of stale registration.
	exec.Command("launchctl", "unload", plistPath).Run() // Ignore error — may not be loaded.

	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}
	return nil
}

// UnloadLaunchd stops and unloads the daemon via launchctl.
func UnloadLaunchd(label string) error {
	plistPath := LaunchdPlistPath(label)
	if err := exec.Command("launchctl", "unload", plistPath).Run(); err != nil {
		return fmt.Errorf("launchctl unload: %w", err)
	}
	return nil
}
