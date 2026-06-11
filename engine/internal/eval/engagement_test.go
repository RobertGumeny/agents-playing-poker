package eval

import "testing"

func userMsg(text string) PiSessionEvent {
	return PiSessionEvent{Type: "message", Message: &PiSessionMessage{
		Role: "user", Content: []PiSessionContentItem{{Type: "text", Text: text}},
	}}
}

func assistantReads(tools ...string) PiSessionEvent {
	content := make([]PiSessionContentItem, 0, len(tools)+1)
	content = append(content, PiSessionContentItem{Type: "thinking", Text: "..."})
	for _, t := range tools {
		content = append(content, PiSessionContentItem{Type: "toolCall", Name: t})
	}
	return PiSessionEvent{Type: "message", Message: &PiSessionMessage{Role: "assistant", Content: content}}
}

func assistantAction(text string) PiSessionEvent {
	return PiSessionEvent{Type: "message", Message: &PiSessionMessage{
		Role: "assistant", Content: []PiSessionContentItem{{Type: "text", Text: text}},
	}}
}

func toolResult() PiSessionEvent {
	return PiSessionEvent{Type: "message", Message: &PiSessionMessage{
		Role: "toolResult", Content: []PiSessionContentItem{{Type: "text", Text: "node body"}},
	}}
}

// TestMemoryEngagementHandScope exercises the load-bearing seam: a single
// hand-scope Pi session carrying three street decisions, where reads are
// attributed to the decision whose user prompt they follow — and a toolResult
// (its own role) must not open a new decision.
func TestMemoryEngagementHandScope(t *testing.T) {
	log := PiSessionLog{Events: []PiSessionEvent{
		{Type: "hand_boundary", HandNumber: 4},
		{Type: "session"},
		// Decision 1: two reads, then act.
		userMsg("Street: preflop"),
		assistantReads("akg_get_node", "akg_get_nodes"),
		toolResult(),
		assistantAction(`{"action":"call"}`),
		// Decision 2: zero reads (trivial fold), straight to act.
		userMsg("Street: flop"),
		assistantAction(`{"action":"fold"}`),
		// Decision 3: one read, then act.
		userMsg("Street: turn"),
		assistantReads("akg_get_node"),
		toolResult(),
		assistantAction(`{"action":"check"}`),
	}}
	m := log.MemoryEngagement()
	if m.Decisions != 3 {
		t.Errorf("decisions = %d, want 3", m.Decisions)
	}
	if m.DecisionsWithRead != 2 {
		t.Errorf("decisions_with_read = %d, want 2", m.DecisionsWithRead)
	}
	if m.TotalReads != 3 {
		t.Errorf("total_reads = %d, want 3", m.TotalReads)
	}
	if m.ReadsByTool["akg_get_node"] != 2 || m.ReadsByTool["akg_get_nodes"] != 1 {
		t.Errorf("reads_by_tool = %v, want akg_get_node:2 akg_get_nodes:1", m.ReadsByTool)
	}
}

// TestMemoryEngagementNoReads is the tripwire shape: a retrieval-capable trace
// that never opened memory.
func TestMemoryEngagementNoReads(t *testing.T) {
	log := PiSessionLog{Events: []PiSessionEvent{
		{Type: "session"},
		userMsg("Street: preflop"),
		assistantAction(`{"action":"fold"}`),
		userMsg("Street: preflop"),
		assistantAction(`{"action":"fold"}`),
	}}
	m := log.MemoryEngagement()
	if m.Decisions != 2 || m.DecisionsWithRead != 0 || m.TotalReads != 0 {
		t.Errorf("got %+v, want 2 decisions / 0 with-read / 0 reads", m)
	}
	if m.ReadsByTool != nil {
		t.Errorf("reads_by_tool = %v, want nil when no reads", m.ReadsByTool)
	}
}

func TestTripwireFail(t *testing.T) {
	tests := []struct {
		name string
		s    EngagementAgentSummary
		want bool
	}{
		{"retrieval zero reads fails", EngagementAgentSummary{Retrieval: true, Decisions: 30, TotalReads: 0}, true},
		{"retrieval with reads passes", EngagementAgentSummary{Retrieval: true, Decisions: 30, TotalReads: 12}, false},
		{"retrieval no decisions skipped", EngagementAgentSummary{Retrieval: true, Decisions: 0, TotalReads: 0}, false},
		{"non-retrieval zero reads ok", EngagementAgentSummary{Retrieval: false, Decisions: 30, TotalReads: 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.TripwireFail(); got != tt.want {
				t.Errorf("TripwireFail() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRetrievalStrategy(t *testing.T) {
	for _, name := range []string{"llm-akg-durable", "llm-md-wiki"} {
		if !IsRetrievalStrategy(name) {
			t.Errorf("%s should be a retrieval strategy", name)
		}
	}
	for _, name := range []string{"llm-md-single", "llm-stateless", "llm-fullhistory", "llm-akg-recent"} {
		if IsRetrievalStrategy(name) {
			t.Errorf("%s should not be a retrieval strategy", name)
		}
	}
}

// TestAccumulateEngagement verifies cross-session aggregation and that a
// retrieval agent's zero-read session is recorded even when another session reads
// (sum-then-check would mask it).
func TestAccumulateEngagement(t *testing.T) {
	byAgent := map[string]*EngagementAgentSummary{}
	good := SeatSummary{Name: "llm-md-wiki", MemoryEngagement: &MemoryEngagement{
		Decisions: 20, DecisionsWithRead: 18, TotalReads: 25, ReadsByTool: map[string]int{"md_read_page": 25},
	}}
	bad := SeatSummary{Name: "llm-md-wiki", MemoryEngagement: &MemoryEngagement{
		Decisions: 20, DecisionsWithRead: 0, TotalReads: 0,
	}}
	accumulateEngagement(byAgent, good, "wiki-vs-akg-1")
	accumulateEngagement(byAgent, bad, "wiki-vs-akg-2")

	agents := finalizeEngagement(byAgent)
	if len(agents) != 1 {
		t.Fatalf("agents = %d, want 1", len(agents))
	}
	a := agents[0]
	if a.Decisions != 40 || a.TotalReads != 25 {
		t.Errorf("aggregate = %d decisions / %d reads, want 40/25", a.Decisions, a.TotalReads)
	}
	if len(a.ZeroReadSessions) != 1 || a.ZeroReadSessions[0] != "wiki-vs-akg-2" {
		t.Errorf("zero_read_sessions = %v, want [wiki-vs-akg-2]", a.ZeroReadSessions)
	}
	if !a.TripwireFail() {
		t.Error("aggregate with a zero-read session should still register a tripwire candidate")
	}
}

// TestAccumulateEngagementNonRetrieval confirms a prompt-injection agent with
// zero reads never registers a zero-read session.
func TestAccumulateEngagementNonRetrieval(t *testing.T) {
	byAgent := map[string]*EngagementAgentSummary{}
	seat := SeatSummary{Name: "llm-md-single", MemoryEngagement: &MemoryEngagement{Decisions: 50, TotalReads: 0}}
	accumulateEngagement(byAgent, seat, "mdsingle-vs-akg-1")
	agents := finalizeEngagement(byAgent)
	if len(agents) != 1 {
		t.Fatalf("agents = %d, want 1", len(agents))
	}
	if agents[0].Retrieval || len(agents[0].ZeroReadSessions) != 0 || agents[0].TripwireFail() {
		t.Errorf("non-retrieval agent should not flag: %+v", agents[0])
	}
}
