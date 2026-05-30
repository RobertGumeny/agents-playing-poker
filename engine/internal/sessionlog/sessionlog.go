package sessionlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type BlindLevel struct {
	SB int `json:"sb"`
	BB int `json:"bb"`
}

type ManifestSeat struct {
	Seat    int    `json:"seat"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type ManifestSeatResult struct {
	ChipsDelta int `json:"chips_delta"`
}

type ManifestMatch struct {
	MatchID   string                     `json:"match_id"`
	Seats     []ManifestSeat             `json:"seats"`
	Result    map[int]ManifestSeatResult `json:"result"`
	Completed bool                       `json:"completed"`
}

type Manifest struct {
	SessionID      string          `json:"session_id"`
	StartedAt      string          `json:"started_at"`
	EndedAt        string          `json:"ended_at"`
	Seed           int64           `json:"seed"`
	HandCount      int             `json:"hand_count"`
	Variant        string          `json:"variant"`
	InfoRealism    string          `json:"info_realism"`
	StartingStack  int             `json:"starting_stack"`
	Blinds         BlindLevel      `json:"blinds"`
	Matches        []ManifestMatch `json:"matches"`
	ServerVersion  string          `json:"server_version"`
	AKGSpecVersion string          `json:"akg_spec_version"`
}

type BlindPosting struct {
	Seat   int `json:"seat"`
	Amount int `json:"amount"`
}

type HandAction struct {
	Seat         int    `json:"seat"`
	Action       string `json:"action"`
	Amount       *int   `json:"amount,omitempty"`
	Street       string `json:"street"`
	ForcedReason string `json:"forced_reason,omitempty"`
}

type HandResult struct {
	Seat       int `json:"seat"`
	ChipsDelta int `json:"chips_delta"`
}

type HandRecord struct {
	MatchID         string           `json:"match_id"`
	HandNumber      int              `json:"hand_number"`
	DealerSeat      int              `json:"dealer_seat"`
	StacksStart     map[int]int      `json:"stacks_start"`
	BlindsPosted    []BlindPosting   `json:"blinds_posted"`
	HoleCards       map[int][]string `json:"hole_cards"`
	Board           []string         `json:"board"`
	Actions         []HandAction     `json:"actions"`
	ShowdownReached bool             `json:"showdown_reached"`
	Result          []HandResult     `json:"result"`
}

type Writer struct {
	sessionDir string
	handsFile  *os.File
}

func New(rootDir string, sessionID string) (*Writer, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("new session log writer: session id is required")
	}
	sessionDir := filepath.Join(rootDir, sessionID)
	if err := os.MkdirAll(filepath.Join(sessionDir, "agents"), 0o755); err != nil {
		return nil, fmt.Errorf("new session log writer: create session dir: %w", err)
	}
	handsFile, err := os.Create(filepath.Join(sessionDir, "hands.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("new session log writer: create hands.jsonl: %w", err)
	}
	return &Writer{sessionDir: sessionDir, handsFile: handsFile}, nil
}

func (w *Writer) SessionDir() string {
	return w.sessionDir
}

func (w *Writer) AgentDir(agentName string) (string, error) {
	if agentName == "" {
		return "", fmt.Errorf("agent dir: agent name is required")
	}
	dir := filepath.Join(w.sessionDir, "agents", agentName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("agent dir: create %s: %w", dir, err)
	}
	return filepath.Abs(dir)
}

func (w *Writer) AppendHand(record HandRecord) error {
	if err := json.NewEncoder(w.handsFile).Encode(record); err != nil {
		return fmt.Errorf("append hand: %w", err)
	}
	return nil
}

func (w *Writer) WriteManifest(manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("write manifest: marshal: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(w.sessionDir, "manifest.json"), data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

const runnerErrorsFileName = "runner-errors.log"

// AppendRunnerError appends a timestamped error line to runner-errors.log in
// the session directory. Errors here are non-fatal post-match failures (e.g.
// memory export) that would otherwise only appear in transient stdout.
func AppendRunnerError(sessionDir, msg string) error {
	path := filepath.Join(sessionDir, runnerErrorsFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("append runner error: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339), msg)
	return err
}

func (w *Writer) Close() error {
	if w == nil || w.handsFile == nil {
		return nil
	}
	if err := w.handsFile.Close(); err != nil {
		return fmt.Errorf("close session log writer: %w", err)
	}
	w.handsFile = nil
	return nil
}
