package main

import (
	"context"
	"encoding/json"
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
	"github.com/RobertGumeny/agent-poker/internal/sessionreport"
)

func runMatch(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand (supported: run)")
	}
	switch args[0] {
	case "run":
		return runMatchRun(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unsupported match subcommand %q", args[0])
	}
}

func runMatchRun(args []string, stdout, _ io.Writer) error {
	fs := flag.NewFlagSet("poker match run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var agent0, agent1, sessionID, sessionsDir, model, thinkingLevel string
	var hands int
	var seed int64

	fs.StringVar(&agent0, "agent0", "", "seat 0 agent key")
	fs.StringVar(&agent1, "agent1", "", "seat 1 agent key")
	fs.IntVar(&hands, "hands", 200, "number of hands to play")
	fs.Int64Var(&seed, "seed", 1, "deterministic match seed")
	fs.StringVar(&sessionID, "session-id", defaultMatchSessionID(), "session id")
	fs.StringVar(&sessionsDir, "sessions-dir", "research/sessions", "session output root directory")
	fs.StringVar(&model, "model", "", "optional PI_POKER_MODEL for Pi agents")
	fs.StringVar(&thinkingLevel, "thinking-level", "low", "PI_POKER_THINKING_LEVEL for Pi agents")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if agent0 == "" || agent1 == "" {
		return fmt.Errorf("both --agent0 and --agent1 are required")
	}

	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	resolver := &agentResolver{
		repoDir:  repoDir,
		lookPath: exec.LookPath,
	}

	spec0, err := resolver.resolve(agent0, model, thinkingLevel)
	if err != nil {
		return fmt.Errorf("resolve --agent0 %q: %w", agent0, err)
	}
	spec1, err := resolver.resolve(agent1, model, thinkingLevel)
	if err != nil {
		return fmt.Errorf("resolve --agent1 %q: %w", agent1, err)
	}

	runner, err := match.NewRunner(match.Config{
		SessionID:        sessionID,
		SessionsRootDir:  sessionsDir,
		MatchID:          "mat_001",
		Seed:             seed,
		HandCount:        hands,
		StartingStack:    200,
		SmallBlind:       1,
		BigBlind:         2,
		DecisionDeadline: 30 * time.Second,
		AgentSpecs:       []match.AgentSpec{spec0, spec1},
		ProgressWriter:   stdout,
	})
	if err != nil {
		return err
	}

	result, err := runner.Run(context.Background())
	if err != nil {
		return err
	}
	if result.Completed {
		if reportErr := sessionreport.Generate(result.SessionDir); reportErr != nil {
			fmt.Fprintf(os.Stderr, "warning: report generation failed: %v\n", reportErr)
		}
	}
	_, _ = fmt.Fprintf(stdout, "session_dir=%s completed=%t\n", result.SessionDir, result.Completed)
	return nil
}

// agentResolver resolves a strategy key to a match.AgentSpec using pi-agents/registry.json.
type agentResolver struct {
	repoDir  string
	lookPath func(string) (string, error)
}

type registryFile struct {
	Strategies []registryEntry `json:"strategies"`
}

type registryEntry struct {
	Key  string `json:"key"`
	Type string `json:"type"`
	Pkg  string `json:"pkg,omitempty"`
}

func (r *agentResolver) loadRegistry() ([]registryEntry, error) {
	path := filepath.Join(r.repoDir, "pi-agents", "registry.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read registry %s: %w", path, err)
	}
	var reg registryFile
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry %s: %w", path, err)
	}
	return reg.Strategies, nil
}

func (r *agentResolver) resolve(key, model, thinkingLevel string) (match.AgentSpec, error) {
	entries, err := r.loadRegistry()
	if err != nil {
		return match.AgentSpec{}, err
	}

	for _, e := range entries {
		if e.Key != key {
			continue
		}
		switch e.Type {
		case "pi-agent":
			return r.resolvePiAgent(key, model, thinkingLevel)
		case "go-agent":
			return r.resolveGoAgent(key, e.Pkg, platformBinaryName(filepath.Base(e.Pkg)))
		default:
			return match.AgentSpec{}, fmt.Errorf("unknown strategy type %q for key %q", e.Type, key)
		}
	}

	keys := make([]string, len(entries))
	for i, e := range entries {
		keys[i] = e.Key
	}
	return match.AgentSpec{}, fmt.Errorf("unknown strategy key %q (known: %s)\nhint: run 'poker strategy new %s' to scaffold it", key, strings.Join(keys, ", "), key)
}

func (r *agentResolver) resolvePiAgent(key, model, thinkingLevel string) (match.AgentSpec, error) {
	nodePath, err := r.lookPath("node")
	if err != nil {
		return match.AgentSpec{}, fmt.Errorf("find node: %w", err)
	}
	nodePath, err = filepath.Abs(nodePath)
	if err != nil {
		return match.AgentSpec{}, fmt.Errorf("absolute node path: %w", err)
	}

	scriptPath := filepath.Join(r.repoDir, "pi-agents", key, "dist", "main.js")
	if _, err := os.Stat(scriptPath); err != nil {
		return match.AgentSpec{}, fmt.Errorf("stat %s: %w\nhint: build with: cd %s && npm run build", scriptPath, err, filepath.Join(r.repoDir, "pi-agents"))
	}

	env := []string{
		"PI_POKER_FAKE_DECISIONS_JSON=",
		fmt.Sprintf("PI_POKER_THINKING_LEVEL=%s", thinkingLevel),
	}
	if model != "" {
		env = append(env, fmt.Sprintf("PI_POKER_MODEL=%s", model))
	}

	return match.AgentSpec{
		Name:    key,
		Command: nodePath,
		Args:    []string{scriptPath},
		Env:     env,
	}, nil
}

func (r *agentResolver) resolveGoAgent(name, pkg, binary string) (match.AgentSpec, error) {
	outputPath := filepath.Join(r.repoDir, ".tmp", "bin", binary)
	if err := buildGoAgentBinary(r.repoDir, pkg, outputPath); err != nil {
		return match.AgentSpec{}, err
	}
	return match.AgentSpec{Name: name, Command: outputPath}, nil
}

func buildGoAgentBinary(repoDir, pkg, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(outputPath), err)
	}
	cmd := exec.Command("go", "build", "-o", outputPath, pkg)
	cmd.Dir = repoDir

	var outBuf, errBuf []byte
	outWriter := &byteWriter{b: &outBuf}
	errWriter := &byteWriter{b: &errBuf}
	cmd.Stdout = outWriter
	cmd.Stderr = errWriter

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build %s: %w\n%s%s", pkg, err, string(outBuf), string(errBuf))
	}
	return nil
}

type byteWriter struct{ b *[]byte }

func (w *byteWriter) Write(p []byte) (int, error) {
	*w.b = append(*w.b, p...)
	return len(p), nil
}

func platformBinaryName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func defaultMatchSessionID() string {
	return "ses_" + strings.ReplaceAll(time.Now().UTC().Format("2006-01-02T15-04-05Z"), ":", "-")
}
