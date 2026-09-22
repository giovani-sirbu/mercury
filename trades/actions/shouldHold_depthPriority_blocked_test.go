package actions

import (
	"fmt"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The wallet as it stood on the run that sent this rule back: five pairs that
// fell together on grids of nine depths, the deepest ladder one entry short of
// its last depth and then funds-blocked on exactly that entry. The wallet went
// on being kept for an entry that ladder could not take, so the four shallower
// ladders waited with free money sitting in the wallet — the starvation the
// gate exists to prevent, caused by the gate itself.
//
// The fix is membership and it lives in the engines: a ladder blocked on its
// next entry is not in the view they hand the tick. What this file pins is the
// behaviour that follows from it, end to end through ShouldHold — the hold
// while the deep ladder is active, the handover the moment it leaves the view,
// the reservation again when it is re-admitted, and the one thing membership
// must never decide: the managed trade's OWN depth and cost, which are read
// from the trade it is asked about and not from the view it is compared
// against.

// blockedRunDepths is the grid every pair of that wallet was configured for.
const blockedRunDepths = 9

// reservedSymbol is the deepest ladder: the one the wallet is kept for while
// it is in the view.
const reservedSymbol = "LINK/USDT"

// runnerUpSymbol is the next deepest, which takes the reservation over the
// moment the deepest one leaves.
const runnerUpSymbol = "HBAR/USDT"

// blockedRunLadders is the observed shape, deepest first.
var blockedRunLadders = []struct {
	id     uint
	symbol string
	depth  int
}{
	{14, reservedSymbol, 8},
	{15, runnerUpSymbol, 7},
	{13, "SOL/USDT", 6},
	{12, "ETH/USDT", 5},
	{11, "BTC/USDT", 4},
}

// requireObservedShape keeps the file honest about what it is written
// against: the gate has to be switched on, and the wallet has to have exactly
// one deepest ladder and one runner-up, since every expectation below names
// them.
func requireObservedShape(t *testing.T) {
	t.Helper()

	if !cooldown.DepthPriority {
		t.Skip("cooldown.DepthPriority is off: the wallet reserves nothing")
	}

	deepest, runnerUp := blockedRunLadders[0], blockedRunLadders[1]
	if deepest.symbol != reservedSymbol || runnerUp.symbol != runnerUpSymbol {
		t.Fatalf("the observed wallet must lead with %s then %s", reservedSymbol, runnerUpSymbol)
	}
	if deepest.depth <= runnerUp.depth || deepest.depth >= blockedRunDepths {
		t.Fatalf("%s must be the deepest ladder and still short of its ceiling", reservedSymbol)
	}
}

// blockedRunTrades builds the five ladders as real trades, so the depths and
// the costs the gate reads are taken off the ladders by the helper every
// engine builds its view with rather than written out beside them.
func blockedRunTrades() []aggragates.Trades {
	trades := make([]aggragates.Trades, 0, len(blockedRunLadders))
	for _, entry := range blockedRunLadders {
		trades = append(trades, testutil.LadderDepthTrade(entry.id, entry.symbol, entry.depth, blockedRunDepths))
	}
	return trades
}

// blockedRunView is the view an engine builds from those trades.
func blockedRunView(trades []aggragates.Trades) []aggragates.LadderDepth {
	view := make([]aggragates.LadderDepth, 0, len(trades))
	for _, trade := range trades {
		view = append(view, ladder.DepthOf(trade))
	}
	return view
}

// blockedRunViewWithout is the view the engines build once that ladder blocks
// on its next entry: the trade is untouched and still ticking, it is only out
// of the wallet the gate reads.
func blockedRunViewWithout(trades []aggragates.Trades, symbol string) []aggragates.LadderDepth {
	view := make([]aggragates.LadderDepth, 0, len(trades))
	for _, trade := range trades {
		if trade.Symbol == symbol {
			continue
		}
		view = append(view, ladder.DepthOf(trade))
	}
	return view
}

// blockedRunWithout is the same removal on the trades themselves.
func blockedRunWithout(trades []aggragates.Trades, symbol string) []aggragates.Trades {
	remaining := make([]aggragates.Trades, 0, len(trades))
	for _, trade := range trades {
		if trade.Symbol == symbol {
			continue
		}
		remaining = append(remaining, trade)
	}
	return remaining
}

// blockedRunDepthOf is the depth the observed wallet puts a pair at, read back
// from the fixture so an expectation cannot drift from the wallet it describes.
func blockedRunDepthOf(t *testing.T, symbol string) int {
	t.Helper()

	for _, entry := range blockedRunLadders {
		if entry.symbol == symbol {
			return entry.depth
		}
	}

	t.Fatalf("%s is not a ladder of the observed wallet", symbol)
	return 0
}

// keepingRow is the row a waiting ladder of this wallet carries.
func keepingRow(t *testing.T, priority string, ownDepth int) string {
	t.Helper()

	return fmt.Sprintf(
		"Hold stopLoss: cooldown: depth priority, %s at depth %d of %d keeps the wallet for its remaining depths, this ladder waits at depth %d of %d",
		priority, blockedRunDepthOf(t, priority), blockedRunDepths, ownDepth, blockedRunDepths,
	)
}

// Phase one, the situation the run was in: the deepest ladder is active, so
// what its remaining depths cost is kept for it and every shallower ladder of
// the wallet waits, each row naming the ladder that is being waited for and
// where this one stands.
func TestDepthPriorityHoldsTheShallowLaddersWhileTheDeepOneIsActive(t *testing.T) {
	requireObservedShape(t)

	trades := blockedRunTrades()
	reserved := blockedRunView(trades)
	_, reserve := reserveOf(t, reserved)

	for _, trade := range trades {
		free := walletShortOf(t, trade, reserve)

		if trade.Symbol == reservedSymbol {
			assertFreeToArm(t, trade, reserved, free)
			continue
		}

		held, err := ShouldHold(priorityEvent(trade, "buy", reserved, free))
		if err == nil {
			t.Fatalf("%s must wait for %s", trade.Symbol, reservedSymbol)
		}
		if len(held.Trade.Logs) != 1 {
			t.Fatalf("%s: expected one row, got %v", trade.Symbol, messages(held.Trade.Logs))
		}

		want := keepingRow(t, reservedSymbol, blockedRunDepthOf(t, trade.Symbol))
		if got := held.Trade.Logs[0].Message; got != want {
			t.Fatalf("%s row = %q, want %q", trade.Symbol, got, want)
		}
	}
}

// Phase two, the fix: the deep ladder blocks on the very entry the wallet was
// being kept for and the engines drop it from the view. Nothing is kept for it
// any more — the reservation is the runner-up's own remainder from that tick
// on, and a wallet that carries that lets every ladder arm, with no row
// written on the way through and nothing anywhere having to un-hold anything.
func TestDepthPriorityStopsKeepingTheWalletForABlockedLadder(t *testing.T) {
	requireObservedShape(t)

	trades := blockedRunTrades()
	reserved := blockedRunView(trades)
	released := blockedRunViewWithout(trades, reservedSymbol)

	_, reserve := reserveOf(t, reserved)
	stillGoing, handedOn := reserveOf(t, released)
	if stillGoing.Symbol != runnerUpSymbol {
		t.Fatalf("the wallet is now kept for %s, want %s", stillGoing.Symbol, runnerUpSymbol)
	}

	remaining := blockedRunWithout(trades, reservedSymbol)
	free := tightestWallet(t, remaining, handedOn)

	for _, trade := range remaining {
		held, err := ShouldHold(priorityEvent(trade, "buy", reserved, walletShortOf(t, trade, reserve)))
		if err == nil {
			t.Fatalf("%s must wait while %s is in the view", trade.Symbol, reservedSymbol)
		}
		if len(held.Trade.Logs) != 1 {
			t.Fatalf("%s: expected one row, got %v", trade.Symbol, messages(held.Trade.Logs))
		}

		// The same trade on the next tick that arms a depth: the hold row it
		// is carrying is the one written above, and SaveHoldLog put the old
		// position back when it wrote it.
		waiting := held.Trade
		waiting.PositionType = "stopLoss"

		armed, err := ShouldHold(priorityEvent(waiting, "buy", released, free))
		if err != nil {
			t.Fatalf("%s must arm its next entry once the blocked ladder is out of the wallet, got %v", trade.Symbol, err)
		}
		if len(armed.Trade.Logs) != 1 {
			t.Fatalf("%s: a released ladder writes no row, got %v", trade.Symbol, messages(armed.Trade.Logs))
		}
	}
}

// Phase three: the wallet can afford that entry again, the engines re-admit
// the ladder, and it keeps the wallet exactly as before — same reservation,
// byte-identical row, which is what keeps a reservation that comes and goes
// from filling the log with one line per tick.
func TestDepthPriorityReservesTheWalletAgainWhenTheLadderIsReAdmitted(t *testing.T) {
	requireObservedShape(t)

	trades := blockedRunTrades()
	reserved := blockedRunView(trades)
	released := blockedRunViewWithout(trades, reservedSymbol)

	_, reserve := reserveOf(t, reserved)
	_, handedOn := reserveOf(t, released)

	for _, trade := range blockedRunWithout(trades, reservedSymbol) {
		short := walletShortOf(t, trade, reserve)

		held, err := ShouldHold(priorityEvent(trade, "buy", reserved, short))
		if err == nil {
			t.Fatalf("%s must wait while %s is in the view", trade.Symbol, reservedSymbol)
		}
		firstRow := held.Trade.Logs[0].Message

		waiting := held.Trade
		waiting.PositionType = "stopLoss"
		rich := tightestWallet(t, blockedRunWithout(trades, reservedSymbol), handedOn)
		if _, err := ShouldHold(priorityEvent(waiting, "buy", released, rich)); err != nil {
			t.Fatalf("%s must be released while the blocked ladder is out of the wallet, got %v", trade.Symbol, err)
		}

		// Re-admitted: the ladder is active again at the depth it blocked on,
		// so the wallet is kept for it again.
		reHeld, err := ShouldHold(priorityEvent(waiting, "buy", reserved, short))
		if err == nil {
			t.Fatalf("%s must wait again once %s is back in the wallet", trade.Symbol, reservedSymbol)
		}
		if len(reHeld.Trade.Logs) != 1 {
			t.Fatalf("%s: the standing row must not be written per tick, got %v", trade.Symbol, messages(reHeld.Trade.Logs))
		}

		// A ladder that was not holding the row yet writes it on the tick the
		// reservation comes back, and it is the same row to the byte — the
		// balance it was decided on does not appear in it.
		fresh := testutil.LadderDepthTrade(trade.ID, trade.Symbol, blockedRunDepthOf(t, trade.Symbol), blockedRunDepths)
		freshlyHeld, err := ShouldHold(priorityEvent(fresh, "buy", reserved, short-1))
		if err == nil {
			t.Fatalf("%s must wait again once %s is back in the wallet", trade.Symbol, reservedSymbol)
		}
		if len(freshlyHeld.Trade.Logs) != 1 {
			t.Fatalf("%s: expected one row, got %v", trade.Symbol, messages(freshlyHeld.Trade.Logs))
		}
		if got := freshlyHeld.Trade.Logs[0].Message; got != firstRow {
			t.Fatalf("%s row after the ladder came back = %q, want %q", trade.Symbol, got, firstRow)
		}
	}
}

// Membership decides who KEEPS the wallet, never who waits for it. A ladder
// the engines dropped because it is blocked on its next entry is still held by
// a deeper active one, because the gate reads the managed trade's own depth
// and its own next entry from the trade it is asked about — the view it is
// compared against is the view that trade is no longer in.
func TestDepthPriorityReadsOwnDepthAndCostFromTheTradeNotTheView(t *testing.T) {
	requireObservedShape(t)

	ownDepth := blockedRunDepthOf(t, runnerUpSymbol)
	blocked := testutil.LadderDepthTrade(15, runnerUpSymbol, ownDepth, blockedRunDepths)
	blocked.Status = aggragates.Blocked

	// The wallet the engines built on that tick: the blocked ladder is out of
	// it, the deeper active one is in it.
	reserved := []aggragates.LadderDepth{
		ladder.DepthOf(testutil.LadderDepthTrade(14, reservedSymbol, blockedRunDepthOf(t, reservedSymbol), blockedRunDepths)),
	}
	_, reserve := reserveOf(t, reserved)

	held, err := ShouldHold(priorityEvent(blocked, "buy", reserved, walletShortOf(t, blocked, reserve)))
	if err == nil {
		t.Fatal("a re-admitted blocked ladder must still wait for a deeper active one")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}

	want := keepingRow(t, reservedSymbol, ownDepth)
	if got := held.Trade.Logs[0].Message; got != want {
		t.Fatalf("row = %q, want %q", got, want)
	}
}

// A wallet of ladders that cannot stand in front of it — the ones that have
// filled nothing — keeps nothing at all, however empty it is.
func TestDepthPriorityKeepsNothingForAWalletOfUnstartedLadders(t *testing.T) {
	requireObservedShape(t)

	wallet := []aggragates.LadderDepth{
		ladder.DepthOf(testutil.LadderDepthTrade(14, reservedSymbol, 0, blockedRunDepths)),
		ladder.DepthOf(testutil.LadderDepthTrade(13, "SOL/USDT", 0, blockedRunDepths)),
	}

	for _, trade := range blockedRunWithout(blockedRunTrades(), reservedSymbol) {
		assertFreeToArm(t, trade, wallet, 0)
	}
}

// The ladder at its ceiling is the one case a shallower sibling can still be
// waiting on, and it waits on the affordability of its OWN entry alone: the
// full ladder keeps nothing, so the wallet either covers the entry or it does
// not. The sibling that cannot afford it is held — active, in the view, still
// ranked — instead of blocked out of the wallet the close is about to refill.
func TestDepthPriorityHoldsAnUnaffordableEntryBehindAFullLadder(t *testing.T) {
	requireObservedShape(t)

	wallet := []aggragates.LadderDepth{
		ladder.DepthOf(testutil.LadderDepthTrade(14, reservedSymbol, blockedRunDepths, blockedRunDepths)),
	}

	for _, trade := range blockedRunWithout(blockedRunTrades(), reservedSymbol) {
		_, ownCost := ladder.NextEntryCost(trade, 0)

		assertFreeToArm(t, trade, wallet, ownCost)

		held, err := ShouldHold(priorityEvent(trade, "buy", wallet, ownCost-1))
		if err == nil {
			t.Fatalf("%s: an entry the wallet cannot pay for must be held, not passed to the funds gate", trade.Symbol)
		}
		if len(held.Trade.Logs) != 1 {
			t.Fatalf("%s: expected one row, got %v", trade.Symbol, messages(held.Trade.Logs))
		}

		want := fmt.Sprintf(
			"Hold stopLoss: cooldown: depth priority, %s at depth %d of %d holds the wallet until it closes, this ladder waits at depth %d of %d",
			reservedSymbol, blockedRunDepths, blockedRunDepths, blockedRunDepthOf(t, trade.Symbol), blockedRunDepths,
		)
		if got := held.Trade.Logs[0].Message; got != want {
			t.Fatalf("%s row = %q, want %q", trade.Symbol, got, want)
		}
	}
}

// A blocked ladder is out of the view, and this is the tick it is re-admitted
// on: it is the deepest ladder of the wallet and must NOT be parked behind
// the shallower one that stood in for it while it was gone. The managed trade
// knows its own depth; the view is what the others see.
func TestDepthPriorityDoesNotParkAReadmittedDeepestLadderBehindAShallowerOne(t *testing.T) {
	requireObservedShape(t)

	deepest := testutil.LadderDepthTrade(14, reservedSymbol, blockedRunDepthOf(t, reservedSymbol), blockedRunDepths)
	deepest.Status = aggragates.Blocked

	shallower := blockedRunViewWithout(blockedRunTrades(), reservedSymbol)

	assertFreeToArm(t, deepest, shallower, 0)
}
