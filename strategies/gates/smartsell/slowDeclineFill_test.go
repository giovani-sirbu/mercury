package smartsell

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The fixtures are the ladder that went pending on its fifth fill and took a
// sixth that the exit never judged: the w3s ladder pending from its fifth fill,
// the row written for that fill, and a sixth fill landed after the quiet reading
// broke. judgeTick is minutes after the sixth fill: the tick the engines stamp the
// judgement's row with. underJudgeBand, judgeBand and overJudgeBand are three
// prints of the dead zone, over the sixth fill and under the ladder's break even,
// in that order, each a loss for the whole position, so the shared action acts on
// a standing trigger at any of them by the ladder's state and never its proposal;
// armingPrice names the print where the engines' buy row arms the next depth, a
// step under the sixth fill, and gainPrint one that books no loss.
const (
	pendingFromFifth = 179.78
	sixthFill        = 175.83
	judgeBand        = 181.0
	overJudgeBand    = 185.0
	underJudgeBand   = 176.5
)

var judgeTick = testutil.At("23:13:00")

// quietDeclineBreakReasons is what sophos names while the leg is not on and
// quiet on a window it read.
var quietDeclineBreakReasons = []string{"NATR(14) 1.04x its median sampled every 24 bars, over 1.00x"}

// pendingAtFive is the six-deep w3s ladder carrying the pending pair the
// engine wrote for its fifth fill, before the sixth landed.
func pendingAtFive() aggragates.Trades {
	trade := testutil.LadderTrade(false, fills(6, "23:09:00")...)
	return withRow(trade, quietPending(pendingFromFifth), testutil.At("15:00:00"))
}

// brokenReading is a reading sophos served on a read window whose leg is not
// on and quiet: what broke and the fill window, which marks the window as read.
func brokenReading() aggragates.AIIndicators {
	return withBlock(withEveryRowTrend(aggragates.SmartSellIndicators{
		QuietDeclineBreakReasons: quietDeclineBreakReasons,
		QuietDeclineFillFrom:     fillWindowFrom.UnixMilli(),
	}))
}

// legOnAndQuiet is a reading whose leg is still on and quiet, the exit's
// reasons and the fill window served, without the verdict.
func legOnAndQuiet() aggragates.AIIndicators {
	return withBlock(withEveryRowTrend(aggragates.SmartSellIndicators{
		QuietDeclineLegQuiet:    true,
		QuietDeclineExitReasons: slowDeclineReasons,
		QuietDeclineFillFrom:    fillWindowFrom.UnixMilli(),
	}))
}

// windowReadAlone is the fill window served with nothing else: a read window
// whose break reasons an older sophos did not name.
func windowReadAlone() aggragates.AIIndicators {
	return withBlock(withEveryRowTrend(aggragates.SmartSellIndicators{QuietDeclineFillFrom: fillWindowFrom.UnixMilli()}))
}

// sixthCancelled is the row that cancels the exit at the sixth fill on the
// reasons sophos names for a broken reading; sixthConfirmed the one that
// confirms it on the leg still on and quiet.
func sixthCancelled() Row {
	return CancelledRow(sixthFill, quietFinding(quietDeclineBreakReasons))
}

func sixthConfirmed() Row {
	return ConfirmedRow(sixthFill, quietFinding(slowDeclineReasons))
}

func TestJudgementFixture(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingAtFive()
	st := rebuildState(trade)
	if !st.slowDeclinePending || st.slowDeclinePendingFrom != pendingFromFifth || st.lastFill().Price != sixthFill {
		t.Fatalf("fixture drifted: pending from the fifth fill, the sixth the newest, got %+v", st)
	}
	if !slowDeclineFillUnjudged(trade, st) || ladder.ConfiguredDepths(trade) <= len(st.fills) {
		t.Fatal("fixture drifted: the sixth fill must be unjudged, short of the last depth")
	}
	if !(armingPrice(trade) < sixthFill && sixthFill < underJudgeBand && underJudgeBand < judgeBand && judgeBand < overJudgeBand && overJudgeBand < ladder.AverageEntryPrice(trade)) {
		t.Fatal("fixture drifted: the print that arms the next depth sits under the sixth fill, the three prints of the dead zone over it in order and under break even")
	}
}

