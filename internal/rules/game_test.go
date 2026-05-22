package rules

import (
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/deck"
)

func TestStartHandPostsBlindsAndRotatesDealer(t *testing.T) {
	match := mustHeadsUpMatch(t, 100, 1, 2)

	hand1 := mustStartHand(t, match, 1)
	if hand1.DealerSeat != 0 || hand1.SmallBlindSeat != 0 || hand1.BigBlindSeat != 1 {
		t.Fatalf("hand 1 blinds/dealer = dealer %d sb %d bb %d, want 0/0/1", hand1.DealerSeat, hand1.SmallBlindSeat, hand1.BigBlindSeat)
	}
	if hand1.ActingSeat != 0 {
		t.Fatalf("hand 1 acting seat = %d, want 0", hand1.ActingSeat)
	}
	if hand1.Players[0].Stack != 99 || hand1.Players[1].Stack != 98 {
		t.Fatalf("hand 1 stacks after blinds = [%d %d], want [99 98]", hand1.Players[0].Stack, hand1.Players[1].Stack)
	}

	match.Players[0].Stack = 0
	hand2 := mustStartHand(t, match, 2)
	if hand2.DealerSeat != 1 || hand2.SmallBlindSeat != 1 || hand2.BigBlindSeat != 0 {
		t.Fatalf("hand 2 blinds/dealer = dealer %d sb %d bb %d, want 1/1/0", hand2.DealerSeat, hand2.SmallBlindSeat, hand2.BigBlindSeat)
	}
	if hand2.Players[0].StartingStack != 100 {
		t.Fatalf("rebuy stack = %d, want 100", hand2.Players[0].StartingStack)
	}
	if hand2.Players[0].Stack != 98 || hand2.Players[1].Stack != 99 {
		t.Fatalf("hand 2 stacks after blinds = [%d %d], want [98 99]", hand2.Players[0].Stack, hand2.Players[1].Stack)
	}
}

func TestLegalActionsPreflopAndAfterCall(t *testing.T) {
	match := mustHeadsUpMatch(t, 100, 1, 2)
	hand := mustStartHand(t, match, 1)

	assertLegalActions(t, hand.LegalActions(), []LegalAction{
		{Type: ActionFold},
		{Type: ActionCall, Amount: 1},
		{Type: ActionRaise, Amount: 100, MinAmount: 4, MaxAmount: 100},
	})

	if err := hand.ApplyAction(Action{Seat: 0, Type: ActionCall, Amount: 1}); err != nil {
		t.Fatalf("ApplyAction(call) error = %v", err)
	}

	if hand.ActingSeat != 1 {
		t.Fatalf("acting seat after sb call = %d, want 1", hand.ActingSeat)
	}
	assertLegalActions(t, hand.LegalActions(), []LegalAction{
		{Type: ActionCheck},
		{Type: ActionRaise, Amount: 100, MinAmount: 4, MaxAmount: 100},
	})
}

func TestBettingRoundsAdvanceFromPreflopToShowdown(t *testing.T) {
	match := mustHeadsUpMatch(t, 100, 1, 2)
	hand := mustStartHand(t, match, 1)

	actions := []Action{
		{Seat: 0, Type: ActionCall, Amount: 1},
		{Seat: 1, Type: ActionCheck},
		{Seat: 1, Type: ActionCheck},
		{Seat: 0, Type: ActionCheck},
		{Seat: 1, Type: ActionCheck},
		{Seat: 0, Type: ActionCheck},
		{Seat: 1, Type: ActionCheck},
		{Seat: 0, Type: ActionCheck},
	}
	for _, action := range actions {
		if err := hand.ApplyAction(action); err != nil {
			t.Fatalf("ApplyAction(%+v) error = %v", action, err)
		}
	}

	if !hand.Complete {
		t.Fatalf("hand should be complete after river check/check")
	}
	if !hand.ShowdownPending {
		t.Fatalf("hand should require showdown after river check/check")
	}
	if hand.Street != StreetShowdown {
		t.Fatalf("street = %v, want showdown", hand.Street)
	}
	if len(hand.Board) != 5 {
		t.Fatalf("board card count = %d, want 5", len(hand.Board))
	}
}

