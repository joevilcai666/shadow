package daemon

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

// acquireLock tries to get an exclusive lock on the PID file.
// Returns a release function on success.
func (d *Daemon) acquireLock() (func(), error) {
	f, err := os.OpenFile(d.pidPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open pid file: %w", err)
	}

	// Try non-blocking exclusive lock.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		// Another instance is running — read its PID.
		data, _ := os.ReadFile(d.pidPath)
		pid, _ := strconv.Atoi(string(data))
		f.Close()
		return nil, fmt.Errorf("another instance is running (pid %d)", pid)
	}

	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
