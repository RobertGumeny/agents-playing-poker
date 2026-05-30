package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/eval"
	"github.com/RobertGumeny/agent-poker/internal/experiment"
	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

func TestStatusPrintsCoverageAndNextStep(t *testing.T) {
	rootDir := t.TempDir()
	experimentsDir := filepath.Join(rootDir, "experiments")
	expSlugDir := filepath.Join(experimentsDir, "bench")
	sessionsDir := filepath.Join(expSlugDir, "sessions")
	if err := os.MkdirAll(expSlugDir, 0o755); err != nil {
		t.Fatalf("MkdirAll experiments slug: %v", err)
	}

	expPath := filepath.Join(expSlugDir, "bench.json")
	writeExperimentFixture(t, expPath, experiment.Definition{
		ID:              "bench",
		Model:           "anthropic:claude-sonnet-4-6",
		HandsPerSession: 2,
		Control:         experiment.Group{SessionBase: "control", SessionsCount: 1, Agent: "llm-stateless", Opponent: "heuristic"},
		Treatment:       experiment.Group{SessionBase: "treatment", SessionsCount: 1, Agent: "llm-akg-recent", Opponent: "heuristic"},
	})
	createSessionFixture(t, sessionsDir, "control-1", fixtureOptions{Seed: 1, HandCount: 2, Completed: true, Seats: []string{"llm-stateless", "heuristic"}, HandsWritten: 2})

	var stdout strings.Builder
	var stderr strings.Builder
	if err := run([]string{"experiment", "status", "-experiments-dir", experimentsDir, "bench"}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"experiment=bench planned=2 present=1 missing=1 incomplete=0",
		"group_summary group=control planned=1 present=1 missing=0 incomplete=0",
		"group_summary group=treatment planned=1 present=0 missing=1 incomplete=0",
		"next=run",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestStatusNextStepIsAnalyzeWhenAllPresent(t *testing.T) {
	rootDir := t.TempDir()
	experimentsDir := filepath.Join(rootDir, "experiments")
	expSlugDir := filepath.Join(experimentsDir, "bench")
	sessionsDir := filepath.Join(expSlugDir, "sessions")
	if err := os.MkdirAll(expSlugDir, 0o755); err != nil {
		t.Fatalf("MkdirAll experiments slug: %v", err)
	}

	expPath := filepath.Join(expSlugDir, "bench.json")
	writeExperimentFixture(t, expPath, experiment.Definition{
		ID:              "bench",
		Model:           "anthropic:claude-sonnet-4-6",
		HandsPerSession: 2,
		Control:         experiment.Group{SessionBase: "control", SessionsCount: 1, Agent: "llm-stateless", Opponent: "heuristic"},
		Treatment:       experiment.Group{SessionBase: "treatment", SessionsCount: 1, Agent: "llm-akg-recent", Opponent: "heuristic"},
	})
	createSessionFixture(t, sessionsDir, "control-1", fixtureOptions{Seed: 1, HandCount: 2, Completed: true, Seats: []string{"llm-stateless", "heuristic"}, HandsWritten: 2})
	createSessionFixture(t, sessionsDir, "treatment-1", fixtureOptions{Seed: 1, HandCount: 2, Completed: true, Seats: []string{"llm-akg-recent", "heuristic"}, HandsWritten: 2})

	var stdout strings.Builder
	var stderr strings.Builder
	if err := run([]string{"experiment", "status", "-experiments-dir", experimentsDir, "bench"}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "next=analyze") {
		t.Fatalf("stdout missing next=analyze\nfull output:\n%s", stdout.String())
	}
}

func TestAnalyzeWritesReport(t *testing.T) {
	rootDir := t.TempDir()
	experimentsDir := filepath.Join(rootDir, "experiments")
	expSlugDir := filepath.Join(experimentsDir, "bench")
	sessionsDir := filepath.Join(expSlugDir, "sessions")

	if err := os.MkdirAll(expSlugDir, 0o755); err != nil {
		t.Fatalf("MkdirAll experiments slug: %v", err)
	}

	expPath := filepath.Join(expSlugDir, "bench.json")
	writeExperimentFixture(t, expPath, experiment.Definition{
		ID:              "bench",
		Model:           "anthropic:claude-sonnet-4-6",
		HandsPerSession: 5,
		Control: experiment.Group{
			SessionBase:   "control",
			SessionsCount: 1,
			Agent:         "control-agent",
			Opponent:      "villain",
		},
		Treatment: experiment.Group{
			SessionBase:   "treatment",
			SessionsCount: 1,
			Agent:         "treatment-agent",
			Opponent:      "villain",
		},
	})

	writeEvalSummaryFixture(t, filepath.Join(sessionsDir, "control-1"), compareSummaryFixture(compareSummaryConfig{
		SessionID: "control-1", Seed: 1, HandCount: 5, AgentName: "control-agent", OpponentName: "villain", ChipsDelta: 5,
	}))
	writeEvalSummaryFixture(t, filepath.Join(sessionsDir, "treatment-1"), compareSummaryFixture(compareSummaryConfig{
		SessionID: "treatment-1", Seed: 1, HandCount: 5, AgentName: "treatment-agent", OpponentName: "villain", ChipsDelta: 10,
	}))

	var stdout strings.Builder
	var stderr strings.Builder
	if err := run([]string{
		"experiment", "analyze",
		"-experiments-dir", experimentsDir,
		"bench",
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	reportPath := filepath.Join(expSlugDir, "reports", "bench.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", reportPath, err)
	}
	if !strings.Contains(string(data), "# Experiment: bench") {
		t.Fatalf("report missing experiment heading, got:\n%s", string(data))
	}
	if !strings.Contains(stdout.String(), "report=") {
		t.Fatalf("stdout missing report= line, got:\n%s", stdout.String())
	}
}

func TestAnalyzeWritesReportToExplicitDir(t *testing.T) {
	rootDir := t.TempDir()
	expSlugDir := filepath.Join(rootDir, "bench")
	sessionsDir := filepath.Join(expSlugDir, "sessions")

	if err := os.MkdirAll(expSlugDir, 0o755); err != nil {
		t.Fatalf("MkdirAll slug dir: %v", err)
	}

	expPath := filepath.Join(expSlugDir, "bench.json")
	writeExperimentFixture(t, expPath, experiment.Definition{
		ID:              "bench",
		Model:           "anthropic:claude-sonnet-4-6",
		HandsPerSession: 5,
		Control:         experiment.Group{SessionBase: "control", SessionsCount: 1, Agent: "control-agent", Opponent: "villain"},
		Treatment:       experiment.Group{SessionBase: "treatment", SessionsCount: 1, Agent: "treatment-agent", Opponent: "villain"},
	})

	writeEvalSummaryFixture(t, filepath.Join(sessionsDir, "control-1"), compareSummaryFixture(compareSummaryConfig{
		SessionID: "control-1", Seed: 1, HandCount: 5, AgentName: "control-agent", OpponentName: "villain", ChipsDelta: 5,
	}))
	writeEvalSummaryFixture(t, filepath.Join(sessionsDir, "treatment-1"), compareSummaryFixture(compareSummaryConfig{
		SessionID: "treatment-1", Seed: 1, HandCount: 5, AgentName: "treatment-agent", OpponentName: "villain", ChipsDelta: 10,
	}))

	var stdout strings.Builder
	var stderr strings.Builder
	if err := run([]string{
		"experiment", "analyze",
		"-experiment", expPath,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	reportPath := filepath.Join(expSlugDir, "reports", "bench.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", reportPath, err)
	}
	if !strings.Contains(string(data), "# Experiment: bench") {
		t.Fatalf("report missing experiment heading, got:\n%s", string(data))
	}
	if !strings.Contains(stdout.String(), "report=") {
		t.Fatalf("stdout missing report= line, got:\n%s", stdout.String())
	}
}

func TestResolveExperimentFindsFile(t *testing.T) {
	rootDir := t.TempDir()
	slugDir := filepath.Join(rootDir, "test-2b")
	if err := os.MkdirAll(slugDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	expPath := filepath.Join(slugDir, "test-2b.json")
	if err := os.WriteFile(expPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := resolveExperiment("test-2b", rootDir)
	if err != nil {
		t.Fatalf("resolveExperiment() error = %v", err)
	}
	if got != expPath {
		t.Fatalf("resolveExperiment() = %q, want %q", got, expPath)
	}
}

func TestResolveExperimentErrorsWhenMissing(t *testing.T) {
	rootDir := t.TempDir()
	_, err := resolveExperiment("nonexistent", rootDir)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("resolveExperiment() error = %v, want not found", err)
	}
}

// --- fixtures ---

type fixtureOptions struct {
	CreateOnly   bool
	Seed         int64
	HandCount    int
	Completed    bool
	Seats        []string
	HandsWritten int
}

func createSessionFixture(t *testing.T, rootDir, sessionID string, opts fixtureOptions) {
	t.Helper()
	if opts.CreateOnly {
		if err := os.MkdirAll(filepath.Join(rootDir, sessionID), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		return
	}

	writer, err := sessionlog.New(rootDir, sessionID)
	if err != nil {
		t.Fatalf("sessionlog.New: %v", err)
	}
	defer func() {
		if err := writer.Close(); err != nil {
			t.Fatalf("writer.Close: %v", err)
		}
	}()

	seats := make([]sessionlog.ManifestSeat, 0, len(opts.Seats))
	for i, name := range opts.Seats {
		seats = append(seats, sessionlog.ManifestSeat{Seat: i, Name: name})
	}
	for handNumber := 1; handNumber <= opts.HandsWritten; handNumber++ {
		if err := writer.AppendHand(sessionlog.HandRecord{MatchID: "mat_001", HandNumber: handNumber}); err != nil {
			t.Fatalf("AppendHand: %v", err)
		}
	}
	if err := writer.WriteManifest(sessionlog.Manifest{
		SessionID: sessionID,
		Seed:      opts.Seed,
		HandCount: opts.HandCount,
		Matches: []sessionlog.ManifestMatch{{
			MatchID:   "mat_001",
			Seats:     seats,
			Completed: opts.Completed,
		}},
	}); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
}

func writeExperimentFixture(t *testing.T, path string, def experiment.Definition) {
	t.Helper()
	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

type compareSummaryConfig struct {
	SessionID    string
	Seed         int64
	HandCount    int
	AgentName    string
	OpponentName string
	ChipsDelta   int
}

func compareSummaryFixture(cfg compareSummaryConfig) eval.Summary {
	return eval.Summary{
		SchemaVersion: 1,
		SessionID:     cfg.SessionID,
		MatchID:       "mat_001",
		Session: eval.SessionSummary{
			Seed:      cfg.Seed,
			HandCount: cfg.HandCount,
			Completed: true,
		},
		Seats: []eval.SeatSummary{
			{Seat: 0, Name: cfg.AgentName, Version: cfg.AgentName, ChipsDelta: cfg.ChipsDelta},
			{Seat: 1, Name: cfg.OpponentName, Version: cfg.OpponentName, ChipsDelta: -cfg.ChipsDelta},
		},
	}
}

func writeEvalSummaryFixture(t *testing.T, sessionDir string, summary eval.Summary) {
	t.Helper()
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := eval.WriteSummary(sessionDir, summary); err != nil {
		t.Fatalf("WriteSummary: %v", err)
	}
}
