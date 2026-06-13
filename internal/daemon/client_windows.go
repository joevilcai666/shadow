//go:build windows

package daemon

import (
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// Windows named pipe path for the daemon.
const pipePath = `\\.\pipe\shadow-daemon`

func defaultSockPath() string {
	return pipePath
}

func dialIPC(addr string) (net.Conn, error) {
	timeout := 3 * time.Second
	return winio.DialPipe(addr, &timeout)
}
