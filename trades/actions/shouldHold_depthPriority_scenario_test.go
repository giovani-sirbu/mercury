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
// number of depths, and one of them short of its last entry.
//
// Everything here is built as real trades and mapped into the view through
// ladder.DepthOf — the helper every engine builds its own view with — so the
// depths the gate reads are counted off the ladders instead of written out
// beside them. A view written by hand could agree with an engine that counts
// differently; this one cannot.
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

// priorityLadder is the one ladder of the wallet inside the margin of its last
// depth, so the wallet is reserved for its next entry.
const priorityLadder = "LINK/USDT"

// fallenWallet builds the five ladders as trades.
func fallenWallet() []aggragates.Trades {
	trades := make([]aggragates.Trades, 0, len(fallenLadders))
	for _, ladderOf := range fallenLadders {
		trades = append(trades, testutil.LadderDepthTrade(ladderOf.id, ladderOf.symbol, ladderOf.depth, scenarioDepths))
	}
	return trades
}

// walletDepthsOf is the view an engine hands the tick: every ladder of the
// wallet, counted by the one helper all four surfaces share.
func walletDepthsOf(trades []aggragates.Trades) []aggragates.LadderDepth {
	wallet := make([]aggragates.LadderDepth, 0, len(trades))
	for _, trade := range trades {
		wallet = append(wallet, ladder.DepthOf(trade))
	}
	return wallet
}

