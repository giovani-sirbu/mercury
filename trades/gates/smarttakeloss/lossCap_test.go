package smarttakeloss

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The loss cap's tests stand on two ladders. The w3s ladder of the other tests
// (pendingTrade, pendingAtFive, lastDepthLadder) has the market's prices, so
// the cap's boundary on it is found by walking the price grid. The exact ladder
// is built of whole units and two-decimal prices, so its used amount, the value
// of its sale and the price the cap names are exact in floating point: the one
// fixture a loss can sit exactly at the cap on. The tests that read a sale run
// under the shipped cap (withLossCap), where the rest of the package runs under
// none (TestMain).

// exactUnits is the size of every fill of the exact ladder, exactBand the sell
// band it is served — under every price its tests sell at — and exactPrices its
// fills, each stamped an hour apart and the newest the one it is pending from.
// The last depth ladder is served the sell band lastDepthBand, and the ticks the
// capital protection tests read sit under (underTheCapitalBand) and over
// (overTheCapitalBand) capital protection's upper band, both over that sell
// band.
const (
	exactUnits          = 100.0
	exactBand           = 90.0
	lastDepthBand       = 170.0
	underTheCapitalBand = 171.0
	overTheCapitalBand  = 173.0
)

var exactPrices = []float64{100, 99, 98, 97}

// exactLadder is the long spot ladder of exactPrices carrying the pending pair
// the engine wrote for its newest fill: watched by the quiet decline, short of
// the last depth, its newest fill judged.
func exactLadder() aggragates.Trades {
	trade := testutil.LadderTrade(false)
	for index, price := range exactPrices {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: exactUnits, Price: price, OrderId: int64(index + 1),
			CreatedAt: testutil.At(fmt.Sprintf("%02d:00:00", 9+index)),
		})
		trade.PositionPrice = price
	}
	return withRows(trade, quietPending(trade.PositionPrice))
}

// exactTotals is what the exact ladder has used and the units it holds.
func exactTotals() (used, units float64) {
	for _, price := range exactPrices {
		used += exactUnits * price
		units += exactUnits
	}
	return used, units
}

// exactBoundary is the sale price at which the exact ladder loses exactly the
// cap, built from the constant, and the price one step of the pair's two
// decimals under it. It fails when the constant no longer gives this fixture a
// boundary it is exact on: the loss at the first must be the cap to the bit, the
// loss at the second more, and the second still at or over the band. The value
// of a sale is rounded as a value on its own (the conversion), the way the sale's
// row holds it, and never fused into the subtraction, which some architectures
// would otherwise do with one rounding for both.
func exactBoundary(t *testing.T) (atTheCap, oneStepLower float64) {
	t.Helper()
	used, units := exactTotals()
	cents := math.Round((used - used*SlowDeclineMaxLossPct/100) / units * 100)
	atTheCap, oneStepLower = cents/100, (cents-1)/100
	loss, lowerLoss := used-float64(units*atTheCap), used-float64(units*oneStepLower)
	if loss*100 != SlowDeclineMaxLossPct*used || lowerLoss*100 <= SlowDeclineMaxLossPct*used || oneStepLower < exactBand {
		t.Fatalf("fixture drifted: the exact ladder must lose exactly the cap at %v and more at %v, over the band %v: got %v and %v of %v used",
			atTheCap, oneStepLower, exactBand, loss, lowerLoss, used)
	}
	return atTheCap, oneStepLower
}

// handLossPct is the loss, as a share of what the ladder has used, of selling
// everything it holds at price, floored to the pair's two decimals, stated on
// its own from the history of a ladder with no fee and whole quantities: what
// the cap's figure is checked against, never read off it. A gain is negative.
func handLossPct(trade aggragates.Trades, price float64) float64 {
	var used, units float64
	for _, row := range trade.History {
		if row.Type == "BUY" && row.Quantity > 0 && row.Price > ladder.AccountingPriceCeiling {
			used += row.Quantity * row.Price
			units += row.Quantity
		}
	}
	return (used - units*helpers.ToFixed(price, 2)) / used * 100
}

// firstPriceWithinTheCap is the lowest price on the two-decimal grid, from the
// band up to the ladder's break even, at which selling it loses no more than the
// cap, and the price one step under it: the boundary of a ladder with no exact
// one.
func firstPriceWithinTheCap(t *testing.T, trade aggragates.Trades, band float64) (within, oneStepLower float64) {
	t.Helper()
	for cents := int(math.Round(band * 100)); float64(cents)/100 <= ladder.AverageEntryPrice(trade); cents++ {
		if handLossPct(trade, float64(cents)/100) <= SlowDeclineMaxLossPct {
			if float64(cents-1)/100 < band {
				t.Fatalf("fixture drifted: the band %v is already within the cap", band)
			}
			return float64(cents) / 100, float64(cents-1) / 100
		}
	}
	t.Fatal("fixture drifted: no price up to break even sells within the cap")
	return 0, 0
}

