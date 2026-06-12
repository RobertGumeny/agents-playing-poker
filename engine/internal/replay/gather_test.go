package replay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixture builds a minimal hermetic session under dir: a manifest with one
// seated agent, a one-hand hands.jsonl, and that agent's legacy decision trace.
// It deliberately uses the legacy pi-session.jsonl name to exercise the dual-read
// path (all archived showcase sessions are legacy / marker-less).
func writeFixture(t *testing.T, dir, manifest, hands, trace string) {
	t.Helper()
	mustWrite(t, filepath.Join(dir, "manifest.json"), manifest)
	mustWrite(t, filepath.Join(dir, "hands.jsonl"), hands)
	mustWrite(t, filepath.Join(dir, "agents", "agent-a", "pi-session.jsonl"), trace)
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const fixtureManifest = `{"session_id":"test-sess","seed":7,"hand_count":1,"variant":"heads-up-nlhe","starting_stack":200,"blinds":{"sb":1,"bb":2},"matches":[{"match_id":"m1","seats":[{"seat":0,"name":"agent-a"}],"result":{"0":{"chips_delta":5}},"completed":true}]}`

// One hand; seat 0 posts a blind then has two non-blind actions (call, bet).
const fixtureHands = `{"match_id":"m1","hand_number":1,"dealer_seat":0,"stacks_start":{"0":200,"1":200},"blinds_posted":[{"seat":0,"amount":1}],"hole_cards":{"0":["As","Ks"],"1":["2d","7h"]},"board":["5c","Ks","Qc"],"actions":[{"seat":0,"action":"post_blind","amount":1,"street":"preflop"},{"seat":0,"action":"call","amount":1,"street":"preflop"},{"seat":0,"action":"bet","amount":3,"street":"flop"}],"showdown_reached":false,"result":[{"seat":0,"chips_delta":5}],"gross_pot_size":10}
`

// Two decisions: the first reasons but reads nothing (deciding); the second reads
// memory and gets a toolResult carrying the recalled fact (recalling).
const fixtureTrace = `{"type":"session","version":3}
{"type":"message","message":{"role":"user","content":[{"type":"text","text":"Hand: 1\nStreet: preflop\nLegal actions: [{\"action\":\"call\"}]"}]}}
{"type":"message","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Decent hand, just call."},{"type":"text","text":"{\"action\":\"call\"}"}],"usage":{"input":1000,"cacheRead":0,"cacheWrite":0,"totalTokens":1200,"cost":{"total":0.01}}}}
{"type":"message","message":{"role":"user","content":[{"type":"text","text":"Hand: 1\nStreet: flop\nLegal actions: [{\"action\":\"bet\"}]\nYour opponent summary follows:\nfolds to cbets 70%"}]}}
{"type":"message","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Let me check the tendency node."},{"type":"toolCall","name":"akg_get_node"}]}}
{"type":"message","message":{"role":"toolResult","content":[{"type":"text","text":"{\"body\":\"villain folds to cbets\"}"}]}}
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"{\"action\":\"bet\",\"amount\":3}"}]}}
`

func TestBuildModelAlignsOverlays(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fixtureManifest, fixtureHands, fixtureTrace)

	model, warnings, err := BuildModel(dir)
	if err != nil {
		t.Fatalf("BuildModel: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(model.Hands) != 1 {
		t.Fatalf("want 1 hand, got %d", len(model.Hands))
	}

	acts := model.Hands[0].Actions
	if len(acts) != 3 {
		t.Fatalf("want 3 actions, got %d", len(acts))
	}
	// Blind carries no overlay.
	if acts[0].Action != "post_blind" || acts[0].Overlay != nil {
		t.Fatalf("blind action should have no overlay: %+v", acts[0])
	}
	// First non-blind action <- first decision: reasoned, no read.
	call := acts[1]
	if call.Overlay == nil {
		t.Fatal("call action missing overlay")
	}
	if call.Overlay.MemoryIndicator != MemoryDeciding {
		t.Errorf("call: want deciding, got %s", call.Overlay.MemoryIndicator)
	}
	if call.Overlay.ThinkingText == "" {
		t.Error("call: want thinking text, got empty")
	}
	if call.Overlay.RecalledFact != "" {
		t.Errorf("call: want no recalled fact, got %q", call.Overlay.RecalledFact)
	}
	// Second non-blind action <- second decision: read memory.
	bet := acts[2]
	if bet.Overlay == nil {
		t.Fatal("bet action missing overlay")
	}
	if bet.Overlay.MemoryIndicator != MemoryRecalling {
		t.Errorf("bet: want recalling, got %s", bet.Overlay.MemoryIndicator)
	}
	if bet.Overlay.RecalledFact != "villain folds to cbets" {
		t.Errorf("bet: recalled fact = %q", bet.Overlay.RecalledFact)
	}
	if bet.Overlay.InjectedSummaryGist != "folds to cbets 70%" {
		t.Errorf("bet: gist = %q", bet.Overlay.InjectedSummaryGist)
	}
}

func TestBuildModelSummary(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fixtureManifest, fixtureHands, fixtureTrace)

	model, _, err := BuildModel(dir)
	if err != nil {
		t.Fatalf("BuildModel: %v", err)
	}
	s := model.Summary
	if s.HandCount != 1 {
		t.Errorf("HandCount = %d, want 1", s.HandCount)
	}
	if len(s.Seats) != 1 {
		t.Fatalf("want 1 summary seat, got %d", len(s.Seats))
	}
	seat := s.Seats[0]
	if seat.ChipsDelta != 5 {
		t.Errorf("ChipsDelta = %d, want 5", seat.ChipsDelta)
	}
	// Two decision prompts; the second read memory.
	if seat.Decisions != 2 || seat.DecisionsWithRead != 1 || seat.TotalReads != 1 {
		t.Errorf("engagement = {dec %d, withRead %d, reads %d}, want {2,1,1}",
			seat.Decisions, seat.DecisionsWithRead, seat.TotalReads)
	}
	if len(seat.CostPoints) == 0 {
		t.Fatal("want >=1 cost point from the usage-bearing decision")
	}
	if seat.CostPoints[0].PromptTokens != 1000 {
		t.Errorf("CostPoints[0].PromptTokens = %d, want 1000", seat.CostPoints[0].PromptTokens)
	}
}

