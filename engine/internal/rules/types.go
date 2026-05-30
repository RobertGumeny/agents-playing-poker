package rules

import "github.com/RobertGumeny/agent-poker/internal/deck"

type Street uint8

const (
	StreetPreflop Street = iota
	StreetFlop
	StreetTurn
	StreetRiver
	StreetShowdown
)

func (s Street) String() string {
	switch s {
	case StreetPreflop:
		return "preflop"
	case StreetFlop:
		return "flop"
	case StreetTurn:
		return "turn"
	case StreetRiver:
		return "river"
	case StreetShowdown:
		return "showdown"
	default:
		return "unknown"
	}
}

type ActionType string

const (
	ActionFold      ActionType = "fold"
	ActionCheck     ActionType = "check"
	ActionCall      ActionType = "call"
	ActionBet       ActionType = "bet"
	ActionRaise     ActionType = "raise"
	ActionPostBlind ActionType = "post_blind"
)

type Action struct {
	Seat   int
	Type   ActionType
	Street Street
	Amount int
	AllIn  bool
}

type LegalAction struct {
	Type      ActionType
	Amount    int
	MinAmount int
	MaxAmount int
}

type PlayerState struct {
	Seat            int
	StartingStack   int
	Stack           int
	Committed       int
	StreetCommitted int
	InHand          bool
	AllIn           bool
	HoleCards       [2]deck.Card
	CardsDealt      bool
}

type BlindPosting struct {
	Seat   int
	Amount int
}

type ShowdownHand struct {
	Seat      int
	HoleCards [2]deck.Card
	Label     string
}

type ChipDelta struct {
	Seat  int
	Delta int
}

type HandResult struct {
	Pot           int
	WinningSeats  []int
	Deltas        []ChipDelta
	ShowdownHands []ShowdownHand
	OddChipSeat   int
	Showdown      bool
}
