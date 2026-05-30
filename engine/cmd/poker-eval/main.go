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
	"runtime"
	"sort"
	"strings"

	"github.com/RobertGumeny/agent-poker/internal/eval"
	"github.com/RobertGumeny/agent-poker/internal/evalrun"
	"github.com/RobertGumeny/agent-poker/internal/experiment"
)

const defaultThinkingLevel = "low"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr, newRunDeps()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer, deps runDeps) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand (supported: collect, compare, init, ls, run, status)")
	}

	switch args[0] {
	case "collect":
		return runCollect(args[1:], stdout, stderr)
	case "compare":
		return runCompare(args[1:], stdout, stderr, deps)
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "ls":
		return runList(args[1:], stdout, stderr, deps)
	case "run":
		return runRun(args[1:], stdout, stderr, deps)
	case "status":
		return runStatus(args[1:], stdout, stderr, deps)
	default:
		return fmt.Errorf("unsupported subcommand %q", args[0])
	}
}

type runDeps struct {
	loadDefinition func(string) (experiment.Definition, error)
	inspectSession func(experiment.PlannedRun, int) (evalrun.SessionInspection, error)
	execute        func(context.Context, evalrun.ExecuteConfig) error
}

func newRunDeps() runDeps {
	repoDir, err := repoRoot()
	if err != nil {
		return runDeps{
			loadDefinition: experiment.Load,
			inspectSession: evalrun.InspectSession,
			execute: func(context.Context, evalrun.ExecuteConfig) error {
				return err
			},
		}
	}

	executor := evalrun.NewExecutor(repoDir, os.Stdout, os.Stderr)

	return runDeps{
		loadDefinition: experiment.Load,
		inspectSession: evalrun.InspectSession,
		execute:        executor.Execute,
	}
}

type runConfig struct {
	experimentPath string
	sessionsDir    string
	dryRun         bool
	model          string
	thinkingLevel  string
}

type statusConfig struct {
	experimentPath string
	sessionsDir    string
}

type collectConfig struct {
	sessionDirs []string
}

type compareConfig struct {
	experimentPath string
	sessionsDir    string
}

type initConfig struct {
	outputPath           string
	id                   string
	hypothesis           string
	model                string
	handsPerSession      int
	sessionsCount        int
	controlAgent         string
	controlOpponent      string
	controlSessionBase   string
	treatmentAgent       string
	treatmentOpponent    string
	treatmentSessionBase string
}

type listConfig struct {
	experimentsDir string
	sessionsDir    string
}

func runCollect(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseCollectConfig(args)
	if err != nil {
		return err
	}

	for _, sessionDir := range cfg.sessionDirs {
		summary, err := eval.CollectSession(sessionDir)
		if err != nil {
			return err
		}
		if err := eval.WriteSummary(sessionDir, summary); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "collected session_id=%s output=%s\n", summary.SessionID, filepath.Join(sessionDir, "eval.json"))
	}
	return nil
}

func runCompare(args []string, stdout, stderr io.Writer, deps runDeps) error {
	cfg, err := parseCompareConfig(args)
	if err != nil {
		return err
	}

	def, err := deps.loadDefinition(cfg.experimentPath)
	if err != nil {
		return err
	}
	report, err := eval.Compare(def, cfg.sessionsDir)
	if err != nil {
		return err
	}
	_, _ = io.WriteString(stdout, eval.RenderComparisonMarkdown(report))
	return nil
}

func runInit(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseInitConfig(args)
	if err != nil {
		return err
	}

	def := experiment.Definition{
		ID:              cfg.id,
		Hypothesis:      cfg.hypothesis,
		Model:           cfg.model,
		HandsPerSession: cfg.handsPerSession,
		Control: experiment.Group{
			SessionBase:   cfg.controlSessionBase,
			SessionsCount: cfg.sessionsCount,
			Agent:         cfg.controlAgent,
			Opponent:      cfg.controlOpponent,
		},
		Treatment: experiment.Group{
			SessionBase:   cfg.treatmentSessionBase,
			SessionsCount: cfg.sessionsCount,
			Agent:         cfg.treatmentAgent,
			Opponent:      cfg.treatmentOpponent,
		},
	}
	if err := def.Validate(); err != nil {
		return err
	}

	data, err := jsonMarshalIndent(def)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.outputPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(cfg.outputPath), err)
	}
	if err := os.WriteFile(cfg.outputPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write experiment template %s: %w", cfg.outputPath, err)
	}
	_, _ = fmt.Fprintf(stdout, "initialized experiment id=%s output=%s planned_sessions=%d\n", def.ID, cfg.outputPath, cfg.sessionsCount*2)
	return nil
}

