package daemon

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
)

// CommandHandler processes IPC commands.
type CommandHandler interface {
	HandleCommand(req IPCRequest) IPCResponse
}

// SocketServer listens on an IPC transport for commands.
// The transport is platform-specific: Unix domain socket on !windows,
// named pipe on windows (see socket_unix.go / socket_windows.go).
type SocketServer struct {
	path     string
	handler  CommandHandler
	listener net.Listener
}

// NewSocketServer creates a new IPC socket server.
func NewSocketServer(path string, handler CommandHandler) *SocketServer {
	return &SocketServer{
		path:    path,
		handler: handler,
	}
}

// acceptLoop accepts connections in a loop and handles each in a goroutine.
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
