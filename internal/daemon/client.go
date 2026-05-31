package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Client is an IPC client for communicating with the Shadow daemon.
type Client struct {
	sockPath string
}

// NewClient creates a new IPC client.
func NewClient() *Client {
	home, _ := os.UserHomeDir()
	return &Client{
		sockPath: filepath.Join(home, ".shadow", "shadow.sock"),
	}
}

// Send sends a command to the daemon and returns the response.
func (c *Client) Send(method string, params any) (*IPCResponse, error) {
	paramsJSON, _ := json.Marshal(params)
	if params == nil {
		paramsJSON = nil
	}

	req := IPCRequest{
		Method: method,
		Params: paramsJSON,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	conn, err := net.DialTimeout("unix", c.sockPath, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w (is shadow running?)", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))
	conn.Write(append(reqBytes, '\n'))

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var resp IPCResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &resp, nil
}

// IsRunning checks if the daemon is currently running.
func (c *Client) IsRunning() bool {
	_, err := c.Send("status", nil)
	return err == nil
}
