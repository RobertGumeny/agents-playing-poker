package match

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/RobertGumeny/agent-poker/internal/deck"
	"github.com/RobertGumeny/agent-poker/internal/rules"
	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
	"github.com/RobertGumeny/agent-poker/internal/wire"
)

const (
	defaultVariant        = "heads-up-nlhe"
	defaultInfoRealism    = "showdown-only"
	defaultAKGSpecVersion = "v1-draft-2"
)

var errDecisionTimeout = errors.New("decision timeout")

type AgentSpec struct {
	Name    string
	Command string
	Args    []string
	Env     []string
}

type Config struct {
	SessionID           string
	SessionsRootDir     string
	MatchID             string
	Seed                int64
	HandCount           int
	StartingStack       int
	SmallBlind          int
	BigBlind            int
	DecisionDeadline    time.Duration
	Variant             string
	InfoRealism         string
	ServerVersion       string
	AKGSpecVersion      string
	AgentSpecs          []AgentSpec
	ShutdownGracePeriod time.Duration
}

type RunResult struct {
	SessionDir string
	Completed  bool
}

type Runner struct {
	config Config
}

func NewRunner(config Config) (*Runner, error) {
	if config.SessionID == "" {
		return nil, fmt.Errorf("new runner: session id is required")
	}
	if config.SessionsRootDir == "" {
		return nil, fmt.Errorf("new runner: sessions root dir is required")
	}
	if config.MatchID == "" {
		return nil, fmt.Errorf("new runner: match id is required")
	}
	if len(config.AgentSpecs) != 2 {
		return nil, fmt.Errorf("new runner: exactly 2 agent specs are required")
	}
	if config.HandCount <= 0 {
		return nil, fmt.Errorf("new runner: hand count must be > 0")
	}
	if config.DecisionDeadline <= 0 {
		return nil, fmt.Errorf("new runner: decision deadline must be > 0")
	}
	if config.Variant == "" {
		config.Variant = defaultVariant
	}
	if config.InfoRealism == "" {
		config.InfoRealism = defaultInfoRealism
	}
	if config.ServerVersion == "" {
		config.ServerVersion = "dev"
	}
	if config.AKGSpecVersion == "" {
		config.AKGSpecVersion = defaultAKGSpecVersion
	}
	if config.ShutdownGracePeriod <= 0 {
		config.ShutdownGracePeriod = 2 * time.Second
	}
	for i, spec := range config.AgentSpecs {
		if spec.Name == "" {
			return nil, fmt.Errorf("new runner: agent %d name is required", i)
		}
		if spec.Command == "" {
			return nil, fmt.Errorf("new runner: agent %d command is required", i)
		}
	}
	return &Runner{config: config}, nil
}

