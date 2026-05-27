package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

type Summary struct {
	SchemaVersion   int             `json:"schema_version"`
	SessionID       string          `json:"session_id"`
	MatchID         string          `json:"match_id"`
	SourceArtifacts SourceArtifacts `json:"source_artifacts"`
	Session         SessionSummary  `json:"session"`
	Metrics         SessionMetrics  `json:"metrics"`
	Seats           []SeatSummary   `json:"seats"`
}

type SourceArtifacts struct {
	Manifest string                          `json:"manifest"`
	Hands    string                          `json:"hands"`
	Agents   map[string]AgentSourceArtifacts `json:"agents"`
}

type AgentSourceArtifacts struct {
	PiSession    *string `json:"pi_session"`
	MemoryExport *string `json:"memory_export"`
	Stderr       *string `json:"stderr"`
}

type SessionSummary struct {
	Seed           int64                 `json:"seed"`
	DurationS      int64                 `json:"duration_s"`
	HandCount      int                   `json:"hand_count"`
	Variant        string                `json:"variant"`
	InfoRealism    string                `json:"info_realism"`
	StartingStack  int                   `json:"starting_stack"`
	Blinds         sessionlog.BlindLevel `json:"blinds"`
	Completed      bool                  `json:"completed"`
	ServerVersion  string                `json:"server_version"`
	AKGSpecVersion string                `json:"akg_spec_version"`
}

type SessionMetrics struct {
	PreflopOnlyHands    int              `json:"preflop_only_hands"`
	PreflopOnlyRate     float64          `json:"preflop_only_rate"`
	ShowdownHands       int              `json:"showdown_hands"`
	ShowdownRate        float64          `json:"showdown_rate"`
	BiggestSwingHand    BiggestSwingHand `json:"biggest_swing_hand"`
	FallbackActionCount int              `json:"fallback_action_count"`
}

type BiggestSwingHand struct {
	HandNumber int `json:"hand_number"`
	Chips      int `json:"chips"`
}

type SeatSummary struct {
	Seat                int                  `json:"seat"`
	Name                string               `json:"name"`
	Version             string               `json:"version"`
	ChipsDelta          int                  `json:"chips_delta"`
	PiSessionPresent    bool                 `json:"pi_session_present"`
	DecisionPromptCount int                  `json:"decision_prompt_count"`
	ToolCalls           map[string]int       `json:"tool_calls"`
	ToolCallsPerHand    map[string]float64   `json:"tool_calls_per_hand"`
	RetryMetrics        RetryMetrics         `json:"retry_metrics"`
	MemoryExport        *MemoryExportSummary `json:"memory_export"`
}

type RetryMetrics struct {
	AttemptFailures        int `json:"attempt_failures"`
	MalformedActionRetries int `json:"malformed_action_retries"`
	ExhaustedCount         int `json:"exhausted_count"`
	MaxAttemptsObserved    int `json:"max_attempts_observed"`
}

