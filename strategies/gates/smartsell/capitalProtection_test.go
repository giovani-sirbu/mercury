package smartsell

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The fixtures: the w3s ladder at its last depth (lastDepthLadder) with lastDepthPrint in its dead
// zone, and the round ladder at its last depth (capitalProtectionLadder), long or inverse, whose
// drawdown prices are exact on the cent grid.
func TestCapitalProtectionFixture(t *testing.T) {
	withCapitalProtectionExit(t, true)
	trade := lastDepthLadder()
	if ladder.ConfiguredDepths(trade) != lastDepthFills || ladder.CountFilledEntries(trade) != lastDepthFills {
		t.Fatal("fixture drifted: the ladder must hold its last configured depth")
	}
	if !(trade.PositionPrice < lastDepthPrint && lastDepthPrint < ladder.AverageEntryPrice(trade)) || handLossPct(trade, lastDepthPrint) <= capitalProtectionMaxDrawdown {
		t.Fatal("fixture drifted: the print must sit over the newest fill, under break even and at a loss over the drawdown")
	}
	for _, inverse := range []bool{false, true} {
		round := capitalProtectionLadder(inverse)
		if !capitalProtectionWatched(round) || ladder.ConfiguredDepths(round) != len(roundPrices) {
			t.Fatalf("fixture drifted: the round ladder (inverse %v) must be at its last depth", inverse)
		}
	}
}

// Capital protection watches a ladder from its last configured depth, its next depth refused for
// funds (TestCapitalProtectionWatchesOnlyAfterTheFundsRefusal) — a partial fill of that depth's
// order and a row lowered under the fills included — and not one depth short of it, long or
// inverse. A ladder whose depth count is unknown, a futures ladder and a ladder under an
// impasse strategy are never watched, and nothing is while capitalprotection.Enabled is off.
// rebuildState reads the same watch off its fills.
func TestCapitalProtectionWatchesTheLastDepthOnly(t *testing.T) {
	withCapitalProtectionExit(t, true)
	partial := lastDepthLadder()
	partial.History[lastDepthFills-1].Quantity = 0.4
	lowered := lastDepthLadder()
	lowered.StrategyPair.StrategySettings[0].Depths = lastDepthFills - 1
	unknown := lastDepthLadder()
	unknown.StrategyPair.StrategySettings[0].Depths = 0
	futures := lastDepthLadder()
	futures.Strategy.TradeType = aggragates.Futures
	impasse := lastDepthLadder()
	impasse.Strategy.Params.Impasse = true

	for name, tc := range map[string]struct {
		trade aggragates.Trades
		want  bool
	}{
		"at its last depth":                 {lastDepthLadder(), true},
		"a partial fill of the last depth":  {partial, true},
		"a row lowered under its fills":     {lowered, true},
		"an inverse ladder":                 {fundsRefused(testutil.LadderTrade(true, fills(lastDepthFills, "21:30:00")...)), true},
		"one depth short":                   {testutil.LadderTrade(false, fills(lastDepthFills-1, "21:30:00")...), false},
		"an inverse ladder one depth short": {testutil.LadderTrade(true, fills(lastDepthFills-1, "21:30:00")...), false},
		"an unknown depth count":            {unknown, false},
		"a futures ladder":                  {futures, false},
		"an impasse strategy":               {impasse, false},
	} {
		if got := capitalProtectionWatched(tc.trade); got != tc.want {
			t.Errorf("%s: watched %v, want %v", name, got, tc.want)
		}
		if got := rebuildState(tc.trade).capitalProtectionWatched; got != tc.want {
			t.Errorf("%s: rebuildState reads the watch as %v, want %v", name, got, tc.want)
		}
	}

	withCapitalProtectionExit(t, false)
	if capitalProtectionWatched(lastDepthLadder()) || rebuildState(lastDepthLadder()).capitalProtectionWatched {
		t.Fatal("switched off, capital protection watches no ladder")
	}
}

