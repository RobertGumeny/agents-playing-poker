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

	binDir, err := os.MkdirTemp("", "poker-demo-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(binDir)

	serverBin := filepath.Join(binDir, binaryName("poker-server"))
	randomBin := filepath.Join(binDir, binaryName("random-agent"))
	heuristicBin := filepath.Join(binDir, binaryName("heuristic-agent"))
	for _, target := range []struct {
		pkg string
		bin string
	}{
		{pkg: "./cmd/poker-server", bin: serverBin},
		{pkg: "./cmd/random-agent", bin: randomBin},
		{pkg: "./cmd/heuristic-agent", bin: heuristicBin},
	} {
		if err := buildDemoBinary(*goBinary, repoDir, target.pkg, target.bin); err != nil {
			return err
		}
	}

	args := []string{
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
		"-agent0-cmd", randomBin,
		"-agent1-name", "heuristic",
		"-agent1-cmd", heuristicBin,
	}

	cmd := exec.Command(serverBin, args...)
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

	manifestPath := filepath.Join(sessionDir, "manifest.json")
	handsPath := filepath.Join(sessionDir, "hands.jsonl")
	agentsDir := filepath.Join(sessionDir, "agents")

	fmt.Printf("demo=random-vs-heuristic session_dir=%s\n", sessionDir)
	fmt.Printf("inspect_next: manifest=%s\n", manifestPath)
	fmt.Printf("inspect_next: hands=%s\n", handsPath)
	fmt.Printf("inspect_next: agent_logs=%s\n", agentsDir)
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

func buildDemoBinary(goBinary, repoDir, pkg, outputPath string) error {
	cmd := exec.Command(goBinary, "build", "-o", outputPath, pkg)
	cmd.Dir = repoDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build %s: %w\n%s%s", pkg, err, stdout.String(), stderr.String())
	}
	return nil
}

func binaryName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
