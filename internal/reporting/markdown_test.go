package reporting

import (
	"os"
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
	data, err := os.ReadFile("testdata/benchmark_report.golden.md")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := string(data)
	if got != want {
		t.Fatalf("RenderMarkdown() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
