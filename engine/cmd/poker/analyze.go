package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// runAnalyze is the harness-neutral query CLI over session traces. It streams the
// (multi-MB) decision trace and returns only the relevant slice, so an agent never
// has to slurp the whole file into its context.
func runAnalyze(args []string, stdout, _ io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand (supported: hand, traces)")
	}
	switch args[0] {
	case "hand":
		return runAnalyzeHand(args[1:], stdout)
	case "traces":
		return runAnalyzeTraces(args[1:], stdout)
	default:
		return fmt.Errorf("unknown analyze subcommand %q (supported: hand, traces)", args[0])
	}
}

func runAnalyzeHand(args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: poker analyze hand <N> <session-dir|trace.jsonl>")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("hand number must be an integer: %q", args[0])
	}
	traces, err := resolveTraces(args[1])
	if err != nil {
		return err
	}
	// Real Pi decision prompts carry the hand as prose ("Hand: N"); scripted test
	// traces carry a structured "hand_number": N. Match either, word-bounded so
	// hand 2 does not match "Hand: 20".
	marker := regexp.MustCompile(fmt.Sprintf(`(?:"hand_number":\s*%d|Hand:\s*%d)\b`, n, n))
	for _, tr := range traces {
		fmt.Fprintf(stdout, "\n--- %s ---\n", tr.label)
		hits := streamContext(tr.path, marker.MatchString, 3, 8)
		if len(hits) == 0 {
			fmt.Fprintln(stdout, "  (hand not found)")
			continue
		}
		for _, h := range hits {
			fmt.Fprintf(stdout, "  L%d: %s\n", h.lineNo, truncate(h.text, 400))
		}
	}
	return nil
}

func runAnalyzeTraces(args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: poker analyze traces <keyword> <session-dir|trace.jsonl>")
	}
	keyword := strings.ToLower(args[0])
	traces, err := resolveTraces(args[1])
	if err != nil {
		return err
	}
	for _, tr := range traces {
		fmt.Fprintf(stdout, "\n--- %s ---\n", tr.label)
		mentions, total := streamReasoning(tr.path, keyword, stdout)
		rate := 0.0
		if total > 0 {
			rate = float64(mentions) / float64(total)
		}
		fmt.Fprintf(stdout, "  %d/%d assistant turns mention %q (%.1f%%)\n", mentions, total, args[0], rate*100)
	}
	return nil
}

type tracePath struct {
	label string
	path  string
}

// resolveTraces accepts either a single trace .jsonl file or a session directory.
// For a directory it returns each agent's decision trace, dual-reading the new
// (session-decisions.jsonl) and legacy (pi-session.jsonl) names.
func resolveTraces(arg string) ([]tracePath, error) {
	info, err := os.Stat(arg)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}
	if !info.IsDir() {
		return []tracePath{{label: filepath.Base(arg), path: arg}}, nil
	}
	agentsDir := filepath.Join(arg, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, fmt.Errorf("analyze: no agents/ under %q: %w", arg, err)
	}
	var traces []tracePath
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentDir := filepath.Join(agentsDir, entry.Name())
		for _, name := range []string{"session-decisions.jsonl", "pi-session.jsonl"} {
			candidate := filepath.Join(agentDir, name)
			if _, err := os.Stat(candidate); err == nil {
				traces = append(traces, tracePath{label: entry.Name(), path: candidate})
				break
			}
		}
	}
	if len(traces) == 0 {
		return nil, fmt.Errorf("analyze: no decision traces found under %q", agentsDir)
	}
	return traces, nil
}

type contextHit struct {
	lineNo int
	text   string
}

// streamContext scans a JSONL file once and returns matching lines plus a window
// of before/after lines around each match, without loading the whole file.
func streamContext(path string, match func(string) bool, before, after int) []contextHit {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	ring := make([]string, 0, before)
	ringNo := make([]int, 0, before)
	var hits []contextHit
	remainingAfter := 0
	lineNo := 0
	scanner := newJSONLScanner(f)
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		switch {
		case match(line):
			for i, buffered := range ring {
				hits = append(hits, contextHit{lineNo: ringNo[i], text: buffered})
			}
			ring = ring[:0]
			ringNo = ringNo[:0]
			hits = append(hits, contextHit{lineNo: lineNo, text: line})
			remainingAfter = after
		case remainingAfter > 0:
			hits = append(hits, contextHit{lineNo: lineNo, text: line})
			remainingAfter--
		default:
			ring = appendCapped(ring, line, before)
			ringNo = appendCapped(ringNo, lineNo, before)
		}
	}
	return hits
}

// streamReasoning prints assistant turns whose joined text mentions keyword and
// returns (mentions, totalAssistantTurns).
func streamReasoning(path, keyword string, stdout io.Writer) (mentions, total int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	lineNo := 0
	scanner := newJSONLScanner(f)
	for scanner.Scan() {
		lineNo++
		var ev struct {
			Type    string `json:"type"`
			Message *struct {
				Role    string `json:"role"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Message == nil || ev.Message.Role != "assistant" {
			continue
		}
		total++
		var sb strings.Builder
		for _, c := range ev.Message.Content {
			sb.WriteString(c.Text)
			sb.WriteString(" ")
		}
		if strings.Contains(strings.ToLower(sb.String()), keyword) {
			mentions++
			fmt.Fprintf(stdout, "  L%d: %s\n", lineNo, truncate(strings.TrimSpace(sb.String()), 300))
		}
	}
	return mentions, total
}

func newJSONLScanner(f *os.File) *bufio.Scanner {
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return scanner
}

func appendCapped[T any](buf []T, v T, cap int) []T {
	if cap == 0 {
		return buf
	}
	if len(buf) >= cap {
		buf = buf[1:]
	}
	return append(buf, v)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
