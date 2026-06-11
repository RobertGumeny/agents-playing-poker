package eval

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/RobertGumeny/agent-poker/internal/sessionlog"
)

// Profile-fidelity cross-validation (port of fidelity.py). Fully deterministic,
// no LLM judging: each memory agent's structured per-hand opponent records are
// cross-validated against hands.jsonl ground truth. The extractor is shared
// across substrates (regex over prose) on purpose — giving one substrate
// structured fields would bias the comparison.

const cardPattern = `(?:10|[2-9TJQKAtjqka])[shdcSHDC]`

var (
	cardRE     = regexp.MustCompile(cardPattern)
	villainRE  = regexp.MustCompile(`[Vv]illain[:* ]+\s*(` + cardPattern + `)\s*[, ]?\s*(` + cardPattern + `)`)
	boardRE    = regexp.MustCompile(`(?m)^[\s*_>#-]*[Bb]oard[:*\s]+(` + cardPattern + `(?:[ ,]+` + cardPattern + `)*)`)
	nodeIDRE   = regexp.MustCompile(`(\d+)`)
	wikiHandRE = regexp.MustCompile(`hand:\s*(\d+)`)
	wikiFileRE = regexp.MustCompile(`hand-(\d+)`)
)

// FidelityRow is one cross-validated per-hand claim. Stored in eval.json so the
// experiment report can aggregate across sessions without re-reading raw memory.
type FidelityRow struct {
	Hand         int  `json:"hand"`
	Fabrication  bool `json:"fabrication"`
	CardError    bool `json:"card_error"`
	BoardError   bool `json:"board_error"`
	CheckedCards bool `json:"checked_cards"`
	CheckedBoard bool `json:"checked_board"`
}

type handTruth struct {
	board    []string
	hole     map[int][]string
	showdown bool
}

type fidelityClaim struct {
	hand    int
	villain []string // nil when no villain holding stated
	board   []string // nil when no board stated
}

func normCard(c string) string {
	c = strings.ReplaceAll(c, "10", "T")
	if len(c) < 2 {
		return c
	}
	return strings.ToUpper(c[:1]) + strings.ToLower(c[1:])
}

func cardsIn(text string) []string {
	matches := cardRE.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, normCard(m))
	}
	return out
}

func groundTruthFromHands(hands []sessionlog.HandRecord) map[int]handTruth {
	gt := map[int]handTruth{}
	for _, h := range hands {
		board := make([]string, 0, len(h.Board))
		for _, c := range h.Board {
			board = append(board, normCard(c))
		}
		hole := map[int][]string{}
		for seat, cards := range h.HoleCards {
			norm := make([]string, 0, len(cards))
			for _, c := range cards {
				norm = append(norm, normCard(c))
			}
			hole[seat] = norm
		}
		gt[h.HandNumber] = handTruth{board: board, hole: hole, showdown: h.ShowdownReached}
	}
	return gt
}

func extractAKGClaims(export *MemoryExport) []fidelityClaim {
	if export == nil {
		return nil
	}
	var claims []fidelityClaim
	for _, node := range export.Nodes {
		if node.Type != "hand" {
			continue
		}
		m := nodeIDRE.FindStringSubmatch(node.ID)
		if m == nil {
			continue
		}
		hn, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		villain, board := parseVillainBoard(node.Body)
		claims = append(claims, fidelityClaim{hand: hn, villain: villain, board: board})
	}
	return claims
}

func extractWikiClaims(agentDir string) []fidelityClaim {
	handsDir := filepath.Join(agentDir, "wiki", "hands")
	entries, err := os.ReadDir(handsDir)
	if err != nil {
		return nil
	}
	var claims []fidelityClaim
	for _, entry := range entries {
		fn := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(fn, ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(handsDir, fn))
		if err != nil {
			continue
		}
		text := string(data)
		var hn int
		if m := wikiHandRE.FindStringSubmatch(text); m != nil {
			hn, _ = strconv.Atoi(m[1])
		} else if m := wikiFileRE.FindStringSubmatch(fn); m != nil {
			hn, _ = strconv.Atoi(m[1])
		} else {
			continue
		}
		villain, board := parseVillainBoard(text)
		claims = append(claims, fidelityClaim{hand: hn, villain: villain, board: board})
	}
	return claims
}

