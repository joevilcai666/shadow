//go:build windows

package main

import (
	"fmt"

	"github.com/joevilcai666/shadow/internal/daemon"
)

func installDaemon(execPath string) error {
	if err := daemon.InstallService(execPath); err != nil {
		return fmt.Errorf("install service: %w", err)
	}
	fmt.Println("✓ Shadow daemon registered as Windows Service")
	return nil
}

func startDaemonService() error {
	fmt.Println("Starting daemon...")
	if err := daemon.StartService(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	fmt.Println("✓ Shadow daemon started")
	return nil
}

func stopDaemonService() {
	if err := daemon.StopService(); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}
}

func uninstallDaemon() {
	if err := daemon.UninstallService(); err != nil {
		fmt.Printf("Warning: %v\n", err)
	} else {
		fmt.Println("✓ Unregistered Windows Service")
	}
}
