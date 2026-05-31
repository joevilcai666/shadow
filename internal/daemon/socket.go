package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
)

// CommandHandler processes IPC commands.
type CommandHandler interface {
	HandleCommand(req IPCRequest) IPCResponse
}

// SocketServer listens on a Unix socket for IPC commands.
type SocketServer struct {
	path    string
	handler CommandHandler
	listener net.Listener
}

// NewSocketServer creates a new IPC socket server.
func NewSocketServer(path string, handler CommandHandler) *SocketServer {
	return &SocketServer{
		path:    path,
		handler: handler,
	}
}

// Start begins listening for IPC connections.
func (s *SocketServer) Start() error {
	// Remove stale socket file.
	os.Remove(s.path)

	l, err := net.Listen("unix", s.path)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.path, err)
	}
	s.listener = l

	// Set socket file permissions.
	os.Chmod(s.path, 0600)

	go s.acceptLoop()
	return nil
}

// Close shuts down the socket server.
func (s *SocketServer) Close() {
	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.path)
}

func (s *SocketServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Listener closed — normal shutdown.
			return
		}
		go s.handleConn(conn)
	}
}

func (s *SocketServer) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 4096), 65536)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req IPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(conn, "invalid request: "+err.Error())
			continue
		}

		slog.Debug("ipc command", "method", req.Method)
		resp := s.handler.HandleCommand(req)

		respBytes, _ := json.Marshal(resp)
		respBytes = append(respBytes, '\n')
		conn.Write(respBytes)
	}
}

func (s *SocketServer) writeError(conn net.Conn, msg string) {
	resp := IPCResponse{Error: msg}
	respBytes, _ := json.Marshal(resp)
	respBytes = append(respBytes, '\n')
	conn.Write(respBytes)
}
