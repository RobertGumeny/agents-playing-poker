package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

func TestCollectSessionFromFixtureArtifacts(t *testing.T) {
	rootDir := t.TempDir()
	sessionDir := createCollectFixture(t, rootDir)

	summary, err := CollectSession(sessionDir)
	if err != nil {
		t.Fatalf("CollectSession() error = %v", err)
	}

	if summary.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", summary.SchemaVersion)
	}
	if summary.SessionID != "fixture" || summary.MatchID != "mat_001" {
		t.Fatalf("summary ids = (%q, %q), want fixture/mat_001", summary.SessionID, summary.MatchID)
	}
	if summary.SourceArtifacts.Manifest != "manifest.json" || summary.SourceArtifacts.Hands != "hands.jsonl" {
		t.Fatalf("source artifacts = %#v, want manifest/hands relative paths", summary.SourceArtifacts)
	}
	if got := summary.SourceArtifacts.Agents["memory-agent"]; got.PiSession == nil || *got.PiSession != "agents/memory-agent/pi-session.jsonl" || got.MemoryExport == nil || *got.MemoryExport != "agents/memory-agent/memory-export.json" || got.Stderr == nil || *got.Stderr != "agents/memory-agent/stderr.log" {
		t.Fatalf("memory-agent source artifacts = %#v, want relative pi/memory/stderr paths", got)
	}
	if got := summary.SourceArtifacts.Agents["plain-agent"]; got.PiSession != nil || got.MemoryExport != nil || got.Stderr != nil {
		t.Fatalf("plain-agent source artifacts = %#v, want nil optional paths", got)
	}

	if summary.Session.DurationS != 10 || summary.Session.HandCount != 2 || !summary.Session.Completed {
		t.Fatalf("session summary = %#v, want duration=10 hand_count=2 completed=true", summary.Session)
	}
	if summary.Metrics.PreflopOnlyHands != 1 || summary.Metrics.ShowdownHands != 1 || summary.Metrics.FallbackActionCount != 1 {
		t.Fatalf("session metrics = %#v, want preflop_only=1 showdown=1 fallback=1", summary.Metrics)
	}
	if summary.Metrics.PreflopOnlyRate != 0.5 || summary.Metrics.ShowdownRate != 0.5 {
		t.Fatalf("rates = (%v, %v), want 0.5 each", summary.Metrics.PreflopOnlyRate, summary.Metrics.ShowdownRate)
	}
	if summary.Metrics.BiggestSwingHand != (BiggestSwingHand{HandNumber: 2, Chips: 10}) {
		t.Fatalf("biggest swing = %#v, want hand 2 chips 10", summary.Metrics.BiggestSwingHand)
	}

	if len(summary.Seats) != 2 {
		t.Fatalf("len(seats) = %d, want 2", len(summary.Seats))
	}
	memorySeat := summary.Seats[0]
	if memorySeat.Name != "memory-agent" || memorySeat.ChipsDelta != 8 {
		t.Fatalf("seat[0] = %#v, want memory-agent chips_delta=8", memorySeat)
	}
	if !memorySeat.PiSessionPresent || memorySeat.DecisionPromptCount != 1 {
		t.Fatalf("memory seat prompt data = %#v, want pi session present with 1 prompt", memorySeat)
	}
	if memorySeat.ToolCalls["akg_get_opponent"] != 1 || memorySeat.ToolCallsPerHand["akg_get_opponent"] != 0.5 {
		t.Fatalf("memory seat tool calls = %#v / %#v, want opponent=1 and 0.5 per hand", memorySeat.ToolCalls, memorySeat.ToolCallsPerHand)
	}
	if memorySeat.RetryMetrics != (RetryMetrics{AttemptFailures: 2, MalformedActionRetries: 1, ExhaustedCount: 1, MaxAttemptsObserved: 2}) {
		t.Fatalf("memory seat retry metrics = %#v, want attempts=2 malformed=1 exhausted=1 max=2", memorySeat.RetryMetrics)
	}
	if memorySeat.MemoryExport == nil || memorySeat.MemoryExport.NodeCount != 2 || memorySeat.MemoryExport.EdgeCount != 1 || memorySeat.MemoryExport.NodesByType["opponent"] != 1 || memorySeat.MemoryExport.EdgesByRelation["supported_by"] != 1 {
		t.Fatalf("memory export summary = %#v, want summarized graph counts", memorySeat.MemoryExport)
	}

	plainSeat := summary.Seats[1]
	if plainSeat.Name != "plain-agent" || plainSeat.ChipsDelta != -8 {
		t.Fatalf("seat[1] = %#v, want plain-agent chips_delta=-8", plainSeat)
	}
	if plainSeat.PiSessionPresent || plainSeat.DecisionPromptCount != 0 || plainSeat.MemoryExport != nil {
		t.Fatalf("plain seat = %#v, want no optional artifacts", plainSeat)
	}
	if plainSeat.RetryMetrics != (RetryMetrics{}) {
		t.Fatalf("plain seat retry metrics = %#v, want zero value", plainSeat.RetryMetrics)
	}
}

