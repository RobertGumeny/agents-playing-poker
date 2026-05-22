package rules

import (
	"fmt"
	"slices"

	"github.com/RobertGumeny/agent-poker/internal/deck"
)

const (
	handCategoryHighCard = iota
	handCategoryOnePair
	handCategoryTwoPair
	handCategoryThreeOfAKind
	handCategoryStraight
	handCategoryFlush
	handCategoryFullHouse
	handCategoryFourOfAKind
	handCategoryStraightFlush
)

type evaluatedHand struct {
	category int
	ranks    []int
	label    string
}

func (h evaluatedHand) compare(other evaluatedHand) int {
	if h.category != other.category {
		if h.category > other.category {
			return 1
		}
		return -1
	}
	return slices.Compare(h.ranks, other.ranks)
}

func (h *HandState) ResolveShowdown() error {
	if h == nil {
		return fmt.Errorf("resolve showdown: hand is nil")
	}
	if !h.Complete || !h.ShowdownPending || h.Street != StreetShowdown {
		return fmt.Errorf("resolve showdown: hand is not awaiting showdown")
	}

	contenders := make([]int, 0, len(h.Players))
	showdownHands := make([]ShowdownHand, 0, len(h.Players))
	bestBySeat := make(map[int]evaluatedHand, len(h.Players))
	var best evaluatedHand
	bestSet := false

	for seat, player := range h.Players {
		if !player.InHand {
			continue
		}
		bestHand := evaluateBestHand(player.HoleCards, h.FullBoard)
		bestBySeat[seat] = bestHand
		contenders = append(contenders, seat)
		showdownHands = append(showdownHands, ShowdownHand{
			Seat:      seat,
			HoleCards: player.HoleCards,
			Label:     bestHand.label,
		})
		if !bestSet || bestHand.compare(best) > 0 {
			best = bestHand
			bestSet = true
		}
	}
	if len(contenders) == 0 {
		return fmt.Errorf("resolve showdown: no players remain in hand")
	}

	winners := make([]int, 0, len(contenders))
	for _, seat := range contenders {
		if bestBySeat[seat].compare(best) == 0 {
			winners = append(winners, seat)
		}
	}

	refundUnmatchedCommitment(h, contenders)
	pot := h.Pot()
	share := pot / len(winners)
	oddChipSeat := -1
	for _, seat := range winners {
		h.Players[seat].Stack += share
	}
	if pot%len(winners) != 0 {
		oddChipSeat = firstWinnerClockwiseFromButton(h.DealerSeat, winners, len(h.Players))
		h.Players[oddChipSeat].Stack++
	}

	for seat := range h.Players {
		h.Players[seat].Committed = 0
		h.Players[seat].StreetCommitted = 0
	}

	deltas := make([]ChipDelta, len(h.Players))
	for seat, player := range h.Players {
		deltas[seat] = ChipDelta{Seat: seat, Delta: player.Stack - player.StartingStack}
	}

	h.ShowdownPending = false
	h.Result = &HandResult{
		Pot:           pot,
		WinningSeats:  winners,
		Deltas:        deltas,
		ShowdownHands: showdownHands,
		OddChipSeat:   oddChipSeat,
		Showdown:      true,
	}

	return nil
}

func refundUnmatchedCommitment(h *HandState, contenders []int) {
	if len(contenders) < 2 {
		return
	}
	matched := h.Players[contenders[0]].Committed
	for _, seat := range contenders[1:] {
		if h.Players[seat].Committed < matched {
			matched = h.Players[seat].Committed
		}
	}
	for _, seat := range contenders {
		player := &h.Players[seat]
		if player.Committed <= matched {
			continue
		}
		refund := player.Committed - matched
		player.Committed -= refund
		player.Stack += refund
		if player.StreetCommitted >= refund {
			player.StreetCommitted -= refund
		}
	}
}

func firstWinnerClockwiseFromButton(buttonSeat int, winners []int, playerCount int) int {
	for offset := 1; offset <= playerCount; offset++ {
		seat := (buttonSeat + offset) % playerCount
		if slices.Contains(winners, seat) {
			return seat
		}
	}
	return winners[0]
}

