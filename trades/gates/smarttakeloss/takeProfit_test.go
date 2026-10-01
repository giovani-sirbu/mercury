package smarttakeloss

import (
	"math"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// breakEvenReading is the move against the average entry price at break
// even: zero, the lowest move the helper raises. A test hands it where only
// the trade decides the answer — a trade the helper leaves alone hands it
// back bit for bit, a pending one reads its newest fill — so the break-even
// guard never answers in its place.
const breakEvenReading = 0.0

// takeProfitStep is the move the `buy` row arms the take profit at on the
// fixture's row: percentage + tolerance.
func takeProfitStep(trade aggragates.Trades) float64 {
	row := trade.StrategyPair.StrategySettings[0]
	return row.Percentage + row.Tolerance
}

// moveAgainst is the strategy's own metric: the move of price against an
// anchor, over the price.
func moveAgainst(price, anchor float64) float64 {
	return (price - anchor) / price * 100
}

// betweenTheTakeProfits is a price a pending fixture arms its take profit at
// from its newest fill and not from its average entry price: the middle of
// the lowest price the newest fill arms it at — its own take profit, or break
// even where that sits higher — and the average entry price's take profit.
func betweenTheTakeProfits(t *testing.T, trade aggragates.Trades) float64 {
	t.Helper()
	step := takeProfitStep(trade) / 100
	average := ladder.AverageEntryPrice(trade)
	fromNewestFill := math.Max(slowDeclineLastFill/(1-step), average)
	fromAverage := average / (1 - step)
	if !(fromNewestFill < fromAverage) {
		t.Fatal("fixture drifted: the newest fill must arm the take profit under the average entry price")
	}
	return (fromNewestFill + fromAverage) / 2
}

// A ladder without the marker reads its input unchanged at every price, at
// or over break even included: it is watched, not pending.
func TestTakeProfitPercentageLeavesATradeWithoutTheMarkerAlone(t *testing.T) {
	trade := watchedTrade()
	if st := rebuildState(trade); !st.slowDeclineWatched || st.slowDeclinePending {
		t.Fatal("fixture drifted: the ladder must be watched and not pending")
	}
	for _, price := range []float64{underTheBand, slowDeclineBand, betweenTheTakeProfits(t, pendingTrade()), 2 * slowDeclineBand} {
		if got := TakeProfitPercentage(trade, price, breakEvenReading); got != breakEvenReading {
			t.Fatalf("at %v a ladder without the marker must read its input, got %v", price, got)
		}
	}
}

// A pending trade at or over break even reads the larger of the two moves.
// Between the lowest price its newest fill arms the take profit at and the
// average entry price's take profit, the move against the newest fill is past
// the take profit and the move against the average entry price is not, so the
// take profit arms at the lower of the two prices; an input larger than the
// newest fill's move comes back as it is.
func TestTakeProfitPercentageReadsTheLargerMoveOnAPendingTrade(t *testing.T) {
	trade := pendingTrade()
	price := betweenTheTakeProfits(t, trade)
	step := takeProfitStep(trade)

	fromAverage := moveAgainst(price, ladder.AverageEntryPrice(trade))
	fromNewestFill := moveAgainst(price, slowDeclineLastFill)
	if fromAverage < 0 || fromAverage >= step || fromNewestFill < step {
		t.Fatalf("fixture drifted: at %v the price must sit at or over break even, the newest fill past the take profit (%v) and the average entry price short of it (%v)", price, fromNewestFill, fromAverage)
	}
	if got := TakeProfitPercentage(trade, price, fromAverage); got != fromNewestFill {
		t.Fatalf("a pending trade must read the move against its newest fill %v, got %v", fromNewestFill, got)
	}
	larger := fromNewestFill + step
	if got := TakeProfitPercentage(trade, price, larger); got != larger {
		t.Fatalf("an input larger than the newest fill's move must come back, got %v, want %v", got, larger)
	}
}

// The newest fill is read from break even up. One float step under break even
// the move against the average entry price is negative and comes back
// unchanged, exactly as on the same ladder without the marker: a take profit
// there could only be refused. At break even and one float step over it the
// pending trade reads its newest fill and the ladder without the marker its
// input. The fixture's newest fill arms its take profit under break even, so
// break even is where the pending trade's take profit arms.
func TestTakeProfitPercentageReadsTheNewestFillFromBreakEvenUp(t *testing.T) {
	pending, plain := pendingTrade(), watchedTrade()
	breakEven := ladder.AverageEntryPrice(pending)
	step := takeProfitStep(pending)
	if moveAgainst(breakEven, slowDeclineLastFill) < step {
		t.Fatal("fixture drifted: the newest fill's take profit must sit under break even")
	}

	under := math.Nextafter(breakEven, 0)
	fromAverage := moveAgainst(under, breakEven)
	if fromAverage >= 0 {
		t.Fatalf("fixture drifted: one float step under break even the move must be negative, got %v", fromAverage)
	}
	for name, trade := range map[string]aggragates.Trades{"pending": pending, "without the marker": plain} {
		if got := TakeProfitPercentage(trade, under, fromAverage); got != fromAverage {
			t.Errorf("%s: under break even the input must come back, got %v, want %v", name, got, fromAverage)
		}
	}

	for name, price := range map[string]float64{"at break even": breakEven, "over break even": math.Nextafter(breakEven, math.Inf(1))} {
		fromAverage := moveAgainst(price, breakEven)
		if fromAverage < 0 {
			t.Fatalf("%s: fixture drifted: the move must not be negative, got %v", name, fromAverage)
		}
		want := moveAgainst(price, slowDeclineLastFill)
		if got := TakeProfitPercentage(pending, price, fromAverage); got != want || got < step {
			t.Errorf("%s: the pending trade must read its newest fill %v, past the take profit, got %v", name, want, got)
		}
		if got := TakeProfitPercentage(plain, price, fromAverage); got != fromAverage {
			t.Errorf("%s: without the marker the input must come back, got %v, want %v", name, got, fromAverage)
		}
	}
}

// The newest fill is the newest entry order in history slice order at the
// time of the read: never Position.Price, which a re-anchor moves, and never
// the price the marker row carried, once the ladder has added after it.
func TestTakeProfitPercentageReadsTheNewestFillNotThePositionPrice(t *testing.T) {
	price := betweenTheTakeProfits(t, pendingTrade())

	reanchored := pendingTrade()
	reanchored.PositionPrice = slowDeclineBand
	want := moveAgainst(price, slowDeclineLastFill)
	if got := TakeProfitPercentage(reanchored, price, breakEvenReading); got != want {
		t.Fatalf("a re-anchored trade must read its newest fill: got %v, want %v (the position price reads %v)", got, want, moveAgainst(price, reanchored.PositionPrice))
	}

	deeper := pendingTrade()
	next := testutil.W3sFills("08:00:00", "08:15:00", "08:30:00", "08:45:00", "18:00:00")[watchedFills]
	deeper.History = append(deeper.History, aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: next.Price, OrderId: watchedFills + 1, CreatedAt: next.At})
	if rebuildState(deeper).lastFill().Price != next.Price || deeper.Logs[0].Price == next.Price {
		t.Fatal("fixture drifted: the depth added after the marker must be the newest fill, off the marker's price")
	}
	if got, want := TakeProfitPercentage(deeper, price, breakEvenReading), moveAgainst(price, next.Price); got != want {
		t.Fatalf("a depth added after the marker must be the fill read: got %v, want %v", got, want)
	}
}

