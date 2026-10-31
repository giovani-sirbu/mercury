package smartsell

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/flatmarket"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// flatMarketOrdinaryProposals are the proposals of a tick on which the ladder buys or proposes
// nothing; the action answers them by the state.
var flatMarketOrdinaryProposals = []string{"", "buy", "update_buy"}

// flatMarketHold is the hold the gate answers for the trade under the market: the
// reason it is asked for and the held event naming what the reading names.
func flatMarketHold(trade aggragates.Trades, ai aggragates.AIIndicators, reason string) gates.Hold {
	return gates.Hold{
		Reason: reason,
		Key:    aggragates.LogKeyHoldSmartSellEntry,
		Param:  aggragates.StrategyParamSmartSell,
		Gate:   GateFlatMarket,
		Data:   EventData{Event: gates.EventHeld, Reasons: flatMarketReading(trade, ai.SmartSell, ai).Reasons},
	}
}

// The episode of a long ladder tick by tick (engineStep): the first holding tick opens it with the
// triggered row, over the cap the held row goes out once, the first tick within the cap activates
// with the activation row alone, and from trailingBuy the action trails (hold, re-anchor, sell); only
// the sale closes it.
func TestFlatMarketEpisodeOfALongLadder(t *testing.T) {
	flatMarketAlone(t)
	start := flatMarketLadder(false)
	ai, fill := flatMarketMarket(start), start.PositionPrice
	atTheCap, oneStepLower := flatMarketPrices(t, start)
	held := FlatMarketSaleHeldRow(fill, -handLossPct(start, oneStepLower))
	activation := FlatMarketActivatedRow(fill, -handLossPct(start, atTheCap))

	trade, got := engineStep(start, "", oneStepLower, testutil.At("13:00:00"), ai)
	assertWaits(t, got, "", triggeredBy(start, ai), held)
	if st := rebuildState(trade); !st.flatMarketWatched || !st.flatMarketActive || !st.flatMarketSaleHeld || st.flatMarketActivated || st.flatMarketFrom != fill {
		t.Fatalf("the first tick opens the episode and the cap holds the action, got %+v", st)
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
	if st := rebuildState(trade); !st.flatMarketActive || !st.flatMarketActivated || !st.flatMarketSaleHeld || st.flatMarketFrom != fill {
		t.Fatalf("the episode is open with its trailingBuy activated, got %+v", st)
	}

	if trade.PositionPrice != atTheCap {
		t.Fatalf("the activation tick anchors the ladder at its print, got %v want %v", trade.PositionPrice, atTheCap)
	}
	gain, falls := ladder.AverageEntryPrice(start)*gainFactor, priceAtMove(trade, atTheCap, -(trailRow(trade).Tolerance+1))
	gone := withBlock(flatMarketBlockOf(basePercentage(start), aggragates.SideLong, flatMarketBars, flatMarketHoldMove))
	for _, reading := range []aggragates.AIIndicators{ai, gone} {
		for _, position := range addSideProposals {
			assertTrailHeld(t, Apply(trade, position, atTheCap, reading))
			assertTrailHeld(t, Apply(trade, position, oneStepLower, reading))
			assertReanchored(t, Apply(trade, position, gain, reading))
			assertSold(t, Apply(trade, position, falls, reading), reasonFlatMarket)
		}
	}

	moved, got := engineStep(trade, "buy", gain, testutil.At("13:35:00"), gone)
	assertReanchored(t, got)
	if st := rebuildState(moved); moved.PositionPrice != gain || moved.PositionType != "trailingBuy" || !st.flatMarketActive || !st.flatMarketActivated || st.flatMarketFrom != fill {
		t.Fatalf("a re-anchor moves the anchor to the print and the episode stands in trailingBuy, got %v %q %+v", moved.PositionPrice, moved.PositionType, st)
	}

	sold := Apply(trade, "buy", falls, gone)
	closed := withRow(trade, ExitRow(trade, sold.Reason, falls), testutil.At("13:45:00"))
	if st := rebuildState(closed); st.flatMarketActive || st.flatMarketSaleHeld || st.flatMarketActivated || st.flatMarketFrom != 0 {
		t.Fatalf("the sold row closes the episode, got %+v", st)
	}
	if want := []string{eventAt(EventTriggered, fill), eventAt(EventSaleHeld, fill), eventAt(EventActivated, fill), eventAt(EventSold, falls)}; !reflect.DeepEqual(episodeEvents(t, closed), want) {
		t.Errorf("events = %q, want %q", episodeEvents(t, closed), want)
	}
}

// An unactivated episode stands whatever its reading does: the reading is read only while no
// episode is open, so a reading that goes writes no row, the trigger keeps standing and the action
// acts by the state as before; the reading coming and going opens and closes nothing.
func TestAnUnactivatedFlatMarketEpisodeStandsWhileItsReadingComesAndGoes(t *testing.T) {
	flatMarketAlone(t)
	start := flatMarketLadder(false)
	ai, fill := flatMarketMarket(start), start.PositionPrice
	atTheCap, oneStepLower := flatMarketPrices(t, start)
	held := FlatMarketSaleHeldRow(fill, -handLossPct(start, oneStepLower))
	activation := FlatMarketActivatedRow(fill, -handLossPct(start, atTheCap))
	gone := withBlock(flatMarketBlockOf(basePercentage(start), aggragates.SideLong, flatMarketBars, flatMarketHoldMove))
	readings := flatMarketMisses(basePercentage(start), aggragates.SideLong)
	readings["gone"] = gone

	activates := func(t *testing.T, got Result, _ string) { assertActivated(t, got, activation) }
	sells := func(t *testing.T, got Result, _ string) { assertSold(t, got, reasonFlatMarket) }
	heldOver := func(t *testing.T, got Result, position string) { assertWaits(t, got, position, held) }
	leaves := func(t *testing.T, got Result, position string) { assertUntouched(t, got, position) }

	open := withRows(start, triggeredBy(start, ai))
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
					if st := rebuildState(withRows(c.episode, got.Rows...)); !st.flatMarketActive || st.flatMarketFrom != fill {
						t.Fatalf("%s, reading %q: the episode stands, got %+v", c.name, name, st)
					}
				}
			}
		}
	}

	trade, got := engineStep(start, "", oneStepLower, testutil.At("13:00:00"), ai)
	assertWaits(t, got, "", triggeredBy(start, ai), held)
	for _, reading := range []aggragates.AIIndicators{gone, ai, gone, ai} {
		trade, got = engineStep(trade, "", oneStepLower, testutil.At("13:15:00"), reading)
		assertUntouched(t, got, "")
		if st := rebuildState(trade); !st.flatMarketActive || !st.flatMarketSaleHeld || st.flatMarketActivated || st.flatMarketFrom != fill {
			t.Fatalf("the episode stands as it was, got %+v", st)
		}
	}
	trade, got = engineStep(trade, "", atTheCap, testutil.At("13:30:00"), gone)
	assertActivated(t, got, activation)
	if want := []string{eventAt(EventTriggered, fill), eventAt(EventSaleHeld, fill), eventAt(EventActivated, fill)}; !reflect.DeepEqual(episodeEvents(t, trade), want) {
		t.Errorf("events = %q, want %q", episodeEvents(t, trade), want)
	}
}

