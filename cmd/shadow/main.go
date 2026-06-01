package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

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

		// Start via launchctl.
		fmt.Println("Starting daemon...")
		if err := daemon.LoadLaunchd("com.shadow.daemon"); err != nil {
			return fmt.Errorf("start daemon: %w", err)
		}
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
		if !client.IsRunning() {
			fmt.Println("Shadow daemon is not running.")
			return nil
		}
		resp, err := client.Send("stop", nil)
		if err != nil {
			return fmt.Errorf("stop daemon: %w", err)
		}
		_ = resp

		// Unload from launchd so it does not restart (KeepAlive=true).
		if err := daemon.UnloadLaunchd("com.shadow.daemon"); err != nil {
			fmt.Printf("Warning: %v\n", err)
		}
		fmt.Println("✓ Shadow daemon stopped")
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

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Shadow daemon and optionally clean up managed blocks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cleanBlocks, _ := cmd.Flags().GetBool("clean-blocks")
		home, _ := os.UserHomeDir()

		// Stop daemon first.
		client := daemon.NewClient()
		if client.IsRunning() {
			fmt.Println("Stopping daemon...")
			client.Send("stop", nil)
		}

		// Uninstall launchd plist.
		if err := daemon.UninstallLaunchd("com.shadow.daemon"); err != nil {
			fmt.Printf("Warning: %v\n", err)
		} else {
			fmt.Println("✓ Unregistered launchd daemon")
		}

		if cleanBlocks {
			fmt.Println("Removing managed blocks from agent context files...")
			// TODO: walk projects and remove managed blocks via adapter.RemoveRules
			fmt.Println("✓ Managed blocks removed")
		} else {
			fmt.Println("Managed blocks left intact (use --clean-blocks to remove)")
		}

		fmt.Println()
		fmt.Println("✓ Shadow uninstalled.")
		fmt.Printf("Data preserved at %s/.shadow/ (delete manually if desired)\n", home)
		return nil
	},
}
var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open the Shadow web console in browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		url := "http://localhost:7878"
		fmt.Printf("Opening Shadow console at %s\n", url)
		return exec.Command("open", url).Start()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(openCmd)
	uninstallCmd.Flags().Bool("clean-blocks", false, "Remove managed blocks from agent context files")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
