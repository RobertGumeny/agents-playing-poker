package eval

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestTokenCallsFromMarkedFile exercises the real seam: a JSONL trace written in
// the format the TS writer emits (hand_boundary marker line, then transcript),
// parsed back through ReadPiSessionLog and attributed by marker.
func TestTokenCallsFromMarkedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-decisions.jsonl")
	lines := `{"type":"hand_boundary","hand_number":3}
{"type":"session","version":3}
{"type":"message","message":{"role":"user","content":[{"type":"text","text":"Hand: 999\nStreet: turn"}]}}
{"type":"message","message":{"role":"assistant","usage":{"input":700,"cacheRead":100,"totalTokens":820,"cost":{"total":0.004}}}}
`
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	log, err := ReadPiSessionLog(path)
	if err != nil {
		t.Fatal(err)
	}
	calls := log.TokenCalls("decision")
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].Hand == nil || *calls[0].Hand != 3 {
		t.Errorf("hand = %v, want 3 (from marker, not prose 999)", calls[0].Hand)
	}
	if calls[0].PromptTokens != 800 || calls[0].Street != "turn" {
		t.Errorf("prompt/street = %d/%q, want 800/turn", calls[0].PromptTokens, calls[0].Street)
	}
}

func decisionLog() PiSessionLog {
	mkUser := func(text string) PiSessionEvent {
		return PiSessionEvent{Type: "message", Message: &PiSessionMessage{
			Role: "user", Content: []PiSessionContentItem{{Type: "text", Text: text}},
		}}
	}
	mkAssistant := func(input, cacheRead, total int, cost float64) PiSessionEvent {
		return PiSessionEvent{Type: "message", Message: &PiSessionMessage{
			Role:  "assistant",
			Usage: &PiUsage{Input: input, CacheRead: cacheRead, TotalTokens: total, Cost: &PiCost{Total: cost}},
		}}
	}
	return PiSessionLog{Events: []PiSessionEvent{
		{Type: "session"},
		mkUser("Hand: 1\nStreet: preflop\nfacing a raise"),
		mkAssistant(400, 100, 520, 0.001),
		{Type: "session"},
		mkUser("Hand: 2\nStreet: flop\nc-bet spot"),
		mkAssistant(600, 50, 700, 0.002),
	}}
}

func TestTokenCalls(t *testing.T) {
	calls := decisionLog().TokenCalls("decision")
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if calls[0].Hand == nil || *calls[0].Hand != 1 {
		t.Errorf("call 0 hand = %v, want 1", calls[0].Hand)
	}
	if calls[0].PromptTokens != 500 { // input 400 + cacheRead 100
		t.Errorf("call 0 prompt = %d, want 500", calls[0].PromptTokens)
	}
	if calls[0].Street != "preflop" {
		t.Errorf("call 0 street = %q, want preflop", calls[0].Street)
	}
	if calls[1].PromptTokens != 650 { // 600 + 50
		t.Errorf("call 1 prompt = %d, want 650", calls[1].PromptTokens)
	}
	if calls[1].TotalTokens != 700 || math.Abs(calls[1].Cost-0.002) > 1e-9 {
		t.Errorf("call 1 total/cost = %d/%v", calls[1].TotalTokens, calls[1].Cost)
	}
}

func TestTokenCallsPrefersHandBoundaryMarker(t *testing.T) {
	// Marker says hand 7; the prose deliberately says "Hand: 99". The structured
	// marker must win, while street still comes from the prompt prose.
	log := PiSessionLog{Events: []PiSessionEvent{
		{Type: "hand_boundary", HandNumber: 7},
		{Type: "session"},
		{Type: "message", Message: &PiSessionMessage{
			Role: "user", Content: []PiSessionContentItem{{Type: "text", Text: "Hand: 99\nStreet: flop"}},
		}},
		{Type: "message", Message: &PiSessionMessage{
			Role: "assistant", Usage: &PiUsage{Input: 500, TotalTokens: 510, Cost: &PiCost{Total: 0.001}},
		}},
	}}
	calls := log.TokenCalls("decision")
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].Hand == nil || *calls[0].Hand != 7 {
		t.Errorf("hand = %v, want 7 (marker wins over prose)", calls[0].Hand)
	}
	if calls[0].Street != "flop" {
		t.Errorf("street = %q, want flop (from prose)", calls[0].Street)
	}
}

func TestTokenCallsTwoMarkedHands(t *testing.T) {
	mkUser := func(text string) PiSessionEvent {
		return PiSessionEvent{Type: "message", Message: &PiSessionMessage{
			Role: "user", Content: []PiSessionContentItem{{Type: "text", Text: text}},
		}}
	}
	mkAsst := func(in int) PiSessionEvent {
		return PiSessionEvent{Type: "message", Message: &PiSessionMessage{
			Role: "assistant", Usage: &PiUsage{Input: in, TotalTokens: in},
		}}
	}
	log := PiSessionLog{Events: []PiSessionEvent{
		{Type: "hand_boundary", HandNumber: 1}, {Type: "session"}, mkUser("Hand: 1"), mkAsst(100),
		{Type: "hand_boundary", HandNumber: 2}, {Type: "session"}, mkUser("Hand: 2"), mkAsst(200),
	}}
	calls := log.TokenCalls("decision")
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if *calls[0].Hand != 1 || *calls[1].Hand != 2 {
		t.Errorf("hands = %v,%v, want 1,2 (no cross-contamination)", *calls[0].Hand, *calls[1].Hand)
	}
}

func TestSummarizeTokensSlope(t *testing.T) {
	calls := decisionLog().TokenCalls("decision")
	summaries := summarizeTokens(map[string][]TokenCall{"agent-a": calls})
	if len(summaries) != 1 {
		t.Fatalf("summaries = %d, want 1", len(summaries))
	}
	s := summaries[0]
	if s.Decisions != 2 {
		t.Errorf("decisions = %d, want 2", s.Decisions)
	}
	// points (1,500),(2,650) -> slope 150
	if s.Slope == nil || math.Abs(*s.Slope-150) > 1e-9 {
		t.Errorf("slope = %v, want 150", s.Slope)
	}
	if s.TotalTokens != 1220 {
		t.Errorf("total tokens = %d, want 1220", s.TotalTokens)
	}
}

func TestLinregressSlopeDegenerate(t *testing.T) {
	if slope, n := linregressSlope([][2]float64{{1, 5}}); slope != nil || n != 1 {
		t.Errorf("single point: slope=%v n=%d, want nil/1", slope, n)
	}
	// vertical (all x equal) -> zero denom -> nil slope
	if slope, _ := linregressSlope([][2]float64{{3, 1}, {3, 9}}); slope != nil {
		t.Errorf("zero-variance x: slope=%v, want nil", slope)
	}
}
