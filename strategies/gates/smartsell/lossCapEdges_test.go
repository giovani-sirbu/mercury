package smartsell

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"github.com/giovani-sirbu/mercury/trades/profit"
)

// The edges of the shared action's bounds that the fixtures of lossCap_test.go leave to
// their defaults: the position price the ladder carries, the value of the cap in force,
// the rows one tick can hand back beside the held one, the features the held row names,
// the order the two bounds are read in and the one row that covers both of them.

// Every row of the gate is filed at the newest fill, the held row included, and
// never at the position price: a re-anchor of the trailing stop loss or take
// profit moves it off the fill, and a state the ladder may rest in leaves it at
// none, which the fold would skip as an event with no price (readEvent) and so
// write the held row afresh on every tick the action stays held. For every state the
// action acts on, and a position price over the fill, under it and none, the held row is
// at the fill, folds back as the action held, and a later tick at the print, under it and
// over it writes nothing; the trigger and the latch of a ladder not pending are at the
// fill too, in every state a ladder can rest in short of a close, read with a buy order
// resting so that the action waits.
func TestTheHeldRowCarriesTheNewestFillNeverThePositionPrice(t *testing.T) {
	underTheShippedCap(t)
	withEarlyTakeProfit(t, true)
	armed, _ := armingOverTheCap(t, pendingTrade())
	moves := []float64{slowDeclineLastFill + 2.5, slowDeclineLastFill - 3, 0}
	for _, state := range actingStates {
		for _, moved := range moves {
			name := fmt.Sprintf("resting in %q with the position price at %v", state, moved)
			trade := pendingTrade()
			trade.PositionType, trade.PositionPrice = state, moved

			got := Apply(trade, "trailingBuy", armed, nothingServed())
			assertWaits(t, got, "trailingBuy", SaleHeldRow(slowDeclineLastFill, -handLossPct(trade, armed), FeatureQuietDecline))

			held := withRow(trade, got.Rows[0], testutil.At("21:45:00"))
			if st := rebuildState(held); !st.saleHeld || st.saleHeldRowDue() {
				t.Errorf("%s: the held row folds back as the action held, and is not due again, got %+v", name, st)
			}
			for _, price := range []float64{armed - 0.5, armed, armed + 0.5} {
				for _, position := range addSideProposals {
					assertUntouched(t, Apply(held, position, price, nothingServed()), position)
				}
			}
		}
	}

	gain := ladder.AverageEntryPrice(watchedTrade()) * gainFactor
	for _, state := range slices.Concat([]string{"new", ""}, actingStates) {
		for _, moved := range moves {
			plain := withRestingOrder(watchedTrade())
			plain.PositionType, plain.PositionPrice = state, moved
			assertRow(t, Apply(plain, "", gain, slowDeclineBlock(true)), quietPending(slowDeclineLastFill))
			assertRow(t, Apply(plain, "", gain, indecisionReading()), LatchedRow(slowDeclineLastFill, indecisionFinding(indecisionReasons)))
		}
	}
}

// firstPriceWithinThe is firstPriceWithinTheCap at another cap: the lowest price
// on the two-decimal grid from floor up to break even at which a sale loses no
// more than pct percent of the used amount, and the price one step under it. It
// reports no boundary when floor is already within the cap.
func firstPriceWithinThe(trade aggragates.Trades, floor, pct float64) (within, oneStepLower float64, found bool) {
	for cents := int(math.Round(floor * 100)); float64(cents)/100 <= ladder.AverageEntryPrice(trade); cents++ {
		if handLossPct(trade, float64(cents)/100) <= pct {
			if float64(cents-1)/100 < floor {
				return 0, 0, false
			}
			return float64(cents) / 100, float64(cents-1) / 100, true
		}
	}
	return 0, 0, false
}