// A reading that is not the leg on and quiet cancels the exit on the first
// tick after the new fill: one cancelled row, naming what broke and carrying
// the new fill's price, and nothing activated or sold at all — on that tick,
// whatever it proposes, an arming included, in buy or in trailingBuy, and on the
// next ones, whatever sophos serves by then, for the ladder is no longer pending
// and no trigger stands. A window read without break reasons cancels too, with
// the bare row.
func TestApplyCancelsThePendingExitOnABrokenReading(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingAtFive()
	proposals := append(slices.Clone(ordinaryProposals), armingProposals...)
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, position := range proposals {
			for _, price := range []float64{armingPrice(trade), underJudgeBand, judgeBand, overJudgeBand} {
				got := Apply(restingIn(trade, state), position, price, brokenReading())
				assertRow(t, got, sixthCancelled())
				assertNoSale(t, got, position)
			}
		}
		bare := Apply(restingIn(trade, state), "trailingBuy", armingPrice(trade), windowReadAlone())
		assertRow(t, bare, CancelledRow(sixthFill, quietFinding(nil)))
		assertNoSale(t, bare, "trailingBuy")
	}

	cancelled := withRow(trade, sixthCancelled(), judgeTick)
	if st := rebuildState(cancelled); st.slowDeclinePending || !st.slowDeclineWatched {
		t.Fatalf("a cancelled ladder is watched and not pending, got %+v", st)
	}
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, reading := range []aggragates.AIIndicators{brokenReading(), windowReadAlone(), nothingServed()} {
			for _, position := range proposals {
				for _, price := range []float64{armingPrice(cancelled), underJudgeBand, judgeBand, overJudgeBand} {
					assertNoSlowDeclineRow(t, Apply(restingIn(cancelled, state), position, price, reading), position)
				}
			}
		}
	}
}

// The leg still on and quiet confirms the exit: the confirmed row, carrying
// the new fill's price, and the ladder pending from that fill. At a print that
// books no loss the row is all the confirming tick writes. At a loss the new fill
// counts as judged on the tick that judges it, so the shared action acts on the
// trigger on that same tick, by the ladder's state and whatever it proposes: the
// trailingBuy of a ladder in buy is activated, the activation row after the confirmed
// one and an arming replaced, and a ladder in trailingBuy is sold with the confirmed
// row beside the sale. From then on the ladder is activated in buy and sold in
// trailingBuy on every tick, as any pending ladder is, and no second row of the
// gate's is written.
func TestApplyConfirmsThePendingExitOnALegOnAndQuiet(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingAtFive()
	got := applyAtAGain(trade, "", legOnAndQuiet())
	assertRow(t, got, sixthConfirmed())
	assertNoSale(t, got, "")
	for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
		price := printFor(trade, position)
		assertActivated(t, Apply(trade, position, price, legOnAndQuiet()), sixthConfirmed(), quietActivation(trade, price))
		assertSoldWith(t, Apply(restingIn(trade, "trailingBuy"), position, price, legOnAndQuiet()), reasonSlowDecline, sixthConfirmed())
	}

	confirmed := withRow(trade, got.Rows[0], judgeTick)
	if st := rebuildState(confirmed); !st.slowDeclinePending || st.slowDeclinePendingFrom != sixthFill || slowDeclineFillUnjudged(confirmed, st) {
		t.Fatalf("a confirmed ladder is pending from the new fill, got %+v", st)
	}
	for _, reading := range []aggragates.AIIndicators{windowReadAlone(), brokenReading(), legOnAndQuiet()} {
		for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
			for _, price := range []float64{armingPrice(confirmed), underJudgeBand, judgeBand, overJudgeBand} {
				assertActivated(t, Apply(confirmed, position, price, reading), quietActivation(confirmed, price))
				assertSold(t, Apply(restingIn(confirmed, "trailingBuy"), position, price, reading), reasonSlowDecline)
			}
			assertNoSlowDeclineRow(t, applyAtAGain(confirmed, position, reading), position)
		}
	}
}

