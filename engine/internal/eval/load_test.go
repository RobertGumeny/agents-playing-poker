package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

func TestLoadSessionReadsRealPiSessionShapes(t *testing.T) {
	sessionDir := filepath.Join("..", "..", "..", "research", "sessions", "akg-durable-prompt-test-1")

	artifacts, err := LoadSession(sessionDir)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if artifacts.Manifest.SessionID != "akg-durable-prompt-test-1" {
		t.Fatalf("session id = %q, want akg-durable-prompt-test-1", artifacts.Manifest.SessionID)
	}
	if len(artifacts.Hands) != 25 {
		t.Fatalf("len(hands) = %d, want 25", len(artifacts.Hands))
	}
	if len(artifacts.Agents) != 2 {
		t.Fatalf("len(agents) = %d, want 2", len(artifacts.Agents))
	}

	durable := artifacts.Agents[0]
	if durable.Seat.Name != "llm-akg-durable" {
		t.Fatalf("agent[0].seat.name = %q, want llm-akg-durable", durable.Seat.Name)
	}
	if durable.PiSession == nil {
		t.Fatal("agent[0].PiSession = nil, want parsed pi-session.jsonl")
	}
	if durable.MemoryExport != nil {
		t.Fatal("agent[0].MemoryExport != nil, want absent optional export")
	}
	if got := durable.PiSession.DecisionPromptCount(); got != 113 {
		t.Fatalf("agent[0] decision prompts = %d, want 113", got)
	}
	if got := durable.PiSession.ToolCallCounts(); got["akg_get_opponent"] != 76 || got["akg_list_patterns"] != 27 || got["akg_get_pattern"] != 4 {
		t.Fatalf("agent[0] tool calls = %#v, want opponent=76 list_patterns=27 get_pattern=4", got)
	}

	stateless := artifacts.Agents[1]
	if stateless.Seat.Name != "llm-stateless" {
		t.Fatalf("agent[1].seat.name = %q, want llm-stateless", stateless.Seat.Name)
	}
	if stateless.PiSession == nil {
		t.Fatal("agent[1].PiSession = nil, want parsed pi-session.jsonl")
	}
	if got := stateless.PiSession.DecisionPromptCount(); got != 85 {
		t.Fatalf("agent[1] decision prompts = %d, want 85", got)
	}
	if got := stateless.PiSession.ToolCallCounts(); len(got) != 0 {
		t.Fatalf("agent[1] tool calls = %#v, want empty", got)
	}
}

func TestLoadSessionReadsHistoricalRealPiSessionShape(t *testing.T) {
	sessionDir := filepath.Join("..", "..", "..", "research", "sessions", "akg-recent-vs-stateless-seed1-a")

	artifacts, err := LoadSession(sessionDir)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if len(artifacts.Agents) != 2 {
		t.Fatalf("len(agents) = %d, want 2", len(artifacts.Agents))
	}
	if artifacts.Agents[0].Seat.Name != "llm-akg" {
		t.Fatalf("agent[0].seat.name = %q, want llm-akg", artifacts.Agents[0].Seat.Name)
	}
	if artifacts.Agents[0].PiSession == nil {
		t.Fatal("agent[0].PiSession = nil, want parsed pi-session.jsonl")
	}
	if got := artifacts.Agents[0].PiSession.DecisionPromptCount(); got != 409 {
		t.Fatalf("agent[0] decision prompts = %d, want 409", got)
	}
	if got := artifacts.Agents[0].PiSession.ToolCallCounts(); len(got) != 0 {
		t.Fatalf("agent[0] tool calls = %#v, want empty", got)
	}
}

