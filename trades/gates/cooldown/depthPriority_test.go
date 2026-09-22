package cooldown

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// walletDepths is the depths every ladder of the fixture wallet is
// configured for.
const walletDepths = 8

// walletAsset is what every ladder of the fixture wallet spends: the quote
// side of a long pair.
const walletAsset = "USDT"

// requireDepthPriority skips when the gate is switched off, so the suite says
// "off" instead of failing a rule nobody is running.
func requireDepthPriority(t *testing.T) {
	t.Helper()

	if !DepthPriority {
		t.Skip("DepthPriority is off: the wallet gate is not in force")
	}
}

// walletLadder maps a fixture ladder into the view an engine hands the tick,
// through the one helper all four surfaces build their view with — so the
// depths AND the costs the gate reads are taken off a real ladder instead of
// written out beside it.
func walletLadder(id uint, symbol string, depth int) aggragates.LadderDepth {
	return ladder.DepthOf(testutil.LadderDepthTrade(id, symbol, depth, walletDepths))
}

// walletView is the shape the gate was built for: the pairs fell together and
// every ladder of the wallet is part way down its grid, the deepest one still
// short of the last depth it was sized for.
func walletView() []aggragates.LadderDepth {
	return []aggragates.LadderDepth{
		walletLadder(11, "BTC/USDT", 5),
		walletLadder(12, "ETH/USDT", 4),
		walletLadder(13, "SOL/USDT", 6),
		walletLadder(14, "LINK/USDT", 7),
		walletLadder(15, "HBAR/USDT", 2),
	}
}

// viewReserve is what the wallet is being kept for: the remaining cost of the
// deepest ladder of the view, read back off the view itself.
func viewReserve(t *testing.T, ladders []aggragates.LadderDepth, asset string) float64 {
	t.Helper()

	priority, found := deepestDepthPriorityCandidate(ladders, asset)
	if !found {
		t.Fatalf("the fixture wallet must reserve for one of its ladders, got none for %s", asset)
	}
	if priority.RemainingCost <= 0 {
		t.Fatalf("%s reserves nothing: the fixture ladder has no cost", priority.Symbol)
	}

	return priority.RemainingCost
}

// walletShortFor is a wallet one unit of quote under the line: it cannot
// carry both the reserve and this trade's next entry.
func walletShortFor(t *testing.T, trade aggragates.Trades, reserve float64) float64 {
	t.Helper()

	return reserve + nextEntryCostOf(t, trade, reserve) - 1
}

// nextEntryCostOf is what the managed trade's next entry would spend out of
// the given wallet, priced the way the gate prices it.
func nextEntryCostOf(t *testing.T, trade aggragates.Trades, free float64) float64 {
	t.Helper()

	_, cost := ladder.NextEntryCost(trade, free)
	if cost <= 0 {
		t.Fatalf("%s must cost something to place its next entry", trade.Symbol)
	}

	return cost
}

// priorityEvent is the managed trade on a tick that carries the wallet view
// and the balance the entry would be placed from.
func priorityEvent(trade aggragates.Trades, oldPosition string, ladders []aggragates.LadderDepth, free float64) events.Events {
	event := blindPriorityEvent(trade, oldPosition, ladders)
	event.Params.WalletFree = []aggragates.AssetFree{{Asset: walletAsset, Free: free}}

	return event
}

// blindPriorityEvent is the same tick with no balance at all: the engine
// could not name one, which is the fail-open side of the rule.
func blindPriorityEvent(trade aggragates.Trades, oldPosition string, ladders []aggragates.LadderDepth) events.Events {
	return events.Events{
		Trade:  trade,
		Params: aggragates.Params{OldPosition: oldPosition, WalletLadders: ladders},
	}
}

