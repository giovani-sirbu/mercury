package smarttakeloss

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"github.com/giovani-sirbu/mercury/trades/profit"
)

// The edges of the loss cap the fixtures of lossCap_test.go leave to their
// defaults: the position price the ladder carries, the value of the cap in force,
// the rows one tick can hand back beside the held one and the features the held
// row names.

// Every row of the gate is filed at the newest fill, the held row included, and
// never at the position price: a re-anchor of the trailing stop loss or take
// profit moves it off the fill, and a state the ladder rests in may leave it at
// none, which the fold would skip as an event with no price (readEvent) and so
// write the held row afresh on every tick the sale stays held. For every state
// the ladder can rest in short of a close, and a position price over the fill,
// under it and none, the held row is at the fill, folds back as the sale held,
// and a later print at the band, under it and over it writes nothing; the
// trigger and the latch of a ladder not pending are at the fill too.
func TestTheHeldRowCarriesTheNewestFillNeverThePositionPrice(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	block := bandBlock(slowDeclineBand)
	for _, state := range []string{"buy", "stopLoss", "forceTrailingStopLoss", ""} {
		for _, moved := range []float64{slowDeclineLastFill + 2.5, slowDeclineLastFill - 3, 0} {
			name := fmt.Sprintf("resting in %q with the position price at %v", state, moved)
			trade := pendingTrade()
			trade.PositionType, trade.PositionPrice = state, moved

			got := Apply(trade, "", slowDeclineBand, block)
			assertNoSale(t, got, "")
			assertRows(t, got, SaleHeldRow(slowDeclineLastFill, handLossPct(trade, slowDeclineBand), FeatureQuietDecline))

			held := withRow(trade, got.Rows[0], testutil.At("21:45:00"))
			if st := rebuildState(held); !st.saleHeld || st.saleHeldRowDue() {
				t.Errorf("%s: the held row folds back as the sale held, and is not due again, got %+v", name, st)
			}
			for _, price := range []float64{slowDeclineBand - 0.5, slowDeclineBand, slowDeclineBand + 0.5} {
				assertUntouched(t, Apply(held, "", price, block), "")
			}

			plain := watchedTrade()
			plain.PositionType, plain.PositionPrice = state, moved
			assertRow(t, Apply(plain, "", underTheBand, slowDeclineBlock(true)), quietPending(slowDeclineLastFill))
			assertRow(t, Apply(plain, "", underTheBand, indecisionReading()), LatchedRow(slowDeclineLastFill, indecisionFinding(indecisionReasons)))
		}
	}
}

// exactBoundaryAt is exactBoundary at another cap: the price at which the exact
// ladder loses exactly pct of what it used and the price one step of its two
// decimals under it. Only the caps that put the boundary on that grid exactly
// give this fixture one — the whole-percent caps whose share of break even is a
// multiple of a cent — and it fails when the cap given is not one of them.
func exactBoundaryAt(t *testing.T, pct, band float64) (atTheCap, oneStepLower float64) {
	t.Helper()
	used, units := exactTotals()
	cents := math.Round((used - used*pct/100) / units * 100)
	atTheCap, oneStepLower = cents/100, (cents-1)/100
	loss, lowerLoss := used-float64(units*atTheCap), used-float64(units*oneStepLower)
	if loss*100 != pct*used || lowerLoss*100 <= pct*used || oneStepLower < band {
		t.Fatalf("fixture drifted: the exact ladder must lose exactly %v%% at %v and more at %v, over the band %v: got %v and %v of %v used", pct, atTheCap, oneStepLower, band, loss, lowerLoss, used)
	}
	return atTheCap, oneStepLower
}

// firstPriceWithinThe is firstPriceWithinTheCap at another cap: the lowest price
// on the two-decimal grid from the band up to break even at which a sale loses no
// more than pct percent of the used amount, and the price one step under it. It
// reports no boundary when the band is already within the cap.
func firstPriceWithinThe(trade aggragates.Trades, band, pct float64) (within, oneStepLower float64, found bool) {
	for cents := int(math.Round(band * 100)); float64(cents)/100 <= ladder.AverageEntryPrice(trade); cents++ {
		if handLossPct(trade, float64(cents)/100) <= pct {
			if float64(cents-1)/100 < band {
				return 0, 0, false
			}
			return float64(cents) / 100, float64(cents-1) / 100, true
		}
	}
	return 0, 0, false
}

