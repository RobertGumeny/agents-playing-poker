package benchreport

import (
	"fmt"
	"sort"
	"strings"
)

type Aggregate struct {
	Sessions         []SessionMetrics
	Strategies       map[string]*StrategyMetrics
	Seats            map[int]*SeatMetrics
	MirrorPairs      []MirrorPairMetrics
	UnpairedSessions []string
	Warnings         []ValidationWarning
	TopSwingHands    []SwingHand
}

type SessionMetrics struct {
	SessionID        string
	Dir              string
	Seed             int64
	HandCount        int
	Variant          string
	InfoRealism      string
	StartingStack    int
	SmallBlind       int
	BigBlind         int
	SeatStrategies   map[int]string
	StrategyDeltas   map[string]int
	WinnerStrategy   string
	WinnerDelta      int
	ShowdownCount    int
	ShowdownRate     float64
	PreflopOnlyCount int
	PreflopOnlyRate  float64
	BiggestSwingHand SwingHand
	FallbackCounts   map[string]int
}

type StrategyMetrics struct {
	Strategy            string
	TotalDelta          int
	Hands               int
	BigBlindUnits       float64
	ChipsPerHand        float64
	BBPer100            float64
	SessionWins         int
	MirrorPairWins      int
	SeatDeltas          map[int]int
	ShowdownDelta       int
	NonShowdownDelta    int
	ActionCounts        map[string]int
	FallbackActionCount int
}

type SeatMetrics struct {
	Seat         int
	TotalDelta   int
	Hands        int
	ChipsPerHand float64
	SessionWins  int
}

type MirrorPairMetrics struct {
	Key              string
	Seed             int64
	SessionIDs       []string
	Strategies       []string
	StrategyDeltas   map[string]int
	WinnerStrategy   string
	WinnerDelta      int
	ValidationStatus string
	Warnings         []ValidationWarning
}

type SwingHand struct {
	SessionID      string
	HandNumber     int
	WinnerStrategy string
	LoserStrategy  string
	WinnerSeat     int
	LoserSeat      int
	Delta          int
	Showdown       bool
	TerminalStreet string
}

const (
	WarningMirrorUnpaired               WarningCode = "mirror_unpaired"
	WarningMirrorSeedMismatch           WarningCode = "mirror_seed_mismatch"
	WarningMirrorHandCountMismatch      WarningCode = "mirror_hand_count_mismatch"
	WarningMirrorGameSettingsMismatch   WarningCode = "mirror_game_settings_mismatch"
	WarningMirrorStrategyPairMismatch   WarningCode = "mirror_strategy_pair_mismatch"
	WarningMirrorSeatAssignmentMismatch WarningCode = "mirror_seat_assignment_mismatch"
	WarningFallbackActionsPresent       WarningCode = "fallback_actions_present"
)

