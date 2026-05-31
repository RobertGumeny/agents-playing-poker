package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

const defaultThinkingLevel = "low"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printHelp(stdout)
		return nil
	}
	switch args[0] {
	case "demo":
		return runDemo(args[1:], stdout, stderr)
	case "experiment":
		return runExperiment(args[1:], stdout, stderr)
	case "match":
		return runMatch(args[1:], stdout, stderr)
	case "strategy":
		return runStrategy(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown subcommand %q — run 'poker help' for usage", args[0])
	}
}

func printHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `poker — agent poker research harness

USAGE
  poker <command> [arguments]

COMMANDS
  demo                                    run a no-LLM smoke match (no API key required)

  experiment ls                           list all experiments and session coverage
  experiment status <id>                  show session status for one experiment
  experiment new <id>                     scaffold a new experiment definition for editing
  experiment run <id> [--model X]         execute missing/incomplete sessions
  experiment analyze <id>                 collect eval.json summaries and write report
  experiment go <id> [--model X]          run + analyze in one shot

  strategy ls                             list known memory strategies and build status
  strategy new <key>                      scaffold a new TypeScript memory strategy

  match run --agent0 X --agent1 Y [opts]  run a single ad-hoc match

QUICKSTART
  make install                            build and install the poker binary (one-time)
  poker demo                              smoke match — no API key required
  poker experiment go <id>                run a full experiment (requires API key + Node)
  poker experiment new my-test            scaffold a new experiment to edit and run

`)
}

func repoRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..")), nil
}
