package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/eval"
	"github.com/RobertGumeny/agent-poker/internal/evalrun"
	"github.com/RobertGumeny/agent-poker/internal/experiment"
	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

func TestInitWritesValidExperimentTemplate(t *testing.T) {
	rootDir := t.TempDir()
	outputPath := filepath.Join(rootDir, "experiments", "retrieval-throttle.json")

	var stdout strings.Builder
	var stderr strings.Builder
	if err := run([]string{"init", "-out", outputPath, "-hypothesis", "Test hypothesis.", "-sessions-count", "3", "-hands-per-session", "50", "-control-opponent", "llm-stateless", "-treatment-agent", "llm-akg-durable"}, &stdout, &stderr, runDeps{}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "initialized experiment id=retrieval-throttle output="+outputPath+" planned_sessions=6") {
		t.Fatalf("stdout = %q, want initialized output", stdout.String())
	}

	def, err := experiment.Load(outputPath)
	if err != nil {
		t.Fatalf("experiment.Load() error = %v", err)
	}
	if def.ID != "retrieval-throttle" || def.HandsPerSession != 50 || def.Control.SessionBase != "retrieval-throttle-control" || def.Treatment.SessionBase != "retrieval-throttle-treatment" || def.Control.Opponent != "llm-stateless" || def.Treatment.Agent != "llm-akg-durable" || def.Treatment.Opponent != "llm-stateless" {
		t.Fatalf("loaded definition = %+v, want derived valid template", def)
	}
}

func TestListPrintsExperimentCoverageSummaries(t *testing.T) {
	rootDir := t.TempDir()
	experimentsDir := filepath.Join(rootDir, "experiments")
	sessionsDir := filepath.Join(rootDir, "sessions")
	if err := os.MkdirAll(experimentsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(experiments) error = %v", err)
	}

	experimentPath := filepath.Join(experimentsDir, "bench.json")
	writeExperimentFixture(t, experimentPath, experiment.Definition{
		ID:              "bench",
		HandsPerSession: 2,
		Control:         experiment.Group{SessionBase: "control", SessionsCount: 1, Agent: "llm-stateless", Opponent: "heuristic"},
		Treatment:       experiment.Group{SessionBase: "treatment", SessionsCount: 1, Agent: "llm-akg-recent", Opponent: "heuristic"},
	})
	createSessionFixture(t, sessionsDir, "control-1", fixtureOptions{Seed: 1, HandCount: 2, Completed: true, Seats: []string{"llm-stateless", "heuristic"}, HandsWritten: 2})

	var stdout strings.Builder
	var stderr strings.Builder
	if err := run([]string{"ls", "-experiments-dir", experimentsDir, "-sessions-dir", sessionsDir}, &stdout, &stderr, newRunDeps()); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"experiments_dir=" + experimentsDir + " count=1",
		"path=" + experimentPath + " id=bench planned=2 present=1 missing=1 incomplete=0 hands_per_session=2",
		"group_summary path=" + experimentPath + " group=control planned=1 present=1 missing=0 incomplete=0",
		"group_summary path=" + experimentPath + " group=treatment planned=1 present=0 missing=1 incomplete=0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestCollectWritesEvalJSON(t *testing.T) {
	rootDir := t.TempDir()
	sessionDir := createCollectCLIFixture(t, rootDir)

	var stdout strings.Builder
	var stderr strings.Builder
	if err := run([]string{"collect", sessionDir}, &stdout, &stderr, runDeps{}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "collected session_id=fixture output="+filepath.Join(sessionDir, "eval.json")) {
		t.Fatalf("stdout = %q, want collected line", stdout.String())
	}
	data, err := os.ReadFile(filepath.Join(sessionDir, "eval.json"))
	if err != nil {
		t.Fatalf("ReadFile(eval.json) error = %v", err)
	}
	if !strings.Contains(string(data), `"session_id": "fixture"`) || !strings.Contains(string(data), `"retry_metrics"`) {
		t.Fatalf("eval.json = %s, want session id and retry metrics", string(data))
	}
}

