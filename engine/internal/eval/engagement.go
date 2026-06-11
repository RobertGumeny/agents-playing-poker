package eval

import "sort"

// retrievalStrategies are the agents whose memory design REQUIRES drilling into
// the store at decision time (slim index -> mandatory read tools before acting).
// For these, zero decision-time reads means the memory is write-only and the run
// is invalid. Non-retrieval strategies (stateless / fullhistory / akg-recent /
// md-single) inject their memory into the prompt and legitimately make zero
// reads, so flagging them would be noise. Ported from analyze.py's
// RETRIEVAL_STRATEGIES — the one strategy-name coupling the eval keeps, because
// "should this agent have read?" is not derivable from the trace alone.
var retrievalStrategies = map[string]bool{
	"llm-akg-durable": true,
	"llm-md-wiki":     true,
}

// IsRetrievalStrategy reports whether an agent is expected to read its memory at
// decision time (and is therefore subject to the zero-reads tripwire).
func IsRetrievalStrategy(name string) bool { return retrievalStrategies[name] }

// MemoryEngagement is the per-decision retrieval-read footprint of a decision
// trace. A "decision" is one user prompt plus the assistant turns that follow it;
// the agent reads, then emits its action as the terminal assistant text, so any
// read in a decision necessarily precedes the action — "read before acting"
// reduces to "the decision recorded >=1 read". Every tool call in a decision
// trace is a retrieval read (write tools are registered only in the post-hand
// update session), so the metric is substrate-neutral: it counts tool calls
// without a per-agent read-tool table.
type MemoryEngagement struct {
	Decisions         int            `json:"decisions"`
	DecisionsWithRead int            `json:"decisions_with_read"`
	TotalReads        int            `json:"total_reads"`
	ReadsByTool       map[string]int `json:"reads_by_tool,omitempty"`
}

// MemoryEngagement walks the decision trace, opening a new decision at each user
// message and attributing every subsequent assistant tool call to it. Works
// uniformly for decision-scope traces (one user prompt per Pi session) and
// hand-scope traces (several street decisions in one session): tool results
// carry their own "toolResult" role, so a "user" message is always a fresh
// decision prompt.
func (l PiSessionLog) MemoryEngagement() MemoryEngagement {
	m := MemoryEngagement{ReadsByTool: map[string]int{}}
	inDecision := false
	curReads := 0
	closeDecision := func() {
		if !inDecision {
			return
		}
		m.Decisions++
		if curReads > 0 {
			m.DecisionsWithRead++
		}
		curReads = 0
		inDecision = false
	}
	for _, ev := range l.Events {
		if ev.Type != "message" || ev.Message == nil {
			continue
		}
		switch ev.Message.Role {
		case "user":
			closeDecision()
			inDecision = true
		case "assistant":
			if !inDecision {
				continue
			}
			for _, item := range ev.Message.Content {
				if item.Type != "toolCall" || item.Name == "" {
					continue
				}
				curReads++
				m.TotalReads++
				m.ReadsByTool[item.Name]++
			}
		}
	}
	closeDecision()
	if len(m.ReadsByTool) == 0 {
		m.ReadsByTool = nil
	}
	return m
}

// EngagementAgentSummary is the per-agent engagement aggregate across every
// session an agent name appears in, plus the retrieval-tripwire verdict.
type EngagementAgentSummary struct {
	Agent             string
	Retrieval         bool
	Decisions         int
	DecisionsWithRead int
	TotalReads        int
	ReadsByTool       map[string]int
	// ZeroReadSessions lists session IDs where a retrieval agent recorded
	// decisions but zero reads — the load-bearing tripwire condition.
	ZeroReadSessions []string
}

// ReadsPerDecision is the mean retrieval reads per decision.
func (s EngagementAgentSummary) ReadsPerDecision() float64 {
	if s.Decisions == 0 {
		return 0
	}
	return float64(s.TotalReads) / float64(s.Decisions)
}

// ReadCoverage is the fraction of decisions with at least one read before acting.
func (s EngagementAgentSummary) ReadCoverage() float64 {
	if s.Decisions == 0 {
		return 0
	}
	return float64(s.DecisionsWithRead) / float64(s.Decisions)
}

// TripwireFail reports the hard-fail condition: an agent expected to retrieve
// recorded decisions but never opened its memory while deciding. It checks
// per-session truth (ZeroReadSessions) so one clean session can't mask a broken
// one in the aggregate; the TotalReads==0 clause covers a summary inspected in
// isolation, before per-session accumulation.
func (s EngagementAgentSummary) TripwireFail() bool {
	if !s.Retrieval {
		return false
	}
	return len(s.ZeroReadSessions) > 0 || (s.Decisions > 0 && s.TotalReads == 0)
}

// accumulateEngagement folds one seat's per-session engagement into the
// cross-session aggregate keyed by agent name, recording sessions where a
// retrieval agent recorded decisions but zero reads.
func accumulateEngagement(byAgent map[string]*EngagementAgentSummary, seat SeatSummary, sessionID string) {
	if seat.MemoryEngagement == nil {
		return
	}
	agg := byAgent[seat.Name]
	if agg == nil {
		agg = &EngagementAgentSummary{
			Agent:       seat.Name,
			Retrieval:   IsRetrievalStrategy(seat.Name),
			ReadsByTool: map[string]int{},
		}
		byAgent[seat.Name] = agg
	}
	e := seat.MemoryEngagement
	agg.Decisions += e.Decisions
	agg.DecisionsWithRead += e.DecisionsWithRead
	agg.TotalReads += e.TotalReads
	for name, count := range e.ReadsByTool {
		agg.ReadsByTool[name] += count
	}
	if agg.Retrieval && e.Decisions > 0 && e.TotalReads == 0 {
		agg.ZeroReadSessions = append(agg.ZeroReadSessions, sessionID)
	}
}

// finalizeEngagement returns the aggregates sorted by agent name.
func finalizeEngagement(byAgent map[string]*EngagementAgentSummary) []EngagementAgentSummary {
	out := make([]EngagementAgentSummary, 0, len(byAgent))
	for _, agg := range byAgent {
		if len(agg.ReadsByTool) == 0 {
			agg.ReadsByTool = nil
		}
		out = append(out, *agg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Agent < out[j].Agent })
	return out
}

// sortedToolReads returns the agent's read-tool names in stable order.
func (s EngagementAgentSummary) sortedToolReads() []string {
	names := make([]string, 0, len(s.ReadsByTool))
	for name := range s.ReadsByTool {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
