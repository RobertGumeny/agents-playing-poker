package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

func TestAgentAliasResolverResolvesLLMStatelessWithAbsolutePathsAndEnv(t *testing.T) {
	t.Parallel()

	repoDir := repoRootForTest(t)
	resolver := agentAliasResolver{
		repoDir:  repoDir,
		goBinary: "go",
		lookPath: func(string) (string, error) { return "/usr/bin/node", nil },
		buildBinary: func(repoDir, goBinary, pkg, outputPath string) error {
			t.Fatalf("buildBinary called unexpectedly for llm-stateless")
			return nil
		},
	}

	spec, err := resolver.resolve(llmStatelessAlias, "anthropic:claude-sonnet-4", "low")
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}
	if spec.Name != llmStatelessAlias {
		t.Fatalf("spec.Name = %q, want %q", spec.Name, llmStatelessAlias)
	}
	if spec.Command != "/usr/bin/node" {
		t.Fatalf("spec.Command = %q, want /usr/bin/node", spec.Command)
	}
	if len(spec.Args) != 1 {
		t.Fatalf("len(spec.Args) = %d, want 1", len(spec.Args))
	}
	if !filepath.IsAbs(spec.Args[0]) {
		t.Fatalf("spec.Args[0] = %q, want absolute path", spec.Args[0])
	}
	if !strings.HasSuffix(spec.Args[0], filepath.Join("pi-agents", "llm-stateless", "dist", "main.js")) {
		t.Fatalf("spec.Args[0] = %q, want llm-stateless dist entrypoint", spec.Args[0])
	}
	wantEnv := []string{
		"PI_POKER_FAKE_DECISIONS_JSON=",
		"PI_POKER_THINKING_LEVEL=low",
		"PI_POKER_MODEL=anthropic:claude-sonnet-4",
	}
	if strings.Join(spec.Env, "\n") != strings.Join(wantEnv, "\n") {
		t.Fatalf("spec.Env = %q, want %q", spec.Env, wantEnv)
	}
}

func TestAgentAliasResolverResolvesLLMFullhistoryWithAbsolutePathsAndEnv(t *testing.T) {
	t.Parallel()

	repoDir := repoRootForTest(t)
	resolver := agentAliasResolver{
		repoDir:  repoDir,
		goBinary: "go",
		lookPath: func(string) (string, error) { return "/usr/bin/node", nil },
		buildBinary: func(repoDir, goBinary, pkg, outputPath string) error {
			t.Fatalf("buildBinary called unexpectedly for llm-fullhistory")
			return nil
		},
	}

	spec, err := resolver.resolve(llmFullhistoryAlias, "anthropic:claude-sonnet-4", "low")
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}
	if spec.Name != llmFullhistoryAlias {
		t.Fatalf("spec.Name = %q, want %q", spec.Name, llmFullhistoryAlias)
	}
	if spec.Command != "/usr/bin/node" {
		t.Fatalf("spec.Command = %q, want /usr/bin/node", spec.Command)
	}
	if len(spec.Args) != 1 {
		t.Fatalf("len(spec.Args) = %d, want 1", len(spec.Args))
	}
	if !filepath.IsAbs(spec.Args[0]) {
		t.Fatalf("spec.Args[0] = %q, want absolute path", spec.Args[0])
	}
	if !strings.HasSuffix(spec.Args[0], filepath.Join("pi-agents", "llm-fullhistory", "dist", "main.js")) {
		t.Fatalf("spec.Args[0] = %q, want llm-fullhistory dist entrypoint", spec.Args[0])
	}
	wantEnv := []string{
		"PI_POKER_FAKE_DECISIONS_JSON=",
		"PI_POKER_THINKING_LEVEL=low",
		"PI_POKER_MODEL=anthropic:claude-sonnet-4",
	}
	if strings.Join(spec.Env, "\n") != strings.Join(wantEnv, "\n") {
		t.Fatalf("spec.Env = %q, want %q", spec.Env, wantEnv)
	}
}

