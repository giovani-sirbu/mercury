package smarttakeloss

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The fixtures are the ladder that sold under break even on a fill the exit
// never judged: the w3s ladder pending from its fifth fill, the marker row
// written for that fill, and a sixth fill landed after the quiet reading
// broke. judgeTick is minutes after the sixth fill: the tick the engines
// stamp the judgement's row with. The readings
// serve judgeBand: a price at it or at overJudgeBand reaches it,
// underJudgeBand does not, and all three sit under the ladder's break even.
const (
	pendingFromFifth = 179.78
	sixthFill        = 175.83
	judgeBand        = 181.0
	overJudgeBand    = 185.0
	underJudgeBand   = 176.5
)

var judgeTick = testutil.At("23:13:00")

// slowDeclineBreakReasons is what sophos names while the leg is not on and
// quiet on a window it read.
var slowDeclineBreakReasons = []string{"NATR(14) 1.04x its median sampled every 24 bars, over 1.00x"}

// pendingAtFive is the six-deep w3s ladder carrying the marker row the engine
// wrote for its fifth fill, before the sixth landed.
func pendingAtFive() aggragates.Trades {
	trade := testutil.LadderTrade(false, fills(6, "23:09:00")...)
	marker := Row{Message: SlowDeclineMessage("buy", slowDeclineReasons), Price: pendingFromFifth}
	trade.Logs = []aggragates.TradesLogs{LogRow(trade, marker, testutil.At("15:00:00"))}
	return trade
}

// withRow is the trade with one more row, written the way the engines write
// the rows Apply hands back.
func withRow(trade aggragates.Trades, row Row, at time.Time) aggragates.Trades {
	trade.Logs = append(append([]aggragates.TradesLogs(nil), trade.Logs...), LogRow(trade, row, at))
	return trade
}

// brokenReading is a reading sophos served on a read window whose leg is not
// on and quiet: the band, what broke and the fill window.
func brokenReading() aggragates.AIIndicators {
	return withBlock(aggragates.SmartTakeLossIndicators{
		SlowDeclineSellBand:     judgeBand,
		SlowDeclineBreakReasons: slowDeclineBreakReasons,
		SlowDeclineFillFrom:     fillWindowFrom.UnixMilli(),
	})
}

// legOnAndQuiet is a reading whose leg is still on and quiet, the band, the
// exit's reasons and the fill window served, without the verdict.
func legOnAndQuiet() aggragates.AIIndicators {
	return withBlock(aggragates.SmartTakeLossIndicators{
		SlowDeclineLegQuiet:    true,
		SlowDeclineSellBand:    judgeBand,
		SlowDeclineExitReasons: slowDeclineReasons,
		SlowDeclineFillFrom:    fillWindowFrom.UnixMilli(),
	})
}

// judgeBandAlone is the band served with nothing else: a read window whose
// break reasons an older sophos did not name.
func judgeBandAlone() aggragates.AIIndicators {
	return withBlock(aggragates.SmartTakeLossIndicators{SlowDeclineSellBand: judgeBand})
}

func assertRow(t *testing.T, got *Row, message string, price float64) {
	t.Helper()
	if got == nil || got.Message != message || got.Price != price {
		t.Fatalf("row = %+v, want %q at %v", got, message, price)
	}
}

// assertNoSale is the answer of a ladder the slow decline does not sell:
// the proposal untouched, no reason.
func assertNoSale(t *testing.T, got Result, position string) {
	t.Helper()
	if got.Position != position || got.Reason != "" {
		t.Fatalf("expected %q with no sale, got %+v", position, got)
	}
}

func TestSlowDeclineFillFixture(t *testing.T) {
	trade := pendingAtFive()
	st := rebuildState(trade)
	if !st.slowDeclinePending || st.slowDeclinePendingFrom != pendingFromFifth || st.lastFill().Price != sixthFill {
		t.Fatalf("fixture drifted: pending from the fifth fill, the sixth the newest, got %+v", st)
	}
	if !slowDeclineFillUnjudged(trade, st) || ladder.ConfiguredDepths(trade) <= len(st.fills) {
		t.Fatal("fixture drifted: the sixth fill must be unjudged, short of the last depth")
	}
	if !(sixthFill < underJudgeBand && underJudgeBand < judgeBand && overJudgeBand < ladder.AverageEntryPrice(trade)) {
		t.Fatal("fixture drifted: the prices must sit over the sixth fill, around the band and under break even")
	}
}

