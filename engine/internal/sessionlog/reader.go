package sessionlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func ReadManifest(sessionDir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(sessionDir, "manifest.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	return m, nil
}

func ReadHands(sessionDir string) ([]HandRecord, error) {
	f, err := os.Open(filepath.Join(sessionDir, "hands.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("read hands: %w", err)
	}
	defer f.Close()

	var hands []HandRecord
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		var h HandRecord
		if err := json.Unmarshal(scanner.Bytes(), &h); err != nil {
			return nil, fmt.Errorf("read hands: line %d: %w", len(hands)+1, err)
		}
		hands = append(hands, h)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read hands: scan: %w", err)
	}
	return hands, nil
}
