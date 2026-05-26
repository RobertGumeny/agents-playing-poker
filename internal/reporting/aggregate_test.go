package reporting

import (
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

func TestComputeAggregateMirroredSessions(t *testing.T) {
	sessions := []Session{
		normalizedTestSession("seed7-a", 7, "alpha", "beta", []sessionlog.HandRecord{
			testHand(1, false, []sessionlog.HandAction{{Seat: 0, Action: "raise", Amount: intPtr(6), Street: "preflop"}, {Seat: 1, Action: "fold", Street: "preflop"}}, 10, -10),
			testHand(2, true, []sessionlog.HandAction{{Seat: 1, Action: "call", Amount: intPtr(1), Street: "preflop"}, {Seat: 0, Action: "check", Street: "preflop"}, {Seat: 0, Action: "bet", Amount: intPtr(8), Street: "river"}, {Seat: 1, Action: "call", Amount: intPtr(8), Street: "river"}}, -30, 30),
		}),
		normalizedTestSession("seed7-b", 7, "beta", "alpha", []sessionlog.HandRecord{
			testHand(1, false, []sessionlog.HandAction{{Seat: 0, Action: "raise", Amount: intPtr(6), Street: "preflop"}, {Seat: 1, Action: "fold", Street: "preflop"}}, 4, -4),
			testHand(2, true, []sessionlog.HandAction{{Seat: 1, Action: "call", Amount: intPtr(1), Street: "preflop"}, {Seat: 0, Action: "check", Street: "preflop"}, {Seat: 0, Action: "bet", Amount: intPtr(8), Street: "turn"}, {Seat: 1, Action: "call", Amount: intPtr(8), Street: "turn"}}, -6, 6),
		}),
	}

	agg := ComputeAggregate(sessions)
	if len(agg.MirrorPairs) != 1 {
		t.Fatalf("mirror pairs = %d, want 1", len(agg.MirrorPairs))
	}
	pair := agg.MirrorPairs[0]
	if pair.ValidationStatus != "ok" || pair.WinnerStrategy != "beta" || pair.WinnerDelta != 18 {
		t.Fatalf("pair = %#v, want beta +18 ok", pair)
	}
	if got := agg.Strategies["beta"].MirrorPairWins; got != 1 {
		t.Fatalf("beta mirror wins = %d, want 1", got)
	}
	if got := agg.Strategies["alpha"].TotalDelta; got != -18 {
		t.Fatalf("alpha total delta = %d, want -18", got)
	}
}

func TestComputeAggregateSessionMetricsAndFallbacks(t *testing.T) {
	session := normalizedTestSession("fallbacks", 11, "alpha", "beta", []sessionlog.HandRecord{
		testHand(1, false, []sessionlog.HandAction{{Seat: 0, Action: "auto_fold", Street: "preflop", ForcedReason: "timeout"}}, -1, 1),
		testHand(2, true, []sessionlog.HandAction{{Seat: 0, Action: "call", Amount: intPtr(1), Street: "preflop"}, {Seat: 1, Action: "check", Street: "preflop"}}, 5, -5),
	})

	agg := ComputeAggregate([]Session{session})
	if len(agg.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(agg.Sessions))
	}
	metrics := agg.Sessions[0]
	if metrics.WinnerStrategy != "alpha" || metrics.WinnerDelta != 4 {
		t.Fatalf("winner = %s %+d, want alpha +4", metrics.WinnerStrategy, metrics.WinnerDelta)
	}
	if metrics.ShowdownCount != 1 || metrics.PreflopOnlyCount != 1 {
		t.Fatalf("showdown/preflop = %d/%d, want 1/1", metrics.ShowdownCount, metrics.PreflopOnlyCount)
	}
	if metrics.BiggestSwingHand.HandNumber != 2 || metrics.BiggestSwingHand.Delta != 5 {
		t.Fatalf("biggest swing = %#v, want hand 2 +5", metrics.BiggestSwingHand)
	}
	if metrics.FallbackCounts["auto_fold"] != 1 || agg.Strategies["alpha"].FallbackActionCount != 1 {
		t.Fatalf("fallback counts session=%#v strategy=%d", metrics.FallbackCounts, agg.Strategies["alpha"].FallbackActionCount)
	}
	if !hasWarning(agg.Warnings, WarningFallbackActionsPresent) || !hasWarning(agg.Warnings, WarningMirrorUnpaired) {
		t.Fatalf("warnings = %#v, want fallback and unpaired", agg.Warnings)
	}
}