// The reading hovering at its bound, tick by tick through flatMarketTick: the first holding tick
// opens the episode, every miss after it writes no row and leaves the trigger standing, and a new
// depth closes it with the reset row (plus a new triggered row on the same tick when the reading holds).
func TestAFlatMarketEpisodeStandsThroughItsMissesAndAResetReopensItOnTheSameTick(t *testing.T) {
	flatMarketAlone(t)
	start := flatMarketLadder(false)
	holding, fill := flatMarketMarket(start), start.PositionPrice
	const newest = 96.0
	misses := flatMarketMisses(basePercentage(start), aggragates.SideLong)
	over, hasOver := misses["a move over"]
	stray, hasStray := misses["closes stray"]
	if !hasOver || !hasStray {
		t.Skip("flatmarket.MaxMovePct leaves no fall over the bound and no stray close to read")
	}
	tick := func(trade aggragates.Trades, reading aggragates.AIIndicators) (aggragates.Trades, state, []Row, bool) {
		next, rows, holds := flatMarketTick(trade, rebuildState(trade), reading.SmartSell, reading)
		return withRows(trade, rows...), next, rows, holds
	}
	assertStands := func(name string, next state, rows []Row, holds bool, from float64) {
		t.Helper()
		if len(rows) != 0 || !holds || !next.flatMarketActive || next.flatMarketFrom != from {
			t.Fatalf("%s: the episode stands with no row and its trigger, got rows %+v, holds %v, state %+v", name, rows, holds, next)
		}
	}

	trade, next, rows, holds := tick(start, holding)
	if want := []Row{triggeredBy(start, holding)}; !reflect.DeepEqual(rows, want) || !holds || !next.flatMarketActive || next.flatMarketFrom != fill || next.flatMarketSaleHeld || next.flatMarketActivated {
		t.Fatalf("the first tick opens the episode, got rows %+v, holds %v, state %+v", rows, holds, next)
	}
	for name, reading := range misses {
		trade, next, rows, holds = tick(trade, reading)
		assertStands(name, next, rows, holds, fill)
		trade, next, rows, holds = tick(trade, holding)
		assertStands("the reading back after "+name, next, rows, holds, fill)
	}
	if want := []string{eventAt(EventTriggered, fill)}; !reflect.DeepEqual(episodeEvents(t, trade), want) {
		t.Fatalf("events = %q, want %q", episodeEvents(t, trade), want)
	}

	deeper := withFill(trade, newest, testutil.At("14:00:00"))
	for name, reading := range misses {
		_, next, rows, holds = tick(deeper, reading)
		if want := []Row{FlatMarketResetRow(newest)}; !reflect.DeepEqual(rows, want) || holds || next.flatMarketActive || next.flatMarketFrom != 0 {
			t.Fatalf("%s: a new depth resets the episode with the reset row alone, got rows %+v, holds %v, state %+v", name, rows, holds, next)
		}
	}
	closed, _, _, _ := tick(deeper, over)
	if _, next, rows, holds = tick(closed, stray); len(rows) != 0 || holds || next.flatMarketActive {
		t.Fatalf("after the reset the reading is read afresh and misses: no row, no trigger, got rows %+v, holds %v, state %+v", rows, holds, next)
	}

	reopened, next, rows, holds := tick(deeper, holding)
	if want := []Row{FlatMarketResetRow(newest), triggeredBy(deeper, holding)}; !reflect.DeepEqual(rows, want) || !holds || !next.flatMarketActive || next.flatMarketFrom != newest || next.flatMarketSaleHeld || next.flatMarketActivated {
		t.Fatalf("a reading that holds reopens the episode on the tick of the reset, got rows %+v, holds %v, state %+v", rows, holds, next)
	}
	if want := []string{eventAt(EventTriggered, fill), eventAt(EventReset, newest), eventAt(EventTriggered, newest)}; !reflect.DeepEqual(episodeEvents(t, reopened), want) {
		t.Errorf("events = %q, want %q", episodeEvents(t, reopened), want)
	}
	_, next, rows, holds = tick(reopened, over)
	assertStands("the new episode", next, rows, holds, newest)
}

