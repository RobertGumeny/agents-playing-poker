package experiment

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseValidSessionBaseDefinition(t *testing.T) {
	def, err := Parse([]byte(`{
		"id": "phase2-mdsingle-vs-akg",
		"hypothesis": "AKG vs md-single on the fidelity-vs-cost frontier.",
		"model": "anthropic:claude-sonnet-4-6",
		"hands_per_session": 60,
		"decision_deadline_seconds": 180,
		"groups": [
			{
				"session_base": "akg-vs-mdsingle",
				"sessions_count": 2,
				"seat0": "llm-akg-durable",
				"seat1": "llm-md-single",
				"seeds": [1, 2]
			},
			{
				"session_base": "mdsingle-vs-akg",
				"sessions_count": 2,
				"seat0": "llm-md-single",
				"seat1": "llm-akg-durable",
				"seeds": [1, 2]
			}
		]
	}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got0 := def.Groups[0].PlannedSessions("group-0")
	want0 := []PlannedSession{
		{GroupLabel: "group-0", SessionID: "akg-vs-mdsingle-1", Seed: 1},
		{GroupLabel: "group-0", SessionID: "akg-vs-mdsingle-2", Seed: 2},
	}
	if !reflect.DeepEqual(got0, want0) {
		t.Fatalf("group-0 planned sessions = %#v, want %#v", got0, want0)
	}

	got1 := def.Groups[1].PlannedSessions("group-1")
	want1 := []PlannedSession{
		{GroupLabel: "group-1", SessionID: "mdsingle-vs-akg-1", Seed: 1},
		{GroupLabel: "group-1", SessionID: "mdsingle-vs-akg-2", Seed: 2},
	}
	if !reflect.DeepEqual(got1, want1) {
		t.Fatalf("group-1 planned sessions = %#v, want %#v", got1, want1)
	}
}

func TestParseValidExplicitSessionDefinition(t *testing.T) {
	def, err := Parse([]byte(`{
		"id": "retro-benchmark",
		"model": "anthropic:claude-sonnet-4-6",
		"hands_per_session": 200,
		"groups": [
			{
				"sessions": ["fullhistory-vs-stateless-a", "fullhistory-vs-stateless-b"],
				"seat0": "llm-fullhistory",
				"seeds": [1, 1]
			},
			{
				"sessions": ["akg-durable-vs-fullhistory-a", "akg-durable-vs-fullhistory-b"],
				"seat0": "llm-akg-durable"
			}
		]
	}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := def.Groups[1].PlannedSessions("group-1")
	want := []PlannedSession{
		{GroupLabel: "group-1", SessionID: "akg-durable-vs-fullhistory-a", Seed: 1},
		{GroupLabel: "group-1", SessionID: "akg-durable-vs-fullhistory-b", Seed: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("group-1 planned sessions = %#v, want %#v", got, want)
	}
}

func TestDefinitionPlanExpandsSessionDirsDeterministically(t *testing.T) {
	def, err := Parse([]byte(`{
		"id": "run-benchmark",
		"model": "anthropic:claude-sonnet-4-6",
		"hands_per_session": 25,
		"groups": [
			{
				"session_base": "control-group",
				"sessions_count": 2,
				"seat0": "llm-stateless",
				"seat1": "heuristic"
			},
			{
				"sessions": ["treatment-a", "treatment-b"],
				"seat0": "llm-akg-recent",
				"seat1": "heuristic",
				"seeds": [17, 23]
			}
		]
	}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	plan, err := def.Plan("sessions")
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	want := []PlannedRun{
		{GroupLabel: "group-0", SessionID: "control-group-1", SessionDir: filepath.Join("sessions", "control-group-1"), Seed: 1, Seat0: "llm-stateless", Seat1: "heuristic", ExplicitSession: false},
		{GroupLabel: "group-0", SessionID: "control-group-2", SessionDir: filepath.Join("sessions", "control-group-2"), Seed: 2, Seat0: "llm-stateless", Seat1: "heuristic", ExplicitSession: false},
		{GroupLabel: "group-1", SessionID: "treatment-a", SessionDir: filepath.Join("sessions", "treatment-a"), Seed: 17, Seat0: "llm-akg-recent", Seat1: "heuristic", ExplicitSession: true},
		{GroupLabel: "group-1", SessionID: "treatment-b", SessionDir: filepath.Join("sessions", "treatment-b"), Seed: 23, Seat0: "llm-akg-recent", Seat1: "heuristic", ExplicitSession: true},
	}
	if !reflect.DeepEqual(plan.PlannedSessions, want) {
		t.Fatalf("plan.PlannedSessions = %#v, want %#v", plan.PlannedSessions, want)
	}
}

func TestDefinitionPlanRejectsConflictingSessionIDsAcrossGroups(t *testing.T) {
	def, err := Parse([]byte(`{
		"id": "conflict",
		"model": "anthropic:claude-sonnet-4-6",
		"hands_per_session": 25,
		"groups": [
			{
				"sessions": ["shared-session"],
				"seat0": "llm-stateless",
				"seat1": "heuristic",
				"seeds": [1]
			},
			{
				"sessions": ["shared-session"],
				"seat0": "llm-akg-recent",
				"seat1": "heuristic",
				"seeds": [2]
			}
		]
	}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	_, err = def.Plan("sessions")
	if err == nil || !strings.Contains(err.Error(), "conflicting planned session \"shared-session\"") {
		t.Fatalf("Plan() error = %v, want conflicting planned session failure", err)
	}
}

func TestParseRejectsUnknownField(t *testing.T) {
	_, err := Parse([]byte(`{
		"id": "bad",
		"model": "anthropic:claude-sonnet-4-6",
		"hands_per_session": 25,
		"groups": [{"sessions": ["a"], "seat0": "x"}],
		"extra": true
	}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field \"extra\"") {
		t.Fatalf("Parse() error = %v, want unknown field failure", err)
	}
}

func TestValidateRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "missing id",
			json: `{
				"model": "anthropic:claude-sonnet-4-6",
				"hands_per_session": 25,
				"groups": [{"sessions": ["a"], "seat0": "x"}]
			}`,
			want: "id is required",
		},
		{
			name: "missing model",
			json: `{
				"id": "bad",
				"hands_per_session": 25,
				"groups": [{"sessions": ["a"], "seat0": "x"}]
			}`,
			want: "model is required",
		},
		{
			name: "zero groups",
			json: `{
				"id": "bad",
				"model": "anthropic:claude-sonnet-4-6",
				"hands_per_session": 25,
				"groups": []
			}`,
			want: "at least one group is required",
		},
		{
			name: "missing seat0",
			json: `{
				"id": "bad",
				"model": "anthropic:claude-sonnet-4-6",
				"hands_per_session": 25,
				"groups": [{"sessions": ["a"]}]
			}`,
			want: "seat0 is required",
		},
		{
			name: "group uses both modes",
			json: `{
				"id": "bad",
				"model": "anthropic:claude-sonnet-4-6",
				"hands_per_session": 25,
				"groups": [{"session_base": "group", "sessions_count": 2, "sessions": ["a"], "seat0": "x"}]
			}`,
			want: "must use exactly one session mode",
		},
		{
			name: "seed length mismatch for session base",
			json: `{
				"id": "bad",
				"model": "anthropic:claude-sonnet-4-6",
				"hands_per_session": 25,
				"groups": [{"session_base": "group", "sessions_count": 2, "seat0": "x", "seeds": [1]}]
			}`,
			want: "seeds length must match sessions_count",
		},
		{
			name: "duplicate explicit sessions",
			json: `{
				"id": "bad",
				"model": "anthropic:claude-sonnet-4-6",
				"hands_per_session": 25,
				"groups": [{"sessions": ["dup", "dup"], "seat0": "x"}]
			}`,
			want: "duplicates \"dup\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.json))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Parse() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
