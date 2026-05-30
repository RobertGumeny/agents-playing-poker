package benchreport

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

const (
	HistoricalAKGStrategy = "llm-akg"
	CanonicalAKGStrategy  = "llm-akg-recent"
)

type WarningCode string

const (
	WarningHistoricalStrategyCanonicalized WarningCode = "historical_strategy_canonicalized"
	WarningSessionIDMismatch               WarningCode = "session_id_mismatch"
	WarningNoMatches                       WarningCode = "no_matches"
	WarningMultipleMatches                 WarningCode = "multiple_matches"
	WarningHandCountMismatch               WarningCode = "hand_count_mismatch"
)

type ValidationWarning struct {
	Code    WarningCode
	Message string
}

type Session struct {
	Dir      string
	Metadata SessionMetadata
	Seats    map[int]Seat
	Hands    []Hand
	Warnings []ValidationWarning
}

type SessionMetadata struct {
	SessionID      string
	StartedAt      string
	EndedAt        string
	Seed           int64
	HandCount      int
	Variant        string
	InfoRealism    string
	StartingStack  int
	SmallBlind     int
	BigBlind       int
	MatchID        string
	Completed      bool
	ServerVersion  string
	AKGSpecVersion string
}

type Seat struct {
	Seat             int
	Strategy         string
	OriginalStrategy string
	Version          string
}

type Hand struct {
	MatchID         string
	HandNumber      int
	DealerSeat      int
	Deltas          []HandDelta
	Actions         []ActionRecord
	ShowdownReached bool
}

type HandDelta struct {
	Seat       int
	Strategy   string
	ChipsDelta int
}

type ActionRecord struct {
	Seat         int
	Strategy     string
	Action       string
	Amount       *int
	Street       string
	ForcedReason string
	Fallback     bool
}

func LoadSession(sessionDir string) (Session, error) {
	manifest, err := sessionlog.ReadManifest(sessionDir)
	if err != nil {
		return Session{}, fmt.Errorf("load session: %w", err)
	}
	hands, err := sessionlog.ReadHands(sessionDir)
	if err != nil {
		return Session{}, fmt.Errorf("load session: %w", err)
	}
	return NormalizeSession(sessionDir, manifest, hands), nil
}

func LoadSessions(sessionDirs []string) ([]Session, error) {
	sessions := make([]Session, 0, len(sessionDirs))
	for _, dir := range sessionDirs {
		session, err := LoadSession(dir)
		if err != nil {
			return nil, fmt.Errorf("load sessions: %s: %w", dir, err)
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func NormalizeSession(sessionDir string, manifest sessionlog.Manifest, records []sessionlog.HandRecord) Session {
	session := Session{
		Dir: sessionDir,
		Metadata: SessionMetadata{
			SessionID:      manifest.SessionID,
			StartedAt:      manifest.StartedAt,
			EndedAt:        manifest.EndedAt,
			Seed:           manifest.Seed,
			HandCount:      manifest.HandCount,
			Variant:        manifest.Variant,
			InfoRealism:    manifest.InfoRealism,
			StartingStack:  manifest.StartingStack,
			SmallBlind:     manifest.Blinds.SB,
			BigBlind:       manifest.Blinds.BB,
			ServerVersion:  manifest.ServerVersion,
			AKGSpecVersion: manifest.AKGSpecVersion,
		},
		Seats: make(map[int]Seat),
		Hands: make([]Hand, 0, len(records)),
	}

	if base := filepath.Base(sessionDir); base != "." && base != string(filepath.Separator) && manifest.SessionID != "" && base != manifest.SessionID {
		session.Warnings = append(session.Warnings, ValidationWarning{Code: WarningSessionIDMismatch, Message: fmt.Sprintf("session directory %q does not match manifest session_id %q", base, manifest.SessionID)})
	}
	if len(manifest.Matches) == 0 {
		session.Warnings = append(session.Warnings, ValidationWarning{Code: WarningNoMatches, Message: "manifest contains no matches"})
	} else {
		match := manifest.Matches[0]
		session.Metadata.MatchID = match.MatchID
		session.Metadata.Completed = match.Completed
		if len(manifest.Matches) > 1 {
			session.Warnings = append(session.Warnings, ValidationWarning{Code: WarningMultipleMatches, Message: "manifest contains multiple matches; normalized model uses the first match"})
		}
		for _, manifestSeat := range match.Seats {
			strategy := canonicalStrategy(manifestSeat.Name)
			if strategy != manifestSeat.Name {
				session.Warnings = append(session.Warnings, ValidationWarning{Code: WarningHistoricalStrategyCanonicalized, Message: fmt.Sprintf("seat %d strategy %q normalized to %q", manifestSeat.Seat, manifestSeat.Name, strategy)})
			}
			session.Seats[manifestSeat.Seat] = Seat{Seat: manifestSeat.Seat, Strategy: strategy, OriginalStrategy: manifestSeat.Name, Version: manifestSeat.Version}
		}
	}
	if manifest.HandCount != len(records) {
		session.Warnings = append(session.Warnings, ValidationWarning{Code: WarningHandCountMismatch, Message: fmt.Sprintf("manifest hand_count %d does not match hands.jsonl records %d", manifest.HandCount, len(records))})
	}

	for _, record := range records {
		hand := Hand{
			MatchID:         record.MatchID,
			HandNumber:      record.HandNumber,
			DealerSeat:      record.DealerSeat,
			ShowdownReached: record.ShowdownReached,
			Deltas:          make([]HandDelta, 0, len(record.Result)),
			Actions:         make([]ActionRecord, 0, len(record.Actions)),
		}
		for _, result := range record.Result {
			hand.Deltas = append(hand.Deltas, HandDelta{Seat: result.Seat, Strategy: strategyForSeat(session.Seats, result.Seat), ChipsDelta: result.ChipsDelta})
		}
		for _, action := range record.Actions {
			hand.Actions = append(hand.Actions, ActionRecord{
				Seat:         action.Seat,
				Strategy:     strategyForSeat(session.Seats, action.Seat),
				Action:       action.Action,
				Amount:       copyIntPtr(action.Amount),
				Street:       action.Street,
				ForcedReason: action.ForcedReason,
				Fallback:     action.Action == "auto_fold" || action.Action == "auto_check" || action.ForcedReason != "",
			})
		}
		session.Hands = append(session.Hands, hand)
	}

	sort.Slice(session.Warnings, func(i, j int) bool {
		if session.Warnings[i].Code == session.Warnings[j].Code {
			return session.Warnings[i].Message < session.Warnings[j].Message
		}
		return session.Warnings[i].Code < session.Warnings[j].Code
	})
	return session
}

func canonicalStrategy(strategy string) string {
	if strategy == HistoricalAKGStrategy {
		return CanonicalAKGStrategy
	}
	return strategy
}

func strategyForSeat(seats map[int]Seat, seat int) string {
	if s, ok := seats[seat]; ok {
		return s.Strategy
	}
	return fmt.Sprintf("seat%d", seat)
}

func copyIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}
