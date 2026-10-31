package smartsell

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/freefallprevention"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The episode of a long ladder tick by tick (engineStep): the first holding tick opens it with the
// triggered row, over the cap the held row goes out once, the first tick within the cap activates
// with the activation row alone, and from trailingBuy the action trails (hold, re-anchor, sell); only
// the sale closes it.
func TestFreeFallPreventionEpisodeOfALongLadder(t *testing.T) {
	freeFallPreventionAlone(t)
	start := flatMarketLadder(false)
	ai, fill := freeFallMarket(start), start.PositionPrice
	atTheCap, oneStepLower := flatMarketPrices(t, start)
	held := FreeFallPreventionSaleHeldRow(fill, -handLossPct(start, oneStepLower))
	activation := FreeFallPreventionActivatedRow(fill, -handLossPct(start, atTheCap))

	trade, got := engineStep(start, "", oneStepLower, testutil.At("13:00:00"), ai)
	assertWaits(t, got, "", freeFallTriggeredBy(start), held)
	if st := rebuildState(trade); !st.freeFallPreventionWatched || !st.freeFallPreventionActive || !st.freeFallPreventionSaleHeld || st.freeFallPreventionActivated || st.freeFallPreventionFrom != fill {
		t.Fatalf("the first tick opens the episode and the cap holds the action, got %+v", st)
	}
	if st := rebuildState(trade); st.flatMarketActive || st.slowDeclinePending {
		t.Fatalf("no other gate's episode opens, got %+v", st)
	}
	trade, got = engineStep(trade, "buy", oneStepLower, testutil.At("13:05:00"), ai)
	assertUntouched(t, got, "buy")
	trade, got = engineStep(trade, "update_trailingBuy", oneStepLower-1, testutil.At("13:15:00"), ai)
	assertUntouched(t, got, "update_trailingBuy")

	trade, got = engineStep(trade, "trailingBuy", atTheCap, testutil.At("13:30:00"), ai)
	assertActivated(t, got, activation)
	if trade.PositionType != "trailingBuy" {
		t.Fatalf("the activation chain leaves the ladder in trailingBuy, got %q", trade.PositionType)
	}
	if st := rebuildState(trade); !st.freeFallPreventionActive || !st.freeFallPreventionActivated || !st.freeFallPreventionSaleHeld || st.freeFallPreventionFrom != fill {
		t.Fatalf("the episode is open with its trailingBuy activated, got %+v", st)
	}

	gain, falls := ladder.AverageEntryPrice(start)*gainFactor, priceAtMove(trade, atTheCap, -(trailRow(trade).Tolerance+1))
	gone := freeFallMisses(aggragates.SideLong)["no row in free fall"]
	for _, reading := range []aggragates.AIIndicators{ai, gone} {
		for _, position := range addSideProposals {
			assertTrailHeld(t, Apply(trade, position, atTheCap, reading))
			assertTrailHeld(t, Apply(trade, position, oneStepLower, reading))
			assertReanchored(t, Apply(trade, position, gain, reading))
			assertSold(t, Apply(trade, position, falls, reading), reasonFreeFallPrevention)
		}
	}

	moved, got := engineStep(trade, "buy", gain, testutil.At("13:35:00"), gone)
	assertReanchored(t, got)
	if st := rebuildState(moved); moved.PositionPrice != gain || moved.PositionType != "trailingBuy" || !st.freeFallPreventionActive || !st.freeFallPreventionActivated || st.freeFallPreventionFrom != fill {
		t.Fatalf("a re-anchor moves the anchor to the print and the episode stands in trailingBuy, got %v %q %+v", moved.PositionPrice, moved.PositionType, st)
	}

	sold := Apply(trade, "buy", falls, gone)
	closed := withRow(trade, ExitRow(trade, sold.Reason, falls), testutil.At("13:45:00"))
	if st := rebuildState(closed); st.freeFallPreventionActive || st.freeFallPreventionSaleHeld || st.freeFallPreventionActivated || st.freeFallPreventionFrom != 0 {
		t.Fatalf("the sold row closes the episode, got %+v", st)
	}
	want := []string{eventAt(EventTriggered, fill), eventAt(EventSaleHeld, fill), eventAt(EventActivated, fill), eventAt(EventSold, falls)}
	if got := gateEvents(t, closed, GateFreeFallPrevention); !reflect.DeepEqual(got, want) {
		t.Errorf("events = %q, want %q", got, want)
	}
}

// The action reads the ladder's state and never its proposal: buy is activated within the cap on
// any proposal with the triggered row beside the activation, trailingBuy (force-trailing too) is sold
// with the triggered row alone, a close comes back as it came, a take profit in buy gives way to the
// activation within the cap and stands beside the held row over it, and any other state only opens
// the episode.
func TestFreeFallPreventionActivatesInBuyAndSellsInStopLossWhateverTheLadderProposes(t *testing.T) {
	freeFallPreventionAlone(t)
	trade := flatMarketLadder(false)
	ai, fill := freeFallMarket(trade), trade.PositionPrice
	atTheCap, oneStepLower := flatMarketPrices(t, trade)
	loss := handLossPct(trade, atTheCap)

	for _, position := range addSideProposals {
		assertActivated(t, Apply(trade, position, atTheCap, ai), freeFallTriggeredBy(trade), FreeFallPreventionActivatedRow(fill, -loss))
		for _, state := range []string{"trailingBuy", "forceTrailingBuy"} {
			assertSoldWith(t, Apply(restingIn(trade, state), position, atTheCap, ai), reasonFreeFallPrevention, freeFallTriggeredBy(trade))
		}
		for _, state := range []string{"new", ""} {
			assertWaits(t, Apply(restingIn(trade, state), position, atTheCap, ai), position, freeFallTriggeredBy(trade))
		}
	}
	for _, position := range closingProposals {
		assertUntouched(t, Apply(trade, position, atTheCap, ai), position)
	}
	for _, position := range takeProfitProposals {
		assertActivated(t, Apply(trade, position, atTheCap, ai), freeFallTriggeredBy(trade), FreeFallPreventionActivatedRow(fill, -loss))
		held := FreeFallPreventionSaleHeldRow(fill, -handLossPct(trade, oneStepLower))
		assertWaits(t, Apply(trade, position, oneStepLower, ai), position, freeFallTriggeredBy(trade), held)
	}
	for _, resting := range []string{"takeProfit", "forceTrailingTakeProfit", "takeLoss", "sell", "sellParent", "impasse"} {
		for _, position := range append([]string{""}, addSideProposals...) {
			assertUntouched(t, Apply(restingIn(trade, resting), position, atTheCap, ai), position)
		}
	}
}