// The cancel row is schema like the marker, framed like it, and neither text
// holds the other — nor any other marker rebuildState reads.
func TestSlowDeclineCancelMessageIsFramedLikeTheMarker(t *testing.T) {
	want := "Hold stopLoss: smartTakeLoss: quiet slow decline broken at the new fill, exit cancelled"
	if got := SlowDeclineCancelMessage("stopLoss", nil); got != want {
		t.Fatalf("cancel row %q, want %q", got, want)
	}
	if got := SlowDeclineCancelMessage("buy", []string{"leg off", "NATR over"}); got != "Hold buy: "+SlowDeclineCancelMarker+" (leg off, NATR over)" {
		t.Fatalf("the reasons follow the marker in parentheses, got %q", got)
	}
	markers := []string{SlowDeclineMarker, SlowDeclineCancelMarker, SlowDeclineEntryHoldReason}
	for _, one := range markers {
		for _, other := range markers {
			if one != other && strings.Contains(one, other) {
				t.Errorf("%q contains %q", one, other)
			}
		}
	}
	if strings.Contains(SlowDeclineCancelMessage("buy", slowDeclineBreakReasons), SlowDeclineMarker) || strings.Contains(SlowDeclineMessage("buy", slowDeclineReasons), SlowDeclineCancelMarker) {
		t.Fatal("a framed row must never read as the other marker")
	}
}

// A reading that is not the leg on and quiet cancels the exit on the first
// tick after the new fill: one cancel row, naming what broke and carrying the
// new fill's price, and no sale at all — on that tick, at and over the band,
// and on the next ones, whatever sophos serves by then. A band served without
// break reasons cancels too, with the bare row.
func TestApplyCancelsThePendingExitOnABrokenReading(t *testing.T) {
	trade := pendingAtFive()
	cancel := SlowDeclineCancelMessage("buy", slowDeclineBreakReasons)
	for _, position := range []string{"", "stopLoss", "buy"} {
		for _, price := range []float64{judgeBand, overJudgeBand, underJudgeBand} {
			got := Apply(trade, position, price, brokenReading())
			assertRow(t, got.SlowDecline, cancel, sixthFill)
			assertNoSale(t, got, position)
		}
	}
	bare := Apply(trade, "", overJudgeBand, judgeBandAlone())
	assertRow(t, bare.SlowDecline, SlowDeclineCancelMessage("buy", nil), sixthFill)
	assertNoSale(t, bare, "")

	cancelled := withRow(trade, Row{Message: cancel, Price: sixthFill}, judgeTick)
	if st := rebuildState(cancelled); st.slowDeclinePending || !st.slowDeclineWatched {
		t.Fatalf("a cancelled ladder is watched and not pending, got %+v", st)
	}
	for _, reading := range []aggragates.AIIndicators{brokenReading(), judgeBandAlone(), withBlock(aggragates.SmartTakeLossIndicators{})} {
		for _, price := range []float64{judgeBand, overJudgeBand, underJudgeBand} {
			assertNoSlowDeclineRow(t, Apply(cancelled, "", price, reading), "")
			assertNoSlowDeclineRow(t, Apply(cancelled, "stopLoss", price, reading), "stopLoss")
		}
	}
}

// The leg still on and quiet confirms the exit: the marker row again,
// carrying the new fill's price, and the ladder pending from that fill — the
// band sells from the confirming tick on, as on any pending ladder, and no
// second row is written.
func TestApplyConfirmsThePendingExitOnALegOnAndQuiet(t *testing.T) {
	trade := pendingAtFive()
	marker := SlowDeclineMessage("buy", slowDeclineReasons)
	got := Apply(trade, "", underJudgeBand, legOnAndQuiet())
	assertRow(t, got.SlowDecline, marker, sixthFill)
	assertNoSale(t, got, "")
	confirming := Apply(trade, "stopLoss", judgeBand, legOnAndQuiet())
	assertRow(t, confirming.SlowDecline, marker, sixthFill)
	assertForced(t, confirming, reasonSellBand)

	confirmed := withRow(trade, *got.SlowDecline, judgeTick)
	if st := rebuildState(confirmed); !st.slowDeclinePending || st.slowDeclinePendingFrom != sixthFill || slowDeclineFillUnjudged(confirmed, st) {
		t.Fatalf("a confirmed ladder is pending from the new fill, got %+v", st)
	}
	for _, reading := range []aggragates.AIIndicators{judgeBandAlone(), brokenReading(), legOnAndQuiet()} {
		for _, price := range []float64{judgeBand, overJudgeBand} {
			sold := Apply(confirmed, "", price, reading)
			assertForced(t, sold, reasonSellBand)
			if sold.SlowDecline != nil {
				t.Fatalf("a judged fill writes no second row, got %+v", sold)
			}
		}
		assertNoSlowDeclineRow(t, Apply(confirmed, "", underJudgeBand, reading), "")
	}
}

