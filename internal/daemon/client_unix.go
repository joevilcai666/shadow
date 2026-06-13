//go:build !windows

package daemon

import (
	"net"
	"os"
	"path/filepath"
	"time"
)

func defaultSockPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".shadow", "shadow.sock")
}

func dialIPC(addr string) (net.Conn, error) {
	return net.DialTimeout("unix", addr, 3*time.Second)
}
