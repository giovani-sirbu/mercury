package actions

import (
	"fmt"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The amounts the opened events of this file carry: their own, apart from the
// shipped constants, so a retune moves no expectation.
const (
	raisedPoints = 0.5
	raisedDepths = 2
)

// raisedLadderOf is a ladder of the fixture wallet at the given depth that
// opened raised: the flag on for a long spot parent and the opened pair the
// engine's own writer appends, on the stored rows of every other ladder.
func raisedLadderOf(id uint, symbol string, depth int) aggragates.Trades {
	trade := testutil.LadderDepthTrade(id, symbol, depth, walletDepths)
	trade.Strategy.Params.DynamicParams = true
	trade.Strategy.TradeType = aggragates.Spot
	row, event := dynamicparams.Opened{Points: raisedPoints, Depths: raisedDepths}.Rows(trade, trade.PositionPrice, time.Time{})

	return aggragates.AppendStrategyRow(trade, row, event)
}

// raisedWaitingRow is the row the operator reads on a ladder held behind one
// that has depths left: the ladder the wallet is kept for, and this one's own.
func raisedWaitingRow(priority aggragates.LadderDepth, ownDepth, ownCeiling int) string {
	return fmt.Sprintf(
		"Hold stopLoss: "+cooldown.DepthPriorityHoldMarker+", %s at depth %d of %d keeps the wallet for its remaining depths, this ladder waits at depth %d of %d",
		priority.Symbol, priority.Depth, priority.MaxDepth, ownDepth, ownCeiling,
	)
}

// rowsWritten is what a tick added to the trade's logs: the opened row a raised
// ladder carries is not the tick's doing, so it is left out.
func rowsWritten(before, after aggragates.Trades) []aggragates.TradesLogs {
	return after.Logs[len(before.Logs):]
}

// assertRaisedFreeToArm fails when the ladder is held on the tick that arms its
// next entry: a released tick writes nothing beyond the opened pair it carries.
func assertRaisedFreeToArm(t *testing.T, trade aggragates.Trades, wallet []aggragates.LadderDepth, free float64) {
	t.Helper()

	released, err := ShouldHold(priorityEvent(trade, "buy", wallet, free))
	if err != nil {
		t.Fatalf("%s must be free to arm its next entry, got %v", trade.Symbol, err)
	}
	written := rowsWritten(trade, released.Trade)
	extra := len(released.Trade.StrategyEvents) - len(trade.StrategyEvents)
	if len(written) != 0 || extra != 0 {
		t.Fatalf("%s: no row and no event may be written on a released ladder, got %v and %d events", trade.Symbol, messages(written), extra)
	}
}

// A sibling's add waits behind a ladder that opened raised and stands at the
// depth its stored rows stop at: its reserve is the cost of the depths its
// opened event added, and a sibling that would spend into it is held, with a row
// that names the raised ceiling. With the ladder stored at that depth it is
// full, and the sibling buys.
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
	assertNewestPair(t, held.Trade, aggragates.StrategyParamCooldown, cooldown.GateDepthPriority, gates.EventHeld)

	assertFreeToArm(t, sibling, wallet, walletCovering(t, sibling, keeper.RemainingCost))

	// The same ladder stored at that depth is full and keeps nothing, so the
	// balance that held the sibling above lets it through.
	full := ladder.DepthOf(testutil.LadderDepthTrade(14, priorityLadder, walletDepths, walletDepths))
	assertFreeToArm(t, sibling, []aggragates.LadderDepth{full}, walletShortOf(t, sibling, keeper.RemainingCost))
}

// The reserve ends where the raised ceiling does, not the stored one: a raised
// ladder that filled the extra depths too is full and the sibling buys.
func TestShouldHoldReleasesARaisedLadderAtItsRaisedCeiling(t *testing.T) {
	full := ladder.DepthOf(raisedLadderOf(14, priorityLadder, walletDepths+raisedDepths))
	if full.Depth != full.MaxDepth || full.RemainingCost != 0 {
		t.Fatalf("view = %+v, want the raised ladder full at its raised ceiling, keeping nothing", full)
	}

	sibling := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	assertFreeToArm(t, sibling, []aggragates.LadderDepth{full}, walletCovering(t, sibling, 0))
}

// The wallet of the five fallen pairs with its deepest ladder at the depth its
// stored rows stop at, having opened raised: it stays the one the wallet is kept
// for and every sibling waits behind it, with a row that prints the raised
// ceiling. On the stored rows it would be full and reserve nothing.
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

// The trade the chain is handed carries its own events, so the ladder asked
// about reads its raise as the wallet view does. Two ladders that opened raised
// and stand level at the stored ceiling split on trade id: the higher waits
// behind the lower, which still has depths left, and the lower is held by nobody.
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

// The opened event is the raise and the row beside it only text: a ladder at its
// stored ceiling whose logs carry the opened row but whose events do not is full
// and keeps nothing, so the sibling the raised ladder holds buys.
func TestShouldHoldReadsTheRaisedLadderFromItsEventNeverItsRow(t *testing.T) {
	raised := raisedLadderOf(14, priorityLadder, walletDepths)
	textOnly := raised
	textOnly.StrategyEvents = nil
	keeper, full := ladder.DepthOf(raised), ladder.DepthOf(textOnly)
	if len(textOnly.Logs) != 1 || full.MaxDepth != walletDepths || full.RemainingCost != 0 {
		t.Fatalf("view = %+v, want the opened row alone to leave the ladder full and reserving nothing", full)
	}

	sibling := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	short := walletShortOf(t, sibling, keeper.RemainingCost)
	if _, err := ShouldHold(priorityEvent(sibling, "buy", []aggragates.LadderDepth{keeper}, short)); err == nil {
		t.Fatal("fixture drifted: the raised ladder must hold the sibling on this wallet")
	}

	assertFreeToArm(t, sibling, []aggragates.LadderDepth{full}, short)
}
