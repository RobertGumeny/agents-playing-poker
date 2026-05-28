package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/RobertGumeny/agent-poker/internal/eval"
	"github.com/RobertGumeny/agent-poker/internal/evalrun"
	"github.com/RobertGumeny/agent-poker/internal/experiment"
)

const defaultThinkingLevel = "low"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand (supported: experiment)")
	}
	switch args[0] {
	case "experiment":
		return runExperiment(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unsupported subcommand %q", args[0])
	}
}

func runExperiment(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand (supported: analyze, go, run, status)")
	}
	switch args[0] {
	case "analyze":
		return runExperimentAnalyze(args[1:], stdout, stderr)
	case "go":
		return runExperimentGo(args[1:], stdout, stderr)
	case "run":
		return runExperimentRun(args[1:], stdout, stderr)
	case "status":
		return runExperimentStatus(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unsupported experiment subcommand %q", args[0])
	}
}

type experimentFlags struct {
	id             string
	experimentsDir string
	sessionsDir    string
	experimentPath string // resolved from id
}

func parseExperimentFlags(fs *flag.FlagSet, args []string) (experimentFlags, error) {
	var ef experimentFlags
	fs.StringVar(&ef.experimentsDir, "experiments-dir", "experiments", "directory containing experiment definition JSON files")
	fs.StringVar(&ef.sessionsDir, "sessions-dir", "sessions", "session output root directory")
	fs.StringVar(&ef.experimentPath, "experiment", "", "explicit path to experiment definition JSON (overrides positional id)")

	if err := fs.Parse(args); err != nil {
		return experimentFlags{}, err
	}

	positional := fs.Args()
	if strings.TrimSpace(ef.experimentPath) == "" {
		if len(positional) == 0 {
			return experimentFlags{}, fmt.Errorf("experiment id is required as a positional argument")
		}
		ef.id = positional[0]
		resolved, err := resolveExperiment(ef.id, ef.experimentsDir)
		if err != nil {
			return experimentFlags{}, err
		}
		ef.experimentPath = resolved
	}
	return ef, nil
}

func resolveExperiment(id, experimentsDir string) (string, error) {
	candidate := filepath.Join(experimentsDir, id+".json")
	if _, err := os.Stat(candidate); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("experiment %q not found at %s", id, candidate)
		}
		return "", fmt.Errorf("stat experiment %q: %w", id, err)
	}
	return candidate, nil
}