func runList(args []string, stdout, stderr io.Writer, deps runDeps) error {
	cfg, err := parseListConfig(args)
	if err != nil {
		return err
	}

	experimentPaths, err := findExperimentFiles(cfg.experimentsDir)
	if err != nil {
		return err
	}
	if len(experimentPaths) == 0 {
		_, _ = fmt.Fprintf(stdout, "experiments_dir=%s count=0\n", cfg.experimentsDir)
		return nil
	}

	_, _ = fmt.Fprintf(stdout, "experiments_dir=%s count=%d\n", cfg.experimentsDir, len(experimentPaths))
	for _, experimentPath := range experimentPaths {
		coverage, err := loadPlanCoverage(experimentPath, cfg.sessionsDir, deps)
		if err != nil {
			_, _ = fmt.Fprintf(stdout, "path=%s status=invalid error=%q\n", experimentPath, err.Error())
			continue
		}
		_, _ = fmt.Fprintf(stdout, "path=%s id=%s planned=%d present=%d missing=%d incomplete=%d hands_per_session=%d\n", experimentPath, coverage.Plan.ExperimentID, len(coverage.Sessions), coverage.Present, coverage.Missing, coverage.Incomplete, coverage.Plan.HandsPerSession)
		for _, label := range []string{"control", "treatment"} {
			summary := coverage.GroupSummaries()[label]
			_, _ = fmt.Fprintf(stdout, "group_summary path=%s group=%s planned=%d present=%d missing=%d incomplete=%d\n", experimentPath, label, summary.Planned, summary.Present, summary.Missing, summary.Incomplete)
		}
	}
	return nil
}

func runRun(args []string, stdout, stderr io.Writer, deps runDeps) error {
	cfg, err := parseRunConfig(args)
	if err != nil {
		return err
	}

	coverage, err := loadPlanCoverage(cfg.experimentPath, cfg.sessionsDir, deps)
	if err != nil {
		return err
	}

	printCoverage(stdout, coverage, func() string {
		return fmt.Sprintf("config hands_per_session=%d sessions_dir=%s model=%s thinking_level=%s", coverage.Plan.HandsPerSession, cfg.sessionsDir, printableValue(cfg.model), cfg.thinkingLevel)
	}())

	if cfg.dryRun {
		return nil
	}

	for _, session := range coverage.Sessions {
		if session.Inspection.Status == "present" {
			_, _ = fmt.Fprintf(stdout, "skip session_id=%s reason=present\n", session.Planned.SessionID)
			continue
		}
		if strings.TrimSpace(session.Planned.Opponent) == "" {
			return fmt.Errorf("session %q cannot run without opponent metadata in experiment definition", session.Planned.SessionID)
		}
		_, _ = fmt.Fprintf(stdout, "run session_id=%s group=%s seed=%d prior_status=%s\n", session.Planned.SessionID, session.Planned.GroupLabel, session.Planned.Seed, session.Inspection.Status)
		if err := deps.execute(context.Background(), evalrun.ExecuteConfig{
			Agent0:        session.Planned.Agent,
			Agent1:        session.Planned.Opponent,
			Hands:         coverage.Plan.HandsPerSession,
			Seed:          session.Planned.Seed,
			SessionID:     session.Planned.SessionID,
			SessionsDir:   cfg.sessionsDir,
			Model:         cfg.model,
			ThinkingLevel: cfg.thinkingLevel,
		}); err != nil {
			return fmt.Errorf("run session %q: %w", session.Planned.SessionID, err)
		}
	}

	return nil
}