// bandBlock is the sell band alone, served beside nothing else: all a pending
// ladder needs to sell.
func bandBlock(band float64) aggragates.AIIndicators {
	return withBlock(aggragates.SmartTakeLossIndicators{SlowDeclineSellBand: band})
}

// assertRows fails unless Apply handed back exactly these rows, whole, in this
// order.
func assertRows(t *testing.T, got Result, want ...Row) {
	t.Helper()
	if len(got.Rows) != len(want) || (len(want) > 0 && !reflect.DeepEqual(got.Rows, want)) {
		t.Fatalf("rows = %+v, want exactly %+v", got.Rows, want)
	}
}

// The fixtures are what the tests say they are: both ladders pending from a
// judged newest fill, watched by the quiet decline, short of the last depth,
// and used as the cap reads it. The w3s band sits over the cap and under break
// even.
func TestTheLossCapFixtures(t *testing.T) {
	used, units := exactTotals()
	for name, trade := range map[string]aggragates.Trades{"exact": exactLadder(), "w3s": pendingTrade()} {
		st := rebuildState(trade)
		if !st.slowDeclinePending || !st.quietDeclineWatched || st.capitalProtectionWatched || slowDeclineFillUnjudged(trade, st) || st.saleHeld {
			t.Fatalf("fixture drifted: the %s ladder must be pending, watched and short of the last depth with its newest fill judged, got %+v", name, st)
		}
	}
	if ladder.EntryNotional(exactLadder()) != used || ladder.AverageEntryPrice(exactLadder()) != used/units {
		t.Fatal("fixture drifted: the exact ladder's used amount and break even must be the hand sums")
	}
	if loss := handLossPct(pendingTrade(), slowDeclineBand); !(SlowDeclineMaxLossPct < loss && slowDeclineBand < ladder.AverageEntryPrice(pendingTrade())) {
		t.Fatalf("fixture drifted: the w3s band must sit under break even at a loss over the cap, got %v", loss)
	}
}

// Over the cap the sale at the band is held: nothing is sold on that tick,
// whatever the ladder proposed comes back untouched, and the one row says so,
// at the newest fill, naming the loss. On the first price at which the loss is
// within the cap the sale goes as it always did, with no row, and one price step
// lower it is held again.
func TestTheSaleAtTheBandIsHeldOverTheCapAndSellsWithinIt(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	trade := pendingTrade()
	block := bandBlock(slowDeclineBand)

	held := handLossPct(trade, slowDeclineBand)
	for _, position := range []string{"", "buy", "stopLoss", "update_buy", "forceTrailingStopLoss"} {
		got := Apply(trade, position, slowDeclineBand, block)
		assertNoSale(t, got, position)
		assertRows(t, got, SaleHeldRow(slowDeclineLastFill, held, FeatureQuietDecline))
	}

	within, oneStepLower := firstPriceWithinTheCap(t, trade, slowDeclineBand)
	for _, position := range []string{"", "buy", "stopLoss"} {
		sold := Apply(trade, position, within, block)
		assertForced(t, sold, reasonSellBand)
		assertRows(t, sold)
		stillHeld := Apply(trade, position, oneStepLower, block)
		assertNoSale(t, stillHeld, position)
		assertRows(t, stillHeld, SaleHeldRow(slowDeclineLastFill, handLossPct(trade, oneStepLower), FeatureQuietDecline))
	}
}

// A loss exactly at the cap sells, and one price step lower is held. The loss is
// the close's own whatever state the trade rests in — the sale is simulated as
// the sellLoss chain would place it, at the tick price with no tolerance haircut
// — and the trade the caller holds is left as it was.
func TestALossExactlyAtTheCapSellsAndOneStepLowerIsHeld(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	atTheCap, oneStepLower := exactBoundary(t)
	block := bandBlock(exactBand)

	trade := exactLadder()
	newest := exactPrices[len(exactPrices)-1]
	for _, state := range []string{"buy", "stopLoss", "forceTrailingStopLoss", ""} {
		trade.PositionType = state
		before := trade
		for _, position := range []string{"", "buy", "stopLoss"} {
			sold := Apply(trade, position, atTheCap, block)
			assertForced(t, sold, reasonSellBand)
			assertRows(t, sold)

			held := Apply(trade, position, oneStepLower, block)
			assertNoSale(t, held, position)
			assertRows(t, held, SaleHeldRow(newest, handLossPct(trade, oneStepLower), FeatureQuietDecline))
		}
		if !reflect.DeepEqual(trade, before) || trade.PositionType != state {
			t.Fatalf("state %q: Apply wrote into the trade it was handed", state)
		}
	}
}

