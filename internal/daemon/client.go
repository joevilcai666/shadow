package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DefaultHTTPAddress is the default address the daemon binds its HTTP
// console to. It is exported so the CLI (shadow open) can use the same
// value without re-deriving the port from config.
const DefaultHTTPAddress = "http://127.0.0.1:7878"

// Client is an IPC client for communicating with the Shadow daemon.
type Client struct {
	sockPath string
	httpURL  string
}

// NewClient creates a new IPC client.
func NewClient() *Client {
	return &Client{
		sockPath: defaultSockPath(),
		httpURL:  DefaultHTTPAddress,
	}
}

// NewClientWithHTTP returns a client that points at a non-default HTTP
// address. Useful for tests; the CLI uses the default.
func NewClientWithHTTP(httpURL string) *Client {
	return &Client{
		sockPath: defaultSockPath(),
		httpURL:  httpURL,
	}
}

// HTTPURL returns the URL the client will probe when waiting for the
// daemon's HTTP server to come up.
func (c *Client) HTTPURL() string { return c.httpURL }

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

	conn, err := dialIPC(c.sockPath)
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

// WaitForHTTP polls the daemon's HTTP endpoint until it answers 2xx
// or the timeout elapses. The daemon's IPC socket is bound at boot
// but the HTTP listener follows ~100ms later (see daemon.Run), so a
// caller that just observed IsRunning() == true still needs to wait
// before opening the browser.
func (c *Client) WaitForHTTP(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	delay := 50 * time.Millisecond
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for HTTP at %s", c.httpURL)
		}
		resp, err := http.Get(c.httpURL + "/api/dashboard")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		time.Sleep(delay)
		if delay < 500*time.Millisecond {
			delay *= 2
		}
	}
}
