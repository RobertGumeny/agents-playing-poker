package eval

import (
	"math"
	"testing"
)

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
