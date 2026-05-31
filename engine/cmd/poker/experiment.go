package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/RobertGumeny/agent-poker/internal/eval"
	"github.com/RobertGumeny/agent-poker/internal/evalrun"
	"github.com/RobertGumeny/agent-poker/internal/experiment"
)

func runExperiment(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand (supported: analyze, go, ls, new, run, status)")
	}
	switch args[0] {
	case "analyze":
		return runExperimentAnalyze(args[1:], stdout, stderr)
	case "go":
		return runExperimentGo(args[1:], stdout, stderr)
	case "ls":
		return runExperimentList(args[1:], stdout, stderr)
	case "new":
		return runExperimentNew(args[1:], stdout, stderr)
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
	fs.StringVar(&ef.experimentsDir, "experiments-dir", "research/experiments", "directory containing experiment slug subdirectories")
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
	ef.sessionsDir = filepath.Join(filepath.Dir(ef.experimentPath), "sessions")
	return ef, nil
}

func resolveExperiment(id, experimentsDir string) (string, error) {
	candidate := filepath.Join(experimentsDir, id, id+".json")
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

func runExperimentNew(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("poker experiment new", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var experimentsDir string
	fs.StringVar(&experimentsDir, "experiments-dir", "research/experiments", "directory containing experiment slug subdirectories")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: poker experiment new <id>")
	}
	id := fs.Arg(0)
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("experiment id must not be empty")
	}

	outputPath := filepath.Join(experimentsDir, id, id+".json")

	def := experiment.Definition{
		ID:              id,
		Hypothesis:      "TODO: describe what you expect to happen and why.",
		Model:           "anthropic:claude-sonnet-4-6",
		HandsPerSession: 25,
		Control: experiment.Group{
			SessionBase:   id + "-control",
			SessionsCount: 5,
			Agent:         "llm-stateless",
			Opponent:      "heuristic",
		},
		Treatment: experiment.Group{
			SessionBase:   id + "-treatment",
			SessionsCount: 5,
			Agent:         "llm-akg-durable",
			Opponent:      "heuristic",
		},
	}

	data, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal experiment template: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(outputPath), err)
	}
	if err := os.WriteFile(outputPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outputPath, err)
	}

	_, _ = fmt.Fprintf(stdout, "created %s\n", outputPath)
	_, _ = fmt.Fprintf(stdout, "edit it, then run: poker experiment go %s\n", id)
	return nil
}

func runExperimentList(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("poker experiment ls", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var experimentsDir string
	fs.StringVar(&experimentsDir, "experiments-dir", "research/experiments", "directory containing experiment slug subdirectories")

	if err := fs.Parse(args); err != nil {
		return err
	}

	paths, err := findExperimentFiles(experimentsDir)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		_, _ = fmt.Fprintf(stdout, "no experiments found in %s\n", experimentsDir)
		return nil
	}

	for _, path := range paths {
		sessionsDir := filepath.Join(filepath.Dir(path), "sessions")
		coverage, err := evalrun.LoadPlanCoverage(path, sessionsDir, experiment.Load, evalrun.InspectSession)
		if err != nil {
			_, _ = fmt.Fprintf(stdout, "id=%-40s status=invalid error=%q\n", filepath.Base(filepath.Dir(path)), err.Error())
			continue
		}
		_, _ = fmt.Fprintf(stdout, "id=%-40s planned=%d present=%d missing=%d incomplete=%d\n",
			coverage.Plan.ExperimentID, len(coverage.Sessions), coverage.Present, coverage.Missing, coverage.Incomplete)
	}
	return nil
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

	effectiveModel := model
	if effectiveModel == "" {
		effectiveModel = coverage.Plan.Model
	}

	var toRun []evalrun.SessionCoverage
	for _, session := range coverage.Sessions {
		if session.Inspection.Status == "present" {
			_, _ = fmt.Fprintf(stdout, "skip session_id=%s reason=present\n", session.Planned.SessionID)
			continue
		}
		if strings.TrimSpace(session.Planned.Opponent) == "" {
			return fmt.Errorf("session %q cannot run without opponent metadata in experiment definition", session.Planned.SessionID)
		}
		toRun = append(toRun, session)
	}

	if len(toRun) == 0 {
		return collectMissing(coverage, stdout)
	}

	// build the binary once before spawning parallel sessions
	if _, err := executor.PrepareBinary(); err != nil {
		return err
	}

	// make sessions dir absolute so the subprocess resolves it correctly
	absSessionsDir, err := filepath.Abs(ef.sessionsDir)
	if err != nil {
		return fmt.Errorf("abs sessions dir: %w", err)
	}

	ids := make([]string, len(toRun))
	for i, s := range toRun {
		ids[i] = s.Planned.SessionID
	}
	disp := newRunDisplay(stdout, ids)
	disp.init()

	var wg sync.WaitGroup
	errc := make(chan error, len(toRun))

	for _, session := range toRun {
		wg.Add(1)
		go func(s evalrun.SessionCoverage) {
			defer wg.Done()
			pw := &handProgressWriter{sessionID: s.Planned.SessionID, disp: disp}
			runErr := executor.Execute(context.Background(), evalrun.ExecuteConfig{
				Agent0:        s.Planned.Agent,
				Agent1:        s.Planned.Opponent,
				Hands:         coverage.Plan.HandsPerSession,
				Seed:          s.Planned.Seed,
				SessionID:     s.Planned.SessionID,
				SessionsDir:   absSessionsDir,
				Model:         effectiveModel,
				ThinkingLevel: thinkingLevel,
				Stdout:        pw,
				Stderr:        stderr,
			})
			if runErr != nil {
				disp.setError(s.Planned.SessionID, runErr.Error())
				errc <- fmt.Errorf("run session %q: %w", s.Planned.SessionID, runErr)
				return
			}
			disp.setDone(s.Planned.SessionID)
		}(session)
	}

	wg.Wait()
	close(errc)
	_, _ = fmt.Fprintln(stdout)

	var firstErr error
	for e := range errc {
		if firstErr == nil {
			firstErr = e
		}
	}
	if firstErr != nil {
		return firstErr
	}

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

	reportDir := filepath.Join(filepath.Dir(ef.experimentPath), "reports")
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

// findExperimentFiles returns paths matching <root>/<id>/<id>.json.
// It only looks one level deep to avoid picking up session artifacts.
func findExperimentFiles(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read experiments dir %s: %w", root, err)
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(root, entry.Name(), entry.Name()+".json")
		if _, err := os.Stat(candidate); err == nil {
			paths = append(paths, candidate)
		}
	}
	sort.Strings(paths)
	return paths, nil
}