// A close that books zero or a gain is no loss and never held, from break even
// up.
func TestACloseBookingZeroOrAGainIsNeverHeld(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	exact := exactLadder()
	breakEven := ladder.AverageEntryPrice(exact)
	for _, price := range []float64{breakEven, breakEven + 0.5, breakEven * 1.1} {
		if got := Apply(exact, "", price, bandBlock(exactBand)); got.Reason != reasonSellBand || len(got.Rows) != 0 {
			t.Errorf("the exact ladder at %v books no loss and must sell with no row, got %+v", price, got)
		}
	}

	w3s := pendingTrade()
	for _, price := range []float64{ladder.AverageEntryPrice(w3s), ladder.AverageEntryPrice(w3s) + 3} {
		if got := Apply(w3s, "", price, bandBlock(slowDeclineBand)); got.Reason != reasonSellBand || len(got.Rows) != 0 {
			t.Errorf("the w3s ladder at %v books no loss and must sell with no row, got %+v", price, got)
		}
	}
}

// The cap at zero or below is off: the sale at the band goes as it did before
// the cap, at any loss, with no row.
func TestACapAtZeroOrBelowSellsAsBeforeWhateverTheLoss(t *testing.T) {
	trade := pendingTrade()
	if handLossPct(trade, slowDeclineBand) <= SlowDeclineMaxLossPct {
		t.Fatal("fixture drifted: the band must sit at a loss over the shipped cap")
	}
	for _, pct := range []float64{0, -1, -SlowDeclineMaxLossPct} {
		withLossCap(t, pct)
		for _, position := range []string{"", "buy", "stopLoss"} {
			got := Apply(trade, position, slowDeclineBand, bandBlock(slowDeclineBand))
			assertForced(t, got, reasonSellBand)
			assertRows(t, got)
		}
	}
}

// A trade that has used nothing is never over the cap: with no entry fill, or
// with bookkeeping rows only, there is no amount to take a share of.
func TestATradeThatHasUsedNothingIsNeverOverTheCap(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	ledger := testutil.LadderTrade(false)
	ledger.History = []aggragates.TradesHistory{{Type: "BUY", Quantity: 12.5, Price: ladder.AccountingPriceCeiling, OrderId: 1}}
	for name, trade := range map[string]aggragates.Trades{"no fill": testutil.LadderTrade(false), "a ledger row only": ledger} {
		if loss, over := bandSaleLoss(trade, 100); over || loss != 0 {
			t.Errorf("%s: bandSaleLoss = %v, %v, want no loss and not over", name, loss, over)
		}
	}
}

// The row of a held sale is written once per pending episode. The confirming
// tick writes the confirmation and then the held row, at the new fill; every tick
// after it, at the band or under it and back, writes nothing; a new fill the gate
// confirms makes it due again at that fill.
func TestTheHeldRowIsWrittenOncePerPendingEpisodeAndAgainAfterAConfirmation(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	w3s := fills(8, "21:30:00")

	trade, got := engineTick(pendingAtFive(), "", judgeBand, judgeTick, legOnAndQuiet())
	assertNoSale(t, got, "")
	assertRows(t, got, sixthConfirmed(), SaleHeldRow(sixthFill, handLossPct(trade, judgeBand), FeatureQuietDecline))
	if st := rebuildState(trade); !st.saleHeld || st.saleHeldRowDue() || !st.slowDeclinePending || st.slowDeclinePendingFrom != sixthFill {
		t.Fatalf("the held row folds back as the sale held in the episode, the ladder still pending from the sixth fill, got %+v", st)
	}

	for _, price := range []float64{judgeBand, underJudgeBand, judgeBand, underJudgeBand - 1, judgeBand} {
		for _, reading := range []aggragates.AIIndicators{judgeBandAlone(), legOnAndQuiet(), brokenReading()} {
			assertUntouched(t, Apply(trade, "", price, reading), "")
			assertUntouched(t, Apply(trade, "stopLoss", price, reading), "stopLoss")
		}
	}

	trade = withFill(trade, w3s[6].Price, testutil.At("23:30:00"))
	trade, got = engineTick(trade, "", judgeBand, testutil.At("23:40:00"), legOnAndQuiet())
	assertNoSale(t, got, "")
	assertRows(t, got, ConfirmedRow(w3s[6].Price, quietFinding(slowDeclineReasons)), SaleHeldRow(w3s[6].Price, handLossPct(trade, judgeBand), FeatureQuietDecline))
	assertUntouched(t, Apply(trade, "", judgeBand, judgeBandAlone()), "")

	_, wantEvents := expectedRows(
		quietPending(pendingFromFifth),
		sixthConfirmed(), SaleHeldRow(sixthFill, 0, FeatureQuietDecline),
		ConfirmedRow(w3s[6].Price, quietFinding(slowDeclineReasons)), SaleHeldRow(w3s[6].Price, 0, FeatureQuietDecline),
	)
	if listed := writtenEvents(t, trade); !reflect.DeepEqual(listed, wantEvents) {
		t.Fatalf("events %v, want one held event per confirmation, %v", listed, wantEvents)
	}
}

