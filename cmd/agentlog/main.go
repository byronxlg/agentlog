// Package main implements the agentlog CLI.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/byronxlg/agentlog/internal/cli"
)

func defaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".agentlog"
	}
	return filepath.Join(home, ".agentlog")
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: agentlog [--dir <path>] <command>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  start         Start the agentlogd daemon")
	fmt.Fprintln(os.Stderr, "  stop          Stop the agentlogd daemon")
	fmt.Fprintln(os.Stderr, "  write         Write a decision entry to the log")
	fmt.Fprintln(os.Stderr, "  log           List decision entries with optional filters")
	fmt.Fprintln(os.Stderr, "  query         Full-text search across decision entries")
	fmt.Fprintln(os.Stderr, "  show <id>     Show all entries in a session")
	fmt.Fprintln(os.Stderr, "  blame <file>  Show decisions referencing a file")
	fmt.Fprintln(os.Stderr, "  context       Get relevant decisions for LLM context")
	fmt.Fprintln(os.Stderr, "  export        Export formatted decision log output")
}

func main() {
	args := os.Args[1:]
	dir := defaultDir()

	// Parse --dir flag
	if len(args) >= 2 && args[0] == "--dir" {
		dir = args[1]
		args = args[2:]
	}

	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	var err error
	switch args[0] {
	case "start":
		err = cli.Start(dir)
	case "stop":
		err = cli.Stop(dir)
	case "write":
		opts, parseErr := cli.ParseWriteArgs(dir, args[1:])
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "agentlog write: %s\n", parseErr)
			os.Exit(1)
		}
		err = cli.Write(opts)
	case "log":
		logOpts, parseErr := cli.ParseLogArgs(dir, args[1:])
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "agentlog log: %s\n", parseErr)
			os.Exit(1)
		}
		err = cli.Log(logOpts)
	case "query":
		cfg, parseErr := cli.ParseQueryArgs(args[1:])
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", parseErr)
			os.Exit(1)
		}
		os.Exit(cli.RunQuery(cfg))
	case "show":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: agentlog show <session_id>")
			os.Exit(1)
		}
		err = cli.Show(cli.ShowOptions{Dir: dir, SessionID: args[1]})
	case "context":
		ctxOpts, parseErr := cli.ParseContextArgs(dir, args[1:])
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "agentlog context: %s\n", parseErr)
			os.Exit(1)
		}
		err = cli.Context(ctxOpts)
	case "export":
		exportOpts, parseErr := cli.ParseExportArgs(dir, args[1:])
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "agentlog export: %s\n", parseErr)
			os.Exit(1)
		}
		err = cli.Export(exportOpts)
	case "blame":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: agentlog blame [--verbose] <file>")
			os.Exit(1)
		}
		opts := cli.BlameOptions{Dir: dir}
		for _, a := range args[1:] {
			if a == "--verbose" {
				opts.Verbose = true
			} else {
				opts.File = a
			}
		}
		if opts.File == "" {
			fmt.Fprintln(os.Stderr, "Usage: agentlog blame [--verbose] <file>")
			os.Exit(1)
		}
		err = cli.Blame(opts)
	default:
		fmt.Fprintf(os.Stderr, "agentlog: unknown command %q\n", args[0])
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "agentlog %s: %s\n", args[0], err)
		os.Exit(1)
	}
}
