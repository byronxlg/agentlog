// Package client provides a Unix socket client for communicating with the agentlogd daemon.
package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/byronxlg/agentlog/internal/daemon"
)

// Client communicates with the agentlogd daemon over a Unix socket.
type Client struct {
	SocketPath string
}

// NewClient returns a Client configured with the given socket path.
// If socketPath is empty, it defaults to ~/.agentlog/agentlogd.sock.
func NewClient(socketPath string) *Client {
	if socketPath == "" {
		home, _ := os.UserHomeDir()
		socketPath = filepath.Join(home, ".agentlog", "agentlogd.sock")
	}
	return &Client{SocketPath: socketPath}
}

// Send connects to the daemon, sends a single JSON request, reads the response,
// and closes the connection. Each call opens a new connection, matching the
// daemon's one-request-per-connection model.
func (c *Client) Send(method string, params any) (*daemon.Response, error) {
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}
	defer func() { _ = conn.Close() }()

	req := daemon.Request{Method: method}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		req.Params = raw
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		return nil, fmt.Errorf("read response: connection closed without response")
	}

	var resp daemon.Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}