// A fill that takes the ladder to its last depth is never judged, so no event
// confirms it and the ladder stays pending from where it was: the episode goes
// on, and a sale the loss cap already held in it is not held again with a row of
// its own, at that fill or on any tick after it. A sale not held before that
// fill is held once, on the first tick over the band after it, and not again.
func TestTheFillThatTakesTheLadderToItsLastDepthWritesNoSecondHeldRow(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	w3s := fills(8, "21:30:00")
	seventh, eighth := w3s[6].Price, w3s[7].Price
	price, block := slowDeclineBand-10, bandBlock(slowDeclineBand-12)
	prints := []float64{price, price - 1, price}

	pending := withRows(testutil.LadderTrade(false, fills(7, "20:00:00")...), quietPending(seventh))
	held := withRows(pending, SaleHeldRow(seventh, 3, FeatureQuietDecline))
	if st := rebuildState(held); !st.saleHeld || st.saleHeldRowDue() {
		t.Fatalf("fixture drifted: the sale is held in the episode and its row is not due, got %+v", st)
	}

	atLastDepth := withFill(held, eighth, testutil.At("21:30:00"))
	if ladder.CountFilledEntries(atLastDepth) != ladder.ConfiguredDepths(atLastDepth) {
		t.Fatal("fixture drifted: the eighth fill must take the ladder to its last depth")
	}
	if st := rebuildState(atLastDepth); !st.slowDeclinePending || st.slowDeclinePendingFrom != seventh || !st.saleHeld || st.saleHeldRowDue() {
		t.Fatalf("the last depth's fill leaves the ladder pending from where it was with its sale still held and no row due, got %+v", st)
	}
	for _, at := range prints {
		assertUntouched(t, Apply(atLastDepth, "", at, block), "")
	}

	// The row's price is its own and no fold reads it, so the comparison is of its
	// filing, its features and its message, which names the loss.
	notHeld := withFill(pending, eighth, testutil.At("21:30:00"))
	first := Apply(notHeld, "", price, block)
	assertNoSale(t, first, "")
	if len(first.Rows) != 1 || first.Rows[0].Gate != GateSlowDecline || first.Rows[0].Event != EventSaleHeld || !reflect.DeepEqual(first.Rows[0].Features, []string{FeatureQuietDecline}) ||
		first.Rows[0].Message != SaleHeldRow(0, handLossPct(notHeld, price), FeatureQuietDecline).Message {
		t.Fatalf("a sale not held before the last depth's fill is held on the first tick over the band after it, with one row naming the loss, got %+v", first.Rows)
	}
	written := withRow(notHeld, first.Rows[0], testutil.At("21:45:00"))
	for _, at := range prints {
		assertUntouched(t, Apply(written, "", at, block), "")
	}
}

// A cancelled ladder is not pending, so a held sale writes no row after it,
// at any price or reading; marked pending again by the verdict it is a new
// episode, and the sale is held again with a row of its own.
func TestNoHeldRowAfterACancelAndADueRowWhenTheLadderGoesPendingAgain(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	w3s := fills(8, "21:30:00")
	seventh := w3s[6].Price

	trade, _ := engineTick(pendingAtFive(), "", judgeBand, judgeTick, legOnAndQuiet())
	trade = withFill(trade, seventh, testutil.At("23:30:00"))
	trade, got := engineTick(trade, "", judgeBand, testutil.At("23:40:00"), brokenReading())
	assertRows(t, got, CancelledRow(seventh, quietFinding(slowDeclineBreakReasons)))
	assertNoSale(t, got, "")
	if st := rebuildState(trade); st.slowDeclinePending || st.saleHeld {
		t.Fatalf("a cancelled ladder is not pending and holds no sale, got %+v", st)
	}

	for _, price := range []float64{underJudgeBand, judgeBand, overJudgeBand} {
		for _, reading := range []aggragates.AIIndicators{brokenReading(), judgeBandAlone(), legOnAndQuiet()} {
			assertUntouched(t, Apply(trade, "", price, reading), "")
		}
	}

	trade, got = engineTick(trade, "", judgeBand, testutil.At("23:50:00"), verdictReading())
	assertNoSale(t, got, "")
	assertRows(t, got, quietPending(seventh), SaleHeldRow(seventh, handLossPct(trade, judgeBand), FeatureQuietDecline))
}