func TestComputeAggregateSeatBiasActionsAndBBPer100(t *testing.T) {
	sessions := []Session{
		normalizedTestSession("s1", 1, "alpha", "beta", []sessionlog.HandRecord{
			testHand(1, false, []sessionlog.HandAction{{Seat: 0, Action: "bet", Amount: intPtr(4), Street: "flop"}, {Seat: 1, Action: "fold", Street: "flop"}}, 8, -8),
			testHand(2, true, []sessionlog.HandAction{{Seat: 0, Action: "raise", Amount: intPtr(8), Street: "preflop"}, {Seat: 1, Action: "call", Amount: intPtr(8), Street: "preflop"}}, -4, 4),
		}),
		normalizedTestSession("s2", 2, "gamma", "alpha", []sessionlog.HandRecord{
			testHand(1, false, []sessionlog.HandAction{{Seat: 1, Action: "check", Street: "flop"}, {Seat: 0, Action: "bet", Amount: intPtr(6), Street: "flop"}, {Seat: 1, Action: "fold", Street: "flop"}}, 6, -6),
		}),
	}

	agg := ComputeAggregate(sessions)
	alpha := agg.Strategies["alpha"]
	if alpha.TotalDelta != -2 || alpha.Hands != 3 {
		t.Fatalf("alpha total/hands = %d/%d, want -2/3", alpha.TotalDelta, alpha.Hands)
	}
	if want := float64(-2) / float64(2) / float64(3) * 100; alpha.BBPer100 != want {
		t.Fatalf("alpha BB/100 = %v, want %v", alpha.BBPer100, want)
	}
	if alpha.NonShowdownDelta != 2 || alpha.ShowdownDelta != -4 {
		t.Fatalf("alpha nonshowdown/showdown = %d/%d, want 2/-4", alpha.NonShowdownDelta, alpha.ShowdownDelta)
	}
	if agg.Seats[0].TotalDelta != 10 || agg.Seats[1].TotalDelta != -10 {
		t.Fatalf("seat totals = %#v, want seat0 +10 seat1 -10", agg.Seats)
	}
	if alpha.ActionCounts["bet"] != 1 || alpha.ActionCounts["raise"] != 1 || alpha.ActionCounts["fold"] != 1 {
		t.Fatalf("alpha action counts = %#v", alpha.ActionCounts)
	}
}

func TestValidateMirrorPairWarnings(t *testing.T) {
	a := normalizedTestSession("a", 1, "alpha", "beta", []sessionlog.HandRecord{testHand(1, false, nil, 1, -1)})
	b := normalizedTestSession("b", 2, "alpha", "gamma", []sessionlog.HandRecord{testHand(1, false, nil, 1, -1), testHand(2, false, nil, -1, 1)})
	b.Metadata.BigBlind = 4

	pair := ValidateMirrorPair(a, b)
	for _, code := range []WarningCode{WarningMirrorSeedMismatch, WarningMirrorHandCountMismatch, WarningMirrorGameSettingsMismatch, WarningMirrorStrategyPairMismatch, WarningMirrorSeatAssignmentMismatch} {
		if !hasWarning(pair.Warnings, code) {
			t.Fatalf("warnings = %#v, want %s", pair.Warnings, code)
		}
	}
}

func normalizedTestSession(sessionID string, seed int64, seat0, seat1 string, hands []sessionlog.HandRecord) Session {
	manifest := manifestFor(sessionID, seat0, seat1)
	manifest.Seed = seed
	manifest.HandCount = len(hands)
	manifest.Matches[0].Result = map[int]sessionlog.ManifestSeatResult{0: {ChipsDelta: totalSeatDelta(hands, 0)}, 1: {ChipsDelta: totalSeatDelta(hands, 1)}}
	return NormalizeSession(sessionID, manifest, hands)
}

func testHand(number int, showdown bool, actions []sessionlog.HandAction, seat0Delta, seat1Delta int) sessionlog.HandRecord {
	return sessionlog.HandRecord{MatchID: "match-1", HandNumber: number, DealerSeat: number % 2, Actions: actions, ShowdownReached: showdown, Result: []sessionlog.HandResult{{Seat: 0, ChipsDelta: seat0Delta}, {Seat: 1, ChipsDelta: seat1Delta}}}
}

func totalSeatDelta(hands []sessionlog.HandRecord, seat int) int {
	total := 0
	for _, hand := range hands {
		for _, result := range hand.Result {
			if result.Seat == seat {
				total += result.ChipsDelta
			}
		}
	}
	return total
}
