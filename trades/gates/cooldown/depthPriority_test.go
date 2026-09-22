package cooldown

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// walletDepths is the depths every ladder of the fixture wallet is
// configured for.
const walletDepths = 8

// requireDepthPriority skips when the gate is switched off, so the suite says
// "off" instead of failing a rule nobody is running.
func requireDepthPriority(t *testing.T) {
	t.Helper()

	if !DepthPriority {
		t.Skip("DepthPriority is off: the wallet gate is not in force")
	}
}

// walletView is the shape the gate was built for: the pairs fell together,
// every ladder of the wallet is part way down its grid, and one of them is
// short of the last depth it was sized for.
func walletView() []aggragates.LadderDepth {
	return []aggragates.LadderDepth{
		{TradeID: 11, Symbol: "BTC/USDT", Depth: 5, MaxDepth: walletDepths},
		{TradeID: 12, Symbol: "ETH/USDT", Depth: 4, MaxDepth: walletDepths},
		{TradeID: 13, Symbol: "SOL/USDT", Depth: 6, MaxDepth: walletDepths},
		{TradeID: 14, Symbol: "LINK/USDT", Depth: 7, MaxDepth: walletDepths},
		{TradeID: 15, Symbol: "HBAR/USDT", Depth: 2, MaxDepth: walletDepths},
	}
}

// priorityEvent is the managed trade on a tick that carries the wallet view.
func priorityEvent(trade aggragates.Trades, oldPosition string, ladders []aggragates.LadderDepth) events.Events {
	return events.Events{
		Trade:  trade,
		Params: aggragates.Params{OldPosition: oldPosition, WalletLadders: ladders},
	}
}

// A ladder reserves the wallet from the first depth past Depths minus the
// margin, and stops reserving it when it is full: a full ladder has no entry
// left to prioritize, which is what releases the siblings it was holding.
// The boundary is derived from the constant, never written out — the margin
// is a calibration knob and a literal here would only pin today's value.
func TestDepthPriorityCandidateBoundary(t *testing.T) {
	requireDepthPriority(t)

	firstCandidateDepth := walletDepths - DepthPriorityMargin + 1

	for depth := 0; depth <= walletDepths; depth++ {
		candidate := aggragates.LadderDepth{TradeID: 1, Symbol: "LINK/USDT", Depth: depth, MaxDepth: walletDepths}
		want := depth >= firstCandidateDepth && depth < walletDepths

		if got := isDepthPriorityCandidate(candidate); got != want {
			t.Errorf("depth %d of %d: candidate = %v, want %v", depth, walletDepths, got, want)
		}
	}

	// A pair carrying no settings row has no configured ceiling, so nothing
	// about it can reserve the wallet.
	if isDepthPriorityCandidate(aggragates.LadderDepth{TradeID: 2, Symbol: "HBAR/USDT", Depth: 7}) {
		t.Error("a ladder with unknown configured depths must never reserve the wallet")
	}
}

// The product rule end to end: the deepest ladder short of its last entry
// takes the wallet and the shallower ones wait, with the row naming both
// sides so an operator can see what is being waited for.
func TestDepthPriorityHoldsTheShallowerLadders(t *testing.T) {
	requireDepthPriority(t)

	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", walletView()), "stopLoss")
	if reason == "" {
		t.Fatal("a ladder shallower than the priority one must wait")
	}
	if !strings.HasPrefix(reason, "cooldown: depth priority,") {
		t.Errorf("reason = %q, want the cooldown family named first", reason)
	}
	if !strings.Contains(reason, "LINK/USDT at depth 7 of 8") {
		t.Errorf("reason = %q, want the deepest ladder named", reason)
	}
	if !strings.Contains(reason, "this ladder waits at depth 4 of 8") {
		t.Errorf("reason = %q, want the held ladder's own depth", reason)
	}
}

// The priority ladder is not held by its own reservation, and neither is a
// sibling standing at the same depth: an equal ladder is not a smaller one,
// and holding it would park both of them for each other.
func TestDepthPriorityNeverHoldsThePriorityOrItsEqual(t *testing.T) {
	requireDepthPriority(t)

	wallet := walletView()
	own := testutil.LadderDepthTrade(14, "LINK/USDT", 7, walletDepths)

	if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", wallet), "stopLoss"); reason != "" {
		t.Errorf("the priority ladder must take its own entry, got %q", reason)
	}

	twin := testutil.LadderDepthTrade(9, "DOT/USDT", 7, walletDepths)
	wallet = append(wallet, aggragates.LadderDepth{TradeID: 9, Symbol: "DOT/USDT", Depth: 7, MaxDepth: walletDepths})

	if reason := DepthPriorityHoldReason(priorityEvent(twin, "buy", wallet), "stopLoss"); reason != "" {
		t.Errorf("a ladder at the priority depth is not a smaller one, got %q", reason)
	}
}