// The trigger is the value in force, not the constant it starts from: at every trigger
// the boundary moves with it — the price that loses exactly what the trigger takes goes
// ahead, one step lower is held — and the rows name the trigger they were made under,
// signed with two decimals, beside the profit: "at or over" on the activation of a ladder
// in buy, "under" on the held row of either state. A trigger under the loss of the arming
// print never holds the action, and one at a loss of the whole used amount never holds it at any price.
func TestTheBoundaryAndTheRowFollowTheCapInForce(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingTrade()
	floor := armingPrice(trade)
	armedLoss := handLossPct(trade, floor)
	for _, pct := range []float64{0.5, 1, 2.5, 3} {
		withAllowedLoss(t, pct)
		within, oneStepLower, found := firstPriceWithinThe(trade, floor, pct)
		if !found {
			t.Fatalf("fixture drifted: the w3s ladder has a boundary between its arming print and break even at a cap of %v%%", pct)
		}
		for _, state := range []string{"buy", "trailingBuy"} {
			resting := restingIn(trade, state)
			ahead := Apply(resting, "trailingBuy", within, nothingServed())
			assertGoesAhead(t, ahead, state)
			if state == "buy" {
				assertRows(t, ahead, ActivatedRow(slowDeclineLastFill, -handLossPct(trade, within), FeatureQuietDecline))
				if want := fmt.Sprintf("at or over %.2f%%", -pct); !strings.HasSuffix(ahead.Rows[0].Message, want) {
					t.Errorf("a cap of %v%%: the activation row must end %q, got %q", pct, want, ahead.Rows[0].Message)
				}
			}
			held := Apply(resting, "trailingBuy", oneStepLower, nothingServed())
			assertWaits(t, held, "trailingBuy", SaleHeldRow(slowDeclineLastFill, -handLossPct(trade, oneStepLower), FeatureQuietDecline))
			if want := fmt.Sprintf("under %.2f%%", -pct); !strings.HasSuffix(held.Rows[0].Message, want) {
				t.Errorf("a cap of %v%%: the held row must end %q, got %q", pct, want, held.Rows[0].Message)
			}
		}
	}

	for _, pct := range []float64{math.Ceil(armedLoss), 1.5 * armedLoss, 20, 99} {
		withAllowedLoss(t, pct)
		if _, _, found := firstPriceWithinThe(trade, floor, pct); found || armedLoss > pct {
			t.Fatalf("fixture drifted: the arming print's own loss must be within a cap of %v%%", pct)
		}
		assertActivated(t, Apply(trade, "trailingBuy", floor, nothingServed()), ActivatedRow(slowDeclineLastFill, -armedLoss, FeatureQuietDecline))
		assertSold(t, Apply(restingIn(trade, "trailingBuy"), "trailingBuy", floor, nothingServed()), reasonSlowDecline)
	}

	withAllowedLoss(t, 100)
	exact := exactLadder()
	for _, price := range []float64{armingPrice(exact), 70, 50} {
		assertActivated(t, Apply(exact, "trailingBuy", price, nothingServed()), ActivatedRow(roundPrices[len(roundPrices)-1], -handLossPct(exact, price), FeatureQuietDecline))
		assertSold(t, Apply(restingIn(exact, "trailingBuy"), "trailingBuy", price, nothingServed()), reasonSlowDecline)
	}

	for _, pct := range []float64{2, 4, 6, 8, 10, 20} {
		withAllowedLoss(t, pct)
		atTheCap, oneStepLower, onTheGrid := exactBoundaryAt(pct)
		if !onTheGrid {
			t.Fatalf("fixture drifted: the exact ladder must lose exactly %v%% at a price of the two-decimal grid, got %v and %v", pct, atTheCap, oneStepLower)
		}
		for _, state := range []string{"buy", "trailingBuy"} {
			resting := restingIn(exact, state)
			assertGoesAhead(t, Apply(resting, "trailingBuy", atTheCap, nothingServed()), state)
			held := Apply(resting, "trailingBuy", oneStepLower, nothingServed())
			assertWaits(t, held, "trailingBuy", SaleHeldRow(roundPrices[len(roundPrices)-1], -handLossPct(exact, oneStepLower), FeatureQuietDecline))
			if want := fmt.Sprintf("under %.2f%%", -pct); !strings.HasSuffix(held.Rows[0].Message, want) {
				t.Errorf("a cap of %v%%: the row must end %q, got %q", pct, want, held.Rows[0].Message)
			}
		}
	}
}

// roundPrices are the fills of the round ladder, the exact ladder of lossCap_test.go
// and the one the flat market gate's tests read: whole units at whole prices whose
// break even is a whole price, so the price a whole-percent cap names is a round
// number and the loss there is exactly the cap times what the ladder used. Divided the
// other way round it is not: at some whole-percent caps the loss over what was used is
// no double, and times a hundred it rounds over the cap.
var roundPrices = []float64{102, 101, 99, 98}

