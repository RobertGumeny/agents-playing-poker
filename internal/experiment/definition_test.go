package experiment

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseValidSessionBaseDefinition(t *testing.T) {
	def, err := Parse([]byte(`{
		"id": "test-2b-retrieval-throttle",
		"hypothesis": "Throttle retrieval to once per hand.",
		"hands_per_session": 25,
		"control": {
			"session_base": "akg-durable-vs-stateless-test",
			"sessions_count": 3,
			"agent": "llm-akg-durable/0.1.0",
			"opponent": "llm-stateless"
		},
		"treatment": {
			"session_base": "akg-durable-throttle-test",
			"sessions_count": 3,
			"agent": "llm-akg-durable@exp-0.1.3-throttle",
			"opponent": "llm-stateless",
			"seeds": [17, 23, 42]
		},
		"expected_direction": {
			"chips_per_hand": "increase",
			"session_duration_s": "decrease"
		}
	}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	control := def.Control.PlannedSessions("control")
	wantControl := []PlannedSession{
		{GroupLabel: "control", SessionID: "akg-durable-vs-stateless-test-1", Seed: 1},
		{GroupLabel: "control", SessionID: "akg-durable-vs-stateless-test-2", Seed: 2},
		{GroupLabel: "control", SessionID: "akg-durable-vs-stateless-test-3", Seed: 3},
	}
	if !reflect.DeepEqual(control, wantControl) {
		t.Fatalf("control planned sessions = %#v, want %#v", control, wantControl)
	}

	treatment := def.Treatment.PlannedSessions("treatment")
	wantTreatment := []PlannedSession{
		{GroupLabel: "treatment", SessionID: "akg-durable-throttle-test-1", Seed: 17},
		{GroupLabel: "treatment", SessionID: "akg-durable-throttle-test-2", Seed: 23},
		{GroupLabel: "treatment", SessionID: "akg-durable-throttle-test-3", Seed: 42},
	}
	if !reflect.DeepEqual(treatment, wantTreatment) {
		t.Fatalf("treatment planned sessions = %#v, want %#v", treatment, wantTreatment)
	}
}

func TestParseValidExplicitSessionDefinition(t *testing.T) {
	def, err := Parse([]byte(`{
		"id": "retro-benchmark",
		"hands_per_session": 200,
		"control": {
			"sessions": ["fullhistory-vs-stateless-a", "fullhistory-vs-stateless-b"],
			"agent": "llm-fullhistory",
			"seeds": [1, 1]
		},
		"treatment": {
			"sessions": ["akg-durable-vs-fullhistory-a", "akg-durable-vs-fullhistory-b"],
			"agent": "llm-akg-durable"
		}
	}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := def.Treatment.PlannedSessions("treatment")
	want := []PlannedSession{
		{GroupLabel: "treatment", SessionID: "akg-durable-vs-fullhistory-a", Seed: 1},
		{GroupLabel: "treatment", SessionID: "akg-durable-vs-fullhistory-b", Seed: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("treatment planned sessions = %#v, want %#v", got, want)
	}
}

func TestParseRejectsUnknownField(t *testing.T) {
	_, err := Parse([]byte(`{
		"id": "bad",
		"hands_per_session": 25,
		"control": {"sessions": ["a"], "agent": "x"},
		"treatment": {"sessions": ["b"], "agent": "y"},
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
				"hands_per_session": 25,
				"control": {"sessions": ["a"], "agent": "x"},
				"treatment": {"sessions": ["b"], "agent": "y"}
			}`,
			want: "id is required",
		},
		{
			name: "group uses both modes",
			json: `{
				"id": "bad",
				"hands_per_session": 25,
				"control": {"session_base": "group", "sessions_count": 2, "sessions": ["a"], "agent": "x"},
				"treatment": {"sessions": ["b"], "agent": "y"}
			}`,
			want: "control must use exactly one session mode",
		},
		{
			name: "seed length mismatch for session base",
			json: `{
				"id": "bad",
				"hands_per_session": 25,
				"control": {"session_base": "group", "sessions_count": 2, "agent": "x", "seeds": [1]},
				"treatment": {"sessions": ["b"], "agent": "y"}
			}`,
			want: "control.seeds length must match sessions_count",
		},
		{
			name: "duplicate explicit sessions",
			json: `{
				"id": "bad",
				"hands_per_session": 25,
				"control": {"sessions": ["dup", "dup"], "agent": "x"},
				"treatment": {"sessions": ["b"], "agent": "y"}
			}`,
			want: "duplicates \"dup\"",
		},
		{
			name: "invalid expected direction",
			json: `{
				"id": "bad",
				"hands_per_session": 25,
				"control": {"sessions": ["a"], "agent": "x"},
				"treatment": {"sessions": ["b"], "agent": "y"},
				"expected_direction": {"chips_per_hand": "sideways"}
			}`,
			want: "must be \"increase\" or \"decrease\"",
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
