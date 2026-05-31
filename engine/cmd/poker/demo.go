package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runDemo(args []string, stdout, _ io.Writer) error {
	fs := flag.NewFlagSet("poker demo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	sessionID := fs.String("session-id", defaultDemoSessionID(), "session id")
	sessionsDir := fs.String("sessions-dir", "research/sessions", "session output root directory")
	seed := fs.Int64("seed", 17, "deterministic match seed")
	handCount := fs.Int("hand-count", 200, "number of hands to play")

	if err := fs.Parse(args); err != nil {
		return err
	}

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

	for _, target := range []struct{ pkg, bin string }{
		{"./cmd/poker-server", serverBin},
		{"./cmd/random-agent", randomBin},
		{"./cmd/heuristic-agent", heuristicBin},
	} {
		if err := buildDemoBinary(repoDir, target.pkg, target.bin); err != nil {
			return err
		}
	}

	serverArgs := []string{
		"-sessions-dir", *sessionsDir,
		"-session-id", *sessionID,
		"-match-id", "mat_demo",
		"-seed", fmt.Sprintf("%d", *seed),
		"-hand-count", fmt.Sprintf("%d", *handCount),
		"-starting-stack", "200",
		"-small-blind", "1",
		"-big-blind", "2",
		"-decision-deadline", "30s",
		"-agent0-name", "random",
		"-agent0-cmd", randomBin,
		"-agent1-name", "heuristic",
		"-agent1-cmd", heuristicBin,
	}

	cmd := exec.Command(serverBin, serverArgs...)
	cmd.Dir = repoDir

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run demo match: %w\n%s%s", err, outBuf.String(), errBuf.String())
	}

	sessionDir := filepath.Join(*sessionsDir, *sessionID)
	if !filepath.IsAbs(sessionDir) {
		sessionDir = filepath.Join(repoDir, sessionDir)
	}

	_, _ = fmt.Fprintf(stdout, "demo=random-vs-heuristic session_dir=%s\n", sessionDir)
	_, _ = fmt.Fprintf(stdout, "inspect_next: manifest=%s\n", filepath.Join(sessionDir, "manifest.json"))
	_, _ = fmt.Fprintf(stdout, "inspect_next: hands=%s\n", filepath.Join(sessionDir, "hands.jsonl"))
	_, _ = fmt.Fprintf(stdout, "inspect_next: agent_logs=%s\n", filepath.Join(sessionDir, "agents"))
	return nil
}

func buildDemoBinary(repoDir, pkg, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(outputPath), err)
	}
	cmd := exec.Command("go", "build", "-o", outputPath, pkg)
	cmd.Dir = repoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build %s: %w\n%s%s", pkg, err, stdout.String(), stderr.String())
	}
	return nil
}

func defaultDemoSessionID() string {
	return "ses_" + strings.ReplaceAll(time.Now().UTC().Format("2006-01-02T15-04-05Z"), ":", "-")
}

func binaryName(name string) string {
	// runtime.GOOS is available at compile time here
	return platformBinaryName(name)
}
