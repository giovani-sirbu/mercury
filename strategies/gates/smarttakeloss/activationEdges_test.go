package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The edges of the activation of the stopLoss, end to end through Apply with the trade
// rebuilt from the rows and events it carries: the ACTIVATE step of the shared action
// (sellLoss) waits while the ladder's own order rests, the memory of a flat market
// activation binds while the ladder rests in stopLoss alone, and the quiet decline
// watches long spot ladders alone. Each test states the rows by hand, whole, from the
// builders the gates share.

// restingOrderID is the id of the buy order the ladders of these tests have resting.
const restingOrderID = 77

// withRestingOrder is the trade with its own order resting (trade.PendingOrder): the
// buy of a depth the chain placed and the exchange has not filled yet.
func withRestingOrder(trade aggragates.Trades) aggragates.Trades {
	trade.PendingOrder = restingOrderID
	return trade
}

// The ACTIVATE step waits in silence while the ladder's own order rests: the activation
// cancels nothing, and an order that filled before the sale would put a new depth under
// a stopLoss activated for a smaller position. A ladder in buy whose trigger stands is
// left alone, with no activation and no held row, whatever it proposes, the loss, the
// cap and the depth bound, and it is asked again on every tick, so that it goes ahead on
// the first tick the order is gone: the activation row alone, the gate's own row having
// gone out before, and the held row a bound refuses, which the wait left due since it
// wrote none. The gate's own rows are not the action's and still go out: the flat market
// gate's triggered row opens its episode on the tick the order rests, and nothing
// follows it.
func TestTheActivationWaitsInSilenceWhileAnOrderRestsOnTheFlatMarketGate(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, 0)
	trade := flatMarketLadder(false)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	atTheCap, oneStepLower := flatMarketPrices(t, trade)
	far := ladder.AverageEntryPrice(trade) * lossFactor
	activation := FlatMarketStopLossActivatedRow(fill, -handLossPct(trade, atTheCap))
	held := FlatMarketSaleHeldRow(fill, -handLossPct(trade, oneStepLower))
	resting := withRestingOrder(trade)

	assertActivated(t, Apply(trade, "", atTheCap, ai), triggeredBy(trade, ai), activation)
	for _, position := range addSideProposals {
		assertWaits(t, Apply(resting, position, atTheCap, ai), position, triggeredBy(trade, ai))
	}

	opened, got := engineStep(resting, "", atTheCap, testutil.At("13:00:00"), ai)
	assertWaits(t, got, "", triggeredBy(trade, ai))
	if st := rebuildState(opened); !st.flatMarketActive || st.flatMarketActivated || st.flatMarketSaleHeld || st.flatMarketFrom != fill {
		t.Fatalf("the episode is open at the newest fill with nothing activated or held, got %+v", st)
	}
	for _, price := range []float64{atTheCap, oneStepLower, far} {
		for _, position := range addSideProposals {
			assertUntouched(t, Apply(opened, position, price, ai), position)
		}
	}
	again, got := engineStep(opened, "", oneStepLower, testutil.At("13:05:00"), ai)
	assertUntouched(t, got, "")
	if len(again.Logs) != len(opened.Logs) || len(again.StrategyEvents) != len(opened.StrategyEvents) {
		t.Fatalf("a tick with the order still resting writes nothing, got %+v", again.Logs)
	}

	gone := opened
	gone.PendingOrder = 0
	for _, position := range addSideProposals {
		assertActivated(t, Apply(gone, position, atTheCap, ai), activation)
		assertWaits(t, Apply(gone, position, oneStepLower, ai), position, held)
	}
	stopped, got := engineStep(gone, "", atTheCap, testutil.At("13:10:00"), ai)
	assertActivated(t, got, activation)
	if st := rebuildState(stopped); !st.flatMarketActive || !st.flatMarketActivated || stopped.PositionType != "stopLoss" {
		t.Fatalf("the first tick without the order activates the stopLoss, got %+v in %q", st, stopped.PositionType)
	}

	// A bound that refuses writes its held row only when no order rests: the wait is silent,
	// so the row is still due once the order is gone.
	filled, ceiling := ladder.CountFilledEntries(trade), ladder.ConfiguredDepths(trade)
	withSellLossIgnoreLastDepths(t, ceiling-filled+1)
	atDepth := FlatMarketSaleHeldAtDepthRow(fill, filled, ceiling)
	for _, position := range addSideProposals {
		assertUntouched(t, Apply(opened, position, atTheCap, ai), position)
		assertWaits(t, Apply(gone, position, atTheCap, ai), position, atDepth)
	}
}