// Two ladders at the same depth are both worth finishing, so the wallet is
// reserved for the lower trade id. Any deterministic choice would do; what
// matters is that every engine makes the same one, and that the row does not
// change its mind tick to tick.
func TestDepthPriorityBreaksTiesOnTheLowestTradeID(t *testing.T) {
	requireDepthPriority(t)

	wallet := append(walletView(), aggragates.LadderDepth{TradeID: 9, Symbol: "DOT/USDT", Depth: 7, MaxDepth: walletDepths})
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", wallet), "stopLoss")
	if !strings.Contains(reason, "DOT/USDT") {
		t.Fatalf("reason = %q, want the lowest trade id of the tied depth named", reason)
	}
}

// The first fill is new capital like any other, and a new trade has the
// smallest depth of all: it waits for the ladder that is nearly finished.
func TestDepthPriorityHoldsTheFirstFill(t *testing.T) {
	requireDepthPriority(t)

	trade := testutil.LadderDepthTrade(21, "ADA/USDT", 0, walletDepths)

	reason := DepthPriorityHoldReason(priorityEvent(trade, "new", walletView()), "buy")
	if !strings.Contains(reason, "this ladder waits at depth 0 of 8") {
		t.Fatalf("reason = %q, want the first fill held at depth 0", reason)
	}
}

// A gate on new capital must never defer a close — and here the close is what
// releases every ladder the wallet was reserved against.
func TestDepthPriorityNeverGatesACloseOrAChild(t *testing.T) {
	requireDepthPriority(t)

	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	for _, position := range []string{"takeProfit", "forceTrailingTakeProfit", "sell", "sellLoss"} {
		if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", walletView()), position); reason != "" {
			t.Errorf("%s must not be gated, got %q", position, reason)
		}
	}

	// An impasse child spends what its parent's close freed, not the wallet
	// the parents compete for.
	child := testutil.LadderDepthTrade(31, "XRP/USDT", 1, walletDepths)
	child.ParentID = 12
	if reason := DepthPriorityHoldReason(priorityEvent(child, "buy", walletView()), "stopLoss"); reason != "" {
		t.Errorf("an impasse child must not be held by the wallet gate, got %q", reason)
	}
}

// Fail open, the posture of this whole package: a wallet view the engine
// could not build, or one where no ladder is near its last depth, holds
// nothing.
func TestDepthPriorityFailsOpenOnAnEmptyWalletView(t *testing.T) {
	requireDepthPriority(t)

	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	for name, wallet := range map[string][]aggragates.LadderDepth{
		"nil":   nil,
		"empty": {},
		"nobody near the last depth": {
			{TradeID: 11, Symbol: "BTC/USDT", Depth: 5, MaxDepth: walletDepths},
			{TradeID: 15, Symbol: "HBAR/USDT", Depth: 2, MaxDepth: walletDepths},
		},
		"every ladder full": {
			{TradeID: 11, Symbol: "BTC/USDT", Depth: walletDepths, MaxDepth: walletDepths},
			{TradeID: 14, Symbol: "LINK/USDT", Depth: walletDepths, MaxDepth: walletDepths},
		},
	} {
		if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", wallet), "stopLoss"); reason != "" {
			t.Errorf("%s wallet view must hold nothing, got %q", name, reason)
		}
	}
}

// gates.SaveHoldLog deduplicates on the full message, so the row has to be
// byte-identical while the hold stands. Both depths in it are frozen for the
// duration of the hold — a held entry is one that has not filled.
func TestDepthPriorityWritesAStableReason(t *testing.T) {
	requireDepthPriority(t)

	event := priorityEvent(testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths), "buy", walletView())

	first := DepthPriorityHoldReason(event, "stopLoss")
	second := DepthPriorityHoldReason(event, "stopLoss")
	if first == "" {
		t.Fatal("expected the ladder to be held")
	}
	if first != second {
		t.Fatalf("the reason moved between ticks: %q then %q", first, second)
	}
}

// The transition rule the engines read before they pay for a wallet view:
// the first fill and every add, including the re-anchors that resolve to a
// stopLoss, and never a close. The flag owns the gate, so with Cooldown off
// no tick ever builds one.
func TestDepthPriorityApplies(t *testing.T) {
	requireDepthPriority(t)

	on := aggragates.StrategyParams{Cooldown: true}

	for position, want := range map[string]bool{
		"stopLoss":                true,
		"forceTrailingStopLoss":   true,
		"takeProfit":              false,
		"forceTrailingTakeProfit": false,
		"sell":                    false,
	} {
		if got := DepthPriorityApplies(on, "buy", position); got != want {
			t.Errorf("DepthPriorityApplies(%q) = %v, want %v", position, got, want)
		}
		if DepthPriorityApplies(aggragates.StrategyParams{}, "buy", position) {
			t.Errorf("the Cooldown flag is off: %q must not build a wallet view", position)
		}
	}

	if !DepthPriorityApplies(on, "new", "buy") {
		t.Error("a first fill spends the wallet and must build the view")
	}
	if DepthPriorityApplies(aggragates.StrategyParams{}, "new", "buy") {
		t.Error("the Cooldown flag is off: a first fill must not build a wallet view either")
	}
}