// Over the cap the action waits and the gate holds nothing else: the held row goes out once beside
// the triggered row, a ladder in buy is activated back within the cap and a self-armed trailingBuy is
// sold back within it, with no second held row; the depth bound is read before the cap and its row
// says so, and a dashboard that meets no rule holds the sale on the trend, once.
func TestFreeFallPreventionWaitsOverTheCapTheDepthBoundAndTheTrend(t *testing.T) {
	freeFallPreventionAlone(t)
	trade := flatMarketLadder(false)
	ai, fill := freeFallMarket(trade), trade.PositionPrice
	atTheCap, oneStepLower := flatMarketPrices(t, trade)
	held := FreeFallPreventionSaleHeldRow(fill, -handLossPct(trade, oneStepLower))

	for _, state := range []string{"buy", "trailingBuy"} {
		resting := restingIn(trade, state)
		for _, position := range addSideProposals {
			got := Apply(resting, position, oneStepLower, ai)
			assertWaits(t, got, position, freeFallTriggeredBy(trade), held)

			written := withRows(resting, got.Rows...)
			for _, again := range addSideProposals {
				assertUntouched(t, Apply(written, again, oneStepLower, ai), again)
				assertUntouched(t, Apply(written, again, oneStepLower-1, ai), again)
			}
			if state == "buy" {
				assertActivated(t, Apply(written, position, atTheCap, ai), FreeFallPreventionActivatedRow(fill, -handLossPct(trade, atTheCap)))
			} else {
				assertSold(t, Apply(written, position, atTheCap, ai), reasonFreeFallPrevention)
			}
		}
	}

	ceiling := ladder.ConfiguredDepths(trade)
	withSaleIgnoreLastDepths(t, ceiling)
	atDepth := FreeFallPreventionSaleHeldAtDepthRow(fill, len(rebuildState(trade).fills), ceiling)
	for _, price := range []float64{atTheCap, oneStepLower} {
		assertWaits(t, Apply(trade, "", price, ai), "", freeFallTriggeredBy(trade), atDepth)
	}
	withSaleIgnoreLastDepths(t, 0)

	unread := inFreeFall(withTrendCounts(aggragates.SmartSellIndicators{}, 0, 0), []bool{true, false, false}, nil)
	byTrend := FreeFallPreventionSaleHeldByTrendRow(fill, unread, false)
	if !strings.Contains(byTrend.Message, "SMC trend meets no trigger rule") || !strings.Contains(byTrend.Message, "1h 0/9 free fall") {
		t.Fatalf("fixture drifted: the dashboard must meet no rule and name the 1h in free fall, got %q", byTrend.Message)
	}
	got := Apply(trade, "", atTheCap, withBlock(unread))
	assertWaits(t, got, "", FreeFallPreventionTriggeredRow(fill, freeFallLastBar), byTrend)
	assertUntouched(t, Apply(withRows(trade, got.Rows...), "", atTheCap, withBlock(unread)), "")
}

// The gate watches a spot ladder from freefallprevention.ArmDepth filled entries, long or inverse:
// one fill short of it the ladder is not watched and not armed for it, futures and an unfilled
// ladder are never watched, a child and a trade without the flag are never armed, and a ladder off
// the watch writes no row and sells nothing (no settings is answered by Apply alone).
func TestFreeFallPreventionWatchesFromItsArmDepthOnASpotLadder(t *testing.T) {
	freeFallPreventionAlone(t)
	short := testutil.LadderTrade(false, fills(freefallprevention.ArmDepth-1, "17:38:00")...)
	armed := testutil.LadderTrade(false, fills(freefallprevention.ArmDepth, "17:38:00")...)
	if freeFallPreventionWatched(short) || !freeFallPreventionWatched(armed) {
		t.Fatalf("the gate watches from %d fills: %d fills watched %v, %d fills watched %v", freefallprevention.ArmDepth,
			freefallprevention.ArmDepth-1, freeFallPreventionWatched(short), freefallprevention.ArmDepth, freeFallPreventionWatched(armed))
	}
	ai := freeFallMarket(armed)
	assertUntouched(t, applyAtAGain(short, "", ai), "")
	if Armed(short) || !Armed(armed) {
		t.Fatalf("the gate alone arms the ladder from its arm depth: short %v, armed %v", Armed(short), Armed(armed))
	}
	fill := rebuildState(armed).lastFill().Price
	assertRow(t, applyAtAGain(armed, "", ai), FreeFallPreventionTriggeredRow(fill, freeFallLastBar))
	for _, inverse := range []bool{false, true} {
		trade := testutil.LadderTrade(inverse, fills(freefallprevention.ArmDepth, "17:38:00")...)
		if !freeFallPreventionWatched(trade) || !Armed(trade) {
			t.Errorf("inverse %v: a spot ladder at its arm depth is watched and armed", inverse)
		}
	}

	off := flatMarketOffTheWatch(armed)
	if freeFallPreventionWatched(off["futures"]) || freeFallPreventionWatched(off["no fill"]) {
		t.Error("the gate watches no futures ladder and no unfilled one")
	}
	for name, trade := range off {
		if name != "no settings" && Armed(trade) {
			t.Errorf("%s: never armed", name)
		}
		assertUntouched(t, Apply(withRestingOrder(trade), "buy", gainPrint(armed), ai), "buy")
	}
}

