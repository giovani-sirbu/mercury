package actions

import (
	"fmt"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The amounts the opened rows of this file carry: their own, apart from the
// shipped constants, so a retune moves no expectation — a ladder trades the
// raise its own row names.
const (
	raisedPoints = 0.5
	raisedDepths = 2
)

// raisedLadderOf is a ladder of the fixture wallet at the given depth that
// opened raised: the DynamicParams flag on for a long spot parent and the one
// opened row in its logs, on the same stored rows as every other ladder of
// the wallet.
func raisedLadderOf(id uint, symbol string, depth int) aggragates.Trades {
	trade := testutil.LadderDepthTrade(id, symbol, depth, walletDepths)
	trade.Strategy.Params.DynamicParams = true
	trade.Strategy.TradeType = aggragates.Spot
	trade.Logs = append(trade.Logs, aggragates.TradesLogs{
		Message: dynamicparams.OpenedMessage(raisedPoints, raisedDepths),
		Type:    aggragates.LOG_INFO,
	})

	return trade
}

// raisedWaitingRow is the row the operator reads on a ladder held behind one
// that has depths left: which ladder the wallet is kept for with its ceiling,
// and where this one stands with its own.
func raisedWaitingRow(priority aggragates.LadderDepth, ownDepth, ownCeiling int) string {
	return fmt.Sprintf(
		"Hold stopLoss: "+cooldown.DepthPriorityHoldMarker+", %s at depth %d of %d keeps the wallet for its remaining depths, this ladder waits at depth %d of %d",
		priority.Symbol, priority.Depth, priority.MaxDepth, ownDepth, ownCeiling,
	)
}

// rowsWritten is what a tick added to the trade's logs: the opened row a
// raised ladder carries is not the tick's doing, so it is left out.
func rowsWritten(before, after aggragates.Trades) []aggragates.TradesLogs {
	return after.Logs[len(before.Logs):]
}

// assertRaisedFreeToArm fails when the ladder is held on the tick that arms
// its next entry: a released tick writes nothing beyond the opened row the
// ladder already carries.
func assertRaisedFreeToArm(t *testing.T, trade aggragates.Trades, wallet []aggragates.LadderDepth, free float64) {
	t.Helper()

	released, err := ShouldHold(priorityEvent(trade, "buy", wallet, free))
	if err != nil {
		t.Fatalf("%s must be free to arm its next entry, got %v", trade.Symbol, err)
	}
	if written := rowsWritten(trade, released.Trade); len(written) != 0 {
		t.Fatalf("%s: no row may be written on a released ladder, got %v", trade.Symbol, messages(written))
	}
}

// A sibling's add waits behind a ladder that opened raised and stands at the
// depth its stored rows stop at. That ladder was sized for more depths than
// that, so its reserve is the cost of the ones its opened row added — and a
// sibling that would spend into it is held, with a row that names the raised
// ceiling. The same wallet with the ladder stored at that depth has nothing
// to keep: it is full, and the sibling buys.
func TestShouldHoldKeepsTheWalletForARaisedLadderAtItsStoredCeiling(t *testing.T) {
	keeper := ladder.DepthOf(raisedLadderOf(14, priorityLadder, walletDepths))
	if keeper.MaxDepth != walletDepths+raisedDepths || keeper.RemainingCost <= 0 {
		t.Fatalf("view = %+v, want the raised ladder to have its %d extra depths left to pay for", keeper, raisedDepths)
	}

	sibling := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	wallet := []aggragates.LadderDepth{keeper}

	held, err := ShouldHold(priorityEvent(sibling, "buy", wallet, walletShortOf(t, sibling, keeper.RemainingCost)))
	if err == nil {
		t.Fatal("an add that breaks into the raised ladder's remaining depths must wait")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}
	if want := raisedWaitingRow(keeper, 4, walletDepths); held.Trade.Logs[0].Message != want {
		t.Fatalf("row = %q, want %q", held.Trade.Logs[0].Message, want)
	}

	assertFreeToArm(t, sibling, wallet, walletCovering(t, sibling, keeper.RemainingCost))

	// The same ladder stored at that depth is full and keeps nothing, so the
	// balance that held the sibling above lets it through.
	full := ladder.DepthOf(testutil.LadderDepthTrade(14, priorityLadder, walletDepths, walletDepths))
	assertFreeToArm(t, sibling, []aggragates.LadderDepth{full}, walletShortOf(t, sibling, keeper.RemainingCost))
}

// The reserve ends where the raised ceiling does, not the stored one: a
// raised ladder that has filled the extra depths too is full, keeps nothing,
// and the sibling buys on the balance that covers its own entry.
func TestShouldHoldReleasesARaisedLadderAtItsRaisedCeiling(t *testing.T) {
	full := ladder.DepthOf(raisedLadderOf(14, priorityLadder, walletDepths+raisedDepths))
	if full.Depth != full.MaxDepth || full.RemainingCost != 0 {
		t.Fatalf("view = %+v, want the raised ladder full at its raised ceiling, keeping nothing", full)
	}

	sibling := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	assertFreeToArm(t, sibling, []aggragates.LadderDepth{full}, walletCovering(t, sibling, 0))
}

// The wallet of the five fallen pairs with its deepest ladder standing at the
// depth its stored rows stop at, having opened raised: it stays the one the
// wallet is kept for, and every sibling waits behind it with a row that prints
// the raised ceiling. Read on the stored rows it would be full, reserve
// nothing, and the wallet would be kept for the next ladder in line.
func TestShouldHoldKeepsTheFallenWalletForTheRaisedDeepestLadder(t *testing.T) {
	trades := fallenWallet()
	for index, trade := range trades {
		if trade.Symbol == priorityLadder {
			trades[index] = raisedLadderOf(trade.ID, trade.Symbol, scenarioDepths)
		}
	}

	wallet := walletDepthsOf(trades)
	priority, reserve := reserveOf(t, wallet)
	if priority.Symbol != priorityLadder || priority.MaxDepth != scenarioDepths+raisedDepths || priority.RemainingCost <= 0 {
		t.Fatalf("priority = %+v, want %s in front with its raised depths left", priority, priorityLadder)
	}

	for index, trade := range trades {
		free := walletShortOf(t, trade, reserve)

		if trade.Symbol == priorityLadder {
			assertRaisedFreeToArm(t, trade, wallet, free)
			continue
		}

		held, err := ShouldHold(priorityEvent(trade, "buy", wallet, free))
		if err == nil {
			t.Fatalf("%s at depth %d must wait for %s", trade.Symbol, fallenLadders[index].depth, priorityLadder)
		}
		if len(held.Trade.Logs) != 1 {
			t.Fatalf("%s: expected one row, got %v", trade.Symbol, messages(held.Trade.Logs))
		}
		if want := raisedWaitingRow(priority, fallenLadders[index].depth, scenarioDepths); held.Trade.Logs[0].Message != want {
			t.Fatalf("%s row = %q, want %q", trade.Symbol, held.Trade.Logs[0].Message, want)
		}
	}
}

// The ladder the chain is handed is the stored trade carrying its own logs,
// so the trade being asked about reads its raise as the wallet view does.
// Two ladders that opened raised and stand level at the stored ceiling are
// split by their trade ids: the higher one waits behind the lower one, which
// still has depths left, and the lower one is held by nobody.
func TestShouldHoldRanksARaisedLadderBeingAskedAboutOnItsRaisedDepths(t *testing.T) {
	ahead := raisedLadderOf(13, "SOL/USDT", walletDepths)
	own := raisedLadderOf(14, priorityLadder, walletDepths)
	wallet := walletDepthsOf([]aggragates.Trades{ahead, own})

	aheadView := wallet[0]
	if aheadView.Depth >= aheadView.MaxDepth {
		t.Fatalf("view = %+v, want a raised ladder with depths left", aheadView)
	}

	held, err := ShouldHold(priorityEvent(own, "buy", wallet, walletShortOf(t, own, aheadView.RemainingCost)))
	if err == nil {
		t.Fatal("the higher id must wait behind the lower one, which has raised depths left")
	}
	written := rowsWritten(own, held.Trade)
	if len(written) != 1 {
		t.Fatalf("expected one row, got %v", messages(written))
	}
	if want := raisedWaitingRow(aheadView, walletDepths, walletDepths+raisedDepths); written[0].Message != want {
		t.Fatalf("row = %q, want %q", written[0].Message, want)
	}

	assertRaisedFreeToArm(t, ahead, wallet, walletShortOf(t, ahead, aheadView.RemainingCost))
}
