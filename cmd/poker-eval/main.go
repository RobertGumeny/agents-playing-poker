package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/RobertGumeny/agent-poker/internal/eval"
	"github.com/RobertGumeny/agent-poker/internal/experiment"
	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
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
		return fmt.Errorf("expected subcommand (supported: collect, compare, run, status)")
	}

	switch args[0] {
	case "collect":
		return runCollect(args[1:], stdout, stderr)
	case "compare":
		return runCompare(args[1:], stdout, stderr, deps)
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
	inspectSession func(experiment.PlannedRun, int) (sessionInspection, error)
	execute        func(context.Context, executeConfig) error
}

type executeConfig struct {
	Agent0        string
	Agent1        string
	Hands         int
	Seed          int64
	SessionID     string
	SessionsDir   string
	Model         string
	ThinkingLevel string
}

type sessionInspection struct {
	Status string
	Reason string
}

type planCoverage struct {
	Plan       experiment.Plan
	Sessions   []sessionCoverage
	Present    int
	Missing    int
	Incomplete int
}

type sessionCoverage struct {
	Planned    experiment.PlannedRun
	Inspection sessionInspection
}

type groupCoverage struct {
	Planned    int
	Present    int
	Missing    int
	Incomplete int
}

func newRunDeps() runDeps {
	repoDir, err := repoRoot()
	if err != nil {
		return runDeps{
			loadDefinition: experiment.Load,
			inspectSession: inspectSession,
			execute: func(context.Context, executeConfig) error {
				return err
			},
		}
	}

	executor := pokerRunExecutor{
		repoDir:  repoDir,
		goBinary: "go",
		buildBinary: func(repoDir, goBinary, pkg, outputPath string) error {
			return buildGoBinary(repoDir, goBinary, pkg, outputPath)
		},
		runCommand: func(cmd *exec.Cmd) error { return cmd.Run() },
		stdout:     os.Stdout,
		stderr:     os.Stderr,
	}

	return runDeps{
		loadDefinition: experiment.Load,
		inspectSession: inspectSession,
		execute:        executor.execute,
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
		if err := deps.execute(context.Background(), executeConfig{
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

func loadPlanCoverage(experimentPath, sessionsDir string, deps runDeps) (planCoverage, error) {
	def, err := deps.loadDefinition(experimentPath)
	if err != nil {
		return planCoverage{}, err
	}
	plan, err := def.Plan(sessionsDir)
	if err != nil {
		return planCoverage{}, err
	}

	coverage := planCoverage{Plan: plan}
	for _, planned := range plan.PlannedSessions {
		inspection, err := deps.inspectSession(planned, plan.HandsPerSession)
		if err != nil {
			return planCoverage{}, fmt.Errorf("inspect session %q: %w", planned.SessionID, err)
		}
		coverage.Sessions = append(coverage.Sessions, sessionCoverage{Planned: planned, Inspection: inspection})
		switch inspection.Status {
		case "present":
			coverage.Present++
		case "incomplete":
			coverage.Incomplete++
		default:
			coverage.Missing++
		}
	}

	return coverage, nil
}

func printCoverage(stdout io.Writer, coverage planCoverage, configLine string) {
	_, _ = fmt.Fprintf(stdout, "experiment=%s planned=%d present=%d missing=%d incomplete=%d\n", coverage.Plan.ExperimentID, len(coverage.Sessions), coverage.Present, coverage.Missing, coverage.Incomplete)
	_, _ = fmt.Fprintln(stdout, configLine)
	for _, label := range []string{"control", "treatment"} {
		summary := coverage.groupSummaries()[label]
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

func (c planCoverage) groupSummaries() map[string]groupCoverage {
	summaries := map[string]groupCoverage{
		"control":   {},
		"treatment": {},
	}
	for _, session := range c.Sessions {
		summary := summaries[session.Planned.GroupLabel]
		summary.Planned++
		switch session.Inspection.Status {
		case "present":
			summary.Present++
		case "incomplete":
			summary.Incomplete++
		default:
			summary.Missing++
		}
		summaries[session.Planned.GroupLabel] = summary
	}
	return summaries
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

func inspectSession(planned experiment.PlannedRun, handsPerSession int) (sessionInspection, error) {
	_, err := os.Stat(planned.SessionDir)
	if err == nil {
		return inspectExistingSession(planned, handsPerSession)
	}
	if errors.Is(err, os.ErrNotExist) {
		return sessionInspection{Status: "missing"}, nil
	}
	return sessionInspection{}, err
}

func inspectExistingSession(planned experiment.PlannedRun, handsPerSession int) (sessionInspection, error) {
	manifest, err := sessionlog.ReadManifest(planned.SessionDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sessionInspection{Status: "incomplete", Reason: "manifest_missing"}, nil
		}
		return sessionInspection{Status: "incomplete", Reason: "manifest_unreadable"}, nil
	}
	if manifest.SessionID != "" && manifest.SessionID != planned.SessionID {
		return sessionInspection{Status: "incomplete", Reason: "session_id_mismatch"}, nil
	}
	if manifest.Seed != planned.Seed {
		return sessionInspection{Status: "incomplete", Reason: "seed_mismatch"}, nil
	}
	if manifest.HandCount != handsPerSession {
		return sessionInspection{Status: "incomplete", Reason: "hand_count_mismatch"}, nil
	}
	if len(manifest.Matches) == 0 {
		return sessionInspection{Status: "incomplete", Reason: "manifest_missing_match"}, nil
	}
	if !manifest.Matches[0].Completed {
		return sessionInspection{Status: "incomplete", Reason: "match_incomplete"}, nil
	}
	if !matchHasSeat(manifest.Matches[0], planned.Agent) {
		return sessionInspection{Status: "incomplete", Reason: "agent_missing"}, nil
	}
	if strings.TrimSpace(planned.Opponent) != "" && !matchHasSeat(manifest.Matches[0], planned.Opponent) {
		return sessionInspection{Status: "incomplete", Reason: "opponent_missing"}, nil
	}

	hands, err := sessionlog.ReadHands(planned.SessionDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sessionInspection{Status: "incomplete", Reason: "hands_missing"}, nil
		}
		return sessionInspection{Status: "incomplete", Reason: "hands_unreadable"}, nil
	}
	if len(hands) != handsPerSession {
		return sessionInspection{Status: "incomplete", Reason: "hands_count_mismatch"}, nil
	}

	return sessionInspection{Status: "present"}, nil
}

func matchHasSeat(match sessionlog.ManifestMatch, name string) bool {
	for _, seat := range match.Seats {
		if seat.Name == name {
			return true
		}
	}
	return false
}

type pokerRunExecutor struct {
	repoDir     string
	goBinary    string
	buildBinary func(repoDir, goBinary, pkg, outputPath string) error
	runCommand  func(*exec.Cmd) error
	stdout      io.Writer
	stderr      io.Writer
	binaryPath  string
}

func (e *pokerRunExecutor) execute(ctx context.Context, cfg executeConfig) error {
	binaryPath, err := e.ensureBinary()
	if err != nil {
		return err
	}

	args := []string{
		"-agent0", cfg.Agent0,
		"-agent1", cfg.Agent1,
		"-hands", strconv.Itoa(cfg.Hands),
		"-seed", strconv.FormatInt(cfg.Seed, 10),
		"-session-id", cfg.SessionID,
		"-sessions-dir", cfg.SessionsDir,
		"-thinking-level", cfg.ThinkingLevel,
	}
	if cfg.Model != "" {
		args = append(args, "-model", cfg.Model)
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = e.repoDir
	cmd.Stdout = e.stdout
	cmd.Stderr = e.stderr
	if err := e.runCommand(cmd); err != nil {
		return fmt.Errorf("execute poker-run: %w", err)
	}
	return nil
}

func (e *pokerRunExecutor) ensureBinary() (string, error) {
	if e.binaryPath != "" {
		return e.binaryPath, nil
	}
	outputPath := filepath.Join(e.repoDir, ".tmp", "bin", binaryName("poker-run"))
	if err := e.buildBinary(e.repoDir, e.goBinary, "./cmd/poker-run", outputPath); err != nil {
		return "", err
	}
	e.binaryPath = outputPath
	return e.binaryPath, nil
}

func buildGoBinary(repoDir, goBinary, pkg, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(outputPath), err)
	}

	cmd := exec.Command(goBinary, "build", "-o", outputPath, pkg)
	cmd.Dir = repoDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build %s: %w\n%s%s", pkg, err, stdout.String(), stderr.String())
	}
	return nil
}

func repoRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..")), nil
}

func binaryName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func printableValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<default>"
	}
	return value
}