// An open episode stands whatever its reading does: the reading is read only while no episode is
// open, so a reading that goes writes no row, the trigger keeps standing and the action acts by the
// state as before; the reading coming and going opens and closes nothing, and only the sale closes it.
func TestAFreeFallPreventionEpisodeStandsWhileItsReadingComesAndGoes(t *testing.T) {
	freeFallPreventionAlone(t)
	start := flatMarketLadder(false)
	ai, fill := freeFallMarket(start), start.PositionPrice
	atTheCap, oneStepLower := flatMarketPrices(t, start)
	held := FreeFallPreventionSaleHeldRow(fill, -handLossPct(start, oneStepLower))
	activation := FreeFallPreventionActivatedRow(fill, -handLossPct(start, atTheCap))
	readings := freeFallMisses(aggragates.SideLong)

	activates := func(t *testing.T, got Result, _ string) { assertActivated(t, got, activation) }
	sells := func(t *testing.T, got Result, _ string) { assertSold(t, got, reasonFreeFallPrevention) }
	heldOver := func(t *testing.T, got Result, position string) { assertWaits(t, got, position, held) }
	leaves := func(t *testing.T, got Result, position string) { assertUntouched(t, got, position) }

	open := withRows(start, freeFallTriggeredBy(start))
	waiting, _ := engineStep(start, "", oneStepLower, testutil.At("13:00:00"), ai)
	for _, c := range []struct {
		name                   string
		episode                aggragates.Trades
		atTheCap, oneStepLower func(*testing.T, Result, string)
	}{
		{"open", open, activates, heldOver},
		{"held", waiting, activates, leaves},
		{"natural trailingBuy", restingIn(open, "trailingBuy"), sells, heldOver},
	} {
		for name, reading := range readings {
			for _, position := range addSideProposals {
				for price, want := range map[float64]func(*testing.T, Result, string){atTheCap: c.atTheCap, oneStepLower: c.oneStepLower} {
					got := Apply(c.episode, position, price, reading)
					want(t, got, position)
					if !reflect.DeepEqual(got, Apply(c.episode, position, price, ai)) {
						t.Fatalf("%s, reading %q: the answer at %v differs from the one under a reading that holds, got %+v", c.name, name, price, got)
					}
					if st := rebuildState(withRows(c.episode, got.Rows...)); !st.freeFallPreventionActive || st.freeFallPreventionFrom != fill {
						t.Fatalf("%s, reading %q: the episode stands, got %+v", c.name, name, st)
					}
				}
			}
		}
	}

	tick := func(trade aggragates.Trades, reading aggragates.AIIndicators) (aggragates.Trades, state, []Row, bool) {
		next, rows, holds := freeFallPreventionTick(trade, rebuildState(trade), reading.SmartSell, reading)
		return withRows(trade, rows...), next, rows, holds
	}
	trade, next, rows, holds := tick(start, ai)
	if want := []Row{freeFallTriggeredBy(start)}; !reflect.DeepEqual(rows, want) || !holds || !next.freeFallPreventionActive || next.freeFallPreventionFrom != fill || next.freeFallPreventionSaleHeld || next.freeFallPreventionActivated {
		t.Fatalf("the first tick opens the episode, got rows %+v, holds %v, state %+v", rows, holds, next)
	}
	for name, reading := range readings {
		for _, again := range []aggragates.AIIndicators{reading, ai} {
			trade, next, rows, holds = tick(trade, again)
			if len(rows) != 0 || !holds || !next.freeFallPreventionActive || next.freeFallPreventionFrom != fill {
				t.Fatalf("%s: the episode stands with no row and its trigger, got rows %+v, holds %v, state %+v", name, rows, holds, next)
			}
		}
	}
	if want := []string{eventAt(EventTriggered, fill)}; !reflect.DeepEqual(gateEvents(t, trade, GateFreeFallPrevention), want) {
		t.Fatalf("events = %q, want %q", gateEvents(t, trade, GateFreeFallPrevention), want)
	}
}