// The cap is the value in force, not the constant it starts from: at every cap
// the boundary moves with it — the price that loses exactly the cap sells, one
// step lower is held — and the held row names the cap it was held under, with two
// decimals, beside the loss. A cap wider than the band's own loss never holds the
// sale, and one as wide as the whole used amount never holds a sale at any price.
func TestTheBoundaryAndTheRowFollowTheCapInForce(t *testing.T) {
	trade := pendingTrade()
	for _, pct := range []float64{0.5, 1, 2.5, 3} {
		withLossCap(t, pct)
		within, oneStepLower, found := firstPriceWithinThe(trade, slowDeclineBand, pct)
		if !found {
			t.Fatalf("fixture drifted: the w3s ladder has a boundary between its band and break even at a cap of %v%%", pct)
		}
		assertForced(t, Apply(trade, "", within, bandBlock(slowDeclineBand)), reasonSellBand)
		held := Apply(trade, "", oneStepLower, bandBlock(slowDeclineBand))
		assertNoSale(t, held, "")
		assertRows(t, held, SaleHeldRow(slowDeclineLastFill, handLossPct(trade, oneStepLower), FeatureQuietDecline))
		if want := fmt.Sprintf("over %.2f%%", pct); !strings.HasSuffix(held.Rows[0].Message, want) {
			t.Errorf("a cap of %v%%: the row must end %q, got %q", pct, want, held.Rows[0].Message)
		}
	}

	for _, pct := range []float64{5, 9.5, 20, 99} {
		withLossCap(t, pct)
		if _, _, found := firstPriceWithinThe(trade, slowDeclineBand, pct); found || handLossPct(trade, slowDeclineBand) > pct {
			t.Fatalf("fixture drifted: the band's own loss must be within a cap of %v%%", pct)
		}
		sold := Apply(trade, "", slowDeclineBand, bandBlock(slowDeclineBand))
		assertForced(t, sold, reasonSellBand)
		assertRows(t, sold)
	}

	withLossCap(t, 100)
	exact := exactLadder()
	for _, price := range []float64{exactBand, 70, 50} {
		sold := Apply(exact, "", price, bandBlock(50))
		assertForced(t, sold, reasonSellBand)
		assertRows(t, sold)
	}

	for _, pct := range []float64{2, 4, 6, 8, 10, 20} {
		withLossCap(t, pct)
		atTheCap, oneStepLower := exactBoundaryAt(t, pct, 70)
		assertForced(t, Apply(exact, "", atTheCap, bandBlock(70)), reasonSellBand)
		held := Apply(exact, "", oneStepLower, bandBlock(70))
		assertNoSale(t, held, "")
		assertRows(t, held, SaleHeldRow(exactPrices[len(exactPrices)-1], handLossPct(exact, oneStepLower), FeatureQuietDecline))
		if want := fmt.Sprintf("over %.2f%%", pct); !strings.HasSuffix(held.Rows[0].Message, want) {
			t.Errorf("a cap of %v%%: the row must end %q, got %q", pct, want, held.Rows[0].Message)
		}
	}
}

// roundPrices are the fills of the round ladder: whole units at whole prices whose
// break even is exactly 100, so the price a whole-percent cap names is a round
// number — 93 at 7%, 86 at 14%, 72 at 28% — and the loss there is exactly the cap
// times what the ladder used. Divided the other way round it is not: the loss over
// what was used is 0.07 at 7%, which is not a double, and 0.07 * 100 rounds over 7.
var roundPrices = []float64{102, 101, 99, 98}

// roundLadder is the long spot ladder of roundPrices pending from its newest fill.
func roundLadder() aggragates.Trades {
	trade := testutil.LadderTrade(false)
	for index, price := range roundPrices {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: exactUnits, Price: price, OrderId: int64(index + 1),
			CreatedAt: testutil.At(fmt.Sprintf("%02d:00:00", 9+index)),
		})
		trade.PositionPrice = price
	}
	return withRows(trade, quietPending(trade.PositionPrice))
}

// A loss exactly at the cap sells whatever the cap is, and the comparison is the
// one the loss and the cap are stated in: the loss times a hundred against the cap
// times what the ladder used, never the loss over what it used times a hundred,
// which at 7% reads exactly seven as a hair over it and holds the sale the cap
// lets go. One price step lower is held at each of them.
func TestALossExactlyAtAWholePercentCapSellsWhateverTheDivisionRoundsTo(t *testing.T) {
	trade := roundLadder()
	if breakEven := ladder.AverageEntryPrice(trade); breakEven != 100 || ladder.EntryNotional(trade) != 40000 {
		t.Fatalf("fixture drifted: the round ladder must break even at 100 on 40000 used, got %v on %v", breakEven, ladder.EntryNotional(trade))
	}
	for _, pct := range []float64{3, 7, 14, 28} {
		withLossCap(t, pct)
		const units = 4 * exactUnits
		atTheCap := 100 - pct
		oneStepLower := atTheCap - 0.01
		loss, lowerLoss := 40000-float64(units*atTheCap), 40000-float64(units*oneStepLower)
		if loss*100 != pct*40000 || lowerLoss*100 <= pct*40000 {
			t.Fatalf("fixture drifted: the round ladder must lose exactly %v%% at %v and more at %v, got %v and %v of 40000", pct, atTheCap, oneStepLower, loss, lowerLoss)
		}

		assertForced(t, Apply(trade, "", atTheCap, bandBlock(50)), reasonSellBand)
		held := Apply(trade, "", oneStepLower, bandBlock(50))
		assertNoSale(t, held, "")
		assertRows(t, held, SaleHeldRow(roundPrices[len(roundPrices)-1], handLossPct(trade, oneStepLower), FeatureQuietDecline))
	}
}