// The flat market gate's own rows are not the shared action's and go out while the order
// rests: the reset row at a new depth, alone when the reading misses on that tick, with the
// triggered row of the episode a reading that still holds opens on the same tick, which the
// activation then waits behind. An episode already open writes nothing while the order
// rests, whatever the reading does: it stands.
func TestTheFlatMarketGatesOwnRowsStillGoOutWhileAnOrderRests(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, 0)
	trade := flatMarketLadder(false)
	ai := flatMarketMarket(trade)
	atTheCap, _ := flatMarketPrices(t, trade)
	gone := withBlock(withEveryRowTrend(aggragates.SmartTakeLossIndicators{}))
	opened := withRestingOrder(withRows(trade, triggeredBy(trade, ai)))

	const newest = 96.0
	deeper := withFill(opened, newest, testutil.At("14:00:00"))
	for _, position := range addSideProposals {
		assertUntouched(t, Apply(opened, position, atTheCap, gone), position)
		assertUntouched(t, Apply(opened, position, atTheCap, ai), position)
		assertWaits(t, Apply(deeper, position, atTheCap, ai), position, FlatMarketResetRow(newest), triggeredBy(deeper, ai))
		assertWaits(t, Apply(deeper, position, atTheCap, gone), position, FlatMarketResetRow(newest))
	}
}

// The slow decline gate's activation waits the same way: a pending ladder in buy whose
// newest fill is judged is left alone while its order rests, within the cap, over it,
// far over it and at a gain, whatever it proposes and whatever is served, and goes ahead
// on the first tick the order is gone, its held row still due over the cap. The tick the
// verdict arrives on writes the gate's own row and no activation; the activation follows
// the order's going.
func TestTheActivationWaitsInSilenceWhileAnOrderRestsOnTheSlowDeclineGate(t *testing.T) {
	underTheShippedCap(t)
	trade := pendingTrade()
	armed, loss := armingOverTheCap(t, trade)
	within, _ := firstPriceWithinTheCap(t, trade, armed)
	activation := quietActivation(trade, within)
	held := SaleHeldRow(slowDeclineLastFill, -loss, FeatureQuietDecline)
	resting := withRestingOrder(trade)

	for _, reading := range []aggragates.AIIndicators{nothingServed(), slowDeclineBlock(true), slowDeclineBlock(false)} {
		for _, position := range addSideProposals {
			for _, price := range []float64{within, armed, armed / 2, gainPrint(trade)} {
				assertUntouched(t, Apply(resting, position, price, reading), position)
			}
			assertActivated(t, Apply(trade, position, within, reading), activation)
			assertWaits(t, Apply(trade, position, armed, reading), position, held)
		}
	}

	pending := quietPending(slowDeclineLastFill)
	watched := withRestingOrder(watchedTrade())
	for _, position := range addSideProposals {
		assertWaits(t, Apply(watched, position, within, slowDeclineBlock(true)), position, pending)
	}
	marked, got := engineStep(watched, "", within, testutil.At("18:00:00"), slowDeclineBlock(true))
	assertWaits(t, got, "", pending)
	assertUntouched(t, Apply(marked, "", within, slowDeclineBlock(true)), "")
	marked.PendingOrder = 0
	assertActivated(t, Apply(marked, "", within, slowDeclineBlock(true)), activation)
}

