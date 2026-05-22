package deck

import (
	"reflect"
	"testing"
)

func TestParseCardRoundTrip(t *testing.T) {
	cases := []string{"As", "Td", "2c", "9h"}
	for _, raw := range cases {
		card, err := ParseCard(raw)
		if err != nil {
			t.Fatalf("ParseCard(%q) error = %v", raw, err)
		}
		if got := card.String(); got != raw {
			t.Fatalf("ParseCard(%q).String() = %q, want %q", raw, got, raw)
		}
	}
}

func TestNewShuffledDeckDeterministic(t *testing.T) {
	deckA := NewShuffledDeck(11)
	deckB := NewShuffledDeck(11)

	if !reflect.DeepEqual(deckA.Cards(), deckB.Cards()) {
		t.Fatalf("same deck seed should produce identical deck order")
	}
}

func TestDeckDraw(t *testing.T) {
	deck := NewShuffledDeck(21)
	seen := map[string]bool{}
	for i := 0; i < 52; i++ {
		card, err := deck.Draw()
		if err != nil {
			t.Fatalf("Draw() error at %d = %v", i, err)
		}
		if seen[card.String()] {
			t.Fatalf("Draw() returned duplicate card %s", card)
		}
		seen[card.String()] = true
	}
	if deck.Remaining() != 0 {
		t.Fatalf("Remaining() = %d, want 0", deck.Remaining())
	}
	if _, err := deck.Draw(); err == nil {
		t.Fatalf("Draw() on exhausted deck should fail")
	}
}

func TestDealHoldemHandDeterministic(t *testing.T) {
	dealerA := NewDealer(42)
	dealerB := NewDealer(42)

	dealA, err := dealerA.DealHoldemHand(7, 2)
	if err != nil {
		t.Fatalf("DealHoldemHand() error = %v", err)
	}
	dealB, err := dealerB.DealHoldemHand(7, 2)
	if err != nil {
		t.Fatalf("DealHoldemHand() error = %v", err)
	}

	if !reflect.DeepEqual(dealA, dealB) {
		t.Fatalf("same seed and hand number should produce identical deals")
	}
}

func TestDealHoldemHandChangesByHandNumber(t *testing.T) {
	dealer := NewDealer(42)

	dealA, err := dealer.DealHoldemHand(1, 2)
	if err != nil {
		t.Fatalf("DealHoldemHand() error = %v", err)
	}
	dealB, err := dealer.DealHoldemHand(2, 2)
	if err != nil {
		t.Fatalf("DealHoldemHand() error = %v", err)
	}

	if reflect.DeepEqual(dealA, dealB) {
		t.Fatalf("different hand numbers should produce different deals")
	}
}

func TestDealHoldemHandStructure(t *testing.T) {
	dealer := NewDealer(99)
	deal, err := dealer.DealHoldemHand(1, 2)
	if err != nil {
		t.Fatalf("DealHoldemHand() error = %v", err)
	}

	if len(deal.PlayerCards) != 2 {
		t.Fatalf("len(PlayerCards) = %d, want 2", len(deal.PlayerCards))
	}
	for seat, cards := range deal.PlayerCards {
		if len(cards) != 2 {
			t.Fatalf("seat %d hole card count = %d, want 2", seat, len(cards))
		}
	}
	if len(deal.BurnCards) != 3 {
		t.Fatalf("len(BurnCards) = %d, want 3", len(deal.BurnCards))
	}
	if len(deal.Board) != 5 {
		t.Fatalf("len(Board) = %d, want 5", len(deal.Board))
	}
	if len(deal.Stub) != 40 {
		t.Fatalf("len(Stub) = %d, want 40", len(deal.Stub))
	}
	if len(deal.Order) != 52 {
		t.Fatalf("len(Order) = %d, want 52", len(deal.Order))
	}

	seen := map[string]bool{}
	for _, card := range deal.Order {
		key := card.String()
		if seen[key] {
			t.Fatalf("duplicate card in order: %s", key)
		}
		seen[key] = true
	}
}

func TestDealHoldemHandDealsRoundRobinWithBurns(t *testing.T) {
	dealer := NewDealer(123)
	deal, err := dealer.DealHoldemHand(3, 2)
	if err != nil {
		t.Fatalf("DealHoldemHand() error = %v", err)
	}

	wantSeat0 := []Card{deal.Order[0], deal.Order[2]}
	wantSeat1 := []Card{deal.Order[1], deal.Order[3]}
	wantBurns := []Card{deal.Order[4], deal.Order[8], deal.Order[10]}
	wantBoard := []Card{deal.Order[5], deal.Order[6], deal.Order[7], deal.Order[9], deal.Order[11]}

	if !reflect.DeepEqual(deal.PlayerCards[0], wantSeat0) {
		t.Fatalf("seat 0 cards = %v, want %v", deal.PlayerCards[0], wantSeat0)
	}
	if !reflect.DeepEqual(deal.PlayerCards[1], wantSeat1) {
		t.Fatalf("seat 1 cards = %v, want %v", deal.PlayerCards[1], wantSeat1)
	}
	if !reflect.DeepEqual(deal.BurnCards, wantBurns) {
		t.Fatalf("burn cards = %v, want %v", deal.BurnCards, wantBurns)
	}
	if !reflect.DeepEqual(deal.Board, wantBoard) {
		t.Fatalf("board = %v, want %v", deal.Board, wantBoard)
	}
}