// An activated episode stands whatever its reading does: the action trails from the activation
// print alone (hold, re-anchor, sell at any loss) with no row of the episode, and only a sale or a
// new depth closes it.
func TestAnActivatedFlatMarketEpisodeSurvivesItsReadingEnding(t *testing.T) {
	flatMarketAlone(t)
	start := flatMarketLadder(false)
	ai, fill := flatMarketMarket(start), start.PositionPrice
	atTheCap, oneStepLower := flatMarketPrices(t, start)
	gone := withBlock(flatMarketBlockOf(basePercentage(start), aggragates.SideLong, flatMarketBars, flatMarketHoldMove))
	activated := committedBy(withRows(start, triggeredBy(start, ai)), FlatMarketActivatedRow(fill, -0.25))
	activated.PositionPrice = atTheCap
	gain, falls := ladder.AverageEntryPrice(start)*gainFactor, priceAtMove(activated, atTheCap, -(trailRow(activated).Tolerance+1))

	for _, state := range []string{"trailingBuy", "forceTrailingBuy"} {
		resting := restingIn(activated, state)
		for _, position := range addSideProposals {
			for _, reading := range []aggragates.AIIndicators{gone, withBlock(withEveryRowTrend(aggragates.SmartSellIndicators{})), ai} {
				assertTrailHeld(t, Apply(resting, position, oneStepLower, reading))
				assertReanchored(t, Apply(resting, position, gain, reading))
				assertSold(t, Apply(resting, position, falls, reading), reasonFlatMarket)
			}
		}
	}
	if st := rebuildState(activated); !st.flatMarketActive || !st.flatMarketActivated {
		t.Fatalf("the fixture must carry the activation, got %+v", st)
	}
	for _, position := range slices.Concat(closingProposals, takeProfitProposals) {
		assertUntouched(t, Apply(activated, position, oneStepLower, gone), position)
	}
}

// The action reads the ladder's state and never its proposal: buy is activated within the cap on
// any proposal with the triggered row beside the activation, trailingBuy (force-trailing too) is sold
// with the triggered row alone, a close comes back as it came, a take profit in buy gives way to
// the activation within the cap and stands beside the held row over it, and any other state only
// opens the episode.
func TestFlatMarketActivatesInBuyAndSellsInStopLossWhateverTheLadderProposes(t *testing.T) {
	flatMarketAlone(t)
	trade := flatMarketLadder(false)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	atTheCap, oneStepLower := flatMarketPrices(t, trade)
	loss := handLossPct(trade, atTheCap)

	for _, position := range addSideProposals {
		assertActivated(t, Apply(trade, position, atTheCap, ai), triggeredBy(trade, ai), FlatMarketActivatedRow(fill, -loss))
		for _, state := range []string{"trailingBuy", "forceTrailingBuy"} {
			assertSoldWith(t, Apply(restingIn(trade, state), position, atTheCap, ai), reasonFlatMarket, triggeredBy(trade, ai))
		}
		for _, state := range []string{"new", ""} {
			assertWaits(t, Apply(restingIn(trade, state), position, atTheCap, ai), position, triggeredBy(trade, ai))
		}
	}
	for _, position := range closingProposals {
		assertUntouched(t, Apply(trade, position, atTheCap, ai), position)
	}
	for _, position := range takeProfitProposals {
		assertActivated(t, Apply(trade, position, atTheCap, ai), triggeredBy(trade, ai), FlatMarketActivatedRow(fill, -loss))
		held := FlatMarketSaleHeldRow(fill, -handLossPct(trade, oneStepLower))
		assertWaits(t, Apply(trade, position, oneStepLower, ai), position, triggeredBy(trade, ai), held)
	}
	for _, resting := range []string{"takeProfit", "forceTrailingTakeProfit", "takeLoss", "sell", "sellParent", "impasse"} {
		for _, position := range append([]string{""}, addSideProposals...) {
			assertUntouched(t, Apply(restingIn(trade, resting), position, atTheCap, ai), position)
		}
	}
}

// A gain or the break even reaches a trigger at zero: buy has its episode opened with the triggered
// row and its trailingBuy activated on the same tick, and an episode already open is activated alike.
func TestFlatMarketActivatesAtAGainOrTheBreakEvenOnTheTickItOpens(t *testing.T) {
	flatMarketAlone(t)
	withAllowedLoss(t, 0)
	trade := flatMarketLadder(false)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	for _, price := range []float64{ladder.AverageEntryPrice(trade) * gainFactor, ladder.AverageEntryPrice(trade)} {
		activation := FlatMarketActivatedRow(fill, 0-handLossPct(trade, price))
		for _, position := range addSideProposals {
			got := Apply(trade, position, price, ai)
			assertActivated(t, got, triggeredBy(trade, ai), activation)
			opened := withRow(trade, got.Rows[0], testutil.At("13:00:00"))
			assertActivated(t, Apply(opened, position, price, ai), activation)
		}
	}
}

