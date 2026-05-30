package benchreport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

func TestLoadSessionValid(t *testing.T) {
	dir := writeSession(t, "valid-session", manifestFor("valid-session", "llm-stateless", "heuristic"), []sessionlog.HandRecord{
		{
			MatchID:         "match-1",
			HandNumber:      1,
			DealerSeat:      0,
			Actions:         []sessionlog.HandAction{{Seat: 0, Action: "raise", Amount: intPtr(6), Street: "preflop"}, {Seat: 1, Action: "fold", Street: "preflop"}},
			ShowdownReached: false,
			Result:          []sessionlog.HandResult{{Seat: 0, ChipsDelta: 3}, {Seat: 1, ChipsDelta: -3}},
		},
	})

	session, err := LoadSession(dir)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if session.Metadata.SessionID != "valid-session" {
		t.Fatalf("SessionID = %q, want valid-session", session.Metadata.SessionID)
	}
	if session.Metadata.MatchID != "match-1" {
		t.Fatalf("MatchID = %q, want match-1", session.Metadata.MatchID)
	}
	if session.Seats[0].Strategy != "llm-stateless" || session.Seats[1].Strategy != "heuristic" {
		t.Fatalf("unexpected seats: %#v", session.Seats)
	}
	if len(session.Hands) != 1 {
		t.Fatalf("hands len = %d, want 1", len(session.Hands))
	}
	if got := session.Hands[0].Deltas[0]; got.Strategy != "llm-stateless" || got.ChipsDelta != 3 {
		t.Fatalf("unexpected delta: %#v", got)
	}
	if got := session.Hands[0].Actions[0]; got.Strategy != "llm-stateless" || got.Action != "raise" || got.Amount == nil || *got.Amount != 6 || got.Fallback {
		t.Fatalf("unexpected action: %#v", got)
	}
	if len(session.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", session.Warnings)
	}
}

func TestLoadSessionMissingRequiredArtifact(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadSession(dir); err == nil || !strings.Contains(err.Error(), "read manifest") {
		t.Fatalf("LoadSession() error = %v, want read manifest error", err)
	}

	dir = t.TempDir()
	writeManifestFile(t, dir, manifestFor(filepath.Base(dir), "llm-stateless", "heuristic"))
	if _, err := LoadSession(dir); err == nil || !strings.Contains(err.Error(), "read hands") {
		t.Fatalf("LoadSession() error = %v, want read hands error", err)
	}
}

func TestLoadSessionMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hands.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSession(dir); err == nil || !strings.Contains(err.Error(), "read manifest") {
		t.Fatalf("LoadSession() error = %v, want malformed manifest error", err)
	}

	dir = t.TempDir()
	writeManifestFile(t, dir, manifestFor(filepath.Base(dir), "llm-stateless", "heuristic"))
	if err := os.WriteFile(filepath.Join(dir, "hands.jsonl"), []byte("not-json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSession(dir); err == nil || !strings.Contains(err.Error(), "read hands: line 1") {
		t.Fatalf("LoadSession() error = %v, want malformed hands error", err)
	}
}

func TestLoadSessionCanonicalizesHistoricalAKGName(t *testing.T) {
	dir := writeSession(t, "historical-session", manifestFor("historical-session", "llm-akg", "llm-stateless"), []sessionlog.HandRecord{
		{
			MatchID:         "match-1",
			HandNumber:      1,
			DealerSeat:      0,
			Actions:         []sessionlog.HandAction{{Seat: 0, Action: "auto_check", Street: "preflop", ForcedReason: "decision_timeout"}},
			ShowdownReached: true,
			Result:          []sessionlog.HandResult{{Seat: 0, ChipsDelta: -2}, {Seat: 1, ChipsDelta: 2}},
		},
	})

	session, err := LoadSession(dir)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if got := session.Seats[0]; got.Strategy != "llm-akg-recent" || got.OriginalStrategy != "llm-akg" {
		t.Fatalf("seat 0 = %#v, want canonical strategy with original preserved", got)
	}
	if got := session.Hands[0].Deltas[0].Strategy; got != "llm-akg-recent" {
		t.Fatalf("delta strategy = %q, want llm-akg-recent", got)
	}
	if got := session.Hands[0].Actions[0]; got.Strategy != "llm-akg-recent" || !got.Fallback {
		t.Fatalf("action = %#v, want canonical strategy fallback", got)
	}
	if !hasWarning(session.Warnings, WarningHistoricalStrategyCanonicalized) {
		t.Fatalf("warnings = %#v, want historical canonicalization warning", session.Warnings)
	}
}

func manifestFor(sessionID string, seat0 string, seat1 string) sessionlog.Manifest {
	return sessionlog.Manifest{
		SessionID:     sessionID,
		StartedAt:     "2026-05-26T00:00:00Z",
		EndedAt:       "2026-05-26T00:01:00Z",
		Seed:          42,
		HandCount:     1,
		Variant:       "heads-up-nlhe",
		InfoRealism:   "showdown-only",
		StartingStack: 200,
		Blinds:        sessionlog.BlindLevel{SB: 1, BB: 2},
		Matches: []sessionlog.ManifestMatch{{
			MatchID: "match-1",
			Seats: []sessionlog.ManifestSeat{
				{Seat: 0, Name: seat0, Version: "test"},
				{Seat: 1, Name: seat1, Version: "test"},
			},
			Result:    map[int]sessionlog.ManifestSeatResult{0: {ChipsDelta: 3}, 1: {ChipsDelta: -3}},
			Completed: true,
		}},
		ServerVersion:  "test",
		AKGSpecVersion: "test",
	}
}

func writeSession(t *testing.T, sessionID string, manifest sessionlog.Manifest, hands []sessionlog.HandRecord) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), sessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifestFile(t, dir, manifest)
	var data strings.Builder
	for _, hand := range hands {
		line, err := marshalJSONLine(hand)
		if err != nil {
			t.Fatal(err)
		}
		data.WriteString(line)
	}
	if err := os.WriteFile(filepath.Join(dir, "hands.jsonl"), []byte(data.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeManifestFile(t *testing.T, dir string, manifest sessionlog.Manifest) {
	t.Helper()
	data, err := marshalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func marshalJSONLine(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

func hasWarning(warnings []ValidationWarning, code WarningCode) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func intPtr(v int) *int {
	return &v
}