// The newest fill is the newest entry ORDER, read the way rebuildState reads
// it, not the newest history row. A second partial row of that order keeps the
// order's first row as its fill, whatever price it carries; an accounting row
// at the sentinel price, an empty row and an exit-side row written after it
// are no fill at all. The take profit is still measured from the order's
// first row.
func TestTakeProfitPercentageReadsTheNewestOrderNotTheNewestRow(t *testing.T) {
	price := betweenTheTakeProfits(t, pendingTrade())
	want := moveAgainst(price, slowDeclineLastFill)
	newest := pendingTrade().History[watchedFills-1]
	if newest.Price != slowDeclineLastFill {
		t.Fatal("fixture drifted: the last history row must be the newest fill")
	}

	for name, row := range map[string]aggragates.TradesHistory{
		"a second partial row of the newest order": {Type: "BUY", Quantity: 0.5, Price: slowDeclineLastFill - 1, OrderId: newest.OrderId, CreatedAt: newest.CreatedAt.Add(time.Minute)},
		"an accounting row":                        {Type: "BUY", Quantity: 0.2, Price: ladder.AccountingPriceCeiling / 10},
		"an empty row":                             {Type: "BUY", Quantity: 0, Price: slowDeclineLastFill - 1, OrderId: newest.OrderId + 1},
		"an exit-side row":                         {Type: "SELL", Quantity: 0.5, Price: slowDeclineBand, OrderId: newest.OrderId + 1},
	} {
		trade := pendingTrade()
		trade.History = append(trade.History, row)
		if st := rebuildState(trade); !st.slowDeclinePending || st.lastFill().Price != slowDeclineLastFill {
			t.Fatalf("%s: fixture drifted: the ladder must stay pending on the same newest fill", name)
		}
		if got := TakeProfitPercentage(trade, price, breakEvenReading); got != want {
			t.Errorf("%s must leave the newest fill where it is: got %v, want %v (the row itself reads %v)", name, got, want, moveAgainst(price, row.Price))
		}
	}
}

