package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The order Apply reads its triggers in: one decision of the shared action for every gate
// whose trigger stands on the tick, named by the first of them — the slow decline gate's,
// then the flat market gate's, then capital protection's, each judged on its own terms.
// The shared action is asked on every tick a trigger stands and reads the ladder's state,
// never its proposal, so every proposal of the add side reaches it, the dead zone's
// included. A print that two of them reach is sold for the first of the two, whatever the
// second would say. The tests below serve both readings on one print and check that each
// trigger alone reaches it, so that moving a trigger above or below its neighbour changes
// the answer and the order is what they pin.

// flatMarketOver is the block of another sale's readings with the market that holds
// for the trade's ladder added to it: the bars and the trend flatMarketBlock serves,
// and nothing else of it.
func flatMarketOver(block aggragates.SmartTakeLossIndicators, trade aggragates.Trades) aggragates.SmartTakeLossIndicators {
	market := flatMarketBlock(basePercentage(trade), aggragates.EntrySide(trade, aggragates.AIIndicators{}))
	block.FlatMarketBarOpens = market.FlatMarketBarOpens
	block.FlatMarketOpenPrices = market.FlatMarketOpenPrices
	block.FlatMarketHighs = market.FlatMarketHighs
	block.FlatMarketLows = market.FlatMarketLows
	block.FlatMarketCloses = market.FlatMarketCloses
	block.SmcTrendDown = market.SmcTrendDown
	block.SmcTrendUp = market.SmcTrendUp
	block.SmcTrendValid = market.SmcTrendValid
	return block
}

// The first trigger names the shared action. Episodes stored side by side (before the gates
// went one at a time) both stand: a pending ladder with its newest fill judged and an open
// flat market episode are sold for the slow decline gate from stopLoss, which stands first,
// and from buy the activation writes one row for each gate, the slow decline gate's first.
// On the pending ladder alone the flat market gate opens nothing beside it. Each gate alone reaches the print under its own reason.
func TestTheFirstTriggerNamesTheSharedAction(t *testing.T) {
	flatMarketAlone(t)
	withQuietDecline(t, true)
	withAllowedLoss(t, anyLoss)
	trade, price := exactLadder(), roundPrices[2]
	newest, loss := trade.PositionPrice, handLossPct(trade, price)
	ai := withBlock(flatMarketOver(aggragates.SmartTakeLossIndicators{}, trade))
	if !flatMarketReading(trade, ai.SmartTakeLoss, ai).Holds {
		t.Fatal("fixture drifted: the reading must hold")
	}

	stopLoss := restingIn(trade, "stopLoss")
	withFlatMarket(t, false)
	if got := Apply(stopLoss, "", price, ai); got.Reason != reasonSlowDecline {
		t.Fatalf("fixture drifted: the slow decline gate alone must reach the print, got reason %q", got.Reason)
	}
	withFlatMarket(t, true)
	withQuietDecline(t, false)
	if got := Apply(stopLoss, "", price, ai); got.Reason != reasonFlatMarket {
		t.Fatalf("fixture drifted: the flat market gate alone must reach the print, got reason %q", got.Reason)
	}
	withQuietDecline(t, true)

	both := withRows(trade, triggeredBy(trade, ai))
	for _, position := range addSideProposals {
		assertSoldWith(t, Apply(restingIn(both, "stopLoss"), position, price, ai), reasonSlowDecline)
		assertActivated(t, Apply(both, position, price, ai),
			StopLossActivatedRow(newest, -loss, FeatureQuietDecline),
			FlatMarketStopLossActivatedRow(newest, -loss),
		)
		assertActivated(t, Apply(trade, position, price, ai), StopLossActivatedRow(newest, -loss, FeatureQuietDecline))
	}
}

// Both triggers standing on a tick the shared action refuses, their episodes stored side by
// side, is one decision: nothing is activated or sold, and each gate whose row is due writes
// its own held row, the slow decline gate's before the flat market gate's, in trigger order. The row is written once per episode and gate: the next tick writes
// nothing, and one flag per gate covers both bounds, so a refusal for the other bound
// writes no second row either. The cap holds the action at one print, the depth bound at a
// ladder one depth past it, in the state buy and in the state stopLoss alike.
func TestBothTriggersHoldTheSharedActionAsOneDecisionWithARowForEachInTriggerOrder(t *testing.T) {
	flatMarketAlone(t)
	withQuietDecline(t, true)
	trade := exactLadder()
	newest := trade.PositionPrice
	_, oneStepLower := exactBoundary(t)
	ai := withBlock(flatMarketOver(aggragates.SmartTakeLossIndicators{}, trade))
	loss := handLossPct(trade, oneStepLower)

	both := withRows(trade, triggeredBy(trade, ai))
	for _, state := range []string{"buy", "stopLoss"} {
		held, got := engineTick(restingIn(both, state), "stopLoss", oneStepLower, testutil.At("13:00:00"), ai)
		assertWaits(t, got, "stopLoss", SaleHeldRow(newest, -loss, FeatureQuietDecline), FlatMarketSaleHeldRow(newest, -loss))
		if st := rebuildState(held); !st.saleHeld || !st.flatMarketActive || !st.flatMarketSaleHeld {
			t.Fatalf("%s: each gate remembers that its action was held, got %+v", state, st)
		}
		for _, position := range addSideProposals {
			assertUntouched(t, Apply(held, position, oneStepLower, ai), position)
		}

		withSellLossIgnoreLastDepths(t, lastDepthFills-len(roundPrices)+1)
		for _, position := range addSideProposals {
			assertUntouched(t, Apply(held, position, oneStepLower, ai), position)
		}
		withSellLossIgnoreLastDepths(t, 0)
	}

	deepest := lastDepthFills - SellLossIgnoreLastDepths
	if SellLossIgnoreLastDepths <= 0 || deepest+1 < quietdecline.ArmDepth {
		t.Skip("SellLossIgnoreLastDepths is off or leaves no depth past it that the quiet decline watches")
	}
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	past := depthLadder(deepest + 1)
	market := withBlock(flatMarketOver(aggragates.SmartTakeLossIndicators{}, past))
	past = withRows(past, quietPending(past.PositionPrice), triggeredBy(past, market))
	for _, state := range []string{"buy", "stopLoss"} {
		got := Apply(restingIn(past, state), "stopLoss", past.PositionPrice, market)
		assertWaits(t, got, "stopLoss",
			SaleHeldAtDepthRow(past.PositionPrice, deepest+1, lastDepthFills, FeatureQuietDecline),
			FlatMarketSaleHeldAtDepthRow(past.PositionPrice, deepest+1, lastDepthFills),
		)
	}
}

