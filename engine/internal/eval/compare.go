package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RobertGumeny/agent-poker/internal/experiment"
)

type Comparison struct {
	ExperimentID     string
	Hypothesis       string
	HandsPerSession  int
	Sessions         []ComparedSession
	Warnings         []string
	TokenAgents      []TokenAgentSummary
	FidelityAgents   []FidelityAgentSummary
	EngagementAgents []EngagementAgentSummary
	TokenCSVRows     []TokenCSVRow
}

// FidelityAgentSummary is the per-agent fidelity aggregate plus drift buckets,
// aggregated across every session an agent name appears in.
type FidelityAgentSummary struct {
	Agent   string
	Overall FidelityAgg
	Buckets []FidelityBucket
}

// TokenCSVRow is one per-call row written to <id>-tokens.csv (the series the
// Track B replay chart reads).
type TokenCSVRow struct {
	Group   string
	Session string
	Agent   string
	Call    TokenCall
}

type ComparedSession struct {
	GroupLabel      string
	SessionID       string
	Seed            int64
	Seat0Name       string
	Seat0Version    string
	Seat1Name       string
	Seat1Version    string
	Seat0ChipsDelta int
	ChipsPerHand    float64
	DurationS       int64
	PreflopOnlyRate float64
	ShowdownRate    float64
	FallbackActions int
	DecisionPrompts int
}

func Compare(def experiment.Definition, sessionsDir string) (Comparison, error) {
	plan, err := def.Plan(sessionsDir)
	if err != nil {
		return Comparison{}, err
	}

	comparison := Comparison{
		ExperimentID:    def.ID,
		Hypothesis:      def.Hypothesis,
		HandsPerSession: def.HandsPerSession,
	}

	warnings := map[string]struct{}{}
	tokensByAgent := map[string][]TokenCall{}
	fidelityByAgent := map[string][]FidelityRow{}
	engagementByAgent := map[string]*EngagementAgentSummary{}

	for _, planned := range plan.PlannedSessions {
		summaryPath := filepath.Join(planned.SessionDir, "eval.json")
		summary, err := ReadSummary(summaryPath)
		if err != nil {
			return Comparison{}, fmt.Errorf("compare experiment %q: load collected session %q: %w", def.ID, planned.SessionID, err)
		}
		session, err := compareSession(planned, def.HandsPerSession, summary)
		if err != nil {
			return Comparison{}, fmt.Errorf("compare experiment %q: %w", def.ID, err)
		}
		comparison.Sessions = append(comparison.Sessions, session)

		for _, seat := range summary.Seats {
			tokensByAgent[seat.Name] = append(tokensByAgent[seat.Name], seat.TokenCalls...)
			fidelityByAgent[seat.Name] = append(fidelityByAgent[seat.Name], seat.FidelityRows...)
			accumulateEngagement(engagementByAgent, seat, planned.SessionID)
			for _, call := range seat.TokenCalls {
				comparison.TokenCSVRows = append(comparison.TokenCSVRows, TokenCSVRow{
					Group:   planned.GroupLabel,
					Session: planned.SessionID,
					Agent:   seat.Name,
					Call:    call,
				})
			}
		}
	}

	comparison.TokenAgents = summarizeTokens(tokensByAgent)
	comparison.FidelityAgents = summarizeFidelity(fidelityByAgent)
	comparison.EngagementAgents = finalizeEngagement(engagementByAgent)
	for _, agent := range comparison.EngagementAgents {
		if agent.TripwireFail() {
			warnings[fmt.Sprintf("TRIPWIRE FAIL: retrieval agent %q made zero decision-time reads (memory write-only) in session(s): %s",
				agent.Agent, strings.Join(agent.ZeroReadSessions, ", "))] = struct{}{}
		}
	}

	sort.Slice(comparison.Sessions, func(i, j int) bool {
		if comparison.Sessions[i].GroupLabel != comparison.Sessions[j].GroupLabel {
			return comparison.Sessions[i].GroupLabel < comparison.Sessions[j].GroupLabel
		}
		return comparison.Sessions[i].SessionID < comparison.Sessions[j].SessionID
	})

	for warning := range warnings {
		comparison.Warnings = append(comparison.Warnings, warning)
	}
	sort.Strings(comparison.Warnings)

	return comparison, nil
}

func ReadSummary(path string) (Summary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Summary{}, fmt.Errorf("collected session data missing: %s", path)
		}
		return Summary{}, fmt.Errorf("read collected session data %s: %w", path, err)
	}
	var summary Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		return Summary{}, fmt.Errorf("parse collected session data %s: %w", path, err)
	}
	if summary.SchemaVersion != 1 {
		return Summary{}, fmt.Errorf("unsupported schema_version %d in %s", summary.SchemaVersion, path)
	}
	if summary.Seats == nil {
		summary.Seats = []SeatSummary{}
	}
	return summary, nil
}

