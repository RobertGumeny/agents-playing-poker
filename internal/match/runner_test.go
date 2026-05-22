package match

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RobertGumeny/agent-poker/internal/heuristicagent"
	"github.com/RobertGumeny/agent-poker/internal/randomagent"
	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
	"github.com/RobertGumeny/agent-poker/internal/wire"
)

func TestRunnerRunWritesSessionArtifactsAndCapturesStderr(t *testing.T) {
	t.Parallel()

	result, err := runTestMatch(t, "caller", "caller", 3, 50*time.Millisecond, 7)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Completed {
		t.Fatalf("RunResult.Completed = false, want true")
	}

	manifestPath := filepath.Join(result.SessionDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(manifest.json) error = %v", err)
	}
	var manifest sessionlog.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("Unmarshal(manifest.json) error = %v", err)
	}
	if len(manifest.Matches) != 1 || !manifest.Matches[0].Completed {
		t.Fatalf("manifest.Matches = %+v, want one completed match", manifest.Matches)
	}
	if manifest.Matches[0].Seats[0].Version == "" || manifest.Matches[0].Seats[1].Version == "" {
		t.Fatalf("manifest seat versions missing: %+v", manifest.Matches[0].Seats)
	}

	handsPath := filepath.Join(result.SessionDir, "hands.jsonl")
	lines := readLines(t, handsPath)
	if len(lines) != 3 {
		t.Fatalf("hands.jsonl line count = %d, want 3", len(lines))
	}

	stderrPath := filepath.Join(result.SessionDir, "agents", "seat-0", "stderr.log")
	stderrData, err := os.ReadFile(stderrPath)
	if err != nil {
		t.Fatalf("ReadFile(stderr.log) error = %v", err)
	}
	if !strings.Contains(string(stderrData), "helper agent caller stderr") {
		t.Fatalf("stderr.log = %q, want helper stderr marker", string(stderrData))
	}
}

func TestRunnerRunRecordsDecisionTimeoutAsAutoFold(t *testing.T) {
	t.Parallel()

	result, err := runTestMatch(t, "slow", "caller", 1, 25*time.Millisecond, 11)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Completed {
		t.Fatalf("RunResult.Completed = false, want true")
	}

	handsPath := filepath.Join(result.SessionDir, "hands.jsonl")
	lines := readLines(t, handsPath)
	if len(lines) != 1 {
		t.Fatalf("hands.jsonl line count = %d, want 1", len(lines))
	}

	var hand sessionlog.HandRecord
	if err := json.Unmarshal([]byte(lines[0]), &hand); err != nil {
		t.Fatalf("Unmarshal hand record error = %v", err)
	}
	found := false
	for _, action := range hand.Actions {
		if action.Action == "auto_fold" {
			found = true
			if action.ForcedReason != "decision_timeout" {
				t.Fatalf("auto_fold forced_reason = %q, want decision_timeout", action.ForcedReason)
			}
		}
	}
	if !found {
		t.Fatalf("hand actions = %+v, want auto_fold", hand.Actions)
	}
}

func TestRunnerRunProducesDeterministicHandsJSONL(t *testing.T) {
	t.Parallel()

	first, err := runTestMatch(t, "caller", "caller", 4, 50*time.Millisecond, 99)
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	second, err := runTestMatch(t, "caller", "caller", 4, 50*time.Millisecond, 99)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}

	firstHands, err := os.ReadFile(filepath.Join(first.SessionDir, "hands.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile(first hands.jsonl) error = %v", err)
	}
	secondHands, err := os.ReadFile(filepath.Join(second.SessionDir, "hands.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile(second hands.jsonl) error = %v", err)
	}
	if !bytes.Equal(firstHands, secondHands) {
		t.Fatalf("hands.jsonl mismatch\nfirst:\n%s\nsecond:\n%s", firstHands, secondHands)
	}
}

func TestRunnerRunWithRandomAgentsCompletes(t *testing.T) {
	t.Parallel()

	result, err := runTestMatch(t, "random", "random", 8, 50*time.Millisecond, 41)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Completed {
		t.Fatalf("RunResult.Completed = false, want true")
	}

	handsPath := filepath.Join(result.SessionDir, "hands.jsonl")
	lines := readLines(t, handsPath)
	if len(lines) != 8 {
		t.Fatalf("hands.jsonl line count = %d, want 8", len(lines))
	}
}