func evaluateBestHand(holeCards [2]deck.Card, board []deck.Card) evaluatedHand {
	cards := make([]deck.Card, 0, 7)
	cards = append(cards, holeCards[0], holeCards[1])
	cards = append(cards, board...)

	rankCounts := make(map[int]int, 13)
	suitCards := make(map[deck.Suit][]int, 4)
	allRanks := make([]int, 0, len(cards))
	for _, card := range cards {
		rank := int(card.Rank)
		rankCounts[rank]++
		suitCards[card.Suit] = append(suitCards[card.Suit], rank)
		allRanks = append(allRanks, rank)
	}

	for suit, ranks := range suitCards {
		_ = suit
		slices.Sort(ranks)
		slices.Reverse(ranks)
		if high, ok := highestStraight(ranks); ok {
			return evaluatedHand{category: handCategoryStraightFlush, ranks: []int{high}, label: "straight flush"}
		}
	}

	quads := ranksWithCount(rankCounts, 4)
	if len(quads) > 0 {
		kickers := topRanksExcluding(rankCounts, map[int]bool{quads[0]: true}, 1)
		return evaluatedHand{category: handCategoryFourOfAKind, ranks: []int{quads[0], kickers[0]}, label: "four of a kind"}
	}

	trips := ranksWithCountAtLeast(rankCounts, 3)
	pairs := ranksWithCountAtLeast(rankCounts, 2)
	if len(trips) > 0 {
		pairRank := 0
		for _, rank := range pairs {
			if rank != trips[0] {
				pairRank = rank
				break
			}
		}
		if pairRank != 0 {
			return evaluatedHand{category: handCategoryFullHouse, ranks: []int{trips[0], pairRank}, label: "full house"}
		}
	}

	for _, ranks := range suitCards {
		if len(ranks) >= 5 {
			return evaluatedHand{category: handCategoryFlush, ranks: append([]int(nil), ranks[:5]...), label: "flush"}
		}
	}

	if high, ok := highestStraight(allRanks); ok {
		return evaluatedHand{category: handCategoryStraight, ranks: []int{high}, label: "straight"}
	}

	if len(trips) > 0 {
		kickers := topRanksExcluding(rankCounts, map[int]bool{trips[0]: true}, 2)
		return evaluatedHand{category: handCategoryThreeOfAKind, ranks: append([]int{trips[0]}, kickers...), label: "three of a kind"}
	}

	if len(pairs) >= 2 {
		kicker := topRanksExcluding(rankCounts, map[int]bool{pairs[0]: true, pairs[1]: true}, 1)
		return evaluatedHand{category: handCategoryTwoPair, ranks: []int{pairs[0], pairs[1], kicker[0]}, label: "two pair"}
	}

	if len(pairs) == 1 {
		kickers := topRanksExcluding(rankCounts, map[int]bool{pairs[0]: true}, 3)
		return evaluatedHand{category: handCategoryOnePair, ranks: append([]int{pairs[0]}, kickers...), label: "one pair"}
	}

	highCards := topRanksExcluding(rankCounts, nil, 5)
	return evaluatedHand{category: handCategoryHighCard, ranks: highCards, label: "high card"}
}

func highestStraight(ranks []int) (int, bool) {
	seen := make(map[int]bool, len(ranks)+1)
	for _, rank := range ranks {
		seen[rank] = true
		if rank == int(deck.Ace) {
			seen[1] = true
		}
	}
	for high := int(deck.Ace); high >= 5; high-- {
		ok := true
		for rank := high; rank > high-5; rank-- {
			if !seen[rank] {
				ok = false
				break
			}
		}
		if ok {
			return high, true
		}
	}
	return 0, false
}

func ranksWithCount(rankCounts map[int]int, want int) []int {
	var ranks []int
	for rank, count := range rankCounts {
		if count == want {
			ranks = append(ranks, rank)
		}
	}
	slices.Sort(ranks)
	slices.Reverse(ranks)
	return ranks
}

func ranksWithCountAtLeast(rankCounts map[int]int, want int) []int {
	var ranks []int
	for rank, count := range rankCounts {
		if count >= want {
			ranks = append(ranks, rank)
		}
	}
	slices.Sort(ranks)
	slices.Reverse(ranks)
	return ranks
}

func topRanksExcluding(rankCounts map[int]int, excluded map[int]bool, n int) []int {
	ranks := make([]int, 0, len(rankCounts))
	for rank := int(deck.Ace); rank >= int(deck.Two); rank-- {
		if rankCounts[rank] == 0 || (excluded != nil && excluded[rank]) {
			continue
		}
		ranks = append(ranks, rank)
		if len(ranks) == n {
			return ranks
		}
	}
	return ranks
}
