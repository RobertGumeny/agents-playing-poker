// Package replay turns a finished session's artifacts into a self-contained HTML
// poker-table replay. It is split across a clean view-model seam: gather.go reads
// artifacts into a ReplayModel, render.go templates a ReplayModel into HTML. The
// renderer depends only on ReplayModel — never on file readers — so a future live
// "Agent Poker League" can feed the same model from a stream instead of files.
package replay

// MemoryIndicator is the binary per-decision memory signal: whether the agent
// actively read its store before acting, or decided on already-available info.
type MemoryIndicator string

const (
	MemoryRecalling MemoryIndicator = "recalling"
	MemoryDeciding  MemoryIndicator = "deciding"
)

// ReplayModel is the entire view-model the renderer consumes. Everything is
// JSON-tagged because the model is marshaled and inlined into the output HTML.
type ReplayModel struct {
	Session SessionMeta `json:"session"`
	Hands   []HandView  `json:"hands"`
	Summary SummaryView `json:"summary"`
}

type SessionMeta struct {
	SessionID     string     `json:"sessionId"`
	Seed          int64      `json:"seed"`
	Blinds        BlindMeta  `json:"blinds"`
	StartingStack int        `json:"startingStack"`
	Seats         []SeatMeta `json:"seats"`
}

type BlindMeta struct {
	SB int `json:"sb"`
	BB int `json:"bb"`
}

type SeatMeta struct {
	Seat    int    `json:"seat"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type HandView struct {
	HandNumber      int              `json:"handNumber"`
	DealerSeat      int              `json:"dealerSeat"`
	StacksStart     map[int]int      `json:"stacksStart"`
	HoleCards       map[int][]string `json:"holeCards"`
	Board           []string         `json:"board"`
	ShowdownReached bool             `json:"showdownReached"`
	Results         map[int]int      `json:"results"`
	GrossPotSize    int              `json:"grossPotSize"`
	Actions         []ActionView     `json:"actions"`
}

type ActionView struct {
	Seat         int    `json:"seat"`
	Street       string `json:"street"`
	Action       string `json:"action"`
	Amount       *int   `json:"amount,omitempty"`
	ForcedReason string `json:"forcedReason,omitempty"`
	// Overlay is nil for blinds and any action whose trace decision could not be
	// aligned; the renderer simply shows no thinking panel in those cases.
	Overlay *DecisionOverlay `json:"overlay,omitempty"`

	// Running-state fields (reconstructed in gather, see computeStates).
	ChipsAdded int         `json:"chipsAdded"`      // chips this action put in the pot (0 for check/fold)
	Label      string      `json:"label"`           // poker-site read: "CALL", "BET 3", "RAISE to 8", "FOLD"...
	State      *TableState `json:"state,omitempty"` // table snapshot AFTER this action
}

// TableState is the reconstructed table after an action: the running pot and each
// seat's stack, current-street commitment, fold status, and last action label —
// everything the renderer needs to draw a live frame without replaying the log.
type TableState struct {
	Pot   int               `json:"pot"`
	Seats map[int]SeatState `json:"seats"`
}

type SeatState struct {
	Stack           int    `json:"stack"`
	StreetCommitted int    `json:"streetCommitted"`
	Folded          bool   `json:"folded"`
	LastAction      string `json:"lastAction"`
}

// DecisionOverlay carries the two orthogonal per-decision signals plus their
// supporting text. MemoryIndicator comes from toolCall presence; the thought
// bubble is shown iff ThinkingText != "" (a snap action has no bubble).
type DecisionOverlay struct {
	MemoryIndicator     MemoryIndicator `json:"memoryIndicator"`
	ThinkingText        string          `json:"thinkingText,omitempty"`
	RecalledFact        string          `json:"recalledFact,omitempty"`
	InjectedSummaryGist string          `json:"injectedSummaryGist,omitempty"`
}

// SummaryView is the closing-scoreboard "verdict": session-level metrics plus a
// per-seat breakdown. It is derived (eval.CollectSession), not raw — it lives in
// the model because the renderer renders it and depends only on the model.
type SummaryView struct {
	HandCount        int               `json:"handCount"`
	ShowdownRate     float64           `json:"showdownRate"`
	PreflopOnlyRate  float64           `json:"preflopOnlyRate"`
	BiggestSwingHand int               `json:"biggestSwingHand"`
	Seats            []SeatSummaryView `json:"seats"`
}

type SeatSummaryView struct {
	Seat       int    `json:"seat"`
	Name       string `json:"name"`
	ChipsDelta int    `json:"chipsDelta"`

	// memory engagement
	Decisions         int     `json:"decisions"`
	DecisionsWithRead int     `json:"decisionsWithRead"`
	ReadCoverage      float64 `json:"readCoverage"`
	TotalReads        int     `json:"totalReads"`

	// fidelity (aggregated across cross-validated per-hand claims)
	FidelityChecked int `json:"fidelityChecked"`
	FidelityErrors  int `json:"fidelityErrors"`

	// cost
	TotalTokens int         `json:"totalTokens"`
	TotalCost   float64     `json:"totalCost"`
	CostPoints  []CostPoint `json:"costPoints"`
}

// CostPoint is one decision's prompt-context size, keyed by hand — the series the
// summary's context-growth chart plots.
type CostPoint struct {
	Hand         int     `json:"hand"`
	PromptTokens int     `json:"promptTokens"`
	Cost         float64 `json:"cost"`
}