// Directly on the shared action: a ladder in buy with its order resting answers nothing
// for every trigger, the gates together included, whatever the loss, a gain, the cap and
// the depth bound, with no held row either; the same ladder without the order is
// activated within the cap and held over it and past the bound, and one in stopLoss with
// the order resting is sold or held as it is without it: the sale is not delayed, and a
// ladder a gate activated trails (holds, re-anchors, sells) as it does without the order.
func TestSellLossWaitsInSilenceInBuyWhileAnOrderRestsAndSellsInStopLoss(t *testing.T) {
	sellLossOn(t)
	farOverTheCap(t, shippedLoss)
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	p := roundLadderPrices(t, trade)
	resting := withRestingOrder(trade)

	var cases []sellLossCase
	for _, stand := range eachStanding() {
		name := func(what string) string { return stand.name + ", " + what }
		cases = append(cases,
			onLadder(name("no order, within the cap"), trade, "buy", p.within, stand.triggers...).activating(fill, lossPctOf(trade, p.within), shippedLoss),
			onLadder(name("no order, over the cap"), trade, "buy", p.far, stand.triggers...).holdingByLoss(fill, lossPctOf(trade, p.far), shippedLoss),
			onLadder(name("an order rests, within the cap"), resting, "buy", p.within, stand.triggers...),
			onLadder(name("an order rests, exactly at the cap"), resting, "buy", p.atTheCap, stand.triggers...),
			onLadder(name("an order rests, one price step over the cap"), resting, "buy", p.over, stand.triggers...),
			onLadder(name("an order rests, far over the cap"), resting, "buy", p.far, stand.triggers...),
			onLadder(name("an order rests, a gain"), resting, "buy", p.gain, stand.triggers...),
			onLadder(name("an order rests, stopLoss, a gain"), resting, "stopLoss", p.gain, stand.triggers...).selling(),
			onLadder(name("an order rests, stopLoss, within the cap"), resting, "stopLoss", p.within, stand.triggers...).selling(),
			onLadder(name("an order rests, stopLoss, far over the cap"), resting, "stopLoss", p.far, stand.triggers...).holdingByLoss(fill, lossPctOf(trade, p.far), shippedLoss),
			onLadder(name("an order rests, force-trailing stopLoss, within the cap"), resting, "forceTrailingStopLoss", p.within, stand.triggers...).selling(),
		)
	}
	for _, trigger := range eachGate() {
		committed := activatedOf(trigger)
		cases = append(cases, trailCases(trigger.gate+" activated, an order rests, stopLoss, far over the cap", resting, "stopLoss", p.far, committed)...)
		cases = append(cases, trailCases(trigger.gate+" activated, an order rests, stopLoss, a gain", resting, "stopLoss", p.gain, committed)...)
		cases = append(cases, onLadder(trigger.gate+" activated, an order rests, buy, far over the cap", resting, "buy", p.far, committed))
	}
	if SellLossIgnoreLastDepths > 0 {
		past := depthLadder(lastDepthFills)
		price := ladder.AverageEntryPrice(past) * lossFactor
		for _, trigger := range eachGate() {
			cases = append(cases,
				onLadder(trigger.gate+", no order, past the depth bound", past, "buy", price, trigger).
					holdingAtItsFill(atDepthText(lastDepthFills, lastDepthFills, lastDepthFills-SellLossIgnoreLastDepths)),
				onLadder(trigger.gate+", an order rests, past the depth bound", withRestingOrder(past), "buy", price, trigger),
			)
		}
	}
	runSellLoss(t, cases)
}

// The SELL step is not delayed by an order resting: the sellLoss chain cancels the order
// first. A ladder in stopLoss, or in the force-trailing state that reads as stopLoss, is
// sold with the order resting exactly as it is without: one a gate activated when its
// anchor sits over the print by more than the tolerance, at any print, a gain and a loss
// far over the cap included, with no row and whatever the market reads by then (the
// same ladder is held between its bounds and re-anchored over the trail, order or no
// order); one that armed itself there on a bounce and within the cap, and held over it
// with the held row once. ExitReached reads the sale with the order resting, true for the
// activated ladder under the tolerance of its anchor at every print, false while it is
// held or re-anchored, and false for the ladder in buy.
func TestTheSaleIsNotDelayedByARestingOrder(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, 0)
	trade := flatMarketLadder(false)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	gone := withBlock(flatMarketBlockOf(basePercentage(trade), aggragates.SideLong, flatMarketBars, flatMarketHoldMove))
	atTheCap, oneStepLower := flatMarketPrices(t, trade)
	gain, far := ladder.AverageEntryPrice(trade)*gainFactor, ladder.AverageEntryPrice(trade)*lossFactor

	first := Apply(trade, "", atTheCap, ai)
	assertActivated(t, first, triggeredBy(trade, ai), FlatMarketStopLossActivatedRow(fill, -handLossPct(trade, atTheCap)))
	committed := withRestingOrder(committedBy(trade, first.Rows...))
	natural := withRestingOrder(restingIn(withRows(trade, first.Rows[0]), "stopLoss"))
	held := FlatMarketSaleHeldRow(fill, -handLossPct(trade, oneStepLower))

	for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
		for _, position := range addSideProposals {
			for _, price := range []float64{gain, atTheCap, oneStepLower, far} {
				for _, reading := range []aggragates.AIIndicators{ai, gone} {
					assertSold(t, Apply(restingIn(anchoredToSell(committed, price), state), position, price, reading), reasonFlatMarket)
					assertReanchored(t, Apply(restingIn(anchoredToReanchor(committed, price), state), position, price, reading))
					assertTrailHeld(t, Apply(restingIn(anchoredToHold(committed, price), state), position, price, reading))
				}
			}
			for _, price := range []float64{gain, atTheCap} {
				assertSold(t, Apply(restingIn(natural, state), position, price, ai), reasonFlatMarket)
			}
			assertWaits(t, Apply(restingIn(natural, state), position, oneStepLower, ai), position, held)
		}
	}

	block := ai.SmartTakeLoss
	for _, price := range []float64{gain, atTheCap, oneStepLower, far} {
		if !ExitReached(anchoredToSell(committed, price), price, block) {
			t.Errorf("a ladder a gate activated is sold at any print under the tolerance of its anchor with its order resting: ExitReached must read true at %v", price)
		}
		if ExitReached(anchoredToHold(committed, price), price, block) || ExitReached(anchoredToReanchor(committed, price), price, block) {
			t.Errorf("a ladder a gate activated is no exit while it is held or re-anchored, with its order resting: ExitReached must read false at %v", price)
		}
		if ExitReached(withRestingOrder(trade), price, block) {
			t.Errorf("a ladder in buy is no exit at any print: ExitReached must read false at %v", price)
		}
	}
}