// Over the cap the action waits and the gate holds nothing else: the held row goes out once beside
// the triggered row, every proposal comes back as it came (the arming included), a self-armed
// trailingBuy is held and sold back within the cap, buy is activated back within it with no second held row.
func TestFlatMarketWaitsOverTheCapAndHoldsNothingElse(t *testing.T) {
	flatMarketAlone(t)
	trade := flatMarketLadder(false)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	atTheCap, oneStepLower := flatMarketPrices(t, trade)
	held := FlatMarketSaleHeldRow(fill, -handLossPct(trade, oneStepLower))

	for _, state := range []string{"buy", "trailingBuy"} {
		resting := restingIn(trade, state)
		for _, position := range addSideProposals {
			got := Apply(resting, position, oneStepLower, ai)
			assertWaits(t, got, position, triggeredBy(trade, ai), held)

			written := withRow(withRow(resting, got.Rows[0], testutil.At("13:00:00")), got.Rows[1], testutil.At("13:00:00"))
			for _, again := range addSideProposals {
				assertUntouched(t, Apply(written, again, oneStepLower, ai), again)
				assertUntouched(t, Apply(written, again, oneStepLower-1, ai), again)
			}
			if state == "buy" {
				assertActivated(t, Apply(written, position, atTheCap, ai), FlatMarketActivatedRow(fill, -handLossPct(trade, atTheCap)))
			} else {
				assertSold(t, Apply(written, position, atTheCap, ai), reasonFlatMarket)
			}
		}
	}
}

// The depth bound is read before the cap: past it the action is held at whatever loss with the depth
// row once per episode, in buy and in trailingBuy; at the deepest depth it lets through it acts, one
// deeper it holds, and an unknown ceiling holds with its row.
func TestFlatMarketWaitsPastTheDepthBoundWhateverTheLoss(t *testing.T) {
	flatMarketAlone(t)
	deepest := lastDepthFills - SaleIgnoreLastDepths
	if SaleIgnoreLastDepths <= 0 || deepest < 1 || deepest >= lastDepthFills {
		t.Skip("SaleIgnoreLastDepths is off or leaves no depth the action is made at and held past")
	}
	withSaleIgnoreLastDepths(t, SaleIgnoreLastDepths)

	within, past := depthLadder(deepest), depthLadder(deepest+1)
	ai := flatMarketMarket(past)
	gain := ladder.AverageEntryPrice(within) * gainFactor
	cheap := aLossWithinTheCap(t, within)
	for _, position := range addSideProposals {
		assertActivated(t, Apply(within, position, cheap, ai), triggeredBy(within, ai), FlatMarketActivatedRow(within.PositionPrice, -lossPctOf(within, cheap)))
		assertSoldWith(t, Apply(restingIn(within, "trailingBuy"), position, cheap, ai), reasonFlatMarket, triggeredBy(within, ai))
	}

	heldRow := FlatMarketSaleHeldAtDepthRow(past.PositionPrice, deepest+1, lastDepthFills)
	for _, state := range []string{"buy", "trailingBuy"} {
		resting := restingIn(past, state)
		for _, price := range []float64{gain, ladder.AverageEntryPrice(past), armingPrice(past), ladder.AverageEntryPrice(past) / 2} {
			for _, position := range addSideProposals {
				assertWaits(t, Apply(resting, position, price, ai), position, triggeredBy(past, ai), heldRow)
			}
		}

		held, got := engineStep(resting, "trailingBuy", gain, testutil.At("13:00:00"), ai)
		assertRows(t, got, triggeredBy(past, ai), heldRow)
		if st := rebuildState(held); !st.flatMarketActive || !st.flatMarketSaleHeld {
			t.Fatalf("the episode is open with its action held, got %+v", st)
		}
		for _, position := range addSideProposals {
			assertUntouched(t, Apply(held, position, gain, ai), position)
		}
	}

	unknown := withDepths(depthLadder(deepest), 0)
	got := Apply(unknown, "trailingBuy", cheap, ai)
	assertWaits(t, got, "trailingBuy", triggeredBy(unknown, ai), FlatMarketSaleHeldAtDepthRow(unknown.PositionPrice, deepest, 0))
}

// With the depth bound off the gate acts at any depth, the last one included.
func TestFlatMarketActsAtAnyDepthWithTheDepthBoundOff(t *testing.T) {
	flatMarketAlone(t)
	withSaleIgnoreLastDepths(t, 0)
	trade := lastDepthLadder()
	ai := flatMarketMarket(trade)
	price := aLossWithinTheCap(t, trade)
	for _, position := range addSideProposals {
		assertActivated(t, Apply(trade, position, price, ai), triggeredBy(trade, ai), FlatMarketActivatedRow(trade.PositionPrice, -lossPctOf(trade, price)))
		assertSoldWith(t, Apply(restingIn(trade, "trailingBuy"), position, price, ai), reasonFlatMarket, triggeredBy(trade, ai))
	}
}