// A new depth resets the episode on that tick with the reset row at the new fill, whatever it held;
// the reading is read afresh and a holding one opens a new episode with its own triggered row on the
// same tick (held row due again, activation within the cap); a miss leaves the reset row alone, and
// with the reset off the episode stands through the new fill as it was.
func TestFreeFallPreventionResetsAtANewDepthAndOpensAnotherEpisodeOnTheSameTick(t *testing.T) {
	freeFallPreventionAlone(t)
	start := flatMarketLadder(false)
	ai, fill := freeFallMarket(start), start.PositionPrice
	_, oneStepLower := flatMarketPrices(t, start)
	const newest = 96.0
	misses := freeFallMisses(aggragates.SideLong)

	open := withRows(start, freeFallTriggeredBy(start))
	waiting := withRows(open, FreeFallPreventionSaleHeldRow(fill, -handLossPct(start, oneStepLower)))
	activated := committedBy(open, FreeFallPreventionActivatedRow(fill, -0.25))
	deeper := func(trade aggragates.Trades) aggragates.Trades {
		return withFill(trade, newest, testutil.At("14:00:00"))
	}
	far := ladder.AverageEntryPrice(deeper(start)) * lossFactor
	cheap := aLossWithinTheCap(t, deeper(start))
	reopened := FreeFallPreventionTriggeredRow(newest, freeFallLastBar)

	for name, episode := range map[string]aggragates.Trades{"open": open, "held": waiting, "activated": activated} {
		onTheNewDepth := deeper(episode)
		overTheCap := FreeFallPreventionSaleHeldRow(newest, -handLossPct(onTheNewDepth, far))

		got := Apply(onTheNewDepth, "buy", far, ai)
		assertWaits(t, got, "buy", FreeFallPreventionResetRow(newest), reopened, overTheCap)
		written, _ := engineStep(onTheNewDepth, "buy", far, testutil.At("14:05:00"), ai)
		if st := rebuildState(written); !st.freeFallPreventionActive || st.freeFallPreventionActivated || !st.freeFallPreventionSaleHeld || st.freeFallPreventionFrom != newest {
			t.Fatalf("%s: the new episode is open at the new fill with its action held and no trailingBuy activated, got %+v", name, st)
		}
		assertUntouched(t, Apply(written, "buy", far, ai), "buy")

		assertActivated(t, Apply(restingIn(onTheNewDepth, "buy"), "", cheap, ai),
			FreeFallPreventionResetRow(newest), reopened, FreeFallPreventionActivatedRow(newest, -lossPctOf(onTheNewDepth, cheap)))
		assertSoldWith(t, Apply(restingIn(onTheNewDepth, "trailingBuy"), "", cheap, ai), reasonFreeFallPrevention, FreeFallPreventionResetRow(newest), reopened)

		for missName, miss := range misses {
			bare := Apply(onTheNewDepth, "buy", far, miss)
			assertWaits(t, bare, "buy", FreeFallPreventionResetRow(newest))
			closed := withRow(onTheNewDepth, bare.Rows[0], testutil.At("14:05:00"))
			if st := rebuildState(closed); st.freeFallPreventionActive || st.freeFallPreventionSaleHeld || st.freeFallPreventionActivated || st.freeFallPreventionFrom != 0 {
				t.Fatalf("%s, reading %q: the reset closes the episode, got %+v", name, missName, st)
			}
			assertUntouched(t, Apply(closed, "buy", far, miss), "buy")
		}
	}

	tick := func(trade aggragates.Trades, reading aggragates.AIIndicators) (aggragates.Trades, state, []Row, bool) {
		next, rows, holds := freeFallPreventionTick(trade, rebuildState(trade), reading.SmartSell, reading)
		return withRows(trade, rows...), next, rows, holds
	}
	reopenedTrade, next, rows, holds := tick(deeper(open), ai)
	if want := []Row{FreeFallPreventionResetRow(newest), reopened}; !reflect.DeepEqual(rows, want) || !holds || !next.freeFallPreventionActive || next.freeFallPreventionFrom != newest || next.freeFallPreventionSaleHeld || next.freeFallPreventionActivated {
		t.Fatalf("a reading that holds reopens the episode on the tick of the reset, got rows %+v, holds %v, state %+v", rows, holds, next)
	}
	if want := []string{eventAt(EventTriggered, fill), eventAt(EventReset, newest), eventAt(EventTriggered, newest)}; !reflect.DeepEqual(gateEvents(t, reopenedTrade, GateFreeFallPrevention), want) {
		t.Errorf("events = %q, want %q", gateEvents(t, reopenedTrade, GateFreeFallPrevention), want)
	}
	for name, miss := range misses {
		_, next, rows, holds = tick(deeper(open), miss)
		if want := []Row{FreeFallPreventionResetRow(newest)}; !reflect.DeepEqual(rows, want) || holds || next.freeFallPreventionActive || next.freeFallPreventionFrom != 0 {
			t.Fatalf("%s: a new depth resets the episode with the reset row alone, got rows %+v, holds %v, state %+v", name, rows, holds, next)
		}
	}

	withFreeFallPreventionResetOnNewDepth(t, false)
	for name, miss := range misses {
		_, next, rows, holds = tick(deeper(open), miss)
		if len(rows) != 0 || !holds || !next.freeFallPreventionActive || next.freeFallPreventionFrom != fill {
			t.Fatalf("%s, reset off: the episode stands through the new fill with no row, got rows %+v, holds %v, state %+v", name, rows, holds, next)
		}
	}
	assertWaits(t, Apply(deeper(open), "buy", far, ai), "buy", FreeFallPreventionSaleHeldRow(newest, -handLossPct(deeper(open), far)))
}

// The share of the window opens the episode as the last bar does: the series served with every bar
// in free fall (free rise for an inverse ladder), the rows in free fall nowhere, the triggered row
// naming the bars, and the series one bar under the share opens nothing.
func TestFreeFallPreventionOpensOnTheWindowsShareAlone(t *testing.T) {
	freeFallPreventionAlone(t)
	bars := freefallprevention.Window
	for _, inverse := range []bool{false, true} {
		trade := flatMarketLadder(inverse)
		side, word := aggragates.SideLong, "free fall"
		if inverse {
			side, word = aggragates.SideShort, "free rise"
		}
		ai := withBlock(freeFallSeries(side, bars))
		reasons := []string{fmt.Sprintf("1h in %s on %d of %d %s bars, over %.0f%%", word, bars, bars, freefallprevention.Timeframe, float64(freefallprevention.MinShare)*100)}
		got := Apply(withRestingOrder(trade), "buy", trade.PositionPrice, ai)
		assertWaits(t, got, "buy", FreeFallPreventionTriggeredRow(trade.PositionPrice, reasons))

		if freefallprevention.MinShare == 0.8 && bars == 240 {
			under := withBlock(freeFallSeries(side, 192))
			assertUntouched(t, Apply(withRestingOrder(trade), "buy", trade.PositionPrice, under), "buy")
			over := withBlock(freeFallSeries(side, 193))
			if got := Apply(withRestingOrder(trade), "buy", trade.PositionPrice, over); len(got.Rows) != 1 || got.Rows[0].Event != EventTriggered {
				t.Errorf("inverse %v: 193 of 240 bars open the episode, got %+v", inverse, got.Rows)
			}
		}
		other := aggragates.SideShort
		if inverse {
			other = aggragates.SideLong
		}
		assertUntouched(t, Apply(withRestingOrder(trade), "buy", trade.PositionPrice, withBlock(freeFallSeries(other, bars))), "buy")
	}
}