// Capital protection waits in silence with an order resting in buy, as the shared action does
// for every trigger: the episode's triggered row goes out, no activation and no held row, within
// the drawdown and over it alike. A ladder resting in stopLoss is sold on its trigger whether an
// order rests or not, and held with the held row over the drawdown.
func TestCapitalProtectionWaitsWithAnOrderRestingInBuy(t *testing.T) {
	capitalProtectionAlone(t)
	trade := lastDepthLadder()
	within, under := firstPriceWithinTheDrawdown(t, trade)
	ai, fill := withBlock(solBlock()), trade.PositionPrice
	triggered := capitalProtectionTriggered(trade)

	assertActivated(t, Apply(trade, "", within, ai), triggered, CapitalProtectionStopLossActivatedRow(fill, -handLossPct(trade, within)))
	for _, price := range []float64{within, under} {
		for _, position := range addSideProposals {
			assertWaits(t, Apply(withRestingOrder(trade), position, price, ai), position, triggered)
		}
	}
	for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
		resting := withRestingOrder(restingIn(trade, state))
		assertSoldWith(t, Apply(resting, "", within, ai), reasonCapitalProtection, triggered)
		assertWaits(t, Apply(resting, "", under, ai), "", triggered, CapitalProtectionSaleHeldRow(fill, -handLossPct(trade, under)))
	}
}