func ComputeAggregate(sessions []Session) Aggregate {
	agg := Aggregate{
		Sessions:         make([]SessionMetrics, 0, len(sessions)),
		Strategies:       make(map[string]*StrategyMetrics),
		Seats:            make(map[int]*SeatMetrics),
		UnpairedSessions: nil,
		Warnings:         nil,
	}

	for _, session := range sessions {
		metrics := computeSessionMetrics(session)
		agg.Sessions = append(agg.Sessions, metrics)
		agg.Warnings = append(agg.Warnings, session.Warnings...)
		if totalFallbacks(metrics.FallbackCounts) > 0 {
			agg.Warnings = append(agg.Warnings, ValidationWarning{Code: WarningFallbackActionsPresent, Message: fmt.Sprintf("session %s contains %d fallback actions", metrics.SessionID, totalFallbacks(metrics.FallbackCounts))})
		}
		for _, hand := range session.Hands {
			for _, delta := range hand.Deltas {
				strategy := ensureStrategyMetric(agg.Strategies, delta.Strategy, metrics.BigBlind)
				strategy.TotalDelta += delta.ChipsDelta
				strategy.Hands++
				if metrics.BigBlind > 0 {
					strategy.BigBlindUnits += float64(delta.ChipsDelta) / float64(metrics.BigBlind)
				}
				strategy.SeatDeltas[delta.Seat] += delta.ChipsDelta
				if hand.ShowdownReached {
					strategy.ShowdownDelta += delta.ChipsDelta
				} else {
					strategy.NonShowdownDelta += delta.ChipsDelta
				}

				seat := ensureSeatMetric(agg.Seats, delta.Seat)
				seat.TotalDelta += delta.ChipsDelta
				seat.Hands++
			}
			for _, action := range hand.Actions {
				strategy := ensureStrategyMetric(agg.Strategies, action.Strategy, metrics.BigBlind)
				strategy.ActionCounts[action.Action]++
				if action.Fallback {
					strategy.FallbackActionCount++
				}
			}
			if swing, ok := swingForHand(session, hand); ok {
				agg.TopSwingHands = append(agg.TopSwingHands, swing)
			}
		}
		if metrics.WinnerStrategy != "" {
			ensureStrategyMetric(agg.Strategies, metrics.WinnerStrategy, metrics.BigBlind).SessionWins++
			if winnerSeat, ok := winnerSeat(session, metrics.WinnerStrategy); ok {
				ensureSeatMetric(agg.Seats, winnerSeat).SessionWins++
			}
		}
	}

	agg.MirrorPairs, agg.UnpairedSessions = validateMirrorPairs(agg.Sessions)
	for _, sessionID := range agg.UnpairedSessions {
		agg.Warnings = append(agg.Warnings, ValidationWarning{Code: WarningMirrorUnpaired, Message: fmt.Sprintf("session %s has no mirror pair", sessionID)})
	}
	for _, pair := range agg.MirrorPairs {
		agg.Warnings = append(agg.Warnings, pair.Warnings...)
		if pair.WinnerStrategy != "" {
			ensureStrategyMetric(agg.Strategies, pair.WinnerStrategy, 0).MirrorPairWins++
		}
	}

	for _, strategy := range agg.Strategies {
		strategy.ChipsPerHand = safeDiv(strategy.TotalDelta, strategy.Hands)
		if strategy.Hands > 0 {
			strategy.BBPer100 = strategy.BigBlindUnits / float64(strategy.Hands) * 100
		}
	}
	for _, seat := range agg.Seats {
		seat.ChipsPerHand = safeDiv(seat.TotalDelta, seat.Hands)
	}
	sortAggregate(&agg)
	return agg
}

func computeSessionMetrics(session Session) SessionMetrics {
	metrics := SessionMetrics{
		SessionID:      session.Metadata.SessionID,
		Dir:            session.Dir,
		Seed:           session.Metadata.Seed,
		HandCount:      len(session.Hands),
		Variant:        session.Metadata.Variant,
		InfoRealism:    session.Metadata.InfoRealism,
		StartingStack:  session.Metadata.StartingStack,
		SmallBlind:     session.Metadata.SmallBlind,
		BigBlind:       session.Metadata.BigBlind,
		SeatStrategies: make(map[int]string),
		StrategyDeltas: make(map[string]int),
		FallbackCounts: make(map[string]int),
	}
	for seat, seatInfo := range session.Seats {
		metrics.SeatStrategies[seat] = seatInfo.Strategy
	}
	for _, hand := range session.Hands {
		if hand.ShowdownReached {
			metrics.ShowdownCount++
		}
		if terminalStreet(hand) == "preflop" {
			metrics.PreflopOnlyCount++
		}
		if swing, ok := swingForHand(session, hand); ok && absInt(swing.Delta) > absInt(metrics.BiggestSwingHand.Delta) {
			metrics.BiggestSwingHand = swing
		}
		for _, delta := range hand.Deltas {
			metrics.StrategyDeltas[delta.Strategy] += delta.ChipsDelta
		}
		for _, action := range hand.Actions {
			if action.Fallback {
				metrics.FallbackCounts[action.Action]++
			}
		}
	}
	metrics.ShowdownRate = safeDiv(metrics.ShowdownCount, metrics.HandCount)
	metrics.PreflopOnlyRate = safeDiv(metrics.PreflopOnlyCount, metrics.HandCount)
	for strategy, delta := range metrics.StrategyDeltas {
		if metrics.WinnerStrategy == "" || delta > metrics.WinnerDelta || (delta == metrics.WinnerDelta && strategy < metrics.WinnerStrategy) {
			metrics.WinnerStrategy = strategy
			metrics.WinnerDelta = delta
		}
	}
	return metrics
}

