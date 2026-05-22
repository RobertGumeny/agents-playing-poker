package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/RobertGumeny/agent-poker/internal/match"
)

const (
	defaultMatchID          = "mat_001"
	defaultStartingStack    = 200
	defaultSmallBlind       = 1
	defaultBigBlind         = 2
	defaultDecisionDeadline = 30 * time.Second
	defaultThinkingLevel    = "low"
	llmStatelessAlias       = "llm-stateless"
	heuristicAlias          = "heuristic"
	randomAlias             = "random"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}

	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	resolver := agentAliasResolver{
		repoDir:  repoDir,
		goBinary: "go",
		lookPath: exec.LookPath,
		buildBinary: func(repoDir, goBinary, pkg, outputPath string) error {
			return buildGoBinary(repoDir, goBinary, pkg, outputPath)
		},
	}

	agent0, err := resolver.resolve(cfg.agent0, cfg.model, cfg.thinkingLevel)
	if err != nil {
		return fmt.Errorf("resolve -agent0 %q: %w", cfg.agent0, err)
	}
	agent1, err := resolver.resolve(cfg.agent1, cfg.model, cfg.thinkingLevel)
	if err != nil {
		return fmt.Errorf("resolve -agent1 %q: %w", cfg.agent1, err)
	}

	runner, err := match.NewRunner(match.Config{
		SessionID:        cfg.sessionID,
		SessionsRootDir:  cfg.sessionsDir,
		MatchID:          defaultMatchID,
		Seed:             cfg.seed,
		HandCount:        cfg.hands,
		StartingStack:    defaultStartingStack,
		SmallBlind:       defaultSmallBlind,
		BigBlind:         defaultBigBlind,
		DecisionDeadline: defaultDecisionDeadline,
		AgentSpecs:       []match.AgentSpec{agent0, agent1},
	})
	if err != nil {
		return err
	}

	result, err := runner.Run(context.Background())
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "session_dir=%s completed=%t\n", result.SessionDir, result.Completed)
	return nil
}

type config struct {
	agent0        string
	agent1        string
	hands         int
	seed          int64
	sessionID     string
	sessionsDir   string
	model         string
	thinkingLevel string
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("poker-run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := config{}
	fs.StringVar(&cfg.agent0, "agent0", "", "seat 0 agent alias")
	fs.StringVar(&cfg.agent1, "agent1", "", "seat 1 agent alias")
	fs.IntVar(&cfg.hands, "hands", 200, "number of hands to play")
	fs.Int64Var(&cfg.seed, "seed", 1, "deterministic match seed")
	fs.StringVar(&cfg.sessionID, "session-id", defaultSessionID(), "session id")
	fs.StringVar(&cfg.sessionsDir, "sessions-dir", "sessions", "session output root directory")
	fs.StringVar(&cfg.model, "model", "", "optional PI_POKER_MODEL for Pi agents")
	fs.StringVar(&cfg.thinkingLevel, "thinking-level", defaultThinkingLevel, "PI_POKER_THINKING_LEVEL for Pi agents")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if fs.NArg() != 0 {
		return config{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if cfg.agent0 == "" || cfg.agent1 == "" {
		return config{}, fmt.Errorf("both -agent0 and -agent1 are required")
	}
	return cfg, nil
}

type agentAliasResolver struct {
	repoDir     string
	goBinary    string
	lookPath    func(string) (string, error)
	buildBinary func(repoDir, goBinary, pkg, outputPath string) error
}

func (r agentAliasResolver) resolve(alias, model, thinkingLevel string) (match.AgentSpec, error) {
	switch alias {
	case llmStatelessAlias:
		return r.resolveLLMStateless(model, thinkingLevel)
	case heuristicAlias:
		return r.resolveGoAgent(heuristicAlias, "./cmd/heuristic-agent", binaryName("heuristic-agent"))
	case randomAlias:
		return r.resolveGoAgent(randomAlias, "./cmd/random-agent", binaryName("random-agent"))
	default:
		return match.AgentSpec{}, fmt.Errorf("unsupported agent alias %q (supported: %s, %s, %s)", alias, llmStatelessAlias, heuristicAlias, randomAlias)
	}
}

func (r agentAliasResolver) resolveLLMStateless(model, thinkingLevel string) (match.AgentSpec, error) {
	nodePath, err := r.lookPath("node")
	if err != nil {
		return match.AgentSpec{}, fmt.Errorf("find node: %w", err)
	}
	nodePath, err = filepath.Abs(nodePath)
	if err != nil {
		return match.AgentSpec{}, fmt.Errorf("absolute node path: %w", err)
	}

	scriptPath := filepath.Join(r.repoDir, "pi-agents", "llm-stateless", "dist", "main.js")
	if _, err := os.Stat(scriptPath); err != nil {
		return match.AgentSpec{}, fmt.Errorf("stat %s: %w (build the Pi agent with: cd %s && npm run build)", scriptPath, err, filepath.Join(r.repoDir, "pi-agents"))
	}

	env := []string{
		"PI_POKER_FAKE_DECISIONS_JSON=",
		fmt.Sprintf("PI_POKER_THINKING_LEVEL=%s", thinkingLevel),
	}
	if model != "" {
		env = append(env, fmt.Sprintf("PI_POKER_MODEL=%s", model))
	}

	return match.AgentSpec{
		Name:    llmStatelessAlias,
		Command: nodePath,
		Args:    []string{scriptPath},
		Env:     env,
	}, nil
}

func (r agentAliasResolver) resolveGoAgent(name, pkg, binary string) (match.AgentSpec, error) {
	outputPath := filepath.Join(r.repoDir, ".tmp", "bin", binary)
	if err := r.buildBinary(r.repoDir, r.goBinary, pkg, outputPath); err != nil {
		return match.AgentSpec{}, err
	}
	return match.AgentSpec{Name: name, Command: outputPath}, nil
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

func defaultSessionID() string {
	return "ses_" + strings.ReplaceAll(time.Now().UTC().Format("2006-01-02T15-04-05Z"), ":", "-")
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