func (r *Runner) Run(ctx context.Context) (result RunResult, runErr error) {
	writer, err := sessionlog.New(r.config.SessionsRootDir, r.config.SessionID)
	if err != nil {
		return RunResult{}, err
	}
	defer func() {
		closeErr := writer.Close()
		if runErr == nil && closeErr != nil {
			runErr = closeErr
		}
	}()

	startedAt := time.Now().UTC()
	result = RunResult{SessionDir: writer.SessionDir()}
	completed := false
	matchTotals := map[int]int{0: 0, 1: 0}
	agentVersions := map[int]string{}
	seats := make([]wire.Seat, len(r.config.AgentSpecs))
	for seat, spec := range r.config.AgentSpecs {
		seats[seat] = wire.Seat{Seat: seat, Name: spec.Name}
	}

	agents := make([]*agentProcess, 0, len(r.config.AgentSpecs))
	defer func() {
		for _, agent := range agents {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), r.config.ShutdownGracePeriod)
			_ = agent.Close(shutdownCtx)
			cancel()
		}
		manifestErr := writer.WriteManifest(r.buildManifest(startedAt, time.Now().UTC(), seats, agentVersions, matchTotals, completed))
		if runErr == nil && manifestErr != nil {
			runErr = manifestErr
		}
		result.Completed = completed
	}()

	for seat, spec := range r.config.AgentSpecs {
		agentDir, err := writer.AgentDir(spec.Name)
		if err != nil {
			return result, err
		}
		agent, err := startAgentProcess(ctx, seat, spec, agentDir)
		if err != nil {
			return result, err
		}
		agents = append(agents, agent)
	}

	for seat, agent := range agents {
		initMsg := wire.NewMessage(wire.MessageTypeSessionInit, agent.NextMessageID(), "", wire.SessionInitPayload{
			SessionID: r.config.SessionID,
			AgentName: r.config.AgentSpecs[seat].Name,
			Match: wire.MatchConfig{
				MatchID:            r.config.MatchID,
				Seed:               r.config.Seed,
				HandCount:          r.config.HandCount,
				Variant:            r.config.Variant,
				InfoRealism:        r.config.InfoRealism,
				StartingStack:      r.config.StartingStack,
				Blinds:             wire.BlindLevel{SB: r.config.SmallBlind, BB: r.config.BigBlind},
				DecisionDeadlineMS: int(r.config.DecisionDeadline / time.Millisecond),
			},
			Seats:     seats,
			YourSeat:  seat,
			MemoryDir: agent.Dir,
		})
		if err := agent.Send(initMsg); err != nil {
			return result, err
		}
		ready, err := agent.AwaitSessionReady(ctx, initMsg.ID, r.config.DecisionDeadline)
		if err != nil {
			return result, err
		}
		agentVersions[seat] = ready.Version
	}

	dealer := deck.NewDealer(uint64(r.config.Seed))
	rulesMatch, err := rules.NewHeadsUpMatch(r.config.StartingStack, r.config.SmallBlind, r.config.BigBlind)
	if err != nil {
		return result, err
	}

	for handNumber := 1; handNumber <= r.config.HandCount; handNumber++ {
		deal, err := dealer.DealHoldemHand(handNumber, 2)
		if err != nil {
			return result, fmt.Errorf("run hand %d: deal hand: %w", handNumber, err)
		}
		hand, err := rulesMatch.StartHand(handNumber, deal)
		if err != nil {
			return result, fmt.Errorf("run hand %d: start hand: %w", handNumber, err)
		}
		handActions := initialHandActions(hand)

		for seat, agent := range agents {
			if err := agent.Send(wire.NewMessage(wire.MessageTypeHandStart, agent.NextMessageID(), "", buildHandStartPayload(hand, seat))); err != nil {
				return result, fmt.Errorf("run hand %d: send hand_start to seat %d: %w", handNumber, seat, err)
			}
		}

		for !hand.Complete {
			actingSeat := hand.ActingSeat
			agent := agents[actingSeat]
			turnMsg := wire.NewMessage(wire.MessageTypeYourTurn, agent.NextMessageID(), "", buildYourTurnPayload(hand, seats))
			if err := agent.Send(turnMsg); err != nil {
				return result, fmt.Errorf("run hand %d: send your_turn to seat %d: %w", handNumber, actingSeat, err)
			}

			actionPayload, err := agent.AwaitAction(ctx, turnMsg.ID, r.config.DecisionDeadline)
			if err != nil {
				if errors.Is(err, errDecisionTimeout) {
					timeoutRulesAction, timeoutArtifactAction, timeoutErr := timeoutFallbackAction(hand, actingSeat)
					if timeoutErr != nil {
						return result, fmt.Errorf("run hand %d: choose timeout fallback action: %w", handNumber, timeoutErr)
					}
					handActions = append(handActions, timeoutArtifactAction)
					if applyErr := hand.ApplyAction(timeoutRulesAction); applyErr != nil {
						return result, fmt.Errorf("run hand %d: apply timeout fallback action: %w", handNumber, applyErr)
					}
					continue
				}
				return result, fmt.Errorf("run hand %d: await action from seat %d: %w", handNumber, actingSeat, err)
			}

			rulesAction, artifactAction, err := translateActionPayload(hand, actingSeat, actionPayload)
			if err != nil {
				return result, fmt.Errorf("run hand %d: translate action from seat %d: %w", handNumber, actingSeat, err)
			}
			if err := hand.ApplyAction(rulesAction); err != nil {
				return result, fmt.Errorf("run hand %d: apply action from seat %d: %w", handNumber, actingSeat, err)
			}
			handActions = append(handActions, artifactAction)
		}

		if hand.ShowdownPending {
			if err := hand.ResolveShowdown(); err != nil {
				return result, fmt.Errorf("run hand %d: resolve showdown: %w", handNumber, err)
			}
		}
		if err := rulesMatch.FinalizeHand(hand); err != nil {
			return result, fmt.Errorf("run hand %d: finalize hand: %w", handNumber, err)
		}

		handRecord := buildHandRecord(r.config.MatchID, hand, handActions)
		if err := writer.AppendHand(handRecord); err != nil {
			return result, fmt.Errorf("run hand %d: append hand record: %w", handNumber, err)
		}
		for _, delta := range hand.Result.Deltas {
			matchTotals[delta.Seat] += delta.Delta
		}
		for seat, agent := range agents {
			if err := agent.Send(wire.NewMessage(wire.MessageTypeHandEnd, agent.NextMessageID(), "", buildHandEndPayload(r.config.InfoRealism, hand, seat))); err != nil {
				return result, fmt.Errorf("run hand %d: send hand_end to seat %d: %w", handNumber, seat, err)
			}
		}
	}

	for _, agent := range agents {
		if err := agent.Send(wire.NewMessage(wire.MessageTypeSessionEnd, agent.NextMessageID(), "", wire.SessionEndPayload{})); err != nil {
			return result, err
		}
	}
	completed = true
	return result, nil
}

