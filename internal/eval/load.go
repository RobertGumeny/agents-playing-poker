package eval

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

const (
	piSessionFileName    = "pi-session.jsonl"
	memoryExportFileName = "memory-export.json"
	stderrFileName       = "stderr.log"
)

type SessionArtifacts struct {
	SessionDir string
	Manifest   sessionlog.Manifest
	Hands      []sessionlog.HandRecord
	Agents     []AgentArtifacts
}

type AgentArtifacts struct {
	Seat             sessionlog.ManifestSeat
	Dir              string
	PiSessionPath    string
	PiSession        *PiSessionLog
	MemoryExport     *MemoryExport
	MemoryExportPath string
	StderrPath       string
	RetrySummary     RetrySummary
}

type PiSessionLog struct {
	Events []PiSessionEvent
}

type PiSessionEvent struct {
	Type           string            `json:"type"`
	Message        *PiSessionMessage `json:"message,omitempty"`
	SessionScope   string            `json:"session_scope,omitempty"`
	SessionNumber  int               `json:"session_number,omitempty"`
	HandNumber     int               `json:"hand_number,omitempty"`
	DecisionNumber int               `json:"decision_number,omitempty"`
	Prompt         string            `json:"prompt,omitempty"`
}

type PiSessionMessage struct {
	Role    string                 `json:"role"`
	Content []PiSessionContentItem `json:"content,omitempty"`
}

type PiSessionContentItem struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type MemoryExport struct {
	Nodes []MemoryExportNode `json:"nodes"`
	Edges []MemoryExportEdge `json:"edges"`
}

type MemoryExportNode struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type MemoryExportEdge struct {
	Relation string `json:"relation"`
}

type MemoryExportSummary struct {
	NodeCount       int
	EdgeCount       int
	NodesByType     map[string]int
	EdgesByRelation map[string]int
}

type RetrySummary struct {
	AttemptFailures        int
	MalformedActionRetries int
	ExhaustedCount         int
	MaxAttemptsObserved    int
}

func LoadSession(sessionDir string) (SessionArtifacts, error) {
	manifest, err := sessionlog.ReadManifest(sessionDir)
	if err != nil {
		return SessionArtifacts{}, requiredArtifactError(filepath.Join(sessionDir, "manifest.json"), err)
	}
	hands, err := sessionlog.ReadHands(sessionDir)
	if err != nil {
		return SessionArtifacts{}, requiredArtifactError(filepath.Join(sessionDir, "hands.jsonl"), err)
	}
	if len(manifest.Matches) == 0 {
		return SessionArtifacts{}, fmt.Errorf("load eval session %q: manifest.json missing required matches[0] entry", sessionDir)
	}

	artifacts := SessionArtifacts{
		SessionDir: sessionDir,
		Manifest:   manifest,
		Hands:      hands,
		Agents:     make([]AgentArtifacts, 0, len(manifest.Matches[0].Seats)),
	}
	for _, seat := range manifest.Matches[0].Seats {
		agentDir := filepath.Join(sessionDir, "agents", seat.Name)
		agent, err := loadAgentArtifacts(agentDir, seat)
		if err != nil {
			return SessionArtifacts{}, err
		}
		artifacts.Agents = append(artifacts.Agents, agent)
	}

	return artifacts, nil
}

func loadAgentArtifacts(agentDir string, seat sessionlog.ManifestSeat) (AgentArtifacts, error) {
	agent := AgentArtifacts{
		Seat: seat,
		Dir:  agentDir,
	}

	piSessionPath := filepath.Join(agentDir, piSessionFileName)
	if _, err := os.Stat(piSessionPath); err == nil {
		log, err := ReadPiSessionLog(piSessionPath)
		if err != nil {
			return AgentArtifacts{}, fmt.Errorf("load eval agent %q: %w", seat.Name, err)
		}
		agent.PiSessionPath = piSessionPath
		agent.PiSession = &log
	} else if !errors.Is(err, os.ErrNotExist) {
		return AgentArtifacts{}, fmt.Errorf("load eval agent %q: stat %s: %w", seat.Name, piSessionPath, err)
	}

	memoryExportPath := filepath.Join(agentDir, memoryExportFileName)
	if _, err := os.Stat(memoryExportPath); err == nil {
		export, err := ReadMemoryExport(memoryExportPath)
		if err != nil {
			return AgentArtifacts{}, fmt.Errorf("load eval agent %q: %w", seat.Name, err)
		}
		agent.MemoryExportPath = memoryExportPath
		agent.MemoryExport = &export
	} else if !errors.Is(err, os.ErrNotExist) {
		return AgentArtifacts{}, fmt.Errorf("load eval agent %q: stat %s: %w", seat.Name, memoryExportPath, err)
	}

	stderrPath := filepath.Join(agentDir, stderrFileName)
	if _, err := os.Stat(stderrPath); err == nil {
		summary, err := ReadRetrySummary(stderrPath)
		if err != nil {
			return AgentArtifacts{}, fmt.Errorf("load eval agent %q: %w", seat.Name, err)
		}
		agent.StderrPath = stderrPath
		agent.RetrySummary = summary
	} else if !errors.Is(err, os.ErrNotExist) {
		return AgentArtifacts{}, fmt.Errorf("load eval agent %q: stat %s: %w", seat.Name, stderrPath, err)
	}

	return agent, nil
}