func runStatus(args []string, stdout, stderr io.Writer, deps runDeps) error {
	cfg, err := parseStatusConfig(args)
	if err != nil {
		return err
	}

	coverage, err := loadPlanCoverage(cfg.experimentPath, cfg.sessionsDir, deps)
	if err != nil {
		return err
	}

	printCoverage(stdout, coverage, fmt.Sprintf("config hands_per_session=%d sessions_dir=%s", coverage.Plan.HandsPerSession, cfg.sessionsDir))
	return nil
}

func loadPlanCoverage(experimentPath, sessionsDir string, deps runDeps) (evalrun.PlanCoverage, error) {
	coverage, err := evalrun.LoadPlanCoverage(experimentPath, sessionsDir, deps.loadDefinition, deps.inspectSession)
	if err != nil {
		return evalrun.PlanCoverage{}, err
	}
	return coverage, nil
}

func printCoverage(stdout io.Writer, coverage evalrun.PlanCoverage, configLine string) {
	_, _ = fmt.Fprintf(stdout, "experiment=%s planned=%d present=%d missing=%d incomplete=%d\n", coverage.Plan.ExperimentID, len(coverage.Sessions), coverage.Present, coverage.Missing, coverage.Incomplete)
	_, _ = fmt.Fprintln(stdout, configLine)
	for _, label := range []string{"control", "treatment"} {
		summary := coverage.GroupSummaries()[label]
		_, _ = fmt.Fprintf(stdout, "group_summary group=%s planned=%d present=%d missing=%d incomplete=%d\n", label, summary.Planned, summary.Present, summary.Missing, summary.Incomplete)
	}
	for _, session := range coverage.Sessions {
		reason := session.Inspection.Reason
		if strings.TrimSpace(reason) == "" {
			reason = "-"
		}
		_, _ = fmt.Fprintf(stdout, "group=%s session_id=%s seed=%d agent=%s opponent=%s status=%s reason=%s dir=%s\n", session.Planned.GroupLabel, session.Planned.SessionID, session.Planned.Seed, session.Planned.Agent, session.Planned.Opponent, session.Inspection.Status, reason, session.Planned.SessionDir)
	}
}

func parseCollectConfig(args []string) (collectConfig, error) {
	fs := flag.NewFlagSet("poker-eval collect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return collectConfig{}, err
	}
	if fs.NArg() == 0 {
		return collectConfig{}, fmt.Errorf("at least one session directory is required")
	}
	return collectConfig{sessionDirs: fs.Args()}, nil
}