// A tick without a reading judges nothing: no band served — sophos down, no
// window cached — writes no row, whatever else the block carries, and the
// fill waits. The first tick that serves the band judges it.
func TestApplyJudgesTheNewFillOnTheFirstTickThatServesTheBand(t *testing.T) {
	trade := pendingAtFive()
	for name, reading := range map[string]aggragates.AIIndicators{
		"a zero block":                 withBlock(aggragates.SmartTakeLossIndicators{}),
		"a quiet leg without a band":   withBlock(aggragates.SmartTakeLossIndicators{SlowDeclineLegQuiet: true, SlowDeclineExitReasons: slowDeclineReasons}),
		"the verdict without a band":   withBlock(aggragates.SmartTakeLossIndicators{SlowDeclineExit: true, SlowDeclineLegQuiet: true}),
		"break reasons without a band": withBlock(aggragates.SmartTakeLossIndicators{SlowDeclineBreakReasons: slowDeclineBreakReasons}),
		"capital protection, no band":  withBlock(solBlock()),
		"a band that is not a price":   withBlock(aggragates.SmartTakeLossIndicators{SlowDeclineSellBand: -judgeBand, SlowDeclineLegQuiet: true}),
		"a smooth bar without a band":  withBlock(aggragates.SmartTakeLossIndicators{SlowDeclineSmoothFrom: testutil.At("23:00:00").UnixMilli()}),
	} {
		for _, position := range []string{"", "stopLoss"} {
			if got := Apply(trade, position, underJudgeBand, reading); got.SlowDecline != nil || got.Position != position {
				t.Fatalf("%s: nothing is judged and the proposal passes, got %+v", name, got)
			}
		}
	}
	if st := rebuildState(trade); !slowDeclineFillUnjudged(trade, st) {
		t.Fatal("the fill must still wait for its judgement")
	}
	assertRow(t, Apply(trade, "", underJudgeBand, brokenReading()).SlowDecline, SlowDeclineCancelMessage("buy", slowDeclineBreakReasons), sixthFill)
}

// The fill that takes the ladder to its last depth is never judged: no row,
// whatever the reading, and the band still sells as on any pending ladder.
// One depth more configured, the same fill is judged.
func TestApplyNeverJudgesTheFillThatTakesTheLadderToItsLastDepth(t *testing.T) {
	last := pendingAtFive()
	last.StrategyPair.StrategySettings[0].Depths = 6
	if ladder.ConfiguredDepths(last) != ladder.CountFilledEntries(last) {
		t.Fatal("fixture drifted: the sixth fill must take the ladder to its last depth")
	}
	for _, reading := range []aggragates.AIIndicators{brokenReading(), judgeBandAlone(), legOnAndQuiet()} {
		assertNoSlowDeclineRow(t, Apply(last, "", underJudgeBand, reading), "")
		for _, price := range []float64{judgeBand, overJudgeBand} {
			sold := Apply(last, "", price, reading)
			assertForced(t, sold, reasonSellBand)
			if sold.SlowDecline != nil {
				t.Fatalf("the last depth's fill writes no row, got %+v", sold)
			}
		}
	}

	short := pendingAtFive()
	short.StrategyPair.StrategySettings[0].Depths = 7
	assertRow(t, Apply(short, "", underJudgeBand, brokenReading()).SlowDecline, SlowDeclineCancelMessage("buy", slowDeclineBreakReasons), sixthFill)
}

