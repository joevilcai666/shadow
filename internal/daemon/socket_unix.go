//go:build !windows

package daemon

import (
	"fmt"
	"net"
	"os"
)

// Start begins listening on a Unix domain socket.
func (s *SocketServer) Start() error {
	// Remove stale socket file.
	os.Remove(s.path)

	l, err := net.Listen("unix", s.path)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.path, err)
	}
	s.listener = l

	// Restrict access to the socket file.
	os.Chmod(s.path, 0600)

	go s.acceptLoop()
	return nil
}

// Close shuts down the Unix socket server and removes the socket file.
func (s *SocketServer) Close() {
	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.path)
}
