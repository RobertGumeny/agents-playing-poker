package rules

import (
	"fmt"
	"slices"

	"github.com/RobertGumeny/agent-poker/internal/deck"
)

type Config struct {
	PlayerCount   int
	StartingStack int
	SmallBlind    int
	BigBlind      int
}

type MatchState struct {
	Config  Config
	Players []PlayerState
}

type HandState struct {
	Config            Config
	HandNumber        int
	DealerSeat        int
	SmallBlindSeat    int
	BigBlindSeat      int
	Street            Street
	Players           []PlayerState
	Board             []deck.Card
	FullBoard         []deck.Card
	CurrentWager      int
	LastFullRaise     int
	ActionHistory     []Action
	ActingSeat        int
	Complete          bool
	ShowdownPending   bool
	Result            *HandResult
	actedThisStreet   []bool
	raiseOptionOpened []bool
}

func NewHeadsUpMatch(startingStack int, smallBlind int, bigBlind int) (*MatchState, error) {
	config := Config{
		PlayerCount:   2,
		StartingStack: startingStack,
		SmallBlind:    smallBlind,
		BigBlind:      bigBlind,
	}
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	players := make([]PlayerState, config.PlayerCount)
	for seat := range players {
		players[seat] = PlayerState{
			Seat:  seat,
			Stack: startingStack,
		}
	}

	return &MatchState{Config: config, Players: players}, nil
}

func (m *MatchState) StartHand(handNumber int, deal deck.HoldemDeal) (*HandState, error) {
	if err := validateConfig(m.Config); err != nil {
		return nil, err
	}
	if handNumber < 1 {
		return nil, fmt.Errorf("start hand: hand number must be >= 1")
	}
	if len(deal.PlayerCards) != m.Config.PlayerCount {
		return nil, fmt.Errorf("start hand: deal has %d players, want %d", len(deal.PlayerCards), m.Config.PlayerCount)
	}
	if len(deal.Board) != 5 {
		return nil, fmt.Errorf("start hand: deal board has %d cards, want 5", len(deal.Board))
	}

	dealerSeat := dealerSeatForHand(handNumber, m.Config.PlayerCount)
	smallBlindSeat := dealerSeat
	bigBlindSeat := nextSeat(smallBlindSeat, m.Config.PlayerCount)

	players := make([]PlayerState, len(m.Players))
	for seat, player := range m.Players {
		stack := player.Stack
		if stack <= 0 {
			stack = m.Config.StartingStack
		}
		players[seat] = PlayerState{
			Seat:          seat,
			StartingStack: stack,
			Stack:         stack,
			InHand:        true,
			HoleCards:     [2]deck.Card{deal.PlayerCards[seat][0], deal.PlayerCards[seat][1]},
			CardsDealt:    true,
		}
	}

	h := &HandState{
		Config:            m.Config,
		HandNumber:        handNumber,
		DealerSeat:        dealerSeat,
		SmallBlindSeat:    smallBlindSeat,
		BigBlindSeat:      bigBlindSeat,
		Street:            StreetPreflop,
		Players:           players,
		Board:             nil,
		FullBoard:         slices.Clone(deal.Board),
		CurrentWager:      m.Config.BigBlind,
		LastFullRaise:     m.Config.BigBlind,
		ActingSeat:        smallBlindSeat,
		actedThisStreet:   make([]bool, len(players)),
		raiseOptionOpened: make([]bool, len(players)),
	}

	if err := h.postBlind(smallBlindSeat, m.Config.SmallBlind); err != nil {
		return nil, err
	}
	if err := h.postBlind(bigBlindSeat, m.Config.BigBlind); err != nil {
		return nil, err
	}

	return h, nil
}

func (m *MatchState) FinalizeHand(hand *HandState) error {
	if hand == nil {
		return fmt.Errorf("finalize hand: hand is nil")
	}
	if len(hand.Players) != len(m.Players) {
		return fmt.Errorf("finalize hand: player count mismatch")
	}
	if !hand.Complete {
		return fmt.Errorf("finalize hand: hand is not complete")
	}
	if hand.ShowdownPending {
		return fmt.Errorf("finalize hand: showdown resolution is still pending")
	}

	for seat := range m.Players {
		m.Players[seat].Seat = seat
		m.Players[seat].Stack = hand.Players[seat].Stack
	}
	return nil
}

func (h *HandState) Pot() int {
	pot := 0
	for _, player := range h.Players {
		pot += player.Committed
	}
	return pot
}

func (h *HandState) ToCall(seat int) int {
	if seat < 0 || seat >= len(h.Players) {
		return 0
	}
	toCall := h.CurrentWager - h.Players[seat].StreetCommitted
	if toCall < 0 {
		return 0
	}
	return toCall
}

