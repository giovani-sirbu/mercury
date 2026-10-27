package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The rows of one tick come back in one order, the order the engines write them in: the
// slow decline gate's row first — pending, confirmed or cancelled, or the latch of the
// early take profit —, then the flat market gate's episode rows — reset, triggered —,
// then the rows of the shared action: the activation rows or the held rows,
// the slow decline gate's before the flat market gate's when episodes stored side by side
// both stand, a gate opening no episode while another's is open. The sold row an engine builds
// comes after all of them. The order is what the log reads and what the exit row's
// features are read off (they are the pending event of the same tick), so each tick below
// carries a row of every family it can and the answer is stated by hand, whole. The cases
// run on the switches a tuning that leaves the declines off ships (the indecision
// direction and flat market) and with the quiet decline on, each set by the test and
// never read off the tree.

// orderLadder is the round ladder on a row whose ceiling leaves the depth bound room, so
// that the cap alone holds or lets the action go ahead.
func orderLadder() aggragates.Trades {
	trade := flatMarketLadder(false)
	trade.StrategyPair.StrategySettings[0].Depths = float64(len(roundPrices) + SellLossIgnoreLastDepths + 4)
	return trade
}

func TestApplyWritesTheGateRowThenTheEpisodeRowsThenTheActionRows(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	atTheCap, oneCentOver := sellLossPrices(t, orderLadder())
	capPct, fill := float64(shippedLoss), orderLadder().PositionPrice
	over := lossPctOf(orderLadder(), oneCentOver)
	slowHeld := heldByLossRowOf(saleTrigger{gate: GateSlowDecline, features: []string{FeatureQuietDecline}}, fill, over, capPct)
	lowHeld := heldByLossRowOf(saleTrigger{gate: GateFlatMarket}, fill, over, capPct)
	loss := lossPctOf(orderLadder(), atTheCap)
	slowActivated, lowActivated := StopLossActivatedRow(fill, -loss, FeatureQuietDecline), FlatMarketStopLossActivatedRow(fill, -loss)
	pending := quietPending(fill)
	latched := LatchedRow(fill, indecisionFinding(indecisionReasons))

	// The two readings a tick can serve: the sophos indecision and the market, and the
	// verdict of the quiet decline with the market, each a block with the market's bars.
	trade := orderLadder()
	latching := withBlock(flatMarketOver(indecisionReading().SmartTakeLoss, trade))
	verdict := withBlock(flatMarketOver(slowDeclineBlock(true).SmartTakeLoss, trade))
	noMarket := slowDeclineBlock(true)
	open := withRows(trade, triggeredBy(trade, verdict))
	pendingOpen := withRows(open, pending)

	t.Run("the indecision latch, the episode and the held row", func(t *testing.T) {
		withQuietDecline(t, false)
		withEarlyTakeProfit(t, true)
		for _, position := range []string{"stopLoss", "buy", ""} {
			assertWaits(t, Apply(trade, position, oneCentOver, latching), position, latched, triggeredBy(trade, latching), lowHeld)
		}
		assertActivated(t, Apply(trade, "stopLoss", atTheCap, latching), latched, triggeredBy(trade, latching), lowActivated)
		assertSoldWith(t, Apply(restingIn(trade, "stopLoss"), "stopLoss", atTheCap, latching), reasonFlatMarket, latched, triggeredBy(trade, latching))
	})

	t.Run("a pending row and its held row, the flat market gate opening nothing beside it", func(t *testing.T) {
		withQuietDecline(t, true)
		assertWaits(t, Apply(trade, "stopLoss", oneCentOver, verdict), "stopLoss", pending, slowHeld)
		assertActivated(t, Apply(trade, "update_stopLoss", atTheCap, verdict), pending, slowActivated)
		assertSoldWith(t, Apply(restingIn(trade, "stopLoss"), "update_stopLoss", atTheCap, verdict), reasonSlowDecline, pending)
	})

	t.Run("the episode standing with no market keeps the slow decline from going pending", func(t *testing.T) {
		withQuietDecline(t, true)
		assertWaits(t, Apply(open, "stopLoss", oneCentOver, noMarket), "stopLoss", lowHeld)
		assertActivated(t, Apply(open, "stopLoss", atTheCap, noMarket), lowActivated)
		assertSoldWith(t, Apply(restingIn(open, "stopLoss"), "stopLoss", atTheCap, noMarket), reasonFlatMarket)
	})

	t.Run("the episode standing with no market and the held rows of a ladder already pending", func(t *testing.T) {
		withQuietDecline(t, true)
		assertWaits(t, Apply(pendingOpen, "stopLoss", oneCentOver, noMarket), "stopLoss", slowHeld, lowHeld)
		assertActivated(t, Apply(pendingOpen, "forceTrailingStopLoss", atTheCap, noMarket), slowActivated, lowActivated)
		assertSoldWith(t, Apply(restingIn(pendingOpen, "stopLoss"), "forceTrailingStopLoss", atTheCap, noMarket), reasonSlowDecline)
	})

	t.Run("the judgement of a new fill and the reset of a stored episode, then the action's rows", func(t *testing.T) {
		withQuietDecline(t, true)
		const newest = 96.0
		deeper := withFill(pendingOpen, newest, testutil.At("14:00:00"))
		reading := withBlock(flatMarketOver(legOnAndQuiet().SmartTakeLoss, deeper))
		confirmed := ConfirmedRow(newest, quietFinding(slowDeclineReasons))
		reset := FlatMarketResetRow(newest)
		far := deeper.PositionPrice * lossFactor
		cheap := aLossWithinTheCap(t, deeper)
		heldAt := func(trigger saleTrigger, price float64) Row {
			return heldByLossRowOf(trigger, newest, lossPctOf(deeper, price), capPct)
		}

		assertWaits(t, Apply(deeper, "buy", far, reading), "buy",
			confirmed, reset,
			heldAt(saleTrigger{gate: GateSlowDecline, features: []string{FeatureQuietDecline}}, far),
		)
		assertActivated(t, Apply(deeper, "buy", cheap, reading),
			confirmed, reset,
			StopLossActivatedRow(newest, -lossPctOf(deeper, cheap), FeatureQuietDecline),
		)
	})
}