func runExperimentStatus(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("poker experiment status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	ef, err := parseExperimentFlags(fs, args)
	if err != nil {
		return err
	}

	coverage, err := evalrun.LoadPlanCoverage(ef.experimentPath, ef.sessionsDir, experiment.Load, evalrun.InspectSession)
	if err != nil {
		return err
	}

	printCoverage(stdout, coverage)

	next := nextStep(coverage)
	_, _ = fmt.Fprintf(stdout, "next=%s\n", next)
	return nil
}

func runExperimentRun(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("poker experiment run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var model string
	var thinkingLevel string
	fs.StringVar(&model, "model", "", "optional PI_POKER_MODEL for Pi agents")
	fs.StringVar(&thinkingLevel, "thinking-level", defaultThinkingLevel, "PI_POKER_THINKING_LEVEL for Pi agents")

	ef, err := parseExperimentFlags(fs, args)
	if err != nil {
		return err
	}

	return execRun(ef, model, thinkingLevel, stdout, stderr)
}

func runExperimentAnalyze(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("poker experiment analyze", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	ef, err := parseExperimentFlags(fs, args)
	if err != nil {
		return err
	}

	return execAnalyze(ef, stdout)
}

func runExperimentGo(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("poker experiment go", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var model string
	var thinkingLevel string
	fs.StringVar(&model, "model", "", "optional PI_POKER_MODEL for Pi agents")
	fs.StringVar(&thinkingLevel, "thinking-level", defaultThinkingLevel, "PI_POKER_THINKING_LEVEL for Pi agents")

	ef, err := parseExperimentFlags(fs, args)
	if err != nil {
		return err
	}

	if err := execRun(ef, model, thinkingLevel, stdout, stderr); err != nil {
		return err
	}
	return execAnalyze(ef, stdout)
}

func execRun(ef experimentFlags, model, thinkingLevel string, stdout, stderr io.Writer) error {
	coverage, err := evalrun.LoadPlanCoverage(ef.experimentPath, ef.sessionsDir, experiment.Load, evalrun.InspectSession)
	if err != nil {
		return err
	}

	printCoverage(stdout, coverage)

	repoDir, err := repoRoot()
	if err != nil {
		return err
	}
	executor := evalrun.NewExecutor(repoDir, stdout, stderr)

	effectiveModel := model
	if effectiveModel == "" {
		effectiveModel = coverage.Plan.Model
	}

	var toRun []evalrun.SessionCoverage
	for _, session := range coverage.Sessions {
		if session.Inspection.Status == "present" {
			_, _ = fmt.Fprintf(stdout, "skip session_id=%s reason=present\n", session.Planned.SessionID)
			continue
		}
		if strings.TrimSpace(session.Planned.Opponent) == "" {
			return fmt.Errorf("session %q cannot run without opponent metadata in experiment definition", session.Planned.SessionID)
		}
		toRun = append(toRun, session)
	}

	if len(toRun) == 0 {
		return collectMissing(coverage, stdout)
	}

	// build the binary once before spawning parallel sessions
	if _, err := executor.PrepareBinary(); err != nil {
		return err
	}

	ids := make([]string, len(toRun))
	for i, s := range toRun {
		ids[i] = s.Planned.SessionID
	}
	disp := newRunDisplay(stdout, ids)
	disp.init()

	var wg sync.WaitGroup
	errc := make(chan error, len(toRun))

	for _, session := range toRun {
		wg.Add(1)
		go func(s evalrun.SessionCoverage) {
			defer wg.Done()
			pw := &handProgressWriter{sessionID: s.Planned.SessionID, disp: disp}
			runErr := executor.Execute(context.Background(), evalrun.ExecuteConfig{
				Agent0:        s.Planned.Agent,
				Agent1:        s.Planned.Opponent,
				Hands:         coverage.Plan.HandsPerSession,
				Seed:          s.Planned.Seed,
				SessionID:     s.Planned.SessionID,
				SessionsDir:   ef.sessionsDir,
				Model:         effectiveModel,
				ThinkingLevel: thinkingLevel,
				Stdout:        pw,
				Stderr:        stderr,
			})
			if runErr != nil {
				disp.setError(s.Planned.SessionID, runErr.Error())
				errc <- fmt.Errorf("run session %q: %w", s.Planned.SessionID, runErr)
				return
			}
			disp.setDone(s.Planned.SessionID)
		}(session)
	}

	wg.Wait()
	close(errc)
	_, _ = fmt.Fprintln(stdout)

	var firstErr error
	for e := range errc {
		if firstErr == nil {
			firstErr = e
		}
	}
	if firstErr != nil {
		return firstErr
	}

	return collectMissing(coverage, stdout)
}

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

func execAnalyze(ef experimentFlags, stdout io.Writer) error {
	// re-load coverage to pick up newly completed sessions
	coverage, err := evalrun.LoadPlanCoverage(ef.experimentPath, ef.sessionsDir, experiment.Load, evalrun.InspectSession)
	if err != nil {
		return err
	}

	if err := collectMissing(coverage, stdout); err != nil {
		return err
	}

	def, err := experiment.Load(ef.experimentPath)
	if err != nil {
		return err
	}

	report, err := eval.Compare(def, ef.sessionsDir)
	if err != nil {
		return err
	}

	reportDir := "reports"
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", reportDir, err)
	}

	reportID := def.ID
	if reportID == "" {
		reportID = strings.TrimSuffix(filepath.Base(ef.experimentPath), ".json")
	}
	reportPath := filepath.Join(reportDir, reportID+".md")
	if err := os.WriteFile(reportPath, []byte(eval.RenderComparisonMarkdown(report)), 0o644); err != nil {
		return fmt.Errorf("write report %s: %w", reportPath, err)
	}

	_, _ = fmt.Fprintf(stdout, "report=%s\n", reportPath)
	return nil
}

func collectMissing(coverage evalrun.PlanCoverage, stdout io.Writer) error {
	for _, session := range coverage.Sessions {
		if session.Inspection.Status != "present" {
			continue
		}
		evalPath := filepath.Join(session.Planned.SessionDir, "eval.json")
		if _, err := os.Stat(evalPath); err == nil {
			continue // already collected
		}
		summary, err := eval.CollectSession(session.Planned.SessionDir)
		if err != nil {
			return fmt.Errorf("collect session %q: %w", session.Planned.SessionID, err)
		}
		if err := eval.WriteSummary(session.Planned.SessionDir, summary); err != nil {
			return fmt.Errorf("write eval.json for session %q: %w", session.Planned.SessionID, err)
		}
		_, _ = fmt.Fprintf(stdout, "collected session_id=%s output=%s\n", summary.SessionID, evalPath)
	}
	return nil
}

func printCoverage(stdout io.Writer, coverage evalrun.PlanCoverage) {
	_, _ = fmt.Fprintf(stdout, "experiment=%s planned=%d present=%d missing=%d incomplete=%d\n", coverage.Plan.ExperimentID, len(coverage.Sessions), coverage.Present, coverage.Missing, coverage.Incomplete)
	for _, label := range []string{"control", "treatment"} {
		summary := coverage.GroupSummaries()[label]
		_, _ = fmt.Fprintf(stdout, "group_summary group=%s planned=%d present=%d missing=%d incomplete=%d\n", label, summary.Planned, summary.Present, summary.Missing, summary.Incomplete)
	}
}

func nextStep(coverage evalrun.PlanCoverage) string {
	if coverage.Missing > 0 || coverage.Incomplete > 0 {
		return "run"
	}
	for _, session := range coverage.Sessions {
		evalPath := filepath.Join(session.Planned.SessionDir, "eval.json")
		if _, err := os.Stat(evalPath); os.IsNotExist(err) {
			return "analyze"
		}
	}
	return "analyze"
}

func repoRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..")), nil
}