// An inverse ladder is read on the mirror: its episode opens on the free rise of the row, not on a
// free fall, and a long ladder opens on the free fall alone; the profit is counted in the base the
// entries sold, and the sale is the shared one.
func TestFreeFallPreventionInverseLadderReadsTheFreeRise(t *testing.T) {
	freeFallPreventionAlone(t)
	long, inverse := flatMarketLadder(false), flatMarketLadder(true)
	fill := inverse.PositionPrice
	rising, falling := freeFallMarket(inverse), freeFallMarket(long)
	reasons := []string{"1h in free rise on the last closed bar"}

	assertWaits(t, Apply(withRestingOrder(inverse), "buy", fill, rising), "buy", FreeFallPreventionTriggeredRow(fill, reasons))
	assertUntouched(t, Apply(withRestingOrder(inverse), "buy", fill, falling), "buy")
	assertUntouched(t, Apply(withRestingOrder(long), "buy", long.PositionPrice, rising), "buy")
	for _, miss := range freeFallMisses(aggragates.SideShort) {
		assertUntouched(t, Apply(withRestingOrder(inverse), "buy", fill, miss), "buy")
	}

	average := ladder.AverageEntryPrice(inverse)
	gain := math.Floor(average/gainFactor*100) / 100
	activation := FreeFallPreventionActivatedRow(fill, -inverseLossPctOf(inverse, gain))
	assertActivated(t, Apply(inverse, "", gain, rising), FreeFallPreventionTriggeredRow(fill, reasons), activation)
	assertSoldWithAt(t, Apply(restingIn(inverse, "trailingBuy"), "", gain, rising), reasonFreeFallPrevention, inverse, gain, FreeFallPreventionTriggeredRow(fill, reasons))
}

// Off with freefallprevention.Enabled the gate watches nothing: no episode is read off its stored
// events, no row goes out, no sale or activation happens in any state, and the ladder is not armed
// for it; the first-fill hold and the latch read whatever the switch says, and stay on.
func TestFreeFallPreventionWatchesNothingAndSellsNothingWithItsSwitchOff(t *testing.T) {
	freeFallPreventionAlone(t)
	withFreeFallPrevention(t, false)
	trade := flatMarketLadder(false)
	ai, fill := freeFallMarket(trade), trade.PositionPrice
	atTheCap, oneStepLower := flatMarketPrices(t, trade)
	open := withRows(trade, freeFallTriggeredBy(trade), FreeFallPreventionSaleHeldRow(fill, -0.9), FreeFallPreventionActivatedRow(fill, -0.25))

	if freeFallPreventionWatched(trade) || Armed(trade) || Triggered(open) {
		t.Fatalf("watched %v, armed %v, triggered %v: the gate off watches, arms and triggers nothing", freeFallPreventionWatched(trade), Armed(trade), Triggered(open))
	}
	if st := rebuildState(open); st.freeFallPreventionWatched || st.freeFallPreventionActive || st.freeFallPreventionSaleHeld || st.freeFallPreventionActivated || st.freeFallPreventionFrom != 0 {
		t.Fatalf("a ladder the gate does not watch reads no episode off its events, got %+v", st)
	}
	for _, episode := range []aggragates.Trades{trade, open} {
		for _, state := range []string{"buy", "trailingBuy", "forceTrailingBuy"} {
			for _, position := range addSideProposals {
				for _, price := range []float64{atTheCap, oneStepLower, ladder.AverageEntryPrice(trade) * gainFactor} {
					assertUntouched(t, Apply(restingIn(episode, state), position, price, ai), position)
				}
			}
		}
	}
	if hold := EntryHold(testutil.LadderTrade(false), aggragates.SideLong, ai); hold.Reason != FreeFallPreventionEntryHoldReason {
		t.Fatalf("the first-fill hold reads whatever the switch says, got %+v", hold)
	}
}

// Every reading that misses leaves the ladder alone: with no episode open a dashboard whose asked
// row is not in free fall, free fall not served or no dashboard at all writes no row and sells
// nothing, in any state and at any print.
func TestFreeFallPreventionDoesNothingWithoutItsReading(t *testing.T) {
	freeFallPreventionAlone(t)
	trade := flatMarketLadder(false)
	atTheCap, oneStepLower := flatMarketPrices(t, trade)
	misses := freeFallMisses(aggragates.SideLong)
	misses["no dashboard"] = withBlock(aggragates.SmartSellIndicators{})
	for name, miss := range misses {
		for _, state := range []string{"buy", "trailingBuy"} {
			for _, position := range addSideProposals {
				for _, price := range []float64{atTheCap, oneStepLower} {
					got := Apply(restingIn(trade, state), position, price, miss)
					if len(got.Rows) != 0 || got.Reason != "" || got.Activated {
						t.Fatalf("%s, state %s, proposal %q at %v: a miss leaves the ladder alone, got %+v", name, state, position, price, got)
					}
				}
			}
		}
	}
}