func TestCollectSessionRealFixtureShape(t *testing.T) {
	summary, err := CollectSession(filepath.Join("..", "..", "..", "research", "sessions", "akg-durable-prompt-test-1"))
	if err != nil {
		t.Fatalf("CollectSession() error = %v", err)
	}
	if summary.SessionID != "akg-durable-prompt-test-1" || summary.MatchID != "mat_001" {
		t.Fatalf("summary ids = (%q, %q), want akg-durable-prompt-test-1/mat_001", summary.SessionID, summary.MatchID)
	}
	if summary.Metrics.PreflopOnlyHands != 17 || summary.Metrics.ShowdownHands != 3 || summary.Metrics.FallbackActionCount != 3 {
		t.Fatalf("metrics = %#v, want preflop_only=17 showdown=3 fallback=3", summary.Metrics)
	}
	if len(summary.Seats) != 2 {
		t.Fatalf("len(seats) = %d, want 2", len(summary.Seats))
	}
	if summary.Seats[0].DecisionPromptCount != 113 || summary.Seats[0].ToolCalls["akg_get_opponent"] != 76 {
		t.Fatalf("seat[0] = %#v, want prompt count 113 and opponent calls 76", summary.Seats[0])
	}
	if summary.Seats[0].RetryMetrics.AttemptFailures == 0 || summary.Seats[0].RetryMetrics.ExhaustedCount == 0 {
		t.Fatalf("seat[0] retry metrics = %#v, want non-zero retries from fixture stderr", summary.Seats[0].RetryMetrics)
	}
}

