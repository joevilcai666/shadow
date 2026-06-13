//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

// ipcAddr returns the IPC socket path for the daemon.
// On Unix this is a filesystem path to a Unix domain socket.
func ipcAddr(homeDir string) string {
	return filepath.Join(homeDir, "shadow.sock")
}

func writePID(d *Daemon) error {
	return os.WriteFile(d.pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
}

func registerSignals(sigCh chan os.Signal) {
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
}

func isReloadSignal(sig os.Signal) bool {
	return sig == syscall.SIGHUP
}
