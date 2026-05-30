package heuristicagent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/wire"
)

func TestAgentRunRespondsToLifecycleMessages(t *testing.T) {
	t.Parallel()

	var input bytes.Buffer
	encoder := json.NewEncoder(&input)
	mustEncode(t, encoder, wire.NewMessage(wire.MessageTypeSessionInit, "msg-1", "", wire.SessionInitPayload{}))
	mustEncode(t, encoder, wire.NewMessage(wire.MessageTypeHandStart, "msg-2", "", wire.HandStartPayload{YourHoleCards: []string{"As", "Ah"}}))
	callAmount := 1
	minRaise := 4
	maxRaise := 20
	mustEncode(t, encoder, wire.NewMessage(wire.MessageTypeYourTurn, "msg-3", "", wire.YourTurnPayload{
		HandNumber: 1,
		Street:     "preflop",
		Pot:        3,
		ToCall:     1,
		LegalActions: []wire.LegalActionOption{
			{Action: "fold"},
			{Action: "call", Amount: &callAmount},
			{Action: "raise", Min: &minRaise, Max: &maxRaise},
		},
	}))
	mustEncode(t, encoder, wire.NewMessage(wire.MessageTypeHandEnd, "msg-4", "", wire.HandEndPayload{}))
	mustEncode(t, encoder, wire.NewMessage(wire.MessageTypeSessionEnd, "msg-5", "", wire.SessionEndPayload{}))

	var output bytes.Buffer
	if err := Run(&input, &output); err != nil {
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
		t.Fatalf("action = %+v, want raise", action)
	}
	if action.Amount == nil || *action.Amount != minRaise {
		t.Fatalf("raise amount = %v, want %d", action.Amount, minRaise)
	}
}

func TestChooseActionFoldsWeakHandFacingPoorPrice(t *testing.T) {
	t.Parallel()

	agent := New()
	if err := agent.setHoleCards([]string{"7c", "2d"}); err != nil {
		t.Fatalf("setHoleCards() error = %v", err)
	}
	callAmount := 8
	minRaise := 12
	maxRaise := 20
	action, err := agent.chooseAction(wire.YourTurnPayload{
		Street: "preflop",
		Pot:    10,
		ToCall: callAmount,
		LegalActions: []wire.LegalActionOption{
			{Action: "fold"},
			{Action: "call", Amount: &callAmount},
			{Action: "raise", Min: &minRaise, Max: &maxRaise},
		},
	})
	if err != nil {
		t.Fatalf("chooseAction() error = %v", err)
	}
	if action.Action != "fold" {
		t.Fatalf("action = %+v, want fold", action)
	}
}

func TestChooseActionCallsWithPotOddsAndDraw(t *testing.T) {
	t.Parallel()

	agent := New()
	if err := agent.setHoleCards([]string{"Ah", "Qh"}); err != nil {
		t.Fatalf("setHoleCards() error = %v", err)
	}
	callAmount := 2
	action, err := agent.chooseAction(wire.YourTurnPayload{
		Street: "flop",
		Board:  []string{"Jh", "7c", "2h"},
		Pot:    12,
		ToCall: callAmount,
		LegalActions: []wire.LegalActionOption{
			{Action: "fold"},
			{Action: "call", Amount: &callAmount},
		},
	})
	if err != nil {
		t.Fatalf("chooseAction() error = %v", err)
	}
	if action.Action != "call" || action.Amount == nil || *action.Amount != callAmount {
		t.Fatalf("action = %+v, want call for %d", action, callAmount)
	}
}

func mustEncode(t *testing.T, encoder *json.Encoder, msg any) {
	t.Helper()
	if err := encoder.Encode(msg); err != nil {
		t.Fatalf("encode message: %v", err)
	}
}