// Multi-street hand exercising the running-state reconstruction: blinds, a call,
// a raise (to-total encoding), a street change (commitment reset), a bet, a fold.
const fixtureStateManifest = `{"session_id":"st","seed":1,"hand_count":1,"variant":"heads-up-nlhe","starting_stack":200,"blinds":{"sb":1,"bb":2},"matches":[{"match_id":"m1","seats":[{"seat":0,"name":"a"},{"seat":1,"name":"b"}],"result":{"0":{"chips_delta":4}},"completed":true}]}`
const fixtureStateHands = `{"match_id":"m1","hand_number":1,"dealer_seat":0,"stacks_start":{"0":200,"1":200},"blinds_posted":[{"seat":0,"amount":1},{"seat":1,"amount":2}],"hole_cards":{"0":["As","Ks"],"1":["2d","7h"]},"board":["5c","Ks","Qc","3h"],"actions":[{"seat":0,"action":"post_blind","amount":1,"street":"preflop"},{"seat":1,"action":"post_blind","amount":2,"street":"preflop"},{"seat":0,"action":"raise","amount":8,"street":"preflop"},{"seat":1,"action":"call","amount":6,"street":"preflop"},{"seat":0,"action":"bet","amount":10,"street":"flop"},{"seat":1,"action":"fold","amount":null,"street":"flop"}],"showdown_reached":false,"result":[{"seat":0,"chips_delta":4},{"seat":1,"chips_delta":-4}],"gross_pot_size":0}
`