// Every trade the marker does not make pending reads its input: an inverse
// ladder, a child, a futures trade, a ladder without a fill, a ladder short
// of SlowDeclineArmDepth, a strategy without the flag, a marker row without a
// price — and a pending trade on no price at all.
func TestTakeProfitPercentageLeavesTheOtherTradesAlone(t *testing.T) {
	marker := pendingTrade().Logs
	price := betweenTheTakeProfits(t, pendingTrade())

	inverse := testutil.LadderTrade(true, fills(watchedFills, "17:38:00")...)
	inverse.Logs = marker
	child := pendingTrade()
	child.ParentID = 7
	futures := pendingTrade()
	futures.Strategy.TradeType = aggragates.Futures
	noFill := testutil.LadderTrade(false)
	noFill.Logs = marker
	shallow := testutil.LadderTrade(false, fills(SlowDeclineArmDepth-1, "17:38:00")...)
	shallow.Logs = marker
	off := pendingTrade()
	off.Strategy.Params.SmartTakeLoss = false
	unpriced := watchedTrade()
	unpriced.Logs = []aggragates.TradesLogs{{Message: SlowDeclineMessage("buy", slowDeclineReasons), Type: aggragates.LOG_INFO}}

	for name, trade := range map[string]aggragates.Trades{
		"an inverse ladder":           inverse,
		"a child":                     child,
		"a futures trade":             futures,
		"a ladder without a fill":     noFill,
		"a ladder too shallow":        shallow,
		"a strategy without the flag": off,
		"a marker without a price":    unpriced,
	} {
		if got := TakeProfitPercentage(trade, price, breakEvenReading); got != breakEvenReading {
			t.Errorf("%s must read its input, got %v", name, got)
		}
	}
	for _, noPrice := range []float64{0, -price} {
		if got := TakeProfitPercentage(pendingTrade(), noPrice, breakEvenReading); got != breakEvenReading {
			t.Errorf("a price of %v must read the input, got %v", noPrice, got)
		}
	}
	if got := TakeProfitPercentage(pendingTrade(), price, breakEvenReading); got == breakEvenReading {
		t.Fatal("control: the pending trade itself must read its newest fill")
	}
}

// A cancel row ends the reading from the newest fill: a cancelled trade reads
// its input at every price, at and over break even included, exactly as the
// ladder without the marker — it carries the marker row, but its last
// slow-decline row is the cancel. A marker after the cancel makes it read its
// newest fill again.
func TestTakeProfitPercentageReadsTheInputOnACancelledTrade(t *testing.T) {
	cancelled := pendingTrade()
	cancelled.Logs = append(cancelled.Logs, aggragates.TradesLogs{Message: SlowDeclineCancelMessage("buy", slowDeclineBreakReasons), Price: slowDeclineLastFill})
	if st := rebuildState(cancelled); st.slowDeclinePending || !carriesTakeProfitMarker(cancelled) {
		t.Fatal("fixture drifted: the cancelled trade must carry the marker and read not pending")
	}
	breakEven := ladder.AverageEntryPrice(cancelled)
	for _, price := range []float64{breakEven, betweenTheTakeProfits(t, pendingTrade()), 2 * slowDeclineBand} {
		fromAverage := moveAgainst(price, breakEven)
		if got := TakeProfitPercentage(cancelled, price, fromAverage); got != fromAverage {
			t.Errorf("at %v a cancelled trade must read its input %v, got %v", price, fromAverage, got)
		}
	}

	repended := cancelled
	repended.Logs = append(append([]aggragates.TradesLogs(nil), cancelled.Logs...), aggragates.TradesLogs{Message: SlowDeclineMessage("buy", nil), Price: slowDeclineLastFill})
	price := betweenTheTakeProfits(t, pendingTrade())
	if got, want := TakeProfitPercentage(repended, price, breakEvenReading), moveAgainst(price, slowDeclineLastFill); got != want {
		t.Fatalf("a marker after the cancel must read the newest fill again: got %v, want %v", got, want)
	}
}