// A pending ladder a depth priority holds is paused: it is reset with its row
// and nothing is sold or held at the band, then or after. The next fill ends
// the hold, and once the verdict marks the ladder pending the held sale is a new
// episode with a row of its own.
func TestNoHeldRowUnderADepthPriorityHoldAndADueRowOnceItEnds(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	held, _ := engineTick(pendingAtFive(), "", judgeBand, judgeTick, legOnAndQuiet())
	held = heldBy(held, testutil.At("23:30:00"))

	trade, got := engineTick(held, "", judgeBand, testutil.At("23:35:00"), judgeBandAlone())
	assertRows(t, got, ResetRow(sixthFill, quietFinding(nil)))
	assertNoSale(t, got, "")
	for _, price := range []float64{underJudgeBand, judgeBand, overJudgeBand} {
		assertUntouched(t, Apply(trade, "", price, judgeBandAlone()), "")
	}

	w3s := fills(8, "21:30:00")
	trade = withFill(trade, w3s[6].Price, testutil.At("23:45:00"))
	trade, got = engineTick(trade, "", judgeBand, testutil.At("23:50:00"), verdictReading())
	assertNoSale(t, got, "")
	assertRows(t, got, quietPending(w3s[6].Price), SaleHeldRow(w3s[6].Price, handLossPct(trade, judgeBand), FeatureQuietDecline))
}

// The ladder's own closes pass the overlay untouched, so a held sale writes no
// row on a proposal or a state that already is a close: only the add side reaches
// the band.
func TestNoHeldRowBesideAClosedPositionOrANonPendingLadder(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	trade := pendingTrade()
	for position := range gridCloses {
		assertUntouched(t, Apply(trade, position, slowDeclineBand, bandBlock(slowDeclineBand)), position)
	}
	resting := trade
	resting.PositionType = "takeProfit"
	assertUntouched(t, Apply(resting, "", slowDeclineBand, bandBlock(slowDeclineBand)), "")

	notPending := watchedTrade()
	assertUntouched(t, Apply(notPending, "", slowDeclineBand, bandBlock(slowDeclineBand)), "")
}

// Capital protection's sale is not capped and its band is read while the band
// sale is held: a ladder at its last depth with both bands reached sells under
// capital protection's own reason, and the held row says why the band did not
// sell first. Within the cap the band names the sale as it always did, and with
// capital protection off or not reached the held sale sells nothing.
func TestCapitalProtectionStillSellsWhileTheBandSaleIsHeld(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	withCapitalProtectionExit(t, true)
	last := lastDepthLadder()
	trade := withRows(last, quietPending(last.PositionPrice))
	if !rebuildState(trade).capitalProtectionWatched {
		t.Fatal("fixture drifted: the ladder must be at its last depth")
	}
	both := solBlock()
	both.SlowDeclineSellBand = lastDepthBand
	if !(lastDepthBand < underTheCapitalBand && underTheCapitalBand < capitalProtectionBand && capitalProtectionBand < overTheCapitalBand) ||
		handLossPct(trade, overTheCapitalBand) <= SlowDeclineMaxLossPct {
		t.Fatal("fixture drifted: the band, the tick under and the tick over capital protection's band must rise in that order, the dearest at a loss over the cap")
	}
	held := func(price float64) Row {
		return SaleHeldRow(last.PositionPrice, handLossPct(trade, price), FeatureQuietDecline)
	}

	for _, position := range []string{"", "buy", "stopLoss"} {
		got := Apply(trade, position, overTheCapitalBand, withBlock(both))
		assertForced(t, got, reasonCapitalProtection)
		assertRows(t, got, held(overTheCapitalBand))
	}

	got := Apply(trade, "", underTheCapitalBand, withBlock(both))
	assertNoSale(t, got, "")
	assertRows(t, got, held(underTheCapitalBand))

	notBearish := both
	notBearish.CapitalProtectionSmcBearish = false
	got = Apply(trade, "", overTheCapitalBand, withBlock(notBearish))
	assertNoSale(t, got, "")
	assertRows(t, got, held(overTheCapitalBand))

	within, _ := firstPriceWithinTheCap(t, trade, lastDepthBand)
	if within <= capitalProtectionBand {
		t.Fatal("fixture drifted: the first price within the cap must be over capital protection's band")
	}
	sold := Apply(trade, "", within, withBlock(both))
	assertForced(t, sold, reasonSellBand)
	assertRows(t, sold)

	withCapitalProtectionExit(t, false)
	got = Apply(trade, "", overTheCapitalBand, withBlock(both))
	assertNoSale(t, got, "")
	assertRows(t, got, held(overTheCapitalBand))
}