// A ladder is in front of the wallet from its FIRST filled depth until it
// closes — its last depth included. The near end matters because arming only
// near the ceiling would reserve the wallet at the point where the biggest
// entries can no longer be afforded; the far end matters because a full
// ladder's close is what refills the wallet, and dropping it there releases
// the siblings into a wallet it has just spent.
func TestDepthPriorityCandidateSpansEveryFilledDepth(t *testing.T) {
	requireDepthPriority(t)

	for depth := 0; depth <= walletDepths; depth++ {
		candidate := aggragates.LadderDepth{TradeID: 1, Symbol: "LINK/USDT", Depth: depth, MaxDepth: walletDepths}
		want := depth >= 1

		if got := isDepthPriorityCandidate(candidate); got != want {
			t.Errorf("depth %d of %d: candidate = %v, want %v", depth, walletDepths, got, want)
		}
	}

	// A pair carrying no settings row has no ladder to speak of, so nothing
	// about it can stand in front of the wallet.
	if isDepthPriorityCandidate(aggragates.LadderDepth{TradeID: 2, Symbol: "HBAR/USDT", Depth: 7}) {
		t.Error("a ladder with unknown configured depths must never keep the wallet")
	}
}

// A ladder that has filled its last depth keeps nothing — its remaining cost
// is nothing — but it stays in front until it closes. So the hold condition
// collapses to the plainest question there is: can the wallet pay for this
// entry? If it can, the sibling buys, and that IS the release at the ceiling.
// If it cannot, the sibling WAITS rather than reaching the funds gate and
// being blocked out of the wallet view altogether.
func TestDepthPriorityFullLadderHoldsOnlyWhatTheWalletCannotPayFor(t *testing.T) {
	requireDepthPriority(t)

	full := []aggragates.LadderDepth{walletLadder(14, "LINK/USDT", walletDepths)}
	if full[0].RemainingCost != 0 {
		t.Fatalf("a full ladder keeps %f, want nothing", full[0].RemainingCost)
	}

	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	ownCost := nextEntryCostOf(t, trade, 0)

	if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", full, ownCost), "stopLoss"); reason != "" {
		t.Fatalf("a wallet that can pay for the entry must let it through, got %q", reason)
	}

	reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", full, ownCost-1), "stopLoss")
	want := "cooldown: depth priority, LINK/USDT at depth 8 of 8 holds the wallet until it closes, this ladder waits at depth 4 of 8"
	if reason != want {
		t.Fatalf("reason = %q, want %q", reason, want)
	}
}

// The managed trade is ranked against the view's best by the same ordering,
// so the deepest ladder never waits — not even when it is missing from the
// view because it is blocked on its own next entry. The view is what the
// OTHER ladders see; this trade knows its own depth first-hand. Judging it by
// the view alone would park it behind a shallower ladder the moment the
// wallet could pay for its entry again.
func TestDepthPriorityRanksTheManagedTradeAgainstTheView(t *testing.T) {
	requireDepthPriority(t)

	// The view the engines built while this ladder was blocked: it is not in
	// it, and the ladder that is, is shallower.
	shallower := []aggragates.LadderDepth{walletLadder(12, "ETH/USDT", 4)}
	deepest := testutil.LadderDepthTrade(14, "LINK/USDT", 7, walletDepths)

	if reason := DepthPriorityHoldReason(priorityEvent(deepest, "buy", shallower, 0), "stopLoss"); reason != "" {
		t.Fatalf("the deepest ladder must not wait for a shallower one, got %q", reason)
	}

	// Level with the view's best: the lower trade id is in front, so the one
	// below it buys and the one above it waits on a wallet that is short.
	level := []aggragates.LadderDepth{walletLadder(12, "ETH/USDT", 4)}
	lower := testutil.LadderDepthTrade(9, "DOT/USDT", 4, walletDepths)
	higher := testutil.LadderDepthTrade(21, "ADA/USDT", 4, walletDepths)

	reserve := viewReserve(t, level, walletAsset)
	if reason := DepthPriorityHoldReason(priorityEvent(lower, "buy", level, walletShortFor(t, lower, reserve)), "stopLoss"); reason != "" {
		t.Fatalf("the lower trade id is in front of a ladder at the same depth, got %q", reason)
	}
	if reason := DepthPriorityHoldReason(priorityEvent(higher, "buy", level, walletShortFor(t, higher, reserve)), "stopLoss"); reason == "" {
		t.Fatal("the higher trade id at the same depth must wait when the wallet is short")
	}
}