func TestComparePrintsAggregatedReportAndDirectionChecks(t *testing.T) {
	rootDir := t.TempDir()
	sessionsDir := filepath.Join(rootDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sessions) error = %v", err)
	}

	writeEvalSummaryFixture(t, filepath.Join(sessionsDir, "control-1"), compareSummaryFixture(compareSummaryConfig{SessionID: "control-1", Seed: 1, HandCount: 5, AgentName: "control-agent", OpponentName: "villain", ChipsDelta: 5, DurationS: 100, PreflopOnlyRate: 0.6, ShowdownRate: 0.2, ToolCalls: map[string]int{"akg_get_opponent": 10}, ToolCallsPerHand: map[string]float64{"akg_get_opponent": 2}}))
	writeEvalSummaryFixture(t, filepath.Join(sessionsDir, "control-2"), compareSummaryFixture(compareSummaryConfig{SessionID: "control-2", Seed: 2, HandCount: 5, AgentName: "control-agent", OpponentName: "villain", ChipsDelta: 5, DurationS: 120, PreflopOnlyRate: 0.5, ShowdownRate: 0.4, ToolCalls: map[string]int{"akg_get_opponent": 8}, ToolCallsPerHand: map[string]float64{"akg_get_opponent": 1.6}}))
	writeEvalSummaryFixture(t, filepath.Join(sessionsDir, "treatment-1"), compareSummaryFixture(compareSummaryConfig{SessionID: "treatment-1", Seed: 1, HandCount: 5, AgentName: "treatment-agent", OpponentName: "villain", ChipsDelta: 10, DurationS: 80, PreflopOnlyRate: 0.4, ShowdownRate: 0.2, ToolCalls: map[string]int{"akg_get_opponent": 4}, ToolCallsPerHand: map[string]float64{"akg_get_opponent": 0.8}}))
	writeEvalSummaryFixture(t, filepath.Join(sessionsDir, "treatment-2"), compareSummaryFixture(compareSummaryConfig{SessionID: "treatment-2", Seed: 2, HandCount: 5, AgentName: "treatment-agent", OpponentName: "villain", ChipsDelta: 10, DurationS: 90, PreflopOnlyRate: 0.3, ShowdownRate: 0.4, ToolCalls: map[string]int{"akg_get_opponent": 6}, ToolCallsPerHand: map[string]float64{"akg_get_opponent": 1.2}}))

	experimentPath := filepath.Join(rootDir, "experiment.json")
	writeExperimentFixture(t, experimentPath, experiment.Definition{
		ID:              "exp-compare",
		Hypothesis:      "Treatment should win more chips with less tool use.",
		HandsPerSession: 5,
		Control: experiment.Group{
			SessionBase:   "control",
			SessionsCount: 2,
			Agent:         "control-agent",
			Opponent:      "villain",
		},
		Treatment: experiment.Group{
			SessionBase:   "treatment",
			SessionsCount: 2,
			Agent:         "treatment-agent",
			Opponent:      "villain",
		},
		ExpectedDirection: map[string]experiment.Direction{
			"chips_per_hand":               experiment.DirectionIncrease,
			"session_duration_s":           experiment.DirectionDecrease,
			"preflop_only_rate":            experiment.DirectionDecrease,
			"akg_get_opponent_per_session": experiment.DirectionDecrease,
		},
	})

	var stdout strings.Builder
	var stderr strings.Builder
	if err := run([]string{"compare", "-experiment", experimentPath, "-sessions-dir", sessionsDir}, &stdout, &stderr, newRunDeps()); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"# Experiment: exp-compare",
		"| chips/hand | 1.00 | 2.00 | +1.00 | ✅ increase |",
		"| session duration (s) | 110 | 85 | -25 | ✅ decrease |",
		"| preflop-only rate | 55.0% | 35.0% | -20.0pp | ✅ decrease |",
		"| akg_get_opponent/session | 9.00 | 5.00 | -4.00 | ✅ decrease |",
		"| control | control-1 | 1 | control-agent | villain | +5 | 1.00 | 100 | 60.0% | 20.0% |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestCompareRejectsMalformedExperiment(t *testing.T) {
	rootDir := t.TempDir()
	experimentPath := filepath.Join(rootDir, "bad.json")
	if err := os.WriteFile(experimentPath, []byte("{not-json\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(bad.json) error = %v", err)
	}

	var stdout strings.Builder
	var stderr strings.Builder
	err := run([]string{"compare", "-experiment", experimentPath}, &stdout, &stderr, newRunDeps())
	if err == nil || !strings.Contains(err.Error(), "parse experiment definition") {
		t.Fatalf("run() error = %v, want malformed experiment failure", err)
	}
}