// A tick without a reading judges nothing: no window read — sophos down, no
// window cached — writes no row, whatever else the block carries, and the fill
// waits and activates and sells nothing, in buy or in trailingBuy, on an arming too,
// for an unjudged fill leaves the gate's trigger of the shared action unmet. The
// first tick that reads the window judges it.
func TestApplyJudgesTheNewFillOnTheFirstTickThatReadsTheWindow(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingAtFive()
	for name, reading := range map[string]aggragates.AIIndicators{
		"a zero block":                    withBlock(aggragates.SmartSellIndicators{}),
		"a quiet leg without a window":    withBlock(aggragates.SmartSellIndicators{QuietDeclineLegQuiet: true, QuietDeclineExitReasons: slowDeclineReasons}),
		"the verdict without a window":    withBlock(aggragates.SmartSellIndicators{QuietDeclineVerdict: true, QuietDeclineLegQuiet: true}),
		"break reasons without a window":  withBlock(aggragates.SmartSellIndicators{QuietDeclineBreakReasons: quietDeclineBreakReasons}),
		"capital protection, no window":   withBlock(solBlock()),
		"a window that is not a time":     withBlock(aggragates.SmartSellIndicators{QuietDeclineFillFrom: -fillWindowFrom.UnixMilli(), QuietDeclineLegQuiet: true}),
		"a smooth bar without a window":   withBlock(aggragates.SmartSellIndicators{QuietDeclineSmoothFrom: testutil.At("23:00:00").UnixMilli()}),
		"the verdict, the window missing": withBlock(aggragates.SmartSellIndicators{QuietDeclineVerdict: true, QuietDeclineExitReasons: slowDeclineReasons}),
		"a recent bar, no window":         withRecent(nothingServed(), testutil.At("22:00:00"), testutil.At("12:00:00")),
	} {
		for _, state := range []string{"buy", "trailingBuy"} {
			for _, position := range []string{"", "trailingBuy"} {
				if got := Apply(restingIn(trade, state), position, printFor(trade, position), reading); len(got.Rows) != 0 || got.Position != position || got.Reason != "" || got.Activated {
					t.Fatalf("%s in %s: nothing is judged and the proposal passes, got %+v", name, state, got)
				}
			}
		}
	}
	if st := rebuildState(trade); !slowDeclineFillUnjudged(trade, st) {
		t.Fatal("the fill must still wait for its judgement")
	}
	assertRow(t, Apply(trade, "", underJudgeBand, brokenReading()), sixthCancelled())
}

// The fill that takes the ladder to its last depth is never judged: no row,
// whatever the reading, and the ladder counts as judged, so the shared action
// acts on it as it does on any pending ladder, the depth bound being off here
// (TestMain): at a print that books a gain with a buy order resting nothing is written at all, and at a
// loss the trailingBuy of a ladder in buy is activated with the activation row alone
// and a ladder in trailingBuy is sold with no row. One depth more configured, the same
// fill is judged.
func TestApplyNeverJudgesTheFillThatTakesTheLadderToItsLastDepth(t *testing.T) {
	withQuietDecline(t, true)
	last := pendingAtFive()
	last.StrategyPair.StrategySettings[0].Depths = 6
	if ladder.ConfiguredDepths(last) != ladder.CountFilledEntries(last) {
		t.Fatal("fixture drifted: the sixth fill must take the ladder to its last depth")
	}
	armed := armingPrice(last)
	for _, reading := range []aggragates.AIIndicators{brokenReading(), windowReadAlone(), legOnAndQuiet()} {
		assertNoSlowDeclineRow(t, applyAtAGain(last, "", reading), "")
		assertActivated(t, Apply(last, "", underJudgeBand, reading), quietActivation(last, underJudgeBand))
		for _, position := range armingProposals {
			assertActivated(t, Apply(last, position, armed, reading), quietActivation(last, armed))
			assertSold(t, Apply(restingIn(last, "trailingBuy"), position, armed, reading), reasonSlowDecline)
		}
	}

	short := pendingAtFive()
	short.StrategyPair.StrategySettings[0].Depths = 7
	assertRow(t, Apply(short, "", underJudgeBand, brokenReading()), sixthCancelled())
}

