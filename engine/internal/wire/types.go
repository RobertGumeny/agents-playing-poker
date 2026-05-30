package wire

import (
	"encoding/json"
	"fmt"
)

const ProtocolVersion = 1

type MessageType string

const (
	MessageTypeSessionInit  MessageType = "session_init"
	MessageTypeHandStart    MessageType = "hand_start"
	MessageTypeYourTurn     MessageType = "your_turn"
	MessageTypeHandEnd      MessageType = "hand_end"
	MessageTypeSessionEnd   MessageType = "session_end"
	MessageTypeSessionReady MessageType = "session_ready"
	MessageTypeAction       MessageType = "action"
	MessageTypeLog          MessageType = "log"
)

func (t MessageType) Valid() bool {
	switch t {
	case MessageTypeSessionInit,
		MessageTypeHandStart,
		MessageTypeYourTurn,
		MessageTypeHandEnd,
		MessageTypeSessionEnd,
		MessageTypeSessionReady,
		MessageTypeAction,
		MessageTypeLog:
		return true
	default:
		return false
	}
}

type Envelope struct {
	V         int             `json:"v"`
	Type      MessageType     `json:"type"`
	ID        string          `json:"id"`
	InReplyTo string          `json:"in_reply_to,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

func DecodeEnvelope(data []byte) (Envelope, error) {
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	if err := envelope.Validate(); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

func (e Envelope) Validate() error {
	if e.V != ProtocolVersion {
		return fmt.Errorf("validate envelope: unsupported protocol version %d", e.V)
	}
	if !e.Type.Valid() {
		return fmt.Errorf("validate envelope: unsupported message type %q", e.Type)
	}
	if e.ID == "" {
		return fmt.Errorf("validate envelope: missing id")
	}
	if len(e.Payload) == 0 {
		return fmt.Errorf("validate envelope: missing payload")
	}
	if requiresReplyCorrelation(e.Type) && e.InReplyTo == "" {
		return fmt.Errorf("validate envelope: message type %q requires in_reply_to", e.Type)
	}
	return nil
}

func (e Envelope) DecodePayload(dst any) error {
	if err := json.Unmarshal(e.Payload, dst); err != nil {
		return fmt.Errorf("decode %s payload: %w", e.Type, err)
	}
	return nil
}

type Message[T any] struct {
	V         int         `json:"v"`
	Type      MessageType `json:"type"`
	ID        string      `json:"id"`
	InReplyTo string      `json:"in_reply_to,omitempty"`
	Payload   T           `json:"payload"`
}

func NewMessage[T any](messageType MessageType, id string, inReplyTo string, payload T) Message[T] {
	return Message[T]{
		V:         ProtocolVersion,
		Type:      messageType,
		ID:        id,
		InReplyTo: inReplyTo,
		Payload:   payload,
	}
}

type MatchConfig struct {
	MatchID            string     `json:"match_id"`
	Seed               int64      `json:"seed"`
	HandCount          int        `json:"hand_count"`
	Variant            string     `json:"variant"`
	InfoRealism        string     `json:"info_realism"`
	StartingStack      int        `json:"starting_stack"`
	Blinds             BlindLevel `json:"blinds"`
	DecisionDeadlineMS int        `json:"decision_deadline_ms"`
}

type BlindLevel struct {
	SB int `json:"sb"`
	BB int `json:"bb"`
}

type Seat struct {
	Seat int    `json:"seat"`
	Name string `json:"name"`
}

type BlindPosting struct {
	Seat   int `json:"seat"`
	Amount int `json:"amount"`
}

type ActionRecord struct {
	Seat   int    `json:"seat"`
	Action string `json:"action"`
	Amount *int   `json:"amount,omitempty"`
	Street string `json:"street"`
}

type LegalActionOption struct {
	Action string `json:"action"`
	Amount *int   `json:"amount,omitempty"`
	Min    *int   `json:"min,omitempty"`
	Max    *int   `json:"max,omitempty"`
}

type ShowdownSeat struct {
	HoleCards []string `json:"hole_cards"`
	Rank      string   `json:"rank"`
}

type HandResult struct {
	Seat       int `json:"seat"`
	ChipsDelta int `json:"chips_delta"`
}

type SessionInitPayload struct {
	SessionID string      `json:"session_id"`
	AgentName string      `json:"agent_name"`
	Match     MatchConfig `json:"match"`
	Seats     []Seat      `json:"seats"`
	YourSeat  int         `json:"your_seat"`
	MemoryDir string      `json:"memory_dir"`
}

type HandStartPayload struct {
	HandNumber    int            `json:"hand_number"`
	DealerSeat    int            `json:"dealer_seat"`
	Stacks        map[int]int    `json:"stacks"`
	BlindsPosted  []BlindPosting `json:"blinds_posted"`
	YourHoleCards []string       `json:"your_hole_cards"`
}

type YourTurnPayload struct {
	HandNumber    int                 `json:"hand_number"`
	Street        string              `json:"street"`
	Board         []string            `json:"board"`
	Pot           int                 `json:"pot"`
	ToCall        int                 `json:"to_call"`
	Stacks        map[int]int         `json:"stacks"`
	Seats         []Seat              `json:"seats"`
	ActionHistory []ActionRecord      `json:"action_history"`
	LegalActions  []LegalActionOption `json:"legal_actions"`
}

type HandEndPayload struct {
	HandNumber      int                  `json:"hand_number"`
	Board           []string             `json:"board"`
	ActionHistory   []ActionRecord       `json:"action_history"`
	ShowdownReached bool                 `json:"showdown_reached"`
	Showdown        map[int]ShowdownSeat `json:"showdown"`
	Result          []HandResult         `json:"result"`
}

type SessionEndPayload struct{}

type SessionReadyPayload struct {
	Version string `json:"version"`
}

type ActionPayload struct {
	Action string `json:"action"`
	Amount *int   `json:"amount,omitempty"`
}

type LogPayload struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

type SessionInitMessage = Message[SessionInitPayload]
type HandStartMessage = Message[HandStartPayload]
type YourTurnMessage = Message[YourTurnPayload]
type HandEndMessage = Message[HandEndPayload]
type SessionEndMessage = Message[SessionEndPayload]
type SessionReadyMessage = Message[SessionReadyPayload]
type ActionMessage = Message[ActionPayload]
type LogMessage = Message[LogPayload]

func requiresReplyCorrelation(messageType MessageType) bool {
	switch messageType {
	case MessageTypeSessionReady, MessageTypeAction:
		return true
	default:
		return false
	}
}
