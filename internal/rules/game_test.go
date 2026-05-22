package rules

import (
	"slices"
	"testing"

	"github.com/RobertGumeny/agent-poker/internal/deck"
)

func TestBlindRotationAndRebuy(t *testing.T) {
	match := mustHeadsUpMatch(t, 100, 1, 2)

	testCases := []struct {
		name              string
		handNumber        int
		preHandStacks     map[int]int
		wantDealerSeat    int
		wantSmallBlind    int
		wantBigBlind      int
		wantActingSeat    int
		wantStartingStack [2]int
		wantStacks        [2]int
	}{
		{
			name:              "hand one posts blinds with seat zero on button",
			handNumber:        1,
			wantDealerSeat:    0,
			wantSmallBlind:    0,
			wantBigBlind:      1,
			wantActingSeat:    0,
			wantStartingStack: [2]int{100, 100},
			wantStacks:        [2]int{99, 98},
		},
		{
			name:              "busted player auto-rebuys on next hand and blinds rotate",
			handNumber:        2,
			preHandStacks:     map[int]int{0: 0},
			wantDealerSeat:    1,
			wantSmallBlind:    1,
			wantBigBlind:      0,
			wantActingSeat:    1,
			wantStartingStack: [2]int{100, 100},
			wantStacks:        [2]int{98, 99},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for seat, stack := range tc.preHandStacks {
				match.Players[seat].Stack = stack
			}

			hand := mustStartHand(t, match, tc.handNumber)
			if hand.DealerSeat != tc.wantDealerSeat || hand.SmallBlindSeat != tc.wantSmallBlind || hand.BigBlindSeat != tc.wantBigBlind {
				t.Fatalf("dealer/sb/bb = %d/%d/%d, want %d/%d/%d", hand.DealerSeat, hand.SmallBlindSeat, hand.BigBlindSeat, tc.wantDealerSeat, tc.wantSmallBlind, tc.wantBigBlind)
			}
			if hand.ActingSeat != tc.wantActingSeat {
				t.Fatalf("acting seat = %d, want %d", hand.ActingSeat, tc.wantActingSeat)
			}

			gotStarting := [2]int{hand.Players[0].StartingStack, hand.Players[1].StartingStack}
			if gotStarting != tc.wantStartingStack {
				t.Fatalf("starting stacks = %v, want %v", gotStarting, tc.wantStartingStack)
			}
			gotStacks := [2]int{hand.Players[0].Stack, hand.Players[1].Stack}
			if gotStacks != tc.wantStacks {
				t.Fatalf("stacks after blinds = %v, want %v", gotStacks, tc.wantStacks)
			}
		})
	}
}

func TestLegalActions(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func(t *testing.T) *HandState
		wantLegal  []LegalAction
		wantToCall int
	}{
		{
			name: "small blind opens preflop facing big blind",
			setup: func(t *testing.T) *HandState {
				return mustStartHand(t, mustHeadsUpMatch(t, 100, 1, 2), 1)
			},
			wantLegal: []LegalAction{
				{Type: ActionFold},
				{Type: ActionCall, Amount: 1},
				{Type: ActionRaise, Amount: 100, MinAmount: 4, MaxAmount: 100},
			},
			wantToCall: 1,
		},
		{
			name: "big blind may check or raise after small blind completes",
			setup: func(t *testing.T) *HandState {
				hand := mustStartHand(t, mustHeadsUpMatch(t, 100, 1, 2), 1)
				mustApplyAction(t, hand, Action{Seat: 0, Type: ActionCall, Amount: 1})
				return hand
			},
			wantLegal: []LegalAction{
				{Type: ActionCheck},
				{Type: ActionRaise, Amount: 100, MinAmount: 4, MaxAmount: 100},
			},
			wantToCall: 0,
		},
		{
			name: "short all-in raise does not reopen betting",
			setup: func(t *testing.T) *HandState {
				hand := mustStartHand(t, mustHeadsUpMatch(t, 100, 1, 2), 1)
				mustApplyAction(t, hand, Action{Seat: 0, Type: ActionCall, Amount: 1})
				mustApplyAction(t, hand, Action{Seat: 1, Type: ActionCheck})
				hand.Players[0].Stack = 3
				mustApplyAction(t, hand, Action{Seat: 1, Type: ActionBet, Amount: 2})
				mustApplyAction(t, hand, Action{Seat: 0, Type: ActionRaise, Amount: 3})
				return hand
			},
			wantLegal: []LegalAction{
				{Type: ActionFold},
				{Type: ActionCall, Amount: 1},
			},
			wantToCall: 1,
		},
		{
			name: "small blind can call full blind when big blind is all-in short",
			setup: func(t *testing.T) *HandState {
				match := mustHeadsUpMatch(t, 100, 1, 2)
				match.Players[0].Stack = 1
				hand := mustStartHand(t, match, 2)
				return hand
			},
			wantLegal: []LegalAction{
				{Type: ActionFold},
				{Type: ActionCall, Amount: 1},
				{Type: ActionRaise, Amount: 100, MinAmount: 4, MaxAmount: 100},
			},
			wantToCall: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hand := tc.setup(t)
			if got := hand.ToCall(hand.ActingSeat); got != tc.wantToCall {
				t.Fatalf("ToCall(%d) = %d, want %d", hand.ActingSeat, got, tc.wantToCall)
			}
			assertLegalActions(t, hand.LegalActions(), tc.wantLegal)
		})
	}
}

