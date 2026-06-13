//go:build windows

package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/windows"
)

// acquireLock acquires an exclusive lock on the PID file using LockFileEx,
// which is the Windows equivalent of Unix flock. It also writes the current
// PID to the file so writePID() in daemon.go can be a no-op.
//
// Unlike named mutexes, file locks are automatically released when the
// process exits and don't require special privileges.
func (d *Daemon) acquireLock() (func(), error) {
	if err := os.MkdirAll(filepath.Dir(d.pidPath), 0755); err != nil {
		return nil, fmt.Errorf("create pid dir: %w", err)
	}

	// Open or create the PID file.
	f, err := os.OpenFile(d.pidPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open pid file: %w", err)
	}

	// Try to acquire an exclusive lock on the first byte (non-blocking).
	// LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY
	ol := new(windows.Overlapped)
	err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
	if err != nil {
		// Another instance is running — read its PID.
		data, _ := os.ReadFile(d.pidPath)
		pid, _ := strconv.Atoi(string(data))
		f.Close()
		return nil, fmt.Errorf("another instance is running (pid %d)", pid)
	}

	// Write PID using the same file handle (Windows locks block other handles).
	pidStr := fmt.Sprintf("%d", os.Getpid())
	f.Truncate(0)
	f.Seek(0, 0)
	if _, err := f.WriteString(pidStr); err != nil {
		windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
		f.Close()
		return nil, fmt.Errorf("write pid: %w", err)
	}

	return func() {
		windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
		f.Close()
	}, nil
}
