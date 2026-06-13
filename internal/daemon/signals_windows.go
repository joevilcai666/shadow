//go:build windows

package daemon

import (
	"os"
	"os/signal"
)

// Windows named pipe address used for IPC between CLI and daemon.
const windowsPipeAddr = `\\.\pipe\shadow-daemon`

// ipcAddr returns the IPC address for the daemon.
// On Windows this is a named pipe path (not a filesystem path).
func ipcAddr(homeDir string) string {
	return windowsPipeAddr
}

// writePID is a no-op on Windows: the PID is written inside acquireLock()
// (lock_windows.go) using the same file handle that holds the LockFileEx lock,
// because Windows mandatory locks block even the owning process from opening
// a second handle.
func writePID(d *Daemon) error {
	return nil
}

func registerSignals(sigCh chan os.Signal) {
	signal.Notify(sigCh, os.Interrupt, os.Kill)
}

func isReloadSignal(sig os.Signal) bool {
	return false // Windows has no reload signal equivalent
}