func ValidateMirrorPair(a, b Session) MirrorPairMetrics {
	return buildMirrorPair(computeSessionMetrics(a), computeSessionMetrics(b))
}

func validateMirrorPairs(sessions []SessionMetrics) ([]MirrorPairMetrics, []string) {
	groups := make(map[string][]SessionMetrics)
	for _, session := range sessions {
		groups[mirrorLooseKey(session)] = append(groups[mirrorLooseKey(session)], session)
	}
	var pairs []MirrorPairMetrics
	var unpaired []string
	for _, group := range groups {
		if len(group) != 2 {
			for _, session := range group {
				unpaired = append(unpaired, session.SessionID)
			}
			continue
		}
		pairs = append(pairs, buildMirrorPair(group[0], group[1]))
	}
	for _, id := range unpaired {
		_ = id
	}
	sort.Strings(unpaired)
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Key < pairs[j].Key })
	return pairs, unpaired
}

func buildMirrorPair(a, b SessionMetrics) MirrorPairMetrics {
	pair := MirrorPairMetrics{Key: mirrorLooseKey(a), Seed: a.Seed, SessionIDs: []string{a.SessionID, b.SessionID}, Strategies: strategyPair(a), StrategyDeltas: make(map[string]int), ValidationStatus: "ok"}
	if a.Seed != b.Seed {
		pair.Warnings = append(pair.Warnings, ValidationWarning{Code: WarningMirrorSeedMismatch, Message: fmt.Sprintf("sessions %s and %s have different seeds", a.SessionID, b.SessionID)})
	}
	if a.HandCount != b.HandCount {
		pair.Warnings = append(pair.Warnings, ValidationWarning{Code: WarningMirrorHandCountMismatch, Message: fmt.Sprintf("sessions %s and %s have different hand counts", a.SessionID, b.SessionID)})
	}
	if !sameGameSettings(a, b) {
		pair.Warnings = append(pair.Warnings, ValidationWarning{Code: WarningMirrorGameSettingsMismatch, Message: fmt.Sprintf("sessions %s and %s have different game settings", a.SessionID, b.SessionID)})
	}
	if strings.Join(strategyPair(a), ",") != strings.Join(strategyPair(b), ",") {
		pair.Warnings = append(pair.Warnings, ValidationWarning{Code: WarningMirrorStrategyPairMismatch, Message: fmt.Sprintf("sessions %s and %s have different strategy pairs", a.SessionID, b.SessionID)})
	}
	if !oppositeSeats(a, b) {
		pair.Warnings = append(pair.Warnings, ValidationWarning{Code: WarningMirrorSeatAssignmentMismatch, Message: fmt.Sprintf("sessions %s and %s do not have opposite seat assignment", a.SessionID, b.SessionID)})
	}
	if len(pair.Warnings) > 0 {
		pair.ValidationStatus = "warning"
	}
	for _, session := range []SessionMetrics{a, b} {
		for strategy, delta := range session.StrategyDeltas {
			pair.StrategyDeltas[strategy] += delta
		}
	}
	for strategy, delta := range pair.StrategyDeltas {
		if pair.WinnerStrategy == "" || delta > pair.WinnerDelta || (delta == pair.WinnerDelta && strategy < pair.WinnerStrategy) {
			pair.WinnerStrategy = strategy
			pair.WinnerDelta = delta
		}
	}
	return pair
}

func mirrorLooseKey(session SessionMetrics) string {
	return fmt.Sprintf("%d:%s", session.Seed, strings.Join(strategyPair(session), "+"))
}

func strategyPair(session SessionMetrics) []string {
	strategies := make([]string, 0, len(session.SeatStrategies))
	seen := make(map[string]bool)
	for _, strategy := range session.SeatStrategies {
		if !seen[strategy] {
			strategies = append(strategies, strategy)
			seen[strategy] = true
		}
	}
	sort.Strings(strategies)
	return strategies
}