func TestShortAllInRaiseDoesNotReopenAction(t *testing.T) {
	match := mustHeadsUpMatch(t, 100, 1, 2)
	hand := mustStartHand(t, match, 1)

	if err := hand.ApplyAction(Action{Seat: 0, Type: ActionCall, Amount: 1}); err != nil {
		t.Fatalf("ApplyAction(call) error = %v", err)
	}
	if err := hand.ApplyAction(Action{Seat: 1, Type: ActionCheck}); err != nil {
		t.Fatalf("ApplyAction(check) error = %v", err)
	}

	hand.Players[0].Stack = 3
	if err := hand.ApplyAction(Action{Seat: 1, Type: ActionBet, Amount: 2}); err != nil {
		t.Fatalf("ApplyAction(bet) error = %v", err)
	}
	if err := hand.ApplyAction(Action{Seat: 0, Type: ActionRaise, Amount: 3}); err != nil {
		t.Fatalf("ApplyAction(short raise) error = %v", err)
	}

	assertLegalActions(t, hand.LegalActions(), []LegalAction{
		{Type: ActionFold},
		{Type: ActionCall, Amount: 1},
	})
}

func TestFoldReturnsUncalledChipsAndAwardsContestedPot(t *testing.T) {
	match := mustHeadsUpMatch(t, 100, 1, 2)
	hand := mustStartHand(t, match, 1)

	if err := hand.ApplyAction(Action{Seat: 0, Type: ActionRaise, Amount: 6}); err != nil {
		t.Fatalf("ApplyAction(raise) error = %v", err)
	}
	if err := hand.ApplyAction(Action{Seat: 1, Type: ActionFold}); err != nil {
		t.Fatalf("ApplyAction(fold) error = %v", err)
	}

	if !hand.Complete || hand.ShowdownPending {
		t.Fatalf("folded hand should be complete without showdown")
	}
	if hand.Result == nil {
		t.Fatalf("folded hand should have a result")
	}
	if hand.Result.Pot != 4 {
		t.Fatalf("contested pot = %d, want 4", hand.Result.Pot)
	}
	if hand.Players[0].Stack != 102 || hand.Players[1].Stack != 98 {
		t.Fatalf("final stacks = [%d %d], want [102 98]", hand.Players[0].Stack, hand.Players[1].Stack)
	}
	if hand.Result.Deltas[0].Delta != 2 || hand.Result.Deltas[1].Delta != -2 {
		t.Fatalf("deltas = %+v, want +2/-2", hand.Result.Deltas)
	}

	if err := match.FinalizeHand(hand); err != nil {
		t.Fatalf("FinalizeHand() error = %v", err)
	}
	if match.Players[0].Stack != 102 || match.Players[1].Stack != 98 {
		t.Fatalf("match stacks = [%d %d], want [102 98]", match.Players[0].Stack, match.Players[1].Stack)
	}
}

func mustHeadsUpMatch(t *testing.T, startingStack int, smallBlind int, bigBlind int) *MatchState {
	t.Helper()
	match, err := NewHeadsUpMatch(startingStack, smallBlind, bigBlind)
	if err != nil {
		t.Fatalf("NewHeadsUpMatch() error = %v", err)
	}
	return match
}

func mustStartHand(t *testing.T, match *MatchState, handNumber int) *HandState {
	t.Helper()
	dealer := deck.NewDealer(99)
	deal, err := dealer.DealHoldemHand(handNumber, 2)
	if err != nil {
		t.Fatalf("DealHoldemHand() error = %v", err)
	}
	hand, err := match.StartHand(handNumber, deal)
	if err != nil {
		t.Fatalf("StartHand() error = %v", err)
	}
	return hand
}

func assertLegalActions(t *testing.T, got []LegalAction, want []LegalAction) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("legal action count = %d, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("legal action %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
