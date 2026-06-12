package replay

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/RobertGumeny/agent-poker/internal/eval"
	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

// BuildModel reads a finished session's artifacts into a ReplayModel. It is the
// gather half of the view-model seam: the only file access is eval.LoadSession;
// everything downstream operates on in-memory types. Alignment slack (a trace
// decision count that does not match the seat's action count) is reported as
// warnings rather than failing the build.
func BuildModel(sessionDir string) (ReplayModel, []string, error) {
	art, err := eval.LoadSession(sessionDir)
	if err != nil {
		return ReplayModel{}, nil, err
	}

	session := buildSessionMeta(art.Manifest)
	model := ReplayModel{
		Session: session,
		Hands:   buildHands(art.Hands, session.Blinds.BB),
	}

	var warnings []string
	for _, agent := range art.Agents {
		if agent.PiSession == nil {
			continue
		}
		decisions := segmentDecisions(agent.PiSession)
		warnings = append(warnings, attachOverlays(model.Hands, agent.Seat.Seat, agent.Seat.Name, decisions)...)
	}

	// Summary is best-effort: a derivation hiccup should not sink the table replay.
	if summary, err := eval.CollectSession(sessionDir); err != nil {
		warnings = append(warnings, fmt.Sprintf("summary unavailable: %v", err))
	} else {
		model.Summary = buildSummary(summary)
	}

	return model, warnings, nil
}

func buildSummary(s eval.Summary) SummaryView {
	out := SummaryView{
		HandCount:        s.Session.HandCount,
		ShowdownRate:     s.Metrics.ShowdownRate,
		PreflopOnlyRate:  s.Metrics.PreflopOnlyRate,
		BiggestSwingHand: s.Metrics.BiggestSwingHand.HandNumber,
		Seats:            make([]SeatSummaryView, 0, len(s.Seats)),
	}
	for _, seat := range s.Seats {
		sv := SeatSummaryView{
			Seat:       seat.Seat,
			Name:       seat.Name,
			ChipsDelta: seat.ChipsDelta,
		}
		if seat.MemoryEngagement != nil {
			sv.Decisions = seat.MemoryEngagement.Decisions
			sv.DecisionsWithRead = seat.MemoryEngagement.DecisionsWithRead
			sv.TotalReads = seat.MemoryEngagement.TotalReads
			if sv.Decisions > 0 {
				sv.ReadCoverage = float64(sv.DecisionsWithRead) / float64(sv.Decisions)
			}
		}
		for _, row := range seat.FidelityRows {
			sv.FidelityChecked++
			if row.Fabrication || row.CardError || row.BoardError {
				sv.FidelityErrors++
			}
		}
		for _, call := range seat.TokenCalls {
			if call.Kind != "decision" || call.Hand == nil {
				continue
			}
			sv.TotalTokens += call.TotalTokens
			sv.TotalCost += call.Cost
			sv.CostPoints = append(sv.CostPoints, CostPoint{
				Hand:         *call.Hand,
				PromptTokens: call.PromptTokens,
				Cost:         call.Cost,
			})
		}
		out.Seats = append(out.Seats, sv)
	}
	return out
}

func buildSessionMeta(m sessionlog.Manifest) SessionMeta {
	seats := make([]SeatMeta, 0, len(m.Matches[0].Seats))
	for _, s := range m.Matches[0].Seats {
		seats = append(seats, SeatMeta{Seat: s.Seat, Name: s.Name, Version: s.Version})
	}
	return SessionMeta{
		SessionID:     m.SessionID,
		Seed:          m.Seed,
		Blinds:        BlindMeta{SB: m.Blinds.SB, BB: m.Blinds.BB},
		StartingStack: m.StartingStack,
		Seats:         seats,
	}
}

func buildHands(hands []sessionlog.HandRecord, bigBlind int) []HandView {
	out := make([]HandView, 0, len(hands))
	for _, h := range hands {
		results := make(map[int]int, len(h.Result))
		for _, r := range h.Result {
			results[r.Seat] = r.ChipsDelta
		}
		actions := make([]ActionView, 0, len(h.Actions))
		for _, a := range h.Actions {
			actions = append(actions, ActionView{
				Seat:         a.Seat,
				Street:       a.Street,
				Action:       a.Action,
				Amount:       a.Amount,
				ForcedReason: a.ForcedReason,
			})
		}
		hv := HandView{
			HandNumber:      h.HandNumber,
			DealerSeat:      h.DealerSeat,
			StacksStart:     h.StacksStart,
			HoleCards:       h.HoleCards,
			Board:           h.Board,
			ShowdownReached: h.ShowdownReached,
			Results:         results,
			GrossPotSize:    h.GrossPotSize,
			Actions:         actions,
		}
		computeStates(&hv, bigBlind)
		out = append(out, hv)
	}
	return out
}

// computeStates reconstructs the running table state after each action, stamping
// ChipsAdded / Label / State onto every ActionView. It mirrors the commit logic in
// rules/game.go (~240–272): blind/call amounts are chips added, bet/raise amounts
// are the to-total (added = amount − the seat's street commitment). Reconstructing
// the pot here also frees the renderer from gross_pot_size, which is absent in
// archived sessions.
func computeStates(h *HandView, bigBlind int) {
	stacks := make(map[int]int, len(h.StacksStart))
	for seat, s := range h.StacksStart {
		stacks[seat] = s
	}
	streetCommitted := map[int]int{}
	folded := map[int]bool{}
	lastAction := map[int]string{}
	pot := 0
	curStreet := ""

	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Street != curStreet {
			streetCommitted = map[int]int{}
			curStreet = a.Street
		}
		amount := 0
		if a.Amount != nil {
			amount = *a.Amount
		}
		added := 0
		label := ""
		switch a.Action {
		case "post_blind":
			added = amount
			if amount >= bigBlind {
				label = fmt.Sprintf("BB %d", amount)
			} else {
				label = fmt.Sprintf("SB %d", amount)
			}
		case "call":
			added = amount
			label = "CALL"
		case "bet":
			added = amount
			label = fmt.Sprintf("BET %d", amount)
		case "raise":
			added = amount - streetCommitted[a.Seat]
			label = fmt.Sprintf("RAISE to %d", amount)
		case "check":
			label = "CHECK"
		case "fold":
			folded[a.Seat] = true
			label = "FOLD"
		default:
			label = strings.ToUpper(a.Action)
		}

		stacks[a.Seat] -= added
		streetCommitted[a.Seat] += added
		pot += added
		lastAction[a.Seat] = label

		a.ChipsAdded = added
		a.Label = label
		a.State = snapshotState(pot, stacks, streetCommitted, folded, lastAction)
	}
}