// A loss exactly at the cap is within it whatever the cap is, and the comparison is the
// one the loss and the cap are stated in: the loss times a hundred against the cap
// times what the ladder used, never the loss over what it used times a hundred,
// which at some whole-percent caps reads exactly the cap as a hair over it and holds
// the action the cap lets go. One price step lower is held at each of them, in the state
// buy and in the state trailingBuy.
func TestALossExactlyAtAWholePercentCapIsWithinItWhateverTheDivisionRoundsTo(t *testing.T) {
	withQuietDecline(t, true)
	trade := exactLadder()
	if breakEven := ladder.AverageEntryPrice(trade); breakEven != 100 || ladder.EntryNotional(trade) != 40000 {
		t.Fatalf("fixture drifted: the round ladder must break even at 100 on 40000 used, got %v on %v", breakEven, ladder.EntryNotional(trade))
	}
	for _, pct := range []float64{3, 7, 14, 28} {
		withAllowedLoss(t, pct)
		const units = 4 * exactUnits
		atTheCap := 100 - pct
		oneStepLower := atTheCap - 0.01
		loss, lowerLoss := 40000-float64(units*atTheCap), 40000-float64(units*oneStepLower)
		if loss*100 != pct*40000 || lowerLoss*100 <= pct*40000 {
			t.Fatalf("fixture drifted: the round ladder must lose exactly %v%% at %v and more at %v, got %v and %v of 40000", pct, atTheCap, oneStepLower, loss, lowerLoss)
		}

		for _, state := range []string{"buy", "trailingBuy"} {
			resting := restingIn(trade, state)
			assertGoesAhead(t, Apply(resting, "trailingBuy", atTheCap, nothingServed()), state)
			held := Apply(resting, "trailingBuy", oneStepLower, nothingServed())
			assertWaits(t, held, "trailingBuy", SaleHeldRow(roundPrices[len(roundPrices)-1], -handLossPct(trade, oneStepLower), FeatureQuietDecline))
		}
	}
}

// A trade that has used nothing has nothing to measure, whatever its close books: a ledger row
// at the accounting price adds no used amount, and fees that outweigh what the close sells
// for make it a loss all the same — so it is the guard on the used amount, and not the
// sign of the profit, that keeps a share of nothing out of the answer.
func TestATradeThatHasUsedNothingHasNothingToMeasureWhateverItsFeesBook(t *testing.T) {
	underTheShippedCap(t)
	ledger := testutil.LadderTrade(false)
	ledger.History = []aggragates.TradesHistory{{
		Type: "BUY", Quantity: 12.5, Price: ladder.AccountingPriceCeiling, OrderId: 1,
		Fees: []aggragates.TradesFees{{Asset: "USDT", Fee: 1000}},
	}}
	if used := ladder.EntryNotional(ledger); used != 0 {
		t.Fatalf("fixture drifted: the ledger row must leave nothing used, got %v", used)
	}
	if booked := profit.SimulatedCloseProfit(events.Events{Trade: ledger}); booked >= 0 {
		t.Fatalf("fixture drifted: the fees must make the close a loss, got %v", booked)
	}
	if profitPct, standing := saleProfit(ledger, 100, FullTrendTrigger()); standing != nothingUsed || profitPct != 0 {
		t.Errorf("saleProfit = %v, %v, want nothing used", profitPct, standing)
	}
}