// After a cancel the ladder goes pending again only by the rule it went
// pending by the first time (slowDeclineGoesPending): the verdict,
// or the leg on and quiet counted smooth from the newest fill with the
// closed bars after it, marks it at its newest fill; a broken reading, the
// band alone, or a leg on and quiet whose count from the newest fill is not
// served marks nothing. Pending again, it sells at the band.
func TestApplyGoesPendingAgainAfterACancelOnlyByTheGoPendingRule(t *testing.T) {
	cancelled := withRow(pendingAtFive(), Row{Message: SlowDeclineCancelMessage("buy", slowDeclineBreakReasons), Price: sixthFill}, judgeTick)
	next := judgeTick.Add(time.Hour)
	for _, reading := range []aggragates.AIIndicators{brokenReading(), judgeBandAlone(), legOnAndQuiet()} {
		assertNoSlowDeclineRow(t, Apply(cancelled, "", underJudgeBand, reading), "")
	}

	verdict := legOnAndQuiet()
	verdict.SmartTakeLoss.SlowDeclineExit = true
	smoothFromTheFill := quietLegBlock(false, testutil.At("23:00:00"), testutil.At("23:59:00"))
	for name, reading := range map[string]aggragates.AIIndicators{"the verdict": verdict, "the count from the newest fill": smoothFromTheFill} {
		got := Apply(cancelled, "", underJudgeBand, reading)
		assertRow(t, got.SlowDecline, SlowDeclineMessage("buy", slowDeclineReasons), sixthFill)
		repended := withRow(cancelled, *got.SlowDecline, next)
		if st := rebuildState(repended); !st.slowDeclinePending || st.slowDeclinePendingFrom != sixthFill {
			t.Fatalf("%s makes the cancelled ladder pending from its newest fill, got %+v", name, st)
		}
		assertForced(t, Apply(repended, "", overJudgeBand, judgeBandAlone()), reasonSellBand)
	}
}

// Several fills landed since the marker are judged as one, at the newest:
// one row, carrying that fill's price.
func TestApplyJudgesTheNewestOfSeveralFills(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(6, "23:09:00")...)
	trade.Logs = []aggragates.TradesLogs{{Message: SlowDeclineMessage("buy", nil), Price: slowDeclineLastFill, Type: aggragates.LOG_INFO}}
	assertRow(t, Apply(trade, "", underJudgeBand, brokenReading()).SlowDecline, SlowDeclineCancelMessage("buy", slowDeclineBreakReasons), sixthFill)
	assertRow(t, Apply(trade, "", underJudgeBand, legOnAndQuiet()).SlowDecline, SlowDeclineMessage("buy", slowDeclineReasons), sixthFill)
}

// The rows hold prices, so a later fill at exactly the price the ladder is
// pending from reads as judged: the accepted limit. Nothing is written and
// the band still sells.
func TestApplyReadsARefillAtThePendingPriceAsJudged(t *testing.T) {
	trade := pendingAtFive()
	trade.History = append(trade.History, aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: pendingFromFifth, OrderId: 7, CreatedAt: testutil.At("23:30:00")})
	st := rebuildState(trade)
	if st.lastFill().Price != pendingFromFifth || slowDeclineFillUnjudged(trade, st) {
		t.Fatalf("fixture drifted: the refill must be the newest fill, at the pending price, got %+v", st)
	}
	got := Apply(trade, "", judgeBand, brokenReading())
	assertForced(t, got, reasonSellBand)
	if got.SlowDecline != nil {
		t.Fatalf("a refill at the pending price writes no row, got %+v", got)
	}
}

// The judgement goes out before the protected return, like the marker: a
// close the ladder proposes, or a trade resting in one, still gets its row,
// and the close is never replaced.
func TestApplyJudgesTheNewFillBesideADecidedClose(t *testing.T) {
	resting := pendingAtFive()
	resting.PositionType = "takeProfit"
	got := Apply(resting, "", overJudgeBand, brokenReading())
	assertRow(t, got.SlowDecline, SlowDeclineCancelMessage("takeProfit", slowDeclineBreakReasons), sixthFill)
	assertNoSale(t, got, "")

	proposed := Apply(pendingAtFive(), "takeProfit", overJudgeBand, legOnAndQuiet())
	assertRow(t, proposed.SlowDecline, SlowDeclineMessage("buy", slowDeclineReasons), sixthFill)
	assertNoSale(t, proposed, "takeProfit")
}

// withFill is the trade with one more entry order filled at price, stamped
// at: the next OrderId, one unit, the position at the fill — the trade as the
// engines leave it after a fill.
func withFill(trade aggragates.Trades, price float64, at time.Time) aggragates.Trades {
	trade.History = append(append([]aggragates.TradesHistory(nil), trade.History...), aggragates.TradesHistory{
		Type: "BUY", Quantity: 1, Price: price, OrderId: int64(len(trade.History) + 1), CreatedAt: at,
	})
	trade.PositionPrice = price
	return trade
}

