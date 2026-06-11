package eval

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// loadUpdateTokenCalls reads the optional post-hand update trace and extracts its
// token calls. Missing/unreadable files yield no rows (non-fatal).
func loadUpdateTokenCalls(agentDir string) []TokenCall {
	path := resolveAgentArtifact(agentDir, updateSessionFileName, legacyUpdateFileName)
	if path == "" {
		return nil
	}
	log, err := ReadPiSessionLog(path)
	if err != nil {
		return nil
	}
	return log.TokenCalls("update")
}

// TokenCall is one LLM call's token footprint, attributed to a hand. It mirrors
// the per-call rows the Python analyze.py produced; PromptTokens is the decision
// prompt context (input+cacheRead+cacheWrite of the first assistant turn), the
// size we chart growing with hand count.
type TokenCall struct {
	Kind         string  `json:"kind"` // "decision" or "update"
	Hand         *int    `json:"hand"`
	Street       string  `json:"street,omitempty"`
	PromptTokens int     `json:"prompt_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	Cost         float64 `json:"cost"`
}

var (
	handDecisionRE = regexp.MustCompile(`Hand:\s*(\d+)`)
	handUpdateRE   = regexp.MustCompile(`hand=(\d+)`)
	streetRE       = regexp.MustCompile(`Street:\s*(\w+)`)
)

// TokenCalls returns one row per Pi sub-session that carried assistant usage.
// Sub-sessions are delimited by {"type":"session"} lines; a preceding
// {"type":"hand_boundary"} marker carries the hand number for the group that
// follows. kind is "decision" or "update".
func (l PiSessionLog) TokenCalls(kind string) []TokenCall {
	var rows []TokenCall
	var cur []PiSessionEvent
	var markerHand *int
	flush := func() {
		if row, ok := buildTokenCall(cur, kind, markerHand); ok {
			rows = append(rows, row)
		}
		cur = nil
	}
	for _, ev := range l.Events {
		switch ev.Type {
		case "hand_boundary":
			flush() // close the prior group with its own marker before advancing
			h := ev.HandNumber
			markerHand = &h
		case "session":
			flush()
		default:
			cur = append(cur, ev)
		}
	}
	flush()
	return rows
}

// buildTokenCall builds one row from a sub-session's events. Hand attribution
// prefers the explicit hand_boundary marker; legacy traces without markers fall
// back to parsing the prompt prose ("Hand: N" / "hand=N"). Returns false when the
// sub-session carried no assistant usage.
func buildTokenCall(sub []PiSessionEvent, kind string, markerHand *int) (TokenCall, bool) {
	usages := assistantUsages(sub)
	if len(usages) == 0 {
		return TokenCall{}, false
	}
	text := firstUserText(sub)
	hand := markerHand
	if hand == nil {
		handRE := handDecisionRE
		if kind != "decision" {
			handRE = handUpdateRE
		}
		if m := handRE.FindStringSubmatch(text); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				hand = &n
			}
		}
	}
	street := ""
	if kind == "decision" {
		if m := streetRE.FindStringSubmatch(text); m != nil {
			street = m[1]
		}
	}
	row := TokenCall{
		Kind:         kind,
		Hand:         hand,
		Street:       street,
		PromptTokens: usages[0].Input + usages[0].CacheRead + usages[0].CacheWrite,
	}
	for _, u := range usages {
		row.TotalTokens += u.TotalTokens
		if u.Cost != nil {
			row.Cost += u.Cost.Total
		}
	}
	return row, true
}

func firstUserText(sub []PiSessionEvent) string {
	for _, ev := range sub {
		if ev.Message == nil || ev.Message.Role != "user" {
			continue
		}
		parts := make([]string, 0, len(ev.Message.Content))
		for _, item := range ev.Message.Content {
			parts = append(parts, item.Text)
		}
		return strings.Join(parts, " ")
	}
	return ""
}

func assistantUsages(sub []PiSessionEvent) []PiUsage {
	var out []PiUsage
	for _, ev := range sub {
		if ev.Message != nil && ev.Message.Role == "assistant" && ev.Message.Usage != nil {
			out = append(out, *ev.Message.Usage)
		}
	}
	return out
}

// TokenAgentSummary is the per-agent token aggregate rendered in the report.
type TokenAgentSummary struct {
	Agent        string
	Decisions    int
	Updates      int
	Slope        *float64 // least-squares tok/hand growth of decision prompt context
	SlopeN       int
	DecPromptAvg float64
	EarlyPrompt  *float64
	LatePrompt   *float64
	UpdPromptAvg float64
	TotalTokens  int
	TotalCost    float64
}

// summarizeTokens aggregates per-agent token calls into report rows, sorted by
// agent name. Mirrors analyze.py token_summary.
func summarizeTokens(byAgent map[string][]TokenCall) []TokenAgentSummary {
	out := make([]TokenAgentSummary, 0, len(byAgent))
	for agent, calls := range byAgent {
		s := TokenAgentSummary{Agent: agent}
		var decPrompts, updPrompts []int
		var points [][2]float64
		for _, c := range calls {
			s.TotalTokens += c.TotalTokens
			s.TotalCost += c.Cost
			switch c.Kind {
			case "decision":
				s.Decisions++
				decPrompts = append(decPrompts, c.PromptTokens)
				if c.Hand != nil {
					points = append(points, [2]float64{float64(*c.Hand), float64(c.PromptTokens)})
				}
			case "update":
				s.Updates++
				updPrompts = append(updPrompts, c.PromptTokens)
			}
		}
		s.Slope, s.SlopeN = linregressSlope(points)
		s.DecPromptAvg = meanInts(decPrompts)
		s.UpdPromptAvg = meanInts(updPrompts)
		s.EarlyPrompt = windowMean(calls, 0.0, 0.1)
		s.LatePrompt = windowMean(calls, 0.9, 1.0)
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Agent < out[j].Agent })
	return out
}

// linregressSlope returns the least-squares slope of y on x and the point count.
func linregressSlope(points [][2]float64) (*float64, int) {
	n := len(points)
	if n < 2 {
		return nil, n
	}
	var sx, sy, sxx, sxy float64
	for _, p := range points {
		sx += p[0]
		sy += p[1]
		sxx += p[0] * p[0]
		sxy += p[0] * p[1]
	}
	denom := float64(n)*sxx - sx*sx
	if denom == 0 {
		return nil, n
	}
	slope := (float64(n)*sxy - sx*sy) / denom
	return &slope, n
}

// windowMean returns the mean decision prompt size over a hand-number window
// [loFrac, hiFrac] of the observed hand span. Mirrors analyze.py _window_mean.
func windowMean(calls []TokenCall, loFrac, hiFrac float64) *float64 {
	var hands []int
	for _, c := range calls {
		if c.Kind == "decision" && c.Hand != nil {
			hands = append(hands, *c.Hand)
		}
	}
	if len(hands) == 0 {
		return nil
	}
	loH, hiH := hands[0], hands[0]
	for _, h := range hands {
		if h < loH {
			loH = h
		}
		if h > hiH {
			hiH = h
		}
	}
	span := float64(hiH - loH)
	var vals []int
	if span == 0 {
		for _, c := range calls {
			if c.Kind == "decision" && c.Hand != nil {
				vals = append(vals, c.PromptTokens)
			}
		}
	} else {
		lo := float64(loH) + loFrac*span
		hi := float64(loH) + hiFrac*span
		for _, c := range calls {
			if c.Kind == "decision" && c.Hand != nil {
				h := float64(*c.Hand)
				if h >= lo && h <= hi {
					vals = append(vals, c.PromptTokens)
				}
			}
		}
	}
	if len(vals) == 0 {
		return nil
	}
	m := meanInts(vals)
	return &m
}

func meanInts(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0
	for _, x := range xs {
		sum += x
	}
	return float64(sum) / float64(len(xs))
}