func TestLoadSessionReadsFixturePiSessionAndOptionalMemoryExport(t *testing.T) {
	rootDir := t.TempDir()
	sessionDir := filepath.Join(rootDir, "fixture")

	writer, err := sessionlog.New(rootDir, "fixture")
	if err != nil {
		t.Fatalf("sessionlog.New() error = %v", err)
	}
	defer func() {
		if err := writer.Close(); err != nil {
			t.Fatalf("writer.Close() error = %v", err)
		}
	}()
	if err := writer.AppendHand(sessionlog.HandRecord{MatchID: "mat_001", HandNumber: 1}); err != nil {
		t.Fatalf("writer.AppendHand() error = %v", err)
	}
	if err := writer.WriteManifest(sessionlog.Manifest{
		SessionID: "fixture",
		Seed:      7,
		HandCount: 1,
		Matches: []sessionlog.ManifestMatch{{
			MatchID: "mat_001",
			Seats:   []sessionlog.ManifestSeat{{Seat: 0, Name: "fake-agent"}, {Seat: 1, Name: "no-log"}},
		}},
	}); err != nil {
		t.Fatalf("writer.WriteManifest() error = %v", err)
	}

	agentDir, err := writer.AgentDir("fake-agent")
	if err != nil {
		t.Fatalf("writer.AgentDir() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, piSessionFileName), []byte(strings.Join([]string{
		`{"type":"fake_pi_session","session_scope":"decision","session_number":1,"hand_number":1,"decision_number":1,"prompt":"Hand: 1"}`,
		`{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"ok"},{"type":"toolCall","name":"akg_get_opponent"}]}}`,
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(pi-session.jsonl) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, memoryExportFileName), []byte(`{
		"nodes": [
		  {"type": "opponent", "id": "villain"},
		  {"type": "hand", "id": "hand-1"}
		],
		"edges": [
		  {"relation": "supported_by"}
		]
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile(memory-export.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, stderrFileName), []byte(strings.Join([]string{
		`decision attempt 1/2 failed: pi decision failed: assistant returned malformed action JSON: "bad"`,
		`decision engine exhausted retries; using safe fallback action`,
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(stderr.log) error = %v", err)
	}

	artifacts, err := LoadSession(sessionDir)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if len(artifacts.Agents) != 2 {
		t.Fatalf("len(agents) = %d, want 2", len(artifacts.Agents))
	}
	if artifacts.Agents[0].PiSession == nil {
		t.Fatal("agent[0].PiSession = nil, want parsed fake pi-session fixture")
	}
	if got := artifacts.Agents[0].PiSession.DecisionPromptCount(); got != 1 {
		t.Fatalf("agent[0] decision prompts = %d, want 1", got)
	}
	if got := artifacts.Agents[0].PiSession.ToolCallCounts(); got["akg_get_opponent"] != 1 {
		t.Fatalf("agent[0] tool calls = %#v, want akg_get_opponent=1", got)
	}
	if artifacts.Agents[0].MemoryExport == nil {
		t.Fatal("agent[0].MemoryExport = nil, want parsed optional memory export")
	}
	summary := artifacts.Agents[0].MemoryExport.Summary()
	if summary.NodeCount != 2 || summary.EdgeCount != 1 || summary.NodesByType["opponent"] != 1 || summary.NodesByType["hand"] != 1 || summary.EdgesByRelation["supported_by"] != 1 {
		t.Fatalf("memory export summary = %#v, want 2 nodes 1 edge with expected buckets", summary)
	}
	if artifacts.Agents[0].RetrySummary.AttemptFailures != 1 || artifacts.Agents[0].RetrySummary.MalformedActionRetries != 1 || artifacts.Agents[0].RetrySummary.ExhaustedCount != 1 {
		t.Fatalf("retry summary = %#v, want attempts=1 malformed=1 exhausted=1", artifacts.Agents[0].RetrySummary)
	}
	if artifacts.Agents[1].PiSession != nil {
		t.Fatal("agent[1].PiSession != nil, want missing optional pi-session artifact")
	}
	if artifacts.Agents[1].MemoryExport != nil {
		t.Fatal("agent[1].MemoryExport != nil, want missing optional memory export")
	}
}

func TestLoadSessionMissingRequiredArtifacts(t *testing.T) {
	rootDir := t.TempDir()

	if _, err := LoadSession(rootDir); err == nil || !strings.Contains(err.Error(), "required artifact missing") || !strings.Contains(err.Error(), "manifest.json") {
		t.Fatalf("LoadSession() error = %v, want missing manifest.json error", err)
	}

	writer, err := sessionlog.New(rootDir, "fixture")
	if err != nil {
		t.Fatalf("sessionlog.New() error = %v", err)
	}
	if err := writer.WriteManifest(sessionlog.Manifest{
		SessionID: "fixture",
		HandCount: 1,
		Matches: []sessionlog.ManifestMatch{{
			MatchID: "mat_001",
			Seats:   []sessionlog.ManifestSeat{{Seat: 0, Name: "agent"}},
		}},
	}); err != nil {
		t.Fatalf("writer.WriteManifest() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
	if err := os.Remove(filepath.Join(rootDir, "fixture", "hands.jsonl")); err != nil {
		t.Fatalf("Remove(hands.jsonl) error = %v", err)
	}

	if _, err := LoadSession(filepath.Join(rootDir, "fixture")); err == nil || !strings.Contains(err.Error(), "required artifact missing") || !strings.Contains(err.Error(), "hands.jsonl") {
		t.Fatalf("LoadSession() error = %v, want missing hands.jsonl error", err)
	}
}
