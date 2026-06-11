package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/RobertGumeny/agent-poker/internal/replay"
)

// runReplay generates a self-contained HTML poker-table replay from a finished
// session's artifacts. It is the only cmd/poker file that imports internal/replay,
// keeping the gather/model/render seam out of the command layer.
func runReplay(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	out := fs.String("out", "", "output HTML path (default <session-dir>/replay.html)")
	fs.StringVar(out, "o", "", "shorthand for --out")
	// Permit flags and the positional arg in any order (Go's flag stops at the
	// first non-flag), so `replay <dir> -o out` works as readily as flags-first.
	var positional []string
	rest := args
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			return err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positional = append(positional, rest[0])
		rest = rest[1:]
	}
	if len(positional) != 1 {
		return fmt.Errorf("usage: poker replay <session-dir> [-o out.html]")
	}
	sessionDir := positional[0]

	model, warnings, err := replay.BuildModel(sessionDir)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintf(stderr, "warning: %s\n", w)
	}

	outPath := *out
	if outPath == "" {
		outPath = filepath.Join(sessionDir, "replay.html")
	}
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("replay: create output: %w", err)
	}
	defer f.Close()
	if err := replay.Render(model, f); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s (%d hands)\n", outPath, len(model.Hands))
	return nil
}