func TestComputeStatesTimeline(t *testing.T) {
	dir := t.TempDir()
	// reuse a minimal trace for seat "a" so BuildModel runs; state recon is what we test.
	writeFixtureTwoSeat(t, dir, fixtureStateManifest, fixtureStateHands)

	model, _, err := BuildModel(dir)
	if err != nil {
		t.Fatalf("BuildModel: %v", err)
	}
	acts := model.Hands[0].Actions
	// expected per-action: label, chipsAdded, pot
	want := []struct {
		label              string
		added              int
		pot                int
		s0commit, s1commit int
		s0stack, s1stack   int
	}{
		{"SB 1", 1, 1, 1, 0, 199, 200},
		{"BB 2", 2, 3, 1, 2, 199, 198},
		{"RAISE to 8", 7, 10, 8, 2, 192, 198}, // added = 8 - 1 (SB already committed)
		{"CALL", 6, 16, 8, 8, 192, 192},
		{"BET 10", 10, 26, 10, 0, 182, 192}, // new street: commitments reset, then bet 10
		{"FOLD", 0, 26, 10, 0, 182, 192},
	}
	if len(acts) != len(want) {
		t.Fatalf("got %d actions, want %d", len(acts), len(want))
	}
	for i, w := range want {
		a := acts[i]
		if a.Label != w.label {
			t.Errorf("action %d: label %q, want %q", i, a.Label, w.label)
		}
		if a.ChipsAdded != w.added {
			t.Errorf("action %d (%s): chipsAdded %d, want %d", i, w.label, a.ChipsAdded, w.added)
		}
		if a.State.Pot != w.pot {
			t.Errorf("action %d (%s): pot %d, want %d", i, w.label, a.State.Pot, w.pot)
		}
		if a.State.Seats[0].StreetCommitted != w.s0commit || a.State.Seats[1].StreetCommitted != w.s1commit {
			t.Errorf("action %d (%s): committed {%d,%d}, want {%d,%d}", i, w.label,
				a.State.Seats[0].StreetCommitted, a.State.Seats[1].StreetCommitted, w.s0commit, w.s1commit)
		}
		if a.State.Seats[0].Stack != w.s0stack || a.State.Seats[1].Stack != w.s1stack {
			t.Errorf("action %d (%s): stacks {%d,%d}, want {%d,%d}", i, w.label,
				a.State.Seats[0].Stack, a.State.Seats[1].Stack, w.s0stack, w.s1stack)
		}
	}
	if !acts[5].State.Seats[1].Folded {
		t.Error("seat 1 should be folded after the fold action")
	}
}

func writeFixtureTwoSeat(t *testing.T, dir, manifest, hands string) {
	t.Helper()
	mustWrite(t, filepath.Join(dir, "manifest.json"), manifest)
	mustWrite(t, filepath.Join(dir, "hands.jsonl"), hands)
	// a minimal one-line trace per seat so loadAgentArtifacts finds something; content irrelevant here.
	stub := `{"type":"session","version":3}` + "\n"
	mustWrite(t, filepath.Join(dir, "agents", "a", "pi-session.jsonl"), stub)
	mustWrite(t, filepath.Join(dir, "agents", "b", "pi-session.jsonl"), stub)
}

func TestBuildModelMismatchToleratesSlack(t *testing.T) {
	dir := t.TempDir()
	// Append a third decision with no corresponding action: 3 decisions vs 2 actions.
	extraDecision := `{"type":"message","message":{"role":"user","content":[{"type":"text","text":"Hand: 1\nStreet: turn\nLegal actions: [{\"action\":\"check\"}]"}]}}
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"{\"action\":\"check\"}"}]}}
`
	writeFixture(t, dir, fixtureManifest, fixtureHands, fixtureTrace+extraDecision)

	model, warnings, err := BuildModel(dir)
	if err != nil {
		t.Fatalf("BuildModel: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("want 1 alignment warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "3 trace decisions vs 2 non-blind actions") {
		t.Errorf("warning text = %q", warnings[0])
	}
	// First two actions still aligned; nothing panicked.
	acts := model.Hands[0].Actions
	if acts[1].Overlay == nil || acts[2].Overlay == nil {
		t.Fatal("first two non-blind actions should still be aligned")
	}
}
