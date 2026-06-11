package eval

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseVillainBoard(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantVillain []string
		wantBoard   []string
	}{
		{
			name:        "villain and board fields (board on its own line)",
			text:        "Villain: Jc Jd\nBoard: Ah Kd 2c",
			wantVillain: []string{"Jc", "Jd"},
			wantBoard:   []string{"Ah", "Kd", "2c"},
		},
		{
			name:        "board mid-line is not matched (line-anchored rule)",
			text:        "Villain: Jc Jd. Board: Ah Kd 2c",
			wantVillain: []string{"Jc", "Jd"},
			wantBoard:   nil,
		},
		{
			name:        "markdown bold board at line start, five cards",
			text:        "**Villain:** As Ks\n**Board:** Ah Kd 2c Ts 3h 9d",
			wantVillain: []string{"As", "Ks"},
			wantBoard:   []string{"Ah", "Kd", "2c", "Ts", "3h"},
		},
		{
			name:        "inline-prose board does not match (line-anchored rule)",
			text:        "Villain: As Ks. It was a paired board, like hand 3's board: 7h 7d.",
			wantVillain: []string{"As", "Ks"},
			wantBoard:   nil,
		},
		{
			name:        "ten normalizes to T",
			text:        "Villain: 10s Jd",
			wantVillain: []string{"Jd", "Ts"},
			wantBoard:   nil,
		},
		{
			name:        "no structured fields",
			text:        "folded preflop, nothing notable",
			wantVillain: nil,
			wantBoard:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			villain, board := parseVillainBoard(tt.text)
			if !reflect.DeepEqual(villain, tt.wantVillain) {
				t.Errorf("villain = %v, want %v", villain, tt.wantVillain)
			}
			if !reflect.DeepEqual(board, tt.wantBoard) {
				t.Errorf("board = %v, want %v", board, tt.wantBoard)
			}
		})
	}
}

func TestVerifyClaims(t *testing.T) {
	gt := map[int]handTruth{
		1: {board: []string{"Ah", "Kd", "2c"}, hole: map[int][]string{0: {"Qs", "Qh"}, 1: {"Jc", "Jd"}}, showdown: true},
		2: {board: []string{"5h", "6d", "7c"}, hole: map[int][]string{0: {"2s", "2h"}, 1: {"3c", "3d"}}, showdown: false},
	}
	const villainSeat = 1
	claims := []fidelityClaim{
		{hand: 1, villain: []string{"Jc", "Jd"}, board: []string{"Ah", "Kd", "2c"}}, // OK
		{hand: 1, villain: []string{"As", "Ks"}},                                    // card error (showdown mismatch)
		{hand: 2, villain: []string{"3c", "3d"}},                                    // fabrication (non-showdown)
		{hand: 1, board: []string{"9h"}},                                            // board error (never dealt)
		{hand: 99, board: []string{"Ah"}},                                           // no such hand -> board error
		{hand: 1},                                                                   // missing claim -> nothing checked
	}
	rows := verifyClaims(claims, gt, villainSeat)
	a := aggFidelity(rows)

	if a.Records != 6 {
		t.Errorf("records = %d, want 6", a.Records)
	}
	if a.CardClaims != 3 {
		t.Errorf("card_claims = %d, want 3", a.CardClaims)
	}
	if a.Fabrications != 1 {
		t.Errorf("fabrications = %d, want 1", a.Fabrications)
	}
	if a.CardErrors != 1 {
		t.Errorf("card_errors = %d, want 1", a.CardErrors)
	}
	if a.BoardErrors != 2 {
		t.Errorf("board_errors = %d, want 2 (one stray, one no-such-hand)", a.BoardErrors)
	}

	// The clean claim must be entirely clean.
	ok := rows[0]
	if ok.Fabrication || ok.CardError || ok.BoardError || !ok.CheckedCards || !ok.CheckedBoard {
		t.Errorf("first claim should be a clean OK record, got %+v", ok)
	}
}

func TestExtractAKGClaims(t *testing.T) {
	export := &MemoryExport{Nodes: []MemoryExportNode{
		{Type: "hand", ID: "hand-1", Body: "Villain: Jc Jd. Board: Ah Kd 2c"},
		{Type: "opponent", ID: "villain", Body: "ignored, not a hand node"},
		{Type: "hand", ID: "hand-2", Body: "folded preflop"},
	}}
	claims := extractAKGClaims(export)
	if len(claims) != 2 {
		t.Fatalf("claims = %d, want 2 (only hand nodes)", len(claims))
	}
	if claims[0].hand != 1 || !reflect.DeepEqual(claims[0].villain, []string{"Jc", "Jd"}) {
		t.Errorf("claim 0 = %+v, want hand 1 villain Jc Jd", claims[0])
	}
	if claims[1].hand != 2 || claims[1].villain != nil {
		t.Errorf("claim 1 = %+v, want hand 2 no villain", claims[1])
	}
}

func TestExtractWikiClaims(t *testing.T) {
	dir := t.TempDir()
	handsDir := filepath.Join(dir, "wiki", "hands")
	if err := os.MkdirAll(handsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(handsDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("hand-1.md", "hand: 1\n\n**Villain:** Jc Jd\n**Board:** Ah Kd 2c\n")
	write("hand-2.md", "hand: 2\n\nfolded preflop\n")
	write("notes.txt", "not markdown, ignored")

	claims := extractWikiClaims(dir)
	if len(claims) != 2 {
		t.Fatalf("claims = %d, want 2", len(claims))
	}
	byHand := map[int]fidelityClaim{}
	for _, c := range claims {
		byHand[c.hand] = c
	}
	if c := byHand[1]; !reflect.DeepEqual(c.villain, []string{"Jc", "Jd"}) || !reflect.DeepEqual(c.board, []string{"Ah", "Kd", "2c"}) {
		t.Errorf("hand 1 claim = %+v", c)
	}
}

func TestDriftBuckets(t *testing.T) {
	rows := []FidelityRow{
		{Hand: 10, CheckedCards: true, Fabrication: true},
		{Hand: 60, CheckedCards: true},
		{Hand: 150, CheckedCards: true, CardError: true},
	}
	buckets := driftBuckets(rows)
	if len(buckets) != 3 {
		t.Fatalf("buckets = %d, want 3", len(buckets))
	}
	if buckets[0].Label != "1-50" || buckets[0].Records != 1 || buckets[0].Fabrications != 1 {
		t.Errorf("bucket 0 = %+v", buckets[0])
	}
	if buckets[1].Label != "51-100" || buckets[1].Records != 1 {
		t.Errorf("bucket 1 = %+v", buckets[1])
	}
	if buckets[2].Label != "101+" || buckets[2].Records != 1 || buckets[2].CardErrors != 1 {
		t.Errorf("bucket 2 = %+v", buckets[2])
	}
}
