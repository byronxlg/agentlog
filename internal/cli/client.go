package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"

	"github.com/byronxlg/agentlog/internal/daemon"
)

// SendRequest connects to the daemon Unix socket, sends a JSON request,
// and returns the parsed response.
func SendRequest(socketPath string, req daemon.Request) (daemon.Response, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return daemon.Response{}, fmt.Errorf("connect to daemon: %w", err)
	}
	defer func() { _ = conn.Close() }()

	data, err := json.Marshal(req)
	if err != nil {
		return daemon.Response{}, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	if _, err := conn.Write(data); err != nil {
		return daemon.Response{}, fmt.Errorf("write request: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return daemon.Response{}, fmt.Errorf("read response: %w", err)
		}
		return daemon.Response{}, fmt.Errorf("read response: connection closed")
	}

	var resp daemon.Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return daemon.Response{}, fmt.Errorf("unmarshal response: %w", err)
	}

	return resp, nil
}
