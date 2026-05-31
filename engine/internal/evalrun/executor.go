package evalrun

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
)

type Executor struct {
	RepoDir     string
	GoBinary    string
	BuildBinary func(repoDir, goBinary, pkg, outputPath string) error
	RunCommand  func(*exec.Cmd) error
	Stdout      io.Writer
	Stderr      io.Writer
	binaryPath  string
	buildOnce   sync.Once
	buildErr    error
}

func NewExecutor(repoDir string, stdout, stderr io.Writer) *Executor {
	return &Executor{
		RepoDir:     repoDir,
		GoBinary:    "go",
		BuildBinary: BuildGoBinary,
		RunCommand:  func(cmd *exec.Cmd) error { return cmd.Run() },
		Stdout:      stdout,
		Stderr:      stderr,
	}
}

func (e *Executor) Execute(ctx context.Context, cfg ExecuteConfig) error {
	binaryPath, err := e.ensureBinary()
	if err != nil {
		return err
	}

	args := []string{
		"match", "run",
		"-agent0", cfg.Agent0,
		"-agent1", cfg.Agent1,
		"-hands", strconv.Itoa(cfg.Hands),
		"-seed", strconv.FormatInt(cfg.Seed, 10),
		"-session-id", cfg.SessionID,
		"-sessions-dir", cfg.SessionsDir,
		"-thinking-level", cfg.ThinkingLevel,
	}
	if cfg.Model != "" {
		args = append(args, "-model", cfg.Model)
	}

	stdout := e.Stdout
	if cfg.Stdout != nil {
		stdout = cfg.Stdout
	}
	stderr := e.Stderr
	if cfg.Stderr != nil {
		stderr = cfg.Stderr
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = e.RepoDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := e.RunCommand(cmd); err != nil {
		return fmt.Errorf("execute poker match run: %w", err)
	}
	return nil
}

// PrepareBinary builds the poker-run binary if not already built. Safe to call
// before spawning parallel Execute calls to avoid a build race.
func (e *Executor) PrepareBinary() (string, error) {
	return e.ensureBinary()
}

func (e *Executor) ensureBinary() (string, error) {
	outputPath := filepath.Join(e.RepoDir, ".tmp", "bin", BinaryName("poker"))
	e.buildOnce.Do(func() {
		if err := e.BuildBinary(e.RepoDir, e.GoBinary, "./cmd/poker", outputPath); err != nil {
			e.buildErr = err
			return
		}
		e.binaryPath = outputPath
	})
	return e.binaryPath, e.buildErr
}

func BuildGoBinary(repoDir, goBinary, pkg, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(outputPath), err)
	}

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

func BinaryName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
