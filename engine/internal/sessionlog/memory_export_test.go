package sessionlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	akg "github.com/RobertGumeny/akg/sdk/akg-go"
)

func TestWriteMemoryExportWritesGraphSnapshot(t *testing.T) {
	t.Parallel()

	agentDir := t.TempDir()
	store, err := akg.Open(filepath.Join(agentDir, memoryFileName))
	if err != nil {
		t.Fatalf("Open(memory.akg) error = %v", err)
	}

	opponent, err := store.PutNode("opponent", "villain", akg.NodeFields{
		Title: "villain",
		Body:  "Villain profile.",
		Meta:  map[string]any{"hands_played": 3, "vpip": 2},
	}, []string{"opponent"})
	if err != nil {
		t.Fatalf("PutNode(opponent) error = %v", err)
	}
	hand, err := store.PutNode("hand", "hand-1", akg.NodeFields{
		Title: "Hand 1",
		Body:  "Hand summary.",
		Meta:  map[string]any{"hand_number": 1},
	}, []string{"hand", "showdown"})
	if err != nil {
		t.Fatalf("PutNode(hand) error = %v", err)
	}
	if err := store.PutEdge(opponent, "supported_by", hand, akg.EdgeFields{Meta: map[string]any{"count": 1}}); err != nil {
		t.Fatalf("PutEdge error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close(memory.akg) error = %v", err)
	}

	if err := WriteMemoryExport(agentDir); err != nil {
		t.Fatalf("WriteMemoryExport() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(agentDir, memoryExportFileName))
	if err != nil {
		t.Fatalf("ReadFile(memory-export.json) error = %v", err)
	}
	var export memoryExport
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("Unmarshal(memory-export.json) error = %v", err)
	}

	if len(export.Nodes) != 2 {
		t.Fatalf("len(export.Nodes) = %d, want 2", len(export.Nodes))
	}
	if len(export.Edges) != 1 {
		t.Fatalf("len(export.Edges) = %d, want 1", len(export.Edges))
	}
	if export.Nodes[0].Type != "hand" || export.Nodes[0].ID != "hand-1" {
		t.Fatalf("export.Nodes[0] = %+v, want hand/hand-1", export.Nodes[0])
	}
	if export.Nodes[1].Type != "opponent" || export.Nodes[1].ID != "villain" {
		t.Fatalf("export.Nodes[1] = %+v, want opponent/villain", export.Nodes[1])
	}
	if got := export.Nodes[0].Meta["hand_number"]; got != float64(1) {
		t.Fatalf("hand node meta hand_number = %#v, want 1", got)
	}
	if got := export.Nodes[1].Meta["hands_played"]; got != float64(3) {
		t.Fatalf("opponent node meta hands_played = %#v, want 3", got)
	}
	if export.Edges[0].From != opponent || export.Edges[0].To != hand || export.Edges[0].Relation != "supported_by" {
		t.Fatalf("export.Edges[0] = %+v, want %v -[supported_by]-> %v", export.Edges[0], opponent, hand)
	}
	if got := export.Edges[0].Meta["count"]; got != float64(1) {
		t.Fatalf("edge meta count = %#v, want 1", got)
	}
}

func TestWriteMemoryExportWithoutMemoryFileIsNoOp(t *testing.T) {
	t.Parallel()

	agentDir := t.TempDir()
	if err := WriteMemoryExport(agentDir); err != nil {
		t.Fatalf("WriteMemoryExport() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(agentDir, memoryExportFileName)); !os.IsNotExist(err) {
		t.Fatalf("memory-export.json stat error = %v, want not exists", err)
	}
}

func TestWriteMemoryExportReturnsErrorForUnreadableMemoryFile(t *testing.T) {
	t.Parallel()

	agentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(agentDir, memoryFileName), []byte("not-akg"), 0o644); err != nil {
		t.Fatalf("WriteFile(memory.akg) error = %v", err)
	}

	if err := WriteMemoryExport(agentDir); err == nil {
		t.Fatal("WriteMemoryExport() error = nil, want error")
	}
	if _, err := os.Stat(filepath.Join(agentDir, memoryExportFileName)); !os.IsNotExist(err) {
		t.Fatalf("memory-export.json stat error = %v, want not exists", err)
	}
}