// ExitReached is Apply's sale on the empty proposal, so it inherits the cap: a
// held sale is no exit — false while the loss is over the cap, true on the first
// price within it — and capital protection's band still makes it true while the
// band sale is held. It writes nothing, and Armed, which asks only what is
// watched, does not move with the cap.
func TestExitReachedInheritsTheCapAndArmedDoesNot(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	trade := pendingTrade()
	rows, events := len(trade.Logs), len(trade.StrategyEvents)
	block := bandBlock(slowDeclineBand).SmartTakeLoss
	within, oneStepLower := firstPriceWithinTheCap(t, trade, slowDeclineBand)

	for price, want := range map[float64]bool{slowDeclineBand: false, oneStepLower: false, within: true, within + 1: true} {
		if got := ExitReached(trade, price, block); got != want {
			t.Errorf("ExitReached at %v = %v, want %v", price, got, want)
		}
	}
	if len(trade.Logs) != rows || len(trade.StrategyEvents) != events {
		t.Fatal("ExitReached must write nothing, the held row included")
	}

	exact := exactLadder()
	atTheCap, lower := exactBoundary(t)
	if !ExitReached(exact, atTheCap, bandBlock(exactBand).SmartTakeLoss) || ExitReached(exact, lower, bandBlock(exactBand).SmartTakeLoss) {
		t.Fatal("ExitReached must be true exactly at the cap and false one price step lower")
	}

	withCapitalProtectionExit(t, true)
	last := lastDepthLadder()
	capital := withRows(last, quietPending(last.PositionPrice))
	both := solBlock()
	both.SlowDeclineSellBand = lastDepthBand
	if !ExitReached(capital, overTheCapitalBand, both) {
		t.Fatal("capital protection's band sells while the band sale is held")
	}
	both.CapitalProtectionSmcBearish = false
	if ExitReached(capital, overTheCapitalBand, both) {
		t.Fatal("a held sale with nothing else reaching is no exit")
	}

	armedOn := Armed(trade)
	withLossCap(t, 0)
	if Armed(trade) != armedOn || !armedOn {
		t.Fatal("Armed asks what the gate watches, and the cap is no part of it")
	}
}

// The loss counts the commissions the close is charged, as acceptLoss books
// them: a quote fee on every fill is paid for the opening leg and again for the
// closing one, so the exact ladder needs a higher price to be within the cap and
// the old boundary is now held.
func TestTheLossCountsTheFeesTheCloseIsCharged(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	atTheCap, _ := exactBoundary(t)
	const feePerFill = 1.0
	_, units := exactTotals()
	extra := 2 * feePerFill * float64(len(exactPrices)) / units

	trade := exactLadder()
	for index := range trade.History {
		trade.History[index].Fees = []aggragates.TradesFees{{Asset: "USDT", Fee: feePerFill}}
	}
	block := bandBlock(exactBand)

	held := Apply(trade, "", atTheCap, block)
	assertNoSale(t, held, "")
	if len(held.Rows) != 1 || held.Rows[0].Event != EventSaleHeld {
		t.Fatalf("with fees the price that was exactly at the cap is over it, got %+v", held.Rows)
	}
	shiftedCents := math.Round((atTheCap + extra) * 100)
	assertForced(t, Apply(trade, "", shiftedCents/100, block), reasonSellBand)
	assertNoSale(t, Apply(trade, "", (shiftedCents-1)/100, block), "")
}

// capturedStdout is what f writes to the process's standard output, which is
// where the log package writes every line: the way to see a line a test never
// meant to be written.
func capturedStdout(t *testing.T, f func()) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = file
	defer func() { os.Stdout = previous }()
	f()
	os.Stdout = previous
	written, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(written)
}