// A new depth resets the episode on that tick with the reset row at the new fill, whatever it held;
// the reading is read afresh and a holding one opens a new episode with its own triggered row on the
// same tick (held row due again, activation within the cap); a miss leaves the reset row alone.
func TestFlatMarketResetsAtANewDepthAndOpensAnotherEpisodeOnTheSameTick(t *testing.T) {
	flatMarketAlone(t)
	start := flatMarketLadder(false)
	ai, fill := flatMarketMarket(start), start.PositionPrice
	_, oneStepLower := flatMarketPrices(t, start)
	const newest = 96.0
	gone := withBlock(flatMarketBlockOf(basePercentage(start), aggragates.SideLong, flatMarketBars, flatMarketHoldMove))

	open := withRows(start, triggeredBy(start, ai))
	waiting := withRows(open, FlatMarketSaleHeldRow(fill, -handLossPct(start, oneStepLower)))
	activated := committedBy(open, FlatMarketActivatedRow(fill, -0.25))
	deeper := func(trade aggragates.Trades) aggragates.Trades {
		return withFill(trade, newest, testutil.At("14:00:00"))
	}
	far := ladder.AverageEntryPrice(deeper(start)) * lossFactor
	cheap := aLossWithinTheCap(t, deeper(start))

	for name, episode := range map[string]aggragates.Trades{"open": open, "held": waiting, "activated": activated} {
		onTheNewDepth := deeper(episode)
		reopened := triggeredBy(onTheNewDepth, ai)
		overTheCap := FlatMarketSaleHeldRow(newest, -handLossPct(onTheNewDepth, far))

		got := Apply(onTheNewDepth, "buy", far, ai)
		assertWaits(t, got, "buy", FlatMarketResetRow(newest), reopened, overTheCap)
		written, _ := engineStep(onTheNewDepth, "buy", far, testutil.At("14:05:00"), ai)
		if st := rebuildState(written); !st.flatMarketActive || st.flatMarketActivated || !st.flatMarketSaleHeld || st.flatMarketFrom != newest {
			t.Fatalf("%s: the new episode is open at the new fill with its action held and no trailingBuy activated, got %+v", name, st)
		}
		assertUntouched(t, Apply(written, "buy", far, ai), "buy")

		assertActivated(t, Apply(restingIn(onTheNewDepth, "buy"), "", cheap, ai),
			FlatMarketResetRow(newest), reopened, FlatMarketActivatedRow(newest, -lossPctOf(onTheNewDepth, cheap)))
		assertSoldWith(t, Apply(restingIn(onTheNewDepth, "trailingBuy"), "", cheap, ai), reasonFlatMarket, FlatMarketResetRow(newest), reopened)

		over := Apply(restingIn(onTheNewDepth, "trailingBuy"), "buy", far, ai)
		assertWaits(t, over, "buy", FlatMarketResetRow(newest), reopened, overTheCap)

		bare := Apply(onTheNewDepth, "buy", far, gone)
		assertWaits(t, bare, "buy", FlatMarketResetRow(newest))
		closed := withRow(onTheNewDepth, bare.Rows[0], testutil.At("14:05:00"))
		if st := rebuildState(closed); st.flatMarketActive || st.flatMarketSaleHeld || st.flatMarketActivated || st.flatMarketFrom != 0 {
			t.Fatalf("%s: the reset closes the episode, got %+v", name, st)
		}
		assertUntouched(t, Apply(closed, "buy", far, gone), "buy")
	}
}

// foldRow is one case of the flat market fold: the rows after the fills and the episode they leave.
type foldRow struct {
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
// unusable events leave it as it was.
func TestFlatMarketFoldsItsEpisodeFromItsEvents(t *testing.T) {
	withFlatMarket(t, true)
	trade := flatMarketLadder(false)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	triggered := triggeredBy(trade, ai)
	held := FlatMarketSaleHeldRow(fill, -0.9)
	atDepth := FlatMarketSaleHeldAtDepthRow(fill, 7, 8)
	activated := FlatMarketActivatedRow(fill, -0.25)
	reset := FlatMarketResetRow(fill)
	sold := ExitRow(trade, reasonFlatMarket, fill+1)
	reopened := triggeredBy(withFill(trade, 96, testutil.At("14:00:00")), ai)

	cases := []foldRow{
		{name: "no event"},
		{name: "triggered", rows: []Row{triggered}, active: true, from: fill},
		{name: "triggered, held", rows: []Row{triggered, held}, active: true, held: true, from: fill},
		{name: "triggered, held by the depth bound", rows: []Row{triggered, atDepth}, active: true, held: true, from: fill},
		{name: "triggered, activated", rows: []Row{triggered, activated}, active: true, activated: true, from: fill},
		{name: "triggered, held, activated", rows: []Row{triggered, held, activated}, active: true, held: true, activated: true, from: fill},
		{name: "triggered, activated, held", rows: []Row{triggered, activated, held}, active: true, held: true, activated: true, from: fill},
		{name: "held alone, the triggered row lost", rows: []Row{held}, active: true, held: true, from: fill},
		{name: "activated alone, the triggered row lost", rows: []Row{activated}, active: true, activated: true, from: fill},
		{name: "triggered, then held at another price keeps the price it opened at", rows: []Row{triggered, FlatMarketSaleHeldRow(fill+1, -0.9)}, active: true, held: true, from: fill},
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
			name:   "an event with no price, a kind the gate does not have",
			rows:   []Row{triggered},
			events: []aggragates.TradesStrategyEvents{eventOf(GateFlatMarket, `{"event":"held","reasons":["a"]}`), eventOf(GateFlatMarket, legacyDocument(EventPending, fill)), eventOf(GateFlatMarket, legacyDocument(EventActivated, 0))},
			active: true, from: fill,
		},
	}
	for _, c := range cases {
		withEvent := withEvents(withRows(trade, c.rows...), c.events...)
		st := rebuildState(withEvent)
		if st.flatMarketActive != c.active || st.flatMarketSaleHeld != c.held || st.flatMarketActivated != c.activated || st.flatMarketFrom != c.from {
			t.Errorf("%s: active %v, held %v, activated %v, from %v, want %v, %v, %v, %v",
				c.name, st.flatMarketActive, st.flatMarketSaleHeld, st.flatMarketActivated, st.flatMarketFrom, c.active, c.held, c.activated, c.from)
		}
	}

	withFlatMarket(t, false)
	if st := rebuildState(withRows(trade, triggered, held, activated)); st.flatMarketWatched || st.flatMarketActive || st.flatMarketSaleHeld || st.flatMarketActivated || st.flatMarketFrom != 0 {
		t.Errorf("a ladder the gate does not watch reads no episode, got %+v", st)
	}
}