// A trade that has used nothing is never over the cap, whatever its close books:
// a ledger row at the accounting price adds no used amount, and fees that outweigh
// what the close sells for make it a loss all the same — so it is the guard on
// the used amount, and not the sign of the loss, that keeps a share of nothing out
// of the answer.
func TestATradeThatHasUsedNothingIsNeverOverTheCapWhateverItsFeesBook(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
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
	if loss, over := bandSaleLoss(ledger, 100); over || loss != 0 {
		t.Errorf("bandSaleLoss = %v, %v, want no loss and not over", loss, over)
	}
}

// A tick can hand back the gate's row and the held one together, the gate's
// first: a pending ladder the sophos indecision latches at a band the cap holds
// writes the latch and then the held sale, and a ladder the verdict makes pending
// on the same tick the indecision latches writes the one combined row and then
// the held sale. The tick after, with every reading still served, writes nothing:
// the latch is written once and the sale is held once, and the fold of the two
// rows reads back both.
func TestTheLatchAndTheHeldSaleOfOneTickAreTwoRowsInThatOrder(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	band := indecisionReading().SmartTakeLoss
	band.SlowDeclineSellBand = slowDeclineBand

	pending := pendingTrade()
	ticked, got := engineTick(pending, "", slowDeclineBand, testutil.At("21:45:00"), withBlock(band))
	assertNoSale(t, got, "")
	assertRows(t, got,
		LatchedRow(slowDeclineLastFill, indecisionFinding(indecisionReasons)),
		SaleHeldRow(slowDeclineLastFill, handLossPct(pending, slowDeclineBand), FeatureQuietDecline),
	)
	if st := rebuildState(ticked); !st.indecision || !st.saleHeld || !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill {
		t.Fatalf("the latch and the held sale fold back, the ladder still pending from its fill, got %+v", st)
	}
	assertUntouched(t, Apply(ticked, "", slowDeclineBand, withBlock(band)), "")
	if len(ticked.Logs) != len(pending.Logs)+2 || len(ticked.StrategyEvents) != len(pending.StrategyEvents)+2 {
		t.Fatalf("two rows and two events over the tick, got %d and %d over %d and %d", len(ticked.Logs), len(ticked.StrategyEvents), len(pending.Logs), len(pending.StrategyEvents))
	}

	both := withVerdict(band)
	both.SlowDeclineSellBand = slowDeclineBand
	plain := watchedTrade()
	ticked, got = engineTick(plain, "", slowDeclineBand, testutil.At("21:45:00"), withBlock(both))
	assertNoSale(t, got, "")
	assertRows(t, got,
		PendingRow(slowDeclineLastFill, indecisionFinding(indecisionReasons), quietFinding(slowDeclineReasons)),
		SaleHeldRow(slowDeclineLastFill, handLossPct(plain, slowDeclineBand), FeatureQuietDecline),
	)
	if st := rebuildState(ticked); !st.indecision || !st.saleHeld || !st.slowDeclinePending || len(st.pendingFeatures) != 1 || st.pendingFeatures[0] != FeatureQuietDecline {
		t.Fatalf("the trigger, the latch and the held sale fold back, the ladder pending on the quiet decline alone, got %+v", st)
	}
	assertUntouched(t, Apply(ticked, "", slowDeclineBand, withBlock(both)), "")
}

// The held row names the decline features the ladder is pending on, the latch of
// the indecision direction apart: the swing decline alone, both declines, and the
// swing decline of a ladder the events written before the merge hold pending. Its
// event carries them and reads back the same, so the sale is held once whichever
// feature made the ladder pending.
func TestTheHeldRowNamesTheFeaturesTheLadderIsPendingOn(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	const held = 186.5
	if loss := handLossPct(stairFourth(), held); loss <= SlowDeclineMaxLossPct {
		t.Fatalf("fixture drifted: a sale at %v must lose more than the cap holds, got %v%%", held, loss)
	}
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
		got := Apply(tc.trade, "", held, block)
		assertNoSale(t, got, "")
		assertRows(t, got, SaleHeldRow(stairPrices[3], handLossPct(tc.trade, held), tc.features...))
		if row := got.Rows[0]; row.Event != EventSaleHeld || !slices.Equal(row.Features, tc.features) {
			t.Errorf("%s: the held row carries %v, want %v", name, row.Features, tc.features)
		}

		stored := withRow(tc.trade, got.Rows[0], testutil.At("21:45:00"))
		st := rebuildState(stored)
		if !st.saleHeld || st.saleHeldRowDue() || !st.slowDeclinePending || !slices.Equal(st.pendingFeatures, tc.features) {
			t.Errorf("%s: the held row folds back as the sale held, the ladder still pending on %v, got %+v", name, tc.features, st)
		}
		assertUntouched(t, Apply(stored, "", held, block), "")
	}
}