// With the depth bound on, the ladder the sixth fill took to its last depth is
// held by its depth, whatever the reading serves and whatever state it rests in:
// the action never activates the trailingBuy of a ladder, nor sells one, whose ceiling
// is at or under the depths it must have left — a ladder a gate activated earlier
// included, the bound being read before the commitment — and the held row, written
// once, names the depth.
func TestAPendingLadderAtItsLastDepthIsHeldByTheDepthBound(t *testing.T) {
	if SaleIgnoreLastDepths <= 0 {
		t.Skip("SaleIgnoreLastDepths is off: no depth bound holds the ladder at its last depth")
	}
	withQuietDecline(t, true)
	withSaleIgnoreLastDepths(t, SaleIgnoreLastDepths)
	last := pendingAtFive()
	last.StrategyPair.StrategySettings[0].Depths = 6
	armed := armingPrice(last)
	heldRow := SaleHeldAtDepthRow(sixthFill, 6, 6, FeatureQuietDecline)
	for _, resting := range []aggragates.Trades{last, restingIn(last, "trailingBuy"), committedBy(last, quietActivation(last, armed))} {
		for _, reading := range []aggragates.AIIndicators{brokenReading(), windowReadAlone(), legOnAndQuiet()} {
			held := Apply(resting, "trailingBuy", armed, reading)
			assertNoSale(t, held, "trailingBuy")
			assertRows(t, held, heldRow)
		}
		written := withRow(resting, heldRow, judgeTick)
		assertUntouched(t, Apply(written, "trailingBuy", armed, brokenReading()), "trailingBuy")
	}
}

// After a cancel the ladder goes pending again only by the rule it went
// pending by the first time (quietDeclineGoesPending): the verdict, or the leg
// on and quiet counted smooth from the newest fill with the closed bars after
// it, marks it at its newest fill; a broken reading, the window alone, or a leg
// on and quiet whose count from the newest fill is not served marks nothing.
// Pending again, the shared action acts on it at every print that books a loss, by
// the ladder's state: the trailingBuy of a ladder in buy is activated and a ladder in
// trailingBuy is sold, with no row of the gate's beside either; at a print that
// books none the ladder in buy is left alone.
func TestApplyGoesPendingAgainAfterACancelOnlyByTheGoPendingRule(t *testing.T) {
	withQuietDecline(t, true)
	cancelled := withRow(pendingAtFive(), sixthCancelled(), judgeTick)
	next := judgeTick.Add(time.Hour)
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, reading := range []aggragates.AIIndicators{brokenReading(), windowReadAlone(), legOnAndQuiet()} {
			assertNoSlowDeclineRow(t, Apply(restingIn(cancelled, state), "", underJudgeBand, reading), "")
			assertNoSlowDeclineRow(t, Apply(restingIn(cancelled, state), "trailingBuy", armingPrice(cancelled), reading), "trailingBuy")
		}
	}

	verdict := legOnAndQuiet()
	verdict.SmartSell.QuietDeclineVerdict = true
	smoothFromTheFill := quietLegBlock(false, testutil.At("23:00:00"), testutil.At("23:59:00"))
	for name, reading := range map[string]aggragates.AIIndicators{"the verdict": verdict, "the count from the newest fill": smoothFromTheFill} {
		got := applyAtAGain(cancelled, "", reading)
		assertRow(t, got, quietPending(sixthFill))
		assertActivated(t, Apply(cancelled, "", underJudgeBand, reading), quietPending(sixthFill), quietActivation(cancelled, underJudgeBand))
		repended := withRow(cancelled, got.Rows[0], next)
		if st := rebuildState(repended); !st.slowDeclinePending || st.slowDeclinePendingFrom != sixthFill {
			t.Fatalf("%s makes the cancelled ladder pending from its newest fill, got %+v", name, st)
		}
		armed := armingPrice(repended)
		assertActivated(t, Apply(repended, "trailingBuy", armed, windowReadAlone()), quietActivation(repended, armed))
		assertSold(t, Apply(restingIn(repended, "trailingBuy"), "trailingBuy", armed, windowReadAlone()), reasonSlowDecline)
		assertNoSlowDeclineRow(t, applyAtAGain(repended, "", windowReadAlone()), "")
		assertActivated(t, Apply(repended, "", overJudgeBand, windowReadAlone()), quietActivation(repended, overJudgeBand))
	}
}

