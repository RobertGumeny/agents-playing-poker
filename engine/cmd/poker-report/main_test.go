package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

func TestRunWritesBenchmarkMarkdown(t *testing.T) {
	root := t.TempDir()
	a := writeCLISession(t, root, "seed3-a", 3, "llm-akg", "llm-stateless", []sessionlog.HandRecord{
		cliHand(1, false, []sessionlog.HandAction{{Seat: 0, Action: "raise", Amount: intPtr(6), Street: "preflop"}, {Seat: 1, Action: "fold", Street: "preflop"}}, 6, -6),
	})
	b := writeCLISession(t, root, "seed3-b", 3, "llm-stateless", "llm-akg-recent", []sessionlog.HandRecord{
		cliHand(1, false, []sessionlog.HandAction{{Seat: 1, Action: "auto_fold", Street: "preflop", ForcedReason: "timeout"}}, 4, -4),
	})
	out := filepath.Join(root, "reports", "benchmark.md")

	if err := run([]string{"-sessions", a + "," + b, "-label", "akg-recent-vs-stateless", "-out", out}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"# Benchmark Review: akg-recent-vs-stateless",
		"## Mirror-corrected aggregate",
		"llm-akg-recent",
		"historical_strategy_canonicalized",
		"Usage/cost data is missing",
		"fallback_actions_present",
		"| 3:llm-akg-recent+llm-stateless | seed3-a, seed3-b |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q\n%s", want, got)
		}
	}
}

func TestRunFailsForMissingRequiredFlags(t *testing.T) {
	if err := run([]string{"-out", filepath.Join(t.TempDir(), "report.md")}); err == nil || !strings.Contains(err.Error(), "-sessions is required") {
		t.Fatalf("run() error = %v, want -sessions required", err)
	}
	if err := run([]string{"-sessions", t.TempDir()}); err == nil || !strings.Contains(err.Error(), "-out is required") {
		t.Fatalf("run() error = %v, want -out required", err)
	}
}

func TestRunFailsForMissingOrMalformedArtifacts(t *testing.T) {
	missing := t.TempDir()
	if err := run([]string{"-sessions", missing, "-out", filepath.Join(t.TempDir(), "report.md")}); err == nil || !strings.Contains(err.Error(), "read manifest") {
		t.Fatalf("run() error = %v, want read manifest failure", err)
	}

	malformed := t.TempDir()
	if err := os.WriteFile(filepath.Join(malformed, "manifest.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(malformed, "hands.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-sessions", malformed, "-out", filepath.Join(t.TempDir(), "report.md")}); err == nil || !strings.Contains(err.Error(), "read manifest") {
		t.Fatalf("run() error = %v, want malformed manifest failure", err)
	}
}

func TestParseSessionDirsRejectsEmptyEntries(t *testing.T) {
	if _, err := parseSessionDirs("a,,b"); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("parseSessionDirs() error = %v, want empty entry failure", err)
	}
}

func writeCLISession(t *testing.T, root, sessionID string, seed int64, seat0, seat1 string, hands []sessionlog.HandRecord) string {
	t.Helper()
	dir := filepath.Join(root, sessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := sessionlog.Manifest{
		SessionID:     sessionID,
		StartedAt:     "2026-05-26T00:00:00Z",
		EndedAt:       "2026-05-26T00:01:00Z",
		Seed:          seed,
		HandCount:     len(hands),
		Variant:       "heads-up-nlhe",
		InfoRealism:   "showdown-only",
		StartingStack: 200,
		Blinds:        sessionlog.BlindLevel{SB: 1, BB: 2},
		Matches: []sessionlog.ManifestMatch{{
			MatchID: "match-1",
			Seats: []sessionlog.ManifestSeat{
				{Seat: 0, Name: seat0, Version: "test"},
				{Seat: 1, Name: seat1, Version: "test"},
			},
			Completed: true,
		}},
		ServerVersion:  "test",
		AKGSpecVersion: "test",
	}
	writeJSONFile(t, filepath.Join(dir, "manifest.json"), manifest)
	var lines strings.Builder
	for _, hand := range hands {
		data, err := json.Marshal(hand)
		if err != nil {
			t.Fatal(err)
		}
		lines.Write(data)
		lines.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "hands.jsonl"), []byte(lines.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func cliHand(number int, showdown bool, actions []sessionlog.HandAction, seat0Delta, seat1Delta int) sessionlog.HandRecord {
	return sessionlog.HandRecord{MatchID: "match-1", HandNumber: number, DealerSeat: number % 2, Actions: actions, ShowdownReached: showdown, Result: []sessionlog.HandResult{{Seat: 0, ChipsDelta: seat0Delta}, {Seat: 1, ChipsDelta: seat1Delta}}}
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func intPtr(v int) *int {
	return &v
}
