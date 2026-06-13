//go:build windows

package daemon

import (
	"fmt"

	"github.com/Microsoft/go-winio"
)

// Start begins listening on a Windows named pipe.
// The pipe path (s.path) must be in the form \\.\pipe\<name>.
func (s *SocketServer) Start() error {
	// winio.ListenPipe creates a named pipe with a default security
	// descriptor that grants access to the current user.
	// Passing nil uses the default SDDL.
	l, err := winio.ListenPipe(s.path, nil)
	if err != nil {
		return fmt.Errorf("listen on pipe %s: %w", s.path, err)
	}
	s.listener = l

	go s.acceptLoop()
	return nil
}

// Close shuts down the named pipe. The pipe is automatically cleaned up
// by the OS when the last handle is closed.
func (s *SocketServer) Close() {
	if s.listener != nil {
		s.listener.Close()
	}
}