// A depth priority hold pauses the gate with the pause on (freefallprevention shares the switch of
// the other sale gates): a held ladder opens no episode and sells and activates nothing, an open
// episode neither writes its held row nor sells while the hold stands, though it keeps standing, and
// the ladder's next fill ends the hold and the gate reads again; with the pause off the hold changes nothing.
func TestADepthPriorityHoldPausesTheFreeFallPrevention(t *testing.T) {
	freeFallPreventionAlone(t)
	withDepthPriorityPause(t, true)
	base := flatMarketLadder(false)
	ai, fill := freeFallMarket(base), base.PositionPrice
	_, oneStepLower := flatMarketPrices(t, base)
	held := heldBy(base, testutil.At("13:00:00"))
	if !rebuildState(held).depthPriorityHeld {
		t.Fatal("fixture drifted: the ladder must read held")
	}

	for _, state := range []string{"buy", "trailingBuy"} {
		for _, position := range addSideProposals {
			assertUntouched(t, Apply(restingIn(held, state), position, oneStepLower, ai), position)
		}
	}
	openHeld := heldBy(withRows(base, freeFallTriggeredBy(base)), testutil.At("13:00:00"))
	for _, state := range []string{"buy", "trailingBuy"} {
		assertUntouched(t, Apply(restingIn(openHeld, state), "", oneStepLower, ai), "")
		assertUntouched(t, Apply(restingIn(openHeld, state), "", oneStepLower, freeFallMisses(aggragates.SideLong)["no row in free fall"]), "")
	}
	if !Triggered(openHeld) {
		t.Fatal("the pause holds the sale, not the episode: it stands triggered")
	}

	next := fills(len(rebuildState(base).fills)+1, "19:00:00")[len(rebuildState(base).fills)]
	resumed := withFill(held, next.Price, next.At)
	if rebuildState(resumed).depthPriorityHeld {
		t.Fatal("fixture drifted: the next fill ends the hold")
	}
	far := ladder.AverageEntryPrice(resumed) * lossFactor
	assertWaits(t, Apply(resumed, "", far, ai), "", FreeFallPreventionTriggeredRow(next.Price, freeFallLastBar),
		FreeFallPreventionSaleHeldRow(next.Price, -handLossPct(resumed, far)))

	withDepthPriorityPause(t, false)
	assertWaits(t, Apply(withRestingOrder(held), "buy", fill, ai), "buy", freeFallTriggeredBy(base))
}

// Triggered stands while an episode is open at the newest fill, the pause or not; a reset, the sale
// or a new depth past it leaves nothing, except with the reset off, where the episode stands at any
// fill; the gate off stands triggered on nothing.
func TestTriggeredWhileAFreeFallPreventionEpisodeIsOpen(t *testing.T) {
	withFreeFallPrevention(t, true)
	withDepthPriorityPause(t, true)
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	const deeper = 96.0
	if Triggered(trade) {
		t.Fatal("a watched ladder with no episode stands triggered on nothing")
	}
	open := withRows(trade, FreeFallPreventionTriggeredRow(fill, nil))
	if !Triggered(open) {
		t.Fatal("an episode open at the newest fill stands triggered")
	}
	if !Triggered(heldBy(open, testutil.At("23:00:00"))) {
		t.Fatal("the depth priority pause holds the sale, not the episode")
	}

	newDepth := withFill(open, deeper, testutil.At("14:00:00"))
	for name, closed := range map[string]aggragates.Trades{
		"sold":               withRows(open, ExitRow(trade, reasonFreeFallPrevention, fill)),
		"a new depth filled": newDepth,
		"reset at the depth": withRows(newDepth, FreeFallPreventionResetRow(deeper)),
	} {
		if Triggered(closed) {
			t.Errorf("%s: no episode stands at the newest fill", name)
		}
	}

	withFreeFallPreventionResetOnNewDepth(t, false)
	if !Triggered(newDepth) {
		t.Fatal("with the reset off the episode stands at any fill")
	}
	if Triggered(withRows(newDepth, FreeFallPreventionResetRow(deeper))) || Triggered(withRows(open, ExitRow(trade, reasonFreeFallPrevention, fill))) {
		t.Fatal("a reset or a sale ends it all the same")
	}
	withFreeFallPreventionResetOnNewDepth(t, true)

	futures, child := open, open
	futures.Strategy.TradeType = aggragates.Futures
	child.ParentID = 7
	unflagged := open
	unflagged.Strategy.Params.SmartSell = false
	for name, other := range map[string]aggragates.Trades{"futures": futures, "child": child, "no flag": unflagged} {
		if Triggered(other) {
			t.Errorf("%s: never stands triggered", name)
		}
	}
	withFreeFallPrevention(t, false)
	if Triggered(open) {
		t.Fatal("with freefallprevention.Enabled off the gate watches no ladder and stands triggered on nothing")
	}
}

// foldFreeFall is one case of the free fall prevention fold: the rows after the fills and the
// episode they leave.
type foldFreeFall struct {
	name      string
	rows      []Row
	events    []aggragates.TradesStrategyEvents
	active    bool
	held      bool
	activated bool
	from      float64
}