func (r *Runner) buildManifest(startedAt time.Time, endedAt time.Time, seats []wire.Seat, agentVersions map[int]string, matchTotals map[int]int, completed bool) sessionlog.Manifest {
	manifestSeats := make([]sessionlog.ManifestSeat, len(seats))
	for i, seat := range seats {
		manifestSeats[i] = sessionlog.ManifestSeat{Seat: seat.Seat, Name: seat.Name, Version: agentVersions[seat.Seat]}
	}
	return sessionlog.Manifest{
		SessionID:     r.config.SessionID,
		StartedAt:     startedAt.Format(time.RFC3339),
		EndedAt:       endedAt.Format(time.RFC3339),
		Seed:          r.config.Seed,
		HandCount:     r.config.HandCount,
		Variant:       r.config.Variant,
		InfoRealism:   r.config.InfoRealism,
		StartingStack: r.config.StartingStack,
		Blinds:        sessionlog.BlindLevel{SB: r.config.SmallBlind, BB: r.config.BigBlind},
		Matches: []sessionlog.ManifestMatch{{
			MatchID: r.config.MatchID,
			Seats:   manifestSeats,
			Result: map[int]sessionlog.ManifestSeatResult{
				0: {ChipsDelta: matchTotals[0]},
				1: {ChipsDelta: matchTotals[1]},
			},
			Completed: completed,
		}},
		ServerVersion:  r.config.ServerVersion,
		AKGSpecVersion: r.config.AKGSpecVersion,
	}
}

func buildHandStartPayload(hand *rules.HandState, seat int) wire.HandStartPayload {
	stacks := make(map[int]int, len(hand.Players))
	for _, player := range hand.Players {
		stacks[player.Seat] = player.StartingStack
	}
	return wire.HandStartPayload{
		HandNumber: hand.HandNumber,
		DealerSeat: hand.DealerSeat,
		Stacks:     stacks,
		BlindsPosted: []wire.BlindPosting{
			{Seat: hand.SmallBlindSeat, Amount: hand.Config.SmallBlind},
			{Seat: hand.BigBlindSeat, Amount: hand.Config.BigBlind},
		},
		YourHoleCards: []string{hand.Players[seat].HoleCards[0].String(), hand.Players[seat].HoleCards[1].String()},
	}
}

func buildYourTurnPayload(hand *rules.HandState, seats []wire.Seat) wire.YourTurnPayload {
	stacks := make(map[int]int, len(hand.Players))
	for _, player := range hand.Players {
		stacks[player.Seat] = player.Stack
	}
	return wire.YourTurnPayload{
		HandNumber:    hand.HandNumber,
		Street:        hand.Street.String(),
		Board:         cardsToStrings(hand.Board),
		Pot:           hand.Pot(),
		ToCall:        hand.ToCall(hand.ActingSeat),
		Stacks:        stacks,
		Seats:         append([]wire.Seat(nil), seats...),
		ActionHistory: rulesActionsToWire(hand.ActionHistory),
		LegalActions:  rulesLegalActionsToWire(hand.LegalActions()),
	}
}

