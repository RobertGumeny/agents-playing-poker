package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	sessionID := flag.String("session-id", defaultSessionID(), "session id")
	sessionsDir := flag.String("sessions-dir", "sessions", "session output root directory")
	matchID := flag.String("match-id", "mat_demo", "match id")
	seed := flag.Int64("seed", 17, "deterministic match seed")
	handCount := flag.Int("hand-count", 200, "number of hands to play")
	startingStack := flag.Int("starting-stack", 200, "starting stack in chips")
	smallBlind := flag.Int("small-blind", 1, "small blind")
	bigBlind := flag.Int("big-blind", 2, "big blind")
	decisionDeadline := flag.Duration("decision-deadline", 30*time.Second, "decision deadline")
	goBinary := flag.String("go-bin", "go", "Go binary used to launch the demo")
	flag.Parse()

	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	args := []string{
		"-C", repoDir,
		"run", "./cmd/poker-server",
		"-sessions-dir", *sessionsDir,
		"-session-id", *sessionID,
		"-match-id", *matchID,
		"-seed", fmt.Sprintf("%d", *seed),
		"-hand-count", fmt.Sprintf("%d", *handCount),
		"-starting-stack", fmt.Sprintf("%d", *startingStack),
		"-small-blind", fmt.Sprintf("%d", *smallBlind),
		"-big-blind", fmt.Sprintf("%d", *bigBlind),
		"-decision-deadline", decisionDeadline.String(),
		"-agent0-name", "random",
		"-agent0-cmd", *goBinary,
		"-agent0-arg", "-C",
		"-agent0-arg", repoDir,
		"-agent0-arg", "run",
		"-agent0-arg", "./cmd/random-agent",
		"-agent1-name", "heuristic",
		"-agent1-cmd", *goBinary,
		"-agent1-arg", "-C",
		"-agent1-arg", repoDir,
		"-agent1-arg", "run",
		"-agent1-arg", "./cmd/heuristic-agent",
	}

	cmd := exec.Command(*goBinary, args...)
	cmd.Dir = repoDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run scripted demo: %w\n%s%s", err, stdout.String(), stderr.String())
	}

	sessionDir := filepath.Join(*sessionsDir, *sessionID)
	if !filepath.IsAbs(sessionDir) {
		sessionDir = filepath.Join(repoDir, sessionDir)
	}

	fmt.Printf("demo=random-vs-heuristic session_dir=%s\n", sessionDir)
	fmt.Printf("inspect: jq . %s\n", filepath.Join(sessionDir, "manifest.json"))
	return nil
}

func defaultSessionID() string {
	return "ses_" + strings.ReplaceAll(time.Now().UTC().Format("2006-01-02T15-04-05Z"), ":", "-")
}

func repoRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..")), nil
}