// Several fills landed since the row are judged as one, at the newest: one
// row, carrying that fill's price.
func TestApplyJudgesTheNewestOfSeveralFills(t *testing.T) {
	withQuietDecline(t, true)
	trade := withRows(testutil.LadderTrade(false, fills(6, "23:09:00")...), PendingRow(slowDeclineLastFill, quietFinding(nil)))
	assertRow(t, Apply(trade, "", underJudgeBand, brokenReading()), sixthCancelled())
	assertRow(t, applyAtAGain(trade, "", legOnAndQuiet()), sixthConfirmed())
	assertActivated(t, Apply(trade, "", underJudgeBand, legOnAndQuiet()), sixthConfirmed(), quietActivation(trade, underJudgeBand))
}

// The rows hold prices, so a later fill at exactly the price the ladder is
// pending from reads as judged: the accepted limit. No row of the gate's is
// written and the shared action acts on the ladder as on any pending one: the
// trailingBuy of a ladder in buy is activated with the activation row alone, in place
// of an arming, and a ladder in trailingBuy is sold.
func TestApplyReadsARefillAtThePendingPriceAsJudged(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingAtFive()
	trade.History = append(trade.History, aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: pendingFromFifth, OrderId: 7, CreatedAt: testutil.At("23:30:00")})
	trade.PositionPrice = pendingFromFifth
	st := rebuildState(trade)
	if st.lastFill().Price != pendingFromFifth || slowDeclineFillUnjudged(trade, st) {
		t.Fatalf("fixture drifted: the refill must be the newest fill, at the pending price, got %+v", st)
	}
	armed := armingPrice(trade)
	assertActivated(t, Apply(trade, "trailingBuy", armed, brokenReading()), quietActivation(trade, armed))
	assertSold(t, Apply(restingIn(trade, "trailingBuy"), "trailingBuy", armed, brokenReading()), reasonSlowDecline)
	assertNoSlowDeclineRow(t, applyAtAGain(trade, "", brokenReading()), "")
	assertActivated(t, Apply(trade, "", judgeBand, brokenReading()), quietActivation(trade, judgeBand))
}

