// Current session/hand state tracking shared by Pi poker agents.
export function createAgentState() {
    return {};
}
export function applySessionInit(state, payload) {
    state.session = {
        sessionId: payload.session_id,
        matchId: payload.match.match_id,
        match: {
            ...payload.match,
            blinds: { ...payload.match.blinds },
        },
        agentName: payload.agent_name,
        yourSeat: payload.your_seat,
        seats: payload.seats.map((seat) => ({ ...seat })),
        memoryDir: payload.memory_dir,
    };
    resetHandState(state);
}
export function applyHandStart(state, payload) {
    state.hand = {
        handNumber: payload.hand_number,
        dealerSeat: payload.dealer_seat,
        stacks: { ...payload.stacks },
        blindsPosted: payload.blinds_posted.map((blind) => ({ ...blind })),
        yourHoleCards: [...payload.your_hole_cards],
    };
}
export function applyYourTurn(state, payload) {
    if (!state.hand || state.hand.handNumber !== payload.hand_number) {
        state.hand = {
            handNumber: payload.hand_number,
            dealerSeat: -1,
            stacks: { ...payload.stacks },
            blindsPosted: [],
            yourHoleCards: [],
        };
    }
    state.hand.currentTurn = {
        street: payload.street,
        board: [...payload.board],
        pot: payload.pot,
        toCall: payload.to_call,
        stacks: { ...payload.stacks },
        seats: payload.seats.map((seat) => ({ ...seat })),
        actionHistory: payload.action_history.map((entry) => ({ ...entry })),
        legalActions: payload.legal_actions.map((action) => ({ ...action })),
    };
    state.hand.stacks = { ...payload.stacks };
}
export function resetHandState(state) {
    delete state.hand;
}
export function resetSessionState(state) {
    resetHandState(state);
    delete state.session;
}