func TestCompareRejectsIncompleteCoverage(t *testing.T) {
	rootDir := t.TempDir()
	sessionsDir := filepath.Join(rootDir, "sessions")
	if err := os.MkdirAll(filepath.Join(sessionsDir, "control-1"), 0o755); err != nil {
		t.Fatalf("MkdirAll(control-1) error = %v", err)
	}
	experimentPath := filepath.Join(rootDir, "experiment.json")
	writeExperimentFixture(t, experimentPath, experiment.Definition{
		ID:              "exp-missing",
		HandsPerSession: 5,
		Control:         experiment.Group{SessionBase: "control", SessionsCount: 1, Agent: "control-agent", Opponent: "villain"},
		Treatment:       experiment.Group{SessionBase: "treatment", SessionsCount: 1, Agent: "treatment-agent", Opponent: "villain"},
	})

	var stdout strings.Builder
	var stderr strings.Builder
	err := run([]string{"compare", "-experiment", experimentPath, "-sessions-dir", sessionsDir}, &stdout, &stderr, newRunDeps())
	if err == nil || !strings.Contains(err.Error(), "collected session data missing") {
		t.Fatalf("run() error = %v, want missing eval.json failure", err)
	}
}

func TestCompareWarnsOnInconsistentObservedGroupData(t *testing.T) {
	rootDir := t.TempDir()
	sessionsDir := filepath.Join(rootDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sessions) error = %v", err)
	}
	writeEvalSummaryFixture(t, filepath.Join(sessionsDir, "control-1"), compareSummaryFixture(compareSummaryConfig{SessionID: "control-1", Seed: 1, HandCount: 5, AgentName: "control-agent", OpponentName: "villain-a", ChipsDelta: 1, DurationS: 10}))
	writeEvalSummaryFixture(t, filepath.Join(sessionsDir, "control-2"), compareSummaryFixture(compareSummaryConfig{SessionID: "control-2", Seed: 2, HandCount: 5, AgentName: "control-agent", OpponentName: "villain-b", ChipsDelta: 1, DurationS: 10}))
	writeEvalSummaryFixture(t, filepath.Join(sessionsDir, "treatment-1"), compareSummaryFixture(compareSummaryConfig{SessionID: "treatment-1", Seed: 1, HandCount: 5, AgentName: "treatment-agent", OpponentName: "villain-a", ChipsDelta: 2, DurationS: 10}))
	writeEvalSummaryFixture(t, filepath.Join(sessionsDir, "treatment-2"), compareSummaryFixture(compareSummaryConfig{SessionID: "treatment-2", Seed: 2, HandCount: 5, AgentName: "treatment-agent", OpponentName: "villain-a", ChipsDelta: 2, DurationS: 10}))

	experimentPath := filepath.Join(rootDir, "experiment.json")
	writeExperimentFixture(t, experimentPath, experiment.Definition{
		ID:              "exp-warning",
		HandsPerSession: 5,
		Control:         experiment.Group{SessionBase: "control", SessionsCount: 2, Agent: "control-agent"},
		Treatment:       experiment.Group{SessionBase: "treatment", SessionsCount: 2, Agent: "treatment-agent"},
	})

	var stdout strings.Builder
	var stderr strings.Builder
	if err := run([]string{"compare", "-experiment", experimentPath, "-sessions-dir", sessionsDir}, &stdout, &stderr, newRunDeps()); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "group control observed mixed opponents: villain-a, villain-b") {
		t.Fatalf("stdout = %s, want mixed-opponent warning", stdout.String())
	}
}

