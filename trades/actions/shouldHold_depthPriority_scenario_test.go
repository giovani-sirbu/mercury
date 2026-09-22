package actions

import (
	"fmt"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The wallet the gate was asked for, end to end through ShouldHold: five pairs
// that fell together, each ladder part way down a grid sized for the same
// number of depths, and one of them deeper than the rest.
//
// Everything here is built as real trades and mapped into the view through
// ladder.DepthOf — the helper every engine builds its own view with — so the
// depths AND the costs the gate reads are taken off the ladders instead of
// written out beside them. A view written by hand could agree with an engine
// that counts or prices differently; this one cannot.
var fallenLadders = []struct {
	id     uint
	symbol string
	depth  int
}{
	{11, "BTC/USDT", 5},
	{12, "ETH/USDT", 4},
	{13, "SOL/USDT", 6},
	{14, "LINK/USDT", 7},
	{15, "HBAR/USDT", 2},
}

// scenarioDepths is the grid every pair of this wallet is configured for.
const scenarioDepths = 8

// priorityLadder is the deepest ladder of the wallet, the one its remaining
// depths are kept for.
const priorityLadder = "LINK/USDT"

// runnerUpLadder is the next deepest, the one the reservation passes to when
// the deepest leaves.
const runnerUpLadder = "SOL/USDT"

// fallenWallet builds the five ladders as trades.
func fallenWallet() []aggragates.Trades {
	trades := make([]aggragates.Trades, 0, len(fallenLadders))
	for _, ladderOf := range fallenLadders {
		trades = append(trades, testutil.LadderDepthTrade(ladderOf.id, ladderOf.symbol, ladderOf.depth, scenarioDepths))
	}
	return trades
}

// walletDepthsOf is the view an engine hands the tick: every ladder of the
// wallet, counted and priced by the one helper all four surfaces share.
func walletDepthsOf(trades []aggragates.Trades) []aggragates.LadderDepth {
	wallet := make([]aggragates.LadderDepth, 0, len(trades))
	for _, trade := range trades {
		wallet = append(wallet, ladder.DepthOf(trade))
	}
	return wallet
}

// reserveOf is the ladder a view puts in front of the wallet — the deepest
// one that has filled anything, its last depth included — and what it keeps.
// A full ladder keeps nothing and still stands in front until it closes.
func reserveOf(t *testing.T, wallet []aggragates.LadderDepth) (aggragates.LadderDepth, float64) {
	t.Helper()

	var deepest aggragates.LadderDepth
	found := false

	for _, candidate := range wallet {
		if candidate.MaxDepth <= 0 || candidate.Depth < 1 {
			continue
		}
		if !found || candidate.Depth > deepest.Depth || (candidate.Depth == deepest.Depth && candidate.TradeID < deepest.TradeID) {
			deepest, found = candidate, true
		}
	}

	if !found {
		t.Fatal("the example wallet must have a ladder that has filled something")
	}

	return deepest, deepest.RemainingCost
}

// tightestWallet is the smallest balance on which nobody of this wallet
// waits: it carries the reserve plus the most expensive next entry in it.
func tightestWallet(t *testing.T, trades []aggragates.Trades, reserve float64) float64 {
	t.Helper()

	dearest := 0.0
	for _, trade := range trades {
		_, ownCost := ladder.NextEntryCost(trade, reserve)
		if ownCost > dearest {
			dearest = ownCost
		}
	}

	return reserve + dearest
}

// withoutLadder is the wallet a close leaves behind: the trade is gone from
// the view altogether, which is what ends its reservation without anything
// having to remember that a hold ever stood.
func withoutLadder(trades []aggragates.Trades, symbol string) []aggragates.Trades {
	remaining := make([]aggragates.Trades, 0, len(trades))
	for _, trade := range trades {
		if trade.Symbol != symbol {
			remaining = append(remaining, trade)
		}
	}
	return remaining
}

// closedPositionValue is what a ladder's close puts back into the wallet: the
// base its entries bought, at the price its position sits at. The close is
// the event the waiting ladders are really waiting for.
func closedPositionValue(trade aggragates.Trades) float64 {
	held := 0.0
	for _, row := range trade.History {
		if row.Type == "BUY" {
			held += row.Quantity
		}
	}
	return held * trade.PositionPrice
}

// configuredDepthOf is the depth the example puts a pair at, read back from
// the fixture so an expectation cannot drift from the wallet it describes.
func configuredDepthOf(t *testing.T, symbol string) int {
	t.Helper()

	for _, ladderOf := range fallenLadders {
		if ladderOf.symbol == symbol {
			return ladderOf.depth
		}
	}

	t.Fatalf("%s is not a ladder of the example wallet", symbol)
	return 0
}

// filledTo is the same ladder after an entry fills.
func filledTo(trade aggragates.Trades, depth int) aggragates.Trades {
	return testutil.LadderDepthTrade(trade.ID, trade.Symbol, depth, scenarioDepths)
}

// waitingRow is the row the operator reads on a held ladder: which ladder the
// wallet is kept for, and where this one stands.
func waitingRow(priority string, priorityDepth, ownDepth int) string {
	return fmt.Sprintf(
		"Hold stopLoss: cooldown: depth priority, %s at depth %d of %d keeps the wallet for its remaining depths, this ladder waits at depth %d of %d",
		priority, priorityDepth, scenarioDepths, ownDepth, scenarioDepths,
	)
}

// assertFreeToArm fails when the ladder is held on the tick that arms its next
// entry.
func assertFreeToArm(t *testing.T, trade aggragates.Trades, wallet []aggragates.LadderDepth, free float64) {
	t.Helper()

	released, err := ShouldHold(priorityEvent(trade, "buy", wallet, free))
	if err != nil {
		t.Fatalf("%s must be free to arm its next entry, got %v", trade.Symbol, err)
	}
	if len(released.Trade.Logs) != 0 {
		t.Fatalf("%s: no row may be written on a released ladder, got %v", trade.Symbol, messages(released.Trade.Logs))
	}
}

// The user's own example on a wallet that cannot carry both: LINK is the
// deepest ladder, so what its remaining depths cost is kept for it and the
// four shallower ones wait, each row naming both sides.
func TestFallenWalletReservesItsRemainingDepthsForTheDeepestLadder(t *testing.T) {
	trades := fallenWallet()
	wallet := walletDepthsOf(trades)

	priority, reserve := reserveOf(t, wallet)
	if priority.Symbol != priorityLadder {
		t.Fatalf("the wallet is kept for %s, want %s", priority.Symbol, priorityLadder)
	}

	for index, trade := range trades {
		free := walletShortOf(t, trade, reserve)

		if trade.Symbol == priorityLadder {
			assertFreeToArm(t, trade, wallet, free)
			continue
		}

		held, err := ShouldHold(priorityEvent(trade, "buy", wallet, free))
		if err == nil {
			t.Fatalf("%s at depth %d must wait for %s", trade.Symbol, fallenLadders[index].depth, priorityLadder)
		}
		if len(held.Trade.Logs) != 1 {
			t.Fatalf("%s: expected one row, got %v", trade.Symbol, messages(held.Trade.Logs))
		}

		want := waitingRow(priorityLadder, priority.Depth, fallenLadders[index].depth)
		if got := held.Trade.Logs[0].Message; got != want {
			t.Fatalf("%s row = %q, want %q", trade.Symbol, got, want)
		}
		if held.Trade.Logs[0].Type != aggragates.LOG_INFO {
			t.Errorf("%s row type = %q, want %q", trade.Symbol, held.Trade.Logs[0].Type, aggragates.LOG_INFO)
		}
	}
}

// The same wallet with enough in it: everybody buys. The reserve is a floor
// under one ladder's remaining depths, so a balance that clears it and the
// entry being placed holds nobody — which is what keeps the gate out of the
// way for all the ticks the wallet is not actually short.
func TestFallenWalletHoldsNobodyWhenItCoversEveryNextEntry(t *testing.T) {
	trades := fallenWallet()
	wallet := walletDepthsOf(trades)

	_, reserve := reserveOf(t, wallet)
	free := tightestWallet(t, trades, reserve)

	for _, trade := range trades {
		assertFreeToArm(t, trade, wallet, free)
	}
}

// Not one close of that wallet is deferred. The close is the event that
// refills the wallet the reserve is measured against, so a gate that parked
// it would deadlock the very situation it exists to resolve.
func TestFallenWalletNeverDefersAClose(t *testing.T) {
	trades := fallenWallet()
	wallet := walletDepthsOf(trades)

	for _, trade := range trades {
		trade.PositionType = "takeProfit"

		free, err := ShouldHold(priorityEvent(trade, "buy", wallet, 0))
		if err != nil {
			t.Fatalf("%s must be free to close, got %v", trade.Symbol, err)
		}
		if len(free.Trade.Logs) != 0 {
			t.Fatalf("%s: no row may be written on a close, got %v", trade.Symbol, messages(free.Trade.Logs))
		}
	}
}

// A trade opened into that wallet spends the same funds, and its first fill
// is priced like any other entry: it waits too, and its row is the wallet
// gate's. The first-fill gate is never consulted, so the verdict it would
// have activated on is left for the tick the wallet can afford the entry on.
func TestFallenWalletHoldsATradeOpenedIntoIt(t *testing.T) {
	newcomer := testutil.LadderDepthTrade(21, "ADA/USDT", 0, scenarioDepths)
	newcomer.PositionType = "buy"

	wallet := walletDepthsOf(fallenWallet())
	priority, reserve := reserveOf(t, wallet)

	held, err := ShouldHold(priorityEvent(newcomer, "new", wallet, walletShortOf(t, newcomer, reserve)))
	if err == nil {
		t.Fatal("a first fill the reserved wallet cannot spare must wait")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}

	row := held.Trade.Logs[0].Message
	want := fmt.Sprintf(
		"Hold entry: cooldown: depth priority, %s at depth %d of %d keeps the wallet for its remaining depths, this ladder waits at depth 0 of %d",
		priorityLadder, priority.Depth, scenarioDepths, scenarioDepths,
	)
	if row != want {
		t.Fatalf("row = %q, want %q", row, want)
	}
	if strings.Contains(row, cooldown.FirstFillWaitingPrefix) {
		t.Fatalf("row = %q, the first-fill gate must not have been consulted", row)
	}
}

// The ladder in front fills its LAST depth. It now keeps nothing, so every
// sibling the wallet can pay for buys — that is the release — but it stays in
// front until it closes, and a sibling the wallet cannot pay for keeps
// waiting on the row that says so.
//
// The waiting is the point. Releasing everybody here sends them all at a
// wallet the ladder has just spent down to its last depth: they reach the
// funds gate, block, and leave the wallet view, so the ladder that finally
// closes hands the wallet to whatever shallow ladder is still active rather
// than to the deepest one.
func TestFallenWalletHoldsOnlyTheUnaffordableWhenTheDeepLadderFills(t *testing.T) {
	trades := fallenWallet()

	finished := make([]aggragates.Trades, 0, len(trades))
	for _, trade := range trades {
		if trade.Symbol == priorityLadder {
			trade = filledTo(trade, scenarioDepths)
		}
		finished = append(finished, trade)
	}

	full := walletDepthsOf(finished)
	stillInFront, reserve := reserveOf(t, full)
	if stillInFront.Symbol != priorityLadder {
		t.Fatalf("the full ladder must stay in front until it closes, got %s", stillInFront.Symbol)
	}
	if reserve != 0 {
		t.Fatalf("a full ladder keeps %f, want nothing", reserve)
	}

	for _, trade := range finished {
		if trade.Symbol == priorityLadder {
			continue
		}

		_, ownCost := ladder.NextEntryCost(trade, 0)

		// A wallet that covers this entry lets it through.
		assertFreeToArm(t, trade, full, ownCost)

		held, err := ShouldHold(priorityEvent(trade, "buy", full, ownCost-1))
		if err == nil {
			t.Fatalf("%s: a wallet that cannot pay for the entry must hold it, not block it", trade.Symbol)
		}

		want := holdsUntilCloseRow(priorityLadder, scenarioDepths, configuredDepthOf(t, trade.Symbol))
		if got := held.Trade.Logs[0].Message; got != want {
			t.Fatalf("%s row = %q, want %q", trade.Symbol, got, want)
		}
	}
}

// And on the close the ladder leaves the view, its position comes back into
// the wallet, and the next deepest takes over — which is the ladder that was
// still active and still ranked because it waited instead of blocking.
func TestFallenWalletIsReleasedWhenTheDeepLadderCloses(t *testing.T) {
	trades := fallenWallet()

	closed := withoutLadder(trades, priorityLadder)
	gone := walletDepthsOf(closed)
	nextInFront, goneReserve := reserveOf(t, gone)
	if nextInFront.Symbol != runnerUpLadder {
		t.Fatalf("the wallet passes to %s, got %s", runnerUpLadder, nextInFront.Symbol)
	}

	proceeds := closedPositionValue(testutil.LadderDepthTrade(14, priorityLadder, configuredDepthOf(t, priorityLadder), scenarioDepths))
	shortBefore := walletShortOf(t, closed[0], goneReserve)
	if shortBefore+proceeds < tightestWallet(t, closed, goneReserve) {
		t.Fatalf("the fixture close must refill the wallet past what the remaining ladders need, got %f", proceeds)
	}

	for _, trade := range closed {
		assertFreeToArm(t, trade, gone, shortBefore+proceeds)
	}
}

// holdsUntilCloseRow is the row a ladder waiting behind a FULL one carries:
// no further entry of that ladder will free the wallet, only its close.
func holdsUntilCloseRow(priority string, priorityDepth, ownDepth int) string {
	return fmt.Sprintf(
		"Hold stopLoss: cooldown: depth priority, %s at depth %d of %d holds the wallet until it closes, this ladder waits at depth %d of %d",
		priority, priorityDepth, scenarioDepths, ownDepth, scenarioDepths,
	)
}

// Once the reserved ladder is gone the wallet is kept for the next deepest
// one — and for ITS remainder, which is a different amount. The row names it,
// so an operator never reads a reservation for a ladder that is no longer
// there.
func TestFallenWalletHandsTheReservationToTheNextDeepestLadder(t *testing.T) {
	trades := withoutLadder(fallenWallet(), priorityLadder)
	wallet := walletDepthsOf(trades)

	priority, reserve := reserveOf(t, wallet)
	if priority.Symbol != runnerUpLadder {
		t.Fatalf("the wallet is kept for %s, want %s", priority.Symbol, runnerUpLadder)
	}

	for _, trade := range trades {
		free := walletShortOf(t, trade, reserve)

		if trade.Symbol == runnerUpLadder {
			assertFreeToArm(t, trade, wallet, free)
			continue
		}

		held, err := ShouldHold(priorityEvent(trade, "buy", wallet, free))
		if err == nil {
			t.Fatalf("%s must now wait for %s", trade.Symbol, runnerUpLadder)
		}

		want := waitingRow(runnerUpLadder, priority.Depth, configuredDepthOf(t, trade.Symbol))
		if got := held.Trade.Logs[0].Message; got != want {
			t.Fatalf("%s row = %q, want %q", trade.Symbol, got, want)
		}
	}
}

// A hold that stands writes one row, not one per tick: the message is
// byte-identical while the ladders are, which is the property
// gates.SaveHoldLog deduplicates on. It is also why the row carries no
// amounts — the balance moves on every tick, and an operator reading a
// multi-day reservation would be reading a wall of near-identical lines.
func TestFallenWalletWritesOneRowWhileTheHoldStands(t *testing.T) {
	wallet := walletDepthsOf(fallenWallet())
	_, reserve := reserveOf(t, wallet)

	waiting := testutil.LadderDepthTrade(12, "ETH/USDT", 4, scenarioDepths)
	free := walletShortOf(t, waiting, reserve)

	held, err := ShouldHold(priorityEvent(waiting, "buy", wallet, free))
	if err == nil {
		t.Fatal("expected the shallow ladder to be held")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}

	// The next tick arms the same depth again, on the same ladders and a
	// balance that has drifted the way a live one does.
	held.Trade.PositionType = "stopLoss"
	again, err := ShouldHold(priorityEvent(held.Trade, "buy", wallet, free-1))
	if err == nil {
		t.Fatal("expected the ladder to still be held on the next tick")
	}
	if len(again.Trade.Logs) != 1 {
		t.Fatalf("a standing hold must not write a row per tick, got %v", messages(again.Trade.Logs))
	}
}