func TestAgentAliasResolverResolvesLLMAkgRecentWithAbsolutePathsAndEnv(t *testing.T) {
	t.Parallel()

	repoDir := repoRootForTest(t)
	resolver := agentAliasResolver{
		repoDir:  repoDir,
		goBinary: "go",
		lookPath: func(string) (string, error) { return "/usr/bin/node", nil },
		buildBinary: func(repoDir, goBinary, pkg, outputPath string) error {
			t.Fatalf("buildBinary called unexpectedly for llm-akg-recent")
			return nil
		},
	}

	spec, err := resolver.resolve(llmAkgAlias, "anthropic:claude-sonnet-4", "medium")
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}
	if spec.Name != llmAkgAlias {
		t.Fatalf("spec.Name = %q, want %q", spec.Name, llmAkgAlias)
	}
	if spec.Command != "/usr/bin/node" {
		t.Fatalf("spec.Command = %q, want /usr/bin/node", spec.Command)
	}
	if len(spec.Args) != 1 {
		t.Fatalf("len(spec.Args) = %d, want 1", len(spec.Args))
	}
	if !filepath.IsAbs(spec.Args[0]) {
		t.Fatalf("spec.Args[0] = %q, want absolute path", spec.Args[0])
	}
	if !strings.HasSuffix(spec.Args[0], filepath.Join("pi-agents", "llm-akg-recent", "dist", "main.js")) {
		t.Fatalf("spec.Args[0] = %q, want llm-akg-recent dist entrypoint", spec.Args[0])
	}
	wantEnv := []string{
		"PI_POKER_FAKE_DECISIONS_JSON=",
		"PI_POKER_THINKING_LEVEL=medium",
		"PI_POKER_MODEL=anthropic:claude-sonnet-4",
	}
	if strings.Join(spec.Env, "\n") != strings.Join(wantEnv, "\n") {
		t.Fatalf("spec.Env = %q, want %q", spec.Env, wantEnv)
	}
}

func TestAgentAliasResolverBuildsAbsoluteGoAgentBinary(t *testing.T) {
	t.Parallel()

	repoDir := repoRootForTest(t)
	var builtRepoDir string
	var builtGoBinary string
	var builtPkg string
	var builtOutput string
	resolver := agentAliasResolver{
		repoDir:  repoDir,
		goBinary: "go1.25",
		lookPath: func(string) (string, error) {
			t.Fatalf("lookPath called unexpectedly for go agent")
			return "", nil
		},
		buildBinary: func(repoDir, goBinary, pkg, outputPath string) error {
			builtRepoDir = repoDir
			builtGoBinary = goBinary
			builtPkg = pkg
			builtOutput = outputPath
			return nil
		},
	}

	spec, err := resolver.resolve(heuristicAlias, "", defaultThinkingLevel)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}
	if spec.Name != heuristicAlias {
		t.Fatalf("spec.Name = %q, want %q", spec.Name, heuristicAlias)
	}
	if !filepath.IsAbs(spec.Command) {
		t.Fatalf("spec.Command = %q, want absolute path", spec.Command)
	}
	if spec.Command != builtOutput {
		t.Fatalf("spec.Command = %q, want built output %q", spec.Command, builtOutput)
	}
	if builtRepoDir != repoDir {
		t.Fatalf("build repoDir = %q, want %q", builtRepoDir, repoDir)
	}
	if builtGoBinary != "go1.25" {
		t.Fatalf("build goBinary = %q, want go1.25", builtGoBinary)
	}
	if builtPkg != "./cmd/heuristic-agent" {
		t.Fatalf("build pkg = %q, want ./cmd/heuristic-agent", builtPkg)
	}
	if builtOutput != filepath.Join(repoDir, ".tmp", "bin", binaryName("heuristic-agent")) {
		t.Fatalf("build output = %q", builtOutput)
	}
}

func TestPokerRunRunsRandomVersusHeuristic(t *testing.T) {
	runBin := buildBinary(t, "./cmd/poker-run")
	sessionsDir := filepath.Join(t.TempDir(), "sessions")
	sessionID := "ses_poker_run"
	output := runCommand(t, runBin,
		"-sessions-dir", sessionsDir,
		"-session-id", sessionID,
		"-seed", "17",
		"-hands", "8",
		"-agent0", "random",
		"-agent1", "heuristic",
	)
	if !strings.Contains(output, "completed=true") {
		t.Fatalf("poker-run output = %q, want completed=true", output)
	}

	sessionDir := filepath.Join(sessionsDir, sessionID)
	manifest := readManifest(t, filepath.Join(sessionDir, "manifest.json"))
	if len(manifest.Matches) != 1 || !manifest.Matches[0].Completed {
		t.Fatalf("manifest.Matches = %+v, want one completed match", manifest.Matches)
	}
	if got := len(readHands(t, filepath.Join(sessionDir, "hands.jsonl"))); got != 8 {
		t.Fatalf("hands.jsonl line count = %d, want 8", got)
	}
	assertFileExists(t, filepath.Join(sessionDir, "agents", "random", "stdout.log"))
	assertFileExists(t, filepath.Join(sessionDir, "agents", "heuristic", "stdout.log"))
}

func buildBinary(t *testing.T, pkg string) string {
	t.Helper()

	binaryPath := filepath.Join(t.TempDir(), filepath.Base(pkg))
	cmd := exec.Command("go", "build", "-o", binaryPath, pkg)
	cmd.Dir = repoRootForTest(t)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build %s error = %v\n%s", pkg, err, output)
	}
	return binaryPath
}

func runCommand(t *testing.T, binary string, args ...string) string {
	t.Helper()

	cmd := exec.Command(binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v error = %v\n%s", binary, args, err, output)
	}
	return string(output)
}

func readManifest(t *testing.T, path string) sessionlog.Manifest {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	var manifest sessionlog.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v", path, err)
	}
	return manifest
}

func readHands(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
