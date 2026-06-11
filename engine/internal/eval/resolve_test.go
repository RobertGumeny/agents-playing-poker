package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAgentArtifactPrefersCurrentName(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Neither present.
	if got := resolveAgentArtifact(dir, piSessionFileName, legacyPiSessionFileName); got != "" {
		t.Errorf("no files: got %q, want empty", got)
	}

	// Legacy only (archived session) still resolves.
	write(legacyPiSessionFileName)
	if got := resolveAgentArtifact(dir, piSessionFileName, legacyPiSessionFileName); got != filepath.Join(dir, legacyPiSessionFileName) {
		t.Errorf("legacy only: got %q, want legacy path", got)
	}

	// When both exist, the current name wins.
	write(piSessionFileName)
	if got := resolveAgentArtifact(dir, piSessionFileName, legacyPiSessionFileName); got != filepath.Join(dir, piSessionFileName) {
		t.Errorf("both present: got %q, want current path", got)
	}
}