// The tick opens an episode on the trend flag of the ladder's side — bearish for a long ladder,
// bullish for an inverse one — with the triggered row, and an open episode is sticky: the flag
// gone writes nothing and the trigger stands. A new fill past the ceiling resets the episode with
// the reset row and the flag is read afresh on the same tick. The activation's memory binds only
// while the ladder rests in trailingBuy.
func TestCapitalProtectionTickOpensAStickyEpisodeAndResetsItAtANewDepth(t *testing.T) {
	capitalProtectionAlone(t)
	start := capitalProtectionLadder(false)
	st, nothing, fill := rebuildState(start), aggragates.SmartSellIndicators{}, start.PositionPrice
	bullish := aggragates.SmartSellIndicators{CapitalProtectionSmcBullish: true}
	if next, rows, holds := capitalProtectionTick(start, st, nothing, time.Time{}); holds || len(rows) != 0 || next.capitalProtectionActive {
		t.Fatalf("no trend reading opens nothing, got %v %+v %+v", holds, rows, next)
	}
	if _, _, holds := capitalProtectionTick(start, st, bullish, time.Time{}); holds {
		t.Fatal("a long ladder reads the bearish flag alone")
	}
	next, rows, holds := capitalProtectionTick(start, st, solBlock(), time.Time{})
	if !holds || !reflect.DeepEqual(rows, []Row{capitalProtectionTriggered(start)}) || !next.capitalProtectionActive || next.capitalProtectionFrom != fill {
		t.Fatalf("the bearish flag opens the episode at the newest fill, got %v %+v %+v", holds, rows, next)
	}
	inverse := capitalProtectionLadder(true)
	if _, rows, holds := capitalProtectionTick(inverse, rebuildState(inverse), bullish, time.Time{}); !holds || !reflect.DeepEqual(rows, []Row{capitalProtectionTriggered(inverse)}) {
		t.Fatalf("an inverse ladder opens on the bullish flag, got %v %+v", holds, rows)
	}
	if _, _, holds := capitalProtectionTick(inverse, rebuildState(inverse), solBlock(), time.Time{}); holds {
		t.Fatal("an inverse ladder reads the bullish flag alone")
	}

	open := withRows(start, capitalProtectionTriggered(start))
	if next, rows, holds := capitalProtectionTick(open, rebuildState(open), nothing, time.Time{}); !holds || len(rows) != 0 || !next.capitalProtectionActive {
		t.Fatalf("an open episode is sticky, got %v %+v %+v", holds, rows, next)
	}

	if rebuildState(withFill(open, 97, testutil.At("14:00:00"))).capitalProtectionWatched {
		t.Fatal("a fill past the ceiling with no refusal after it leaves the ladder unwatched")
	}
	deeper := fundsRefused(withFill(open, 97, testutil.At("14:00:00")))
	if st := rebuildState(deeper); !st.capitalProtectionWatched || !st.capitalProtectionActive {
		t.Fatalf("fixture drifted: a fill past the ceiling keeps the ladder watched with its episode open, got %+v", st)
	}
	next, rows, holds = capitalProtectionTick(deeper, rebuildState(deeper), solBlock(), time.Time{})
	if want := []Row{CapitalProtectionResetRow(97), capitalProtectionTriggered(deeper)}; !holds || !reflect.DeepEqual(rows, want) || next.capitalProtectionFrom != 97 {
		t.Fatalf("a new depth resets the episode and the flag opens another on the same tick, got %v %+v %+v", holds, rows, next)
	}
	next, rows, holds = capitalProtectionTick(deeper, rebuildState(deeper), nothing, time.Time{})
	if want := []Row{CapitalProtectionResetRow(97)}; holds || !reflect.DeepEqual(rows, want) || next.capitalProtectionActive {
		t.Fatalf("a new depth with no reading leaves the episode closed, got %v %+v %+v", holds, rows, next)
	}

	committed := withRows(open, CapitalProtectionActivatedRow(fill, -0.5))
	if next, _, _ := capitalProtectionTick(restingIn(committed, "trailingBuy"), rebuildState(committed), nothing, time.Time{}); !next.capitalProtectionActivated {
		t.Fatal("the activation binds while the ladder rests in trailingBuy")
	}
	if next, _, _ := capitalProtectionTick(committed, rebuildState(committed), nothing, time.Time{}); next.capitalProtectionActivated {
		t.Fatal("the activation is forgotten once the ladder rests elsewhere")
	}
}

