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
	if !strings.Contains(output, filepath.Join(sessionsDir, sessionID)) {
		t.Fatalf("poker-demo output = %q, want session dir", output)
	}

	manifest := readManifest(t, filepath.Join(sessionsDir, sessionID, "manifest.json"))
	if len(manifest.Matches) != 1 || !manifest.Matches[0].Completed {
		t.Fatalf("manifest.Matches = %+v, want one completed match", manifest.Matches)
	}
	if got := len(readHands(t, filepath.Join(sessionsDir, sessionID, "hands.jsonl"))); got != 8 {
		t.Fatalf("hands.jsonl line count = %d, want 8", got)
	}
	assertFileExists(t, filepath.Join(sessionsDir, sessionID, "agents", "random", "stdout.log"))
	assertFileExists(t, filepath.Join(sessionsDir, sessionID, "agents", "heuristic", "stdout.log"))
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
