package deck

import "fmt"

type Deck struct {
	cards []Card
	next  int
}

func NewShuffledDeck(seed uint64) Deck {
	cards := Full()
	rng := splitMix64{state: seed}
	for i := len(cards) - 1; i > 0; i-- {
		j := int(rng.Next() % uint64(i+1))
		cards[i], cards[j] = cards[j], cards[i]
	}
	return Deck{cards: cards}
}

func (d *Deck) Draw() (Card, error) {
	if d.next >= len(d.cards) {
		return Card{}, fmt.Errorf("draw card: deck exhausted")
	}
	card := d.cards[d.next]
	d.next++
	return card, nil
}

func (d Deck) Remaining() int {
	return len(d.cards) - d.next
}

func (d Deck) Cards() []Card {
	return append([]Card(nil), d.cards...)
}