func (h *HandState) LegalActions() []LegalAction {
	if h.Complete {
		return nil
	}
	player := h.Players[h.ActingSeat]
	if !player.InHand || player.AllIn {
		return nil
	}

	toCall := h.ToCall(player.Seat)
	if toCall == 0 {
		actions := []LegalAction{{Type: ActionCheck}}
		actionType := ActionBet
		if h.CurrentWager > 0 {
			actionType = ActionRaise
		}
		if action, ok := h.legalAggression(player, actionType); ok {
			actions = append(actions, action)
		}
		return actions
	}

	actions := []LegalAction{{Type: ActionFold}}
	callAmount := toCall
	if callAmount > player.Stack {
		callAmount = player.Stack
	}
	actions = append(actions, LegalAction{Type: ActionCall, Amount: callAmount})
	if h.raiseAllowed(player) {
		if action, ok := h.legalAggression(player, ActionRaise); ok {
			actions = append(actions, action)
		}
	}
	return actions
}

func (h *HandState) ApplyAction(action Action) error {
	if h.Complete {
		return fmt.Errorf("apply action: hand is complete")
	}
	if action.Seat != h.ActingSeat {
		return fmt.Errorf("apply action: seat %d is not to act", action.Seat)
	}

	player := &h.Players[action.Seat]
	if !player.InHand {
		return fmt.Errorf("apply action: seat %d is not in hand", action.Seat)
	}
	if player.AllIn {
		return fmt.Errorf("apply action: seat %d is already all-in", action.Seat)
	}

	legal := h.LegalActions()
	if !matchesLegalAction(action, legal) {
		return fmt.Errorf("apply action: action %+v is not legal", action)
	}

	switch action.Type {
	case ActionFold:
		player.InHand = false
		h.actedThisStreet[action.Seat] = true
		h.ActionHistory = append(h.ActionHistory, Action{Seat: action.Seat, Type: ActionFold, Street: h.Street})
		h.finishByFold()
		return nil
	case ActionCheck:
		h.actedThisStreet[action.Seat] = true
		h.ActionHistory = append(h.ActionHistory, Action{Seat: action.Seat, Type: ActionCheck, Street: h.Street})
	case ActionCall:
		callAmount := h.ToCall(action.Seat)
		if callAmount > player.Stack {
			callAmount = player.Stack
		}
		h.commitChips(player, callAmount)
		h.actedThisStreet[action.Seat] = true
		h.ActionHistory = append(h.ActionHistory, Action{Seat: action.Seat, Type: ActionCall, Street: h.Street, Amount: callAmount, AllIn: player.AllIn})
	case ActionBet, ActionRaise:
		target := action.Amount
		previousWager := h.CurrentWager
		added := target - player.StreetCommitted
		h.commitChips(player, added)
		h.actedThisStreet[action.Seat] = true

		fullRaise := false
		if action.Type == ActionBet {
			fullRaise = target >= h.Config.BigBlind
			if fullRaise {
				h.LastFullRaise = target
			}
		} else {
			increment := target - previousWager
			fullRaise = increment >= h.LastFullRaise
			if fullRaise {
				h.LastFullRaise = increment
			}
		}
		h.CurrentWager = target
		if fullRaise {
			h.raiseOptionOpened[nextSeat(action.Seat, len(h.Players))] = true
		}
		h.ActionHistory = append(h.ActionHistory, Action{Seat: action.Seat, Type: action.Type, Street: h.Street, Amount: target, AllIn: player.AllIn})
	default:
		return fmt.Errorf("apply action: unsupported action type %q", action.Type)
	}

	h.advance()
	return nil
}

func (h *HandState) legalAggression(player PlayerState, actionType ActionType) (LegalAction, bool) {
	maxTotal := player.StreetCommitted + player.Stack
	if maxTotal <= h.CurrentWager {
		return LegalAction{}, false
	}

	minTotal := h.Config.BigBlind
	if actionType == ActionRaise {
		minTotal = h.CurrentWager + h.LastFullRaise
	}
	if minTotal > maxTotal {
		minTotal = maxTotal
	}

	return LegalAction{
		Type:      actionType,
		Amount:    maxTotal,
		MinAmount: minTotal,
		MaxAmount: maxTotal,
	}, true
}

func (h *HandState) raiseAllowed(player PlayerState) bool {
	if !player.InHand || player.AllIn {
		return false
	}
	if player.Stack <= h.ToCall(player.Seat) {
		return false
	}
	return !h.actedThisStreet[player.Seat] || h.raiseOptionOpened[player.Seat]
}

func (h *HandState) commitChips(player *PlayerState, amount int) {
	player.Stack -= amount
	player.Committed += amount
	player.StreetCommitted += amount
	if player.Stack == 0 {
		player.AllIn = true
	}
}

func (h *HandState) postBlind(seat int, amount int) error {
	player := &h.Players[seat]
	if player.Stack < amount {
		return fmt.Errorf("post blind: seat %d has stack %d, needs %d", seat, player.Stack, amount)
	}
	player.Stack -= amount
	player.Committed += amount
	player.StreetCommitted += amount
	blind := Action{Seat: seat, Type: ActionPostBlind, Street: StreetPreflop, Amount: amount, AllIn: player.Stack == 0}
	if player.Stack == 0 {
		player.AllIn = true
	}
	h.ActionHistory = append(h.ActionHistory, blind)
	return nil
}