func buildHandEndPayload(infoRealism string, hand *rules.HandState, recipientSeat int) wire.HandEndPayload {
	showdown := make(map[int]wire.ShowdownSeat)
	if hand.Result.Showdown {
		for _, showdownHand := range hand.Result.ShowdownHands {
			showdown[showdownHand.Seat] = wire.ShowdownSeat{
				HoleCards: []string{showdownHand.HoleCards[0].String(), showdownHand.HoleCards[1].String()},
				Rank:      showdownHand.Label,
			}
		}
	} else if infoRealism == "perfect-info" {
		for _, player := range hand.Players {
			showdown[player.Seat] = wire.ShowdownSeat{
				HoleCards: []string{player.HoleCards[0].String(), player.HoleCards[1].String()},
			}
		}
	} else {
		for _, winningSeat := range hand.Result.WinningSeats {
			player := hand.Players[winningSeat]
			showdown[winningSeat] = wire.ShowdownSeat{HoleCards: []string{player.HoleCards[0].String(), player.HoleCards[1].String()}}
		}
		if infoRealism == "perfect-info" {
			player := hand.Players[recipientSeat]
			showdown[player.Seat] = wire.ShowdownSeat{HoleCards: []string{player.HoleCards[0].String(), player.HoleCards[1].String()}}
		}
	}
	return wire.HandEndPayload{
		HandNumber:      hand.HandNumber,
		Board:           cardsToStrings(hand.Board),
		ActionHistory:   rulesActionsToWire(hand.ActionHistory),
		ShowdownReached: hand.Result.Showdown,
		Showdown:        showdown,
		Result:          chipDeltasToWire(hand.Result.Deltas),
	}
}

func buildHandRecord(matchID string, hand *rules.HandState, actions []sessionlog.HandAction) sessionlog.HandRecord {
	holeCards := make(map[int][]string, len(hand.Players))
	stacksStart := make(map[int]int, len(hand.Players))
	for _, player := range hand.Players {
		holeCards[player.Seat] = []string{player.HoleCards[0].String(), player.HoleCards[1].String()}
		stacksStart[player.Seat] = player.StartingStack
	}
	return sessionlog.HandRecord{
		MatchID:     matchID,
		HandNumber:  hand.HandNumber,
		DealerSeat:  hand.DealerSeat,
		StacksStart: stacksStart,
		BlindsPosted: []sessionlog.BlindPosting{
			{Seat: hand.SmallBlindSeat, Amount: hand.Config.SmallBlind},
			{Seat: hand.BigBlindSeat, Amount: hand.Config.BigBlind},
		},
		HoleCards:       holeCards,
		Board:           cardsToStrings(hand.Board),
		Actions:         actions,
		ShowdownReached: hand.Result.Showdown,
		Result:          chipDeltasToSessionLog(hand.Result.Deltas),
	}
}

func initialHandActions(hand *rules.HandState) []sessionlog.HandAction {
	actions := make([]sessionlog.HandAction, 0, len(hand.ActionHistory))
	for _, action := range hand.ActionHistory {
		actions = append(actions, rulesActionToSessionLog(action))
	}
	return actions
}

func timeoutFallbackAction(hand *rules.HandState, seat int) (rules.Action, sessionlog.HandAction, error) {
	legalActions := hand.LegalActions()
	for _, legalAction := range legalActions {
		if legalAction.Type == rules.ActionCheck {
			return rules.Action{Seat: seat, Street: hand.Street, Type: rules.ActionCheck}, sessionlog.HandAction{Seat: seat, Action: "auto_check", Street: hand.Street.String(), ForcedReason: "decision_timeout"}, nil
		}
	}
	for _, legalAction := range legalActions {
		if legalAction.Type == rules.ActionFold {
			return rules.Action{Seat: seat, Street: hand.Street, Type: rules.ActionFold}, sessionlog.HandAction{Seat: seat, Action: "auto_fold", Street: hand.Street.String(), ForcedReason: "decision_timeout"}, nil
		}
	}
	return rules.Action{}, sessionlog.HandAction{}, fmt.Errorf("no legal timeout fallback action for seat %d", seat)
}

