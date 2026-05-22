package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
	"github.com/RobertGumeny/agent-poker/internal/wire"
)

func TestPokerServerRunsRandomVersusHeuristicDemo(t *testing.T) {
	t.Parallel()

	serverBin := buildCommandBinary(t, "./cmd/poker-server")
	randomBin := buildCommandBinary(t, "./cmd/random-agent")
	heuristicBin := buildCommandBinary(t, "./cmd/heuristic-agent")

	sessionsDir := filepath.Join(t.TempDir(), "sessions")
	sessionID := "ses_cli_demo"
	output := runPokerServerCommand(t, 5*time.Second, serverBin,
		"-sessions-dir", sessionsDir,
		"-session-id", sessionID,
		"-match-id", "mat_cli_demo",
		"-seed", "17",
		"-hand-count", "12",
		"-starting-stack", "40",
		"-agent0-name", "random",
		"-agent0-cmd", randomBin,
		"-agent1-name", "heuristic",
		"-agent1-cmd", heuristicBin,
	)
	if !strings.Contains(output, "completed=true") {
		t.Fatalf("poker-server output = %q, want completed=true", output)
	}

	sessionDir := filepath.Join(sessionsDir, sessionID)
	manifest := readManifest(t, filepath.Join(sessionDir, "manifest.json"))
	if len(manifest.Matches) != 1 || !manifest.Matches[0].Completed {
		t.Fatalf("manifest.Matches = %+v, want one completed match", manifest.Matches)
	}
	if got := len(readHands(t, filepath.Join(sessionDir, "hands.jsonl"))); got != 12 {
		t.Fatalf("hands.jsonl line count = %d, want 12", got)
	}
	assertFileExists(t, filepath.Join(sessionDir, "agents", "random", "stdout.log"))
	assertFileExists(t, filepath.Join(sessionDir, "agents", "random", "stderr.log"))
	assertFileExists(t, filepath.Join(sessionDir, "agents", "heuristic", "stdout.log"))
	assertFileExists(t, filepath.Join(sessionDir, "agents", "heuristic", "stderr.log"))
}

func TestPokerServerRecordsTimeoutAutoFoldAndExitsCleanly(t *testing.T) {
	t.Parallel()

	serverBin := buildCommandBinary(t, "./cmd/poker-server")
	sessionsDir := filepath.Join(t.TempDir(), "sessions")
	sessionID := "ses_cli_timeout"
	helperBin := os.Args[0]
	output := runPokerServerCommand(t, 5*time.Second, serverBin,
		"-sessions-dir", sessionsDir,
		"-session-id", sessionID,
		"-match-id", "mat_cli_timeout",
		"-seed", "29",
		"-hand-count", "1",
		"-decision-deadline", "25ms",
		"-agent0-name", "slow",
		"-agent0-cmd", helperBin,
		"-agent0-arg", "-test.run=TestPokerServerHelperAgentProcess",
		"-agent0-arg", "--",
		"-agent0-arg", "slow",
		"-agent1-name", "caller",
		"-agent1-cmd", helperBin,
		"-agent1-arg", "-test.run=TestPokerServerHelperAgentProcess",
		"-agent1-arg", "--",
		"-agent1-arg", "caller",
	)
	if !strings.Contains(output, "completed=true") {
		t.Fatalf("poker-server output = %q, want completed=true", output)
	}

	sessionDir := filepath.Join(sessionsDir, sessionID)
	manifest := readManifest(t, filepath.Join(sessionDir, "manifest.json"))
	if len(manifest.Matches) != 1 || !manifest.Matches[0].Completed {
		t.Fatalf("manifest.Matches = %+v, want one completed match", manifest.Matches)
	}

	hands := readHands(t, filepath.Join(sessionDir, "hands.jsonl"))
	if len(hands) != 1 {
		t.Fatalf("hands.jsonl line count = %d, want 1", len(hands))
	}
	var hand sessionlog.HandRecord
	if err := json.Unmarshal([]byte(hands[0]), &hand); err != nil {
		t.Fatalf("Unmarshal hand record error = %v", err)
	}
	found := false
	for _, action := range hand.Actions {
		if action.Action != "auto_fold" {
			continue
		}
		found = true
		if action.ForcedReason != "decision_timeout" {
			t.Fatalf("auto_fold forced_reason = %q, want decision_timeout", action.ForcedReason)
		}
	}
	if !found {
		t.Fatalf("hand actions = %+v, want auto_fold", hand.Actions)
	}
}

func TestPokerServerHelperAgentProcess(t *testing.T) {
	behavior := helperBehavior(os.Args)
	if behavior == "" {
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	responseID := 0
	for scanner.Scan() {
		envelope, err := wire.DecodeEnvelope(scanner.Bytes())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		responseID++
		switch envelope.Type {
		case wire.MessageTypeSessionInit:
			_ = encoder.Encode(wire.NewMessage(wire.MessageTypeSessionReady, fmt.Sprintf("helper-%d", responseID), envelope.ID, wire.SessionReadyPayload{Version: "helper/0.1.0"}))
		case wire.MessageTypeYourTurn:
			if behavior == "slow" {
				time.Sleep(200 * time.Millisecond)
			}
			var payload wire.YourTurnPayload
			if err := envelope.DecodePayload(&payload); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			_ = encoder.Encode(wire.NewMessage(wire.MessageTypeAction, fmt.Sprintf("helper-action-%d", responseID), envelope.ID, chooseHelperAction(payload.LegalActions)))
		case wire.MessageTypeSessionEnd:
			return
		}
	}
	os.Exit(0)
}

func buildCommandBinary(t *testing.T, pkg string) string {
	t.Helper()

	binaryPath := filepath.Join(t.TempDir(), filepath.Base(pkg))
	cmd := exec.Command("go", "build", "-o", binaryPath, pkg)
	cmd.Dir = repoRoot(t)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build %s error = %v\n%s", pkg, err, output)
	}
	return binaryPath
}

func runPokerServerCommand(t *testing.T, timeout time.Duration, binary string, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("%s timed out after %s\n%s", binary, timeout, output)
	}
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

func chooseHelperAction(actions []wire.LegalActionOption) wire.ActionPayload {
	for _, action := range actions {
		if action.Action == "check" {
			return wire.ActionPayload{Action: "check"}
		}
	}
	for _, action := range actions {
		if action.Action == "call" {
			return wire.ActionPayload{Action: "call", Amount: action.Amount}
		}
	}
	for _, action := range actions {
		if action.Action == "fold" {
			return wire.ActionPayload{Action: "fold"}
		}
	}
	panic("no supported action")
}

func helperBehavior(args []string) string {
	for i, arg := range args {
		if arg == "--" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