// withoutLadder is the wallet a close leaves behind: the trade is gone from
// the view altogether, which is what releases the ladders it was holding
// without anything having to remember that they were held.
func withoutLadder(trades []aggragates.Trades, symbol string) []aggragates.Trades {
	remaining := make([]aggragates.Trades, 0, len(trades))
	for _, trade := range trades {
		if trade.Symbol != symbol {
			remaining = append(remaining, trade)
		}
	}
	return remaining
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
// wallet is reserved for, and where this one stands.
func waitingRow(priority string, priorityDepth, ownDepth int) string {
	return fmt.Sprintf(
		"Hold stopLoss: cooldown: depth priority, %s at depth %d of %d takes the next entry, this ladder waits at depth %d of %d",
		priority, priorityDepth, scenarioDepths, ownDepth, scenarioDepths,
	)
}

// assertFreeToArm fails when the ladder is held on the tick that arms its next
// entry.
func assertFreeToArm(t *testing.T, trade aggragates.Trades, wallet []aggragates.LadderDepth) {
	t.Helper()

	released, err := ShouldHold(priorityEvent(trade, "buy", wallet))
	if err != nil {
		t.Fatalf("%s must be free to arm its next entry, got %v", trade.Symbol, err)
	}
	if len(released.Trade.Logs) != 0 {
		t.Fatalf("%s: no row may be written on a released ladder, got %v", trade.Symbol, messages(released.Trade.Logs))
	}
}

// The user's own example: BTC 5, ETH 4, SOL 6, LINK 7, HBAR 2 of eight. LINK
// is the only ladder past the margin, so it takes the next entry and the four
// shallower ones wait, each row naming both sides.
func TestFallenWalletReservesTheNextEntryForTheDeepestLadder(t *testing.T) {
	trades := fallenWallet()
	wallet := walletDepthsOf(trades)

	for index, trade := range trades {
		if trade.Symbol == priorityLadder {
			assertFreeToArm(t, trade, wallet)
			continue
		}

		held, err := ShouldHold(priorityEvent(trade, "buy", wallet))
		if err == nil {
			t.Fatalf("%s at depth %d must wait for %s", trade.Symbol, fallenLadders[index].depth, priorityLadder)
		}
		if len(held.Trade.Logs) != 1 {
			t.Fatalf("%s: expected one row, got %v", trade.Symbol, messages(held.Trade.Logs))
		}

		want := waitingRow(priorityLadder, 7, fallenLadders[index].depth)
		if got := held.Trade.Logs[0].Message; got != want {
			t.Fatalf("%s row = %q, want %q", trade.Symbol, got, want)
		}
		if held.Trade.Logs[0].Type != aggragates.LOG_INFO {
			t.Errorf("%s row type = %q, want %q", trade.Symbol, held.Trade.Logs[0].Type, aggragates.LOG_INFO)
		}
	}
}

// Not one close of that wallet is deferred. The close is the event that frees
// the funds the deep ladder is waiting for, so a gate that parked it would
// deadlock the very situation it exists to resolve.
func TestFallenWalletNeverDefersAClose(t *testing.T) {
	trades := fallenWallet()
	wallet := walletDepthsOf(trades)

	for _, trade := range trades {
		trade.PositionType = "takeProfit"

		free, err := ShouldHold(priorityEvent(trade, "buy", wallet))
		if err != nil {
			t.Fatalf("%s must be free to close, got %v", trade.Symbol, err)
		}
		if len(free.Trade.Logs) != 0 {
			t.Fatalf("%s: no row may be written on a close, got %v", trade.Symbol, messages(free.Trade.Logs))
		}
	}
}

// A trade opened into that wallet has the smallest depth of all, and its first
// fill spends the same funds: it waits too, and its row is the wallet gate's.
// The first-fill gate is never consulted, so the verdict it would have
// activated on is left for the tick the wallet is free on.
func TestFallenWalletHoldsATradeOpenedIntoIt(t *testing.T) {
	newcomer := testutil.LadderDepthTrade(21, "ADA/USDT", 0, scenarioDepths)
	newcomer.PositionType = "buy"
	newcomer.PositionPrice = 100

	held, err := ShouldHold(priorityEvent(newcomer, "new", walletDepthsOf(fallenWallet())))
	if err == nil {
		t.Fatal("a first fill into a reserved wallet must wait")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}

	row := held.Trade.Logs[0].Message
	want := "Hold entry: cooldown: depth priority, LINK/USDT at depth 7 of 8 takes the next entry, this ladder waits at depth 0 of 8"
	if row != want {
		t.Fatalf("row = %q, want %q", row, want)
	}
	if strings.Contains(row, cooldown.FirstFillWaitingPrefix) {
		t.Fatalf("row = %q, the first-fill gate must not have been consulted", row)
	}
}

// The release the user asked for, both ways round: the reserved ladder fills
// its last entry, or it closes and leaves the view. Either way every ladder
// that was waiting is free on the next tick, with nothing to un-hold.
func TestFallenWalletIsReleasedWhenTheDeepLadderFinishesOrCloses(t *testing.T) {
	trades := fallenWallet()

	finished := make([]aggragates.Trades, 0, len(trades))
	for _, trade := range trades {
		if trade.Symbol == priorityLadder {
			trade = filledTo(trade, scenarioDepths)
		}
		finished = append(finished, trade)
	}

	full := walletDepthsOf(finished)
	for _, trade := range finished {
		assertFreeToArm(t, trade, full)
	}

	closed := withoutLadder(trades, priorityLadder)
	gone := walletDepthsOf(closed)
	for _, trade := range closed {
		assertFreeToArm(t, trade, gone)
	}
}

// Once the reserved ladder is gone the wallet is not reserved for the next
// deepest one until that one is itself inside the margin: SOL at six of eight
// releases everybody, and the entry that takes it to seven reserves the wallet
// in LINK's place.
func TestFallenWalletHandsTheReservationOnWhenTheNextLadderPassesTheMargin(t *testing.T) {
	trades := withoutLadder(fallenWallet(), priorityLadder)

	deepened := make([]aggragates.Trades, 0, len(trades))
	for _, trade := range trades {
		if trade.Symbol == "SOL/USDT" {
			trade = filledTo(trade, 7)
		}
		deepened = append(deepened, trade)
	}
	wallet := walletDepthsOf(deepened)

	for _, trade := range deepened {
		if trade.Symbol == "SOL/USDT" {
			assertFreeToArm(t, trade, wallet)
			continue
		}

		held, err := ShouldHold(priorityEvent(trade, "buy", wallet))
		if err == nil {
			t.Fatalf("%s must now wait for SOL/USDT", trade.Symbol)
		}

		want := waitingRow("SOL/USDT", 7, configuredDepthOf(t, trade.Symbol))
		if got := held.Trade.Logs[0].Message; got != want {
			t.Fatalf("%s row = %q, want %q", trade.Symbol, got, want)
		}
	}
}

// A hold that stands writes one row, not one per tick: the message is
// byte-identical while the wallet view is, which is the property
// gates.SaveHoldLog deduplicates on. Without it an operator reading a
// multi-day reservation would be reading a wall of identical lines.
func TestFallenWalletWritesOneRowWhileTheHoldStands(t *testing.T) {
	wallet := walletDepthsOf(fallenWallet())
	waiting := testutil.LadderDepthTrade(12, "ETH/USDT", 4, scenarioDepths)

	held, err := ShouldHold(priorityEvent(waiting, "buy", wallet))
	if err == nil {
		t.Fatal("expected the shallow ladder to be held")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}

	// The next tick arms the same depth again on an unchanged wallet.
	held.Trade.PositionType = "stopLoss"
	again, err := ShouldHold(priorityEvent(held.Trade, "buy", wallet))
	if err == nil {
		t.Fatal("expected the ladder to still be held on the next tick")
	}
	if len(again.Trade.Logs) != 1 {
		t.Fatalf("a standing hold must not write a row per tick, got %v", messages(again.Trade.Logs))
	}
}
