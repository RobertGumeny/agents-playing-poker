package sessionreport

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

func Generate(sessionDir string) error {
	manifest, err := sessionlog.ReadManifest(sessionDir)
	if err != nil {
		return fmt.Errorf("generate report: %w", err)
	}
	hands, err := sessionlog.ReadHands(sessionDir)
	if err != nil {
		return fmt.Errorf("generate report: %w", err)
	}
	md := buildMarkdown(manifest, hands)
	if err := os.WriteFile(filepath.Join(sessionDir, "report.md"), []byte(md), 0o644); err != nil {
		return fmt.Errorf("generate report: write: %w", err)
	}
	return nil
}

func buildMarkdown(manifest sessionlog.Manifest, hands []sessionlog.HandRecord) string {
	if len(manifest.Matches) == 0 || len(hands) == 0 {
		return "# Session: " + manifest.SessionID + "\n\nNo match data.\n"
	}

	match := manifest.Matches[0]
	seat0Name := seatName(match, 0)
	seat1Name := seatName(match, 1)
	seat0Delta := match.Result[0].ChipsDelta
	seat1Delta := match.Result[1].ChipsDelta
	n := len(hands)

	startedAt, _ := time.Parse(time.RFC3339, manifest.StartedAt)
	endedAt, _ := time.Parse(time.RFC3339, manifest.EndedAt)
	duration := endedAt.Sub(startedAt).Round(time.Second)

	dateStr := ""
	if !startedAt.IsZero() {
		dateStr = startedAt.UTC().Format("2006-01-02")
	}

	var sb strings.Builder

	// Header
	fmt.Fprintf(&sb, "# Session: %s\n\n", manifest.SessionID)
	fmt.Fprintf(&sb, "**Date:** %s · **Agents:** %s vs %s · **Hands:** %d · **Seed:** %d\n\n",
		dateStr, seat0Name, seat1Name, n, manifest.Seed)
	sb.WriteString("---\n\n")

	// Result
	sb.WriteString("## Result\n\n")
	sb.WriteString("| Agent | Version | Total Δ | chips/hand |\n")
	sb.WriteString("|---|---|---|---|\n")
	winner0 := seat0Delta >= seat1Delta
	rows := []struct {
		name    string
		version string
		delta   int
		cpp     float64
		winner  bool
	}{
		{seat0Name, seatVersion(match, 0), seat0Delta, float64(seat0Delta) / float64(n), winner0},
		{seat1Name, seatVersion(match, 1), seat1Delta, float64(seat1Delta) / float64(n), !winner0},
	}
	for _, row := range rows {
		if row.winner {
			fmt.Fprintf(&sb, "| **%s** | %s | **%+d** | **%+.1f** |\n", row.name, row.version, row.delta, row.cpp)
		} else {
			fmt.Fprintf(&sb, "| %s | %s | %+d | %+.1f |\n", row.name, row.version, row.delta, row.cpp)
		}
	}
	fmt.Fprintf(&sb, "\n**Duration:** %s\n\n", formatDuration(duration))
	sb.WriteString("---\n\n")

	// Key Hands
	sb.WriteString("## Key Hands\n\n")
	sb.WriteString("*Top 5 pots by chip swing.*\n\n")
	fmt.Fprintf(&sb, "| Hand | %s | %s | Board | Street | Δ |\n", seat0Name, seat1Name)
	sb.WriteString("|---|---|---|---|---|---|\n")
	sorted := make([]sessionlog.HandRecord, len(hands))
	copy(sorted, hands)
	sort.Slice(sorted, func(i, j int) bool {
		return absInt(deltaFor(sorted[i], 0)) > absInt(deltaFor(sorted[j], 0))
	})
	limit := 5
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for _, h := range sorted[:limit] {
		board := strings.Join(h.Board, " ")
		if board == "" {
			board = "-"
		}
		fmt.Fprintf(&sb, "| %d | %s | %s | %s | %s | %+d |\n",
			h.HandNumber,
			strings.Join(h.HoleCards[0], " "),
			strings.Join(h.HoleCards[1], " "),
			board,
			streetReached(h),
			deltaFor(h, 0),
		)
	}
	sb.WriteString("\n---\n\n")

	// Hand Log
	sb.WriteString("## Hand Log\n\n")
	fmt.Fprintf(&sb, "| # | %s Δ | %s total | %s Δ | %s total | SD | Street |\n",
		seat0Name, seat0Name, seat1Name, seat1Name)
	sb.WriteString("|---|---|---|---|---|---|---|\n")
	running0, running1 := 0, 0
	for _, h := range hands {
		d0 := deltaFor(h, 0)
		d1 := deltaFor(h, 1)
		running0 += d0
		running1 += d1
		sd := "N"
		if h.ShowdownReached {
			sd = "Y"
		}
		fmt.Fprintf(&sb, "| %d | %+d | %+d | %+d | %+d | %s | %s |\n",
			h.HandNumber, d0, running0, d1, running1, sd, streetReached(h))
	}
	sb.WriteString("\n---\n\n")

	// Stats
	sb.WriteString("## Stats\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|---|---|\n")
	showdownCount, preflopOnlyCount, biggestPot, biggestPotHand := 0, 0, 0, 0
	for _, h := range hands {
		if h.ShowdownReached {
			showdownCount++
		}
		if streetReached(h) == "preflop" {
			preflopOnlyCount++
		}
		if d := absInt(deltaFor(h, 0)); d > biggestPot {
			biggestPot = d
			biggestPotHand = h.HandNumber
		}
	}
	fmt.Fprintf(&sb, "| Showdown rate | %d/%d (%.0f%%) |\n", showdownCount, n, pct(showdownCount, n))
	fmt.Fprintf(&sb, "| Preflop-only | %d/%d (%.0f%%) |\n", preflopOnlyCount, n, pct(preflopOnlyCount, n))
	fmt.Fprintf(&sb, "| Biggest pot | Hand %d (%d chips) |\n", biggestPotHand, biggestPot)
	fmt.Fprintf(&sb, "| Duration | %s |\n", formatDuration(duration))
	sb.WriteString("\n")

	return sb.String()
}

func seatName(match sessionlog.ManifestMatch, seat int) string {
	for _, s := range match.Seats {
		if s.Seat == seat {
			return s.Name
		}
	}
	return fmt.Sprintf("seat%d", seat)
}

func seatVersion(match sessionlog.ManifestMatch, seat int) string {
	for _, s := range match.Seats {
		if s.Seat == seat {
			return s.Version
		}
	}
	return ""
}

func deltaFor(h sessionlog.HandRecord, seat int) int {
	for _, r := range h.Result {
		if r.Seat == seat {
			return r.ChipsDelta
		}
	}
	return 0
}

func streetReached(h sessionlog.HandRecord) string {
	if h.ShowdownReached {
		return "showdown"
	}
	order := []string{"preflop", "flop", "turn", "river"}
	seen := make(map[string]bool, len(h.Actions))
	for _, a := range h.Actions {
		seen[a.Street] = true
	}
	last := "preflop"
	for _, s := range order {
		if seen[s] {
			last = s
		}
	}
	return last
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m == 0 {
		return fmt.Sprintf("%ds", s)
	}
	return fmt.Sprintf("%dm %ds", m, s)
}