// The fold reads the episode off the gate's events in slice order: triggered opens it at its price,
// a held or activated event keeps it open and sets its own flag (opening it when none came before),
// reset and sold close it and forget everything, and a triggered event after them opens another;
// unusable events and the events of the other gates leave it as it was.
func TestFreeFallPreventionFoldsItsEpisodeFromItsEvents(t *testing.T) {
	withFreeFallPrevention(t, true)
	withFlatMarket(t, true)
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	triggered := FreeFallPreventionTriggeredRow(fill, freeFallLastBar)
	held := FreeFallPreventionSaleHeldRow(fill, -0.9)
	atDepth := FreeFallPreventionSaleHeldAtDepthRow(fill, 7, 8)
	byTrend := FreeFallPreventionSaleHeldByTrendRow(fill, aggragates.SmartSellIndicators{}, false)
	activated := FreeFallPreventionActivatedRow(fill, -0.25)
	reset := FreeFallPreventionResetRow(fill)
	sold := ExitRow(trade, reasonFreeFallPrevention, fill+1)
	reopened := FreeFallPreventionTriggeredRow(96, freeFallLastBar)

	cases := []foldFreeFall{
		{name: "no event"},
		{name: "triggered", rows: []Row{triggered}, active: true, from: fill},
		{name: "triggered, held", rows: []Row{triggered, held}, active: true, held: true, from: fill},
		{name: "triggered, held by the depth bound", rows: []Row{triggered, atDepth}, active: true, held: true, from: fill},
		{name: "triggered, held by the trend", rows: []Row{triggered, byTrend}, active: true, held: true, from: fill},
		{name: "triggered, activated", rows: []Row{triggered, activated}, active: true, activated: true, from: fill},
		{name: "triggered, held, activated", rows: []Row{triggered, held, activated}, active: true, held: true, activated: true, from: fill},
		{name: "triggered, activated, held", rows: []Row{triggered, activated, held}, active: true, held: true, activated: true, from: fill},
		{name: "held alone, the triggered row lost", rows: []Row{held}, active: true, held: true, from: fill},
		{name: "activated alone, the triggered row lost", rows: []Row{activated}, active: true, activated: true, from: fill},
		{name: "triggered, then held at another price keeps the price it opened at", rows: []Row{triggered, FreeFallPreventionSaleHeldRow(fill+1, -0.9)}, active: true, held: true, from: fill},
		{name: "triggered, held, activated, reset", rows: []Row{triggered, held, activated, reset}},
		{name: "triggered, held, activated, sold", rows: []Row{triggered, held, activated, sold}},
		{name: "triggered, activated, then triggered again opens afresh", rows: []Row{triggered, activated, triggered}, active: true, from: fill},
		{name: "triggered, held, activated, reset, triggered at the new depth", rows: []Row{triggered, held, activated, reset, reopened}, active: true, from: 96},
		{name: "triggered, reset, held, the episode reopened by the held row", rows: []Row{triggered, reset, held}, active: true, held: true, from: fill},
		{name: "triggered, held, sold, activated, the episode reopened by the activated row", rows: []Row{triggered, held, sold, activated}, active: true, activated: true, from: fill},
		{
			name: "an event of the slow decline gate leaves it alone", rows: []Row{triggered, ActivatedRow(fill, -0.25, FeatureQuietDecline), SaleHeldRow(fill, -0.9, FeatureQuietDecline)},
			active: true, from: fill,
		},
		{
			name: "the flat market's events are not the gate's", rows: []Row{triggered, FlatMarketResetRow(fill), FlatMarketSaleHeldRow(fill, -0.9), ExitRow(trade, reasonFlatMarket, fill)},
			active: true, from: fill,
		},
		{name: "the flat market's episode opens nothing here", rows: []Row{FlatMarketTriggeredRow(fill, nil), FlatMarketActivatedRow(fill, -0.25)}},
		{
			name:   "an event with no price, a kind the gate does not have",
			rows:   []Row{triggered},
			events: []aggragates.TradesStrategyEvents{eventOf(GateFreeFallPrevention, `{"event":"held","reasons":["a"]}`), eventOf(GateFreeFallPrevention, legacyDocument(EventPending, fill)), eventOf(GateFreeFallPrevention, legacyDocument(EventActivated, 0))},
			active: true, from: fill,
		},
	}
	for _, c := range cases {
		st := rebuildState(withEvents(withRows(trade, c.rows...), c.events...))
		if st.freeFallPreventionActive != c.active || st.freeFallPreventionSaleHeld != c.held || st.freeFallPreventionActivated != c.activated || st.freeFallPreventionFrom != c.from {
			t.Errorf("%s: active %v, held %v, activated %v, from %v, want %v, %v, %v, %v",
				c.name, st.freeFallPreventionActive, st.freeFallPreventionSaleHeld, st.freeFallPreventionActivated, st.freeFallPreventionFrom, c.active, c.held, c.activated, c.from)
		}
	}

	both := rebuildState(withRows(trade, triggered, FlatMarketTriggeredRow(fill, nil)))
	if !both.freeFallPreventionActive || !both.flatMarketActive {
		t.Errorf("each gate folds its own events, so episodes stored side by side read open in both, got %+v", both)
	}

	shallow := testutil.LadderTrade(false, fills(freefallprevention.ArmDepth-1, "17:38:00")...)
	if st := rebuildState(withRows(shallow, triggered, held, activated)); st.freeFallPreventionWatched || st.freeFallPreventionActive || st.freeFallPreventionFrom != 0 {
		t.Errorf("a ladder short of the arm depth reads no episode, got %+v", st)
	}
	withFreeFallPrevention(t, false)
	if st := rebuildState(withRows(trade, triggered, held, activated)); st.freeFallPreventionWatched || st.freeFallPreventionActive || st.freeFallPreventionSaleHeld || st.freeFallPreventionActivated || st.freeFallPreventionFrom != 0 {
		t.Errorf("a ladder the gate does not watch reads no episode, got %+v", st)
	}
}

