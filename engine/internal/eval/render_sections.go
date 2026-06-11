package eval

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// summarizeFidelity aggregates per-agent fidelity rows (across sessions) into
// report rows, sorted by agent name. Agents with no extractable claims are
// omitted (their absence is itself the auditability point).
func summarizeFidelity(byAgent map[string][]FidelityRow) []FidelityAgentSummary {
	out := make([]FidelityAgentSummary, 0, len(byAgent))
	for agent, rows := range byAgent {
		if len(rows) == 0 {
			continue
		}
		out = append(out, FidelityAgentSummary{
			Agent:   agent,
			Overall: aggFidelity(rows),
			Buckets: driftBuckets(rows),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Agent < out[j].Agent })
	return out
}

func appendTokenSection(b *strings.Builder, agents []TokenAgentSummary) {
	hasData := false
	for _, a := range agents {
		if a.Decisions > 0 || a.Updates > 0 {
			hasData = true
			break
		}
	}
	if !hasData {
		return
	}
	fmt.Fprintf(b, "## Token Cost\n\n")
	fmt.Fprintf(b, "Decision prompt context (input+cacheRead+cacheWrite) vs. hand count. "+
		"Slope is least-squares growth per hand — ≈flat is a structured-memory cost win; "+
		"steep is context ballooning.\n\n")
	fmt.Fprintf(b, "| Agent | Decisions | Updates | Slope (tok/hand) | Early→Late prompt | Update prompt | Total tokens | Total $ |\n")
	fmt.Fprintf(b, "|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, a := range agents {
		slope := "n/a"
		if a.Slope != nil {
			slope = fmt.Sprintf("%+.1f", *a.Slope)
		}
		fmt.Fprintf(b, "| %s | %d | %d | %s | %s | %.0f | %d | %.2f |\n",
			a.Agent, a.Decisions, a.Updates, slope,
			formatEarlyLate(a.EarlyPrompt, a.LatePrompt),
			a.UpdPromptAvg, a.TotalTokens, a.TotalCost)
	}
	b.WriteString("\n")
}

func appendFidelitySection(b *strings.Builder, agents []FidelityAgentSummary) {
	if len(agents) == 0 {
		return
	}
	fmt.Fprintf(b, "## Fidelity\n\n")
	fmt.Fprintf(b, "Structured per-hand opponent records cross-validated against `hands.jsonl` "+
		"ground truth (deterministic, no LLM judging). **Fabrication** = a specific villain holding "+
		"stated on a non-showdown hand (unknowable in showdown-only info mode); **Card err** = holding "+
		"stated for a real showdown that doesn't match; **Board err** = a board card never dealt.\n\n")
	fmt.Fprintf(b, "| Agent | Records | Card-claims | Fabricated | Card err | Board err | Fab %% of claims |\n")
	fmt.Fprintf(b, "|---|---:|---:|---:|---:|---:|---:|\n")
	for _, a := range agents {
		fmt.Fprintf(b, "| %s | %d | %d | %d | %d | %d | %s |\n",
			a.Agent, a.Overall.Records, a.Overall.CardClaims, a.Overall.Fabrications,
			a.Overall.CardErrors, a.Overall.BoardErrors, fabPct(a.Overall))
	}
	b.WriteString("\n")

	fmt.Fprintf(b, "### Fidelity drift (by hand-number bucket)\n\n")
	for _, a := range agents {
		fmt.Fprintf(b, "**%s**\n\n", a.Agent)
		fmt.Fprintf(b, "| Hands | Records | Card-claims | Fabricated | Fab %% | Card err |\n")
		fmt.Fprintf(b, "|---|---:|---:|---:|---:|---:|\n")
		for _, bucket := range a.Buckets {
			fmt.Fprintf(b, "| %s | %d | %d | %d | %s | %d |\n",
				bucket.Label, bucket.Records, bucket.CardClaims, bucket.Fabrications,
				fabPct(bucket.FidelityAgg), bucket.CardErrors)
		}
		b.WriteString("\n")
	}
}

func formatEarlyLate(early, late *float64) string {
	if early == nil || late == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.0f→%.0f", *early, *late)
}

func fabPct(a FidelityAgg) string {
	if a.CardClaims == 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", 100.0*float64(a.Fabrications)/float64(a.CardClaims))
}

// WriteTokenCSV writes the per-call token series (the schema the Track B chart
// reads): group,session,agent,kind,hand,street,prompt_tokens,total_tokens,cost.
func WriteTokenCSV(path string, rows []TokenCSVRow) error {
	sorted := append([]TokenCSVRow(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Agent != b.Agent {
			return a.Agent < b.Agent
		}
		if a.Session != b.Session {
			return a.Session < b.Session
		}
		if handOrZero(a.Call.Hand) != handOrZero(b.Call.Hand) {
			return handOrZero(a.Call.Hand) < handOrZero(b.Call.Hand)
		}
		return kindOrder(a.Call.Kind) < kindOrder(b.Call.Kind)
	})

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("write token csv %s: %w", path, err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"group", "session", "agent", "kind", "hand", "street", "prompt_tokens", "total_tokens", "cost"}); err != nil {
		return fmt.Errorf("write token csv %s: %w", path, err)
	}
	for _, r := range sorted {
		hand := ""
		if r.Call.Hand != nil {
			hand = strconv.Itoa(*r.Call.Hand)
		}
		record := []string{
			r.Group, r.Session, r.Agent, r.Call.Kind, hand, r.Call.Street,
			strconv.Itoa(r.Call.PromptTokens), strconv.Itoa(r.Call.TotalTokens),
			strconv.FormatFloat(r.Call.Cost, 'f', -1, 64),
		}
		if err := w.Write(record); err != nil {
			return fmt.Errorf("write token csv %s: %w", path, err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("write token csv %s: %w", path, err)
	}
	return nil
}

func handOrZero(h *int) int {
	if h == nil {
		return 0
	}
	return *h
}

func kindOrder(kind string) int {
	if kind == "decision" {
		return 0
	}
	return 1
}
