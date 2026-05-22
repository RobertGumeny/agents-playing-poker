package randomagent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/wire"
)

func TestAgentRunRespondsToLifecycleMessages(t *testing.T) {
	t.Parallel()

	var input bytes.Buffer
	encoder := json.NewEncoder(&input)
	if err := encoder.Encode(wire.NewMessage(wire.MessageTypeSessionInit, "msg-1", "", wire.SessionInitPayload{})); err != nil {
		t.Fatalf("encode session_init: %v", err)
	}
	if err := encoder.Encode(wire.NewMessage(wire.MessageTypeHandStart, "msg-2", "", wire.HandStartPayload{})); err != nil {
		t.Fatalf("encode hand_start: %v", err)
	}
	callAmount := 3
	minRaise := 5
	maxRaise := 9
	if err := encoder.Encode(wire.NewMessage(wire.MessageTypeYourTurn, "msg-3", "", wire.YourTurnPayload{
		LegalActions: []wire.LegalActionOption{
			{Action: "fold"},
			{Action: "call", Amount: &callAmount},
			{Action: "raise", Min: &minRaise, Max: &maxRaise},
		},
	})); err != nil {
		t.Fatalf("encode your_turn: %v", err)
	}
	if err := encoder.Encode(wire.NewMessage(wire.MessageTypeHandEnd, "msg-4", "", wire.HandEndPayload{})); err != nil {
		t.Fatalf("encode hand_end: %v", err)
	}
	if err := encoder.Encode(wire.NewMessage(wire.MessageTypeSessionEnd, "msg-5", "", wire.SessionEndPayload{})); err != nil {
		t.Fatalf("encode session_end: %v", err)
	}

	var output bytes.Buffer
	if err := Run(&input, &output, rand.New(rand.NewSource(1))); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	scanner := bufio.NewScanner(&output)
	var responses []wire.Envelope
	for scanner.Scan() {
		envelope, err := wire.DecodeEnvelope(scanner.Bytes())
		if err != nil {
			t.Fatalf("DecodeEnvelope() error = %v", err)
		}
		responses = append(responses, envelope)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner.Err() = %v", err)
	}
	if len(responses) != 2 {
		t.Fatalf("response count = %d, want 2", len(responses))
	}
	if responses[0].Type != wire.MessageTypeSessionReady || responses[0].InReplyTo != "msg-1" {
		t.Fatalf("first response = %+v, want session_ready replying to msg-1", responses[0])
	}
	var ready wire.SessionReadyPayload
	if err := responses[0].DecodePayload(&ready); err != nil {
		t.Fatalf("decode session_ready payload: %v", err)
	}
	if ready.Version != Version {
		t.Fatalf("session_ready version = %q, want %q", ready.Version, Version)
	}
	if responses[1].Type != wire.MessageTypeAction || responses[1].InReplyTo != "msg-3" {
		t.Fatalf("second response = %+v, want action replying to msg-3", responses[1])
	}
	var action wire.ActionPayload
	if err := responses[1].DecodePayload(&action); err != nil {
		t.Fatalf("decode action payload: %v", err)
	}
	if action.Action != "raise" {
		t.Fatalf("action = %+v, want seeded raise", action)
	}
	if action.Amount == nil || *action.Amount < minRaise || *action.Amount > maxRaise {
		t.Fatalf("raise amount = %v, want within [%d,%d]", action.Amount, minRaise, maxRaise)
	}
}

func TestChooseActionPreservesServerAdvertisedCallAmount(t *testing.T) {
	t.Parallel()

	agent := New(rand.New(rand.NewSource(0)))
	amount := 7
	action, err := agent.chooseAction([]wire.LegalActionOption{{Action: "call", Amount: &amount}})
	if err != nil {
		t.Fatalf("chooseAction() error = %v", err)
	}
	if action.Action != "call" || action.Amount == nil || *action.Amount != amount {
		t.Fatalf("action = %+v, want call for %d", action, amount)
	}
}