func ReadPiSessionLog(path string) (PiSessionLog, error) {
	f, err := os.Open(path)
	if err != nil {
		return PiSessionLog{}, fmt.Errorf("read pi session log %s: %w", path, err)
	}
	defer f.Close()

	var events []PiSessionEvent
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		var event PiSessionEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return PiSessionLog{}, fmt.Errorf("read pi session log %s: line %d: %w", path, len(events)+1, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return PiSessionLog{}, fmt.Errorf("read pi session log %s: scan: %w", path, err)
	}
	return PiSessionLog{Events: events}, nil
}

func ReadMemoryExport(path string) (MemoryExport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MemoryExport{}, fmt.Errorf("read memory export %s: %w", path, err)
	}
	var export MemoryExport
	if err := json.Unmarshal(data, &export); err != nil {
		return MemoryExport{}, fmt.Errorf("read memory export %s: %w", path, err)
	}
	if export.Nodes == nil {
		export.Nodes = []MemoryExportNode{}
	}
	if export.Edges == nil {
		export.Edges = []MemoryExportEdge{}
	}
	return export, nil
}

func ReadRetrySummary(path string) (RetrySummary, error) {
	f, err := os.Open(path)
	if err != nil {
		return RetrySummary{}, fmt.Errorf("read retry summary %s: %w", path, err)
	}
	defer f.Close()

	var summary RetrySummary
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		var attempt int
		var maxAttempts int
		var message string
		if _, err := fmt.Sscanf(line, "decision attempt %d/%d failed: %s", &attempt, &maxAttempts, &message); err == nil {
			summary.AttemptFailures++
			if maxAttempts > summary.MaxAttemptsObserved {
				summary.MaxAttemptsObserved = maxAttempts
			}
			if strings.Contains(line, "malformed action JSON") {
				summary.MalformedActionRetries++
			}
			continue
		}
		if strings.Contains(line, "exhausted retries; using safe fallback action") {
			summary.ExhaustedCount++
		}
	}
	if err := scanner.Err(); err != nil {
		return RetrySummary{}, fmt.Errorf("read retry summary %s: scan: %w", path, err)
	}
	return summary, nil
}

func (l PiSessionLog) DecisionPromptCount() int {
	count := 0
	for _, event := range l.Events {
		switch {
		case event.Type == "fake_pi_session":
			count++
		case event.Type == "message" && event.Message != nil && event.Message.Role == "user":
			count++
		}
	}
	return count
}

func (l PiSessionLog) ToolCallCounts() map[string]int {
	counts := map[string]int{}
	for _, event := range l.Events {
		if event.Type != "message" || event.Message == nil || event.Message.Role != "assistant" {
			continue
		}
		for _, item := range event.Message.Content {
			if item.Type != "toolCall" || item.Name == "" {
				continue
			}
			counts[item.Name]++
		}
	}
	return counts
}

func (m MemoryExport) Summary() MemoryExportSummary {
	summary := MemoryExportSummary{
		NodeCount:       len(m.Nodes),
		EdgeCount:       len(m.Edges),
		NodesByType:     map[string]int{},
		EdgesByRelation: map[string]int{},
	}
	for _, node := range m.Nodes {
		summary.NodesByType[node.Type]++
	}
	for _, edge := range m.Edges {
		summary.EdgesByRelation[edge.Relation]++
	}
	return summary
}

func SortedToolCallNames(counts map[string]int) []string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func requiredArtifactError(path string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load eval session: required artifact missing: %s", path)
	}
	return fmt.Errorf("load eval session: required artifact unreadable: %s: %w", path, err)
}