// engineTick is one engine tick: Apply's answer on the trade, and the trade
// carrying every row that answer handed back, appended in the order the
// engines append them — the slow-decline row, then the indecision row — and
// stamped with the tick.
func engineTick(trade aggragates.Trades, position string, price float64, now time.Time, ai aggragates.AIIndicators) (aggragates.Trades, Result) {
	got := Apply(trade, position, price, ai)
	if got.SlowDecline != nil {
		trade = withRow(trade, *got.SlowDecline, now)
	}
	if got.Indecision != nil {
		trade = withRow(trade, *got.Indecision, now)
	}
	return trade, got
}

// slowDeclineRowsInOrder lists the trade's slow-decline rows in slice order,
// each as its kind and the price it carries.
func slowDeclineRowsInOrder(trade aggragates.Trades) []string {
	var rows []string
	for _, row := range trade.Logs {
		switch {
		case strings.Contains(row.Message, SlowDeclineCancelMarker):
			rows = append(rows, fmt.Sprintf("cancel@%v", row.Price))
		case strings.Contains(row.Message, SlowDeclineMarker):
			rows = append(rows, fmt.Sprintf("marker@%v", row.Price))
		}
	}
	return rows
}

// verdictReading is the verdict served with the band: the reading a
// watched ladder that is not pending goes pending on.
func verdictReading() aggragates.AIIndicators {
	reading := legOnAndQuiet()
	reading.SmartTakeLoss.SlowDeclineExit = true
	return reading
}

// pastTheNewestFillsTakeProfit is a price past the take profit measured from
// the trade's newest fill — its row's Percentage plus Tolerance — and under
// the band judgeBand: where the removed newest-fill sale fired on a pending
// ladder under break even.
func pastTheNewestFillsTakeProfit(t *testing.T, trade aggragates.Trades) float64 {
	t.Helper()
	row := trade.StrategyPair.StrategySettings[0]
	fromNewest := trade.PositionPrice / (1 - (row.Percentage+row.Tolerance)/100)
	price := (fromNewest + judgeBand) / 2
	if !(fromNewest < price && price < judgeBand && judgeBand < ladder.AverageEntryPrice(trade)) {
		t.Fatalf("fixture drifted: the newest fill's take profit %v must sit under the band %v, under break even %v", fromNewest, judgeBand, ladder.AverageEntryPrice(trade))
	}
	return price
}