// The episode of a long ladder tick by tick (engineStep): the first tick the trend reads against
// it opens the episode with the triggered row, over the drawdown the held row goes out once, the
// proposal passing through, the first tick within the drawdown activates with the activation row
// alone, and from trailingBuy the action trails (hold, re-anchor, sell) under capital protection's
// reason; only the sale closes it.
func TestCapitalProtectionEpisodeOfALongLadder(t *testing.T) {
	capitalProtectionAlone(t)
	start := capitalProtectionLadder(false)
	ai, gone, fill := withBlock(solBlock()), nothingServed(), start.PositionPrice
	atTheDrawdown, oneStepLower := drawdownPrices(t, start)
	triggered := capitalProtectionTriggered(start)
	held := CapitalProtectionSaleHeldRow(fill, -handLossPct(start, oneStepLower))
	activation := CapitalProtectionActivatedRow(fill, -handLossPct(start, atTheDrawdown))

	trade, got := engineStep(start, "", oneStepLower, testutil.At("13:00:00"), ai)
	assertWaits(t, got, "", triggered, held)
	if st := rebuildState(trade); !st.capitalProtectionWatched || !st.capitalProtectionActive || !st.capitalProtectionSaleHeld || st.capitalProtectionActivated || st.capitalProtectionFrom != fill {
		t.Fatalf("the first tick opens the episode and the drawdown holds the action, got %+v", st)
	}
	trade, got = engineStep(trade, "buy", oneStepLower, testutil.At("13:05:00"), gone)
	assertUntouched(t, got, "buy")
	trade, got = engineStep(trade, "update_trailingBuy", oneStepLower-1, testutil.At("13:15:00"), ai)
	assertUntouched(t, got, "update_trailingBuy")

	trade, got = engineStep(trade, "trailingBuy", atTheDrawdown, testutil.At("13:30:00"), gone)
	assertActivated(t, got, activation)
	if trade.PositionType != "trailingBuy" || trade.PositionPrice != atTheDrawdown {
		t.Fatalf("the activation chain leaves the ladder in trailingBuy anchored at its print, got %q at %v", trade.PositionType, trade.PositionPrice)
	}
	if st := rebuildState(trade); !st.capitalProtectionActive || !st.capitalProtectionActivated || !st.capitalProtectionSaleHeld || st.capitalProtectionFrom != fill {
		t.Fatalf("the episode is open with its trailingBuy activated, got %+v", st)
	}

	gain, falls := ladder.AverageEntryPrice(start)*gainFactor, priceAtMove(trade, atTheDrawdown, -(trailRow(trade).Tolerance+1))
	for _, reading := range []aggragates.AIIndicators{ai, gone} {
		for _, position := range addSideProposals {
			assertTrailHeld(t, Apply(trade, position, atTheDrawdown, reading))
			assertTrailHeld(t, Apply(trade, position, oneStepLower, reading))
			assertReanchored(t, Apply(trade, position, gain, reading))
			assertSold(t, Apply(trade, position, falls, reading), reasonCapitalProtection)
		}
	}

	moved, got := engineStep(trade, "buy", gain, testutil.At("13:35:00"), gone)
	assertReanchored(t, got)
	if st := rebuildState(moved); moved.PositionPrice != gain || moved.PositionType != "trailingBuy" || !st.capitalProtectionActive || !st.capitalProtectionActivated || st.capitalProtectionFrom != fill {
		t.Fatalf("a re-anchor moves the anchor to the print and the episode stands in trailingBuy, got %v %q %+v", moved.PositionPrice, moved.PositionType, st)
	}

	sold := Apply(trade, "buy", falls, gone)
	closed := withRow(trade, ExitRow(trade, sold.Reason, falls), testutil.At("13:45:00"))
	if st := rebuildState(closed); st.capitalProtectionActive || st.capitalProtectionSaleHeld || st.capitalProtectionActivated || st.capitalProtectionFrom != 0 {
		t.Fatalf("the sold row closes the episode, got %+v", st)
	}
	if want := []string{eventAt(EventTriggered, fill), eventAt(EventSaleHeld, fill), eventAt(EventActivated, fill), eventAt(EventSold, falls)}; !reflect.DeepEqual(gateEvents(t, closed, GateCapitalProtection), want) {
		t.Errorf("events = %q, want %q", gateEvents(t, closed, GateCapitalProtection), want)
	}
	if msg := ExitMessage(lastDepthLadder(), sold.Reason, lastDepthPrint); msg != "smartSell: take loss at capital protection 172.40" {
		t.Fatalf("unexpected exit row %q", msg)
	}
}

