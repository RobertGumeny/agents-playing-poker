package replay

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderSmoke(t *testing.T) {
	amt := 3
	model := ReplayModel{
		Session: SessionMeta{
			SessionID:     "smoke-sess",
			StartingStack: 200,
			Blinds:        BlindMeta{SB: 1, BB: 2},
			Seats:         []SeatMeta{{Seat: 0, Name: "agent-a"}},
		},
		Hands: []HandView{{
			HandNumber:  1,
			HoleCards:   map[int][]string{0: {"As", "Ks"}},
			StacksStart: map[int]int{0: 200},
			Board:       []string{"5c", "Ks", "Qc"},
			Actions: []ActionView{{
				Seat: 0, Street: "flop", Action: "bet", Amount: &amt,
				Overlay: &DecisionOverlay{
					MemoryIndicator: MemoryRecalling,
					ThinkingText:    "check the read",
					RecalledFact:    "villain folds to cbets",
				},
			}},
		}},
		Summary: SummaryView{
			HandCount: 1,
			Seats: []SeatSummaryView{{
				Seat: 0, Name: "agent-a", ChipsDelta: 5,
				CostPoints: []CostPoint{{Hand: 1, PromptTokens: 1000, Cost: 0.01}},
			}},
		},
	}

	var buf bytes.Buffer
	if err := Render(model, &buf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`id="replay-model"`,
		"smoke-sess",
		"agent-a",
		"villain folds to cbets",
		"<!DOCTYPE html>",
		`id="summary"`,
		`id="scoreboard"`,
		`id="chart"`,
		"Session summary",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
}

// A recalled fact containing a script-close sequence must not break out of the
// inlined JSON script element — encoding/json escapes < > & by default.
func TestRenderEscapesScriptClose(t *testing.T) {
	model := ReplayModel{
		Session: SessionMeta{SessionID: "x", Seats: []SeatMeta{{Seat: 0, Name: "a"}}},
		Hands: []HandView{{
			HandNumber:  1,
			HoleCards:   map[int][]string{0: {"As"}},
			StacksStart: map[int]int{0: 200},
			Actions: []ActionView{{
				Seat: 0, Street: "preflop", Action: "bet",
				Overlay: &DecisionOverlay{MemoryIndicator: MemoryRecalling, RecalledFact: "</script><script>alert(1)"},
			}},
		}},
	}
	var buf bytes.Buffer
	if err := Render(model, &buf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(buf.String(), "</script><script>alert(1)") {
		t.Error("raw script-close sequence leaked unescaped into output")
	}
}
