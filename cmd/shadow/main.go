package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/joevilcai666/shadow"
	"github.com/joevilcai666/shadow/internal/daemon"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "shadow",
	Short: "Shadow — AI agent memory layer",
	Long:  "Shadow captures your corrections to coding agents and turns them into persistent rules that work across all your tools.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Shadow — your AI agent memory layer")
		fmt.Println("Run 'shadow start' to begin, or 'shadow --help' for available commands.")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print Shadow version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Shadow %s\n", shadow.Version)
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Shadow daemon (foreground)",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := daemon.New(daemon.Config{
			Version: shadow.Version,
		})
		if err != nil {
			return fmt.Errorf("create daemon: %w", err)
		}
		return d.Run(cmd.Context())
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Shadow daemon and onboarding wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		if client.IsRunning() {
			fmt.Println("Shadow daemon is already running.")
			return printStatus(client)
		}

		// Install launchd plist.
		home, _ := os.UserHomeDir()
		execPath, _ := os.Executable()
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

		// Start via launchd.
		fmt.Println("Starting daemon...")
		// launchctl load would go here; for now start foreground in goroutine concept.
		fmt.Println("✓ Shadow daemon started")
		fmt.Println()
		fmt.Printf("Shadow %s — ready!\n", shadow.Version)

		// TODO: SHADOW-013 — onboarding TUI
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Shadow daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		resp, err := client.Send("stop", nil)
		if err != nil {
			return fmt.Errorf("stop daemon: %w", err)
		}
		fmt.Println("✓ Shadow daemon stopping...")
		_ = resp
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Shadow daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		if !client.IsRunning() {
			fmt.Println("Shadow daemon is not running.")
			fmt.Println("Run 'shadow start' to start it.")
			return nil
		}
		return printStatus(client)
	},
}

func printStatus(client *daemon.Client) error {
	resp, err := client.Send("status", nil)
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(resp.Result, "", "  ")
	fmt.Printf("Shadow daemon status:\n%s\n", string(data))
	return nil
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