func TestBettingProgressionAndPotAccounting(t *testing.T) {
	testCases := []struct {
		name              string
		preHandStacks     map[int]int
		deal              *deck.HoldemDeal
		actions           []Action
		wantStreet        Street
		wantBoardCount    int
		wantComplete      bool
		wantShowdown      bool
		wantPot           int
		wantStacks        [2]int
		wantDeltas        [2]int
		wantMatchStacks   [2]int
		wantWinningSeats  []int
		wantShowdownHands int
		resolveShowdown   bool
	}{
		{
			name: "check down reaches showdown with full board exposed",
			deal: ptrDeal(mustDeal(t,
				[][]string{{"Ac", "Kd"}, {"Qh", "Js"}},
				[]string{"As", "Ks", "Qs", "Js", "Ts"},
			)),
			actions: []Action{
				{Seat: 0, Type: ActionCall, Amount: 1},
				{Seat: 1, Type: ActionCheck},
				{Seat: 1, Type: ActionCheck},
				{Seat: 0, Type: ActionCheck},
				{Seat: 1, Type: ActionCheck},
				{Seat: 0, Type: ActionCheck},
				{Seat: 1, Type: ActionCheck},
				{Seat: 0, Type: ActionCheck},
			},
			wantStreet:        StreetShowdown,
			wantBoardCount:    5,
			wantComplete:      true,
			wantShowdown:      true,
			wantPot:           4,
			wantStacks:        [2]int{100, 100},
			wantDeltas:        [2]int{0, 0},
			wantMatchStacks:   [2]int{100, 100},
			wantWinningSeats:  []int{0, 1},
			wantShowdownHands: 2,
			resolveShowdown:   true,
		},
		{
			name: "fold returns uncalled chips and awards only contested pot",
			actions: []Action{
				{Seat: 0, Type: ActionRaise, Amount: 6},
				{Seat: 1, Type: ActionFold},
			},
			wantStreet:        StreetPreflop,
			wantBoardCount:    0,
			wantComplete:      true,
			wantShowdown:      false,
			wantPot:           4,
			wantStacks:        [2]int{102, 98},
			wantDeltas:        [2]int{2, -2},
			wantMatchStacks:   [2]int{102, 98},
			wantWinningSeats:  []int{0},
			wantShowdownHands: 0,
		},
		{
			name:          "short all-in blind closes action after full call and refunds excess",
			preHandStacks: map[int]int{1: 1},
			deal: ptrDeal(mustDeal(t,
				[][]string{{"Kc", "Qd"}, {"As", "Ad"}},
				[]string{"2c", "3d", "4h", "5s", "7c"},
			)),
			actions: []Action{
				{Seat: 0, Type: ActionCall, Amount: 1},
			},
			wantStreet:        StreetShowdown,
			wantBoardCount:    5,
			wantComplete:      true,
			wantShowdown:      true,
			wantPot:           2,
			wantStacks:        [2]int{99, 2},
			wantDeltas:        [2]int{-1, 1},
			wantMatchStacks:   [2]int{99, 2},
			wantWinningSeats:  []int{1},
			wantShowdownHands: 2,
			resolveShowdown:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			match := mustHeadsUpMatch(t, 100, 1, 2)
			for seat, stack := range tc.preHandStacks {
				match.Players[seat].Stack = stack
			}
			var hand *HandState
			if tc.deal != nil {
				var err error
				hand, err = match.StartHand(1, *tc.deal)
				if err != nil {
					t.Fatalf("StartHand() error = %v", err)
				}
			} else {
				hand = mustStartHand(t, match, 1)
			}
			for _, action := range tc.actions {
				mustApplyAction(t, hand, action)
			}

			if hand.Street != tc.wantStreet {
				t.Fatalf("street = %v, want %v", hand.Street, tc.wantStreet)
			}
			if len(hand.Board) != tc.wantBoardCount {
				t.Fatalf("board count = %d, want %d", len(hand.Board), tc.wantBoardCount)
			}
			if hand.Complete != tc.wantComplete {
				t.Fatalf("complete = %v, want %v", hand.Complete, tc.wantComplete)
			}
			if hand.ShowdownPending != tc.wantShowdown {
				t.Fatalf("showdown pending = %v, want %v", hand.ShowdownPending, tc.wantShowdown)
			}

			if tc.resolveShowdown {
				if err := hand.ResolveShowdown(); err != nil {
					t.Fatalf("ResolveShowdown() error = %v", err)
				}
			}

			if hand.Result == nil {
				t.Fatalf("hand result should be populated")
			}
			if hand.Result.Pot != tc.wantPot {
				t.Fatalf("pot = %d, want %d", hand.Result.Pot, tc.wantPot)
			}
			gotStacks := [2]int{hand.Players[0].Stack, hand.Players[1].Stack}
			if gotStacks != tc.wantStacks {
				t.Fatalf("stacks = %v, want %v", gotStacks, tc.wantStacks)
			}
			gotDeltas := [2]int{hand.Result.Deltas[0].Delta, hand.Result.Deltas[1].Delta}
			if gotDeltas != tc.wantDeltas {
				t.Fatalf("deltas = %v, want %v", gotDeltas, tc.wantDeltas)
			}
			if !slices.Equal(hand.Result.WinningSeats, tc.wantWinningSeats) {
				t.Fatalf("winning seats = %v, want %v", hand.Result.WinningSeats, tc.wantWinningSeats)
			}
			if len(hand.Result.ShowdownHands) != tc.wantShowdownHands {
				t.Fatalf("showdown hands = %d, want %d", len(hand.Result.ShowdownHands), tc.wantShowdownHands)
			}

			if err := match.FinalizeHand(hand); err != nil {
				t.Fatalf("FinalizeHand() error = %v", err)
			}
			gotMatchStacks := [2]int{match.Players[0].Stack, match.Players[1].Stack}
			if gotMatchStacks != tc.wantMatchStacks {
				t.Fatalf("match stacks = %v, want %v", gotMatchStacks, tc.wantMatchStacks)
			}
		})
	}
}