// A ladder judged fill after fill folds the rows Apply itself hands back, in
// the order the engines append them. The verdict marks it pending at its
// newest fill. Two fills land while sophos serves no reading: nothing is
// judged and nothing sells, and the first tick that serves the band judges
// the newest of them once — the cancel row at its price — while the ticks
// after it write nothing and sell nothing, at the band or past the newest
// fill's take profit, and the take profit reads the average alone. The
// verdict marks the ladder pending again at that fill, where only the band
// sells it; the next fill, on a broken reading, is cancelled in turn — marker,
// cancel, marker, cancel — and the ladder is not pending. Marked pending once
// more, the fill that takes it to its last depth is never judged, and the
// band alone sells it: capital protection, which watches it from that fill
// on, is served no band.
func TestApplyFoldsItsOwnRowsFillAfterFill(t *testing.T) {
	noReading := withBlock(aggragates.SmartTakeLossIndicators{})
	cancel := SlowDeclineCancelMessage("buy", slowDeclineBreakReasons)
	marker := SlowDeclineMessage("buy", slowDeclineReasons)
	w3s := fills(8, "21:30:00")
	fill5, fill6, fill7, fill8 := w3s[4].Price, w3s[5].Price, w3s[6].Price, w3s[7].Price

	trade := testutil.LadderTrade(false, fills(4, "17:38:00")...)
	if ladder.ConfiguredDepths(trade) != 8 || fill5 != pendingFromFifth || fill6 != sixthFill || trade.PositionPrice != slowDeclineLastFill {
		t.Fatal("fixture drifted: the w3s row is eight deep, its fourth fill the marked one, its fifth and sixth the ones the other fixtures judge")
	}

	trade, got := engineTick(trade, "", underJudgeBand, testutil.At("18:00:00"), verdictReading())
	assertRow(t, got.SlowDecline, marker, slowDeclineLastFill)
	assertNoSale(t, got, "")

	trade = withFill(trade, fill5, testutil.At("18:30:00"))
	for _, position := range []string{"", "stopLoss"} {
		next, got := engineTick(trade, position, overJudgeBand, testutil.At("18:45:00"), noReading)
		assertNoSlowDeclineRow(t, got, position)
		if len(next.Logs) != len(trade.Logs) {
			t.Fatalf("a tick without a reading judges nothing, got %+v", next.Logs)
		}
	}
	trade = withFill(trade, fill6, testutil.At("19:00:00"))
	trade, got = engineTick(trade, "", overJudgeBand, testutil.At("19:15:00"), noReading)
	assertNoSlowDeclineRow(t, got, "")
	if st := rebuildState(trade); !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill || !slowDeclineFillUnjudged(trade, st) {
		t.Fatalf("two fills landed without a reading: still pending from the marked fill, the newest unjudged, got %+v", st)
	}

	trade, got = engineTick(trade, "", overJudgeBand, testutil.At("19:30:00"), brokenReading())
	assertRow(t, got.SlowDecline, cancel, fill6)
	assertNoSale(t, got, "")
	breakEven := ladder.AverageEntryPrice(trade)
	overBreakEven := breakEven + 1
	for _, reading := range []aggragates.AIIndicators{brokenReading(), judgeBandAlone(), legOnAndQuiet(), noReading} {
		for _, price := range []float64{judgeBand, overJudgeBand, pastTheNewestFillsTakeProfit(t, trade)} {
			assertNoSlowDeclineRow(t, Apply(trade, "", price, reading), "")
		}
	}
	if got := TakeProfitPercentage(trade, overBreakEven, moveAgainst(overBreakEven, breakEven)); got != moveAgainst(overBreakEven, breakEven) {
		t.Fatalf("a cancelled ladder's take profit reads the average alone, got %v", got)
	}

	trade, got = engineTick(trade, "", underJudgeBand, testutil.At("20:00:00"), verdictReading())
	assertRow(t, got.SlowDecline, marker, fill6)
	assertNoSale(t, got, "")
	assertNoSlowDeclineRow(t, Apply(trade, "", pastTheNewestFillsTakeProfit(t, trade), judgeBandAlone()), "")
	assertForced(t, Apply(trade, "", judgeBand, judgeBandAlone()), reasonSellBand)
	if got := TakeProfitPercentage(trade, overBreakEven, moveAgainst(overBreakEven, breakEven)); got != moveAgainst(overBreakEven, fill6) {
		t.Fatalf("pending again, the take profit reads the newest fill from break even up, got %v", got)
	}

	trade = withFill(trade, fill7, testutil.At("20:30:00"))
	trade, got = engineTick(trade, "", overJudgeBand, testutil.At("20:45:00"), brokenReading())
	assertRow(t, got.SlowDecline, cancel, fill7)
	assertNoSale(t, got, "")
	if st := rebuildState(trade); st.slowDeclinePending || st.slowDeclinePendingFrom != 0 || !st.slowDeclineWatched {
		t.Fatalf("marker, cancel, marker, cancel: watched and not pending, got %+v", st)
	}
	for _, price := range []float64{judgeBand, overJudgeBand, pastTheNewestFillsTakeProfit(t, trade)} {
		assertNoSlowDeclineRow(t, Apply(trade, "", price, brokenReading()), "")
	}

	trade, got = engineTick(trade, "", underJudgeBand, testutil.At("21:15:00"), verdictReading())
	assertRow(t, got.SlowDecline, marker, fill7)
	trade = withFill(trade, fill8, testutil.At("21:30:00"))
	if ladder.CountFilledEntries(trade) != ladder.ConfiguredDepths(trade) {
		t.Fatal("fixture drifted: the eighth fill must take the ladder to its last depth")
	}
	for _, reading := range []aggragates.AIIndicators{brokenReading(), judgeBandAlone(), legOnAndQuiet()} {
		assertNoSlowDeclineRow(t, Apply(trade, "", underJudgeBand, reading), "")
		assertNoSlowDeclineRow(t, Apply(trade, "", pastTheNewestFillsTakeProfit(t, trade), reading), "")
		sold := Apply(trade, "", overJudgeBand, reading)
		assertForced(t, sold, reasonSellBand)
		if sold.SlowDecline != nil {
			t.Fatalf("the last depth's fill is never judged, got %+v", sold)
		}
	}

	want := []string{
		fmt.Sprintf("marker@%v", slowDeclineLastFill), fmt.Sprintf("cancel@%v", fill6), fmt.Sprintf("marker@%v", fill6),
		fmt.Sprintf("cancel@%v", fill7), fmt.Sprintf("marker@%v", fill7),
	}
	if rows := slowDeclineRowsInOrder(trade); fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("rows %v, want %v", rows, want)
	}
}
