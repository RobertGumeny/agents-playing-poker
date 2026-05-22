package wire

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMessageRoundTrip(t *testing.T) {
	t.Parallel()

	amount1 := 1
	amount2 := 2
	min4 := 4
	max197 := 197

	tests := []struct {
		name string
		msg  any
	}{
		{
			name: "session_init",
			msg: NewMessage(MessageTypeSessionInit, "msg-1", "", SessionInitPayload{
				SessionID: "ses_2026-05-21_001",
				AgentName: "llm-akg",
				Match: MatchConfig{
					MatchID:            "mat_001",
					Seed:               12345,
					HandCount:          200,
					Variant:            "heads-up-nlhe",
					InfoRealism:        "showdown-only",
					StartingStack:      200,
					Blinds:             BlindLevel{SB: 1, BB: 2},
					DecisionDeadlineMS: 30000,
				},
				Seats:     []Seat{{Seat: 0, Name: "llm-akg"}, {Seat: 1, Name: "llm-fullhistory"}},
				YourSeat:  0,
				MemoryDir: "/abs/path/to/agent/dir",
			}),
		},
		{
			name: "hand_start",
			msg: NewMessage(MessageTypeHandStart, "msg-2", "", HandStartPayload{
				HandNumber:    47,
				DealerSeat:    1,
				Stacks:        map[int]int{0: 200, 1: 200},
				BlindsPosted:  []BlindPosting{{Seat: 0, Amount: 1}, {Seat: 1, Amount: 2}},
				YourHoleCards: []string{"As", "Kh"},
			}),
		},
		{
			name: "your_turn",
			msg: NewMessage(MessageTypeYourTurn, "msg-3", "", YourTurnPayload{
				HandNumber: 47,
				Street:     "flop",
				Board:      []string{"Td", "9h", "2c"},
				Pot:        6,
				ToCall:     2,
				Stacks:     map[int]int{0: 197, 1: 197},
				Seats:      []Seat{{Seat: 0, Name: "llm-akg"}, {Seat: 1, Name: "llm-fullhistory"}},
				ActionHistory: []ActionRecord{
					{Seat: 1, Action: "call", Amount: &amount1, Street: "preflop"},
					{Seat: 0, Action: "check", Street: "preflop"},
					{Seat: 0, Action: "bet", Amount: &amount2, Street: "flop"},
				},
				LegalActions: []LegalActionOption{
					{Action: "fold"},
					{Action: "call", Amount: &amount2},
					{Action: "raise", Min: &min4, Max: &max197},
				},
			}),
		},
		{
			name: "hand_end",
			msg: NewMessage(MessageTypeHandEnd, "msg-4", "", HandEndPayload{
				HandNumber: 47,
				Board:      []string{"Td", "9h", "2c", "5s", "Kc"},
				Showdown: map[int]ShowdownSeat{
					0: {HoleCards: []string{"As", "Kh"}, Rank: "two pair, kings and tens"},
					1: {HoleCards: []string{"9s", "9d"}, Rank: "three of a kind, nines"},
				},
				Result: []HandResult{{Seat: 1, ChipsDelta: 14}, {Seat: 0, ChipsDelta: -14}},
			}),
		},
		{
			name: "session_end",
			msg:  NewMessage(MessageTypeSessionEnd, "msg-5", "", SessionEndPayload{}),
		},
		{
			name: "session_ready",
			msg: NewMessage(MessageTypeSessionReady, "msg-6", "msg-1", SessionReadyPayload{
				Version: "heuristic/0.1.0",
			}),
		},
		{
			name: "action",
			msg: NewMessage(MessageTypeAction, "msg-7", "msg-3", ActionPayload{
				Action: "call",
				Amount: &amount2,
			}),
		},
		{
			name: "log",
			msg: NewMessage(MessageTypeLog, "msg-8", "", LogPayload{
				Level:   "info",
				Message: "raised turn blocker candidate",
				Fields:  map[string]any{"hand_number": float64(47), "street": "turn"},
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.msg)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			envelope, err := DecodeEnvelope(data)
			if err != nil {
				t.Fatalf("decode envelope: %v", err)
			}

			decoded, err := decodeByType(envelope)
			if err != nil {
				t.Fatalf("decode typed message: %v", err)
			}

			got, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("re-marshal decoded message: %v", err)
			}

			if string(data) != string(got) {
				t.Fatalf("round-trip mismatch\nwant: %s\n got: %s", data, got)
			}
		})
	}
}

func TestDecodeEnvelopeRejectsMalformedMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "invalid json",
			input:   `{"v":1,`,
			wantErr: "decode envelope",
		},
		{
			name:    "unsupported version",
			input:   `{"v":2,"type":"session_init","id":"msg-1","payload":{}}`,
			wantErr: "unsupported protocol version",
		},
		{
			name:    "unsupported type",
			input:   `{"v":1,"type":"bogus","id":"msg-1","payload":{}}`,
			wantErr: "unsupported message type",
		},
		{
			name:    "missing id",
			input:   `{"v":1,"type":"session_init","id":"","payload":{}}`,
			wantErr: "missing id",
		},
		{
			name:    "missing payload",
			input:   `{"v":1,"type":"session_init","id":"msg-1"}`,
			wantErr: "missing payload",
		},
		{
			name:    "missing reply correlation",
			input:   `{"v":1,"type":"action","id":"msg-1","payload":{"action":"fold"}}`,
			wantErr: "requires in_reply_to",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeEnvelope([]byte(tc.input))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %q", tc.wantErr, err)
			}
		})
	}
}

func decodeByType(envelope Envelope) (any, error) {
	switch envelope.Type {
	case MessageTypeSessionInit:
		var msg SessionInitMessage
		if err := json.Unmarshal(mustMarshalEnvelope(envelope), &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case MessageTypeHandStart:
		var msg HandStartMessage
		if err := json.Unmarshal(mustMarshalEnvelope(envelope), &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case MessageTypeYourTurn:
		var msg YourTurnMessage
		if err := json.Unmarshal(mustMarshalEnvelope(envelope), &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case MessageTypeHandEnd:
		var msg HandEndMessage
		if err := json.Unmarshal(mustMarshalEnvelope(envelope), &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case MessageTypeSessionEnd:
		var msg SessionEndMessage
		if err := json.Unmarshal(mustMarshalEnvelope(envelope), &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case MessageTypeSessionReady:
		var msg SessionReadyMessage
		if err := json.Unmarshal(mustMarshalEnvelope(envelope), &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case MessageTypeAction:
		var msg ActionMessage
		if err := json.Unmarshal(mustMarshalEnvelope(envelope), &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case MessageTypeLog:
		var msg LogMessage
		if err := json.Unmarshal(mustMarshalEnvelope(envelope), &msg); err != nil {
			return nil, err
		}
		return msg, nil
	default:
		return nil, nil
	}
}

func mustMarshalEnvelope(envelope Envelope) []byte {
	data, err := json.Marshal(envelope)
	if err != nil {
		panic(err)
	}
	return data
}
