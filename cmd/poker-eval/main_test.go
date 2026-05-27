package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/experiment"
	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

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
		inspectSession: func(planned experiment.PlannedRun, _ int) (sessionInspection, error) {
			switch planned.SessionID {
			case "control-1":
				return sessionInspection{Status: "present"}, nil
			case "control-2":
				return sessionInspection{Status: "incomplete", Reason: "hand_count_mismatch"}, nil
			default:
				return sessionInspection{Status: "missing"}, nil
			}
		},
		execute: func(context.Context, executeConfig) error {
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
	var executed []executeConfig
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
		inspectSession: func(planned experiment.PlannedRun, _ int) (sessionInspection, error) {
			switch planned.SessionID {
			case "control-a":
				return sessionInspection{Status: "present"}, nil
			case "control-b":
				return sessionInspection{Status: "incomplete", Reason: "match_incomplete"}, nil
			default:
				return sessionInspection{Status: "missing"}, nil
			}
		},
		execute: func(_ context.Context, cfg executeConfig) error {
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
		inspectSession: func(experiment.PlannedRun, int) (sessionInspection, error) {
			return sessionInspection{Status: "missing"}, nil
		},
		execute: func(context.Context, executeConfig) error {
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
		inspectSession: func(planned experiment.PlannedRun, _ int) (sessionInspection, error) {
			if planned.SessionID == "control-a" {
				return sessionInspection{Status: "present"}, nil
			}
			return sessionInspection{Status: "incomplete", Reason: "hands_missing"}, nil
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
	inspection, err := inspectExistingSession(planned, 2)
	if err != nil {
		t.Fatalf("inspectExistingSession() error = %v", err)
	}
	if inspection != (sessionInspection{Status: "present"}) {
		t.Fatalf("inspectExistingSession() = %+v, want present", inspection)
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
		wantInspection  sessionInspection
		createDirectory bool
	}{
		{
			name:           "manifest missing",
			sessionID:      "manifest-missing",
			plannedHands:   2,
			fixture:        fixtureOptions{CreateOnly: true},
			wantInspection: sessionInspection{Status: "incomplete", Reason: "manifest_missing"},
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
			wantInspection: sessionInspection{Status: "incomplete", Reason: "seed_mismatch"},
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
			wantInspection: sessionInspection{Status: "incomplete", Reason: "match_incomplete"},
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
			wantInspection: sessionInspection{Status: "incomplete", Reason: "hands_count_mismatch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planned := basePlanned
			planned.SessionID = tt.sessionID
			planned.SessionDir = filepath.Join(rootDir, tt.sessionID)
			createSessionFixture(t, rootDir, tt.sessionID, tt.fixture)

			inspection, err := inspectExistingSession(planned, tt.plannedHands)
			if err != nil {
				t.Fatalf("inspectExistingSession() error = %v", err)
			}
			if inspection != tt.wantInspection {
				t.Fatalf("inspectExistingSession() = %+v, want %+v", inspection, tt.wantInspection)
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