// The product rule end to end: a wallet that cannot cover both the deepest
// ladder's remaining depths and this entry holds the entry, with the row
// naming both sides so an operator can see what is being waited for.
func TestDepthPriorityHoldsAnEntryTheWalletCannotSpare(t *testing.T) {
	requireDepthPriority(t)

	wallet := walletView()
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	reserve := viewReserve(t, wallet, walletAsset)
	free := reserve + nextEntryCostOf(t, trade, reserve) - 1

	reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", wallet, free), "stopLoss")
	if reason == "" {
		t.Fatal("an entry that would break into the reserve must wait")
	}
	if !strings.HasPrefix(reason, "cooldown: depth priority,") {
		t.Errorf("reason = %q, want the cooldown family named first", reason)
	}
	if !strings.Contains(reason, "LINK/USDT at depth 7 of 8") {
		t.Errorf("reason = %q, want the reserved ladder named", reason)
	}
	if !strings.Contains(reason, "this ladder waits at depth 4 of 8") {
		t.Errorf("reason = %q, want the held ladder's own depth", reason)
	}
}

// When the wallet covers both, everybody buys. The reserve is a floor under
// the deepest ladder's remaining depths, not a queue: a rich wallet holds
// nothing at all.
func TestDepthPriorityHoldsNothingWhenTheWalletCoversBoth(t *testing.T) {
	requireDepthPriority(t)

	wallet := walletView()
	reserve := viewReserve(t, wallet, walletAsset)

	for _, trade := range []aggragates.Trades{
		testutil.LadderDepthTrade(11, "BTC/USDT", 5, walletDepths),
		testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths),
		testutil.LadderDepthTrade(15, "HBAR/USDT", 2, walletDepths),
	} {
		free := reserve + nextEntryCostOf(t, trade, reserve)

		if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", wallet, free), "stopLoss"); reason != "" {
			t.Errorf("%s: a wallet that covers both must hold nothing, got %q", trade.Symbol, reason)
		}
	}
}

// The boundary is exactly what the rule says it is: the entry is held while
// what would be left after placing it falls short of the reserve, and goes
// through the moment the two are level. One unit of quote either side of the
// line, far above float noise and far below any entry of this ladder.
func TestDepthPriorityBoundaryIsTheReserveItself(t *testing.T) {
	requireDepthPriority(t)

	wallet := walletView()
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	reserve := viewReserve(t, wallet, walletAsset)
	ownCost := nextEntryCostOf(t, trade, reserve)

	if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", wallet, reserve+ownCost-1), "stopLoss"); reason == "" {
		t.Error("a wallet one unit short of the reserve must hold the entry")
	}
	if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", wallet, reserve+ownCost), "stopLoss"); reason != "" {
		t.Errorf("a wallet level with the reserve must let the entry through, got %q", reason)
	}
}

// The reserved ladder is never held by its own reservation: it is the one the
// wallet is being kept for, so the funds it needs are its own.
func TestDepthPriorityNeverHoldsTheReservedLadder(t *testing.T) {
	requireDepthPriority(t)

	wallet := walletView()
	own := testutil.LadderDepthTrade(14, "LINK/USDT", 7, walletDepths)

	// Nothing but the reserve in the wallet, which is what would hold any
	// other ladder of it.
	free := viewReserve(t, wallet, walletAsset)

	if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", wallet, free), "stopLoss"); reason != "" {
		t.Errorf("the reserved ladder must take its own entry, got %q", reason)
	}
}

// Two ladders at the same depth are both worth finishing, so the wallet is
// reserved for the lower trade id and the other one is simply one of the
// others: equal depth is no exemption. Any deterministic choice would do;
// what matters is that every engine makes the same one, and that the row does
// not change its mind tick to tick.
func TestDepthPriorityBreaksTiesOnTheLowestTradeID(t *testing.T) {
	requireDepthPriority(t)

	wallet := append(walletView(), walletLadder(9, "DOT/USDT", 7))
	twin := testutil.LadderDepthTrade(14, "LINK/USDT", 7, walletDepths)

	reserve := viewReserve(t, wallet, walletAsset)
	free := reserve + nextEntryCostOf(t, twin, reserve) - 1

	reason := DepthPriorityHoldReason(priorityEvent(twin, "buy", wallet, free), "stopLoss")
	if !strings.Contains(reason, "DOT/USDT at depth 7 of 8") {
		t.Fatalf("reason = %q, want the lowest trade id of the tied depth to keep the wallet", reason)
	}
}

