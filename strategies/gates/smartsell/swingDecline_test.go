package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// withBothDeclines runs the calling test with the features the staircase fixtures
// are read under: the quiet decline, the swing decline and the early take profit
// on and the linear decline off, so that what a tick writes is the staircase's own
// whatever the tree is tuned to. The switches are put back when the test ends.
func withBothDeclines(t *testing.T) {
	t.Helper()
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	withEarlyTakeProfit(t, true)
	withLinearDecline(t, false)
}

// everyProposal is the proposals a ladder of the swing tests is ticked on: none, the
// fill on a bounce, and the three that arm a depth. The shared action decides by the
// ladder's state and never reads the proposal, so every one of them is answered alike.
func everyProposal() []string {
	return append(slices.Clone(ordinaryProposals), armingProposals...)
}

// swingTick is one tick of a ladder: the proposal it made and the print it made it at.
type swingTick struct {
	position string
	price    float64
}

// everyTick is the ticks the swing tests ask a ladder on: every proposal at four prints
// on the pair's two-decimal grid. Three sit under the ladder's break even, where a sale
// of the whole position books a real loss: a third and two thirds of the way from the
// newest fill up to it, and the print the engines arm the next depth at (armingPrice).
// The fourth sits over it (gainPrint) with a buy order resting, where the sale books a gain. The trigger takes any
// loss while these tests run (anyLoss), so every print reaches it, a gain as a loss.
func everyTick(trade aggragates.Trades) []swingTick {
	newest, breakEven := trade.PositionPrice, ladder.AverageEntryPrice(trade)
	prices := []float64{
		helpers.ToFixed(newest+(breakEven-newest)/3, 2),
		helpers.ToFixed(newest+2*(breakEven-newest)/3, 2),
		armingPrice(trade),
		gainPrint(trade),
	}
	var ticks []swingTick
	for _, position := range everyProposal() {
		for _, price := range prices {
			ticks = append(ticks, swingTick{position, price})
		}
	}
	return ticks
}

// assertActedOn ticks the trade, whose pending ladder's trigger stands, on every tick
// (everyTick) and fails unless the shared action does what the ladder's state says,
// whatever the ladder proposes. The ladder resting in buy has its trailingBuy activated on
// each print, a gain over its break even included: Position trailingBuy, Result.Activated,
// no Reason, and the activation row after rows, the gate's own rows of the tick (none
// when the ladder is pending already), naming the profit a sale would book and the
// decline features the ladder is pending on once those rows are written. The ladder
// resting in trailingBuy is sold on every print with rows beside the sale and no row of
// the action's own: the sold row is the engines' ExitRow.
func assertActedOn(t *testing.T, trade aggragates.Trades, ai aggragates.AIIndicators, rows ...Row) {
	t.Helper()
	fill := rebuildState(trade).lastFill().Price
	features := rebuildState(withRows(trade, rows...)).pendingFeatures
	if len(features) == 0 {
		t.Fatalf("fixture drifted: the ladder must be pending on a decline feature once the rows %+v are written, got %+v", rows, rebuildState(withRows(trade, rows...)))
	}
	for _, tick := range everyTick(trade) {
		got := Apply(trade, tick.position, tick.price, ai)
		want := append(slices.Clone(rows), ActivatedRow(fill, 0-handLossPct(trade, tick.price), features...))
		if got.Position != activationStateAt(trade, tick.price) || !got.Activated || got.Reason != "" || !sameRows(got.Rows, want) {
			t.Fatalf("resting in buy, on %q at %v: want the sale activated with rows %+v, got %+v", tick.position, tick.price, want, got)
		}
		if sold := Apply(restingIn(trade, "trailingBuy"), tick.position, tick.price, ai); sold.Position != terminalAt(trade, tick.price) || sold.Reason != reasonSlowDecline || sold.Activated || !sameRows(sold.Rows, rows) {
			t.Fatalf("resting in trailingBuy, on %q at %v: want the sale for %q with rows %+v, got %+v", tick.position, tick.price, reasonSlowDecline, rows, sold)
		}
	}
}