func RenderComparisonMarkdown(c Comparison) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Experiment: %s\n\n", c.ExperimentID)
	if strings.TrimSpace(c.Hypothesis) != "" {
		fmt.Fprintf(&b, "**Hypothesis:** %s\n\n", c.Hypothesis)
	}

	if len(c.Warnings) > 0 {
		fmt.Fprintf(&b, "## Warnings\n\n")
		for _, warning := range c.Warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "## Per-Session Results\n\n")
	fmt.Fprintf(&b, "| Group | Session | Seed | Seat 0 | Seat 1 | Chips Δ | Chips/hand | Duration (s) | Preflop-only | Showdown |\n")
	fmt.Fprintf(&b, "|---|---|---:|---|---|---:|---:|---:|---:|---:|\n")
	for _, session := range c.Sessions {
		fmt.Fprintf(&b, "| %s | %s | %d | %s | %s | %s | %.2f | %d | %.1f%% | %.1f%% |\n",
			session.GroupLabel,
			session.SessionID,
			session.Seed,
			displaySeat(session.Seat0Name, session.Seat0Version),
			displaySeat(session.Seat1Name, session.Seat1Version),
			formatSignedInt(session.Seat0ChipsDelta),
			session.ChipsPerHand,
			session.DurationS,
			session.PreflopOnlyRate*100,
			session.ShowdownRate*100,
		)
	}
	b.WriteString("\n")
	appendTokenSection(&b, c.TokenAgents)
	appendEngagementSection(&b, c.EngagementAgents)
	appendFidelitySection(&b, c.FidelityAgents)
	return b.String()
}

func compareSession(planned experiment.PlannedRun, handsPerSession int, summary Summary) (ComparedSession, error) {
	if summary.SessionID != planned.SessionID {
		return ComparedSession{}, fmt.Errorf("session %q has collected session_id %q", planned.SessionID, summary.SessionID)
	}
	if summary.Session.Seed != planned.Seed {
		return ComparedSession{}, fmt.Errorf("session %q has collected seed %d, want %d", planned.SessionID, summary.Session.Seed, planned.Seed)
	}
	if summary.Session.HandCount != handsPerSession {
		return ComparedSession{}, fmt.Errorf("session %q has collected hand_count %d, want %d", planned.SessionID, summary.Session.HandCount, handsPerSession)
	}
	if !summary.Session.Completed {
		return ComparedSession{}, fmt.Errorf("session %q is not marked completed in collected data", planned.SessionID)
	}

	seat0, err := selectSeat(summary.Seats, planned.Seat0)
	if err != nil {
		return ComparedSession{}, fmt.Errorf("session %q: seat0: %w", planned.SessionID, err)
	}

	session := ComparedSession{
		GroupLabel:      planned.GroupLabel,
		SessionID:       planned.SessionID,
		Seed:            planned.Seed,
		Seat0Name:       seat0.Name,
		Seat0Version:    seat0.Version,
		Seat0ChipsDelta: seat0.ChipsDelta,
		ChipsPerHand:    safeRate(seat0.ChipsDelta, summary.Session.HandCount),
		DurationS:       summary.Session.DurationS,
		PreflopOnlyRate: summary.Metrics.PreflopOnlyRate,
		ShowdownRate:    summary.Metrics.ShowdownRate,
		FallbackActions: summary.Metrics.FallbackActionCount,
		DecisionPrompts: seat0.DecisionPromptCount,
	}

	if strings.TrimSpace(planned.Seat1) != "" {
		seat1, err := selectSeat(summary.Seats, planned.Seat1)
		if err != nil {
			return ComparedSession{}, fmt.Errorf("session %q: seat1: %w", planned.SessionID, err)
		}
		session.Seat1Name = seat1.Name
		session.Seat1Version = seat1.Version
	} else {
		for _, seat := range summary.Seats {
			if seat.Seat != seat0.Seat {
				session.Seat1Name = seat.Name
				session.Seat1Version = seat.Version
				break
			}
		}
	}

	return session, nil
}

func selectSeat(seats []SeatSummary, identifier string) (SeatSummary, error) {
	trimmed := strings.TrimSpace(identifier)
	if trimmed == "" {
		return SeatSummary{}, fmt.Errorf("seat identifier is required")
	}
	var matches []SeatSummary
	for _, seat := range seats {
		if seat.Name == trimmed || seat.Version == trimmed {
			matches = append(matches, seat)
		}
	}
	switch len(matches) {
	case 0:
		return SeatSummary{}, fmt.Errorf("no seat matched identifier %q", trimmed)
	case 1:
		return matches[0], nil
	default:
		return SeatSummary{}, fmt.Errorf("multiple seats matched identifier %q", trimmed)
	}
}

func displaySeat(name, version string) string {
	if strings.TrimSpace(name) == "" {
		return strings.TrimSpace(version)
	}
	if strings.TrimSpace(version) == "" || version == name {
		return name
	}
	return fmt.Sprintf("%s [%s]", name, version)
}

func formatSignedInt(v int) string {
	if v > 0 {
		return fmt.Sprintf("+%d", v)
	}
	return fmt.Sprintf("%d", v)
}