// A first fill is an entry like any other. It has no fills to multiply, so
// what it would spend is the ladder's initial bid off the wallet it is being
// sized against — and a wallet that cannot carry both that bid and the
// reserve holds it.
func TestDepthPriorityPricesAFirstFillThroughTheInitialBid(t *testing.T) {
	requireDepthPriority(t)

	wallet := walletView()
	newcomer := testutil.LadderDepthTrade(21, "ADA/USDT", 0, walletDepths)

	reserve := viewReserve(t, wallet, walletAsset)
	if cost := nextEntryCostOf(t, newcomer, reserve); cost <= 0 {
		t.Fatalf("a first fill must be priced, got %f", cost)
	}

	reason := DepthPriorityHoldReason(priorityEvent(newcomer, "new", wallet, reserve), "buy")
	if !strings.Contains(reason, "this ladder waits at depth 0 of 8") {
		t.Fatalf("reason = %q, want the first fill held at depth 0", reason)
	}

	// The same first fill against a wallet that carries the reserve and the
	// bid together goes straight through.
	free := reserve * 2
	if reason := DepthPriorityHoldReason(priorityEvent(newcomer, "new", wallet, free), "buy"); reason != "" {
		t.Fatalf("a wallet that covers the reserve and the bid must let a first fill through, got %q", reason)
	}
}

// An inverse ladder spends the BASE side of its pair, so its reserve and its
// entry are both counted in base units and it competes only with the ladders
// spending the same base. Pricing it in quote would compare two currencies.
func TestDepthPriorityPricesAnInverseLadderInBase(t *testing.T) {
	requireDepthPriority(t)

	deep := inverseLadderTrade(31, "BTC/USDT", 6, walletDepths)
	shallow := inverseLadderTrade(32, "BTC/EUR", 2, walletDepths)

	view := []aggragates.LadderDepth{ladder.DepthOf(deep), ladder.DepthOf(shallow)}
	if view[0].Asset != "BTC" {
		t.Fatalf("an inverse ladder spends %q, want the base asset", view[0].Asset)
	}

	reserve := viewReserve(t, view, "BTC")
	_, ownCost := ladder.NextEntryCost(shallow, reserve)

	event := blindPriorityEvent(shallow, "buy", view)
	event.Params.WalletFree = []aggragates.AssetFree{{Asset: "BTC", Free: reserve + ownCost - 0.0001}}

	if reason := DepthPriorityHoldReason(event, "stopLoss"); reason == "" {
		t.Fatal("an inverse ladder short of the base reserve must wait")
	}

	event.Params.WalletFree = []aggragates.AssetFree{{Asset: "BTC", Free: reserve + ownCost}}
	if reason := DepthPriorityHoldReason(event, "stopLoss"); reason != "" {
		t.Fatalf("an inverse wallet level with the reserve must let the entry through, got %q", reason)
	}
}

// Ladders funded from different currencies never take money from each other,
// so a deep ladder spending another asset reserves nothing here — however
// short this wallet is.
func TestDepthPriorityIgnoresLaddersSpendingAnotherAsset(t *testing.T) {
	requireDepthPriority(t)

	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	elsewhere := []aggragates.LadderDepth{ladder.DepthOf(inverseLadderTrade(31, "BTC/USDT", 7, walletDepths))}

	if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", elsewhere, 0), "stopLoss"); reason != "" {
		t.Errorf("a ladder spending another asset must reserve nothing here, got %q", reason)
	}
}

// A balance the engine could not name holds nothing: the gate never parks a
// wallet on a guess, which is the fail-open posture of this whole package.
func TestDepthPriorityHoldsNothingOnAnUnknownBalance(t *testing.T) {
	requireDepthPriority(t)

	wallet := walletView()
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	if reason := DepthPriorityHoldReason(blindPriorityEvent(trade, "buy", wallet), "stopLoss"); reason != "" {
		t.Errorf("a tick with no balance must hold nothing, got %q", reason)
	}

	// A balance named for some OTHER asset is no balance for this ladder.
	event := blindPriorityEvent(trade, "buy", wallet)
	event.Params.WalletFree = []aggragates.AssetFree{{Asset: "BTC", Free: 0}}
	if reason := DepthPriorityHoldReason(event, "stopLoss"); reason != "" {
		t.Errorf("a balance for another asset must hold nothing, got %q", reason)
	}
}