// parseVillainBoard pulls the villain holding and board off a structured record.
// The board match must START a line (after optional markdown bullet/bold) so
// inline prose ("paired board", or a citation of another hand) cannot mis-attribute
// cards to this record.
func parseVillainBoard(text string) (villain, board []string) {
	if m := villainRE.FindStringSubmatch(text); m != nil {
		villain = []string{normCard(m[1]), normCard(m[2])}
		sort.Strings(villain)
	}
	if m := boardRE.FindStringSubmatch(text); m != nil {
		bc := cardsIn(m[1])
		if len(bc) > 0 {
			if len(bc) > 5 {
				bc = bc[:5]
			}
			board = bc
		}
	}
	return villain, board
}

func verifyClaims(claims []fidelityClaim, gt map[int]handTruth, villainSeat int) []FidelityRow {
	rows := make([]FidelityRow, 0, len(claims))
	for _, c := range claims {
		rec := FidelityRow{Hand: c.hand}
		g, ok := gt[c.hand]
		if !ok {
			rec.BoardError = len(c.board) > 0 // references a hand that never happened
			rows = append(rows, rec)
			continue
		}
		if len(c.villain) > 0 {
			rec.CheckedCards = true
			actual := append([]string(nil), g.hole[villainSeat]...)
			sort.Strings(actual)
			if !g.showdown {
				rec.Fabrication = true
			} else if !equalStrings(c.villain, actual) {
				rec.CardError = true
			}
		}
		if len(c.board) > 0 {
			rec.CheckedBoard = true
			if hasStray(c.board, g.board) {
				rec.BoardError = true
			}
		}
		rows = append(rows, rec)
	}
	return rows
}

// FidelityAgg is the per-agent aggregate rendered in the report.
type FidelityAgg struct {
	Records      int
	CardClaims   int
	Fabrications int
	CardErrors   int
	BoardErrors  int
}

func aggFidelity(rows []FidelityRow) FidelityAgg {
	a := FidelityAgg{Records: len(rows)}
	for _, r := range rows {
		if r.CheckedCards {
			a.CardClaims++
		}
		if r.Fabrication {
			a.Fabrications++
		}
		if r.CardError {
			a.CardErrors++
		}
		if r.BoardError {
			a.BoardErrors++
		}
	}
	return a
}

// FidelityBucket is a hand-number-range slice of the per-agent aggregate.
type FidelityBucket struct {
	Label string
	FidelityAgg
}

func driftBuckets(rows []FidelityRow) []FidelityBucket {
	type bound struct {
		lo, hi int
		label  string
	}
	bounds := []bound{{0, 50, "1-50"}, {50, 100, "51-100"}, {100, 1 << 30, "101+"}}
	buckets := make([]FidelityBucket, 0, len(bounds))
	for _, b := range bounds {
		var sub []FidelityRow
		for _, r := range rows {
			if r.Hand > b.lo && r.Hand <= b.hi {
				sub = append(sub, r)
			}
		}
		buckets = append(buckets, FidelityBucket{Label: b.label, FidelityAgg: aggFidelity(sub)})
	}
	return buckets
}

// fidelityExtractorFor returns the claim extractor for a given agent strategy, or
// nil for substrates with no machine-extractable structured records.
func fidelityRows(agent AgentArtifacts, gt map[int]handTruth) []FidelityRow {
	var claims []fidelityClaim
	switch agent.Seat.Name {
	case "llm-akg-durable":
		claims = extractAKGClaims(agent.MemoryExport)
	case "llm-md-wiki":
		claims = extractWikiClaims(agent.Dir)
	default:
		return nil
	}
	if claims == nil {
		return nil
	}
	return verifyClaims(claims, gt, 1-agent.Seat.Seat)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hasStray(claimed, dealt []string) bool {
	for _, c := range claimed {
		found := false
		for _, d := range dealt {
			if c == d {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}
	return false
}
