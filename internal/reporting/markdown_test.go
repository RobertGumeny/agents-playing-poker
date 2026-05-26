package reporting

import (
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

func TestRenderMarkdownRepresentativeAggregate(t *testing.T) {
	sessions := []Session{
		normalizedTestSession("seed9-a", 9, "llm-akg", "llm-stateless", []sessionlog.HandRecord{
			testHand(1, false, []sessionlog.HandAction{{Seat: 0, Action: "raise", Amount: intPtr(6), Street: "preflop"}, {Seat: 1, Action: "fold", Street: "preflop"}}, 12, -12),
			testHand(2, true, []sessionlog.HandAction{{Seat: 1, Action: "call", Amount: intPtr(2), Street: "preflop"}, {Seat: 0, Action: "auto_check", Street: "river", ForcedReason: "decision_timeout"}}, -20, 20),
		}),
		normalizedTestSession("seed9-b", 9, "llm-stateless", "llm-akg-recent", []sessionlog.HandRecord{
			testHand(1, false, []sessionlog.HandAction{{Seat: 0, Action: "bet", Amount: intPtr(4), Street: "flop"}, {Seat: 1, Action: "fold", Street: "flop"}}, 6, -6),
			testHand(2, true, []sessionlog.HandAction{{Seat: 1, Action: "raise", Amount: intPtr(8), Street: "turn"}, {Seat: 0, Action: "call", Amount: intPtr(8), Street: "turn"}}, -10, 10),
		}),
		normalizedTestSession("unpaired", 10, "heuristic", "llm-akg-recent", []sessionlog.HandRecord{
			testHand(1, false, []sessionlog.HandAction{{Seat: 0, Action: "check", Street: "flop"}, {Seat: 1, Action: "bet", Amount: intPtr(5), Street: "flop"}, {Seat: 0, Action: "fold", Street: "flop"}}, -5, 5),
		}),
	}

	got := RenderMarkdown("akg-recent-vs-stateless", ComputeAggregate(sessions))
	want := `# Benchmark Review: akg-recent-vs-stateless

## Framing

This review is generated from server-authoritative ` + "`" + `manifest.json` + "`" + ` and ` + "`" + `hands.jsonl` + "`" + ` artifacts. It reports chip flow and behavior diagnostics for the selected sessions.

` + "`" + `llm-akg-recent` + "`" + ` is a shallow bounded-memory baseline that injects recent context; it is not the durable AKG thesis agent and this report should not be read as proof of durable AKG memory. Claims require mirror-corrected aggregates across enough seeds.

## Inputs

| Session | Directory | Seed | Hands | Seat 0 | Seat 1 | Validation |
|---|---:|---:|---:|---|---|---|
| seed9-a | seed9-a | 9 | 2 | llm-akg-recent | llm-stateless | ok |
| seed9-b | seed9-b | 9 | 2 | llm-stateless | llm-akg-recent | ok |
| unpaired | unpaired | 10 | 1 | heuristic | llm-akg-recent | warning |

Notes and warnings:
- Model/provider and strategy-version metadata are incomplete in current manifests; mirror validation cannot fully compare those fields.
- Usage/cost data is missing or not normalized, so the cost section is informational only.
- fallback_actions_present: session seed9-a contains 1 fallback actions
- historical_strategy_canonicalized: seat 0 strategy "llm-akg" normalized to "llm-akg-recent"
- mirror_unpaired: session unpaired has no mirror pair

## Headline

Across 1 mirror pair(s), ` + "`" + `llm-stateless` + "`" + ` led the selected aggregate by +4 chips. This is a conservative benchmark read: seat/card-stream variance, showdowns, fallbacks, and small sample size may dominate the result.

## Raw session results

| Session | Seed | Winner | Delta | Showdowns | Preflop-only | Biggest swing | Fallbacks |
|---|---:|---|---:|---:|---:|---:|---:|
| seed9-a | 9 | llm-stateless | +8 | 1 (50.0%) | 1 (50.0%) | +20 | 1 |
| seed9-b | 9 | llm-akg-recent | +4 | 1 (50.0%) | 0 (0.0%) | +10 | 0 |
| unpaired | 10 | llm-akg-recent | +5 | 0 (0.0%) | 0 (0.0%) | +5 | 0 |

## Mirror-corrected aggregate

| Strategy | Total chips | Hands | Chips/hand | BB/100 | Session wins | Mirror-pair wins |
|---|---:|---:|---:|---:|---:|---:|
| heuristic | -5 | 1 | -5.00 | -250.00 | 0 | 0 |
| llm-akg-recent | +1 | 5 | 0.20 | 10.00 | 2 | 0 |
| llm-stateless | +4 | 4 | 1.00 | 50.00 | 1 | 1 |

| Pair | Sessions | Winner | Delta | Status |
|---|---|---|---:|---|
| 9:llm-akg-recent+llm-stateless | seed9-a, seed9-b | llm-stateless | +4 | ok |

## Seat-bias check

| Seat | Total chips | Hands | Chips/hand | Session wins |
|---:|---:|---:|---:|---:|
| 0 | -17 | 5 | -3.40 | 0 |
| 1 | +17 | 5 | 3.40 | 3 |

Warning: seat 1 won every included session; raw outcomes may be dominated by seat/card-stream effects.

## Behavioral diagnostics

Showdown/non-showdown chip flow:

| Strategy | Showdown chips | Non-showdown chips |
|---|---:|---:|
| heuristic | +0 | -5 |
| llm-akg-recent | -10 | +11 |
| llm-stateless | +10 | -6 |

Action summary:

| Strategy | Calls | Checks | Bets | Raises | Folds | Aggression share | Fold share | Fallbacks |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| heuristic | 0 | 1 | 0 | 0 | 1 | 0.0% | 50.0% | 0 |
| llm-akg-recent | 0 | 0 | 1 | 2 | 1 | 60.0% | 20.0% | 1 |
| llm-stateless | 2 | 0 | 1 | 0 | 1 | 25.0% | 25.0% | 0 |

## Key hands

Largest chip swings are variance diagnostics, not cherry-picked proof.

| Session | Hand | Winner | Loser | Delta | Terminal | Showdown |
|---|---:|---|---|---:|---|---|
| seed9-a | 2 | llm-stateless (seat 1) | llm-akg-recent (seat 0) | +20 | showdown | true |
| seed9-a | 1 | llm-akg-recent (seat 0) | llm-stateless (seat 1) | +12 | preflop | false |
| seed9-b | 2 | llm-akg-recent (seat 1) | llm-stateless (seat 0) | +10 | showdown | true |
| seed9-b | 1 | llm-stateless (seat 0) | llm-akg-recent (seat 1) | +6 | flop | false |
| unpaired | 1 | llm-akg-recent (seat 1) | heuristic (seat 0) | +5 | flop | false |

## Cost and context efficiency

Usage/cost artifacts were not found or are not yet normalized for this aggregate, so token counts, model cost, context growth, and chips per token are not reported. This is a reporting gap, not a blocker for chip and behavior metrics.

## Interpretation

Use this report as baseline characterization. Prefer mirror-pair totals over raw session wins, and treat large showdown pots, seat effects, fallbacks, and missing metadata as caveats. Do not infer durable AKG superiority from ` + "`" + `llm-akg-recent` + "`" + `; it is only the shallow recent-memory baseline.
Fallback actions occurred, so some decisions were server-forced rather than strategy-selected.
One or more sessions were unpaired, so mirror correction is incomplete.

## Recommended next step

Add the missing mirrored counterpart sessions before making a strategy claim.
`
	if got != want {
		t.Fatalf("RenderMarkdown() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