// The episode of an inverse ladder is the mirror: the bullish flag opens it, the loss is counted
// on the base its entries sold and grows as the price rises, the pullback is the fall back toward
// break even, and the trail reads the move the other way round.
func TestCapitalProtectionEpisodeOfAnInverseLadder(t *testing.T) {
	capitalProtectionAlone(t)
	start := capitalProtectionLadder(true)
	ai, gone, fill := withBlock(aggragates.SmartSellIndicators{CapitalProtectionSmcBullish: true}), nothingServed(), start.PositionPrice
	average := ladder.AverageEntryPrice(start)
	within := math.Floor(average/(1-capitalProtectionMaxDrawdown/100)*100) / 100
	over := within + 0.01
	if _, standing := saleProfit(start, within, 0-capitalProtectionMaxDrawdown); standing != reached {
		t.Fatalf("fixture drifted: %v must be within the drawdown", within)
	}
	if _, standing := saleProfit(start, over, 0-capitalProtectionMaxDrawdown); standing != short {
		t.Fatalf("fixture drifted: %v must be over the drawdown", over)
	}
	triggered := capitalProtectionTriggered(start)
	held := CapitalProtectionSaleHeldRow(fill, -inverseLossPctOf(start, over))
	activation := CapitalProtectionActivatedRow(fill, -inverseLossPctOf(start, within))

	assertUntouched(t, Apply(start, "", over, withBlock(solBlock())), "")
	trade, got := engineStep(start, "", over, testutil.At("13:00:00"), ai)
	assertWaits(t, got, "", triggered, held)
	trade, got = engineStep(trade, "trailingBuy", over, testutil.At("13:05:00"), gone)
	assertUntouched(t, got, "trailingBuy")
	trade, got = engineStep(trade, "trailingBuy", within, testutil.At("13:30:00"), gone)
	assertActivated(t, got, activation)
	if trade.PositionType != "trailingBuy" || trade.PositionPrice != within {
		t.Fatalf("the activation leaves the ladder in trailingBuy anchored at its print, got %q at %v", trade.PositionType, trade.PositionPrice)
	}

	row := trailRow(trade)
	falls, rises := priceAtMove(trade, within, row.TrailingTakeProfit+1), priceAtMove(trade, within, -(row.Tolerance+1))
	for _, position := range addSideProposals {
		assertTrailHeld(t, Apply(trade, position, within, gone))
		assertTrailHeld(t, Apply(trade, position, over, gone))
		assertReanchored(t, Apply(trade, position, falls, gone))
		assertSold(t, Apply(trade, position, rises, gone), reasonCapitalProtection)
	}
	closed := withRow(trade, ExitRow(trade, reasonCapitalProtection, rises), testutil.At("13:45:00"))
	if want := []string{eventAt(EventTriggered, fill), eventAt(EventSaleHeld, fill), eventAt(EventActivated, fill), eventAt(EventSold, rises)}; !reflect.DeepEqual(gateEvents(t, closed, GateCapitalProtection), want) {
		t.Errorf("events = %q, want %q", gateEvents(t, closed, GateCapitalProtection), want)
	}
}

