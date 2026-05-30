package deck

import (
	"fmt"
)

type Suit uint8

const (
	Clubs Suit = iota
	Diamonds
	Hearts
	Spades
)

var allSuits = [...]Suit{Clubs, Diamonds, Hearts, Spades}

func (s Suit) String() string {
	switch s {
	case Clubs:
		return "c"
	case Diamonds:
		return "d"
	case Hearts:
		return "h"
	case Spades:
		return "s"
	default:
		return "?"
	}
}

type Rank uint8

const (
	Two   Rank = 2
	Three Rank = 3
	Four  Rank = 4
	Five  Rank = 5
	Six   Rank = 6
	Seven Rank = 7
	Eight Rank = 8
	Nine  Rank = 9
	Ten   Rank = 10
	Jack  Rank = 11
	Queen Rank = 12
	King  Rank = 13
	Ace   Rank = 14
)

var allRanks = [...]Rank{Two, Three, Four, Five, Six, Seven, Eight, Nine, Ten, Jack, Queen, King, Ace}

func (r Rank) String() string {
	switch r {
	case Two:
		return "2"
	case Three:
		return "3"
	case Four:
		return "4"
	case Five:
		return "5"
	case Six:
		return "6"
	case Seven:
		return "7"
	case Eight:
		return "8"
	case Nine:
		return "9"
	case Ten:
		return "T"
	case Jack:
		return "J"
	case Queen:
		return "Q"
	case King:
		return "K"
	case Ace:
		return "A"
	default:
		return "?"
	}
}

type Card struct {
	Rank Rank
	Suit Suit
}

func (c Card) String() string {
	return c.Rank.String() + c.Suit.String()
}

func ParseCard(raw string) (Card, error) {
	if len(raw) != 2 {
		return Card{}, fmt.Errorf("parse card %q: expected 2 characters", raw)
	}

	rank, err := parseRank(raw[0])
	if err != nil {
		return Card{}, fmt.Errorf("parse card %q: %w", raw, err)
	}

	suit, err := parseSuit(raw[1])
	if err != nil {
		return Card{}, fmt.Errorf("parse card %q: %w", raw, err)
	}

	return Card{Rank: rank, Suit: suit}, nil
}

func parseRank(raw byte) (Rank, error) {
	switch raw {
	case '2':
		return Two, nil
	case '3':
		return Three, nil
	case '4':
		return Four, nil
	case '5':
		return Five, nil
	case '6':
		return Six, nil
	case '7':
		return Seven, nil
	case '8':
		return Eight, nil
	case '9':
		return Nine, nil
	case 'T':
		return Ten, nil
	case 'J':
		return Jack, nil
	case 'Q':
		return Queen, nil
	case 'K':
		return King, nil
	case 'A':
		return Ace, nil
	default:
		return 0, fmt.Errorf("unknown rank %q", raw)
	}
}

func parseSuit(raw byte) (Suit, error) {
	switch raw {
	case 'c':
		return Clubs, nil
	case 'd':
		return Diamonds, nil
	case 'h':
		return Hearts, nil
	case 's':
		return Spades, nil
	default:
		return 0, fmt.Errorf("unknown suit %q", raw)
	}
}

func Full() []Card {
	cards := make([]Card, 0, len(allSuits)*len(allRanks))
	for _, suit := range allSuits {
		for _, rank := range allRanks {
			cards = append(cards, Card{Rank: rank, Suit: suit})
		}
	}
	return cards
}