func snapshotState(pot int, stacks, streetCommitted map[int]int, folded map[int]bool, lastAction map[int]string) *TableState {
	seats := make(map[int]SeatState, len(stacks))
	for seat, stack := range stacks {
		seats[seat] = SeatState{
			Stack:           stack,
			StreetCommitted: streetCommitted[seat],
			Folded:          folded[seat],
			LastAction:      lastAction[seat],
		}
	}
	return &TableState{Pot: pot, Seats: seats}
}

// decision is one segmented decision from an agent's trace.
type decision struct {
	hand     *int
	street   string
	hasRead  bool
	thinking string
	recalled string
	gist     string
}

// segmentDecisions splits a decision trace into per-decision records. A decision
// is a user message carrying "Legal actions" (which excludes non-decision user
// turns) plus the assistant/toolResult turns up to the next such user message.
func segmentDecisions(log *eval.PiSessionLog) []decision {
	var out []decision
	var cur *decision
	finish := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, ev := range log.Events {
		if ev.Type != "message" || ev.Message == nil {
			continue
		}
		msg := ev.Message
		switch msg.Role {
		case "user":
			text := joinText(msg.Content)
			if !strings.Contains(text, "Legal actions") {
				continue
			}
			finish()
			h, s := eval.ParseDecisionHandStreet(text)
			cur = &decision{hand: h, street: s, gist: extractGist(text)}
		case "assistant":
			if cur == nil {
				continue
			}
			for _, item := range msg.Content {
				switch item.Type {
				case "toolCall":
					if item.Name != "" {
						cur.hasRead = true
					}
				case "thinking":
					if cur.thinking == "" && item.Thinking != "" {
						cur.thinking = item.Thinking
					}
				}
			}
		case "toolResult":
			if cur == nil {
				continue
			}
			if cur.recalled == "" {
				cur.recalled = extractRecalledFact(msg.Content)
			}
		}
	}
	finish()
	return out
}

// attachOverlays zips a seat's decisions onto its non-blind actions, in order,
// per hand. Mismatched counts are tolerated (zip the shorter, warn) so a single
// odd hand never crashes the build.
func attachOverlays(hands []HandView, seat int, agentName string, decisions []decision) []string {
	byHand := map[int][]decision{}
	for _, d := range decisions {
		if d.hand == nil {
			continue
		}
		byHand[*d.hand] = append(byHand[*d.hand], d)
	}

	var warnings []string
	for hi := range hands {
		h := &hands[hi]
		decs := byHand[h.HandNumber]
		var actionIdxs []int
		for ai, a := range h.Actions {
			if a.Seat == seat && a.Action != "post_blind" {
				actionIdxs = append(actionIdxs, ai)
			}
		}
		n := len(decs)
		if len(actionIdxs) < n {
			n = len(actionIdxs)
		}
		if len(decs) != len(actionIdxs) {
			warnings = append(warnings, fmt.Sprintf(
				"hand %d seat %d (%s): %d trace decisions vs %d non-blind actions; aligned first %d by order",
				h.HandNumber, seat, agentName, len(decs), len(actionIdxs), n))
		}
		for k := 0; k < n; k++ {
			h.Actions[actionIdxs[k]].Overlay = buildOverlay(decs[k])
		}
	}
	return warnings
}

func buildOverlay(d decision) *DecisionOverlay {
	indicator := MemoryDeciding
	if d.hasRead {
		indicator = MemoryRecalling
	}
	return &DecisionOverlay{
		MemoryIndicator:     indicator,
		ThinkingText:        d.thinking,
		RecalledFact:        d.recalled,
		InjectedSummaryGist: d.gist,
	}
}

func joinText(items []eval.PiSessionContentItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.Text)
	}
	return strings.Join(parts, " ")
}

// extractGist returns the injected opponent-memory summary that the server appends
// to the decision prompt, dropping the boilerplate lead-in line.
func extractGist(text string) string {
	const marker = "Your opponent summary"
	idx := strings.Index(text, marker)
	if idx < 0 {
		return ""
	}
	sub := text[idx:]
	if nl := strings.IndexByte(sub, '\n'); nl >= 0 {
		return strings.TrimSpace(sub[nl+1:])
	}
	return ""
}

// extractRecalledFact pulls the human-readable recalled fact from a toolResult.
// Memory tools return a JSON blob whose "body" field is the stored fact; when that
// shape does not apply (e.g. multi-node reads) it falls back to the raw text.
func extractRecalledFact(items []eval.PiSessionContentItem) string {
	for _, item := range items {
		if item.Text == "" {
			continue
		}
		var probe struct {
			Body string `json:"body"`
		}
		if err := json.Unmarshal([]byte(item.Text), &probe); err == nil && probe.Body != "" {
			return probe.Body
		}
		return strings.TrimSpace(item.Text)
	}
	return ""
}