// A fee paid in a third asset cannot be priced here — no wallet snapshot and no
// exchange reach Apply — and the cap leaves it out, as a backtest, which has none,
// reads it: the boundary is the fee-free one, and nothing is looked up, so no
// price lookup fails and logs a line on every print the sale is held. The history
// the caller holds keeps its fees.
func TestAThirdAssetFeeIsLeftOutOfTheLossAndNeverPriced(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	atTheCap, oneStepLower := exactBoundary(t)

	trade := exactLadder()
	for index := range trade.History {
		trade.History[index].Fees = []aggragates.TradesFees{{Asset: "BNB", Fee: 0.5}, {Asset: "USDT", Fee: 0}}
	}
	before := make([]aggragates.TradesHistory, len(trade.History))
	for index, row := range trade.History {
		before[index] = row
		before[index].Fees = append([]aggragates.TradesFees(nil), row.Fees...)
	}

	block := bandBlock(exactBand)
	var sold, held Result
	logged := capturedStdout(t, func() {
		sold = Apply(trade, "", atTheCap, block)
		held = Apply(trade, "", oneStepLower, block)
	})
	assertForced(t, sold, reasonSellBand)
	assertNoSale(t, held, "")
	if len(held.Rows) != 1 || held.Rows[0].Event != EventSaleHeld {
		t.Fatalf("one price step under the boundary is held, got %+v", held.Rows)
	}
	if strings.Contains(logged, "getSymbolPrice") {
		t.Fatalf("a third-asset fee was looked up and the lookup logged: %s", logged)
	}
	if !reflect.DeepEqual(trade.History, before) {
		t.Fatalf("the caller's history lost or changed its fees: %+v, want %+v", trade.History, before)
	}
}

// The dust the close keeps is credited as the close books it: a ladder whose
// quantity the lot-size floor cuts a remainder from — a pair that trades whole
// units — loses what a sale of everything it holds loses, so the price that is
// exactly at the cap sells even though the units actually sold are fewer.
func TestTheLossCountsTheDustTheCloseKeeps(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	trade := testutil.LadderTrade(false)
	for index, quantity := range []float64{50.25, 50.25} {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: quantity, Price: 100, OrderId: int64(index + 1), CreatedAt: testutil.At(fmt.Sprintf("%02d:00:00", 9+index)),
		})
	}
	trade.PositionPrice = 100
	trade = withRows(trade, quietPending(100))
	used, units := 100.5*100, 100.5

	cents := math.Round((used - used*SlowDeclineMaxLossPct/100) / units * 100)
	atTheCap, oneStepLower := cents/100, (cents-1)/100
	if (used-float64(units*atTheCap))*100 != SlowDeclineMaxLossPct*used || ladder.CountFilledEntries(trade) < QuietDeclineArmDepth {
		t.Fatalf("fixture drifted: the dust ladder must lose exactly the cap at %v and be watched", atTheCap)
	}

	block := bandBlock(exactBand)
	assertForced(t, Apply(trade, "", atTheCap, block), reasonSellBand)
	held := Apply(trade, "", oneStepLower, block)
	assertNoSale(t, held, "")
	if len(held.Rows) != 1 || held.Rows[0].Event != EventSaleHeld {
		t.Fatalf("one price step under the boundary is held, got %+v", held.Rows)
	}
}

// The state a tick leaves forgets the held sale the way the fold of the events
// that tick writes does: a ladder the gate makes pending on a decline — a trigger
// or a confirmation — and one it makes not pending — a cancellation or a reset —
// holds no sale from then on, so the row is due again on that very tick.
func TestTheStateATickLeavesForgetsTheHeldSaleLikeTheFold(t *testing.T) {
	held := state{
		fills:                  []entryFill{{Price: 100}, {Price: 99}},
		slowDeclinePending:     true,
		slowDeclinePendingFrom: 100,
		saleHeld:               true,
		pendingFeatures:        []string{FeatureQuietDecline},
	}
	if got := held.pendingOn([]Finding{quietFinding(nil)}); got.saleHeld || !got.slowDeclinePending || got.slowDeclinePendingFrom != 99 || !got.saleHeldRowDue() {
		t.Errorf("a ladder made pending again holds no sale and is due its row, got %+v", got)
	}
	if got := held.notPending(); got.saleHeld || got.slowDeclinePending {
		t.Errorf("a ladder made not pending holds no sale, got %+v", got)
	}
}

// Each held row is filed under the slow decline gate as the sale held kind at
// the newest fill, naming the decline features the ladder is pending on and no
// reasons, and what Rows writes of it is the document of that kind, that fill and
// those features.
func TestTheHeldRowIsFiledUnderItsGateAndKindAndReadsBack(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	trade := pendingTrade()
	got := Apply(trade, "", slowDeclineBand, bandBlock(slowDeclineBand))
	row := firstRow(got)
	if row == nil {
		t.Fatalf("no single row: %+v", got.Rows)
	}
	if row.Gate != "slowDecline" || row.Event != "saleHeld" || row.Price != slowDeclineLastFill || !reflect.DeepEqual(row.Features, []string{"quietDecline"}) || row.Reasons != nil {
		t.Fatalf("the held row is filed %q/%q at %v with %v and %v, want slowDecline/saleHeld at the newest fill on the quiet decline and no reasons", row.Gate, row.Event, row.Price, row.Features, row.Reasons)
	}

	logRow, event := Rows(trade, *row, testutil.At("21:45:00"))
	if logRow.Message != row.Message || logRow.Price != slowDeclineLastFill || event.Gate != GateSlowDecline || event.Kind() != EventSaleHeld {
		t.Fatalf("the pair is the row and the sale held event, got %+v and %+v", logRow, event)
	}
	if string(event.Data) != `{"event":"saleHeld","price":184.45,"features":["quietDecline"]}` {
		t.Fatalf("the document is the kind, the fill and the features, got %s", event.Data)
	}
}