// The judgement goes out before the protected return, like the going-pending
// row: a close the ladder proposes, or a trade resting in one, still gets its
// row, and the close is never replaced.
func TestApplyJudgesTheNewFillBesideADecidedClose(t *testing.T) {
	withQuietDecline(t, true)
	resting := pendingAtFive()
	resting.PositionType = "takeProfit"
	got := Apply(resting, "", overJudgeBand, brokenReading())
	assertRow(t, got, sixthCancelled())
	assertNoSale(t, got, "")

	proposed := Apply(pendingAtFive(), "sell", overJudgeBand, legOnAndQuiet())
	assertRow(t, proposed, sixthConfirmed())
	assertNoSale(t, proposed, "sell")
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
// carrying every row that answer handed back, each with its event, appended in
// order and stamped with the tick.
func engineTick(trade aggragates.Trades, position string, price float64, now time.Time, ai aggragates.AIIndicators) (aggragates.Trades, Result) {
	got := Apply(trade, position, price, ai)
	for _, row := range got.Rows {
		trade = withRow(trade, row, now)
	}
	return trade, got
}

// writtenRows lists the trade's log rows in slice order: the text the operator
// reads, the twin of writtenEvents.
func writtenRows(trade aggragates.Trades) []string {
	rows := make([]string, len(trade.Logs))
	for index, logged := range trade.Logs {
		rows[index] = fmt.Sprintf("%s@%v", logged.Message, logged.Price)
	}
	return rows
}

// writtenEvents lists the trade's slow decline events in slice order, each as
// its kind and the price it carries, in the notation the rows are listed in
// (expectedRows).
func writtenEvents(t *testing.T, trade aggragates.Trades) []string {
	t.Helper()
	var listed []string
	for _, event := range trade.StrategyEventsOf(aggragates.StrategyParamSmartSell, GateSlowDecline) {
		var data EventData
		if err := event.DecodeData(&data); err != nil {
			t.Fatalf("event %+v does not decode: %v", event, err)
		}
		listed = append(listed, fmt.Sprintf("%s@%v", data.Event, data.Price))
	}
	return listed
}

// expectedRows lists rows as writtenRows lists the rows written, and as
// writtenEvents lists their events: the message and kind first, the price
// after.
func expectedRows(rows ...Row) (messages, events []string) {
	for _, row := range rows {
		messages = append(messages, fmt.Sprintf("%s@%v", row.Message, row.Price))
		events = append(events, fmt.Sprintf("%s@%v", row.Event, row.Price))
	}
	return messages, events
}

// verdictReading is the verdict served with the fill window: the reading a
// watched ladder that is not pending goes pending on.
func verdictReading() aggragates.AIIndicators {
	reading := legOnAndQuiet()
	reading.SmartSell.QuietDeclineVerdict = true
	return reading
}

// pastTheNewestFillsTakeProfit is a price past the take profit measured from
// the trade's newest fill — its row's Percentage plus Tolerance — and under
// judgeBand, itself under break even: a print of the dead zone where a sale
// off the newest fill once fired on a pending ladder under break even, and where
// nothing sells now.
func pastTheNewestFillsTakeProfit(t *testing.T, trade aggragates.Trades) float64 {
	t.Helper()
	row := trade.StrategyPair.StrategySettings[0]
	fromNewest := trade.PositionPrice / (1 - (row.Percentage+row.Tolerance)/100)
	price := (fromNewest + judgeBand) / 2
	if !(fromNewest < price && price < judgeBand && judgeBand < ladder.AverageEntryPrice(trade)) {
		t.Fatalf("fixture drifted: the newest fill's take profit %v must sit under %v, under break even %v", fromNewest, judgeBand, ladder.AverageEntryPrice(trade))
	}
	return price
}

// A ladder judged fill after fill folds the events beside the rows Apply
// itself hands back, in the order the engines append them. The verdict marks
// it pending at its newest fill, on a print that books a gain with a buy order resting: the ladder rests in
// buy and goes on buying its depths, where a loss would have the shared action
// activate its trailingBuy on that very tick. Two fills land while sophos serves no
// reading: nothing is judged and nothing is activated or sold, an arming
// included, and the first tick that reads the window judges the newest of them
// once — the cancelled row at its price — while the ticks after it write nothing
// and act on nothing, on an arming or past the newest fill's take profit, and the
// take profit reads the average alone. The verdict marks the ladder pending again
// at that fill, where the action activates the trailingBuy of a ladder in buy at a
// loss and sells one in trailingBuy, whatever either proposes; the next fill, on a
// broken reading, is cancelled in turn — pending, cancelled, pending, cancelled —
// and the ladder is not pending. Marked pending once more, the fill that takes it
// to its last depth is never judged, and the action acts on it all the same, the
// depth bound being off here: capital protection, which watches it from that fill
// on, is served no band.
func TestApplyFoldsItsOwnRowsFillAfterFill(t *testing.T) {
	withQuietDecline(t, true)
	noReading := withBlock(aggragates.SmartSellIndicators{})
	w3s := fills(8, "21:30:00")
	fill5, fill6, fill7, fill8 := w3s[4].Price, w3s[5].Price, w3s[6].Price, w3s[7].Price
	cancel := func(fill float64) Row { return CancelledRow(fill, quietFinding(quietDeclineBreakReasons)) }

	trade := testutil.LadderTrade(false, fills(4, "17:38:00")...)
	if ladder.ConfiguredDepths(trade) != 8 || fill5 != pendingFromFifth || fill6 != sixthFill || trade.PositionPrice != slowDeclineLastFill {
		t.Fatal("fixture drifted: the w3s row is eight deep, its fourth fill the marked one, its fifth and sixth the ones the other fixtures judge")
	}

	trade, got := tickAtAGain(trade, "", testutil.At("18:00:00"), verdictReading())
	assertRow(t, got, quietPending(slowDeclineLastFill))
	assertNoSale(t, got, "")

	trade = withFill(trade, fill5, testutil.At("18:30:00"))
	for _, position := range []string{"", "trailingBuy"} {
		next, got := engineTick(trade, position, printFor(trade, position), testutil.At("18:45:00"), noReading)
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
	assertRow(t, got, cancel(fill6))
	assertNoSale(t, got, "")
	breakEven := ladder.AverageEntryPrice(trade)
	overBreakEven := breakEven + 1
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, reading := range []aggragates.AIIndicators{brokenReading(), windowReadAlone(), legOnAndQuiet(), noReading} {
			for _, price := range []float64{judgeBand, overJudgeBand, pastTheNewestFillsTakeProfit(t, trade)} {
				assertNoSlowDeclineRow(t, Apply(restingIn(trade, state), "", price, reading), "")
			}
			assertNoSlowDeclineRow(t, Apply(restingIn(trade, state), "trailingBuy", armingPrice(trade), reading), "trailingBuy")
		}
	}
	if got := TakeProfitPercentage(trade, overBreakEven, moveAgainst(overBreakEven, breakEven)); got != moveAgainst(overBreakEven, breakEven) {
		t.Fatalf("a cancelled ladder's take profit reads the average alone, got %v", got)
	}

	trade, got = tickAtAGain(trade, "", testutil.At("20:00:00"), verdictReading())
	assertRow(t, got, quietPending(fill6))
	assertNoSale(t, got, "")
	for _, price := range []float64{pastTheNewestFillsTakeProfit(t, trade), judgeBand} {
		assertActivated(t, Apply(trade, "", price, windowReadAlone()), quietActivation(trade, price))
		assertSold(t, Apply(restingIn(trade, "trailingBuy"), "", price, windowReadAlone()), reasonSlowDecline)
	}
	armed := armingPrice(trade)
	assertActivated(t, Apply(trade, "trailingBuy", armed, windowReadAlone()), quietActivation(trade, armed))
	assertSold(t, Apply(restingIn(trade, "trailingBuy"), "trailingBuy", armed, windowReadAlone()), reasonSlowDecline)
	if got := TakeProfitPercentage(trade, overBreakEven, moveAgainst(overBreakEven, breakEven)); got != moveAgainst(overBreakEven, fill6) {
		t.Fatalf("pending again, the take profit reads the newest fill from break even up, got %v", got)
	}

	trade = withFill(trade, fill7, testutil.At("20:30:00"))
	trade, got = engineTick(trade, "", overJudgeBand, testutil.At("20:45:00"), brokenReading())
	assertRow(t, got, cancel(fill7))
	assertNoSale(t, got, "")
	if st := rebuildState(trade); st.slowDeclinePending || st.slowDeclinePendingFrom != 0 || !st.slowDeclineWatched {
		t.Fatalf("pending, cancelled, pending, cancelled: watched and not pending, got %+v", st)
	}
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, price := range []float64{judgeBand, overJudgeBand, pastTheNewestFillsTakeProfit(t, trade)} {
			assertNoSlowDeclineRow(t, Apply(restingIn(trade, state), "", price, brokenReading()), "")
		}
		assertNoSlowDeclineRow(t, Apply(restingIn(trade, state), "trailingBuy", armingPrice(trade), brokenReading()), "trailingBuy")
	}

	trade, got = tickAtAGain(trade, "", testutil.At("21:15:00"), verdictReading())
	assertRow(t, got, quietPending(fill7))
	trade = withFill(trade, fill8, testutil.At("21:30:00"))
	if ladder.CountFilledEntries(trade) != ladder.ConfiguredDepths(trade) {
		t.Fatal("fixture drifted: the eighth fill must take the ladder to its last depth")
	}
	for _, reading := range []aggragates.AIIndicators{brokenReading(), windowReadAlone(), legOnAndQuiet()} {
		assertNoSlowDeclineRow(t, applyAtAGain(trade, "", reading), "")
		for _, price := range []float64{underJudgeBand, pastTheNewestFillsTakeProfit(t, trade)} {
			assertActivated(t, Apply(trade, "", price, reading), quietActivation(trade, price))
		}
		armed := armingPrice(trade)
		assertActivated(t, Apply(trade, "trailingBuy", armed, reading), quietActivation(trade, armed))
		sold := Apply(restingIn(trade, "trailingBuy"), "trailingBuy", armed, reading)
		assertSold(t, sold, reasonSlowDecline)
	}

	wantRows, wantEvents := expectedRows(quietPending(slowDeclineLastFill), cancel(fill6), quietPending(fill6), cancel(fill7), quietPending(fill7))
	if rows := writtenRows(trade); !reflect.DeepEqual(rows, wantRows) {
		t.Fatalf("rows %v, want %v", rows, wantRows)
	}
	if listed := writtenEvents(t, trade); !reflect.DeepEqual(listed, wantEvents) {
		t.Fatalf("events %v, want the rows' %v", listed, wantEvents)
	}
}