func TestEvaluateBestHand(t *testing.T) {
	testCases := []struct {
		name         string
		holeCards    [2]string
		board        []string
		wantCategory int
		wantRanks    []int
	}{
		{
			name:         "ace low straight",
			holeCards:    [2]string{"Ac", "2d"},
			board:        []string{"3h", "4s", "5c", "Kd", "Qh"},
			wantCategory: handCategoryStraight,
			wantRanks:    []int{5},
		},
		{
			name:         "flush chooses top five suited cards",
			holeCards:    [2]string{"Ah", "2h"},
			board:        []string{"Kh", "Th", "7h", "3h", "Qc"},
			wantCategory: handCategoryFlush,
			wantRanks:    []int{14, 13, 10, 7, 3},
		},
		{
			name:         "full house prefers best trips then pair",
			holeCards:    [2]string{"Ac", "Ad"},
			board:        []string{"Ah", "Kc", "Kd", "2s", "3h"},
			wantCategory: handCategoryFullHouse,
			wantRanks:    []int{14, 13},
		},
		{
			name:         "two pair keeps correct kicker",
			holeCards:    [2]string{"Ac", "Qd"},
			board:        []string{"Ah", "Qs", "9c", "4d", "2h"},
			wantCategory: handCategoryTwoPair,
			wantRanks:    []int{14, 12, 9},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hand := evaluateBestHand(mustCardPair(t, tc.holeCards), mustCards(t, tc.board))
			if hand.category != tc.wantCategory {
				t.Fatalf("category = %d, want %d", hand.category, tc.wantCategory)
			}
			if !slices.Equal(hand.ranks, tc.wantRanks) {
				t.Fatalf("ranks = %v, want %v", hand.ranks, tc.wantRanks)
			}
		})
	}
}