// ExitReached reads the sale and never the activation: buy is no exit at any print, a self-armed
// trailingBuy is an exit at no loss or within the cap, and an activated one only more than the
// tolerance under its activation print.
func TestExitReachedReadsTheFlatMarketSaleNeverItsActivation(t *testing.T) {
	flatMarketAlone(t)
	trade := flatMarketLadder(false)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	block := ai.SmartSell
	atTheCap, oneStepLower := flatMarketPrices(t, trade)
	gain := ladder.AverageEntryPrice(trade) * gainFactor
	gone := flatMarketBlockOf(basePercentage(trade), aggragates.SideLong, flatMarketBars, flatMarketHoldMove)

	assertActivated(t, Apply(trade, "", atTheCap, ai), triggeredBy(trade, ai), FlatMarketActivatedRow(fill, -handLossPct(trade, atTheCap)))
	for _, price := range []float64{gain, fill, atTheCap, oneStepLower, ladder.AverageEntryPrice(trade) * lossFactor} {
		if ExitReached(trade, price, block) {
			t.Errorf("a ladder in buy has its trailingBuy activated, held or left alone and is sold at no price: ExitReached must read false at %v", price)
		}
	}

	natural := restingIn(withRows(trade, triggeredBy(trade, ai)), "trailingBuy")
	for _, c := range []struct {
		price float64
		exit  bool
	}{{gain, true}, {ladder.AverageEntryPrice(trade), true}, {atTheCap, true}, {oneStepLower, false}, {ladder.AverageEntryPrice(trade) * lossFactor, false}} {
		if got := ExitReached(natural, c.price, block); got != c.exit {
			t.Errorf("a ladder that armed itself in trailingBuy: ExitReached at %v = %v, want %v", c.price, got, c.exit)
		}
	}

	committed := committedBy(withRows(trade, triggeredBy(trade, ai)), FlatMarketActivatedRow(fill, -0.25))
	committed.PositionPrice = atTheCap
	falls := priceAtMove(committed, atTheCap, -(trailRow(committed).Tolerance + 1))
	for _, c := range []struct {
		price float64
		exit  bool
	}{{gain, false}, {atTheCap, false}, {oneStepLower, false}, {falls, true}, {ladder.AverageEntryPrice(trade) * lossFactor, true}} {
		for _, reading := range []aggragates.SmartSellIndicators{block, gone} {
			if got := ExitReached(committed, c.price, reading); got != c.exit {
				t.Errorf("a ladder the gate activated trails from its activation print: ExitReached at %v = %v, want %v", c.price, got, c.exit)
			}
		}
	}
}

// Nothing is read, written or acted on off the watch or without the market: futures, a child, no
// flag, no settings, no fill, a depth priority hold, a missing market and the switch off.
func TestFlatMarketDoesNothingOffItsWatchOrWithoutItsMarket(t *testing.T) {
	flatMarketAlone(t)
	withDepthPriorityPause(t, true)
	base := flatMarketLadder(false)
	atTheCap, _ := flatMarketPrices(t, base)
	off := flatMarketOffTheWatch(base)
	off["depth priority held"] = heldBy(base, testutil.At("13:00:00"))
	for name, trade := range off {
		for _, state := range []string{"buy", "trailingBuy"} {
			assertUntouched(t, Apply(restingIn(trade, state), "trailingBuy", atTheCap, flatMarketMarket(trade)), "trailingBuy")
		}
		if name == "depth priority held" && !rebuildState(trade).depthPriorityHeld {
			t.Errorf("fixture drifted: the %s ladder must read held", name)
		}
	}
	if flatMarketWatched(off["futures"]) || flatMarketWatched(off["no fill"]) || !flatMarketWatched(base) {
		t.Error("the gate watches a spot ladder from its first fill, and no futures ladder or unfilled one")
	}
	for _, miss := range flatMarketMisses(basePercentage(base), aggragates.SideLong) {
		for _, state := range []string{"buy", "trailingBuy"} {
			assertUntouched(t, Apply(restingIn(base, state), "trailingBuy", atTheCap, miss), "trailingBuy")
		}
	}
	withFlatMarket(t, false)
	assertUntouched(t, Apply(base, "trailingBuy", atTheCap, flatMarketMarket(base)), "trailingBuy")
}

// The shipped trigger holds a far loss and acts on a gain; the trigger at zero acts at break even
// and holds every loss, and one at a profit holds the break even until that profit.
func TestFlatMarketTriggerAtZeroOrAProfitWaitsForIt(t *testing.T) {
	flatMarketAlone(t)
	trade := flatMarketLadder(false)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	_, oneStepLower := flatMarketPrices(t, trade)
	average := ladder.AverageEntryPrice(trade)
	gain := average * gainFactor
	assertWaits(t, Apply(trade, "", average/2, ai), "", triggeredBy(trade, ai), FlatMarketSaleHeldRow(fill, -handLossPct(trade, average/2)))
	assertWaits(t, Apply(trade, "", oneStepLower, ai), "", triggeredBy(trade, ai), FlatMarketSaleHeldRow(fill, -handLossPct(trade, oneStepLower)))
	assertActivated(t, Apply(trade, "", gain, ai), triggeredBy(trade, ai), FlatMarketActivatedRow(fill, -handLossPct(trade, gain)))

	withAllowedLoss(t, 0)
	assertWaits(t, Apply(trade, "", oneStepLower, ai), "", triggeredBy(trade, ai), FlatMarketSaleHeldRow(fill, -handLossPct(trade, oneStepLower)))
	assertActivated(t, Apply(trade, "", average, ai), triggeredBy(trade, ai), FlatMarketActivatedRow(fill, 0-handLossPct(trade, average)))
	assertSoldWithAt(t, Apply(restingIn(trade, "trailingBuy"), "", average, ai), reasonFlatMarket, trade, average, triggeredBy(trade, ai))

	withAllowedLoss(t, -1)
	for _, state := range []string{"buy", "trailingBuy"} {
		assertWaits(t, Apply(restingIn(trade, state), "", average, ai), "", triggeredBy(trade, ai), FlatMarketSaleHeldRow(fill, 0-handLossPct(trade, average)))
	}
	assertActivated(t, Apply(trade, "", gain, ai), triggeredBy(trade, ai), FlatMarketActivatedRow(fill, -handLossPct(trade, gain)))
	assertSoldWithAt(t, Apply(restingIn(trade, "trailingBuy"), "", gain, ai), reasonFlatMarket, trade, gain, triggeredBy(trade, ai))
}