// A ladder resting in a self-armed trailingBuy, the force-trailing state included, is sold on
// capital protection's trigger within the drawdown and held with the held row over it, whatever
// the add-side proposal; the sale carries the episode's row of the tick and no row of the action.
func TestCapitalProtectionSellsASelfArmedStopLossOnItsTrigger(t *testing.T) {
	capitalProtectionAlone(t)
	trade := lastDepthLadder()
	within, under := firstPriceWithinTheDrawdown(t, trade)
	ai, fill := withBlock(solBlock()), trade.PositionPrice
	triggered := capitalProtectionTriggered(trade)
	for _, state := range []string{"trailingBuy", "forceTrailingBuy"} {
		for _, position := range addSideProposals {
			assertSoldWith(t, Apply(restingIn(trade, state), position, within, ai), reasonCapitalProtection, triggered)
			assertWaits(t, Apply(restingIn(trade, state), position, under, ai), position, triggered, CapitalProtectionSaleHeldRow(fill, -handLossPct(trade, under)))
		}
	}
	for _, position := range addSideProposals {
		assertActivated(t, Apply(trade, position, within, ai), triggered, CapitalProtectionActivatedRow(fill, -handLossPct(trade, within)))
	}
}

// It replaces only a take profit proposed in buy: that one gives way to the activation within
// the drawdown and stands beside the held row over it, and passes through in trailingBuy; every
// other close, the ladder's or the trade's own state when that is a close — the trailing take
// profit included — passes through within the drawdown and over it, with no row. It leaves the
// take profit reading alone and holds nothing: the first fill of a new ladder is not held on it.
func TestApplyCapitalProtectionReplacesOnlyATakeProfitProposedInBuy(t *testing.T) {
	capitalProtectionAlone(t)
	trade := lastDepthLadder()
	within, _ := firstPriceWithinTheDrawdown(t, trade)
	ai, fill := withBlock(solBlock()), trade.PositionPrice
	triggered := capitalProtectionTriggered(trade)
	for _, position := range takeProfitProposals {
		assertActivated(t, Apply(trade, position, within, ai), triggered, CapitalProtectionActivatedRow(fill, -handLossPct(trade, within)))
		held := CapitalProtectionSaleHeldRow(fill, -handLossPct(trade, lastDepthPrint))
		assertWaits(t, Apply(trade, position, lastDepthPrint, ai), position, triggered, held)
		for _, price := range []float64{within, lastDepthPrint} {
			assertUntouched(t, Apply(restingIn(trade, "trailingBuy"), position, price, ai), position)
		}
	}
	for _, price := range []float64{within, lastDepthPrint} {
		for _, position := range []string{"sell", "update_takeProfit", "sellParent", "impasse", "takeLoss"} {
			assertUntouched(t, Apply(trade, position, price, ai), position)
		}
		for _, state := range []string{"sell", "takeProfit", "sellParent", "impasse", "takeLoss", "forceTrailingTakeProfit"} {
			for _, position := range []string{"", "trailingBuy"} {
				assertUntouched(t, Apply(restingIn(trade, state), position, price, ai), position)
			}
		}
		if got := TakeProfitPercentage(trade, price, breakEvenReading); got != breakEvenReading {
			t.Fatalf("at %v the take profit must read its input, got %v", price, got)
		}
	}
	if got := EntryHold(testutil.LadderTrade(false), aggragates.SideLong, ai); got.Held() {
		t.Fatalf("capital protection holds no first fill, got %+v", got)
	}
}