// The flat market gate and capital protection on one tick at the last depth: the depth bound
// holds the flat market trigger (its held-at-depth row once per episode) while capital
// protection's is judged on its own terms beside it — held over the drawdown with its own row,
// activating buy within it, selling a self-armed stopLoss within it under its own reason; the
// episode rows come first, the flat market gate's before capital protection's, then the refusal,
// then capital protection's own row. With the bound off both triggers are live: the flat market
// gate, first in trigger order, names a sale both reach and its activation row comes first.
func TestFlatMarketIsHeldByTheDepthBoundWhileCapitalProtectionActs(t *testing.T) {
	flatMarketAlone(t)
	withCapitalProtectionExit(t, true)
	withAllowedLoss(t, anyLoss)
	withSellLossIgnoreLastDepths(t, max(SellLossIgnoreLastDepths, 1))
	trade := lastDepthLadder()
	ai := withBlock(flatMarketOver(solBlock(), trade))
	if !flatMarketReading(trade, ai.SmartTakeLoss, ai).Holds {
		t.Fatal("fixture drifted: the reading must hold")
	}
	within, under := firstPriceWithinTheDrawdown(t, trade)
	newest := trade.PositionPrice
	triggered, capital := triggeredBy(trade, ai), capitalProtectionTriggered(trade)
	atDepth := FlatMarketSaleHeldAtDepthRow(newest, lastDepthFills, lastDepthFills)
	activation := CapitalProtectionStopLossActivatedRow(newest, -handLossPct(trade, within))
	for _, position := range addSideProposals {
		assertWaits(t, Apply(trade, position, under, ai), position, triggered, capital, atDepth, CapitalProtectionSaleHeldRow(newest, -handLossPct(trade, under)))
		assertActivated(t, Apply(trade, position, within, ai), triggered, capital, atDepth, activation)
		assertSoldWith(t, Apply(restingIn(trade, "stopLoss"), position, within, ai), reasonCapitalProtection, triggered, capital)
	}

	withSellLossIgnoreLastDepths(t, 0)
	for _, position := range addSideProposals {
		assertSoldWith(t, Apply(restingIn(trade, "stopLoss"), position, within, ai), reasonFlatMarket, triggered, capital)
		assertActivated(t, Apply(trade, position, within, ai), triggered, capital, FlatMarketStopLossActivatedRow(newest, -handLossPct(trade, within)), activation)
	}
}

// Under the shipped cap the slow decline's trigger and capital protection's are judged apart on
// one tick: a ladder pending on the quiet decline at its last depth, over the cap and over the
// drawdown, is held twice with a row each; within the drawdown but still over the cap capital
// protection activates buy with its row alone and sells a self-armed stopLoss under its reason,
// while the slow decline keeps waiting; within the cap the slow decline, first in trigger order,
// names the sale. With capital protection off the cap holds the action as before.
func TestCapitalProtectionActsOnItsOwnTriggerWhileTheSharedTriggerHolds(t *testing.T) {
	underTheShippedCap(t)
	withCapitalProtectionExit(t, true)
	if shippedLoss <= 0 || shippedLoss >= capitalProtectionMaxDrawdown {
		t.Skip("SellLossTriggerPercentage takes no loss, or as much as capital protection's drawdown")
	}
	last := lastDepthLadder()
	trade := withRows(last, quietPending(last.PositionPrice))
	withinDrawdown, overDrawdown := firstPriceWithinTheDrawdown(t, last)
	withinCap, _ := firstPriceWithinTheCap(t, last, withinDrawdown)
	ai, fill := withBlock(withEveryRowTrend(solBlock())), last.PositionPrice
	capital := capitalProtectionTriggered(last)
	quietHeld := func(price float64) Row {
		return SaleHeldRow(fill, -handLossPct(last, price), FeatureQuietDecline)
	}

	for _, position := range addSideProposals {
		assertWaits(t, Apply(trade, position, overDrawdown, ai), position, capital, quietHeld(overDrawdown), CapitalProtectionSaleHeldRow(fill, -handLossPct(last, overDrawdown)))
		assertActivated(t, Apply(trade, position, withinDrawdown, ai), capital, CapitalProtectionStopLossActivatedRow(fill, -handLossPct(last, withinDrawdown)))
		assertSoldWith(t, Apply(restingIn(trade, "stopLoss"), position, withinDrawdown, ai), reasonCapitalProtection, capital)
		assertSoldWith(t, Apply(restingIn(trade, "stopLoss"), position, withinCap, ai), reasonSlowDecline, capital)
	}

	withCapitalProtectionExit(t, false)
	for _, state := range []string{"buy", "stopLoss"} {
		assertWaits(t, Apply(restingIn(trade, state), "stopLoss", withinDrawdown, ai), "stopLoss", quietHeld(withinDrawdown))
	}
}