func CollectSession(sessionDir string) (Summary, error) {
	artifacts, err := LoadSession(sessionDir)
	if err != nil {
		return Summary{}, err
	}
	match := artifacts.Manifest.Matches[0]
	durationS, err := durationSeconds(artifacts.Manifest.StartedAt, artifacts.Manifest.EndedAt)
	if err != nil {
		return Summary{}, fmt.Errorf("collect eval session %q: %w", sessionDir, err)
	}

	summary := Summary{
		SchemaVersion: 1,
		SessionID:     artifacts.Manifest.SessionID,
		MatchID:       match.MatchID,
		SourceArtifacts: SourceArtifacts{
			Manifest: "manifest.json",
			Hands:    "hands.jsonl",
			Agents:   map[string]AgentSourceArtifacts{},
		},
		Session: SessionSummary{
			Seed:           artifacts.Manifest.Seed,
			DurationS:      durationS,
			HandCount:      artifacts.Manifest.HandCount,
			Variant:        artifacts.Manifest.Variant,
			InfoRealism:    artifacts.Manifest.InfoRealism,
			StartingStack:  artifacts.Manifest.StartingStack,
			Blinds:         artifacts.Manifest.Blinds,
			Completed:      match.Completed,
			ServerVersion:  artifacts.Manifest.ServerVersion,
			AKGSpecVersion: artifacts.Manifest.AKGSpecVersion,
		},
		Metrics: collectSessionMetrics(artifacts.Hands, artifacts.Manifest.HandCount),
		Seats:   make([]SeatSummary, 0, len(artifacts.Agents)),
	}

	for _, agent := range artifacts.Agents {
		source := AgentSourceArtifacts{
			PiSession:    optionalRelPath(sessionDir, agent.PiSessionPath),
			MemoryExport: optionalRelPath(sessionDir, agent.MemoryExportPath),
			Stderr:       optionalRelPath(sessionDir, agent.StderrPath),
		}
		summary.SourceArtifacts.Agents[agent.Seat.Name] = source

		seatSummary := SeatSummary{
			Seat:             agent.Seat.Seat,
			Name:             agent.Seat.Name,
			Version:          agent.Seat.Version,
			ChipsDelta:       chipsDeltaForSeat(match.Result, agent.Seat.Seat),
			PiSessionPresent: agent.PiSession != nil,
			ToolCalls:        map[string]int{},
			ToolCallsPerHand: map[string]float64{},
			RetryMetrics: RetryMetrics{
				AttemptFailures:        agent.RetrySummary.AttemptFailures,
				MalformedActionRetries: agent.RetrySummary.MalformedActionRetries,
				ExhaustedCount:         agent.RetrySummary.ExhaustedCount,
				MaxAttemptsObserved:    agent.RetrySummary.MaxAttemptsObserved,
			},
		}
		if agent.PiSession != nil {
			seatSummary.DecisionPromptCount = agent.PiSession.DecisionPromptCount()
			seatSummary.ToolCalls = agent.PiSession.ToolCallCounts()
			for name, count := range seatSummary.ToolCalls {
				seatSummary.ToolCallsPerHand[name] = safeRate(count, artifacts.Manifest.HandCount)
			}
		}
		if agent.MemoryExport != nil {
			memorySummary := agent.MemoryExport.Summary()
			seatSummary.MemoryExport = &memorySummary
		}
		summary.Seats = append(summary.Seats, seatSummary)
	}

	return summary, nil
}

func WriteSummary(sessionDir string, summary Summary) error {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("write eval summary %q: marshal: %w", sessionDir, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(sessionDir, "eval.json"), data, 0o644); err != nil {
		return fmt.Errorf("write eval summary %q: %w", sessionDir, err)
	}
	return nil
}

func collectSessionMetrics(hands []sessionlog.HandRecord, handCount int) SessionMetrics {
	metrics := SessionMetrics{}
	for _, hand := range hands {
		if isPreflopOnly(hand) {
			metrics.PreflopOnlyHands++
		}
		if hand.ShowdownReached {
			metrics.ShowdownHands++
		}
		for _, action := range hand.Actions {
			if action.Action == "auto_fold" || action.Action == "auto_check" || action.ForcedReason != "" {
				metrics.FallbackActionCount++
			}
		}
		swing := biggestPositiveResult(hand.Result)
		if swing > metrics.BiggestSwingHand.Chips {
			metrics.BiggestSwingHand = BiggestSwingHand{HandNumber: hand.HandNumber, Chips: swing}
		}
	}
	metrics.PreflopOnlyRate = safeRate(metrics.PreflopOnlyHands, handCount)
	metrics.ShowdownRate = safeRate(metrics.ShowdownHands, handCount)
	return metrics
}

func isPreflopOnly(hand sessionlog.HandRecord) bool {
	for _, action := range hand.Actions {
		if action.Street != "preflop" {
			return false
		}
	}
	return true
}

func biggestPositiveResult(results []sessionlog.HandResult) int {
	best := 0
	for _, result := range results {
		if result.ChipsDelta > best {
			best = result.ChipsDelta
		}
	}
	return best
}

func chipsDeltaForSeat(results map[int]sessionlog.ManifestSeatResult, seat int) int {
	result, ok := results[seat]
	if !ok {
		return 0
	}
	return result.ChipsDelta
}

func optionalRelPath(sessionDir, path string) *string {
	if path == "" {
		return nil
	}
	rel, err := filepath.Rel(sessionDir, path)
	if err != nil {
		value := filepath.ToSlash(path)
		return &value
	}
	value := filepath.ToSlash(rel)
	return &value
}

func durationSeconds(startedAt, endedAt string) (int64, error) {
	if startedAt == "" || endedAt == "" {
		return 0, nil
	}
	start, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return 0, fmt.Errorf("parse started_at: %w", err)
	}
	end, err := time.Parse(time.RFC3339, endedAt)
	if err != nil {
		return 0, fmt.Errorf("parse ended_at: %w", err)
	}
	return int64(end.Sub(start).Seconds()), nil
}

func safeRate(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
