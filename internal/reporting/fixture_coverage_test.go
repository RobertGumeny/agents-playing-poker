package reporting

import (
	"strings"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

func TestDeterministicBenchmarkFixtureCoverage(t *testing.T) {
	sessions := benchmarkFixtureSessions()
	agg := ComputeAggregate(sessions)

	if len(agg.MirrorPairs) != 1 {
		t.Fatalf("mirror pairs = %d, want 1", len(agg.MirrorPairs))
	}
	if got := agg.UnpairedSessions; len(got) != 1 || got[0] != "fixture-unpaired" {
		t.Fatalf("unpaired sessions = %#v, want fixture-unpaired", got)
	}
	if !hasWarning(agg.Warnings, WarningHistoricalStrategyCanonicalized) || !hasWarning(agg.Warnings, WarningFallbackActionsPresent) || !hasWarning(agg.Warnings, WarningMirrorUnpaired) {
		t.Fatalf("warnings = %#v, want historical canonicalization, fallback, and unpaired warnings", agg.Warnings)
	}

	akg := agg.Strategies[CanonicalAKGStrategy]
	if akg == nil {
		t.Fatalf("missing %s metrics", CanonicalAKGStrategy)
	}
	if akg.TotalDelta != 9 || akg.Hands != 5 || akg.SessionWins != 2 || akg.MirrorPairWins != 1 {
		t.Fatalf("akg totals = %#v, want delta +9 hands 5 session wins 2 mirror wins 1", akg)
	}
	if akg.ShowdownDelta != 6 || akg.NonShowdownDelta != 3 {
		t.Fatalf("akg showdown/non-showdown = %d/%d, want +6/+3", akg.ShowdownDelta, akg.NonShowdownDelta)
	}
	if akg.ActionCounts["raise"] != 1 || akg.ActionCounts["auto_check"] != 1 || akg.FallbackActionCount != 1 {
		t.Fatalf("akg action/fallback counts = %#v/%d", akg.ActionCounts, akg.FallbackActionCount)
	}
	if akg.BBPer100 != 90 {
		t.Fatalf("akg BB/100 = %.2f, want 90.00", akg.BBPer100)
	}

	stateless := agg.Strategies["llm-stateless"]
	if stateless.TotalDelta != -7 || stateless.ActionCounts["bet"] != 1 || stateless.ActionCounts["fold"] != 1 {
		t.Fatalf("stateless metrics = %#v", stateless)
	}

	markdown := RenderMarkdown("fixture", agg)
	for _, want := range []string{
		"historical_strategy_canonicalized",
		"fallback_actions_present",
		"mirror_unpaired",
		"| llm-akg-recent | +9 | 5 | 1.80 | 90.00 | 2 | 1 |",
		"| llm-akg-recent | +6 | +3 |",
		"| llm-akg-recent | 2 | 0 | 0 | 1 | 1 | 20.0% | 20.0% | 1 |",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing %q\n%s", want, markdown)
		}
	}
}

func benchmarkFixtureSessions() []Session {
	return []Session{
		normalizedTestSession("fixture-seed5-a", 5, HistoricalAKGStrategy, "llm-stateless", []sessionlog.HandRecord{
			testHand(1, false, []sessionlog.HandAction{{Seat: 0, Action: "raise", Amount: intPtr(6), Street: "preflop"}, {Seat: 1, Action: "fold", Street: "preflop"}}, 4, -4),
			testHand(2, true, []sessionlog.HandAction{{Seat: 1, Action: "call", Amount: intPtr(2), Street: "preflop"}, {Seat: 0, Action: "auto_check", Street: "river", ForcedReason: "decision_timeout"}}, -6, 6),
		}),
		normalizedTestSession("fixture-seed5-b", 5, "llm-stateless", CanonicalAKGStrategy, []sessionlog.HandRecord{
			testHand(1, false, []sessionlog.HandAction{{Seat: 0, Action: "bet", Amount: intPtr(5), Street: "flop"}, {Seat: 1, Action: "fold", Street: "flop"}}, 3, -3),
			testHand(2, true, []sessionlog.HandAction{{Seat: 0, Action: "call", Amount: intPtr(4), Street: "turn"}, {Seat: 1, Action: "call", Amount: intPtr(4), Street: "turn"}}, -12, 12),
		}),
		normalizedTestSession("fixture-unpaired", 6, "heuristic", CanonicalAKGStrategy, []sessionlog.HandRecord{
			testHand(1, false, []sessionlog.HandAction{{Seat: 0, Action: "check", Street: "flop"}, {Seat: 1, Action: "call", Amount: intPtr(2), Street: "flop"}}, -2, 2),
		}),
	}
}
