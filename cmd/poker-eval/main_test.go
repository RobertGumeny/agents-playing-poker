package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/experiment"
)

func TestRunDryRunPrintsDeterministicPlan(t *testing.T) {
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
		sessionStatus: func(dir string) (string, error) {
			if strings.HasSuffix(dir, "control-1") {
				return "existing", nil
			}
			return "missing", nil
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
		"experiment=exp-1 planned=3 existing=1 missing=2 dry_run=true",
		"config hands_per_session=25 sessions_dir=sessions model=<default> thinking_level=low",
		"group=control session_id=control-1 seed=1 agent=llm-stateless opponent=heuristic status=existing dir=" + filepath.Join("sessions", "control-1"),
		"group=control session_id=control-2 seed=2 agent=llm-stateless opponent=heuristic status=missing dir=" + filepath.Join("sessions", "control-2"),
		"group=treatment session_id=treatment-a seed=17 agent=llm-akg-recent opponent=heuristic status=missing dir=" + filepath.Join("sessions", "treatment-a"),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestRunExecutesOnlyMissingSessions(t *testing.T) {
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
		sessionStatus: func(dir string) (string, error) {
			if strings.HasSuffix(dir, "control-a") {
				return "existing", nil
			}
			return "missing", nil
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
	if !strings.Contains(stdout.String(), "skip session_id=control-a reason=existing") {
		t.Fatalf("stdout = %q, want skip line for existing session", stdout.String())
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
		sessionStatus: func(string) (string, error) { return "missing", nil },
		execute: func(context.Context, executeConfig) error {
			t.Fatal("execute called unexpectedly")
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot run without opponent metadata") {
		t.Fatalf("run() error = %v, want missing opponent failure", err)
	}
}