// A tick can hand back the gate's row and the held one together, the gate's
// first: a pending ladder the sophos indecision latches on a tick the cap holds
// writes the latch and then the held action, and a ladder the verdict makes pending
// on the same tick the indecision latches writes the one combined row and then
// the held action. The tick after, with every reading still served, writes nothing:
// the latch is written once and the action is held once, and the fold of the two
// rows reads back both.
func TestTheLatchAndTheHeldActionOfOneTickAreTwoRowsInThatOrder(t *testing.T) {
	underTheShippedCap(t)
	withEarlyTakeProfit(t, true)
	latching := withEveryRowTrend(aggragates.SmartSellIndicators{QuietDeclineBreakReasons: indecisionReasons, Indecision: true})

	pending := pendingTrade()
	armed, _ := armingOverTheCap(t, pending)
	ticked, got := engineTick(pending, "trailingBuy", armed, testutil.At("21:45:00"), withBlock(latching))
	assertWaits(t, got, "trailingBuy",
		LatchedRow(slowDeclineLastFill, indecisionFinding(indecisionReasons)),
		SaleHeldRow(slowDeclineLastFill, -handLossPct(pending, armed), FeatureQuietDecline),
	)
	if st := rebuildState(ticked); !st.earlyTakeProfit || !st.saleHeld || !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill {
		t.Fatalf("the latch and the held action fold back, the ladder still pending from its fill, got %+v", st)
	}
	assertUntouched(t, Apply(ticked, "trailingBuy", armed, withBlock(latching)), "trailingBuy")
	if len(ticked.Logs) != len(pending.Logs)+2 || len(ticked.StrategyEvents) != len(pending.StrategyEvents)+2 {
		t.Fatalf("two rows and two events over the tick, got %d and %d over %d and %d", len(ticked.Logs), len(ticked.StrategyEvents), len(pending.Logs), len(pending.StrategyEvents))
	}

	both := withVerdict(latching)
	plain := watchedTrade()
	ticked, got = engineTick(plain, "trailingBuy", armed, testutil.At("21:45:00"), withBlock(both))
	assertWaits(t, got, "trailingBuy",
		PendingRow(slowDeclineLastFill, indecisionFinding(indecisionReasons), quietFinding(slowDeclineReasons)),
		SaleHeldRow(slowDeclineLastFill, -handLossPct(plain, armed), FeatureQuietDecline),
	)
	if st := rebuildState(ticked); !st.earlyTakeProfit || !st.saleHeld || !st.slowDeclinePending || len(st.pendingFeatures) != 1 || st.pendingFeatures[0] != FeatureQuietDecline {
		t.Fatalf("the trigger, the latch and the held action fold back, the ladder pending on the quiet decline alone, got %+v", st)
	}
	assertUntouched(t, Apply(ticked, "trailingBuy", armed, withBlock(both)), "trailingBuy")
}

// The held row names the decline features the ladder is pending on, the latch of
// the early take profit apart: the swing decline alone, both declines, and the
// swing decline of a ladder the events written before the merge hold pending. Its
// event carries them and reads back the same, so the action is held once whichever
// feature made the ladder pending.
func TestTheHeldRowNamesTheFeaturesTheLadderIsPendingOn(t *testing.T) {
	underTheShippedCap(t)
	withSwingDecline(t, true)
	held, _ := armingOverTheCap(t, stairFourth())
	block := withBlock(stairFourthBlock())
	for name, tc := range map[string]struct {
		trade    aggragates.Trades
		features []string
	}{
		"the swing decline": {withRows(stairFourth(), PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(nil))), []string{FeatureSwingDecline}},
		"both declines":     {withRows(stairFourth(), PendingRow(stairPrices[3], indecisionFinding(nil), quietFinding(nil), swingFinding(nil))), []string{FeatureQuietDecline, FeatureSwingDecline}},
		"the legacy slow pattern": {withSteps(stairFourth(), patternEvent(EventPending, stairPrices[3]), legacyLatch(stairPrices[3])),
			[]string{FeatureSwingDecline}},
		"the legacy quiet decline and slow pattern": {withSteps(stairFourth(), quietEvent(EventPending, stairPrices[3]), patternEvent(EventPending, stairPrices[3]), legacyLatch(stairPrices[3])),
			[]string{FeatureQuietDecline, FeatureSwingDecline}},
	} {
		got := Apply(tc.trade, "trailingBuy", held, block)
		assertWaits(t, got, "trailingBuy", SaleHeldRow(stairPrices[3], -handLossPct(tc.trade, held), tc.features...))
		if row := got.Rows[0]; row.Event != EventSaleHeld || !slices.Equal(row.Features, tc.features) {
			t.Errorf("%s: the held row carries %v, want %v", name, row.Features, tc.features)
		}

		stored := withRow(tc.trade, got.Rows[0], testutil.At("21:45:00"))
		st := rebuildState(stored)
		if !st.saleHeld || st.saleHeldRowDue() || !st.slowDeclinePending || !slices.Equal(st.pendingFeatures, tc.features) {
			t.Errorf("%s: the held row folds back as the action held, the ladder still pending on %v, got %+v", name, tc.features, st)
		}
		assertUntouched(t, Apply(stored, "trailingBuy", held, block), "trailingBuy")
	}
}