func TestRunDryRunPrintsDeterministicPlanAndCoverage(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	err := run([]string{"run", "-experiment", "experiment.json", "-sessions-dir", "sessions", "-dry-run"}, &stdout, &stderr, runDeps{
		loadDefinition: func(string) (experiment.Definition, error) {
			return experiment.Definition{
				ID:              "exp-1",
				HandsPerSession: 25,
				Control: experiment.Group{
					SessionBase:   "control",
					SessionsCount: 2,
					Agent:         "llm-stateless",
					Opponent:      "heuristic",
				},
				Treatment: experiment.Group{
					Sessions: []string{"treatment-a"},
					Agent:    "llm-akg-recent",
					Opponent: "heuristic",
					Seeds:    []int64{17},
				},
			}, nil
		},
		inspectSession: func(planned experiment.PlannedRun, _ int) (evalrun.SessionInspection, error) {
			switch planned.SessionID {
			case "control-1":
				return evalrun.SessionInspection{Status: "present"}, nil
			case "control-2":
				return evalrun.SessionInspection{Status: "incomplete", Reason: "hand_count_mismatch"}, nil
			default:
				return evalrun.SessionInspection{Status: "missing"}, nil
			}
		},
		execute: func(context.Context, evalrun.ExecuteConfig) error {
			t.Fatal("execute called during dry-run")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"experiment=exp-1 planned=3 present=1 missing=1 incomplete=1",
		"config hands_per_session=25 sessions_dir=sessions model=<default> thinking_level=low",
		"group_summary group=control planned=2 present=1 missing=0 incomplete=1",
		"group_summary group=treatment planned=1 present=0 missing=1 incomplete=0",
		"group=control session_id=control-1 seed=1 agent=llm-stateless opponent=heuristic status=present reason=- dir=" + filepath.Join("sessions", "control-1"),
		"group=control session_id=control-2 seed=2 agent=llm-stateless opponent=heuristic status=incomplete reason=hand_count_mismatch dir=" + filepath.Join("sessions", "control-2"),
		"group=treatment session_id=treatment-a seed=17 agent=llm-akg-recent opponent=heuristic status=missing reason=- dir=" + filepath.Join("sessions", "treatment-a"),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestRunExecutesMissingAndIncompleteSessions(t *testing.T) {
	var executed []evalrun.ExecuteConfig
	var stdout strings.Builder
	var stderr strings.Builder
	err := run([]string{"run", "-experiment", "experiment.json", "-sessions-dir", "sessions"}, &stdout, &stderr, runDeps{
		loadDefinition: func(string) (experiment.Definition, error) {
			return experiment.Definition{
				ID:              "exp-1",
				HandsPerSession: 25,
				Control: experiment.Group{
					Sessions: []string{"control-a", "control-b"},
					Agent:    "llm-stateless",
					Opponent: "heuristic",
				},
				Treatment: experiment.Group{
					Sessions: []string{"treatment-a"},
					Agent:    "llm-akg-recent",
					Opponent: "heuristic",
					Seeds:    []int64{17},
				},
			}, nil
		},
		inspectSession: func(planned experiment.PlannedRun, _ int) (evalrun.SessionInspection, error) {
			switch planned.SessionID {
			case "control-a":
				return evalrun.SessionInspection{Status: "present"}, nil
			case "control-b":
				return evalrun.SessionInspection{Status: "incomplete", Reason: "match_incomplete"}, nil
			default:
				return evalrun.SessionInspection{Status: "missing"}, nil
			}
		},
		execute: func(_ context.Context, cfg evalrun.ExecuteConfig) error {
			executed = append(executed, cfg)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if len(executed) != 2 {
		t.Fatalf("len(executed) = %d, want 2", len(executed))
	}
	if executed[0].SessionID != "control-b" || executed[0].Seed != 2 {
		t.Fatalf("executed[0] = %+v, want control-b with seed 2", executed[0])
	}
	if executed[1].SessionID != "treatment-a" || executed[1].Seed != 17 {
		t.Fatalf("executed[1] = %+v, want treatment-a with seed 17", executed[1])
	}
	got := stdout.String()
	for _, want := range []string{
		"skip session_id=control-a reason=present",
		"run session_id=control-b group=control seed=2 prior_status=incomplete",
		"run session_id=treatment-a group=treatment seed=17 prior_status=missing",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestRunRejectsMissingOpponentWhenExecutionNeeded(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	err := run([]string{"run", "-experiment", "experiment.json"}, &stdout, &stderr, runDeps{
		loadDefinition: func(string) (experiment.Definition, error) {
			return experiment.Definition{
				ID:              "exp-1",
				HandsPerSession: 25,
				Control: experiment.Group{
					Sessions: []string{"control-a"},
					Agent:    "llm-stateless",
				},
				Treatment: experiment.Group{
					Sessions: []string{"treatment-a"},
					Agent:    "llm-akg-recent",
					Opponent: "heuristic",
				},
			}, nil
		},
		inspectSession: func(experiment.PlannedRun, int) (evalrun.SessionInspection, error) {
			return evalrun.SessionInspection{Status: "missing"}, nil
		},
		execute: func(context.Context, evalrun.ExecuteConfig) error {
			t.Fatal("execute called unexpectedly")
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot run without opponent metadata") {
		t.Fatalf("run() error = %v, want missing opponent failure", err)
	}
}

func TestStatusPrintsCoverage(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	err := run([]string{"status", "-experiment", "experiment.json", "-sessions-dir", "sessions"}, &stdout, &stderr, runDeps{
		loadDefinition: func(string) (experiment.Definition, error) {
			return experiment.Definition{
				ID:              "exp-1",
				HandsPerSession: 25,
				Control: experiment.Group{
					Sessions: []string{"control-a"},
					Agent:    "llm-stateless",
					Opponent: "heuristic",
				},
				Treatment: experiment.Group{
					Sessions: []string{"treatment-a"},
					Agent:    "llm-akg-recent",
					Opponent: "heuristic",
				},
			}, nil
		},
		inspectSession: func(planned experiment.PlannedRun, _ int) (evalrun.SessionInspection, error) {
			if planned.SessionID == "control-a" {
				return evalrun.SessionInspection{Status: "present"}, nil
			}
			return evalrun.SessionInspection{Status: "incomplete", Reason: "hands_missing"}, nil
		},
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"experiment=exp-1 planned=2 present=1 missing=0 incomplete=1",
		"config hands_per_session=25 sessions_dir=sessions",
		"group=treatment session_id=treatment-a seed=1 agent=llm-akg-recent opponent=heuristic status=incomplete reason=hands_missing dir=" + filepath.Join("sessions", "treatment-a"),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestInspectExistingSession(t *testing.T) {
	rootDir := t.TempDir()
	planned := experiment.PlannedRun{
		GroupLabel: "control",
		SessionID:  "session-1",
		SessionDir: filepath.Join(rootDir, "session-1"),
		Seed:       17,
		Agent:      "llm-stateless",
		Opponent:   "heuristic",
	}

	createSessionFixture(t, rootDir, planned.SessionID, fixtureOptions{
		Seed:         planned.Seed,
		HandCount:    2,
		Completed:    true,
		Seats:        []string{planned.Agent, planned.Opponent},
		HandsWritten: 2,
	})
	inspection, err := evalrun.InspectExistingSession(planned, 2)
	if err != nil {
		t.Fatalf("evalrun.InspectExistingSession() error = %v", err)
	}
	if inspection != (evalrun.SessionInspection{Status: "present"}) {
		t.Fatalf("evalrun.InspectExistingSession() = %+v, want present", inspection)
	}
}

func TestInspectExistingSessionIncompleteReasons(t *testing.T) {
	rootDir := t.TempDir()
	basePlanned := experiment.PlannedRun{
		GroupLabel: "control",
		Seed:       17,
		Agent:      "llm-stateless",
		Opponent:   "heuristic",
	}

	tests := []struct {
		name            string
		sessionID       string
		plannedHands    int
		fixture         fixtureOptions
		wantInspection  evalrun.SessionInspection
		createDirectory bool
	}{
		{
			name:           "manifest missing",
			sessionID:      "manifest-missing",
			plannedHands:   2,
			fixture:        fixtureOptions{CreateOnly: true},
			wantInspection: evalrun.SessionInspection{Status: "incomplete", Reason: "manifest_missing"},
		},
		{
			name:         "seed mismatch",
			sessionID:    "seed-mismatch",
			plannedHands: 2,
			fixture: fixtureOptions{
				Seed:         99,
				HandCount:    2,
				Completed:    true,
				Seats:        []string{basePlanned.Agent, basePlanned.Opponent},
				HandsWritten: 2,
			},
			wantInspection: evalrun.SessionInspection{Status: "incomplete", Reason: "seed_mismatch"},
		},
		{
			name:         "match incomplete",
			sessionID:    "match-incomplete",
			plannedHands: 2,
			fixture: fixtureOptions{
				Seed:         basePlanned.Seed,
				HandCount:    2,
				Completed:    false,
				Seats:        []string{basePlanned.Agent, basePlanned.Opponent},
				HandsWritten: 2,
			},
			wantInspection: evalrun.SessionInspection{Status: "incomplete", Reason: "match_incomplete"},
		},
		{
			name:         "hands count mismatch",
			sessionID:    "hands-count-mismatch",
			plannedHands: 2,
			fixture: fixtureOptions{
				Seed:         basePlanned.Seed,
				HandCount:    2,
				Completed:    true,
				Seats:        []string{basePlanned.Agent, basePlanned.Opponent},
				HandsWritten: 1,
			},
			wantInspection: evalrun.SessionInspection{Status: "incomplete", Reason: "hands_count_mismatch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planned := basePlanned
			planned.SessionID = tt.sessionID
			planned.SessionDir = filepath.Join(rootDir, tt.sessionID)
			createSessionFixture(t, rootDir, tt.sessionID, tt.fixture)

			inspection, err := evalrun.InspectExistingSession(planned, tt.plannedHands)
			if err != nil {
				t.Fatalf("evalrun.InspectExistingSession() error = %v", err)
			}
			if inspection != tt.wantInspection {
				t.Fatalf("evalrun.InspectExistingSession() = %+v, want %+v", inspection, tt.wantInspection)
			}
		})
	}
}

type fixtureOptions struct {
	CreateOnly   bool
	Seed         int64
	HandCount    int
	Completed    bool
	Seats        []string
	HandsWritten int
}

func createCollectCLIFixture(t *testing.T, rootDir string) string {
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

	if err := writer.AppendHand(sessionlog.HandRecord{MatchID: "mat_001", HandNumber: 1, Actions: []sessionlog.HandAction{{Seat: 0, Action: "post_blind", Street: "preflop"}, {Seat: 1, Action: "post_blind", Street: "preflop"}, {Seat: 0, Action: "raise", Street: "preflop", Amount: intPtr(6)}, {Seat: 1, Action: "fold", Street: "preflop"}}, Result: []sessionlog.HandResult{{Seat: 0, ChipsDelta: 2}, {Seat: 1, ChipsDelta: -2}}}); err != nil {
		t.Fatalf("writer.AppendHand() error = %v", err)
	}
	if err := writer.WriteManifest(sessionlog.Manifest{
		SessionID:      "fixture",
		StartedAt:      "2026-05-27T10:00:00Z",
		EndedAt:        "2026-05-27T10:00:05Z",
		Seed:           1,
		HandCount:      1,
		Variant:        "heads-up-nlhe",
		InfoRealism:    "showdown-only",
		StartingStack:  200,
		Blinds:         sessionlog.BlindLevel{SB: 1, BB: 2},
		ServerVersion:  "dev",
		AKGSpecVersion: "v1-draft-2",
		Matches: []sessionlog.ManifestMatch{{
			MatchID:   "mat_001",
			Seats:     []sessionlog.ManifestSeat{{Seat: 0, Name: "agent-a"}, {Seat: 1, Name: "agent-b"}},
			Result:    map[int]sessionlog.ManifestSeatResult{0: {ChipsDelta: 2}, 1: {ChipsDelta: -2}},
			Completed: true,
		}},
	}); err != nil {
		t.Fatalf("writer.WriteManifest() error = %v", err)
	}
	agentDir, err := writer.AgentDir("agent-a")
	if err != nil {
		t.Fatalf("writer.AgentDir() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "pi-session.jsonl"), []byte("{\"type\":\"fake_pi_session\",\"session_scope\":\"decision\",\"session_number\":1,\"hand_number\":1,\"decision_number\":1,\"prompt\":\"Hand: 1\"}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(pi-session.jsonl) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "stderr.log"), []byte("decision attempt 1/2 failed: pi decision failed: assistant returned malformed action JSON: \"bad\"\ndecision engine exhausted retries; using safe fallback action\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(stderr.log) error = %v", err)
	}
	return filepath.Join(rootDir, "fixture")
}

func intPtr(v int) *int {
	return &v
}

type compareSummaryConfig struct {
	SessionID        string
	Seed             int64
	HandCount        int
	AgentName        string
	AgentVersion     string
	OpponentName     string
	OpponentVersion  string
	ChipsDelta       int
	DurationS        int64
	PreflopOnlyRate  float64
	ShowdownRate     float64
	FallbackActions  int
	DecisionPrompts  int
	ToolCalls        map[string]int
	ToolCallsPerHand map[string]float64
}

func compareSummaryFixture(cfg compareSummaryConfig) eval.Summary {
	if cfg.AgentVersion == "" {
		cfg.AgentVersion = cfg.AgentName
	}
	if cfg.OpponentVersion == "" {
		cfg.OpponentVersion = cfg.OpponentName
	}
	return eval.Summary{
		SchemaVersion: 1,
		SessionID:     cfg.SessionID,
		MatchID:       "mat_001",
		Session: eval.SessionSummary{
			Seed:      cfg.Seed,
			DurationS: cfg.DurationS,
			HandCount: cfg.HandCount,
			Completed: true,
		},
		Metrics: eval.SessionMetrics{
			PreflopOnlyRate:     cfg.PreflopOnlyRate,
			ShowdownRate:        cfg.ShowdownRate,
			FallbackActionCount: cfg.FallbackActions,
		},
		Seats: []eval.SeatSummary{
			{
				Seat:                0,
				Name:                cfg.AgentName,
				Version:             cfg.AgentVersion,
				ChipsDelta:          cfg.ChipsDelta,
				DecisionPromptCount: cfg.DecisionPrompts,
				ToolCalls:           cfg.ToolCalls,
				ToolCallsPerHand:    cfg.ToolCallsPerHand,
			},
			{
				Seat:       1,
				Name:       cfg.OpponentName,
				Version:    cfg.OpponentVersion,
				ChipsDelta: -cfg.ChipsDelta,
			},
		},
	}
}

func writeEvalSummaryFixture(t *testing.T, sessionDir string, summary eval.Summary) {
	t.Helper()
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", sessionDir, err)
	}
	if err := eval.WriteSummary(sessionDir, summary); err != nil {
		t.Fatalf("WriteSummary(%s) error = %v", sessionDir, err)
	}
}

func writeExperimentFixture(t *testing.T, path string, def experiment.Definition) {
	t.Helper()
	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("json.Marshal(experiment) error = %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func createSessionFixture(t *testing.T, rootDir, sessionID string, opts fixtureOptions) {
	t.Helper()

	if opts.CreateOnly {
		if err := os.MkdirAll(filepath.Join(rootDir, sessionID), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		return
	}

	writer, err := sessionlog.New(rootDir, sessionID)
	if err != nil {
		t.Fatalf("sessionlog.New() error = %v", err)
	}
	defer func() {
		if err := writer.Close(); err != nil {
			t.Fatalf("writer.Close() error = %v", err)
		}
	}()

	seats := make([]sessionlog.ManifestSeat, 0, len(opts.Seats))
	for i, name := range opts.Seats {
		seats = append(seats, sessionlog.ManifestSeat{Seat: i, Name: name})
	}

	for handNumber := 1; handNumber <= opts.HandsWritten; handNumber++ {
		if err := writer.AppendHand(sessionlog.HandRecord{MatchID: "mat_001", HandNumber: handNumber}); err != nil {
			t.Fatalf("writer.AppendHand() error = %v", err)
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
		t.Fatalf("writer.WriteManifest() error = %v", err)
	}
}