func translateActionPayload(hand *rules.HandState, seat int, payload wire.ActionPayload) (rules.Action, sessionlog.HandAction, error) {
	rulesAction := rules.Action{Seat: seat, Street: hand.Street}
	sessionAction := sessionlog.HandAction{Seat: seat, Action: payload.Action, Street: hand.Street.String()}
	switch payload.Action {
	case string(rules.ActionFold):
		rulesAction.Type = rules.ActionFold
	case string(rules.ActionCheck):
		rulesAction.Type = rules.ActionCheck
	case string(rules.ActionCall):
		if payload.Amount == nil {
			return rules.Action{}, sessionlog.HandAction{}, fmt.Errorf("call action missing amount")
		}
		rulesAction.Type = rules.ActionCall
		rulesAction.Amount = *payload.Amount
		sessionAction.Amount = intPtr(*payload.Amount)
	case string(rules.ActionBet):
		if payload.Amount == nil {
			return rules.Action{}, sessionlog.HandAction{}, fmt.Errorf("bet action missing amount")
		}
		rulesAction.Type = rules.ActionBet
		rulesAction.Amount = *payload.Amount
		sessionAction.Amount = intPtr(*payload.Amount)
	case string(rules.ActionRaise):
		if payload.Amount == nil {
			return rules.Action{}, sessionlog.HandAction{}, fmt.Errorf("raise action missing amount")
		}
		rulesAction.Type = rules.ActionRaise
		rulesAction.Amount = *payload.Amount
		sessionAction.Amount = intPtr(*payload.Amount)
	default:
		return rules.Action{}, sessionlog.HandAction{}, fmt.Errorf("unsupported action %q", payload.Action)
	}
	return rulesAction, sessionAction, nil
}

func rulesActionsToWire(actions []rules.Action) []wire.ActionRecord {
	out := make([]wire.ActionRecord, 0, len(actions))
	for _, action := range actions {
		out = append(out, wire.ActionRecord{Seat: action.Seat, Action: string(action.Type), Amount: amountPtrForRulesAction(action), Street: action.Street.String()})
	}
	return out
}

func rulesLegalActionsToWire(actions []rules.LegalAction) []wire.LegalActionOption {
	out := make([]wire.LegalActionOption, 0, len(actions))
	for _, action := range actions {
		option := wire.LegalActionOption{Action: string(action.Type)}
		switch action.Type {
		case rules.ActionCall:
			option.Amount = intPtr(action.Amount)
		case rules.ActionBet, rules.ActionRaise:
			option.Min = intPtr(action.MinAmount)
			option.Max = intPtr(action.MaxAmount)
		}
		out = append(out, option)
	}
	return out
}

func chipDeltasToWire(deltas []rules.ChipDelta) []wire.HandResult {
	out := make([]wire.HandResult, 0, len(deltas))
	for _, delta := range deltas {
		out = append(out, wire.HandResult{Seat: delta.Seat, ChipsDelta: delta.Delta})
	}
	return out
}

func chipDeltasToSessionLog(deltas []rules.ChipDelta) []sessionlog.HandResult {
	out := make([]sessionlog.HandResult, 0, len(deltas))
	for _, delta := range deltas {
		out = append(out, sessionlog.HandResult{Seat: delta.Seat, ChipsDelta: delta.Delta})
	}
	return out
}

func rulesActionToSessionLog(action rules.Action) sessionlog.HandAction {
	return sessionlog.HandAction{Seat: action.Seat, Action: string(action.Type), Amount: amountPtrForRulesAction(action), Street: action.Street.String()}
}

func amountPtrForRulesAction(action rules.Action) *int {
	switch action.Type {
	case rules.ActionFold, rules.ActionCheck:
		return nil
	default:
		return intPtr(action.Amount)
	}
}

func cardsToStrings(cards []deck.Card) []string {
	out := make([]string, 0, len(cards))
	for _, card := range cards {
		out = append(out, card.String())
	}
	return out
}

func intPtr(v int) *int {
	return &v
}

type agentProcess struct {
	seat       int
	Dir        string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdoutFile *os.File
	stderrFile *os.File
	incoming   chan incomingMessage
	exit       chan error
	messageSeq int
	mu         sync.Mutex
}

type incomingMessage struct {
	envelope wire.Envelope
	err      error
}