// assertSoldNever ticks the trade on every tick (everyTick), resting in buy and in
// trailingBuy, and fails unless each writes exactly these rows and does nothing else: the
// ladder's trigger does not stand (it is not pending, or its newest fill is not judged,
// or it is held), so the shared action neither activates the trailingBuy of the ladder in
// buy nor sells the one in trailingBuy, at any print and whatever it proposes, and the
// proposal comes back as the ladder made it.
func assertSoldNever(t *testing.T, trade aggragates.Trades, ai aggragates.AIIndicators, rows ...Row) {
	t.Helper()
	for _, state := range []string{"buy", "trailingBuy"} {
		resting := restingIn(trade, state)
		for _, tick := range everyTick(trade) {
			if got := Apply(resting, tick.position, tick.price, ai); got.Position != tick.position || got.Reason != "" || got.Activated || !sameRows(got.Rows, rows) {
				t.Fatalf("resting in %s, on %q at %v: want %+v and nothing done, got %+v", state, tick.position, tick.price, rows, got)
			}
		}
	}
}

// stairTurns is a staircase on the bars of stairBars — bars 0 to 49 — which reads
// a decline at each of its fills' bars under the rule as it ships: from the 100
// on bar 4, two down legs that count and one bounce that does by bar 23, three
// and two by bar 36; the first bounce, to the 96.5, is over the bound and is not counted.
var stairTurns = []float64{94, 100, 93, 96.5, 89.5, 91.5, 85, 87, 81, 83, 77}

// stairBounce is the same staircase rallying from its 91.5 instead of falling,
// which reads no decline at bar 30: the only up leg that counted was the
// provisional one, and the rally runs it past the bound.
var stairBounce = []float64{94, 100, 93, 96.5, 89.5, 91.5, 96, 98}

// stairBars are the bars the staircases' turns sit on: six-bar legs, the bounce
// after the first leg a short one, and every turn past the schedule six bars on.
var stairBars = []int{0, 4, 10, 13, 19, 25, 31, 34, 40, 43, 49}

// stairBarOf is the bar of the staircase's turn at index.
func stairBarOf(index int) int {
	if index < len(stairBars) {
		return stairBars[index]
	}
	return stairBars[len(stairBars)-1] + 6*(index-len(stairBars)+1)
}

// stairPrices are the w3s ladder's entry prices, the fills of stairLadder.
var stairPrices = []float64{201.46, 192.81, 188.58, 184.45, 179.78, 175.83, 171.97, 168.2}

// stairOpen is the open time of the staircase's bar: the fixture day's hours.
func stairOpen(bar int) time.Time {
	return testutil.At("00:00:00").Add(time.Duration(bar) * time.Hour)
}

// stairBlock serves bars 0 to lastBar of the staircase of turns as the block's
// series, straight from turn to turn on the bars of stairBars with flat wicks on
// the closes, with the fill window beside them — sophos read its window, so the
// quiet decline's reading of a new fill is taken, and misses where nothing else is
// served — and a Trend Direction row that just meets the need (withSmcTrend,
// patternTrendDown of patternTrendValid rows down): the tests that read a
// staircase's shape read it under a trend that never decides.
func stairBlock(turns []float64, lastBar int) aggragates.SmartSellIndicators {
	closes := []float64{turns[0]}
	for index := 1; index < len(turns); index++ {
		from, to := turns[index-1], turns[index]
		bars := stairBarOf(index) - stairBarOf(index-1)
		for step := 1; step <= bars; step++ {
			closes = append(closes, from+(to-from)*float64(step)/float64(bars))
		}
	}
	block := aggragates.SmartSellIndicators{QuietDeclineFillFrom: fillWindowFrom.UnixMilli()}
	for bar := 0; bar <= lastBar && bar < len(closes); bar++ {
		block.SeriesOpens = append(block.SeriesOpens, stairOpen(bar).UnixMilli())
		block.SeriesCloses = append(block.SeriesCloses, closes[bar])
		block.SeriesHighs = append(block.SeriesHighs, closes[bar])
		block.SeriesLows = append(block.SeriesLows, closes[bar])
	}
	return withSmcTrend(block, patternTrendDown, patternTrendValid)
}

// stairLadder is the w3s ladder with a fill inside each of these bars of the
// staircase, at the w3s prices, in order.
func stairLadder(bars ...int) aggragates.Trades {
	fills := make([]testutil.LadderFill, len(bars))
	for index, bar := range bars {
		fills[index] = testutil.LadderFill{Price: stairPrices[index], At: stairOpen(bar).Add(17*time.Minute + 9*time.Second)}
	}
	return testutil.LadderTrade(false, fills...)
}