// The activation names the decline features the ladder is pending on as the held row
// does, and the sale that trails it (the print next to the activation's is held, the one
// under its tolerance sells) is the slow decline gate's whichever feature made the ladder
// pending: the swing decline alone, both declines, and the swing decline of a ladder the
// events written before the merge hold pending.
func TestTheActivationRowNamesTheFeaturesTheLadderIsPendingOn(t *testing.T) {
	underTheShippedCap(t)
	withSwingDecline(t, true)
	armed, _ := armingOverTheCap(t, stairFourth())
	within, _ := firstPriceWithinTheCap(t, stairFourth(), armed)
	block := withBlock(stairFourthBlock())
	for name, tc := range map[string]struct {
		trade    aggragates.Trades
		features []string
	}{
		"the swing decline": {withRows(stairFourth(), PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(nil))), []string{FeatureSwingDecline}},
		"both declines":     {withRows(stairFourth(), PendingRow(stairPrices[3], indecisionFinding(nil), quietFinding(nil), swingFinding(nil))), []string{FeatureQuietDecline, FeatureSwingDecline}},
		"the legacy slow pattern": {withSteps(stairFourth(), patternEvent(EventPending, stairPrices[3]), legacyLatch(stairPrices[3])),
			[]string{FeatureSwingDecline}},
	} {
		got := Apply(tc.trade, "", within, block)
		assertActivated(t, got, ActivatedRow(stairPrices[3], -handLossPct(tc.trade, within), tc.features...))
		if row := got.Rows[0]; row.Event != EventActivated || !slices.Equal(row.Features, tc.features) {
			t.Errorf("%s: the activation row carries %v, want %v", name, row.Features, tc.features)
		}

		next, _ := engineStep(tc.trade, "", within, testutil.At("21:45:00"), block)
		st := rebuildState(next)
		if !st.slowDeclineActivated || !st.slowDeclinePending || !slices.Equal(st.pendingFeatures, tc.features) {
			t.Errorf("%s: the activation folds back with the ladder still pending on %v, got %+v", name, tc.features, st)
		}
		assertTrailHeld(t, Apply(next, "buy", within+1, block))
		assertSold(t, Apply(next, "buy", priceAtMove(next, within, -(trailRow(next).Tolerance+1)), block), reasonSlowDecline)
	}
}

// A pending ladder is acted on only within both bounds, and held for the first that
// fails, the depth bound read before the cap. A ladder at the deepest depth the bound
// lets go has its trailingBuy activated (in buy) or is sold (in trailingBuy) at a gain and at a
// print within the trigger, and is held for its loss at one under it; a ladder one depth past it is held for
// its depth at a gain, at a print within the cap and at one over it alike, since the loss
// is not read there. Each held row is written once, naming the bound that held it,
// whichever proposal asked.
func TestAPendingLadderIsActedOnOnlyWithinBothBounds(t *testing.T) {
	underTheShippedCap(t)
	deepest := lastDepthFills - SaleIgnoreLastDepths
	if SaleIgnoreLastDepths <= 0 || deepest < quietdecline.ArmDepth || deepest >= lastDepthFills {
		t.Skip("SaleIgnoreLastDepths is off or leaves no depth the quiet decline watches the action is made at and held past")
	}
	withSaleIgnoreLastDepths(t, SaleIgnoreLastDepths)
	pendingAt := func(depth int) aggragates.Trades {
		trade := depthLadder(depth)
		return withRows(trade, quietPending(trade.PositionPrice))
	}

	within, past := pendingAt(deepest), pendingAt(deepest+1)
	armed, armedLoss := armingOverTheCap(t, within)
	cheap, _ := firstPriceWithinTheCap(t, within, armed)
	gain := ladder.AverageEntryPrice(within) * 1.02
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, position := range addSideProposals {
			assertGoesAhead(t, Apply(restingIn(within, state), position, cheap, nothingServed()), state)
			assertGoesAhead(t, Apply(restingIn(within, state), position, gain, nothingServed()), state)
			held := Apply(restingIn(within, state), position, armed, nothingServed())
			assertWaits(t, held, position, SaleHeldRow(within.PositionPrice, -armedLoss, FeatureQuietDecline))

			for _, price := range []float64{gain, cheap, armingPrice(past)} {
				held = Apply(restingIn(past, state), position, price, nothingServed())
				assertWaits(t, held, position, SaleHeldAtDepthRow(past.PositionPrice, deepest+1, lastDepthFills, FeatureQuietDecline))
			}
		}
	}
}