func startAgentProcess(ctx context.Context, seat int, spec AgentSpec, dir string) (*agentProcess, error) {
	stdoutPath := filepath.Join(dir, "stdout.log")
	stderrPath := filepath.Join(dir, "stderr.log")
	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return nil, fmt.Errorf("start agent process: create stdout log: %w", err)
	}
	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		_ = stdoutFile.Close()
		return nil, fmt.Errorf("start agent process: create stderr log: %w", err)
	}

	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Stderr = stderrFile
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = stdoutFile.Close()
		_ = stderrFile.Close()
		return nil, fmt.Errorf("start agent process: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdoutFile.Close()
		_ = stderrFile.Close()
		return nil, fmt.Errorf("start agent process: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdoutFile.Close()
		_ = stderrFile.Close()
		return nil, fmt.Errorf("start agent process: start %s: %w", spec.Name, err)
	}

	agent := &agentProcess{
		seat:       seat,
		Dir:        dir,
		cmd:        cmd,
		stdin:      stdin,
		stdoutFile: stdoutFile,
		stderrFile: stderrFile,
		incoming:   make(chan incomingMessage, 64),
		exit:       make(chan error, 1),
	}
	go agent.readStdout(stdout)
	go func() {
		agent.exit <- cmd.Wait()
		close(agent.exit)
	}()
	return agent, nil
}

func (a *agentProcess) NextMessageID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messageSeq++
	return fmt.Sprintf("msg-%d", a.messageSeq)
}

func (a *agentProcess) Send(msg any) error {
	if err := json.NewEncoder(a.stdin).Encode(msg); err != nil {
		return fmt.Errorf("send message to seat %d: %w", a.seat, err)
	}
	return nil
}

func (a *agentProcess) AwaitSessionReady(ctx context.Context, inReplyTo string, timeout time.Duration) (wire.SessionReadyPayload, error) {
	payload, err := a.awaitCorrelatedPayload(ctx, timeout, inReplyTo, wire.MessageTypeSessionReady)
	if err != nil {
		return wire.SessionReadyPayload{}, err
	}
	var ready wire.SessionReadyPayload
	if err := payload.DecodePayload(&ready); err != nil {
		return wire.SessionReadyPayload{}, err
	}
	return ready, nil
}

func (a *agentProcess) AwaitAction(ctx context.Context, inReplyTo string, timeout time.Duration) (wire.ActionPayload, error) {
	envelope, err := a.awaitCorrelatedPayload(ctx, timeout, inReplyTo, wire.MessageTypeAction)
	if err != nil {
		return wire.ActionPayload{}, err
	}
	var action wire.ActionPayload
	if err := envelope.DecodePayload(&action); err != nil {
		return wire.ActionPayload{}, err
	}
	return action, nil
}

func (a *agentProcess) awaitCorrelatedPayload(ctx context.Context, timeout time.Duration, inReplyTo string, wantType wire.MessageType) (wire.Envelope, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return wire.Envelope{}, fmt.Errorf("await %s from seat %d: %w", wantType, a.seat, ctx.Err())
		case <-deadline.C:
			return wire.Envelope{}, errDecisionTimeout
		case incoming, ok := <-a.incoming:
			if !ok {
				return wire.Envelope{}, fmt.Errorf("await %s from seat %d: stdout closed", wantType, a.seat)
			}
			if incoming.err != nil {
				return wire.Envelope{}, fmt.Errorf("await %s from seat %d: %w", wantType, a.seat, incoming.err)
			}
			if incoming.envelope.Type == wire.MessageTypeLog {
				continue
			}
			if incoming.envelope.Type != wantType || incoming.envelope.InReplyTo != inReplyTo {
				continue
			}
			return incoming.envelope, nil
		}
	}
}

func (a *agentProcess) readStdout(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		_, _ = a.stdoutFile.Write(append(line, '\n'))
		envelope, err := wire.DecodeEnvelope(line)
		if err != nil {
			a.incoming <- incomingMessage{err: err}
			continue
		}
		a.incoming <- incomingMessage{envelope: envelope}
	}
	if err := scanner.Err(); err != nil {
		a.incoming <- incomingMessage{err: fmt.Errorf("read stdout: %w", err)}
	}
	close(a.incoming)
}

func (a *agentProcess) Close(ctx context.Context) error {
	if a == nil {
		return nil
	}
	_ = a.stdin.Close()
	waitCh := a.exit
	select {
	case <-ctx.Done():
		if a.cmd.Process != nil {
			_ = a.cmd.Process.Kill()
		}
	case <-waitCh:
	}
	var errs bytes.Buffer
	if err := a.stdoutFile.Close(); err != nil {
		errs.WriteString(err.Error())
	}
	if err := a.stderrFile.Close(); err != nil {
		if errs.Len() > 0 {
			errs.WriteString("; ")
		}
		errs.WriteString(err.Error())
	}
	if errs.Len() > 0 {
		return errors.New(errs.String())
	}
	return nil
}