// The life of the flat market gate with the indecision latch, tick by tick, as the engines
// write it: each tick's rows are written back, the state the chain leaves with them, and the
// next tick's state is rebuilt from them. The first tick latches the ladder, opens the
// episode and holds the action over the cap, all in that order; the tick after, a re-arming
// lower and a tick of the dead zone write nothing; the market stopping reading writes
// nothing, the episode standing, and no latch again; a market that reads again writes
// nothing either, since the episode is the same one and its held row is not due again; a
// tick within the cap activates the stopLoss with the activation row alone; and from there
// the ladder TRAILS from the print it was activated at, whatever the market reads: the
// tick after holds it, a rise over the trailing take profit re-anchors it, a print that the
// old anchor would have re-anchored holds from the new one, and a fall under the tolerance
// of the new anchor sells it.
func TestTheRowsOfTheShippedSwitchesLifeAreWrittenOnceInTheirOrder(t *testing.T) {
	flatMarketAlone(t)
	withEarlyTakeProfit(t, true)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	atTheCap, oneCentOver := sellLossPrices(t, orderLadder())
	trade := orderLadder()
	fill, capPct := trade.PositionPrice, float64(shippedLoss)
	lowHeld := heldByLossRowOf(saleTrigger{gate: GateFlatMarket}, fill, lossPctOf(trade, oneCentOver), capPct)
	reading := withBlock(flatMarketOver(indecisionReading().SmartTakeLoss, trade))
	gone := indecisionReading()
	latched := LatchedRow(fill, indecisionFinding(indecisionReasons))
	triggered := triggeredBy(trade, reading)
	activation := FlatMarketStopLossActivatedRow(fill, -lossPctOf(trade, atTheCap))

	// The trail is measured from the print the activation was made at (atTheCap), by the
	// row of the held depth: a rise of one and a half trailing take profits re-anchors, a
	// fall of half a tolerance from the new anchor holds although the old anchor would
	// re-anchor on it, and a fall of one and a half tolerances sells.
	row := trailRow(trade)
	rise := priceAtMove(trade, atTheCap, 1.5*row.TrailingTakeProfit)
	back := priceAtMove(trade, rise, -0.5*row.Tolerance)
	fall := priceAtMove(trade, rise, -1.5*row.Tolerance)
	if moveAgainst(back, atTheCap) <= row.TrailingTakeProfit {
		t.Fatalf("fixture drifted: the old anchor must re-anchor on a print of %v", back)
	}

	life := []struct {
		name      string
		position  string
		price     float64
		ai        aggragates.AIIndicators
		rows      []Row
		reason    string
		activated bool
		held      bool
	}{
		{"the first tick over the cap", "stopLoss", oneCentOver, reading, []Row{latched, triggered, lowHeld}, "", false, false},
		{"the same print again", "stopLoss", oneCentOver, reading, nil, "", false, false},
		{"a re-arming lower", "update_stopLoss", oneCentOver - 0.5, reading, nil, "", false, false},
		{"a tick of the dead zone", "", oneCentOver, reading, nil, "", false, false},
		{"the market stops reading, the episode stands", "stopLoss", oneCentOver, gone, nil, "", false, false},
		{"the market reads again, the same episode", "stopLoss", oneCentOver, reading, nil, "", false, false},
		{"a later tick within the cap", "update_stopLoss", atTheCap, reading, []Row{activation}, "", true, false},
		{"the tick after, at the same print, the market gone", "buy", atTheCap, gone, nil, "", false, true},
		{"a print inside the bounds, the market gone", "update_buy", oneCentOver, gone, nil, "", false, true},
		{"a rise over the trailing take profit, the market gone", "", rise, gone, nil, "", true, false},
		{"a print the old anchor would re-anchor on", "buy", back, gone, nil, "", false, true},
		{"a fall under the tolerance of the new anchor", "update_stopLoss", fall, gone, nil, reasonFlatMarket, false, false},
	}
	for _, step := range life {
		var got Result
		trade, got = engineStep(trade, step.position, step.price, testutil.At("13:00:00"), step.ai)
		wantPosition := step.position
		switch {
		case step.reason != "":
			wantPosition = "sellLoss"
		case step.activated:
			wantPosition = "stopLoss"
		case step.held:
			wantPosition = ""
		}
		if got.Position != wantPosition || got.Reason != step.reason || got.Activated != step.activated {
			t.Fatalf("%s: answered %q (reason %q, activated %v), want %q (reason %q, activated %v)", step.name, got.Position, got.Reason, got.Activated, wantPosition, step.reason, step.activated)
		}
		assertRows(t, got, step.rows...)
	}
}