// The shared action's depth bounds hold the other gates' triggers and never capital protection's:
// a ladder pending on the quiet decline at its last depth has the slow decline's trigger refused
// by the ceiling (its held-at-depth row once) and by the floor, while capital protection's is judged
// on the drawdown beside it — held over it with its own row, activating buy within it, selling a
// self-armed trailingBuy within it under its own reason; the refusal rows come before its own.
func TestCapitalProtectionIgnoresTheSharedBoundsThatHoldTheOtherGates(t *testing.T) {
	capitalProtectionAlone(t)
	withQuietDecline(t, true)
	last := lastDepthLadder()
	pending := withRows(last, quietPending(last.PositionPrice))
	within, under := firstPriceWithinTheDrawdown(t, last)
	ai, fill := withBlock(solBlock()), last.PositionPrice
	if st := rebuildState(pending); !slowDeclineTriggered(pending, st) || !st.capitalProtectionWatched {
		t.Fatalf("fixture drifted: the slow decline's trigger must stand on a ladder capital protection watches, got %+v", st)
	}
	triggered := capitalProtectionTriggered(last)
	activation := CapitalProtectionActivatedRow(fill, -handLossPct(last, within))

	withSaleIgnoreLastDepths(t, max(SaleIgnoreLastDepths, 1))
	atDepth := SaleHeldAtDepthRow(fill, lastDepthFills, lastDepthFills, FeatureQuietDecline)
	for _, position := range addSideProposals {
		assertWaits(t, Apply(pending, position, under, ai), position, triggered, atDepth, CapitalProtectionSaleHeldRow(fill, -handLossPct(last, under)))
		assertActivated(t, Apply(pending, position, within, ai), triggered, atDepth, activation)
		assertSoldWith(t, Apply(restingIn(pending, "trailingBuy"), position, within, ai), reasonCapitalProtection, triggered)
	}

	withSaleIgnoreLastDepths(t, 0)
	withSaleStartAtDepth(t, lastDepthFills+1)
	underFloor := heldRowOf(saleTrigger{gate: GateSlowDecline, features: []string{FeatureQuietDecline}}, fill, heldUnderDepth(lastDepthFills, lastDepthFills+1))
	assertActivated(t, Apply(pending, "", within, ai), triggered, underFloor, activation)
}

// Capital protection is immune to the depth priority pause: a ladder at its last depth the
// cooldown gate holds after its newest fill still opens its episode, is held over the drawdown,
// activates within it and sells a self-armed trailingBuy, while a ladder pending on the quiet decline
// is reset by the pause and the slow decline's trigger stays paused beside capital protection's.
func TestCapitalProtectionIsImmuneToTheDepthPriorityPause(t *testing.T) {
	capitalProtectionAlone(t)
	withDepthPriorityPause(t, true)
	last := lastDepthLadder()
	held := heldBy(last, testutil.At("21:45:00"))
	within, under := firstPriceWithinTheDrawdown(t, last)
	ai, fill := withBlock(solBlock()), last.PositionPrice
	if !rebuildState(held).depthPriorityHeld {
		t.Fatal("fixture drifted: the ladder must be held by the depth priority")
	}
	triggered := capitalProtectionTriggered(last)
	activation := CapitalProtectionActivatedRow(fill, -handLossPct(last, within))
	assertWaits(t, Apply(held, "", under, ai), "", triggered, CapitalProtectionSaleHeldRow(fill, -handLossPct(last, under)))
	assertActivated(t, Apply(held, "trailingBuy", within, ai), triggered, activation)
	assertSoldWith(t, Apply(restingIn(held, "trailingBuy"), "", within, ai), reasonCapitalProtection, triggered)

	withQuietDecline(t, true)
	pending := heldBy(withRows(last, quietPending(fill)), testutil.At("21:45:00"))
	assertActivated(t, Apply(pending, "", within, ai), ResetRow(fill, quietFinding(nil)), triggered, activation)
}

