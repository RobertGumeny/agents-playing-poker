package heuristicagent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"github.com/RobertGumeny/agent-poker/internal/deck"
	"github.com/RobertGumeny/agent-poker/internal/wire"
)

const Version = "heuristic/0.1.0"

type Agent struct {
	messageSeq int
	holeCards  [2]deck.Card
	haveHole   bool
}

func New() *Agent {
	return &Agent{}
}

func Run(stdin io.Reader, stdout io.Writer) error {
	return New().Run(stdin, stdout)
}

func (a *Agent) Run(stdin io.Reader, stdout io.Writer) error {
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(stdout)

	for scanner.Scan() {
		envelope, err := wire.DecodeEnvelope(scanner.Bytes())
		if err != nil {
			return err
		}

		switch envelope.Type {
		case wire.MessageTypeSessionInit:
			if err := encoder.Encode(wire.NewMessage(wire.MessageTypeSessionReady, a.nextMessageID(), envelope.ID, wire.SessionReadyPayload{Version: Version})); err != nil {
				return fmt.Errorf("encode session_ready: %w", err)
			}
		case wire.MessageTypeHandStart:
			var payload wire.HandStartPayload
			if err := envelope.DecodePayload(&payload); err != nil {
				return err
			}
			if err := a.setHoleCards(payload.YourHoleCards); err != nil {
				return err
			}
		case wire.MessageTypeYourTurn:
			var payload wire.YourTurnPayload
			if err := envelope.DecodePayload(&payload); err != nil {
				return err
			}
			action, err := a.chooseAction(payload)
			if err != nil {
				return err
			}
			if err := encoder.Encode(wire.NewMessage(wire.MessageTypeAction, a.nextMessageID(), envelope.ID, action)); err != nil {
				return fmt.Errorf("encode action: %w", err)
			}
		case wire.MessageTypeHandEnd:
			continue
		case wire.MessageTypeSessionEnd:
			return nil
		default:
			return fmt.Errorf("unsupported server message type %q", envelope.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan stdin: %w", err)
	}
	return nil
}

func (a *Agent) setHoleCards(raw []string) error {
	if len(raw) != 2 {
		return fmt.Errorf("set hole cards: got %d cards, want 2", len(raw))
	}
	for i, cardRaw := range raw {
		card, err := deck.ParseCard(cardRaw)
		if err != nil {
			return fmt.Errorf("set hole cards: %w", err)
		}
		a.holeCards[i] = card
	}
	a.haveHole = true
	return nil
}

func (a *Agent) chooseAction(payload wire.YourTurnPayload) (wire.ActionPayload, error) {
	if !a.haveHole {
		return wire.ActionPayload{}, fmt.Errorf("choose action: hole cards not set")
	}
	profile, err := buildDecisionProfile(a.holeCards, payload.Board)
	if err != nil {
		return wire.ActionPayload{}, err
	}

	if payload.ToCall == 0 {
		if profile.aggression >= 0.62 {
			if action, ok := aggressiveAction(payload.LegalActions); ok {
				return action, nil
			}
		}
		if action, ok := checkAction(payload.LegalActions); ok {
			return action, nil
		}
		return wire.ActionPayload{}, fmt.Errorf("choose action: no zero-cost action available")
	}

	potOdds := float64(payload.ToCall) / float64(payload.Pot+payload.ToCall)
	if profile.equity >= potOdds+0.25 {
		if action, ok := aggressiveAction(payload.LegalActions); ok {
			return action, nil
		}
	}
	if profile.equity+0.05 < potOdds {
		if action, ok := foldAction(payload.LegalActions); ok {
			return action, nil
		}
	}
	if action, ok := callAction(payload.LegalActions); ok {
		return action, nil
	}
	if action, ok := checkAction(payload.LegalActions); ok {
		return action, nil
	}
	return wire.ActionPayload{}, fmt.Errorf("choose action: no supported legal action")
}

func aggressiveAction(actions []wire.LegalActionOption) (wire.ActionPayload, bool) {
	for _, action := range actions {
		switch action.Action {
		case "raise", "bet":
			if action.Min == nil {
				return wire.ActionPayload{}, false
			}
			amount := *action.Min
			return wire.ActionPayload{Action: action.Action, Amount: &amount}, true
		}
	}
	return wire.ActionPayload{}, false
}

func checkAction(actions []wire.LegalActionOption) (wire.ActionPayload, bool) {
	for _, action := range actions {
		if action.Action == "check" {
			return wire.ActionPayload{Action: "check"}, true
		}
	}
	return wire.ActionPayload{}, false
}

func callAction(actions []wire.LegalActionOption) (wire.ActionPayload, bool) {
	for _, action := range actions {
		if action.Action == "call" && action.Amount != nil {
			amount := *action.Amount
			return wire.ActionPayload{Action: "call", Amount: &amount}, true
		}
	}
	return wire.ActionPayload{}, false
}

func foldAction(actions []wire.LegalActionOption) (wire.ActionPayload, bool) {
	for _, action := range actions {
		if action.Action == "fold" {
			return wire.ActionPayload{Action: "fold"}, true
		}
	}
	return wire.ActionPayload{}, false
}

func (a *Agent) nextMessageID() string {
	a.messageSeq++
	return fmt.Sprintf("heuristic-%d", a.messageSeq)
}

type decisionProfile struct {
	equity     float64
	aggression float64
}

func buildDecisionProfile(hole [2]deck.Card, boardRaw []string) (decisionProfile, error) {
	board := make([]deck.Card, 0, len(boardRaw))
	for _, raw := range boardRaw {
		card, err := deck.ParseCard(raw)
		if err != nil {
			return decisionProfile{}, fmt.Errorf("build decision profile: %w", err)
		}
		board = append(board, card)
	}
	if len(board) == 0 {
		equity := preflopStrength(hole)
		return decisionProfile{equity: equity, aggression: equity}, nil
	}

	best := evaluateBestHand(hole, board)
	equity := categoryEquity(best.category)
	if draw := flushDrawBonus(hole, board); draw > 0 {
		equity += draw
	}
	if draw := straightDrawBonus(hole, board); draw > 0 {
		equity += draw
	}
	if overcards := overcardBonus(hole, board, best.category); overcards > 0 {
		equity += overcards
	}
	if equity > 0.98 {
		equity = 0.98
	}
	aggression := equity
	if best.category >= handCategoryTwoPair {
		aggression += 0.08
	}
	if aggression > 0.98 {
		aggression = 0.98
	}
	return decisionProfile{equity: equity, aggression: aggression}, nil
}

func preflopStrength(hole [2]deck.Card) float64 {
	ranks := []int{int(hole[0].Rank), int(hole[1].Rank)}
	slices.Sort(ranks)
	low, high := ranks[0], ranks[1]
	score := high*4 + low*2
	if high == low {
		score += 30
	}
	if hole[0].Suit == hole[1].Suit {
		score += 8
	}
	gap := high - low
	switch gap {
	case 0, 1:
		score += 8
	case 2:
		score += 4
	}
	if high == int(deck.Ace) && low >= int(deck.Ten) {
		score += 10
	}
	if high >= int(deck.Ten) && low >= int(deck.Ten) {
		score += 10
	}
	if low < int(deck.Five) && gap > 4 {
		score -= 8
	}
	if score < 15 {
		score = 15
	}
	if score > 95 {
		score = 95
	}
	return float64(score) / 100
}

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
}

func evaluateBestHand(hole [2]deck.Card, board []deck.Card) evaluatedHand {
	cards := make([]deck.Card, 0, 2+len(board))
	cards = append(cards, hole[0], hole[1])
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

	for _, ranks := range suitCards {
		slices.Sort(ranks)
		slices.Reverse(ranks)
		if high, ok := highestStraight(ranks); ok {
			return evaluatedHand{category: handCategoryStraightFlush, ranks: []int{high}}
		}
	}
	if quads := ranksWithCount(rankCounts, 4); len(quads) > 0 {
		return evaluatedHand{category: handCategoryFourOfAKind, ranks: quads}
	}
	trips := ranksWithCountAtLeast(rankCounts, 3)
	pairs := ranksWithCountAtLeast(rankCounts, 2)
	if len(trips) > 0 {
		for _, pair := range pairs {
			if pair != trips[0] {
				return evaluatedHand{category: handCategoryFullHouse, ranks: []int{trips[0], pair}}
			}
		}
	}
	for _, ranks := range suitCards {
		if len(ranks) >= 5 {
			return evaluatedHand{category: handCategoryFlush, ranks: append([]int(nil), ranks[:5]...)}
		}
	}
	if high, ok := highestStraight(allRanks); ok {
		return evaluatedHand{category: handCategoryStraight, ranks: []int{high}}
	}
	if len(trips) > 0 {
		return evaluatedHand{category: handCategoryThreeOfAKind, ranks: trips}
	}
	if len(pairs) >= 2 {
		return evaluatedHand{category: handCategoryTwoPair, ranks: pairs[:2]}
	}
	if len(pairs) == 1 {
		return evaluatedHand{category: handCategoryOnePair, ranks: pairs[:1]}
	}
	return evaluatedHand{category: handCategoryHighCard, ranks: topRanks(rankCounts, 5)}
}

func categoryEquity(category int) float64 {
	switch category {
	case handCategoryStraightFlush, handCategoryFourOfAKind:
		return 0.97
	case handCategoryFullHouse:
		return 0.95
	case handCategoryFlush:
		return 0.85
	case handCategoryStraight:
		return 0.8
	case handCategoryThreeOfAKind:
		return 0.72
	case handCategoryTwoPair:
		return 0.65
	case handCategoryOnePair:
		return 0.52
	default:
		return 0.28
	}
}

func flushDrawBonus(hole [2]deck.Card, board []deck.Card) float64 {
	cards := append([]deck.Card{hole[0], hole[1]}, board...)
	counts := map[deck.Suit]int{}
	for _, card := range cards {
		counts[card.Suit]++
	}
	for _, count := range counts {
		if count == 4 {
			return 0.18
		}
	}
	return 0
}

func straightDrawBonus(hole [2]deck.Card, board []deck.Card) float64 {
	cards := append([]deck.Card{hole[0], hole[1]}, board...)
	seen := make(map[int]bool, len(cards)+1)
	for _, card := range cards {
		rank := int(card.Rank)
		seen[rank] = true
		if rank == int(deck.Ace) {
			seen[1] = true
		}
	}
	best := 0.0
	for high := int(deck.Ace); high >= 5; high-- {
		count := 0
		missing := -1
		for rank := high; rank > high-5; rank-- {
			if seen[rank] {
				count++
			} else {
				missing = rank
			}
		}
		if count != 4 {
			continue
		}
		bonus := 0.08
		if missing == high || missing == high-4 {
			bonus = 0.16
		}
		if bonus > best {
			best = bonus
		}
	}
	return best
}

func overcardBonus(hole [2]deck.Card, board []deck.Card, category int) float64 {
	if category != handCategoryHighCard || len(board) == 0 {
		return 0
	}
	highBoard := int(board[0].Rank)
	for _, card := range board[1:] {
		if int(card.Rank) > highBoard {
			highBoard = int(card.Rank)
		}
	}
	if int(hole[0].Rank) > highBoard && int(hole[1].Rank) > highBoard {
		return 0.06
	}
	return 0
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

func topRanks(rankCounts map[int]int, n int) []int {
	ranks := make([]int, 0, n)
	for rank := int(deck.Ace); rank >= int(deck.Two); rank-- {
		if rankCounts[rank] == 0 {
			continue
		}
		ranks = append(ranks, rank)
		if len(ranks) == n {
			return ranks
		}
	}
	return ranks
}
