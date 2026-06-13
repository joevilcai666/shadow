//go:build !windows

package main

import (
	"fmt"
	"os"

	"github.com/joevilcai666/shadow/internal/daemon"
)

func installDaemon(execPath string) error {
	home, _ := os.UserHomeDir()
	cfg := daemon.LaunchdConfig{
		Label:      "com.shadow.daemon",
		BinaryPath: execPath,
		LogDir:     home + "/.shadow/logs",
		HomeDir:    home + "/.shadow",
	}
	if err := daemon.InstallLaunchd(cfg); err != nil {
		return fmt.Errorf("install launchd: %w", err)
	}
	fmt.Println("✓ Shadow daemon registered with launchd")
	return nil
}

func startDaemonService() error {
	fmt.Println("Starting daemon...")
	if err := daemon.LoadLaunchd("com.shadow.daemon"); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	fmt.Println("✓ Shadow daemon started")
	return nil
}

func stopDaemonService() {
	if err := daemon.UnloadLaunchd("com.shadow.daemon"); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}
}

func uninstallDaemon() {
	if err := daemon.UninstallLaunchd("com.shadow.daemon"); err != nil {
		fmt.Printf("Warning: %v\n", err)
	} else {
		fmt.Println("✓ Unregistered launchd daemon")
	}
}