// Each event of the gate folds into the episode: triggered opens it at its price, the held sale and
// the activation keep it open (opening it if none was) and remember themselves, reset and sold close
// it; another gate's events and a ladder short of its last depth fold nothing.
func TestCapitalProtectionFoldsItsEpisodeFromItsEvents(t *testing.T) {
	capitalProtectionAlone(t)
	start := capitalProtectionLadder(false)
	fill := start.PositionPrice
	triggered, reset := capitalProtectionTriggered(start), CapitalProtectionResetRow(fill)
	held, activated := CapitalProtectionSaleHeldRow(fill, -2), CapitalProtectionActivatedRow(fill, -0.5)
	sold := ExitRow(start, reasonCapitalProtection, fill)
	for name, tc := range map[string]struct {
		rows                        []Row
		active, saleHeld, activated bool
		from                        float64
	}{
		"nothing":                           {nil, false, false, false, 0},
		"triggered":                         {[]Row{triggered}, true, false, false, fill},
		"triggered, held":                   {[]Row{triggered, held}, true, true, false, fill},
		"triggered, held, activated":        {[]Row{triggered, held, activated}, true, true, true, fill},
		"held alone opens the episode":      {[]Row{held}, true, true, false, fill},
		"activated alone opens the episode": {[]Row{activated}, true, false, true, fill},
		"triggered, reset":                  {[]Row{triggered, reset}, false, false, false, 0},
		"triggered, held, sold":             {[]Row{triggered, held, sold}, false, false, false, 0},
		"reset, triggered again":            {[]Row{triggered, reset, triggered}, true, false, false, fill},
		"another gate's events":             {[]Row{FlatMarketTriggeredRow(fill, nil), FlatMarketSaleHeldRow(fill, -2)}, false, false, false, 0},
	} {
		st := rebuildState(restingIn(withRows(start, tc.rows...), "trailingBuy"))
		if st.capitalProtectionActive != tc.active || st.capitalProtectionSaleHeld != tc.saleHeld || st.capitalProtectionActivated != tc.activated || st.capitalProtectionFrom != tc.from {
			t.Errorf("%s: got active %v held %v activated %v from %v, want %v %v %v %v", name, st.capitalProtectionActive, st.capitalProtectionSaleHeld, st.capitalProtectionActivated, st.capitalProtectionFrom, tc.active, tc.saleHeld, tc.activated, tc.from)
		}
	}
	if st := rebuildState(withRows(flatMarketLadder(false), triggered)); st.capitalProtectionActive {
		t.Fatalf("a ladder short of its last depth folds no episode, got %+v", st)
	}
}

// At its last depth within capital protection's drawdown, a ladder in trailingBuy that the flat market
// sells on the same tick is the flat market's sale, and capital protection acts on nothing beside it;
// switched off, capital protection acts on nothing where the trend reads against the ladder.
func TestCapitalProtectionNeverActsBesideAnotherGatesSaleNorSwitchedOff(t *testing.T) {
	capitalProtectionAlone(t)
	last := lastDepthLadder()
	within, _ := firstPriceWithinTheDrawdown(t, last)
	bearish := solBlock()
	if !CapitalProtectionActs(last, within, bearish, time.Time{}) {
		t.Fatal("fixture drifted: the trend against the ladder within the drawdown must act for capital protection")
	}

	withFlatMarket(t, true)
	market := withBlock(flatMarketOver(bearish, last))
	if !flatMarketReading(last, market.SmartSell, market).Holds {
		t.Fatal("fixture drifted: the flat market reading must hold")
	}
	sold := Apply(restingIn(last, "trailingBuy"), "", within, market)
	if sold.Reason != reasonFlatMarket || CapitalProtectionActs(restingIn(last, "trailingBuy"), within, market.SmartSell, time.Time{}) {
		t.Fatalf("another gate's sale on the same tick never counts, got %+v", sold)
	}
	withCapitalProtectionExit(t, false)
	if CapitalProtectionActs(last, within, bearish, time.Time{}) {
		t.Fatal("switched off, nothing acts")
	}
}
