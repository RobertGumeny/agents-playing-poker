package deck

import "fmt"

type Dealer struct {
	matchSeed uint64
}

func NewDealer(matchSeed uint64) Dealer {
	return Dealer{matchSeed: matchSeed}
}

type HoldemDeal struct {
	HandNumber  int
	PlayerCards [][]Card
	BurnCards   []Card
	Board       []Card
	Stub        []Card
	Order       []Card
}

func (d Dealer) DealHoldemHand(handNumber int, playerCount int) (HoldemDeal, error) {
	if handNumber < 1 {
		return HoldemDeal{}, fmt.Errorf("deal hand: hand number must be >= 1")
	}
	if playerCount < 2 {
		return HoldemDeal{}, fmt.Errorf("deal hand: player count must be >= 2")
	}
	if playerCount*2+5+3 > 52 {
		return HoldemDeal{}, fmt.Errorf("deal hand: player count %d exceeds hold'em deck capacity", playerCount)
	}

	shoe := NewShuffledDeck(deriveHandSeed(d.matchSeed, handNumber))
	order := shoe.Cards()
	cursor := 0

	playerCards := make([][]Card, playerCount)
	for i := range playerCards {
		playerCards[i] = make([]Card, 0, 2)
	}

	for round := 0; round < 2; round++ {
		for seat := 0; seat < playerCount; seat++ {
			playerCards[seat] = append(playerCards[seat], order[cursor])
			cursor++
		}
	}

	burnCards := make([]Card, 0, 3)
	board := make([]Card, 0, 5)

	burnCards = append(burnCards, order[cursor])
	cursor++
	board = append(board, order[cursor:cursor+3]...)
	cursor += 3

	burnCards = append(burnCards, order[cursor])
	cursor++
	board = append(board, order[cursor])
	cursor++

	burnCards = append(burnCards, order[cursor])
	cursor++
	board = append(board, order[cursor])
	cursor++

	stub := append([]Card(nil), order[cursor:]...)

	return HoldemDeal{
		HandNumber:  handNumber,
		PlayerCards: playerCards,
		BurnCards:   burnCards,
		Board:       board,
		Stub:        stub,
		Order:       append([]Card(nil), order...),
	}, nil
}

func deriveHandSeed(matchSeed uint64, handNumber int) uint64 {
	seed := matchSeed ^ (uint64(handNumber) + 0x9e3779b97f4a7c15)
	mix := splitMix64{state: seed}
	return mix.Next()
}

type splitMix64 struct {
	state uint64
}

func (s *splitMix64) Next() uint64 {
	s.state += 0x9e3779b97f4a7c15
	z := s.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