// stairReading is what the swing decline reads for the ladder on the block's
// series and trend: swingdecline.Trigger, whose own tests pin its texts, so the
// rows these tests expect follow whatever constants the rule is tuned to. It
// fails the test when the reading is not the one asked for.
func stairReading(t *testing.T, trade aggragates.Trades, block aggragates.SmartSellIndicators, holds bool) swingdecline.Reading {
	t.Helper()
	reading, got := swingdecline.Trigger(swingFills(rebuildState(trade).fills), series.SeriesOf(block), series.TrendOf(block))
	if got != holds {
		t.Fatalf("fixture drifted: the swing decline reads holds=%v for the ladder, want %v: %+v", got, holds, reading)
	}
	return reading
}

// stairFourthBlock and stairFifthBlock are the staircase's series as the ticks
// that judge the fourth and the fifth fill serve it: closed to bar 24 and to bar
// 30, the bars the fills sit in.
func stairFourthBlock() aggragates.SmartSellIndicators {
	return stairBlock(stairTurns, 24)
}

func stairFifthBlock() aggragates.SmartSellIndicators {
	return stairBlock(stairTurns, 30)
}

// stairFourth is the ladder the staircase reads on at bar 24, its fourth
// fill's bar, before any tick; stairPending is that ladder with the rows of the
// tick it went pending on written: the one row, pending on the swing decline and
// latching the early take profit beside it. The tick runs at a print over the
// ladder's break even with a buy order resting (tickAtAGain): the shared action
// waits in silence there and the tick writes the gate's row alone, the state the
// next ticks of the tests start from.
func stairFourth() aggragates.Trades {
	return stairLadder(0, 9, 15, 24)
}

func stairPending(t *testing.T) aggragates.Trades {
	t.Helper()
	trade, got := tickAtAGain(stairFourth(), "", testutil.At("00:30:00"), withBlock(stairFourthBlock()))
	if len(got.Rows) != 1 || got.Rows[0].Event != EventPending || got.Activated || got.Reason != "" {
		t.Fatalf("fixture drifted: the staircase must go pending at bar 24, the tick writing that row alone, got %+v", got)
	}
	return trade
}

// stairFifthFill is the pending, latched staircase ladder (stairPending) with a
// fifth fill stamped inside bar 30, the bar the staircase's second reading ends at.
func stairFifthFill(t *testing.T) aggragates.Trades {
	t.Helper()
	return withFill(stairPending(t), stairPrices[4], stairOpen(30).Add(17*time.Minute+9*time.Second))
}

// The swing decline goes pending on the staircase at the fourth fill, and the
// row is the one a ladder that latches too writes: pending on the swing
// decline, the early take profit beside it with no reasons of its own.
func stairPendingRow(t *testing.T) Row {
	t.Helper()
	return PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(stairReading(t, stairFourth(), stairFourthBlock(), true).Reasons))
}

// The read is bound to nothing but the series' last closed bar: on every tick the
// swing decline reads the windows from the earlier fills up to it, so the pending
// row is written on every tick whose last bar holds — before the bar of the newest
// fill has closed, at that bar and past it alike — and on none whose last bar does
// not. A ladder already deep at deploy, or whose bars after a fill got no tick,
// triggers late. The ticks run at a print over the ladder's break even (gainPrint) with a buy order resting,
// where the ladder in buy is activated nothing, so the only row a tick can write is
// the gate's own.
func TestSwingDeclineReadsTheLastClosedBarOnEveryTick(t *testing.T) {
	trade, turns := stairFourth(), append(slices.Clone(stairTurns), 84, 83, 82, 81)
	var before, after bool
	for lastBar := 0; lastBar < 1+3*(len(turns)-1); lastBar++ {
		block := stairBlock(turns, lastBar)
		_, holds := readsAsOf(trade, block)
		if got := applyAtAGain(trade, "", withBlock(block)); (len(got.Rows) != 0) != holds {
			t.Errorf("series to bar %d: pending row %v, want %v", lastBar, len(got.Rows) != 0, holds)
		}
		before, after = before || (holds && lastBar < 24), after || (holds && lastBar > 24)
	}
	needsAScenario(t, before && after, "a staircase that reads before the newest fill's bar closes and after it")
}

