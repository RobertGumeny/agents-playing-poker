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

var baseComparisonMetricOrder = []string{
	"chips_per_hand",
	"session_duration_s",
	"preflop_only_rate",
	"showdown_rate",
	"fallback_action_count",
	"decision_prompt_count_per_session",
}

type Comparison struct {
	ExperimentID    string
	Hypothesis      string
	HandsPerSession int
	Control         ComparedGroup
	Treatment       ComparedGroup
	SummaryRows     []MetricComparison
	ToolRows        []MetricComparison
	Sessions        []ComparedSession
	Warnings        []string
}

type ComparedGroup struct {
	Label      string
	Agent      string
	Opponent   string
	SessionIDs []string
}

type ComparedSession struct {
	GroupLabel      string
	SessionID       string
	Seed            int64
	AgentName       string
	AgentVersion    string
	OpponentName    string
	OpponentVersion string
	ChipsDelta      int
	ChipsPerHand    float64
	DurationS       int64
	PreflopOnlyRate float64
	ShowdownRate    float64
	FallbackActions int
	DecisionPrompts int
	MetricValues    map[string]float64
}

type MetricComparison struct {
	Name               string
	ControlMean        float64
	TreatmentMean      float64
	Delta              float64
	ExpectedDirection  *experiment.Direction
	ObservedInSessions bool
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
		Control: ComparedGroup{
			Label:    "control",
			Agent:    def.Control.Agent,
			Opponent: def.Control.Opponent,
		},
		Treatment: ComparedGroup{
			Label:    "treatment",
			Agent:    def.Treatment.Agent,
			Opponent: def.Treatment.Opponent,
		},
	}

	var controlSessions []ComparedSession
	var treatmentSessions []ComparedSession
	toolMetrics := map[string]struct{}{}
	warnings := map[string]struct{}{}
	groupAgentLabels := map[string]map[string]struct{}{
		"control":   {},
		"treatment": {},
	}
	groupOpponentLabels := map[string]map[string]struct{}{
		"control":   {},
		"treatment": {},
	}

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
		for metric := range session.MetricValues {
			if strings.HasSuffix(metric, "_per_session") || strings.HasSuffix(metric, "_per_hand") {
				if !isBaseMetric(metric) {
					toolMetrics[metric] = struct{}{}
				}
			}
		}

		agentLabel := displaySeat(session.AgentName, session.AgentVersion)
		opponentLabel := displaySeat(session.OpponentName, session.OpponentVersion)
		groupAgentLabels[planned.GroupLabel][agentLabel] = struct{}{}
		if opponentLabel != "" {
			groupOpponentLabels[planned.GroupLabel][opponentLabel] = struct{}{}
		}

		comparison.Sessions = append(comparison.Sessions, session)
		switch planned.GroupLabel {
		case "control":
			comparison.Control.SessionIDs = append(comparison.Control.SessionIDs, planned.SessionID)
			controlSessions = append(controlSessions, session)
		case "treatment":
			comparison.Treatment.SessionIDs = append(comparison.Treatment.SessionIDs, planned.SessionID)
			treatmentSessions = append(treatmentSessions, session)
		}
	}

	for _, label := range []string{"control", "treatment"} {
		if len(groupAgentLabels[label]) > 1 {
			warnings[fmt.Sprintf("group %s observed mixed agent identities: %s", label, joinSortedKeys(groupAgentLabels[label]))] = struct{}{}
		}
		if plannedOpponentForGroup(def, label) == "" && len(groupOpponentLabels[label]) > 1 {
			warnings[fmt.Sprintf("group %s observed mixed opponents: %s", label, joinSortedKeys(groupOpponentLabels[label]))] = struct{}{}
		}
	}

	metricOrder := append([]string(nil), baseComparisonMetricOrder...)
	for metric := range def.ExpectedDirection {
		if !contains(metricOrder, metric) {
			metricOrder = append(metricOrder, metric)
		}
	}
	sort.Strings(comparison.Control.SessionIDs)
	sort.Strings(comparison.Treatment.SessionIDs)
	sort.Slice(comparison.Sessions, func(i, j int) bool {
		if comparison.Sessions[i].GroupLabel != comparison.Sessions[j].GroupLabel {
			return comparison.Sessions[i].GroupLabel < comparison.Sessions[j].GroupLabel
		}
		return comparison.Sessions[i].SessionID < comparison.Sessions[j].SessionID
	})

	for _, metric := range metricOrder {
		row, ok := buildMetricComparison(metric, controlSessions, treatmentSessions, def.ExpectedDirection)
		if !ok {
			return Comparison{}, fmt.Errorf("compare experiment %q: expected_direction references unsupported metric %q", def.ID, metric)
		}
		comparison.SummaryRows = append(comparison.SummaryRows, row)
	}

	toolMetricNames := sortedKeys(toolMetrics)
	for _, metric := range toolMetricNames {
		row, ok := buildMetricComparison(metric, controlSessions, treatmentSessions, def.ExpectedDirection)
		if !ok {
			return Comparison{}, fmt.Errorf("compare experiment %q: expected_direction references unsupported metric %q", def.ID, metric)
		}
		comparison.ToolRows = append(comparison.ToolRows, row)
	}
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

	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Control (n=%d) | Treatment (n=%d) | Δ (T-C) | Direction |\n", len(c.Control.SessionIDs), len(c.Treatment.SessionIDs))
	fmt.Fprintf(&b, "|---|---:|---:|---:|---|\n")
	for _, row := range c.SummaryRows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", metricLabel(row.Name), formatMetricValue(row.Name, row.ControlMean), formatMetricValue(row.Name, row.TreatmentMean), formatMetricDelta(row.Name, row.Delta), directionResult(row))
	}
	b.WriteString("\n")

	if len(c.ToolRows) > 0 {
		fmt.Fprintf(&b, "## Tool Use\n\n")
		fmt.Fprintf(&b, "| Metric | Control (n=%d) | Treatment (n=%d) | Δ (T-C) | Direction |\n", len(c.Control.SessionIDs), len(c.Treatment.SessionIDs))
		fmt.Fprintf(&b, "|---|---:|---:|---:|---|\n")
		for _, row := range c.ToolRows {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", metricLabel(row.Name), formatMetricValue(row.Name, row.ControlMean), formatMetricValue(row.Name, row.TreatmentMean), formatMetricDelta(row.Name, row.Delta), directionResult(row))
		}
		b.WriteString("\n")
	}

	if len(c.Warnings) > 0 {
		fmt.Fprintf(&b, "## Warnings\n\n")
		for _, warning := range c.Warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "## Per-Session Results\n\n")
	fmt.Fprintf(&b, "| Group | Session | Seed | Agent | Opponent | Chips Δ | Chips/Hand | Duration (s) | Preflop-only | Showdown |\n")
	fmt.Fprintf(&b, "|---|---|---:|---|---|---:|---:|---:|---:|---:|\n")
	for _, session := range c.Sessions {
		fmt.Fprintf(&b, "| %s | %s | %d | %s | %s | %s | %s | %d | %s | %s |\n",
			session.GroupLabel,
			session.SessionID,
			session.Seed,
			displaySeat(session.AgentName, session.AgentVersion),
			displaySeat(session.OpponentName, session.OpponentVersion),
			formatSignedInt(session.ChipsDelta),
			formatMetricValue("chips_per_hand", session.ChipsPerHand),
			session.DurationS,
			formatMetricValue("preflop_only_rate", session.PreflopOnlyRate),
			formatMetricValue("showdown_rate", session.ShowdownRate),
		)
	}
	b.WriteString("\n")
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

	agentSeat, err := selectSeat(summary.Seats, planned.Agent)
	if err != nil {
		return ComparedSession{}, fmt.Errorf("session %q: %w", planned.SessionID, err)
	}

	var opponentSeat *SeatSummary
	if strings.TrimSpace(planned.Opponent) != "" {
		seat, err := selectSeat(summary.Seats, planned.Opponent)
		if err != nil {
			return ComparedSession{}, fmt.Errorf("session %q: %w", planned.SessionID, err)
		}
		opponentSeat = &seat
	} else {
		for _, seat := range summary.Seats {
			if seat.Seat != agentSeat.Seat {
				copySeat := seat
				opponentSeat = &copySeat
				break
			}
		}
	}

	metricValues := map[string]float64{
		"chips_per_hand":                    safeRate(agentSeat.ChipsDelta, summary.Session.HandCount),
		"session_duration_s":                float64(summary.Session.DurationS),
		"preflop_only_rate":                 summary.Metrics.PreflopOnlyRate,
		"showdown_rate":                     summary.Metrics.ShowdownRate,
		"fallback_action_count":             float64(summary.Metrics.FallbackActionCount),
		"decision_prompt_count_per_session": float64(agentSeat.DecisionPromptCount),
	}
	for name, count := range agentSeat.ToolCalls {
		metricValues[name+"_per_session"] = float64(count)
	}
	for name, rate := range agentSeat.ToolCallsPerHand {
		metricValues[name+"_per_hand"] = rate
	}

	session := ComparedSession{
		GroupLabel:      planned.GroupLabel,
		SessionID:       planned.SessionID,
		Seed:            planned.Seed,
		AgentName:       agentSeat.Name,
		AgentVersion:    agentSeat.Version,
		ChipsDelta:      agentSeat.ChipsDelta,
		ChipsPerHand:    metricValues["chips_per_hand"],
		DurationS:       summary.Session.DurationS,
		PreflopOnlyRate: summary.Metrics.PreflopOnlyRate,
		ShowdownRate:    summary.Metrics.ShowdownRate,
		FallbackActions: summary.Metrics.FallbackActionCount,
		DecisionPrompts: agentSeat.DecisionPromptCount,
		MetricValues:    metricValues,
	}
	if opponentSeat != nil {
		session.OpponentName = opponentSeat.Name
		session.OpponentVersion = opponentSeat.Version
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

func buildMetricComparison(metric string, controlSessions, treatmentSessions []ComparedSession, expected map[string]experiment.Direction) (MetricComparison, bool) {
	if !metricAvailable(metric, controlSessions, treatmentSessions) {
		return MetricComparison{}, false
	}
	row := MetricComparison{
		Name:          metric,
		ControlMean:   groupMetricMean(metric, controlSessions),
		TreatmentMean: groupMetricMean(metric, treatmentSessions),
	}
	row.Delta = row.TreatmentMean - row.ControlMean
	if direction, ok := expected[metric]; ok {
		copyDirection := direction
		row.ExpectedDirection = &copyDirection
	}
	return row, true
}

func metricAvailable(metric string, groups ...[]ComparedSession) bool {
	if isBaseMetric(metric) {
		return true
	}
	for _, sessions := range groups {
		for _, session := range sessions {
			if _, ok := session.MetricValues[metric]; ok {
				return true
			}
		}
	}
	return false
}

func groupMetricMean(metric string, sessions []ComparedSession) float64 {
	if len(sessions) == 0 {
		return 0
	}
	total := 0.0
	for _, session := range sessions {
		total += session.MetricValues[metric]
	}
	return total / float64(len(sessions))
}

func isBaseMetric(metric string) bool {
	for _, base := range baseComparisonMetricOrder {
		if metric == base {
			return true
		}
	}
	return false
}

func metricLabel(metric string) string {
	switch metric {
	case "chips_per_hand":
		return "chips/hand"
	case "session_duration_s":
		return "session duration (s)"
	case "preflop_only_rate":
		return "preflop-only rate"
	case "showdown_rate":
		return "showdown rate"
	case "fallback_action_count":
		return "fallback actions/session"
	case "decision_prompt_count_per_session":
		return "decision prompts/session"
	}
	if strings.HasSuffix(metric, "_per_session") {
		return strings.TrimSuffix(metric, "_per_session") + "/session"
	}
	if strings.HasSuffix(metric, "_per_hand") {
		return strings.TrimSuffix(metric, "_per_hand") + "/hand"
	}
	return metric
}

func formatMetricValue(metric string, value float64) string {
	switch {
	case metric == "session_duration_s":
		return fmt.Sprintf("%.0f", value)
	case strings.HasSuffix(metric, "_rate"):
		return fmt.Sprintf("%.1f%%", value*100)
	default:
		return fmt.Sprintf("%.2f", value)
	}
}

func formatMetricDelta(metric string, value float64) string {
	switch {
	case metric == "session_duration_s":
		return fmt.Sprintf("%+.0f", value)
	case strings.HasSuffix(metric, "_rate"):
		return fmt.Sprintf("%+.1fpp", value*100)
	default:
		return fmt.Sprintf("%+.2f", value)
	}
}

func directionResult(row MetricComparison) string {
	if row.ExpectedDirection == nil {
		return "-"
	}
	observed := "unchanged"
	if row.Delta > 0 {
		observed = "increase"
	}
	if row.Delta < 0 {
		observed = "decrease"
	}
	pass := (*row.ExpectedDirection == experiment.DirectionIncrease && row.Delta > 0) || (*row.ExpectedDirection == experiment.DirectionDecrease && row.Delta < 0)
	if pass {
		return "✅ " + observed
	}
	return fmt.Sprintf("❌ %s (expected %s)", observed, *row.ExpectedDirection)
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

func plannedOpponentForGroup(def experiment.Definition, label string) string {
	if label == "control" {
		return def.Control.Opponent
	}
	return def.Treatment.Opponent
}

func joinSortedKeys(values map[string]struct{}) string {
	return strings.Join(sortedKeys(values), ", ")
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
