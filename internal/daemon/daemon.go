package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/byronxlg/agentlog/internal/index"
	"github.com/byronxlg/agentlog/internal/store"
)

// Config holds the daemon's file paths and directories.
type Config struct {
	Dir        string
	SocketPath string
	PIDPath    string
	LogPath    string
}

func (c *Config) applyDefaults() {
	if c.Dir == "" {
		home, _ := os.UserHomeDir()
		c.Dir = filepath.Join(home, ".agentlog")
	}
	if c.SocketPath == "" {
		c.SocketPath = filepath.Join(c.Dir, "agentlogd.sock")
	}
	if c.PIDPath == "" {
		c.PIDPath = filepath.Join(c.Dir, "agentlogd.pid")
	}
	if c.LogPath == "" {
		c.LogPath = filepath.Join(c.Dir, "agentlogd.log")
	}
}

// Daemon is the agentlogd background process that owns the log store and index.
type Daemon struct {
	cfg      Config
	store    *store.Store
	index    *index.Index
	listener net.Listener
	logger   *slog.Logger
	writeCh  chan writeRequest
	done     chan struct{}
	sessions map[string]bool
	mu       sync.RWMutex
	wg       sync.WaitGroup
}

// New creates a Daemon with the given configuration, opening the store and index.
func New(cfg Config) (*Daemon, error) {
	cfg.applyDefaults()

	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	if err := checkStalePID(cfg.PIDPath); err != nil {
		return nil, err
	}

	s := store.New(cfg.Dir)

	idx, err := index.Open(filepath.Join(cfg.Dir, "index.db"))
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}

	logFile, err := os.OpenFile(cfg.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		_ = idx.Close()
		return nil, fmt.Errorf("open log file: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo}))

	return &Daemon{
		cfg:      cfg,
		store:    s,
		index:    idx,
		logger:   logger,
		writeCh:  make(chan writeRequest, 64),
		done:     make(chan struct{}),
		sessions: make(map[string]bool),
	}, nil
}

// Run starts the daemon, listening on the Unix socket until the context is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	if err := d.writePIDFile(); err != nil {
		return err
	}

	// Clean up stale socket file before listening
	if err := os.Remove(d.cfg.SocketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	ln, err := net.Listen("unix", d.cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("listen on socket: %w", err)
	}
	d.listener = ln
	d.logger.Info("daemon started", "socket", d.cfg.SocketPath, "pid", os.Getpid())

	d.wg.Add(1)
	go d.writeLoop()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				d.logger.Info("stopped accepting connections")
				return d.shutdown()
			default:
				d.logger.Error("accept connection", "error", err)
				continue
			}
		}
		d.wg.Add(1)
		go d.handleConn(conn)
	}
}

func (d *Daemon) shutdown() error {
	close(d.done)
	d.wg.Wait()

	var firstErr error
	if err := d.index.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("close index: %w", err)
	}
	if err := os.Remove(d.cfg.PIDPath); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = fmt.Errorf("remove pid file: %w", err)
	}
	if err := os.Remove(d.cfg.SocketPath); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = fmt.Errorf("remove socket file: %w", err)
	}

	d.logger.Info("daemon stopped")
	return firstErr
}

func (d *Daemon) handleConn(conn net.Conn) {
	defer d.wg.Done()
	defer func() { _ = conn.Close() }()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}

	var req Request
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		resp := errResponse("invalid request: " + err.Error())
		data, _ := json.Marshal(resp)
		_, _ = conn.Write(append(data, '\n'))
		return
	}

	d.logger.Info("request", "method", req.Method)
	resp := d.handleRequest(req)

	data, err := json.Marshal(resp)
	if err != nil {
		d.logger.Error("marshal response", "error", err)
		return
	}
	_, _ = conn.Write(append(data, '\n'))
}

func (d *Daemon) writeLoop() {
	defer d.wg.Done()
	for {
		select {
		case wr := <-d.writeCh:
			err := d.store.Append(wr.entry)
			if err == nil {
				err = d.index.Insert(wr.entry)
			}
			wr.result <- writeResult{entry: wr.entry, err: err}
		case <-d.done:
			// Drain remaining writes
			for {
				select {
				case wr := <-d.writeCh:
					err := d.store.Append(wr.entry)
					if err == nil {
						err = d.index.Insert(wr.entry)
					}
					wr.result <- writeResult{entry: wr.entry, err: err}
				default:
					return
				}
			}
		}
	}
}

func (d *Daemon) writePIDFile() error {
	data := []byte(strconv.Itoa(os.Getpid()))
	if err := os.WriteFile(d.cfg.PIDPath, data, 0o644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	return nil
}

func checkStalePID(pidPath string) error {
	data, err := os.ReadFile(pidPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read pid file: %w", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		// Corrupt PID file - safe to overwrite
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}

	// Signal 0 checks if process exists without actually sending a signal
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// Process not running - stale PID file
		return nil
	}

	return fmt.Errorf("daemon already running with pid %d", pid)
}