// The action is asked on every tick a trigger stands, the dead zone's included: it does
// not wait for a depth arming, and what the ladder proposes does not matter. For a
// pending ladder at every print from far under its arming print to far over its break
// even, whatever the reading served beside it, with the trigger taking any loss or the
// shipped one, the depth bound off or on, a ladder in buy has its trailingBuy activated on a
// profit at or over the trigger, a gain included, and is held under it; a ladder in
// trailingBuy is sold unless the trigger holds it. The answer is stated from the loss worked out by hand.
func TestTheActionIsAskedOnEveryTickTheDeadZoneIncluded(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingTrade()
	breakEven := ladder.AverageEntryPrice(trade)
	filled, ceiling := ladder.CountFilledEntries(trade), ladder.ConfiguredDepths(trade)
	prices := []float64{breakEven / 2, armingPrice(trade), trade.PositionPrice, underTheBand, slowDeclineBand, breakEven, breakEven * 1.02, breakEven * 2}
	readings := []aggragates.AIIndicators{nothingServed(), legOnAndQuiet(), brokenReading(), slowDeclineBlock(false), slowDeclineBlock(true)}
	for _, bounds := range []struct {
		capPct     float64
		depthsLeft int
	}{{capPct: anyLoss}, {capPct: shippedLoss}, {depthsLeft: SaleIgnoreLastDepths, capPct: anyLoss}, {depthsLeft: SaleIgnoreLastDepths, capPct: shippedLoss}} {
		withAllowedLoss(t, bounds.capPct)
		withSaleIgnoreLastDepths(t, bounds.depthsLeft)
		pastTheDepthBound := bounds.depthsLeft > 0 && filled > ceiling-bounds.depthsLeft
		for _, price := range prices {
			loss := handLossPct(trade, price)
			overTheCap := loss > bounds.capPct
			for _, reading := range readings {
				for _, position := range addSideProposals {
					inBuy := Apply(restingIn(trade, "buy"), position, price, reading)
					inStopLoss := Apply(restingIn(trade, "trailingBuy"), position, price, reading)
					switch {
					case pastTheDepthBound:
						depth := SaleHeldAtDepthRow(slowDeclineLastFill, filled, ceiling, FeatureQuietDecline)
						assertWaits(t, inBuy, position, depth)
						assertWaits(t, inStopLoss, position, depth)
						continue
					case overTheCap:
						assertWaits(t, inBuy, position, SaleHeldRow(slowDeclineLastFill, -loss, FeatureQuietDecline))
						assertWaits(t, inStopLoss, position, SaleHeldRow(slowDeclineLastFill, -loss, FeatureQuietDecline))
						continue
					default:
						assertActivated(t, inBuy, ActivatedRow(slowDeclineLastFill, 0-loss, FeatureQuietDecline))
					}
					assertSoldAt(t, inStopLoss, reasonSlowDecline, trade, price)
				}
			}
		}
	}
}

// One flag covers both bounds of a pending episode: the action the cap held and
// the depth bound holds again writes no second row, and a new fill the gate confirms is
// a new episode, in which the row is due again and worded for the bound that holds
// the action then.
func TestOneHeldRowCoversBothBoundsOfAPendingEpisode(t *testing.T) {
	underTheShippedCap(t)
	trade := pendingTrade()
	armed, loss := armingOverTheCap(t, trade)
	filled := ladder.CountFilledEntries(trade)

	written, got := engineTick(trade, "trailingBuy", armed, testutil.At("18:00:00"), nothingServed())
	assertWaits(t, got, "trailingBuy", SaleHeldRow(slowDeclineLastFill, -loss, FeatureQuietDecline))
	pastTheBound(t, filled)
	for _, position := range addSideProposals {
		assertUntouched(t, Apply(written, position, armed, nothingServed()), position)
	}

	next := fills(lastDepthFills, "21:30:00")[filled].Price
	fifth := withFill(written, next, testutil.At("18:30:00"))
	pastTheBound(t, filled+1)
	_, got = engineTick(fifth, "trailingBuy", armingPrice(fifth), testutil.At("18:35:00"), legOnAndQuiet())
	assertWaits(t, got, "trailingBuy",
		ConfirmedRow(next, quietFinding(slowDeclineReasons)),
		SaleHeldAtDepthRow(next, filled+1, lastDepthFills, FeatureQuietDecline),
	)
}