// No series, no read: a block without the keys, one that serves none, and one
// whose arrays differ in length leave the ladder as it was — no row, and nothing
// done on any proposal, at any print, in buy or in trailingBuy: a ladder that did not
// go pending has no trigger for the shared action to act on.
func TestSwingDeclineWaitsWithoutASeries(t *testing.T) {
	withBothDeclines(t)
	mismatched := stairFourthBlock()
	mismatched.SeriesCloses = mismatched.SeriesCloses[1:]
	for name, block := range map[string]aggragates.SmartSellIndicators{
		"no keys":    windowReadAlone().SmartSell,
		"empty":      {SeriesOpens: []int64{}, SeriesCloses: []float64{}},
		"mismatched": mismatched,
	} {
		t.Run(name, func(t *testing.T) {
			assertSoldNever(t, stairFourth(), withBlock(block))
		})
	}
}

// Only a long spot parent ladder from swingdecline.ArmDepth fills
// is read: one fill short, an inverse ladder, a futures one, a child and a
// strategy without the flag write nothing; an impasse strategy's parent is read
// like any other. A futures ladder is watched by none of the declines, the quiet
// decline reading long spot ladders alone. With the swing decline's switch off the
// gate goes pending on nothing, but the early take profit's latch on the swing
// reading fires whatever swingdecline.Enabled says, so the row is the latch's alone
// until that latch is off too.
func TestSwingDeclineWatchesLongSpotParentsFromTheArmDepth(t *testing.T) {
	withLinearDecline(t, false)
	block := withBlock(stairFourthBlock())
	// One fill short of the arm depth, the newest still in bar 24.
	short := append(append([]int(nil), []int{0, 9, 15}[:max(0, min(swingdecline.ArmDepth-2, 3))]...), 24)
	for name, tc := range map[string]struct {
		mutate func(*aggragates.Trades)
		reads  bool
	}{
		"a long spot parent":          {func(*aggragates.Trades) {}, true},
		"an impasse strategy":         {func(trade *aggragates.Trades) { trade.Strategy.Params.Impasse = true }, true},
		"one fill short of the arm":   {func(trade *aggragates.Trades) { *trade = stairLadder(short...) }, false},
		"a futures ladder":            {func(trade *aggragates.Trades) { trade.Strategy.TradeType = aggragates.Futures }, false},
		"a child":                     {func(trade *aggragates.Trades) { trade.ParentID = 7 }, false},
		"a strategy without the flag": {func(trade *aggragates.Trades) { trade.Strategy.Params.SmartSell = false }, false},
		"an inverse ladder": {func(trade *aggragates.Trades) {
			trade.Inverse = true
			for index := range trade.History {
				trade.History[index].Type = "SELL"
			}
		}, false},
	} {
		trade := stairFourth()
		tc.mutate(&trade)
		if got := applyAtAGain(trade, "", block); (len(got.Rows) != 0) != tc.reads {
			t.Errorf("%s: pending row %v, want %v", name, len(got.Rows) != 0, tc.reads)
		}
	}
	withSwingDecline(t, false)
	assertRow(t, Apply(stairFourth(), "", slowDeclineBand-1, block), LatchedRow(stairPrices[3], swingFinding(stairReading(t, stairFourth(), stairFourthBlock(), true).Reasons)))
	withEarlyTakeProfitSwingDecline(t, false)
	assertUntouched(t, Apply(stairFourth(), "", slowDeclineBand-1, block), "")
}

// The swing decline writes one row on the staircase at its fourth fill: the
// ladder goes pending on it, and the early take profit, which watches the
// ladder too, latches on the same row, the event carrying the latch's marker
// (FeatureEarlyTakeProfit; FeatureIndecision only labels sophos' reading in a
// row's message). The row names the window and the
// readings that held, and the trend last. The ladder is pending with its newest fill
// judged on that very tick, so the shared action is asked on it: a ladder in buy
// loses nothing at a print over its break even and gets the row alone, and has its
// trailingBuy activated at a print under it with the activation row after the pending
// one; a ladder in trailingBuy is sold with the row beside the sale. Whatever it proposes.
func TestSwingDeclineGoesPendingAndLatchesOnOneRow(t *testing.T) {
	withBothDeclines(t)
	got := applyAtAGain(stairFourth(), "", withBlock(stairFourthBlock()))
	want := stairPendingRow(t)
	assertRow(t, got, want)
	if want.Event != EventPending || want.Gate != GateSlowDecline || len(want.Features) != 2 || want.Features[0] != FeatureEarlyTakeProfit || want.Features[1] != FeatureSwingDecline {
		t.Fatalf("the row is the gate's pending kind on the early take profit and the swing decline, got %+v", want)
	}
	assertNoSale(t, got, "")
	assertActedOn(t, stairFourth(), withBlock(stairFourthBlock()), want)
	if st := rebuildState(withRow(stairFourth(), want, time.Time{})); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[3] || !st.earlyTakeProfit || len(st.pendingFeatures) != 1 || st.pendingFeatures[0] != FeatureSwingDecline {
		t.Fatalf("the row leaves the ladder pending from its fill on the swing decline, and latched, got %+v", st)
	}
}

