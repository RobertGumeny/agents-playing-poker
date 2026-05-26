package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RobertGumeny/agent-poker/internal/reporting"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "poker-report: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("poker-report", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var sessionsFlag string
	var label string
	var outPath string
	fs.StringVar(&sessionsFlag, "sessions", "", "comma-separated session directories to include")
	fs.StringVar(&label, "label", "", "benchmark label for the Markdown title")
	fs.StringVar(&outPath, "out", "", "output Markdown path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	sessionDirs, err := parseSessionDirs(sessionsFlag)
	if err != nil {
		return err
	}
	if outPath == "" {
		return fmt.Errorf("-out is required")
	}

	sessions, err := reporting.LoadSessions(sessionDirs)
	if err != nil {
		return err
	}
	markdown := reporting.RenderMarkdown(label, reporting.ComputeAggregate(sessions))
	if dir := filepath.Dir(outPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}
	if err := os.WriteFile(outPath, []byte(markdown), 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

func parseSessionDirs(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("-sessions is required")
	}
	parts := strings.Split(value, ",")
	dirs := make([]string, 0, len(parts))
	for _, part := range parts {
		dir := strings.TrimSpace(part)
		if dir == "" {
			return nil, fmt.Errorf("-sessions contains an empty session directory")
		}
		dirs = append(dirs, dir)
	}
	return dirs, nil
}
