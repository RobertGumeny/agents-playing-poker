package benchreport

import (
	"fmt"
	"sort"
	"strings"
)

func RenderMarkdown(label string, agg Aggregate) string {
	if label == "" {
		label = "selected sessions"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Benchmark Review: %s\n\n", label)
	renderFraming(&b)
	renderInputs(&b, agg)
	renderHeadline(&b, agg)
	renderRawSessionResults(&b, agg)
	renderMirrorAggregate(&b, agg)
	renderSeatBias(&b, agg)
	renderBehavioralDiagnostics(&b, agg)
	renderKeyHands(&b, agg)
	renderCostEfficiency(&b)
	renderInterpretation(&b, agg)
	renderRecommendedNextStep(&b, agg)
	return b.String()
}

func renderFraming(b *strings.Builder) {
	b.WriteString("## Framing\n\n")
	b.WriteString("This review is generated from server-authoritative `manifest.json` and `hands.jsonl` artifacts. It reports chip flow and behavior diagnostics for the selected sessions.\n\n")
	b.WriteString("`llm-akg-recent` is a shallow bounded-memory baseline that injects recent context; it is not the durable AKG thesis agent and this report should not be read as proof of durable AKG memory. Claims require mirror-corrected aggregates across enough seeds.\n\n")
}

func renderInputs(b *strings.Builder, agg Aggregate) {
	b.WriteString("## Inputs\n\n")
	if len(agg.Sessions) == 0 {
		b.WriteString("No sessions were included.\n\n")
	} else {
		b.WriteString("| Session | Directory | Seed | Hands | Seat 0 | Seat 1 | Validation |\n")
		b.WriteString("|---|---:|---:|---:|---|---|---|\n")
		for _, session := range agg.Sessions {
			fmt.Fprintf(b, "| %s | %s | %d | %d | %s | %s | %s |\n", session.SessionID, session.Dir, session.Seed, session.HandCount, strategyAtSeat(session, 0), strategyAtSeat(session, 1), validationForSession(agg, session.SessionID))
		}
		b.WriteString("\n")
	}
	b.WriteString("Notes and warnings:\n")
	for _, note := range inputNotes(agg) {
		fmt.Fprintf(b, "- %s\n", note)
	}
	b.WriteString("\n")
}

func renderHeadline(b *strings.Builder, agg Aggregate) {
	b.WriteString("## Headline\n\n")
	if len(agg.MirrorPairs) == 0 {
		b.WriteString("No complete mirror pair was available, so the result is useful only as a raw-session smoke check. Treat any apparent strategy edge as provisional.\n\n")
		return
	}
	winner, delta := leadingStrategy(agg)
	if winner == "" || delta == 0 {
		b.WriteString("The mirror-corrected aggregate was effectively tied. The useful signal is behavioral shape and artifact quality, not a thesis claim.\n\n")
		return
	}
	fmt.Fprintf(b, "Across %d mirror pair(s), `%s` led the selected aggregate by %+d chips. This is a conservative benchmark read: seat/card-stream variance, showdowns, fallbacks, and small sample size may dominate the result.\n\n", len(agg.MirrorPairs), winner, delta)
}

func renderRawSessionResults(b *strings.Builder, agg Aggregate) {
	b.WriteString("## Raw session results\n\n")
	b.WriteString("| Session | Seed | Winner | Delta | Showdowns | Preflop-only | Biggest swing | Fallbacks |\n")
	b.WriteString("|---|---:|---|---:|---:|---:|---:|---:|\n")
	for _, session := range agg.Sessions {
		fmt.Fprintf(b, "| %s | %d | %s | %+d | %d (%.1f%%) | %d (%.1f%%) | %+d | %d |\n", session.SessionID, session.Seed, session.WinnerStrategy, session.WinnerDelta, session.ShowdownCount, pct(session.ShowdownRate), session.PreflopOnlyCount, pct(session.PreflopOnlyRate), session.BiggestSwingHand.Delta, totalFallbacks(session.FallbackCounts))
	}
	b.WriteString("\n")
}

func renderMirrorAggregate(b *strings.Builder, agg Aggregate) {
	b.WriteString("## Mirror-corrected aggregate\n\n")
	b.WriteString("| Strategy | Total chips | Hands | Chips/hand | BB/100 | Session wins | Mirror-pair wins |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|\n")
	for _, strategy := range sortedStrategies(agg.Strategies) {
		m := agg.Strategies[strategy]
		fmt.Fprintf(b, "| %s | %+d | %d | %.2f | %.2f | %d | %d |\n", strategy, m.TotalDelta, m.Hands, m.ChipsPerHand, m.BBPer100, m.SessionWins, m.MirrorPairWins)
	}
	b.WriteString("\n")
	if len(agg.MirrorPairs) == 0 {
		b.WriteString("No complete mirror-pair aggregate was available.\n\n")
		return
	}
	b.WriteString("| Pair | Sessions | Winner | Delta | Status |\n")
	b.WriteString("|---|---|---|---:|---|\n")
	for _, pair := range agg.MirrorPairs {
		fmt.Fprintf(b, "| %s | %s | %s | %+d | %s |\n", pair.Key, strings.Join(pair.SessionIDs, ", "), pair.WinnerStrategy, pair.WinnerDelta, pair.ValidationStatus)
	}
	b.WriteString("\n")
}

func renderSeatBias(b *strings.Builder, agg Aggregate) {
	b.WriteString("## Seat-bias check\n\n")
	b.WriteString("| Seat | Total chips | Hands | Chips/hand | Session wins |\n")
	b.WriteString("|---:|---:|---:|---:|---:|\n")
	for _, seat := range sortedSeats(agg.Seats) {
		m := agg.Seats[seat]
		fmt.Fprintf(b, "| %d | %+d | %d | %.2f | %d |\n", seat, m.TotalDelta, m.Hands, m.ChipsPerHand, m.SessionWins)
	}
	b.WriteString("\n")
	if len(agg.Sessions) > 0 {
		for _, seat := range sortedSeats(agg.Seats) {
			if agg.Seats[seat].SessionWins == len(agg.Sessions) {
				fmt.Fprintf(b, "Warning: seat %d won every included session; raw outcomes may be dominated by seat/card-stream effects.\n\n", seat)
				return
			}
		}
	}
	b.WriteString("No single seat won every included session. Continue to compare against mirror-corrected strategy totals rather than raw seat results.\n\n")
}

func renderBehavioralDiagnostics(b *strings.Builder, agg Aggregate) {
	b.WriteString("## Behavioral diagnostics\n\n")
	b.WriteString("Showdown/non-showdown chip flow:\n\n")
	b.WriteString("| Strategy | Showdown chips | Non-showdown chips |\n")
	b.WriteString("|---|---:|---:|\n")
	for _, strategy := range sortedStrategies(agg.Strategies) {
		m := agg.Strategies[strategy]
		fmt.Fprintf(b, "| %s | %+d | %+d |\n", strategy, m.ShowdownDelta, m.NonShowdownDelta)
	}
	b.WriteString("\nAction summary:\n\n")
	b.WriteString("| Strategy | Calls | Checks | Bets | Raises | Folds | Aggression share | Fold share | Fallbacks |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, strategy := range sortedStrategies(agg.Strategies) {
		m := agg.Strategies[strategy]
		calls, checks, bets, raises, folds := m.ActionCounts["call"], m.ActionCounts["check"], m.ActionCounts["bet"], m.ActionCounts["raise"], m.ActionCounts["fold"]
		nonBlind := calls + checks + bets + raises + folds + m.ActionCounts["auto_fold"] + m.ActionCounts["auto_check"]
		fmt.Fprintf(b, "| %s | %d | %d | %d | %d | %d | %.1f%% | %.1f%% | %d |\n", strategy, calls, checks, bets, raises, folds, pct(safeDiv(bets+raises, nonBlind)), pct(safeDiv(folds+m.ActionCounts["auto_fold"], nonBlind)), m.FallbackActionCount)
	}
	b.WriteString("\n")
}

func renderKeyHands(b *strings.Builder, agg Aggregate) {
	b.WriteString("## Key hands\n\n")
	if len(agg.TopSwingHands) == 0 {
		b.WriteString("No swing hands were available.\n\n")
		return
	}
	b.WriteString("Largest chip swings are variance diagnostics, not cherry-picked proof.\n\n")
	b.WriteString("| Session | Hand | Winner | Loser | Delta | Terminal | Showdown |\n")
	b.WriteString("|---|---:|---|---|---:|---|---|\n")
	for _, hand := range agg.TopSwingHands {
		fmt.Fprintf(b, "| %s | %d | %s (seat %d) | %s (seat %d) | %+d | %s | %t |\n", hand.SessionID, hand.HandNumber, hand.WinnerStrategy, hand.WinnerSeat, hand.LoserStrategy, hand.LoserSeat, hand.Delta, hand.TerminalStreet, hand.Showdown)
	}
	b.WriteString("\n")
}

func renderCostEfficiency(b *strings.Builder) {
	b.WriteString("## Cost and context efficiency\n\n")
	b.WriteString("Usage/cost artifacts were not found or are not yet normalized for this aggregate, so token counts, model cost, context growth, and chips per token are not reported. This is a reporting gap, not a blocker for chip and behavior metrics.\n\n")
}

func renderInterpretation(b *strings.Builder, agg Aggregate) {
	b.WriteString("## Interpretation\n\n")
	b.WriteString("Use this report as baseline characterization. Prefer mirror-pair totals over raw session wins, and treat large showdown pots, seat effects, fallbacks, and missing metadata as caveats. Do not infer durable AKG superiority from `llm-akg-recent`; it is only the shallow recent-memory baseline.\n")
	if warningPresent(agg.Warnings, WarningFallbackActionsPresent) {
		b.WriteString("Fallback actions occurred, so some decisions were server-forced rather than strategy-selected.\n")
	}
	if len(agg.UnpairedSessions) > 0 {
		b.WriteString("One or more sessions were unpaired, so mirror correction is incomplete.\n")
	}
	b.WriteString("\n")
}

func renderRecommendedNextStep(b *strings.Builder, agg Aggregate) {
	b.WriteString("## Recommended next step\n\n")
	if len(agg.UnpairedSessions) > 0 || len(agg.MirrorPairs) == 0 {
		b.WriteString("Add the missing mirrored counterpart sessions before making a strategy claim.\n")
		return
	}
	b.WriteString("Run additional mirrored seeds and add normalized usage/cost artifacts before comparing this shallow baseline to durable AKG candidates or `llm-fullhistory`.\n")
}

func inputNotes(agg Aggregate) []string {
	notes := []string{"Model/provider and strategy-version metadata are incomplete in current manifests; mirror validation cannot fully compare those fields.", "Usage/cost data is missing or not normalized, so the cost section is informational only."}
	for _, warning := range agg.Warnings {
		notes = append(notes, fmt.Sprintf("%s: %s", warning.Code, warning.Message))
	}
	if len(notes) == 0 {
		notes = append(notes, "No warnings.")
	}
	return notes
}

func validationForSession(agg Aggregate, sessionID string) string {
	for _, unpaired := range agg.UnpairedSessions {
		if unpaired == sessionID {
			return "warning"
		}
	}
	return "ok"
}

func strategyAtSeat(session SessionMetrics, seat int) string {
	if strategy, ok := session.SeatStrategies[seat]; ok {
		return strategy
	}
	return "unknown"
}

func leadingStrategy(agg Aggregate) (string, int) {
	var winner string
	var delta int
	for _, strategy := range sortedStrategies(agg.Strategies) {
		m := agg.Strategies[strategy]
		if winner == "" || m.TotalDelta > delta {
			winner, delta = strategy, m.TotalDelta
		}
	}
	return winner, delta
}

func warningPresent(warnings []ValidationWarning, code WarningCode) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func sortedStrategies(strategies map[string]*StrategyMetrics) []string {
	keys := make([]string, 0, len(strategies))
	for key := range strategies {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSeats(seats map[int]*SeatMetrics) []int {
	keys := make([]int, 0, len(seats))
	for key := range seats {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

func pct(rate float64) float64 {
	return rate * 100
}