// The memory of a flat market activation binds while the ladder rests in stopLoss
// alone. An activation whose chain never ran leaves the activated event in the trade
// (the rows go out with the tick) and the ladder in buy; there the episode is read as
// one not activated: the activation is retried, with its row alone and, over the cap, the
// held row due, and the shared sale sees no committed trigger, whatever the market reads,
// gone or holding, since the episode stands and its reading is not read. The same events
// with the ladder resting in stopLoss, or in the force-trailing state that reads as
// stopLoss, are the committed sale: the ladder trails from its anchor at any print,
// whatever the market reads (sold under the tolerance, re-anchored over the trailing take
// profit, held in between).
func TestAStaleFlatMarketActivationBindsOnlyWhileTheLadderRestsInStopLoss(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, 0)
	trade := flatMarketLadder(false)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	gone := withBlock(withEveryRowTrend(aggragates.SmartTakeLossIndicators{}))
	atTheCap, oneStepLower := flatMarketPrices(t, trade)
	gain, far := ladder.AverageEntryPrice(trade)*gainFactor, ladder.AverageEntryPrice(trade)*lossFactor
	activation := FlatMarketStopLossActivatedRow(fill, -handLossPct(trade, atTheCap))

	first := Apply(trade, "", atTheCap, ai)
	assertActivated(t, first, triggeredBy(trade, ai), activation)
	stale := withRows(trade, first.Rows...)
	ran := committedBy(trade, first.Rows...)
	if st := rebuildState(stale); !st.flatMarketActive || !st.flatMarketActivated || stale.PositionType != "buy" {
		t.Fatalf("fixture drifted: the events say the stopLoss was activated and the ladder still rests in buy, got %+v in %q", st, stale.PositionType)
	}

	// Buy, the market gone or holding: the episode stands, read as one not activated, so the
	// activation is retried with its row alone, at a gain too, and over the cap the shared sale
	// sees no committed trigger and holds, with the held row still due.
	// The market gone changes nothing, and nothing closes the episode.
	for _, reading := range []aggragates.AIIndicators{gone, ai} {
		for _, position := range addSideProposals {
			assertActivated(t, Apply(stale, position, atTheCap, reading), activation)
			assertWaits(t, Apply(stale, position, oneStepLower, reading), position, FlatMarketSaleHeldRow(fill, -handLossPct(trade, oneStepLower)))
			assertWaits(t, Apply(stale, position, far, reading), position, FlatMarketSaleHeldRow(fill, -handLossPct(trade, far)))
			assertActivated(t, Apply(stale, position, gain, reading), FlatMarketStopLossActivatedRow(fill, -handLossPct(trade, gain)))
		}
	}

	// Resting in stopLoss, the market gone: the memory binds, the sale is committed and the
	// ladder trails from its anchor whatever the loss, and nothing ends.
	for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
		resting := restingIn(ran, state)
		for _, position := range addSideProposals {
			for _, price := range []float64{atTheCap, oneStepLower, far, gain} {
				assertSold(t, Apply(anchoredToSell(resting, price), position, price, gone), reasonFlatMarket)
				assertReanchored(t, Apply(anchoredToReanchor(resting, price), position, price, gone))
				assertTrailHeld(t, Apply(anchoredToHold(resting, price), position, price, gone))
			}
		}
	}

	// The unit itself: the state it hands back drops the memory outside stopLoss alone.
	block := ai.SmartTakeLoss
	next, rows, holds := flatMarketTick(stale, rebuildState(stale), block, ai)
	if !next.flatMarketActive || next.flatMarketActivated || !holds || rows != nil {
		t.Fatalf("buy, the market holds: the episode stands unactivated, got %+v rows %v holds %v", next, rows, holds)
	}
	next, rows, holds = flatMarketTick(ran, rebuildState(ran), gone.SmartTakeLoss, gone)
	if !next.flatMarketActive || !next.flatMarketActivated || !holds || rows != nil {
		t.Fatalf("stopLoss, the market gone: the episode stands activated and unread, got %+v rows %v holds %v", next, rows, holds)
	}
	next, rows, holds = flatMarketTick(stale, rebuildState(stale), gone.SmartTakeLoss, gone)
	if !next.flatMarketActive || next.flatMarketActivated || !holds || rows != nil {
		t.Fatalf("buy, the market gone: the episode stands unactivated and unread, got %+v rows %v holds %v", next, rows, holds)
	}
}

// The slow decline gate needs no such care: its activated memory is read in the state
// stopLoss alone by the shared action itself. An activation whose chain never ran leaves
// the ladder in buy, where the action is a candidate to ACTIVATE again, never to sell: it
// retries the activation within the cap, with its row, and holds over it, with the held
// row due; the same events with the ladder resting in stopLoss commit the sale, which
// trails from the ladder's anchor at any print (sold, re-anchored or held).
func TestAStaleSlowDeclineActivationIsRetriedInBuyAndCommitsTheSaleInStopLoss(t *testing.T) {
	underTheShippedCap(t)
	trade := pendingTrade()
	armed, loss := armingOverTheCap(t, trade)
	within, _ := firstPriceWithinTheCap(t, trade, armed)
	activation := quietActivation(trade, within)
	stale := withRows(trade, activation)
	ran := committedBy(trade, activation)
	if st := rebuildState(stale); !st.slowDeclinePending || !st.slowDeclineActivated || stale.PositionType != "buy" {
		t.Fatalf("fixture drifted: the events say the stopLoss was activated and the ladder still rests in buy, got %+v in %q", st, stale.PositionType)
	}

	for _, position := range addSideProposals {
		assertActivated(t, Apply(stale, position, within, nothingServed()), activation)
		assertWaits(t, Apply(stale, position, armed, nothingServed()), position, SaleHeldRow(slowDeclineLastFill, -loss, FeatureQuietDecline))
		assertUntouched(t, applyAtAGain(stale, position, nothingServed()), position)
		for _, price := range []float64{gainPrint(ran), within, armed, armed / 2} {
			assertSold(t, Apply(anchoredToSell(ran, price), position, price, nothingServed()), reasonSlowDecline)
			assertReanchored(t, Apply(anchoredToReanchor(ran, price), position, price, nothingServed()))
			assertTrailHeld(t, Apply(anchoredToHold(ran, price), position, price, nothingServed()))
		}
	}
}