// The two steps of the shared action on the staircase, tick after tick as the engines play
// them (engineStep). The tick that goes pending activates the trailingBuy at a print under the
// ladder's break even, the activation row after the pending one, and leaves the ladder resting
// in trailingBuy, anchored at the activation print. From then on the ladder TRAILS on every tick
// with no row, whatever it proposes and whatever sophos serves by then: the print of the
// activation is held, every print over the trail, the one over the break even included, is a
// re-anchor, and a print under the tolerance sells. The activation committed the sale, which
// goes on over the loss cap too. A ladder resting in trailingBuy that no gate activated is
// another matter: over the cap it is held, with the gate's held row.
func TestSwingDeclineActivatesOnTheTickItGoesPendingAndTrailsBeforeItSells(t *testing.T) {
	withBothDeclines(t)
	trade, block := stairFourth(), withBlock(stairFourthBlock())
	armed := armingPrice(trade)
	loss := handLossPct(trade, armed)
	stepped, got := engineStep(trade, "", armed, stairOpen(26), block)
	assertActivated(t, got, stairPendingRow(t), ActivatedRow(stairPrices[3], -loss, FeatureSwingDecline))
	if st := rebuildState(stepped); !st.slowDeclinePending || !st.slowDeclineActivated || stepped.PositionType != "trailingBuy" || stepped.PositionPrice != armed {
		t.Fatalf("the activation leaves the ladder pending, activated and resting in trailingBuy at its print, got %+v resting in %q at %v", st, stepped.PositionType, stepped.PositionPrice)
	}
	falls := priceAtMove(stepped, armed, -(trailRow(stepped).Tolerance + 1))
	for _, ai := range []aggragates.AIIndicators{block, nothingServed()} {
		for _, tick := range everyTick(trade) {
			if answer := Apply(stepped, tick.position, tick.price, ai); tick.price == armed {
				assertTrailHeld(t, answer)
			} else {
				assertReanchored(t, answer)
			}
			assertSold(t, Apply(stepped, tick.position, falls, ai), reasonSlowDecline)
		}
	}

	withAllowedLoss(t, loss/2)
	natural := restingIn(stairPending(t), "trailingBuy")
	for _, position := range everyProposal() {
		assertTrailHeld(t, Apply(stepped, position, armed, block))
		assertSold(t, Apply(stepped, position, falls, block), reasonSlowDecline)
		assertWaits(t, Apply(natural, position, armed, block), position, SaleHeldRow(stairPrices[3], -loss, FeatureSwingDecline))
	}
}

// A ladder the swing decline made pending without a latch — the indecision
// direction was off when it went pending — is latched by the confirmation of
// its next fill once the direction watches it again: the confirmed row names the
// latch beside the swing decline, and no second one is written after it.
func TestSwingDeclineLatchesAConfirmationOnceToo(t *testing.T) {
	withEarlyTakeProfit(t, false)
	pending := stairPending(t)
	withEarlyTakeProfit(t, true)
	if rebuildState(pending).earlyTakeProfit {
		t.Fatal("fixture drifted: the ladder went pending without a latch")
	}
	fifth := withFill(pending, stairPrices[4], stairOpen(30).Add(17*time.Minute+9*time.Second))
	reading := stairReading(t, fifth, stairFifthBlock(), true)
	confirmed, got := tickAtAGain(fifth, "", stairOpen(31), withBlock(stairFifthBlock()))
	assertRow(t, got, ConfirmedRow(stairPrices[4], indecisionFinding(nil), swingFinding(reading.Reasons)))
	sixth := withFill(confirmed, stairPrices[5], stairOpen(36).Add(17*time.Minute))
	sixthBlock := stairBlock(stairDecline, 36)
	again := applyAtAGain(sixth, "", withBlock(sixthBlock))
	assertRow(t, again, ConfirmedRow(stairPrices[5], swingFinding(stairReading(t, sixth, sixthBlock, true).Reasons)))
}