// The quote this wallet's inverse trades have already committed is not free
// to spend, and the funds gate subtracts it before it compares — so the
// reserve is measured against the same number, or a long ladder would be let
// through on money an inverse position is holding.
func TestDepthPrioritySubtractsTheQuoteInverseTradesCommitted(t *testing.T) {
	requireDepthPriority(t)

	wallet := walletView()
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	reserve := viewReserve(t, wallet, walletAsset)
	free := reserve + nextEntryCostOf(t, trade, reserve)

	if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", wallet, free), "stopLoss"); reason != "" {
		t.Fatalf("the wallet covers both before the inverse commitment, got %q", reason)
	}

	committed := priorityEvent(trade, "buy", wallet, free)
	committed.Params.InverseUsedAmount = []aggragates.UsedAmountResult{{QuoteCurrency: walletAsset, UsedAmount: 1}}

	if reason := DepthPriorityHoldReason(committed, "stopLoss"); reason == "" {
		t.Fatal("quote committed by an inverse trade must not count towards the reserve")
	}
}

// A gate on new capital must never defer a close — and here the close is what
// refills the wallet the reserve is measured against.
func TestDepthPriorityNeverGatesACloseOrAChild(t *testing.T) {
	requireDepthPriority(t)

	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	for _, position := range []string{"takeProfit", "forceTrailingTakeProfit", "sell", "sellLoss"} {
		if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", walletView(), 0), position); reason != "" {
			t.Errorf("%s must not be gated, got %q", position, reason)
		}
	}

	// An impasse child spends what its parent's close freed, not the wallet
	// the parents compete for.
	child := testutil.LadderDepthTrade(31, "XRP/USDT", 1, walletDepths)
	child.ParentID = 12
	if reason := DepthPriorityHoldReason(priorityEvent(child, "buy", walletView(), 0), "stopLoss"); reason != "" {
		t.Errorf("an impasse child must not be held by the wallet gate, got %q", reason)
	}
}

// Fail open, the posture of this whole package: a wallet view the engine
// could not build, or one holding no ladder at all, keeps nothing however
// empty the wallet is. A ladder that has filled nothing is no ladder yet.
func TestDepthPriorityFailsOpenOnAnEmptyWalletView(t *testing.T) {
	requireDepthPriority(t)

	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	for name, wallet := range map[string][]aggragates.LadderDepth{
		"nil":   nil,
		"empty": {},
		"nothing filled yet": {
			walletLadder(11, "BTC/USDT", 0),
			walletLadder(15, "HBAR/USDT", 0),
		},
	} {
		if reason := DepthPriorityHoldReason(priorityEvent(trade, "buy", wallet, 0), "stopLoss"); reason != "" {
			t.Errorf("%s wallet view must hold nothing, got %q", name, reason)
		}
	}
}

// gates.SaveHoldLog deduplicates on the full message, so the row has to be
// byte-identical while the hold stands. The balance is the reason it carries
// neither ladder's cost: it moves on every tick, and a row that moved with it
// would be a new row per tick for as long as the reservation stood.
func TestDepthPriorityWritesAStableReason(t *testing.T) {
	requireDepthPriority(t)

	wallet := walletView()
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	reserve := viewReserve(t, wallet, walletAsset)
	ownCost := nextEntryCostOf(t, trade, reserve)

	first := DepthPriorityHoldReason(priorityEvent(trade, "buy", wallet, reserve+ownCost-1), "stopLoss")
	if first == "" {
		t.Fatal("expected the ladder to be held")
	}

	// The next tick, with the wallet moved by a fraction of an entry the way
	// a live balance moves.
	second := DepthPriorityHoldReason(priorityEvent(trade, "buy", wallet, reserve+ownCost-2), "stopLoss")
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

// inverseLadderTrade is the fixture ladder on the other side: an inverse
// trade enters by SELLING base, so its entries are SELL rows and everything
// it spends is counted in the base asset.
func inverseLadderTrade(id uint, symbol string, depth int, maxDepths float64) aggragates.Trades {
	trade := testutil.LadderDepthTrade(id, symbol, depth, maxDepths)
	trade.Inverse = true

	for index := range trade.History {
		trade.History[index].Type = "SELL"
	}

	return trade
}