// An inverse ladder is read on the mirror: the trend must read up and the move is the rise, the
// profit is counted in the base its entries sold, and a price under its break even is a gain.
func TestFlatMarketInverseLadderReadsTheMirror(t *testing.T) {
	flatMarketAlone(t)
	if allowedLoss() <= 0 || allowedLoss() >= 100 || flatMarketOverMove >= 100 {
		t.Skip("SaleTriggerPercentage or flatmarket.MaxMovePct leave no price or no move to read the mirror at")
	}
	long, inverse := flatMarketLadder(false), flatMarketLadder(true)
	ai, fill := flatMarketMarket(inverse), inverse.PositionPrice
	average := ladder.AverageEntryPrice(inverse)
	within := math.Floor(average/(1-allowedLoss()/100)*100) / 100
	over := within + 0.01

	profitPct, standing := saleProfit(inverse, over, FullTrendTrigger())
	if want := (average/over - 1) * 100; standing != short || math.Abs(profitPct-want) > 1e-9 {
		t.Fatalf("the profit of an inverse sale is counted on the base: got %v (%v), want %v", profitPct, standing, want)
	}
	assertWaits(t, Apply(inverse, "trailingBuy", over, ai), "trailingBuy", triggeredBy(inverse, ai), FlatMarketSaleHeldRow(fill, profitPct))
	assertActivated(t, Apply(inverse, "trailingBuy", within, ai), triggeredBy(inverse, ai), FlatMarketActivatedRow(fill, -inverseLossPctOf(inverse, within)))
	gain := math.Floor(average/gainFactor*100) / 100
	assertActivated(t, Apply(inverse, "", gain, ai), triggeredBy(inverse, ai), FlatMarketActivatedRow(fill, -inverseLossPctOf(inverse, gain)))
	assertSoldWith(t, Apply(restingIn(inverse, "trailingBuy"), "", within, ai), reasonFlatMarket, triggeredBy(inverse, ai))

	falling := flatMarketLineBlockOf(basePercentage(inverse), aggragates.SideLong, -4)
	falling.SmcTrendUp = flatmarket.MinSmcTrend
	if got := Apply(inverse, "trailingBuy", within, withBlock(falling)); got.Activated || len(got.Rows) != 0 {
		t.Fatalf("a line falling against an inverse ladder holds for it on neither reading, got %+v", got)
	}
	if got := Apply(long, "trailingBuy", within, withBlock(falling)); !got.Activated {
		t.Fatalf("the same line falls the long ladder's way and holds for it, got %+v", got)
	}
	if flatmarket.MinSmcTrend > 0 {
		assertUntouched(t, Apply(inverse, "trailingBuy", within, flatMarketMarket(long)), "trailingBuy")
		assertUntouched(t, Apply(long, "trailingBuy", within, ai), "trailingBuy")
	}
}

// The percentage the bodies are read against is the base row of the settings, not a later row.
func TestFlatMarketReadsTheBaseRowOfTheSettings(t *testing.T) {
	flatMarketAlone(t)
	trade := flatMarketLadder(false)
	ai := flatMarketMarket(trade)
	base := trade.StrategyPair.StrategySettings[0]
	other := base
	other.Percentage = base.Percentage / 10
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{base, other}
	if basePercentage(trade) != base.Percentage || !flatMarketReading(trade, ai.SmartSell, ai).Holds {
		t.Fatalf("the base row must be the one read, got a percentage of %v", basePercentage(trade))
	}
}

// A ladder that opened raised is read against the base percentage it trades, not the stored one, by
// the flat market and the flat market gates alike; the stored rows are never written.
func TestTheGatesReadTheBasePercentageTheLadderTrades(t *testing.T) {
	const points = 0.7
	plain := flatMarketLadder(false)
	plain.Strategy.Params.DynamicParams, plain.Strategy.TradeType = true, aggragates.Spot
	stored := plain.StrategyPair.StrategySettings[0].Percentage
	row, event := dynamicparams.Opened{Points: points, Depths: raisedDepths}.Rows(plain, plain.PositionPrice, testutil.At("08:00:00"))
	raised := aggragates.AppendStrategyRow(plain, row, event)
	if traded, isRaised := dynamicparams.RaisedSettings(raised); !isRaised || math.Abs(traded[0].Percentage-(stored+points)) > 1e-9 {
		t.Fatalf("fixture drifted: the ladder must trade a base row raised by %v, got %+v (raised %v)", points, traded, isRaised)
	}

	for name, trade := range map[string]aggragates.Trades{"unraised": plain, "raised": raised} {
		want := stored
		if name == "raised" {
			want += points
		}
		if got := basePercentage(trade); math.Abs(got-want) > 1e-9 {
			t.Errorf("%s: basePercentage = %v, want %v", name, got, want)
		}
		got := flatMarketReading(trade, aggragates.SmartSellIndicators{}, aggragates.AIIndicators{}).Line.Bound
		if math.Abs(got-flatmarket.MaxDeviation*want) > 1e-9 {
			t.Errorf("%s: flat market bound = %v, want %v", name, got, flatmarket.MaxDeviation*want)
		}
	}
	if kept := raised.StrategyPair.StrategySettings[0].Percentage; kept != stored {
		t.Errorf("the trade keeps its own rows, got a stored percentage of %v, want %v", kept, stored)
	}
}