func (h *HandState) advance() {
	if h.Complete {
		return
	}
	if h.activePlayers() == 1 {
		h.finishByFold()
		return
	}
	if h.noFurtherActionPossible() {
		h.finishForShowdown()
		return
	}
	if h.bettingRoundClosed() {
		h.advanceStreet()
		return
	}
	h.ActingSeat = h.nextActor()
}

func (h *HandState) activePlayers() int {
	count := 0
	for _, player := range h.Players {
		if player.InHand {
			count++
		}
	}
	return count
}

func (h *HandState) noFurtherActionPossible() bool {
	for _, player := range h.Players {
		if player.InHand && !player.AllIn {
			return false
		}
	}
	return true
}

func (h *HandState) bettingRoundClosed() bool {
	for seat, player := range h.Players {
		if !player.InHand || player.AllIn {
			continue
		}
		if !h.actedThisStreet[seat] {
			return false
		}
		if player.StreetCommitted != h.CurrentWager {
			return false
		}
	}
	return true
}

func (h *HandState) advanceStreet() {
	for seat := range h.Players {
		h.Players[seat].StreetCommitted = 0
		h.actedThisStreet[seat] = false
		h.raiseOptionOpened[seat] = false
	}
	h.CurrentWager = 0
	h.LastFullRaise = h.Config.BigBlind

	switch h.Street {
	case StreetPreflop:
		h.Street = StreetFlop
		h.Board = slices.Clone(h.FullBoard[:3])
	case StreetFlop:
		h.Street = StreetTurn
		h.Board = slices.Clone(h.FullBoard[:4])
	case StreetTurn:
		h.Street = StreetRiver
		h.Board = slices.Clone(h.FullBoard[:5])
	case StreetRiver:
		h.finishForShowdown()
		return
	default:
		panic("advanceStreet called from invalid street")
	}

	if h.noFurtherActionPossible() {
		h.finishForShowdown()
		return
	}
	h.ActingSeat = h.BigBlindSeat
	if !h.Players[h.ActingSeat].InHand || h.Players[h.ActingSeat].AllIn {
		h.ActingSeat = h.nextActor()
	}
}

func (h *HandState) nextActor() int {
	seat := nextSeat(h.ActingSeat, len(h.Players))
	for i := 0; i < len(h.Players); i++ {
		player := h.Players[seat]
		if player.InHand && !player.AllIn {
			return seat
		}
		seat = nextSeat(seat, len(h.Players))
	}
	return h.ActingSeat
}

func (h *HandState) finishByFold() {
	winnerSeat := -1
	for seat, player := range h.Players {
		if player.InHand {
			winnerSeat = seat
			break
		}
	}
	if winnerSeat < 0 {
		panic("finishByFold called without a remaining player")
	}

	loserSeat := nextSeat(winnerSeat, len(h.Players))
	refund := h.Players[winnerSeat].Committed - h.Players[loserSeat].Committed
	if refund < 0 {
		refund = 0
	}
	if refund > 0 {
		h.Players[winnerSeat].Committed -= refund
		h.Players[winnerSeat].Stack += refund
	}

	pot := h.Pot()
	h.Players[winnerSeat].Stack += pot
	for seat := range h.Players {
		h.Players[seat].Committed = 0
		h.Players[seat].StreetCommitted = 0
	}

	deltas := make([]ChipDelta, len(h.Players))
	for seat, player := range h.Players {
		deltas[seat] = ChipDelta{Seat: seat, Delta: player.Stack - player.StartingStack}
	}

	h.Complete = true
	h.ShowdownPending = false
	h.Result = &HandResult{
		Pot:          pot,
		WinningSeats: []int{winnerSeat},
		Deltas:       deltas,
		Showdown:     false,
	}
}

func (h *HandState) finishForShowdown() {
	h.Street = StreetShowdown
	h.Board = slices.Clone(h.FullBoard)
	h.Complete = true
	h.ShowdownPending = true
	h.ActingSeat = -1
}

func validateConfig(config Config) error {
	if config.PlayerCount != 2 {
		return fmt.Errorf("rules config: player count must be 2 for v0")
	}
	if config.StartingStack <= 0 {
		return fmt.Errorf("rules config: starting stack must be > 0")
	}
	if config.SmallBlind <= 0 {
		return fmt.Errorf("rules config: small blind must be > 0")
	}
	if config.BigBlind <= 0 {
		return fmt.Errorf("rules config: big blind must be > 0")
	}
	if config.SmallBlind >= config.BigBlind {
		return fmt.Errorf("rules config: small blind must be less than big blind")
	}
	return nil
}

func dealerSeatForHand(handNumber int, playerCount int) int {
	return (handNumber - 1) % playerCount
}

func nextSeat(seat int, playerCount int) int {
	return (seat + 1) % playerCount
}

func matchesLegalAction(action Action, legal []LegalAction) bool {
	for _, candidate := range legal {
		if candidate.Type != action.Type {
			continue
		}
		switch action.Type {
		case ActionFold, ActionCheck:
			return action.Amount == 0
		case ActionCall:
			return action.Amount == candidate.Amount
		case ActionBet, ActionRaise:
			return action.Amount >= candidate.MinAmount && action.Amount <= candidate.MaxAmount
		}
	}
	return false
}
