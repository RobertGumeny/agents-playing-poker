package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

func TestPokerDemoRunsDefaultScriptedMatch(t *testing.T) {
	t.Parallel()

	demoBin := buildBinary(t, "./cmd/poker-demo")
	sessionsDir := filepath.Join(t.TempDir(), "sessions")
	sessionID := "ses_scripted_demo"
	output := runCommand(t, demoBin,
		"-sessions-dir", sessionsDir,
		"-session-id", sessionID,
		"-match-id", "mat_scripted_demo",
		"-seed", "17",
		"-hand-count", "8",
		"-starting-stack", "40",
	)
	if !strings.Contains(output, "demo=random-vs-heuristic") {
		t.Fatalf("poker-demo output = %q, want demo banner", output)
	}
	sessionDir := filepath.Join(sessionsDir, sessionID)
	if !strings.Contains(output, sessionDir) {
		t.Fatalf("poker-demo output = %q, want session dir", output)
	}
	if !strings.Contains(output, filepath.Join(sessionDir, "manifest.json")) {
		t.Fatalf("poker-demo output = %q, want manifest path", output)
	}
	if !strings.Contains(output, filepath.Join(sessionDir, "hands.jsonl")) {
		t.Fatalf("poker-demo output = %q, want hands path", output)
	}
	if !strings.Contains(output, filepath.Join(sessionDir, "agents")) {
		t.Fatalf("poker-demo output = %q, want agent logs path", output)
	}

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

func TestDemoConfigServerArgs(t *testing.T) {
	t.Parallel()

	cfg := demoConfig{
		sessionID:        "ses_test",
		sessionsDir:      "custom-sessions",
		matchID:          "mat_test",
		seed:             99,
		handCount:        12,
		startingStack:    150,
		smallBlind:       2,
		bigBlind:         4,
		decisionDeadline: 1500 * time.Millisecond,
		goBinary:         "go1.24",
	}
	binaries := demoBinaries{
		server:    "/tmp/poker-server",
		random:    "/tmp/random-agent",
		heuristic: "/tmp/heuristic-agent",
	}

	got := cfg.serverArgs(binaries)
	want := []string{
		"-sessions-dir", "custom-sessions",
		"-session-id", "ses_test",
		"-match-id", "mat_test",
		"-seed", "99",
		"-hand-count", "12",
		"-starting-stack", "150",
		"-small-blind", "2",
		"-big-blind", "4",
		"-decision-deadline", "1.5s",
		"-agent0-name", "random",
		"-agent0-cmd", "/tmp/random-agent",
		"-agent1-name", "heuristic",
		"-agent1-cmd", "/tmp/heuristic-agent",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("serverArgs() = %q, want %q", got, want)
	}
}

func TestDemoConfigInspectPaths(t *testing.T) {
	t.Parallel()

	repoDir := "/repo"
	relative := demoConfig{sessionID: "ses_rel", sessionsDir: "sessions"}.inspectPaths(repoDir)
	if relative.sessionDir != filepath.Join(repoDir, "sessions", "ses_rel") {
		t.Fatalf("relative sessionDir = %q", relative.sessionDir)
	}
	if relative.manifest != filepath.Join(relative.sessionDir, "manifest.json") {
		t.Fatalf("relative manifest = %q", relative.manifest)
	}
	if relative.hands != filepath.Join(relative.sessionDir, "hands.jsonl") {
		t.Fatalf("relative hands = %q", relative.hands)
	}
	if relative.agentLogs != filepath.Join(relative.sessionDir, "agents") {
		t.Fatalf("relative agentLogs = %q", relative.agentLogs)
	}

	absoluteRoot := filepath.Join(string(filepath.Separator), "tmp", "sessions")
	absolute := demoConfig{sessionID: "ses_abs", sessionsDir: absoluteRoot}.inspectPaths(repoDir)
	if absolute.sessionDir != filepath.Join(absoluteRoot, "ses_abs") {
		t.Fatalf("absolute sessionDir = %q", absolute.sessionDir)
	}
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