// The sale files its sold event under the flat market gate in a byte-stable text and names no decline feature.
func TestFlatMarketExitRowFilesUnderItsGateAndNamesNoFeature(t *testing.T) {
	trade := flatMarketLadder(false)
	const level = 98.0
	want := Row{Message: "smartSell: take loss at flat market 98.00", Price: level, Gate: GateFlatMarket, Event: EventSold}
	if got := ExitRow(trade, reasonFlatMarket, level); !reflect.DeepEqual(got, want) {
		t.Fatalf("exit row = %+v, want %+v", got, want)
	}
	if got := ExitMessage(trade, reasonFlatMarket, level); got != want.Message {
		t.Fatalf("exit message = %q, want %q", got, want.Message)
	}

	withQuietDecline(t, true)
	pending := pendingTrade()
	if got := ExitRow(pending, reasonFlatMarket, level); !reflect.DeepEqual(got, want) {
		t.Fatalf("a pending ladder's flat market sale names no feature, got %+v", got)
	}
	if got := ExitRow(pending, reasonSlowDecline, level); got.Gate != GateSlowDecline || len(got.Features) == 0 {
		t.Fatalf("control: the shared sale of the slow decline gate names the pending features, got %+v", got)
	}
}

// The first fill is held while the reading stands, long and inverse (aggragates.SideShort), the held
// event carrying the reasons, after the quiet decline's hold; it is the gate's only hold.
func TestFlatMarketEntryHoldHoldsAFirstFillWhileItsReadingStands(t *testing.T) {
	flatMarketAlone(t)
	long, inverse := testutil.LadderTrade(false), testutil.LadderTrade(true)
	ai, mirror := flatMarketMarket(long), flatMarketMarket(inverse)
	if got, want := EntryHold(long, aggragates.SideLong, ai), flatMarketHold(long, ai, FlatMarketEntryHoldReason); !got.Held() || !reflect.DeepEqual(got, want) {
		t.Fatalf("a long first fill under the market must be held, got %+v want %+v", got, want)
	}
	if got, want := EntryHold(inverse, aggragates.SideShort, mirror), flatMarketHold(inverse, mirror, FlatMarketEntryHoldReason); !got.Held() || !reflect.DeepEqual(got, want) {
		t.Fatalf("an inverse first fill must be held on the mirror, got %+v want %+v", got, want)
	}
	if FlatMarketEntryHoldReason != "smartSell: flat market, first fill held" {
		t.Fatalf("the reason is byte-stable text, got %q", FlatMarketEntryHoldReason)
	}

	both := ai
	both.SmartSell.QuietDeclineVerdict = true
	withQuietDecline(t, true)
	if got := EntryHold(long, aggragates.SideLong, both); got.Reason != SlowDeclineEntryHoldReason || got.Gate != GateEntryHold {
		t.Errorf("the quiet decline holds first, got %+v", got)
	}
	if got := EntryHold(long, aggragates.SideLong, ai); got.Reason != FlatMarketEntryHoldReason {
		t.Errorf("without its verdict the flat market hold speaks, got %+v", got)
	}
	withQuietDecline(t, false)
	if got := EntryHold(long, aggragates.SideLong, both); got.Reason != FlatMarketEntryHoldReason {
		t.Errorf("with the quiet decline off the flat market hold speaks, got %+v", got)
	}
}

// The first fill goes through when the gate has nothing to say: an unread side, futures, a child,
// no flag, a missing market, the hold's switch off; the gate's own switch off leaves the hold reading.
func TestFlatMarketEntryHoldLetsAFirstFillThroughWithoutItsMarket(t *testing.T) {
	flatMarketAlone(t)
	trade := testutil.LadderTrade(false)
	ai := flatMarketMarket(trade)
	off := flatMarketOffTheWatch(trade)
	delete(off, "no fill")
	for name, variant := range off {
		if got := EntryHold(variant, aggragates.SideLong, flatMarketMarket(variant)); got.Held() {
			t.Errorf("%s: must go through, got %+v", name, got)
		}
	}
	for _, side := range []string{"", "flat"} {
		if got := EntryHold(trade, side, ai); got.Held() {
			t.Errorf("side %q is not read, got %+v", side, got)
		}
	}
	for name, miss := range flatMarketMisses(basePercentage(trade), aggragates.SideLong) {
		if got := EntryHold(trade, aggragates.SideLong, miss); got.Held() {
			t.Errorf("%s: the market misses, got %+v", name, got)
		}
	}
	withFlatMarket(t, false)
	if got := EntryHold(trade, aggragates.SideLong, ai); got.Reason != FlatMarketEntryHoldReason {
		t.Errorf("the gate off, the hold still reads, got %+v", got)
	}
	withFlatMarketHoldInitialDepth(t, false)
	if got := EntryHold(trade, aggragates.SideLong, ai); got.Held() {
		t.Errorf("the hold switched off, got %+v", got)
	}
}