// The rows file under the gate in the gate's own words: the triggered row lists the reasons in
// parentheses, owns a copy of them, and the held and activation rows say what the shared action says
// with the gate's label; the reset row names the new depth; the sale files its sold event under the gate.
func TestFreeFallPreventionRowsFileUnderTheirGate(t *testing.T) {
	reasons := []string{"1h in free fall on the last closed bar", "1h in free fall on 205 of 240 1h bars, over 80%"}
	triggered := FreeFallPreventionTriggeredRow(98, reasons)
	want := Row{
		Message: "smartSell: free fall prevention triggered (1h in free fall on the last closed bar, 1h in free fall on 205 of 240 1h bars, over 80%)",
		Price:   98, Gate: GateFreeFallPrevention, Event: EventTriggered, Reasons: reasons,
	}
	if !reflect.DeepEqual(triggered, want) {
		t.Fatalf("triggered row = %+v, want %+v", triggered, want)
	}
	reasons[0] = "changed"
	if triggered.Reasons[0] == "changed" {
		t.Error("the row owns a copy of the reasons")
	}
	bare := FreeFallPreventionTriggeredRow(98, nil)
	if bare.Message != "smartSell: free fall prevention triggered" || len(bare.Reasons) != 0 {
		t.Errorf("a triggered row with no reasons prints no group, got %+v", bare)
	}

	wantReset := Row{Message: "smartSell: free fall prevention reset at the new depth", Price: 96, Gate: GateFreeFallPrevention, Event: EventReset}
	if got := FreeFallPreventionResetRow(96); !reflect.DeepEqual(got, wantReset) {
		t.Errorf("reset row = %+v, want %+v", got, wantReset)
	}
	for _, c := range []struct {
		name   string
		row    Row
		prefix string
		event  string
		within string
	}{
		{"held", FreeFallPreventionSaleHeldRow(98, -0.9), "smartSell: free fall prevention sale held, profit -0.90% of the used amount under ", EventSaleHeld, ""},
		{"held at the depth bound", FreeFallPreventionSaleHeldAtDepthRow(98, 7, 8), "smartSell: free fall prevention sale held, depth 7 of 8 over ", EventSaleHeld, ""},
		{"held by the trend", FreeFallPreventionSaleHeldByTrendRow(98, aggragates.SmartSellIndicators{}, false), "smartSell: free fall prevention sale held, SMC trend not read", EventSaleHeld, ""},
		{"activated", FreeFallPreventionActivatedRow(98, -0.25), "smartSell: free fall prevention take loss activated, profit -0.25% of the used amount at or over ", EventActivated, ""},
	} {
		if !strings.HasPrefix(c.row.Message, c.prefix) || c.row.Gate != GateFreeFallPrevention || c.row.Event != c.event || c.row.Price != 98 || len(c.row.Features) != 0 {
			t.Errorf("%s row = %+v, want %q... under the gate with event %s at 98 and no feature", c.name, c.row, c.prefix, c.event)
		}
	}

	trade := flatMarketLadder(false)
	const level = 98.0
	sale := Row{Message: "smartSell: take loss at free fall prevention 98.00", Price: level, Gate: GateFreeFallPrevention, Event: EventSold}
	if got := ExitRow(trade, reasonFreeFallPrevention, level); !reflect.DeepEqual(got, sale) {
		t.Fatalf("exit row = %+v, want %+v", got, sale)
	}
	if got := ExitMessage(trade, reasonFreeFallPrevention, level); got != sale.Message {
		t.Fatalf("exit message = %q, want %q", got, sale.Message)
	}
	withQuietDecline(t, true)
	if got := ExitRow(pendingTrade(), reasonFreeFallPrevention, level); !reflect.DeepEqual(got, sale) {
		t.Fatalf("a pending ladder's free fall prevention sale names no feature, got %+v", got)
	}
}

// A tick's rows of the gate merge into one log line at its first row, the reset re-arming the
// episode, the held row or the activation joining the opening, as the flat market's do, the line
// keyed by its newest event; the events stay one per row.
func TestFreeFallPreventionRowsMergeIntoOneLogLine(t *testing.T) {
	reasons := []string{"1h in free fall on the last closed bar"}
	held := FreeFallPreventionSaleHeldRow(98, -0.9)
	activated := FreeFallPreventionActivatedRow(98, -0.25)
	cases := []struct {
		name   string
		rows   []Row
		prefix string
		event  string
	}{
		{"opened and held", []Row{FreeFallPreventionTriggeredRow(98, reasons), held}, "smartSell: free fall prevention pending (1h in free fall on the last closed bar), sale held: profit -0.90% of the used amount under ", EventSaleHeld},
		{"opened and activated", []Row{FreeFallPreventionTriggeredRow(98, reasons), activated}, "smartSell: free fall prevention take loss activated (1h in free fall on the last closed bar), profit -0.25% of the used amount at or over ", EventActivated},
		{"re-armed", []Row{FreeFallPreventionResetRow(98), FreeFallPreventionTriggeredRow(98, reasons)}, "smartSell: free fall prevention re-armed at the new depth (1h in free fall on the last closed bar)", EventTriggered},
		{"re-armed and held", []Row{FreeFallPreventionResetRow(98), FreeFallPreventionTriggeredRow(98, reasons), held}, "smartSell: free fall prevention re-armed at the new depth, pending (1h in free fall on the last closed bar), sale held: ", EventSaleHeld},
	}
	for _, c := range cases {
		lines := logLines(c.rows)
		if len(lines) != 1 || !strings.HasPrefix(lines[0].Message, c.prefix) || lines[0].Gate != GateFreeFallPrevention || lines[0].Event != c.event {
			t.Errorf("%s: lines = %+v, want one line under the gate opening %q", c.name, lines, c.prefix)
		}
		_, events := TickRows(flatMarketLadder(false), c.rows, testutil.At("13:00:00"))
		if len(events) != len(c.rows) {
			t.Errorf("%s: %d events for %d rows", c.name, len(events), len(c.rows))
		}
	}
}