// Switched off, the quiet slow decline reads no newest fill: a pending trade
// hands back its input at every price, at and over break even included, as
// the ladder without the marker does. Switched back on, it reads the newest
// fill again.
func TestTakeProfitPercentageReadsTheInputWhileSwitchedOff(t *testing.T) {
	price := betweenTheTakeProfits(t, pendingTrade())
	breakEven := ladder.AverageEntryPrice(pendingTrade())
	withQuietSlowDeclineExit(t, false)
	for _, at := range []float64{breakEven, price, 2 * slowDeclineBand} {
		fromAverage := moveAgainst(at, breakEven)
		if got := TakeProfitPercentage(pendingTrade(), at, fromAverage); got != fromAverage {
			t.Errorf("switched off, at %v the pending trade must read its input %v, got %v", at, fromAverage, got)
		}
	}
	withQuietSlowDeclineExit(t, true)
	if got := TakeProfitPercentage(pendingTrade(), price, breakEvenReading); got != moveAgainst(price, slowDeclineLastFill) {
		t.Fatalf("control: switched on, the pending trade reads its newest fill, got %v", got)
	}
}

// The scan asked before the fold never turns a pending or a latched trade
// away: every fixture rebuildState reads as pending or latched carries a row
// the take profit's scan (carriesTakeProfitMarker) finds, an indecision row
// included. A row without a price is found by neither the scan nor the fold,
// as rebuildState folds none.
func TestTakeProfitMarkerScanAgreesWithRebuildState(t *testing.T) {
	framed := watchedTrade()
	framed.Logs = []aggragates.TradesLogs{
		{Message: "Hold stopLoss: cooldown: depth held", Price: slowDeclineLastFill},
		{Message: SlowDeclineMessage("stopLoss", nil), Price: slowDeclineLastFill},
	}
	shallowest := testutil.LadderTrade(false, fills(SlowDeclineArmDepth, "17:38:00")...)
	shallowest.Logs = pendingTrade().Logs
	for name, trade := range map[string]aggragates.Trades{"pending": pendingTrade(), "framed": framed, "shallowest watched": shallowest} {
		if !rebuildState(trade).slowDeclinePending {
			t.Fatalf("%s: fixture drifted: rebuildState must read it as pending", name)
		}
		if !carriesTakeProfitMarker(trade) {
			t.Errorf("%s: the take profit's scan must find the row rebuildState folds", name)
		}
	}
	if carriesTakeProfitMarker(watchedTrade()) {
		t.Error("a ladder without the rows carries no marker")
	}

	latchedFramed := watchedTrade()
	latchedFramed.Logs = []aggragates.TradesLogs{
		{Message: "Hold stopLoss: cooldown: depth held", Price: slowDeclineLastFill},
		{Message: IndecisionMessage("stopLoss", indecisionReasons), Price: slowDeclineLastFill},
	}
	shallowestLatched := latchedBy(testutil.LadderTrade(false, fills(IndecisionArmDepth, "17:38:00")...))
	for name, trade := range map[string]aggragates.Trades{"latched": latchedBy(watchedTrade()), "framed": latchedFramed, "shallowest watched": shallowestLatched, "pending and latched": latchedBy(pendingTrade())} {
		if !rebuildState(trade).indecision {
			t.Fatalf("%s: fixture drifted: rebuildState must read it as latched", name)
		}
		if !carriesTakeProfitMarker(trade) {
			t.Errorf("%s: the take profit's scan must find the indecision row rebuildState folds", name)
		}
	}
	unpriced := watchedTrade()
	unpriced.Logs = []aggragates.TradesLogs{{Message: IndecisionMessage("buy", nil)}, {Message: SlowDeclineMessage("buy", nil)}}
	if carriesTakeProfitMarker(unpriced) || rebuildState(unpriced).indecision || rebuildState(unpriced).slowDeclinePending {
		t.Error("a row without a price is found by neither the scan nor the fold")
	}
}