// The fold remembers that the episode's sale was held, whatever fill the row
// carries, and forgets it on the events that clear it — a new pending event, a
// cancelled, a reset, a sold one — while the ladder stays pending through the
// held sale itself. A held event at another price than the fill the ladder is
// pending from, as the last depth's fill leaves one, holds the sale all the same.
// A held event that names no features, has no price, belongs to another gate or
// lands on a ladder nothing watches is no sale held; one on a ladder pending by
// the events the gate wrote before the merge is read all the same, those legacy
// states left as they are.
func TestTheFoldRemembersTheEpisodesSaleWasHeldWhateverFillItCarries(t *testing.T) {
	trade := watchedTrade()
	fill := trade.PositionPrice
	pending := quietPending(fill)
	held := SaleHeldRow(fill, 3, FeatureQuietDecline)
	sold := ExitRow(withRows(trade, pending), reasonSellBand, 186)
	document := func(price float64) string {
		return fmt.Sprintf(`{"event":"saleHeld","price":%v,"features":["quietDecline"]}`, price)
	}

	cases := []struct {
		name          string
		trade         aggragates.Trades
		pending       bool
		held          bool
		switchedOff   bool
		wantPendingOn []string
	}{
		{"pending, then held", withRows(trade, pending, held), true, true, false, []string{FeatureQuietDecline}},
		{"held, then sold", withRows(trade, pending, held, sold), true, false, false, []string{FeatureQuietDecline}},
		{"held, then cancelled", withRows(trade, pending, held, CancelledRow(fill, quietFinding(nil))), false, false, false, nil},
		{"held, then reset", withRows(trade, pending, held, ResetRow(fill, quietFinding(nil))), false, false, false, nil},
		{"held, then pending again at the same fill", withRows(trade, pending, held, ConfirmedRow(fill, quietFinding(nil))), true, false, false, []string{FeatureQuietDecline}},
		{"held, cancelled, pending again", withRows(trade, pending, held, CancelledRow(fill, quietFinding(nil)), pending), true, false, false, []string{FeatureQuietDecline}},
		{"held twice", withRows(trade, pending, held, held), true, true, false, []string{FeatureQuietDecline}},
		{"held at a lower price than the fill the ladder is pending from", withRows(trade, pending, SaleHeldRow(fill-1, 3, FeatureQuietDecline)), true, true, false, []string{FeatureQuietDecline}},
		{"held at a higher price than the fill the ladder is pending from", withRows(trade, pending, SaleHeldRow(fill+1, 3, FeatureQuietDecline)), true, true, false, []string{FeatureQuietDecline}},
		{"held with no features", withEvents(withRows(trade, pending), eventOf(GateSlowDecline, `{"event":"saleHeld","price":184.45}`)), true, false, false, []string{FeatureQuietDecline}},
		{"held at no price", withEvents(withRows(trade, pending), eventOf(GateSlowDecline, document(0))), true, false, false, []string{FeatureQuietDecline}},
		{"held under another gate", withEvents(withRows(trade, pending), eventOf(GateCapitalProtection, document(fill))), true, false, false, []string{FeatureQuietDecline}},
		{"held on the legacy quiet decline state", withEvents(trade, eventOf(GateSlowDecline, legacyDocument(EventPending, fill)), eventOf(GateSlowDecline, document(fill))), true, true, false, []string{FeatureQuietDecline}},
		{"held where nothing watches", withRows(trade, pending, held), false, false, true, nil},
	}
	for _, c := range cases {
		if c.switchedOff {
			withQuietDecline(t, false)
			withSwingDecline(t, false)
			withIndecision(t, false)
		}
		st := rebuildState(c.trade)
		if st.slowDeclinePending != c.pending || st.saleHeld != c.held || !reflect.DeepEqual(st.pendingFeatures, c.wantPendingOn) {
			t.Errorf("%s: pending %v on %v, sale held %v, want pending %v on %v, sale held %v", c.name, st.slowDeclinePending, st.pendingFeatures, st.saleHeld, c.pending, c.wantPendingOn, c.held)
		}
		if c.pending && st.saleHeldRowDue() == c.held {
			t.Errorf("%s: the held row is due %v with the sale held %v", c.name, st.saleHeldRowDue(), st.saleHeld)
		}
	}
}
