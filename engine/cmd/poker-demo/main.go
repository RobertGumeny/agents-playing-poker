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
	cfg := parseConfig()

	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	binDir, err := os.MkdirTemp("", "poker-demo-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(binDir)

	binaries := cfg.binaryPaths(binDir)
	for _, target := range cfg.buildTargets(binaries) {
		if err := buildDemoBinary(cfg.goBinary, repoDir, target.pkg, target.bin); err != nil {
			return err
		}
	}

	cmd := exec.Command(binaries.server, cfg.serverArgs(binaries)...)
	cmd.Dir = repoDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run scripted demo: %w\n%s%s", err, stdout.String(), stderr.String())
	}

	inspect := cfg.inspectPaths(repoDir)
	fmt.Printf("demo=random-vs-heuristic session_dir=%s\n", inspect.sessionDir)
	fmt.Printf("inspect_next: manifest=%s\n", inspect.manifest)
	fmt.Printf("inspect_next: hands=%s\n", inspect.hands)
	fmt.Printf("inspect_next: agent_logs=%s\n", inspect.agentLogs)
	return nil
}

type demoConfig struct {
	sessionID        string
	sessionsDir      string
	matchID          string
	seed             int64
	handCount        int
	startingStack    int
	smallBlind       int
	bigBlind         int
	decisionDeadline time.Duration
	goBinary         string
}

type demoBinaries struct {
	server    string
	random    string
	heuristic string
}

type buildTarget struct {
	pkg string
	bin string
}

type inspectPaths struct {
	sessionDir string
	manifest   string
	hands      string
	agentLogs  string
}

func parseConfig() demoConfig {
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

	return demoConfig{
		sessionID:        *sessionID,
		sessionsDir:      *sessionsDir,
		matchID:          *matchID,
		seed:             *seed,
		handCount:        *handCount,
		startingStack:    *startingStack,
		smallBlind:       *smallBlind,
		bigBlind:         *bigBlind,
		decisionDeadline: *decisionDeadline,
		goBinary:         *goBinary,
	}
}

func (c demoConfig) binaryPaths(binDir string) demoBinaries {
	return demoBinaries{
		server:    filepath.Join(binDir, binaryName("poker-server")),
		random:    filepath.Join(binDir, binaryName("random-agent")),
		heuristic: filepath.Join(binDir, binaryName("heuristic-agent")),
	}
}

func (c demoConfig) buildTargets(binaries demoBinaries) []buildTarget {
	return []buildTarget{
		{pkg: "./cmd/poker-server", bin: binaries.server},
		{pkg: "./cmd/random-agent", bin: binaries.random},
		{pkg: "./cmd/heuristic-agent", bin: binaries.heuristic},
	}
}

func (c demoConfig) serverArgs(binaries demoBinaries) []string {
	return []string{
		"-sessions-dir", c.sessionsDir,
		"-session-id", c.sessionID,
		"-match-id", c.matchID,
		"-seed", fmt.Sprintf("%d", c.seed),
		"-hand-count", fmt.Sprintf("%d", c.handCount),
		"-starting-stack", fmt.Sprintf("%d", c.startingStack),
		"-small-blind", fmt.Sprintf("%d", c.smallBlind),
		"-big-blind", fmt.Sprintf("%d", c.bigBlind),
		"-decision-deadline", c.decisionDeadline.String(),
		"-agent0-name", "random",
		"-agent0-cmd", binaries.random,
		"-agent1-name", "heuristic",
		"-agent1-cmd", binaries.heuristic,
	}
}

func (c demoConfig) inspectPaths(repoDir string) inspectPaths {
	sessionDir := filepath.Join(c.sessionsDir, c.sessionID)
	if !filepath.IsAbs(sessionDir) {
		sessionDir = filepath.Join(repoDir, sessionDir)
	}
	return inspectPaths{
		sessionDir: sessionDir,
		manifest:   filepath.Join(sessionDir, "manifest.json"),
		hands:      filepath.Join(sessionDir, "hands.jsonl"),
		agentLogs:  filepath.Join(sessionDir, "agents"),
	}
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