func TestWriteSummaryWritesDeterministicEvalJSON(t *testing.T) {
	rootDir := t.TempDir()
	sessionDir := createCollectFixture(t, rootDir)
	summary, err := CollectSession(sessionDir)
	if err != nil {
		t.Fatalf("CollectSession() error = %v", err)
	}
	if err := WriteSummary(sessionDir, summary); err != nil {
		t.Fatalf("WriteSummary() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(sessionDir, "eval.json"))
	if err != nil {
		t.Fatalf("ReadFile(eval.json) error = %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("eval.json missing trailing newline: %q", string(data))
	}
}

func createCollectFixture(t *testing.T, rootDir string) string {
	t.Helper()

	writer, err := sessionlog.New(rootDir, "fixture")
	if err != nil {
		t.Fatalf("sessionlog.New() error = %v", err)
	}
	defer func() {
		if err := writer.Close(); err != nil {
			t.Fatalf("writer.Close() error = %v", err)
		}
	}()

	if err := writer.AppendHand(sessionlog.HandRecord{
		MatchID:    "mat_001",
		HandNumber: 1,
		Actions: []sessionlog.HandAction{
			{Seat: 0, Action: "post_blind", Street: "preflop"},
			{Seat: 1, Action: "post_blind", Street: "preflop"},
			{Seat: 0, Action: "raise", Street: "preflop", Amount: intPtr(6)},
			{Seat: 1, Action: "fold", Street: "preflop"},
		},
		Result: []sessionlog.HandResult{{Seat: 0, ChipsDelta: 2}, {Seat: 1, ChipsDelta: -2}},
	}); err != nil {
		t.Fatalf("writer.AppendHand(hand1) error = %v", err)
	}
	if err := writer.AppendHand(sessionlog.HandRecord{
		MatchID:         "mat_001",
		HandNumber:      2,
		ShowdownReached: true,
		Actions: []sessionlog.HandAction{
			{Seat: 0, Action: "post_blind", Street: "preflop"},
			{Seat: 1, Action: "post_blind", Street: "preflop"},
			{Seat: 0, Action: "call", Street: "preflop", Amount: intPtr(1)},
			{Seat: 1, Action: "check", Street: "preflop"},
			{Seat: 1, Action: "bet", Street: "flop", Amount: intPtr(3)},
			{Seat: 0, Action: "auto_fold", Street: "flop", ForcedReason: "decision_timeout"},
		},
		Result: []sessionlog.HandResult{{Seat: 0, ChipsDelta: -10}, {Seat: 1, ChipsDelta: 10}},
	}); err != nil {
		t.Fatalf("writer.AppendHand(hand2) error = %v", err)
	}

	if err := writer.WriteManifest(sessionlog.Manifest{
		SessionID:      "fixture",
		StartedAt:      "2026-05-27T10:00:00Z",
		EndedAt:        "2026-05-27T10:00:10Z",
		Seed:           7,
		HandCount:      2,
		Variant:        "heads-up-nlhe",
		InfoRealism:    "showdown-only",
		StartingStack:  200,
		Blinds:         sessionlog.BlindLevel{SB: 1, BB: 2},
		ServerVersion:  "dev",
		AKGSpecVersion: "v1-draft-2",
		Matches: []sessionlog.ManifestMatch{{
			MatchID: "mat_001",
			Seats:   []sessionlog.ManifestSeat{{Seat: 0, Name: "memory-agent", Version: "memory-agent/1.0.0"}, {Seat: 1, Name: "plain-agent", Version: "plain-agent/1.0.0"}},
			Result: map[int]sessionlog.ManifestSeatResult{
				0: {ChipsDelta: 8},
				1: {ChipsDelta: -8},
			},
			Completed: true,
		}},
	}); err != nil {
		t.Fatalf("writer.WriteManifest() error = %v", err)
	}

	memoryDir, err := writer.AgentDir("memory-agent")
	if err != nil {
		t.Fatalf("writer.AgentDir(memory-agent) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, piSessionFileName), []byte("{\"type\":\"fake_pi_session\",\"session_scope\":\"decision\",\"session_number\":1,\"hand_number\":1,\"decision_number\":1,\"prompt\":\"Hand: 1\"}\n{\"type\":\"message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"toolCall\",\"name\":\"akg_get_opponent\"}]}}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(pi-session.jsonl) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, memoryExportFileName), []byte("{\"nodes\":[{\"type\":\"opponent\",\"id\":\"villain\"},{\"type\":\"hand\",\"id\":\"hand-1\"}],\"edges\":[{\"relation\":\"supported_by\"}]}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(memory-export.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, stderrFileName), []byte("decision attempt 1/2 failed: pi decision failed: assistant returned malformed action JSON: \"bad\"\ndecision attempt 2/2 failed: unknown Pi model \"foo\"\ndecision engine exhausted retries; using safe fallback action\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(stderr.log) error = %v", err)
	}

	return filepath.Join(rootDir, "fixture")
}

func intPtr(v int) *int {
	return &v
}