func TestResolveShowdown(t *testing.T) {
	testCases := []struct {
		name              string
		preHandStacks     map[int]int
		deal              deck.HoldemDeal
		actions           []Action
		wantWinningSeats  []int
		wantOddChipSeat   int
		wantStacks        [2]int
		wantDeltas        [2]int
		wantShowdownLabel []string
	}{
		{
			name: "higher made hand wins entire pot",
			deal: mustDeal(t,
				[][]string{{"As", "Kd"}, {"9s", "9d"}},
				[]string{"Td", "9h", "2c", "5s", "Kc"},
			),
			actions: []Action{
				{Seat: 0, Type: ActionCall, Amount: 1},
				{Seat: 1, Type: ActionCheck},
				{Seat: 1, Type: ActionCheck},
				{Seat: 0, Type: ActionCheck},
				{Seat: 1, Type: ActionCheck},
				{Seat: 0, Type: ActionCheck},
				{Seat: 1, Type: ActionCheck},
				{Seat: 0, Type: ActionCheck},
			},
			wantWinningSeats:  []int{1},
			wantOddChipSeat:   -1,
			wantStacks:        [2]int{98, 102},
			wantDeltas:        [2]int{-2, 2},
			wantShowdownLabel: []string{"one pair", "three of a kind"},
		},
		{
			name: "tied board splits even contested pot",
			deal: mustDeal(t,
				[][]string{{"Ac", "Kd"}, {"Qh", "Js"}},
				[]string{"As", "Ks", "Qs", "Js", "Ts"},
			),
			actions: []Action{
				{Seat: 0, Type: ActionCall, Amount: 1},
				{Seat: 1, Type: ActionCheck},
				{Seat: 1, Type: ActionCheck},
				{Seat: 0, Type: ActionCheck},
				{Seat: 1, Type: ActionCheck},
				{Seat: 0, Type: ActionCheck},
				{Seat: 1, Type: ActionCheck},
				{Seat: 0, Type: ActionCheck},
			},
			wantWinningSeats:  []int{0, 1},
			wantOddChipSeat:   -1,
			wantStacks:        [2]int{100, 100},
			wantDeltas:        [2]int{0, 0},
			wantShowdownLabel: []string{"straight flush", "straight flush"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			match := mustHeadsUpMatch(t, 100, 1, 2)
			for seat, stack := range tc.preHandStacks {
				match.Players[seat].Stack = stack
			}
			hand, err := match.StartHand(1, tc.deal)
			if err != nil {
				t.Fatalf("StartHand() error = %v", err)
			}
			for _, action := range tc.actions {
				mustApplyAction(t, hand, action)
			}
			if !hand.ShowdownPending {
				t.Fatalf("hand should be awaiting showdown")
			}
			if err := hand.ResolveShowdown(); err != nil {
				t.Fatalf("ResolveShowdown() error = %v", err)
			}

			if !slices.Equal(hand.Result.WinningSeats, tc.wantWinningSeats) {
				t.Fatalf("winning seats = %v, want %v", hand.Result.WinningSeats, tc.wantWinningSeats)
			}
			if hand.Result.OddChipSeat != tc.wantOddChipSeat {
				t.Fatalf("odd chip seat = %d, want %d", hand.Result.OddChipSeat, tc.wantOddChipSeat)
			}
			gotStacks := [2]int{hand.Players[0].Stack, hand.Players[1].Stack}
			if gotStacks != tc.wantStacks {
				t.Fatalf("stacks = %v, want %v", gotStacks, tc.wantStacks)
			}
			gotDeltas := [2]int{hand.Result.Deltas[0].Delta, hand.Result.Deltas[1].Delta}
			if gotDeltas != tc.wantDeltas {
				t.Fatalf("deltas = %v, want %v", gotDeltas, tc.wantDeltas)
			}
			labels := []string{hand.Result.ShowdownHands[0].Label, hand.Result.ShowdownHands[1].Label}
			if !slices.Equal(labels, tc.wantShowdownLabel) {
				t.Fatalf("showdown labels = %v, want %v", labels, tc.wantShowdownLabel)
			}
		})
	}
}

func TestFirstWinnerClockwiseFromButton(t *testing.T) {
	if got := firstWinnerClockwiseFromButton(0, []int{0, 1}, 2); got != 1 {
		t.Fatalf("firstWinnerClockwiseFromButton() = %d, want 1", got)
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

func mustApplyAction(t *testing.T, hand *HandState, action Action) {
	t.Helper()
	if err := hand.ApplyAction(action); err != nil {
		t.Fatalf("ApplyAction(%+v) error = %v", action, err)
	}
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

func mustDeal(t *testing.T, playerCards [][]string, board []string) deck.HoldemDeal {
	t.Helper()
	deal := deck.HoldemDeal{PlayerCards: make([][]deck.Card, len(playerCards)), Board: mustCards(t, board)}
	for seat := range playerCards {
		deal.PlayerCards[seat] = mustCards(t, playerCards[seat])
	}
	return deal
}

func ptrDeal(deal deck.HoldemDeal) *deck.HoldemDeal {
	return &deal
}

func mustCardPair(t *testing.T, raw [2]string) [2]deck.Card {
	t.Helper()
	cards := mustCards(t, []string{raw[0], raw[1]})
	return [2]deck.Card{cards[0], cards[1]}
}

func mustCards(t *testing.T, raw []string) []deck.Card {
	t.Helper()
	cards := make([]deck.Card, len(raw))
	for i, s := range raw {
		card, err := deck.ParseCard(s)
		if err != nil {
			t.Fatalf("ParseCard(%q) error = %v", s, err)
		}
		cards[i] = card
	}
	return cards
}
