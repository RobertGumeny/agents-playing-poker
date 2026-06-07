package experiment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Definition struct {
	ID                      string `json:"id"`
	Hypothesis              string `json:"hypothesis,omitempty"`
	Model                   string `json:"model"`
	HandsPerSession         int    `json:"hands_per_session"`
	DecisionDeadlineSeconds int    `json:"decision_deadline_seconds,omitempty"`
	Groups                  []Group `json:"groups"`
}

type Group struct {
	SessionBase   string   `json:"session_base,omitempty"`
	SessionsCount int      `json:"sessions_count,omitempty"`
	Sessions      []string `json:"sessions,omitempty"`
	Seat0         string   `json:"seat0"`
	Seat1         string   `json:"seat1,omitempty"`
	Seeds         []int64  `json:"seeds,omitempty"`
}

type PlannedSession struct {
	GroupLabel string
	SessionID  string
	Seed       int64
}

type Plan struct {
	ExperimentID            string
	Model                   string
	HandsPerSession         int
	DecisionDeadlineSeconds int
	SessionsRootDir         string
	PlannedSessions         []PlannedRun
}

type PlannedRun struct {
	GroupLabel      string
	SessionID       string
	SessionDir      string
	Seed            int64
	Seat0           string
	Seat1           string
	ExplicitSession bool // true when session was named explicitly rather than generated from session_base
}

func Load(path string) (Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, fmt.Errorf("load experiment definition: %w", err)
	}
	def, err := Parse(data)
	if err != nil {
		return Definition{}, fmt.Errorf("load experiment definition %s: %w", filepath.Base(path), err)
	}
	return def, nil
}

func Parse(data []byte) (Definition, error) {
	var def Definition
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&def); err != nil {
		return Definition{}, fmt.Errorf("parse experiment definition: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != nil && err != io.EOF {
		return Definition{}, fmt.Errorf("parse experiment definition: trailing content")
	}
	if err := def.Validate(); err != nil {
		return Definition{}, err
	}
	return def, nil
}

func (d Definition) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return fmt.Errorf("validate experiment definition: id is required")
	}
	if strings.TrimSpace(d.Model) == "" {
		return fmt.Errorf("validate experiment definition: model is required")
	}
	if d.HandsPerSession <= 0 {
		return fmt.Errorf("validate experiment definition: hands_per_session must be > 0")
	}
	if d.DecisionDeadlineSeconds < 0 {
		return fmt.Errorf("validate experiment definition: decision_deadline_seconds must be >= 0")
	}
	if len(d.Groups) == 0 {
		return fmt.Errorf("validate experiment definition: at least one group is required")
	}
	for i, g := range d.Groups {
		if err := g.validate(fmt.Sprintf("groups[%d]", i)); err != nil {
			return err
		}
	}
	return nil
}

func (d Definition) Plan(sessionsRootDir string) (Plan, error) {
	rootDir := filepath.Clean(sessionsRootDir)
	plan := Plan{
		ExperimentID:            d.ID,
		Model:                   d.Model,
		HandsPerSession:         d.HandsPerSession,
		DecisionDeadlineSeconds: d.DecisionDeadlineSeconds,
		SessionsRootDir:         rootDir,
	}

	seen := make(map[string]PlannedRun)
	for i, g := range d.Groups {
		label := fmt.Sprintf("group-%d", i)
		for _, planned := range g.PlannedSessions(label) {
			run := PlannedRun{
				GroupLabel:      label,
				SessionID:       planned.SessionID,
				SessionDir:      filepath.Join(rootDir, planned.SessionID),
				Seed:            planned.Seed,
				Seat0:           g.Seat0,
				Seat1:           g.Seat1,
				ExplicitSession: len(g.Sessions) > 0,
			}
			if prior, ok := seen[run.SessionID]; ok {
				if prior == run {
					return Plan{}, fmt.Errorf("plan experiment %q: duplicate planned session %q", d.ID, run.SessionID)
				}
				return Plan{}, fmt.Errorf("plan experiment %q: conflicting planned session %q (%s seed=%d seat0=%q seat1=%q vs %s seed=%d seat0=%q seat1=%q)", d.ID, run.SessionID, prior.GroupLabel, prior.Seed, prior.Seat0, prior.Seat1, run.GroupLabel, run.Seed, run.Seat0, run.Seat1)
			}
			seen[run.SessionID] = run
			plan.PlannedSessions = append(plan.PlannedSessions, run)
		}
	}

	return plan, nil
}

func (g Group) PlannedSessions(label string) []PlannedSession {
	sessionIDs := g.sessionIDs()
	seeds := g.derivedSeeds(len(sessionIDs))
	planned := make([]PlannedSession, 0, len(sessionIDs))
	for i, sessionID := range sessionIDs {
		planned = append(planned, PlannedSession{GroupLabel: label, SessionID: sessionID, Seed: seeds[i]})
	}
	return planned
}

func (g Group) validate(label string) error {
	if strings.TrimSpace(g.Seat0) == "" {
		return fmt.Errorf("validate experiment definition: %s.seat0 is required", label)
	}

	hasSessionBaseMode := strings.TrimSpace(g.SessionBase) != "" || g.SessionsCount != 0
	hasExplicitSessionsMode := len(g.Sessions) > 0
	if hasSessionBaseMode == hasExplicitSessionsMode {
		return fmt.Errorf("validate experiment definition: %s must use exactly one session mode", label)
	}

	if hasSessionBaseMode {
		if strings.TrimSpace(g.SessionBase) == "" {
			return fmt.Errorf("validate experiment definition: %s.session_base is required in session-base mode", label)
		}
		if g.SessionsCount <= 0 {
			return fmt.Errorf("validate experiment definition: %s.sessions_count must be > 0 in session-base mode", label)
		}
		if len(g.Seeds) > 0 && len(g.Seeds) != g.SessionsCount {
			return fmt.Errorf("validate experiment definition: %s.seeds length must match sessions_count", label)
		}
		return nil
	}

	seen := make(map[string]struct{}, len(g.Sessions))
	for i, sessionID := range g.Sessions {
		if strings.TrimSpace(sessionID) == "" {
			return fmt.Errorf("validate experiment definition: %s.sessions[%d] must not be empty", label, i)
		}
		if _, ok := seen[sessionID]; ok {
			return fmt.Errorf("validate experiment definition: %s.sessions[%d] duplicates %q", label, i, sessionID)
		}
		seen[sessionID] = struct{}{}
	}
	if len(g.Seeds) > 0 && len(g.Seeds) != len(g.Sessions) {
		return fmt.Errorf("validate experiment definition: %s.seeds length must match sessions length", label)
	}
	return nil
}

func (g Group) sessionIDs() []string {
	if len(g.Sessions) > 0 {
		return append([]string(nil), g.Sessions...)
	}
	sessionIDs := make([]string, 0, g.SessionsCount)
	for i := 1; i <= g.SessionsCount; i++ {
		sessionIDs = append(sessionIDs, fmt.Sprintf("%s-%d", g.SessionBase, i))
	}
	return sessionIDs
}

func (g Group) derivedSeeds(count int) []int64 {
	if len(g.Seeds) > 0 {
		return append([]int64(nil), g.Seeds...)
	}
	seeds := make([]int64, 0, count)
	for i := 1; i <= count; i++ {
		seeds = append(seeds, int64(i))
	}
	return seeds
}