func parseCompareConfig(args []string) (compareConfig, error) {
	fs := flag.NewFlagSet("poker-eval compare", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := compareConfig{}
	fs.StringVar(&cfg.experimentPath, "experiment", "", "path to experiment definition JSON")
	fs.StringVar(&cfg.sessionsDir, "sessions-dir", "sessions", "session output root directory")

	if err := fs.Parse(args); err != nil {
		return compareConfig{}, err
	}
	if fs.NArg() != 0 {
		return compareConfig{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(cfg.experimentPath) == "" {
		return compareConfig{}, fmt.Errorf("-experiment is required")
	}
	return cfg, nil
}

func parseInitConfig(args []string) (initConfig, error) {
	fs := flag.NewFlagSet("poker-eval init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := initConfig{}
	fs.StringVar(&cfg.outputPath, "out", "", "path to write experiment definition JSON")
	fs.StringVar(&cfg.id, "id", "", "experiment id (defaults to output filename without .json)")
	fs.StringVar(&cfg.hypothesis, "hypothesis", "", "optional hypothesis text")
	fs.StringVar(&cfg.model, "model", "", "model identifier (e.g. anthropic:claude-sonnet-4-6)")
	fs.IntVar(&cfg.handsPerSession, "hands-per-session", 25, "expected hand count for each session")
	fs.IntVar(&cfg.sessionsCount, "sessions-count", 5, "number of sessions to plan per group")
	fs.StringVar(&cfg.controlAgent, "control-agent", "llm-stateless", "control group agent identifier")
	fs.StringVar(&cfg.controlOpponent, "control-opponent", "heuristic", "control group opponent identifier")
	fs.StringVar(&cfg.controlSessionBase, "control-session-base", "", "control group session_base (defaults to <id>-control)")
	fs.StringVar(&cfg.treatmentAgent, "treatment-agent", "llm-akg-recent", "treatment group agent identifier")
	fs.StringVar(&cfg.treatmentOpponent, "treatment-opponent", "", "treatment group opponent identifier (defaults to control-opponent)")
	fs.StringVar(&cfg.treatmentSessionBase, "treatment-session-base", "", "treatment group session_base (defaults to <id>-treatment)")

	if err := fs.Parse(args); err != nil {
		return initConfig{}, err
	}
	if fs.NArg() != 0 {
		return initConfig{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(cfg.outputPath) == "" {
		return initConfig{}, fmt.Errorf("-out is required")
	}
	if strings.TrimSpace(cfg.id) == "" {
		cfg.id = strings.TrimSuffix(filepath.Base(cfg.outputPath), filepath.Ext(cfg.outputPath))
	}
	if strings.TrimSpace(cfg.id) == "" {
		return initConfig{}, fmt.Errorf("-id is required when it cannot be derived from -out")
	}
	if strings.TrimSpace(cfg.treatmentOpponent) == "" {
		cfg.treatmentOpponent = cfg.controlOpponent
	}
	if strings.TrimSpace(cfg.controlSessionBase) == "" {
		cfg.controlSessionBase = cfg.id + "-control"
	}
	if strings.TrimSpace(cfg.treatmentSessionBase) == "" {
		cfg.treatmentSessionBase = cfg.id + "-treatment"
	}
	return cfg, nil
}

func parseListConfig(args []string) (listConfig, error) {
	fs := flag.NewFlagSet("poker-eval ls", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := listConfig{}
	fs.StringVar(&cfg.experimentsDir, "experiments-dir", "experiments", "directory containing experiment definition JSON files")
	fs.StringVar(&cfg.sessionsDir, "sessions-dir", "sessions", "session output root directory")

	if err := fs.Parse(args); err != nil {
		return listConfig{}, err
	}
	if fs.NArg() != 0 {
		return listConfig{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	return cfg, nil
}

func parseRunConfig(args []string) (runConfig, error) {
	fs := flag.NewFlagSet("poker-eval run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := runConfig{}
	fs.StringVar(&cfg.experimentPath, "experiment", "", "path to experiment definition JSON")
	fs.StringVar(&cfg.sessionsDir, "sessions-dir", "sessions", "session output root directory")
	fs.BoolVar(&cfg.dryRun, "dry-run", false, "print the deterministic session plan without launching missing or incomplete sessions")
	fs.StringVar(&cfg.model, "model", "", "optional PI_POKER_MODEL for Pi agents")
	fs.StringVar(&cfg.thinkingLevel, "thinking-level", defaultThinkingLevel, "PI_POKER_THINKING_LEVEL for Pi agents")

	if err := fs.Parse(args); err != nil {
		return runConfig{}, err
	}
	if fs.NArg() != 0 {
		return runConfig{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(cfg.experimentPath) == "" {
		return runConfig{}, fmt.Errorf("-experiment is required")
	}
	return cfg, nil
}

func parseStatusConfig(args []string) (statusConfig, error) {
	fs := flag.NewFlagSet("poker-eval status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := statusConfig{}
	fs.StringVar(&cfg.experimentPath, "experiment", "", "path to experiment definition JSON")
	fs.StringVar(&cfg.sessionsDir, "sessions-dir", "sessions", "session output root directory")

	if err := fs.Parse(args); err != nil {
		return statusConfig{}, err
	}
	if fs.NArg() != 0 {
		return statusConfig{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(cfg.experimentPath) == "" {
		return statusConfig{}, fmt.Errorf("-experiment is required")
	}
	return cfg, nil
}

func repoRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..")), nil
}

func printableValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<default>"
	}
	return value
}

func findExperimentFiles(root string) ([]string, error) {
	var paths []string
	if _, err := os.Stat(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat experiments dir %s: %w", root, err)
	}
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk experiments dir %s: %w", root, err)
	}
	sort.Strings(paths)
	return paths, nil
}

func jsonMarshalIndent(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal experiment template: %w", err)
	}
	return data, nil
}