func TestRunnerRunWithHeuristicAgentCompletes(t *testing.T) {
	t.Parallel()

	result, err := runTestMatch(t, "heuristic", "random", 8, 50*time.Millisecond, 52)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Completed {
		t.Fatalf("RunResult.Completed = false, want true")
	}

	handsPath := filepath.Join(result.SessionDir, "hands.jsonl")
	lines := readLines(t, handsPath)
	if len(lines) != 8 {
		t.Fatalf("hands.jsonl line count = %d, want 8", len(lines))
	}
}

func TestRunnerRunMarksIncompleteMatchAndPersistsCompletedHandsWhenAgentDies(t *testing.T) {
	t.Parallel()

	result, err := runTestMatch(t, "die-on-hand-2", "caller", 3, 50*time.Millisecond, 23)
	if err == nil {
		t.Fatal("Run() error = nil, want agent failure")
	}
	if result.Completed {
		t.Fatalf("RunResult.Completed = true, want false")
	}

	manifestPath := filepath.Join(result.SessionDir, "manifest.json")
	manifestData, readErr := os.ReadFile(manifestPath)
	if readErr != nil {
		t.Fatalf("ReadFile(manifest.json) error = %v", readErr)
	}
	var manifest sessionlog.Manifest
	if unmarshalErr := json.Unmarshal(manifestData, &manifest); unmarshalErr != nil {
		t.Fatalf("Unmarshal(manifest.json) error = %v", unmarshalErr)
	}
	if len(manifest.Matches) != 1 || manifest.Matches[0].Completed {
		t.Fatalf("manifest.Matches = %+v, want one incomplete match", manifest.Matches)
	}

	handsPath := filepath.Join(result.SessionDir, "hands.jsonl")
	lines := readLines(t, handsPath)
	if len(lines) != 1 {
		t.Fatalf("hands.jsonl line count = %d, want 1 completed hand before failure", len(lines))
	}
}

func runTestMatch(t *testing.T, seat0Behavior string, seat1Behavior string, handCount int, deadline time.Duration, seed int64) (RunResult, error) {
	t.Helper()

	rootDir := t.TempDir()
	runner, err := NewRunner(Config{
		SessionID:        fmt.Sprintf("ses_test_%d", time.Now().UnixNano()),
		SessionsRootDir:  rootDir,
		MatchID:          "mat_test",
		Seed:             seed,
		HandCount:        handCount,
		StartingStack:    20,
		SmallBlind:       1,
		BigBlind:         2,
		DecisionDeadline: deadline,
		AgentSpecs: []AgentSpec{
			testAgentSpec(t, "seat-0", seat0Behavior),
			testAgentSpec(t, "seat-1", seat1Behavior),
		},
	})
	if err != nil {
		return RunResult{}, err
	}
	return runner.Run(context.Background())
}

func testAgentSpec(t *testing.T, name string, behavior string) AgentSpec {
	t.Helper()
	return AgentSpec{
		Name:    name,
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperAgentProcess", "--", behavior},
		Env:     []string{"GO_WANT_HELPER_PROCESS=1"},
	}
}

func TestHelperAgentProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	behavior := "caller"
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			behavior = os.Args[i+1]
			break
		}
	}
	fmt.Fprintf(os.Stderr, "helper agent %s stderr\n", behavior)
	if behavior == "random" {
		if err := randomagent.Run(os.Stdin, os.Stdout, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if behavior == "heuristic" {
		if err := heuristicagent.Run(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
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
			if behavior == "die-on-hand-2" && payload.HandNumber == 2 {
				fmt.Fprintln(os.Stderr, "helper agent exiting on hand 2")
				os.Exit(1)
			}
			_ = encoder.Encode(wire.NewMessage(wire.MessageTypeLog, fmt.Sprintf("helper-log-%d", responseID), "", wire.LogPayload{Level: "info", Message: "thinking"}))
			action := chooseAction(payload.LegalActions)
			_ = encoder.Encode(wire.NewMessage(wire.MessageTypeAction, fmt.Sprintf("helper-action-%d", responseID), envelope.ID, action))
		case wire.MessageTypeSessionEnd:
			return
		}
	}
	os.Exit(0)
}

func chooseAction(actions []wire.LegalActionOption) wire.ActionPayload {
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

func readLines(t *testing.T, path string) []string {
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
