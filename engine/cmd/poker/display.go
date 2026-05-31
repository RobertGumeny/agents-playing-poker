package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
)

// runDisplay manages a fixed block of per-session rows updated in place.
type runDisplay struct {
	mu   sync.Mutex
	out  io.Writer
	rows []displayRow
}

type displayRow struct {
	id      string
	hand    int
	total   int
	a0name  string
	a0total int
	a1name  string
	a1total int
	done    bool
	errMsg  string
}

func newRunDisplay(out io.Writer, sessionIDs []string) *runDisplay {
	rows := make([]displayRow, len(sessionIDs))
	for i, id := range sessionIDs {
		rows[i] = displayRow{id: id}
	}
	return &runDisplay{out: out, rows: rows}
}

func (d *runDisplay) init() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, row := range d.rows {
		_, _ = fmt.Fprintf(d.out, "%s\n", formatDisplayRow(row))
	}
}

func (d *runDisplay) update(sessionID string, hand, total int, a0name string, a0total int, a1name string, a1total int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.rows {
		if d.rows[i].id == sessionID {
			d.rows[i].hand = hand
			d.rows[i].total = total
			d.rows[i].a0name = a0name
			d.rows[i].a0total = a0total
			d.rows[i].a1name = a1name
			d.rows[i].a1total = a1total
			break
		}
	}
	d.redraw()
}

func (d *runDisplay) setDone(sessionID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.rows {
		if d.rows[i].id == sessionID {
			d.rows[i].done = true
			break
		}
	}
	d.redraw()
}

func (d *runDisplay) setError(sessionID string, msg string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.rows {
		if d.rows[i].id == sessionID {
			d.rows[i].errMsg = msg
			d.rows[i].done = true
			break
		}
	}
	d.redraw()
}

// redraw must be called with d.mu held.
func (d *runDisplay) redraw() {
	_, _ = fmt.Fprintf(d.out, "\033[%dA", len(d.rows))
	for _, row := range d.rows {
		_, _ = fmt.Fprintf(d.out, "\r\033[K%s\n", formatDisplayRow(row))
	}
}

func formatDisplayRow(r displayRow) string {
	if r.errMsg != "" {
		return fmt.Sprintf("%-36s error: %s", r.id, r.errMsg)
	}
	if r.total == 0 {
		return fmt.Sprintf("%-36s starting...", r.id)
	}
	status := "running"
	if r.done {
		status = "done   "
	}
	return fmt.Sprintf("%-36s hand %3d/%d [%s] | %s: %+d | %s: %+d",
		r.id, r.hand, r.total, status, r.a0name, r.a0total, r.a1name, r.a1total)
}

// handProgressWriter parses poker-run stdout progress lines and feeds the display.
type handProgressWriter struct {
	sessionID string
	disp      *runDisplay
	buf       []byte
}

func (w *handProgressWriter) Write(b []byte) (int, error) {
	w.buf = append(w.buf, b...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		if hand, total, a0name, a0total, a1name, a1total, ok := parseHandProgressLine(line); ok {
			w.disp.update(w.sessionID, hand, total, a0name, a0total, a1name, a1total)
		}
	}
	return len(b), nil
}

// parseHandProgressLine parses: "hand  42/100 | llm-akg-durable +6 (total: +52) | llm-stateless -6 (total: -52)"
func parseHandProgressLine(line string) (hand, total int, a0name string, a0total int, a1name string, a1total int, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(line), " | ", 3)
	if len(parts) != 3 {
		return
	}
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "hand %d/%d", &hand, &total); err != nil {
		return
	}
	a0name, a0total, ok = parseAgentProgressPart(parts[1])
	if !ok {
		return
	}
	a1name, a1total, ok = parseAgentProgressPart(parts[2])
	return
}

// parseAgentProgressPart parses: "llm-akg-durable +6 (total: +52)"
func parseAgentProgressPart(s string) (name string, total int, ok bool) {
	idx := strings.Index(s, "(total: ")
	if idx < 0 {
		return
	}
	totalStr := strings.TrimSuffix(strings.TrimSpace(s[idx+8:]), ")")
	if _, err := fmt.Sscanf(totalStr, "%d", &total); err != nil {
		return
	}
	nameAndDelta := strings.TrimSpace(s[:idx])
	lastSpace := strings.LastIndex(nameAndDelta, " ")
	if lastSpace < 0 {
		return
	}
	return strings.TrimSpace(nameAndDelta[:lastSpace]), total, true
}
