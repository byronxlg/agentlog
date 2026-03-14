package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/byronxlg/agentlog/internal/daemon"
)

func run() error {
	var dir string
	flag.StringVar(&dir, "dir", "", "data directory (default ~/.agentlog)")
	flag.Parse()

	cfg := daemon.Config{Dir: dir}

	d, err := daemon.New(cfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	return d.Run(ctx)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "agentlogd: %v\n", err)
		os.Exit(1)
	}
}
