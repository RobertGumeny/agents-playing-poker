package sessionlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	akg "github.com/RobertGumeny/akg/sdk/akg-go"
)

const (
	memoryFileName       = "memory.akg"
	memoryExportFileName = "memory-export.json"
)

type memoryExport struct {
	Nodes []memoryExportNode `json:"nodes"`
	Edges []memoryExportEdge `json:"edges"`
}

type memoryExportNode struct {
	Type  string         `json:"type"`
	ID    string         `json:"id"`
	Title string         `json:"title"`
	Body  string         `json:"body"`
	Meta  map[string]any `json:"meta"`
	Tags  []string       `json:"tags"`
}

type memoryExportEdge struct {
	From     akg.NodeRef    `json:"from"`
	Relation string         `json:"relation"`
	To       akg.NodeRef    `json:"to"`
	Meta     map[string]any `json:"meta"`
}

func WriteMemoryExport(agentDir string) error {
	memoryPath := filepath.Join(agentDir, memoryFileName)
	if _, err := os.Stat(memoryPath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("write memory export: stat memory.akg: %w", err)
	}

	store, err := akg.Open(memoryPath)
	if err != nil {
		return fmt.Errorf("write memory export: open memory.akg: %w", err)
	}
	defer store.Close()

	nodes, err := store.ListNodes("")
	if err != nil {
		return fmt.Errorf("write memory export: list nodes: %w", err)
	}

	export := memoryExport{
		Nodes: make([]memoryExportNode, 0, len(nodes)),
		Edges: make([]memoryExportEdge, 0),
	}
	for _, node := range nodes {
		export.Nodes = append(export.Nodes, memoryExportNode{
			Type:  node.Type,
			ID:    node.ID,
			Title: node.Title,
			Body:  node.Body,
			Meta:  normalizeMeta(node.Meta),
			Tags:  normalizeTags(node.Tags),
		})

		edges, err := store.OutboundEdges(akg.NodeRef{Type: node.Type, ID: node.ID}, "")
		if err != nil {
			return fmt.Errorf("write memory export: list outbound edges for %s/%s: %w", node.Type, node.ID, err)
		}
		for _, edge := range edges {
			export.Edges = append(export.Edges, memoryExportEdge{
				From:     edge.From,
				Relation: edge.Relation,
				To:       edge.To,
				Meta:     normalizeMeta(edge.Meta),
			})
		}
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("write memory export: marshal: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(agentDir, memoryExportFileName), data, 0o644); err != nil {
		return fmt.Errorf("write memory export: write memory-export.json: %w", err)
	}
	return nil
}

func normalizeMeta(meta map[string]any) map[string]any {
	if meta == nil {
		return map[string]any{}
	}
	clone := make(map[string]any, len(meta))
	for key, value := range meta {
		clone[key] = value
	}
	return clone
}

func normalizeTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return append([]string(nil), tags...)
}
