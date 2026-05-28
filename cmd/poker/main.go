package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/RobertGumeny/agent-poker/internal/eval"
	"github.com/RobertGumeny/agent-poker/internal/evalrun"
	"github.com/RobertGumeny/agent-poker/internal/experiment"
)

const defaultThinkingLevel = "low"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand (supported: experiment)")
	}
	switch args[0] {
	case "experiment":
		return runExperiment(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unsupported subcommand %q", args[0])
	}
}

func runExperiment(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand (supported: analyze, go, run, status)")
	}
	switch args[0] {
	case "analyze":
		return runExperimentAnalyze(args[1:], stdout, stderr)
	case "go":
		return runExperimentGo(args[1:], stdout, stderr)
	case "run":
		return runExperimentRun(args[1:], stdout, stderr)
	case "status":
		return runExperimentStatus(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unsupported experiment subcommand %q", args[0])
	}
}

type experimentFlags struct {
	id             string
	experimentsDir string
	sessionsDir    string
	experimentPath string // resolved from id
}

func parseExperimentFlags(fs *flag.FlagSet, args []string) (experimentFlags, error) {
	var ef experimentFlags
	fs.StringVar(&ef.experimentsDir, "experiments-dir", "experiments", "directory containing experiment definition JSON files")
	fs.StringVar(&ef.sessionsDir, "sessions-dir", "sessions", "session output root directory")
	fs.StringVar(&ef.experimentPath, "experiment", "", "explicit path to experiment definition JSON (overrides positional id)")

	if err := fs.Parse(args); err != nil {
		return experimentFlags{}, err
	}

	positional := fs.Args()
	if strings.TrimSpace(ef.experimentPath) == "" {
		if len(positional) == 0 {
			return experimentFlags{}, fmt.Errorf("experiment id is required as a positional argument")
		}
		ef.id = positional[0]
		resolved, err := resolveExperiment(ef.id, ef.experimentsDir)
		if err != nil {
			return experimentFlags{}, err
		}
		ef.experimentPath = resolved
	}
	return ef, nil
}

func resolveExperiment(id, experimentsDir string) (string, error) {
	candidate := filepath.Join(experimentsDir, id+".json")
	if _, err := os.Stat(candidate); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("experiment %q not found at %s", id, candidate)
		}
		return "", fmt.Errorf("stat experiment %q: %w", id, err)
	}
	return candidate, nil
}