// The quiet decline watches long spot ladders alone, as the other gates do. A futures
// ladder is not watched by it, nor eligible for it, nor armed for it, whatever its depth;
// neither is an inverse ladder of either market; a long spot ladder short of
// quietdecline.ArmDepth is eligible and not watched yet. A ladder carrying the
// pending pair is acted on only where it is watched: the stopLoss activated in buy, the
// ladder sold in stopLoss, ExitReached true there; the others come back untouched with no
// row, their take profit reading the move it is handed.
func TestTheQuietDeclineWatchesLongSpotLaddersAlone(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, false)
	withEarlyTakeProfit(t, false)
	withCapitalProtectionExit(t, false)
	withFlatMarket(t, false)
	withFlatMarket(t, false)

	// Two entries at least, so that the newest fill is not the average entry price and a
	// take profit measured from the one is told from a take profit measured from the other.
	armDepth := quietdecline.ArmDepth
	depth := max(armDepth, 2)
	spot := testutil.LadderTrade(false, fills(depth, "17:38:00")...)
	futures := spot
	futures.Strategy.TradeType = aggragates.Futures
	inverse := testutil.LadderTrade(true, fills(depth, "17:38:00")...)
	inverseFutures := inverse
	inverseFutures.Strategy.TradeType = aggragates.Futures
	short := testutil.LadderTrade(false, fills(armDepth-1, "17:38:00")...)

	type watch struct {
		name              string
		trade             aggragates.Trades
		eligible, watched bool
	}
	ticked := []watch{
		{"a long spot ladder at or past the arm depth", spot, true, true},
		{"a futures ladder", futures, false, false},
		{"an inverse spot ladder", inverse, false, false},
		{"an inverse futures ladder", inverseFutures, false, false},
	}
	for _, tc := range append(ticked, watch{"a long spot ladder one entry short of the arm depth", short, true, false}) {
		st := rebuildState(tc.trade)
		if quietDeclineEligible(tc.trade) != tc.eligible || quietDeclineWatched(tc.trade) != tc.watched || slowDeclineWatched(tc.trade) != tc.watched ||
			st.quietDeclineWatched != tc.watched || st.slowDeclineWatched != tc.watched || Armed(tc.trade) != tc.watched {
			t.Errorf("%s: eligible %v, watched %v, state %+v, armed %v, want eligible %v, watched %v", tc.name, quietDeclineEligible(tc.trade), quietDeclineWatched(tc.trade), st, Armed(tc.trade), tc.eligible, tc.watched)
		}
	}

	for _, tc := range ticked {
		newest := tc.trade.PositionPrice
		pending := withRows(tc.trade, quietPending(newest))
		loss, gain := armingPrice(spot), gainPrint(pending)
		block := nothingServed().SmartTakeLoss
		if tc.watched {
			assertActivated(t, Apply(pending, "stopLoss", loss, nothingServed()), quietActivation(pending, loss))
			assertSold(t, Apply(restingIn(pending, "stopLoss"), "stopLoss", loss, nothingServed()), reasonSlowDecline)
		} else {
			for _, state := range []string{"buy", "stopLoss"} {
				for _, position := range append([]string{""}, addSideProposals...) {
					assertUntouched(t, Apply(restingIn(pending, state), position, loss, nothingServed()), position)
				}
			}
		}
		if got := ExitReached(restingIn(pending, "stopLoss"), loss, block); got != tc.watched {
			t.Errorf("%s: ExitReached in stopLoss = %v, want %v", tc.name, got, tc.watched)
		}
		profit := moveAgainst(gain, ladder.AverageEntryPrice(pending))
		want := profit
		if tc.watched {
			want = moveAgainst(gain, newest)
		}
		if got := TakeProfitPercentage(pending, gain, profit); got != want {
			t.Errorf("%s: the take profit reads %v, want %v", tc.name, got, want)
		}
	}
}