// While a depth priority holds the ladder (depthPriorityHeld) the pending
// reading — the move against the newest fill — waits for the next fill, and
// the latched reading — the move against the position price — applies all the
// same: the hold pauses the quiet slow-decline exit, never the indecision
// direction. A ladder both pending and latched reads the larger of the move it
// is handed and the latched move, the position price's re-anchor over or
// under the newest fill, and under break even it reads its input. Each
// control is the same ladder unheld, and switched off the pause a held ladder
// reads as an unheld one. Every product is rounded on its own (an explicit
// conversion), so the compiler cannot fuse it into the subtraction a move
// makes.
func TestTheHeldTakeProfitSkipsThePendingReadingAndKeepsTheLatchedOne(t *testing.T) {
	newest := shallowDecline().PositionPrice
	average := ladder.AverageEntryPrice(shallowDecline())
	price := float64(average * 1.01)
	handed := moveAgainst(price, average)
	over, under := float64(newest*1.01), float64(newest*0.99)
	hold := func(trade aggragates.Trades) aggragates.Trades { return heldBy(trade, testutil.At("22:00:00")) }
	marker := Row{Message: SlowDeclineMessage("buy", slowDeclineReasons), Price: newest}
	pending := withRow(shallowDecline(), marker, testutil.At("15:00:00"))
	latched := latchedBy(shallowDecline())
	both := latchedBy(pending)
	if st := rebuildState(hold(both)); !st.slowDeclinePending || !st.indecision || !st.depthPriorityHeld || !(handed < moveAgainst(price, newest)) {
		t.Fatalf("fixture drifted: pending, latched and held, the newest fill's move over the input, got %+v", st)
	}

	reanchored := func(trade aggragates.Trades, positionPrice float64) aggragates.Trades {
		trade.PositionPrice = positionPrice
		return trade
	}
	for name, tc := range map[string]struct {
		trade        aggragates.Trades
		held, unheld float64
	}{
		"pending alone":                       {pending, handed, moveAgainst(price, newest)},
		"latched alone, re-anchored over":     {reanchored(latched, over), moveAgainst(price, over), moveAgainst(price, over)},
		"pending and latched, over":           {reanchored(both, over), moveAgainst(price, over), moveAgainst(price, newest)},
		"pending and latched, under":          {reanchored(both, under), moveAgainst(price, under), moveAgainst(price, under)},
		"pending and latched at the position": {both, moveAgainst(price, newest), moveAgainst(price, newest)},
	} {
		if got := TakeProfitPercentage(tc.trade, price, handed); got != tc.unheld {
			t.Errorf("%s: control, unheld the take profit reads %v, got %v", name, tc.unheld, got)
		}
		if got := TakeProfitPercentage(hold(tc.trade), price, handed); got != tc.held {
			t.Errorf("%s: held the take profit reads %v, got %v", name, tc.held, got)
		}
	}

	larger := moveAgainst(price, under) + 1
	if got := TakeProfitPercentage(hold(reanchored(both, under)), price, larger); got != larger {
		t.Errorf("held, an input larger than the latched move comes back, got %v want %v", got, larger)
	}
	below := average - 1
	if input := moveAgainst(below, average); input >= 0 || TakeProfitPercentage(hold(both), below, input) != input {
		t.Errorf("held, under break even the input comes back, got %v for %v", TakeProfitPercentage(hold(both), below, input), input)
	}

	withDepthPriorityPause(t, false)
	if got := TakeProfitPercentage(hold(reanchored(both, over)), price, handed); got != moveAgainst(price, newest) {
		t.Errorf("switched off, the held ladder reads as the unheld one %v, got %v", moveAgainst(price, newest), got)
	}
}