func runExperimentStatus(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("poker experiment status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	ef, err := parseExperimentFlags(fs, args)
	if err != nil {
		return err
	}

	coverage, err := evalrun.LoadPlanCoverage(ef.experimentPath, ef.sessionsDir, experiment.Load, evalrun.InspectSession)
	if err != nil {
		return err
	}

	printCoverage(stdout, coverage)

	next := nextStep(coverage)
	_, _ = fmt.Fprintf(stdout, "next=%s\n", next)
	return nil
}

func runExperimentRun(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("poker experiment run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var model string
	var thinkingLevel string
	fs.StringVar(&model, "model", "", "optional PI_POKER_MODEL for Pi agents")
	fs.StringVar(&thinkingLevel, "thinking-level", defaultThinkingLevel, "PI_POKER_THINKING_LEVEL for Pi agents")

	ef, err := parseExperimentFlags(fs, args)
	if err != nil {
		return err
	}

	return execRun(ef, model, thinkingLevel, stdout, stderr)
}

func runExperimentAnalyze(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("poker experiment analyze", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	ef, err := parseExperimentFlags(fs, args)
	if err != nil {
		return err
	}

	return execAnalyze(ef, stdout)
}

func runExperimentGo(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("poker experiment go", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var model string
	var thinkingLevel string
	fs.StringVar(&model, "model", "", "optional PI_POKER_MODEL for Pi agents")
	fs.StringVar(&thinkingLevel, "thinking-level", defaultThinkingLevel, "PI_POKER_THINKING_LEVEL for Pi agents")

	ef, err := parseExperimentFlags(fs, args)
	if err != nil {
		return err
	}

	if err := execRun(ef, model, thinkingLevel, stdout, stderr); err != nil {
		return err
	}
	return execAnalyze(ef, stdout)
}

func execRun(ef experimentFlags, model, thinkingLevel string, stdout, stderr io.Writer) error {
	coverage, err := evalrun.LoadPlanCoverage(ef.experimentPath, ef.sessionsDir, experiment.Load, evalrun.InspectSession)
	if err != nil {
		return err
	}

	printCoverage(stdout, coverage)

	repoDir, err := repoRoot()
	if err != nil {
		return err
	}
	executor := evalrun.NewExecutor(repoDir, stdout, stderr)

	for _, session := range coverage.Sessions {
		if session.Inspection.Status == "present" {
			_, _ = fmt.Fprintf(stdout, "skip session_id=%s reason=present\n", session.Planned.SessionID)
			continue
		}
		if strings.TrimSpace(session.Planned.Opponent) == "" {
			return fmt.Errorf("session %q cannot run without opponent metadata in experiment definition", session.Planned.SessionID)
		}
		_, _ = fmt.Fprintf(stdout, "run session_id=%s group=%s seed=%d prior_status=%s\n", session.Planned.SessionID, session.Planned.GroupLabel, session.Planned.Seed, session.Inspection.Status)
		if err := executor.Execute(context.Background(), evalrun.ExecuteConfig{
			Agent0:        session.Planned.Agent,
			Agent1:        session.Planned.Opponent,
			Hands:         coverage.Plan.HandsPerSession,
			Seed:          session.Planned.Seed,
			SessionID:     session.Planned.SessionID,
			SessionsDir:   ef.sessionsDir,
			Model:         model,
			ThinkingLevel: thinkingLevel,
		}); err != nil {
			return fmt.Errorf("run session %q: %w", session.Planned.SessionID, err)
		}
	}

	// collect any sessions missing eval.json
	return collectMissing(coverage, stdout)
}

func execAnalyze(ef experimentFlags, stdout io.Writer) error {
	// re-load coverage to pick up newly completed sessions
	coverage, err := evalrun.LoadPlanCoverage(ef.experimentPath, ef.sessionsDir, experiment.Load, evalrun.InspectSession)
	if err != nil {
		return err
	}

	if err := collectMissing(coverage, stdout); err != nil {
		return err
	}

	def, err := experiment.Load(ef.experimentPath)
	if err != nil {
		return err
	}

	report, err := eval.Compare(def, ef.sessionsDir)
	if err != nil {
		return err
	}

	reportDir := "reports"
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", reportDir, err)
	}

	reportID := def.ID
	if reportID == "" {
		reportID = strings.TrimSuffix(filepath.Base(ef.experimentPath), ".json")
	}
	reportPath := filepath.Join(reportDir, reportID+".md")
	if err := os.WriteFile(reportPath, []byte(eval.RenderComparisonMarkdown(report)), 0o644); err != nil {
		return fmt.Errorf("write report %s: %w", reportPath, err)
	}

	_, _ = fmt.Fprintf(stdout, "report=%s\n", reportPath)
	return nil
}

func collectMissing(coverage evalrun.PlanCoverage, stdout io.Writer) error {
	for _, session := range coverage.Sessions {
		if session.Inspection.Status != "present" {
			continue
		}
		evalPath := filepath.Join(session.Planned.SessionDir, "eval.json")
		if _, err := os.Stat(evalPath); err == nil {
			continue // already collected
		}
		summary, err := eval.CollectSession(session.Planned.SessionDir)
		if err != nil {
			return fmt.Errorf("collect session %q: %w", session.Planned.SessionID, err)
		}
		if err := eval.WriteSummary(session.Planned.SessionDir, summary); err != nil {
			return fmt.Errorf("write eval.json for session %q: %w", session.Planned.SessionID, err)
		}
		_, _ = fmt.Fprintf(stdout, "collected session_id=%s output=%s\n", summary.SessionID, evalPath)
	}
	return nil
}

func printCoverage(stdout io.Writer, coverage evalrun.PlanCoverage) {
	_, _ = fmt.Fprintf(stdout, "experiment=%s planned=%d present=%d missing=%d incomplete=%d\n", coverage.Plan.ExperimentID, len(coverage.Sessions), coverage.Present, coverage.Missing, coverage.Incomplete)
	for _, label := range []string{"control", "treatment"} {
		summary := coverage.GroupSummaries()[label]
		_, _ = fmt.Fprintf(stdout, "group_summary group=%s planned=%d present=%d missing=%d incomplete=%d\n", label, summary.Planned, summary.Present, summary.Missing, summary.Incomplete)
	}
}

func nextStep(coverage evalrun.PlanCoverage) string {
	if coverage.Missing > 0 || coverage.Incomplete > 0 {
		return "run"
	}
	for _, session := range coverage.Sessions {
		evalPath := filepath.Join(session.Planned.SessionDir, "eval.json")
		if _, err := os.Stat(evalPath); os.IsNotExist(err) {
			return "analyze"
		}
	}
	return "analyze"
}

func repoRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..")), nil
}
