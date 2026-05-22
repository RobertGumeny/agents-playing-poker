package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/RobertGumeny/agent-poker/internal/match"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var agent0Args multiFlag
	var agent1Args multiFlag

	sessionID := flag.String("session-id", defaultSessionID(), "session id")
	sessionsDir := flag.String("sessions-dir", "sessions", "session output root directory")
	matchID := flag.String("match-id", "mat_001", "match id")
	seed := flag.Int64("seed", 1, "deterministic match seed")
	handCount := flag.Int("hand-count", 200, "number of hands to play")
	startingStack := flag.Int("starting-stack", 200, "starting stack in chips")
	smallBlind := flag.Int("small-blind", 1, "small blind")
	bigBlind := flag.Int("big-blind", 2, "big blind")
	decisionDeadline := flag.Duration("decision-deadline", 30*time.Second, "decision deadline")
	agent0Name := flag.String("agent0-name", "agent-0", "seat 0 agent name")
	agent0Cmd := flag.String("agent0-cmd", "", "seat 0 agent command")
	agent1Name := flag.String("agent1-name", "agent-1", "seat 1 agent name")
	agent1Cmd := flag.String("agent1-cmd", "", "seat 1 agent command")
	flag.Var(&agent0Args, "agent0-arg", "seat 0 agent argument (repeatable)")
	flag.Var(&agent1Args, "agent1-arg", "seat 1 agent argument (repeatable)")
	flag.Parse()

	if *agent0Cmd == "" || *agent1Cmd == "" {
		return fmt.Errorf("both -agent0-cmd and -agent1-cmd are required")
	}

	runner, err := match.NewRunner(match.Config{
		SessionID:        *sessionID,
		SessionsRootDir:  *sessionsDir,
		MatchID:          *matchID,
		Seed:             *seed,
		HandCount:        *handCount,
		StartingStack:    *startingStack,
		SmallBlind:       *smallBlind,
		BigBlind:         *bigBlind,
		DecisionDeadline: *decisionDeadline,
		AgentSpecs: []match.AgentSpec{
			{Name: *agent0Name, Command: *agent0Cmd, Args: agent0Args},
			{Name: *agent1Name, Command: *agent1Cmd, Args: agent1Args},
		},
	})
	if err != nil {
		return err
	}

	result, err := runner.Run(context.Background())
	if err != nil {
		return err
	}
	fmt.Printf("session_dir=%s completed=%t\n", result.SessionDir, result.Completed)
	return nil
}

func defaultSessionID() string {
	return "ses_" + strings.ReplaceAll(time.Now().UTC().Format("2006-01-02T15-04-05Z"), ":", "-")
}

type multiFlag []string

func (f *multiFlag) String() string {
	return strings.Join(*f, " ")
}

func (f *multiFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}
