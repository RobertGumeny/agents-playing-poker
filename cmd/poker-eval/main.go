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
		return fmt.Errorf("expected subcommand (supported: run)")
	}

	switch args[0] {
	case "run":
		return runRun(args[1:], stdout, stderr, deps)
	default:
		return fmt.Errorf("unsupported subcommand %q", args[0])
	}
}

type runDeps struct {
	loadDefinition func(string) (experiment.Definition, error)
	sessionStatus  func(string) (string, error)
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

func newRunDeps() runDeps {
	repoDir, err := repoRoot()
	if err != nil {
		return runDeps{
			loadDefinition: experiment.Load,
			sessionStatus:  sessionStatus,
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
		sessionStatus:  sessionStatus,
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

func runRun(args []string, stdout, stderr io.Writer, deps runDeps) error {
	cfg, err := parseRunConfig(args)
	if err != nil {
		return err
	}

	def, err := deps.loadDefinition(cfg.experimentPath)
	if err != nil {
		return err
	}
	plan, err := def.Plan(cfg.sessionsDir)
	if err != nil {
		return err
	}

	existingCount := 0
	missingCount := 0
	statuses := make([]string, len(plan.PlannedSessions))
	for i, planned := range plan.PlannedSessions {
		status, err := deps.sessionStatus(planned.SessionDir)
		if err != nil {
			return fmt.Errorf("inspect session %q: %w", planned.SessionID, err)
		}
		statuses[i] = status
		switch status {
		case "existing":
			existingCount++
		default:
			missingCount++
		}
	}

	_, _ = fmt.Fprintf(stdout, "experiment=%s planned=%d existing=%d missing=%d dry_run=%t\n", plan.ExperimentID, len(plan.PlannedSessions), existingCount, missingCount, cfg.dryRun)
	_, _ = fmt.Fprintf(stdout, "config hands_per_session=%d sessions_dir=%s model=%s thinking_level=%s\n", plan.HandsPerSession, cfg.sessionsDir, printableValue(cfg.model), cfg.thinkingLevel)
	for i, planned := range plan.PlannedSessions {
		_, _ = fmt.Fprintf(stdout, "group=%s session_id=%s seed=%d agent=%s opponent=%s status=%s dir=%s\n", planned.GroupLabel, planned.SessionID, planned.Seed, planned.Agent, planned.Opponent, statuses[i], planned.SessionDir)
	}

	if cfg.dryRun {
		return nil
	}

	for i, planned := range plan.PlannedSessions {
		if statuses[i] == "existing" {
			_, _ = fmt.Fprintf(stdout, "skip session_id=%s reason=existing\n", planned.SessionID)
			continue
		}
		if strings.TrimSpace(planned.Opponent) == "" {
			return fmt.Errorf("session %q cannot run without opponent metadata in experiment definition", planned.SessionID)
		}
		_, _ = fmt.Fprintf(stdout, "run session_id=%s group=%s seed=%d\n", planned.SessionID, planned.GroupLabel, planned.Seed)
		if err := deps.execute(context.Background(), executeConfig{
			Agent0:        planned.Agent,
			Agent1:        planned.Opponent,
			Hands:         plan.HandsPerSession,
			Seed:          planned.Seed,
			SessionID:     planned.SessionID,
			SessionsDir:   cfg.sessionsDir,
			Model:         cfg.model,
			ThinkingLevel: cfg.thinkingLevel,
		}); err != nil {
			return fmt.Errorf("run session %q: %w", planned.SessionID, err)
		}
	}

	return nil
}

func parseRunConfig(args []string) (runConfig, error) {
	fs := flag.NewFlagSet("poker-eval run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := runConfig{}
	fs.StringVar(&cfg.experimentPath, "experiment", "", "path to experiment definition JSON")
	fs.StringVar(&cfg.sessionsDir, "sessions-dir", "sessions", "session output root directory")
	fs.BoolVar(&cfg.dryRun, "dry-run", false, "print the deterministic session plan without launching missing sessions")
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

func sessionStatus(sessionDir string) (string, error) {
	_, err := os.Stat(sessionDir)
	if err == nil {
		return "existing", nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return "missing", nil
	}
	return "", err
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