func sameGameSettings(a, b SessionMetrics) bool {
	return a.Variant == b.Variant && a.InfoRealism == b.InfoRealism && a.StartingStack == b.StartingStack && a.SmallBlind == b.SmallBlind && a.BigBlind == b.BigBlind
}

func oppositeSeats(a, b SessionMetrics) bool {
	if len(a.SeatStrategies) != len(b.SeatStrategies) || len(a.SeatStrategies) == 0 {
		return false
	}
	for seat, strategy := range a.SeatStrategies {
		otherSeat := 1 - seat
		if b.SeatStrategies[otherSeat] != strategy {
			return false
		}
	}
	return true
}

func swingForHand(session Session, hand Hand) (SwingHand, bool) {
	var winner, loser HandDelta
	foundWinner, foundLoser := false, false
	for _, delta := range hand.Deltas {
		if !foundWinner || delta.ChipsDelta > winner.ChipsDelta {
			winner = delta
			foundWinner = true
		}
		if !foundLoser || delta.ChipsDelta < loser.ChipsDelta {
			loser = delta
			foundLoser = true
		}
	}
	if !foundWinner || winner.ChipsDelta <= 0 {
		return SwingHand{}, false
	}
	return SwingHand{SessionID: session.Metadata.SessionID, HandNumber: hand.HandNumber, WinnerStrategy: winner.Strategy, LoserStrategy: loser.Strategy, WinnerSeat: winner.Seat, LoserSeat: loser.Seat, Delta: winner.ChipsDelta, Showdown: hand.ShowdownReached, TerminalStreet: terminalStreet(hand)}, true
}

func terminalStreet(hand Hand) string {
	if hand.ShowdownReached {
		return "showdown"
	}
	order := []string{"preflop", "flop", "turn", "river"}
	seen := make(map[string]bool, len(hand.Actions))
	for _, action := range hand.Actions {
		seen[action.Street] = true
	}
	last := "preflop"
	for _, street := range order {
		if seen[street] {
			last = street
		}
	}
	return last
}

func ensureStrategyMetric(metrics map[string]*StrategyMetrics, strategy string, _ int) *StrategyMetrics {
	if metrics[strategy] == nil {
		metrics[strategy] = &StrategyMetrics{Strategy: strategy, SeatDeltas: make(map[int]int), ActionCounts: make(map[string]int)}
	}
	return metrics[strategy]
}

func ensureSeatMetric(metrics map[int]*SeatMetrics, seat int) *SeatMetrics {
	if metrics[seat] == nil {
		metrics[seat] = &SeatMetrics{Seat: seat}
	}
	return metrics[seat]
}

func winnerSeat(session Session, strategy string) (int, bool) {
	for seat, seatInfo := range session.Seats {
		if seatInfo.Strategy == strategy {
			return seat, true
		}
	}
	return 0, false
}

func totalFallbacks(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func safeDiv(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func sortAggregate(agg *Aggregate) {
	sort.Slice(agg.Sessions, func(i, j int) bool { return agg.Sessions[i].SessionID < agg.Sessions[j].SessionID })
	sort.Slice(agg.TopSwingHands, func(i, j int) bool {
		if absInt(agg.TopSwingHands[i].Delta) == absInt(agg.TopSwingHands[j].Delta) {
			return agg.TopSwingHands[i].SessionID < agg.TopSwingHands[j].SessionID
		}
		return absInt(agg.TopSwingHands[i].Delta) > absInt(agg.TopSwingHands[j].Delta)
	})
	if len(agg.TopSwingHands) > 10 {
		agg.TopSwingHands = agg.TopSwingHands[:10]
	}
	sort.Slice(agg.Warnings, func(i, j int) bool {
		if agg.Warnings[i].Code == agg.Warnings[j].Code {
			return agg.Warnings[i].Message < agg.Warnings[j].Message
		}
		return agg.Warnings[i].Code < agg.Warnings[j].Code
	})
}
