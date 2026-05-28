package evalrun

import (
	"errors"
	"os"
	"strings"

	"github.com/RobertGumeny/agent-poker/internal/experiment"
	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

type ExecuteConfig struct {
	Agent0        string
	Agent1        string
	Hands         int
	Seed          int64
	SessionID     string
	SessionsDir   string
	Model         string
	ThinkingLevel string
}

type SessionInspection struct {
	Status string
	Reason string
}

type PlanCoverage struct {
	Plan       experiment.Plan
	Sessions   []SessionCoverage
	Present    int
	Missing    int
	Incomplete int
}

type SessionCoverage struct {
	Planned    experiment.PlannedRun
	Inspection SessionInspection
}

type GroupCoverage struct {
	Planned    int
	Present    int
	Missing    int
	Incomplete int
}

func (c PlanCoverage) GroupSummaries() map[string]GroupCoverage {
	summaries := map[string]GroupCoverage{
		"control":   {},
		"treatment": {},
	}
	for _, session := range c.Sessions {
		summary := summaries[session.Planned.GroupLabel]
		summary.Planned++
		switch session.Inspection.Status {
		case "present":
			summary.Present++
		case "incomplete":
			summary.Incomplete++
		default:
			summary.Missing++
		}
		summaries[session.Planned.GroupLabel] = summary
	}
	return summaries
}

func LoadPlanCoverage(
	experimentPath, sessionsDir string,
	loadDefinition func(string) (experiment.Definition, error),
	inspectSession func(experiment.PlannedRun, int) (SessionInspection, error),
) (PlanCoverage, error) {
	def, err := loadDefinition(experimentPath)
	if err != nil {
		return PlanCoverage{}, err
	}
	plan, err := def.Plan(sessionsDir)
	if err != nil {
		return PlanCoverage{}, err
	}

	coverage := PlanCoverage{Plan: plan}
	for _, planned := range plan.PlannedSessions {
		inspection, err := inspectSession(planned, plan.HandsPerSession)
		if err != nil {
			return PlanCoverage{}, err
		}
		coverage.Sessions = append(coverage.Sessions, SessionCoverage{Planned: planned, Inspection: inspection})
		switch inspection.Status {
		case "present":
			coverage.Present++
		case "incomplete":
			coverage.Incomplete++
		default:
			coverage.Missing++
		}
	}

	return coverage, nil
}

func InspectSession(planned experiment.PlannedRun, handsPerSession int) (SessionInspection, error) {
	_, err := os.Stat(planned.SessionDir)
	if err == nil {
		return InspectExistingSession(planned, handsPerSession)
	}
	if errors.Is(err, os.ErrNotExist) {
		return SessionInspection{Status: "missing"}, nil
	}
	return SessionInspection{}, err
}

func InspectExistingSession(planned experiment.PlannedRun, handsPerSession int) (SessionInspection, error) {
	manifest, err := sessionlog.ReadManifest(planned.SessionDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SessionInspection{Status: "incomplete", Reason: "manifest_missing"}, nil
		}
		return SessionInspection{Status: "incomplete", Reason: "manifest_unreadable"}, nil
	}
	if manifest.SessionID != "" && manifest.SessionID != planned.SessionID {
		return SessionInspection{Status: "incomplete", Reason: "session_id_mismatch"}, nil
	}
	if manifest.Seed != planned.Seed {
		return SessionInspection{Status: "incomplete", Reason: "seed_mismatch"}, nil
	}
	if manifest.HandCount != handsPerSession {
		return SessionInspection{Status: "incomplete", Reason: "hand_count_mismatch"}, nil
	}
	if len(manifest.Matches) == 0 {
		return SessionInspection{Status: "incomplete", Reason: "manifest_missing_match"}, nil
	}
	if !manifest.Matches[0].Completed {
		return SessionInspection{Status: "incomplete", Reason: "match_incomplete"}, nil
	}
	if !MatchHasSeat(manifest.Matches[0], planned.Agent) {
		return SessionInspection{Status: "incomplete", Reason: "agent_missing"}, nil
	}
	if strings.TrimSpace(planned.Opponent) != "" && !MatchHasSeat(manifest.Matches[0], planned.Opponent) {
		return SessionInspection{Status: "incomplete", Reason: "opponent_missing"}, nil
	}

	hands, err := sessionlog.ReadHands(planned.SessionDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SessionInspection{Status: "incomplete", Reason: "hands_missing"}, nil
		}
		return SessionInspection{Status: "incomplete", Reason: "hands_unreadable"}, nil
	}
	if len(hands) != handsPerSession {
		return SessionInspection{Status: "incomplete", Reason: "hands_count_mismatch"}, nil
	}

	return SessionInspection{Status: "present"}, nil
}

func MatchHasSeat(match sessionlog.ManifestMatch, name string) bool {
	for _, seat := range match.Seats {
		if seat.Name == name {
			return true
		}
	}
	return false
}
