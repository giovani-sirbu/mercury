package smartsell

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The loss cap's tests stand on two ladders: the w3s ladder of the other tests, whose cap boundary
// is found by walking the price grid, and the exact ladder of whole units at whole prices
// (roundPrices), where a loss sits exactly at the cap in floating point. Every test reads a ladder
// in both acting states on every proposal under the shipped cap (underTheShippedCap).

// exactUnits is the size of every fill of the exact ladder; underTheCapitalBand and overTheCapitalBand
// sit either side of capital protection's upper band.
const (
	exactUnits          = 100.0
	underTheCapitalBand = 171.0
	overTheCapitalBand  = 173.0
)

// tickPrice is the print of the tests that read the gate's own row alone, which comes first on the tick.
const tickPrice = 185.0

// exactLadder is the long spot ladder of roundPrices, exactUnits each, pending from its judged newest
// fill, watched by the quiet decline, short of the last depth, resting in buy.
func exactLadder() aggragates.Trades {
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

// exactTotals is what the exact ladder has used and the units it holds.
func exactTotals() (used, units float64) {
	for _, price := range roundPrices {
		used += exactUnits * price
		units += exactUnits
	}
	return used, units
}

// capBoundary is the sale price at which a ladder that used used and holds units loses exactly pct
// percent, and the price one cent under it; exact says whether the cap names such a price on the grid.
// The sale's value is rounded on its own, never fused into the subtraction.
func capBoundary(used, units, pct float64) (atTheCap, oneStepLower float64, exact bool) {
	cents := math.Round((used - used*pct/100) / units * 100)
	atTheCap, oneStepLower = cents/100, (cents-1)/100
	loss, lowerLoss := used-float64(units*atTheCap), used-float64(units*oneStepLower)
	exact = pct > 0 && oneStepLower > 0 && loss*100 == pct*used && lowerLoss*100 > pct*used
	return atTheCap, oneStepLower, exact
}

// exactBoundaryAt is capBoundary of the exact ladder at another cap.
func exactBoundaryAt(pct float64) (atTheCap, oneStepLower float64, exact bool) {
	used, units := exactTotals()
	return capBoundary(used, units, pct)
}

// exactBoundary is exactBoundaryAt at the shipped cap. It skips the test when the cap
// is off or names no price on the grid the exact ladder loses exactly that much at.
func exactBoundary(t *testing.T) (atTheCap, oneStepLower float64) {
	t.Helper()
	atTheCap, oneStepLower, exact := exactBoundaryAt(float64(shippedLoss))
	if !exact {
		t.Skip("SaleTriggerPercentage takes no loss or names no price above zero on the cent grid the exact ladder loses exactly that much at")
	}
	return atTheCap, oneStepLower
}

// handLossPct is the loss, as a share of what the ladder used, of selling everything at price on the
// cent grid, stated from a fee-free history; a gain is negative.
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

// armingPrice is the price the engines' buy row arms the next depth at: the move against the newest
// fill is the row's percentage plus its tolerance, a loss over any cap under that.
func armingPrice(trade aggragates.Trades) float64 {
	row := trade.StrategyPair.StrategySettings[0]
	return helpers.ToFixed(trade.PositionPrice/(1+(row.Percentage+row.Tolerance)/100), 2)
}

// armingOverTheCap is armingPrice with its loss; it skips the test when the trigger takes no loss or as much as that.
func armingOverTheCap(t *testing.T, trade aggragates.Trades) (price, lossPct float64) {
	t.Helper()
	price = armingPrice(trade)
	lossPct = handLossPct(trade, price)
	if shippedLoss <= 0 || lossPct <= shippedLoss {
		t.Skip("SaleTriggerPercentage takes no loss or wide enough to let the arming print be acted on")
	}
	return price, lossPct
}

// firstPriceWithinTheCap is the lowest grid price from floor up to break even within the cap, and the
// cent under it; it skips the test when the trigger takes no loss or the floor is already within it.
func firstPriceWithinTheCap(t *testing.T, trade aggragates.Trades, floor float64) (within, oneStepLower float64) {
	t.Helper()
	if shippedLoss <= 0 {
		t.Skip("SaleTriggerPercentage takes no loss")
	}
	for cents := int(math.Round(floor * 100)); float64(cents)/100 <= ladder.AverageEntryPrice(trade); cents++ {
		if handLossPct(trade, float64(cents)/100) <= shippedLoss {
			if float64(cents-1)/100 < floor {
				t.Skip("SaleTriggerPercentage is wide enough to let the floor price be acted on")
			}
			return float64(cents) / 100, float64(cents-1) / 100
		}
	}
	t.Fatal("fixture drifted: no price up to break even is within the cap")
	return 0, 0
}

// underTheShippedCap runs the test with the quiet decline on and the cap at SaleTriggerPercentage, the depth bound off.
func underTheShippedCap(t *testing.T) {
	t.Helper()
	withQuietDecline(t, true)
	withAllowedLoss(t, shippedLoss)
}

// pastTheBound runs the test with the depth bound one more than the w3s row's ceiling leaves, and answers it.
func pastTheBound(t *testing.T, filled int) int {
	t.Helper()
	left := lastDepthFills - filled + 1
	withSaleIgnoreLastDepths(t, left)
	return left
}

// confirmationsOverTheCap skips the test unless the cap holds the action at the natural arming print
// of the sixth and seventh fills of pendingAtFive.
func confirmationsOverTheCap(t *testing.T) {
	t.Helper()
	sixth := pendingAtFive()
	armingOverTheCap(t, sixth)
	armingOverTheCap(t, withFill(sixth, fills(lastDepthFills, "21:30:00")[6].Price, testutil.At("23:30:00")))
}

// nothingServed is the reading that serves nothing: a pending ladder whose newest
// fill is judged is acted on without one.
func nothingServed() aggragates.AIIndicators {
	return withBlock(withEveryRowTrend(aggragates.SmartSellIndicators{}))
}

// assertRows fails unless Apply handed back exactly these rows, whole, in this
// order.
func assertRows(t *testing.T, got Result, want ...Row) {
	t.Helper()
	if len(got.Rows) != len(want) || (len(want) > 0 && !reflect.DeepEqual(got.Rows, want)) {
		t.Fatalf("rows = %+v, want exactly %+v", got.Rows, want)
	}
}

// assertGoesAhead fails unless Apply let the action through: activated in buy, sold in trailingBuy; the rows are not read.
func assertGoesAhead(t *testing.T, got Result, state string) {
	t.Helper()
	if state == "buy" {
		if got.Position != "trailingBuy" || !got.Activated || got.Reason != "" {
			t.Fatalf("a ladder in buy must have its trailingBuy activated, got %+v", got)
		}
		return
	}
	assertSold(t, got, reasonSlowDecline)
}

// The fixtures are what the tests say: both ladders pending from a judged newest fill, watched, short
// of the last depth, resting in buy, and a step under the newest fill each loses more than the cap.
func TestTheLossCapFixtures(t *testing.T) {
	withQuietDecline(t, true)
	used, units := exactTotals()
	for name, trade := range map[string]aggragates.Trades{"exact": exactLadder(), "w3s": pendingTrade()} {
		st := rebuildState(trade)
		if !st.slowDeclinePending || !st.quietDeclineWatched || st.capitalProtectionWatched || slowDeclineFillUnjudged(trade, st) || st.saleHeld || st.slowDeclineActivated {
			t.Fatalf("fixture drifted: the %s ladder must be pending, watched and short of the last depth with its newest fill judged, got %+v", name, st)
		}
		if !slowDeclineTriggered(trade, st) {
			t.Fatalf("fixture drifted: the %s ladder's trigger of the shared action must stand, got %+v", name, st)
		}
		if trade.PositionType != "buy" {
			t.Fatalf("fixture drifted: the %s ladder must rest in buy, got %q", name, trade.PositionType)
		}
		if armed := armingPrice(trade); !(armed < ladder.AverageEntryPrice(trade) && handLossPct(trade, armed) > 0) {
			t.Fatalf("fixture drifted: the %s ladder's arming print %v must sit under its break even", name, armed)
		}
	}
	if ladder.EntryNotional(exactLadder()) != used || ladder.AverageEntryPrice(exactLadder()) != used/units {
		t.Fatal("fixture drifted: the exact ladder's used amount and break even must be the hand sums")
	}
	if ladder.AverageEntryPrice(exactLadder()) != 100 {
		t.Fatal("fixture drifted: the exact ladder must break even at exactly one hundred")
	}
}

// Over the cap the action waits in every acting state and on every proposal, the one held row naming
// the loss; at the first price within the cap buy is activated (a natural arming replaced) and
// trailingBuy sold with no row, one step lower it waits again. The ladder's own closes pass untouched,
// but a take profit in buy, activated within the cap and standing beside the held row over it.
func TestTheActionIsHeldOverTheCapAndGoesAheadWithinIt(t *testing.T) {
	underTheShippedCap(t)
	trade := pendingTrade()
	armed, held := armingOverTheCap(t, trade)
	within, oneStepLower := firstPriceWithinTheCap(t, trade, armed)
	activation := ActivatedRow(slowDeclineLastFill, -handLossPct(trade, within), FeatureQuietDecline)

	for _, state := range actingStates {
		resting := restingIn(trade, state)
		for _, position := range addSideProposals {
			assertWaits(t, Apply(resting, position, armed, nothingServed()), position, SaleHeldRow(slowDeclineLastFill, -held, FeatureQuietDecline))

			ahead := Apply(resting, position, within, nothingServed())
			if state == "buy" {
				assertActivated(t, ahead, activation)
			} else {
				assertSold(t, ahead, reasonSlowDecline)
			}
			assertWaits(t, Apply(resting, position, oneStepLower, nothingServed()), position,
				SaleHeldRow(slowDeclineLastFill, -handLossPct(trade, oneStepLower), FeatureQuietDecline))
		}
		for _, position := range closingProposals {
			for _, price := range []float64{armed, within, oneStepLower} {
				assertUntouched(t, Apply(resting, position, price, nothingServed()), position)
			}
		}
		for _, position := range takeProfitProposals {
			if state != "buy" {
				for _, price := range []float64{armed, within, oneStepLower} {
					assertUntouched(t, Apply(resting, position, price, nothingServed()), position)
				}
				continue
			}
			assertWaits(t, Apply(resting, position, armed, nothingServed()), position, SaleHeldRow(slowDeclineLastFill, -held, FeatureQuietDecline))
			assertActivated(t, Apply(resting, position, within, nothingServed()), activation)
			assertWaits(t, Apply(resting, position, oneStepLower, nothingServed()), position,
				SaleHeldRow(slowDeclineLastFill, -handLossPct(trade, oneStepLower), FeatureQuietDecline))
		}
	}
}

// A loss exactly at the cap is within it and one step lower is held, the loss simulated at the tick
// price with no haircut and the caller's trade left as it was; an inert state is left alone.
func TestALossExactlyAtTheCapActivatesOrSellsAndOneStepLowerIsHeld(t *testing.T) {
	underTheShippedCap(t)
	atTheCap, oneStepLower := exactBoundary(t)

	trade := exactLadder()
	newest := roundPrices[len(roundPrices)-1]
	for _, state := range actingStates {
		resting := restingIn(trade, state)
		before := resting
		for _, position := range addSideProposals {
			ahead := Apply(resting, position, atTheCap, nothingServed())
			if state == "buy" {
				assertActivated(t, ahead, ActivatedRow(newest, -handLossPct(trade, atTheCap), FeatureQuietDecline))
			} else {
				assertSold(t, ahead, reasonSlowDecline)
			}

			assertWaits(t, Apply(resting, position, oneStepLower, nothingServed()), position,
				SaleHeldRow(newest, -handLossPct(trade, oneStepLower), FeatureQuietDecline))
		}
		if !reflect.DeepEqual(resting, before) || resting.PositionType != state {
			t.Fatalf("state %q: Apply wrote into the trade it was handed", state)
		}
	}
	for _, state := range inertStates {
		for _, position := range addSideProposals {
			for _, price := range []float64{atTheCap, oneStepLower} {
				assertUntouched(t, Apply(restingIn(trade, state), position, price, nothingServed()), position)
			}
		}
	}
}

// A profit reaches any trigger at or under it: with the trigger at a loss or at zero, a ladder at
// break even or a gain is activated from buy with the row naming the profit, and sold from trailingBuy.
func TestAProfitAtOrOverTheTriggerActivatesBuyAndSellsStopLoss(t *testing.T) {
	withQuietDecline(t, true)
	trade := exactLadder()
	newest := roundPrices[len(roundPrices)-1]
	breakEven := ladder.AverageEntryPrice(trade)
	for _, allowed := range []float64{1, 0} {
		withAllowedLoss(t, allowed)
		for _, price := range []float64{breakEven, breakEven + 0.5, breakEven * 1.1} {
			activation := ActivatedRow(newest, 0-handLossPct(trade, price), FeatureQuietDecline)
			for _, position := range addSideProposals {
				assertActivated(t, Apply(trade, position, price, nothingServed()), activation)
				for _, state := range []string{"trailingBuy", "forceTrailingBuy"} {
					assertSoldAt(t, Apply(restingIn(trade, state), position, price, nothingServed()), reasonSlowDecline, trade, price)
				}
			}
		}
	}
}

// A trigger at zero sells at break even and holds any loss; one over zero waits for that profit:
// under it buy and trailingBuy wait with the row naming the profit, at it buy is activated and trailingBuy sold.
func TestATriggerAtZeroOrOverWaitsForThatProfit(t *testing.T) {
	withQuietDecline(t, true)
	trade := exactLadder()
	newest := roundPrices[len(roundPrices)-1]
	breakEven := ladder.AverageEntryPrice(trade)
	for _, tc := range []struct {
		name           string
		allowed        float64
		under, reaches float64
	}{
		{"zero", 0, breakEven - 0.01, breakEven},
		{"one percent", -1, breakEven * 1.005, breakEven * 1.01},
	} {
		withAllowedLoss(t, tc.allowed)
		held := SaleHeldRow(newest, 0-handLossPct(trade, tc.under), FeatureQuietDecline)
		activation := ActivatedRow(newest, 0-handLossPct(trade, tc.reaches), FeatureQuietDecline)
		for _, position := range addSideProposals {
			for _, state := range actingStates {
				assertWaits(t, Apply(restingIn(trade, state), position, tc.under, nothingServed()), position, held)
			}
			assertActivated(t, Apply(trade, position, tc.reaches, nothingServed()), activation)
			assertSoldAt(t, Apply(restingIn(trade, "trailingBuy"), position, tc.reaches, nothingServed()), reasonSlowDecline, trade, tc.reaches)
		}
	}
}

// A trade that has used nothing has no profit to measure: there is no amount to take a share of.
func TestATradeThatHasUsedNothingHasNoProfitToMeasure(t *testing.T) {
	underTheShippedCap(t)
	ledger := testutil.LadderTrade(false)
	ledger.History = []aggragates.TradesHistory{{Type: "BUY", Quantity: 12.5, Price: ladder.AccountingPriceCeiling, OrderId: 1}}
	for name, trade := range map[string]aggragates.Trades{"no fill": testutil.LadderTrade(false), "a ledger row only": ledger} {
		if profitPct, standing := saleProfit(trade, 100, FullTrendTrigger()); standing != nothingUsed || profitPct != 0 {
			t.Errorf("%s: saleProfit = %v, %v, want nothing used", name, profitPct, standing)
		}
	}
}

// The held row is written once per pending episode: on the confirming tick after the confirmation,
// then on no tick or proposal until a new confirmed fill makes it due again.
func TestTheHeldRowIsWrittenOncePerPendingEpisodeAndAgainAfterAConfirmation(t *testing.T) {
	underTheShippedCap(t)
	confirmationsOverTheCap(t)
	w3s := fills(8, "21:30:00")

	sixth := pendingAtFive()
	armed := armingPrice(sixth)
	trade, got := engineTick(sixth, "trailingBuy", armed, judgeTick, legOnAndQuiet())
	assertWaits(t, got, "trailingBuy", sixthConfirmed(), SaleHeldRow(sixthFill, -handLossPct(trade, armed), FeatureQuietDecline))
	if st := rebuildState(trade); !st.saleHeld || st.saleHeldRowDue() || !st.slowDeclinePending || st.slowDeclinePendingFrom != sixthFill {
		t.Fatalf("the held row folds back as the sale held in the episode, the ladder still pending from the sixth fill, got %+v", st)
	}

	for _, price := range []float64{armed, armed - 1, armed + 2, armed - 2, armed} {
		for _, reading := range []aggragates.AIIndicators{nothingServed(), legOnAndQuiet(), brokenReading()} {
			for _, position := range addSideProposals {
				assertUntouched(t, Apply(trade, position, price, reading), position)
			}
		}
	}

	trade = withFill(trade, w3s[6].Price, testutil.At("23:30:00"))
	armed = armingPrice(trade)
	trade, got = engineTick(trade, "", armed, testutil.At("23:40:00"), legOnAndQuiet())
	assertWaits(t, got, "", ConfirmedRow(w3s[6].Price, quietFinding(slowDeclineReasons)), SaleHeldRow(w3s[6].Price, -handLossPct(trade, armed), FeatureQuietDecline))
	assertUntouched(t, Apply(trade, "trailingBuy", armed, nothingServed()), "trailingBuy")

	_, wantEvents := expectedRows(
		quietPending(pendingFromFifth),
		sixthConfirmed(), SaleHeldRow(sixthFill, -0, FeatureQuietDecline),
		ConfirmedRow(w3s[6].Price, quietFinding(slowDeclineReasons)), SaleHeldRow(w3s[6].Price, -0, FeatureQuietDecline),
	)
	if listed := writtenEvents(t, trade); !reflect.DeepEqual(listed, wantEvents) {
		t.Fatalf("events %v, want one held event per confirmation, %v", listed, wantEvents)
	}
}

// A last-depth fill is never judged, so the episode goes on and an action already held is not held
// again; one not held before is held once, on the first tick after it.
func TestTheFillThatTakesTheLadderToItsLastDepthWritesNoSecondHeldRow(t *testing.T) {
	underTheShippedCap(t)
	w3s := fills(8, "21:30:00")
	seventh, eighth := w3s[6].Price, w3s[7].Price

	pending := withRows(testutil.LadderTrade(false, fills(7, "20:00:00")...), quietPending(seventh))
	held := withRows(pending, SaleHeldRow(seventh, -3, FeatureQuietDecline))
	if st := rebuildState(held); !st.saleHeld || st.saleHeldRowDue() {
		t.Fatalf("fixture drifted: the action is held in the episode and its row is not due, got %+v", st)
	}

	atLastDepth := withFill(held, eighth, testutil.At("21:30:00"))
	if ladder.CountFilledEntries(atLastDepth) != ladder.ConfiguredDepths(atLastDepth) {
		t.Fatal("fixture drifted: the eighth fill must take the ladder to its last depth")
	}
	if st := rebuildState(atLastDepth); !st.slowDeclinePending || st.slowDeclinePendingFrom != seventh || !st.saleHeld || st.saleHeldRowDue() {
		t.Fatalf("the last depth's fill leaves the ladder pending from where it was with its action still held and no row due, got %+v", st)
	}
	price, _ := armingOverTheCap(t, atLastDepth)
	prints := []float64{price, price - 1, price}
	for _, at := range prints {
		for _, position := range addSideProposals {
			assertUntouched(t, Apply(atLastDepth, position, at, nothingServed()), position)
		}
	}

	notHeld := withFill(pending, eighth, testutil.At("21:30:00"))
	first := Apply(notHeld, "trailingBuy", price, nothingServed())
	assertWaits(t, first, "trailingBuy", SaleHeldRow(eighth, -handLossPct(notHeld, price), FeatureQuietDecline))
	written := withRow(notHeld, first.Rows[0], testutil.At("21:45:00"))
	for _, at := range prints {
		for _, position := range addSideProposals {
			assertUntouched(t, Apply(written, position, at, nothingServed()), position)
		}
	}
}

// A cancelled ladder is not pending and writes no held row; marked pending again it is a new episode, held again.
func TestNoHeldRowAfterACancelAndADueRowWhenTheLadderGoesPendingAgain(t *testing.T) {
	underTheShippedCap(t)
	confirmationsOverTheCap(t)
	w3s := fills(8, "21:30:00")
	seventh := w3s[6].Price

	sixth := pendingAtFive()
	trade, _ := engineTick(sixth, "trailingBuy", armingPrice(sixth), judgeTick, legOnAndQuiet())
	trade = withFill(trade, seventh, testutil.At("23:30:00"))
	armed := armingPrice(trade)
	trade, got := engineTick(trade, "trailingBuy", armed, testutil.At("23:40:00"), brokenReading())
	assertRows(t, got, CancelledRow(seventh, quietFinding(quietDeclineBreakReasons)))
	assertNoSale(t, got, "trailingBuy")
	if st := rebuildState(trade); st.slowDeclinePending || st.saleHeld {
		t.Fatalf("a cancelled ladder is not pending and holds no sale, got %+v", st)
	}

	for _, price := range []float64{armed - 1, armed, armed + 2} {
		for _, reading := range []aggragates.AIIndicators{brokenReading(), nothingServed(), legOnAndQuiet()} {
			for _, position := range addSideProposals {
				assertUntouched(t, Apply(trade, position, price, reading), position)
			}
		}
	}

	trade, got = engineTick(trade, "trailingBuy", armed, testutil.At("23:50:00"), verdictReading())
	assertWaits(t, got, "trailingBuy", quietPending(seventh), SaleHeldRow(seventh, -handLossPct(trade, armed), FeatureQuietDecline))
}

// A pending ladder a depth priority holds is reset and nothing is activated, sold or held until the
// next fill; pending again, the held action is a new episode with its own row.
func TestNoHeldRowUnderADepthPriorityHoldAndADueRowOnceItEnds(t *testing.T) {
	underTheShippedCap(t)
	withDepthPriorityPause(t, true)
	confirmationsOverTheCap(t)
	sixth := pendingAtFive()
	held, _ := engineTick(sixth, "trailingBuy", armingPrice(sixth), judgeTick, legOnAndQuiet())
	held = heldBy(held, testutil.At("23:30:00"))
	armed := armingPrice(held)

	trade, got := engineTick(held, "trailingBuy", armed, testutil.At("23:35:00"), nothingServed())
	assertRows(t, got, ResetRow(sixthFill, quietFinding(nil)))
	assertNoSale(t, got, "trailingBuy")
	for _, state := range actingStates {
		for _, price := range []float64{armed - 1, armed, armed + 2} {
			for _, position := range addSideProposals {
				assertUntouched(t, Apply(restingIn(trade, state), position, price, nothingServed()), position)
			}
		}
	}

	w3s := fills(8, "21:30:00")
	trade = withFill(trade, w3s[6].Price, testutil.At("23:45:00"))
	armed = armingPrice(trade)
	trade, got = engineTick(trade, "trailingBuy", armed, testutil.At("23:50:00"), verdictReading())
	assertWaits(t, got, "trailingBuy", quietPending(w3s[6].Price), SaleHeldRow(w3s[6].Price, -handLossPct(trade, armed), FeatureQuietDecline))
}

// The ladder's own closes pass the overlay untouched; only the add side and a take profit in buy
// reach the shared action.
func TestNoHeldRowBesideAClosedPositionOrANonPendingLadder(t *testing.T) {
	underTheShippedCap(t)
	trade := pendingTrade()
	armed := armingPrice(trade)
	for _, position := range closingProposals {
		assertUntouched(t, Apply(trade, position, armed, nothingServed()), position)
	}
	for _, resting := range []string{"takeProfit", "forceTrailingTakeProfit", "takeLoss", "sell", "sellParent", "impasse"} {
		for _, position := range append([]string{""}, addSideProposals...) {
			assertUntouched(t, Apply(restingIn(trade, resting), position, armed, nothingServed()), position)
		}
	}

	notPending := watchedTrade()
	for _, state := range actingStates {
		for _, position := range append([]string{""}, addSideProposals...) {
			assertUntouched(t, Apply(restingIn(notPending, state), position, armed, nothingServed()), position)
		}
	}
}

// The loss counts the commissions the close is charged, as acceptLoss books them, so the exact ladder
// needs a higher price to be within the cap, in buy and in trailingBuy.
func TestTheLossCountsTheFeesTheCloseIsCharged(t *testing.T) {
	underTheShippedCap(t)
	atTheCap, _ := exactBoundary(t)
	const feePerFill = 1.0
	_, units := exactTotals()
	extra := 2 * feePerFill * float64(len(roundPrices)) / units

	trade := exactLadder()
	for index := range trade.History {
		trade.History[index].Fees = []aggragates.TradesFees{{Asset: "USDT", Fee: feePerFill}}
	}

	for _, state := range []string{"buy", "trailingBuy"} {
		resting := restingIn(trade, state)
		held := Apply(resting, "trailingBuy", atTheCap, nothingServed())
		assertNoSale(t, held, "trailingBuy")
		if len(held.Rows) != 1 || held.Rows[0].Event != EventSaleHeld {
			t.Fatalf("%s: with fees the price that was exactly at the cap is over it, got %+v", state, held.Rows)
		}
		shiftedCents := math.Round((atTheCap + extra) * 100)
		assertGoesAhead(t, Apply(resting, "trailingBuy", shiftedCents/100, nothingServed()), state)
		assertNoSale(t, Apply(resting, "trailingBuy", (shiftedCents-1)/100, nothingServed()), "trailingBuy")
	}
}

// capturedStdout is what f writes to standard output, where the log package writes every line.
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

// A fee paid in a third asset cannot be priced here and the cap leaves it out, as a backtest reads it;
// nothing is looked up, so no line is logged on a held tick, and the caller's history keeps its fees.
func TestAThirdAssetFeeIsLeftOutOfTheLossAndNeverPriced(t *testing.T) {
	underTheShippedCap(t)
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

	for _, state := range []string{"buy", "trailingBuy"} {
		var ahead, held Result
		logged := capturedStdout(t, func() {
			ahead = Apply(restingIn(trade, state), "trailingBuy", atTheCap, nothingServed())
			held = Apply(restingIn(trade, state), "trailingBuy", oneStepLower, nothingServed())
		})
		assertGoesAhead(t, ahead, state)
		assertNoSale(t, held, "trailingBuy")
		if len(held.Rows) != 1 || held.Rows[0].Event != EventSaleHeld {
			t.Fatalf("%s: one price step under the boundary is held, got %+v", state, held.Rows)
		}
		if strings.Contains(logged, "getSymbolPrice") {
			t.Fatalf("%s: a third-asset fee was looked up and the lookup logged: %s", state, logged)
		}
	}
	if !reflect.DeepEqual(trade.History, before) {
		t.Fatalf("the caller's history lost or changed its fees: %+v, want %+v", trade.History, before)
	}
}

// The dust the close keeps is credited as the close books it: a lot-size remainder loses what a sale
// of everything loses, so the price exactly at the cap is within it.
func TestTheLossCountsTheDustTheCloseKeeps(t *testing.T) {
	underTheShippedCap(t)
	trade := testutil.LadderTrade(false)
	for index, quantity := range []float64{50.25, 50.25} {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: quantity, Price: 100, OrderId: int64(index + 1), CreatedAt: testutil.At(fmt.Sprintf("%02d:00:00", 9+index)),
		})
	}
	trade.PositionPrice = 100
	trade = withRows(trade, quietPending(100))

	atTheCap, oneStepLower, exact := capBoundary(100.5*100, 100.5, float64(shippedLoss))
	if !exact {
		t.Skip("SaleTriggerPercentage takes no loss or names no price above zero on the cent grid the dust ladder loses exactly that much at")
	}
	if ladder.CountFilledEntries(trade) < quietdecline.ArmDepth {
		t.Skip("quietdecline.ArmDepth leaves the dust ladder unwatched")
	}

	for _, state := range []string{"buy", "trailingBuy"} {
		resting := restingIn(trade, state)
		assertGoesAhead(t, Apply(resting, "trailingBuy", atTheCap, nothingServed()), state)
		held := Apply(resting, "trailingBuy", oneStepLower, nothingServed())
		assertNoSale(t, held, "trailingBuy")
		if len(held.Rows) != 1 || held.Rows[0].Event != EventSaleHeld {
			t.Fatalf("%s: one price step under the boundary is held, got %+v", state, held.Rows)
		}
	}
}

// An action the cap held goes ahead on the first tick within it, whatever the ladder proposes: buy is
// activated with the activation row alone, trailingBuy sold with no row, and the sold row leaves no action held.
func TestAHeldActionGoesAheadOnTheFirstTickWithinTheCap(t *testing.T) {
	underTheShippedCap(t)
	atTheCap, oneStepLower := exactBoundary(t)
	newest := roundPrices[len(roundPrices)-1]
	loss := handLossPct(exactLadder(), atTheCap)

	for _, state := range []string{"buy", "trailingBuy"} {
		held, got := engineStep(restingIn(exactLadder(), state), "", oneStepLower, testutil.At("13:00:00"), nothingServed())
		assertWaits(t, got, "", SaleHeldRow(newest, -handLossPct(exactLadder(), oneStepLower), FeatureQuietDecline))
		assertUntouched(t, Apply(held, "update_trailingBuy", oneStepLower-1, nothingServed()), "update_trailingBuy")
		assertUntouched(t, Apply(held, "", oneStepLower-1, nothingServed()), "")

		ahead := Apply(held, "update_trailingBuy", atTheCap, nothingServed())
		if state == "trailingBuy" {
			assertSold(t, ahead, reasonSlowDecline)
			closed := withRow(held, ExitRow(held, ahead.Reason, atTheCap), testutil.At("13:15:00"))
			if st := rebuildState(closed); st.saleHeld || !st.slowDeclinePending {
				t.Fatalf("a sold event forgets the held action and leaves the ladder pending, got %+v", st)
			}
			continue
		}
		assertActivated(t, ahead, ActivatedRow(newest, -loss, FeatureQuietDecline))
		activated, _ := engineStep(held, "update_trailingBuy", atTheCap, testutil.At("13:15:00"), nothingServed())
		if st := rebuildState(activated); !st.saleHeld || !st.slowDeclineActivated || !st.slowDeclinePending {
			t.Fatalf("the activated event leaves the ladder pending with its held row written and its trailingBuy activated, got %+v", st)
		}
		assertSold(t, Apply(activated, "buy", oneStepLower-1, nothingServed()), reasonSlowDecline)
	}
}

// The slow decline gate's ticks end to end, off the trade's events: tick one activates the trailingBuy
// from buy with the one row (ExitReached reads no exit), the ticks after trail from the activation
// print whatever the ladder proposes or sophos serves (re-anchor, hold, sell), and the sold row
// closes the episode; a depth filled before the sale is a new episode and drops the commitment.
func TestTheSlowDeclineActivatesTheStopLossOnOneTickAndTrailsBeforeItSells(t *testing.T) {
	underTheShippedCap(t)
	trade := pendingTrade()
	armed, _ := armingOverTheCap(t, trade)
	within, _ := firstPriceWithinTheCap(t, trade, armed)
	average := ladder.AverageEntryPrice(trade)

	if ExitReached(trade, within, nothingServed().SmartSell) {
		t.Error("an activation is no exit: ExitReached reads the sale alone")
	}
	activated, first := engineStep(trade, "", within, testutil.At("18:00:00"), nothingServed())
	assertActivated(t, first, ActivatedRow(slowDeclineLastFill, -handLossPct(trade, within), FeatureQuietDecline))
	if activated.PositionType != "trailingBuy" || activated.PositionPrice != within {
		t.Fatalf("the activation chain leaves the ladder in trailingBuy at the tick price, got %q at %v", activated.PositionType, activated.PositionPrice)
	}
	if st := rebuildState(activated); !st.slowDeclinePending || !st.slowDeclineActivated || st.saleHeld {
		t.Fatalf("the activated event folds back as the trailingBuy activated, the ladder still pending, got %+v", st)
	}

	for _, tick := range []struct {
		name     string
		position string
		price    float64
		answer   saleStep
		reason   string
		result   string
	}{
		{"a bounce over break even proposing buy", "buy", average * 1.02, stepReanchor, "", "trailingBuy"},
		{"a small bounce proposing update_buy", "update_buy", within + 0.1, stepHold, "", ""},
		{"a fall over the cap proposing update_trailingBuy", "update_trailingBuy", armed, stepSell, reasonSlowDecline, "takeLoss"},
		{"a fall far over the cap proposing nothing", "", armed * 0.9, stepSell, reasonSlowDecline, "takeLoss"},
		{"the same print again", "", within, stepHold, "", ""},
	} {
		for _, reading := range []aggragates.AIIndicators{nothingServed(), brokenReading(), legOnAndQuiet()} {
			got := Apply(activated, tick.position, tick.price, reading)
			if answer, reason := trailAnswer(got); answer != tick.answer || reason != tick.reason || got.Position != tick.result || len(got.Rows) != 0 {
				t.Fatalf("a later tick, %s: want %s (%q) with no row, got %+v", tick.name, stepNames[tick.answer], tick.reason, got)
			}
		}
		if got := ExitReached(activated, tick.price, nothingServed().SmartSell); got != (tick.answer == stepSell) {
			t.Errorf("a later tick, %s: ExitReached = %v, the activated ladder's exit is the sale alone", tick.name, got)
		}
	}

	sold := withRow(activated, ExitRow(activated, reasonSlowDecline, within), testutil.At("18:05:00"))
	if st := rebuildState(sold); st.slowDeclineActivated || st.saleHeld || !st.slowDeclinePending {
		t.Fatalf("the sold event forgets the activation and leaves the ladder pending, got %+v", st)
	}
}

// A new depth drops the commitment: the ladder is pending from the fill before it with the new fill
// unjudged, so nothing is sold until the reading judges it, a break cancelling and a confirmation
// opening a new episode that has activated nothing, held over the cap and sold within it.
func TestANewDepthDropsTheSlowDeclineCommitmentAndIsJudgedAsBefore(t *testing.T) {
	underTheShippedCap(t)
	trade := pendingTrade()
	armed, _ := armingOverTheCap(t, trade)
	within, _ := firstPriceWithinTheCap(t, trade, armed)
	activated, _ := engineStep(trade, "", within, testutil.At("18:00:00"), nothingServed())
	if st := rebuildState(activated); !st.slowDeclineActivated || activated.PositionType != "trailingBuy" {
		t.Fatalf("fixture drifted: the gate must have activated the trailingBuy, got %+v", st)
	}

	newest := fills(lastDepthFills, "21:30:00")[4].Price
	deeper := withFill(activated, newest, testutil.At("19:00:00"))
	if st := rebuildState(deeper); !st.slowDeclinePending || !slowDeclineFillUnjudged(deeper, st) {
		t.Fatalf("fixture drifted: the new fill must wait unjudged on a ladder still pending, got %+v", st)
	}
	farOver := armingPrice(deeper)
	cheap := aLossWithinTheCap(t, deeper)
	for _, price := range []float64{cheap, ladder.AverageEntryPrice(deeper) * gainFactor, farOver} {
		for _, position := range addSideProposals {
			assertUntouched(t, Apply(deeper, position, price, nothingServed()), position)
		}
	}

	cancelled := Apply(deeper, "buy", farOver, brokenReading())
	assertWaits(t, cancelled, "buy", CancelledRow(newest, quietFinding(quietDeclineBreakReasons)))
	after := withRow(deeper, cancelled.Rows[0], testutil.At("19:05:00"))
	if st := rebuildState(after); st.slowDeclinePending || st.slowDeclineActivated || st.saleHeld {
		t.Fatalf("a cancelled exit has no commitment left, got %+v", st)
	}
	for _, position := range addSideProposals {
		assertUntouched(t, Apply(after, position, farOver, brokenReading()), position)
	}

	confirmed := ConfirmedRow(newest, quietFinding(slowDeclineReasons))
	assertWaits(t, Apply(deeper, "buy", farOver, legOnAndQuiet()), "buy",
		confirmed, SaleHeldRow(newest, -handLossPct(deeper, farOver), FeatureQuietDecline))
	assertSoldWith(t, Apply(deeper, "buy", cheap, legOnAndQuiet()), reasonSlowDecline, confirmed)
}

// The state a tick leaves forgets the held action and the activation the way the fold does: a tick
// that makes the ladder pending or not pending holds no action from then on, so the row is due again.
func TestTheStateATickLeavesForgetsTheHeldActionLikeTheFold(t *testing.T) {
	held := state{
		fills:                  []entryFill{{Price: 100}, {Price: 99}},
		slowDeclinePending:     true,
		slowDeclinePendingFrom: 100,
		saleHeld:               true,
		saleHeldUnder:          heldTrigger{pct: -0.5, named: true},
		slowDeclineActivated:   true,
		pendingFeatures:        []string{FeatureQuietDecline},
	}
	if got := held.pendingOn([]Finding{quietFinding(nil)}); got.saleHeld || got.saleHeldUnder != (heldTrigger{}) || got.slowDeclineActivated || !got.slowDeclinePending || got.slowDeclinePendingFrom != 99 || !got.saleHeldRowDue() {
		t.Errorf("a ladder made pending again holds no action, has activated nothing and is due its row, got %+v", got)
	}
	if got := held.notPending(); got.saleHeld || got.saleHeldUnder != (heldTrigger{}) || got.slowDeclineActivated || got.slowDeclinePending {
		t.Errorf("a ladder made not pending holds no action and has activated nothing, got %+v", got)
	}
}

// Each held row is filed under the slow decline gate as the sale held kind at the newest fill with the
// pending features, the same document for the loss cap and the depth bound alike but the trigger
// the loss cap's row names.
func TestTheHeldRowIsFiledUnderItsGateAndKindAndReadsBack(t *testing.T) {
	underTheShippedCap(t)
	trade := pendingTrade()
	armed, _ := armingOverTheCap(t, trade)
	filled := ladder.CountFilledEntries(trade)
	byLoss := `,"trigger":` + strconv.FormatFloat(0-shippedLoss, 'f', -1, 64)
	for _, tc := range []struct {
		name    string
		capPct  float64
		left    int
		trigger string
	}{
		{"the loss cap", shippedLoss, 0, byLoss},
		{"the depth bound", 0, lastDepthFills - filled + 1, ""},
	} {
		withAllowedLoss(t, tc.capPct)
		withSaleIgnoreLastDepths(t, tc.left)
		got := Apply(trade, "trailingBuy", armed, nothingServed())
		row := firstRow(got)
		if row == nil {
			t.Fatalf("%s: no single row: %+v", tc.name, got.Rows)
		}
		if row.Gate != "slowDecline" || row.Event != "saleHeld" || row.Price != slowDeclineLastFill || !reflect.DeepEqual(row.Features, []string{"quietDecline"}) || row.Reasons != nil {
			t.Fatalf("%s: the held row is filed %q/%q at %v with %v and %v, want slowDecline/saleHeld at the newest fill on the quiet decline and no reasons", tc.name, row.Gate, row.Event, row.Price, row.Features, row.Reasons)
		}

		logRow, event := Rows(trade, *row, testutil.At("21:45:00"))
		if logRow.Message != row.Message || logRow.Price != slowDeclineLastFill || event.Gate != GateSlowDecline || event.Kind() != EventSaleHeld {
			t.Fatalf("%s: the pair is the row and the sale held event, got %+v and %+v", tc.name, logRow, event)
		}
		if string(event.Data) != `{"event":"saleHeld","price":184.45,"features":["quietDecline"]`+tc.trigger+`}` {
			t.Fatalf("%s: the document is the kind, the fill and the features, got %s", tc.name, event.Data)
		}
		if st := rebuildState(withRow(trade, *row, testutil.At("21:45:00"))); !st.saleHeld || st.saleHeldRowDue() {
			t.Fatalf("%s: the held row folds back as the sale held, got %+v", tc.name, st)
		}
	}
}

// foldCase is one ladder the slow decline fold reads: whether it ends up pending, with
// its action held, with its trailingBuy activated, and on which features.
type foldCase struct {
	name        string
	trade       aggragates.Trades
	pending     bool
	held        bool
	activated   bool
	switchedOff bool
	pendingOn   []string
}

// The fold remembers the episode's held action and activated trailingBuy, whatever fill the row carries,
// and forgets both on a new pending, cancelled, reset or sold event; a held or activated event with no
// features, no price, another gate or an unwatched ladder is nothing, and legacy pending states are kept.
func TestTheFoldRemembersTheEpisodesActionWasHeldOrActivatedWhateverFillItCarries(t *testing.T) {
	withQuietDecline(t, true)
	trade := watchedTrade()
	fill := trade.PositionPrice
	quiet := []string{FeatureQuietDecline}
	pending := quietPending(fill)
	held := SaleHeldRow(fill, -3, FeatureQuietDecline)
	heldAtDepth := SaleHeldAtDepthRow(fill, 7, 8, FeatureQuietDecline)
	activated := ActivatedRow(fill, -0.25, FeatureQuietDecline)
	sold := ExitRow(withRows(trade, pending), reasonSlowDecline, 186)
	document := func(kind string, price float64) string {
		return fmt.Sprintf(`{"event":%q,"price":%v,"features":["quietDecline"]}`, kind, price)
	}

	cases := []foldCase{
		{name: "pending, then held", trade: withRows(trade, pending, held), pending: true, held: true, pendingOn: quiet},
		{name: "pending, then held by the depth bound", trade: withRows(trade, pending, heldAtDepth), pending: true, held: true, pendingOn: quiet},
		{name: "held, then sold", trade: withRows(trade, pending, held, sold), pending: true, pendingOn: quiet},
		{name: "held by the depth bound, then sold", trade: withRows(trade, pending, heldAtDepth, sold), pending: true, pendingOn: quiet},
		{name: "held, then cancelled", trade: withRows(trade, pending, held, CancelledRow(fill, quietFinding(nil)))},
		{name: "held, then reset", trade: withRows(trade, pending, held, ResetRow(fill, quietFinding(nil)))},
		{name: "held, then pending again at the same fill", trade: withRows(trade, pending, held, ConfirmedRow(fill, quietFinding(nil))), pending: true, pendingOn: quiet},
		{name: "held, cancelled, pending again", trade: withRows(trade, pending, held, CancelledRow(fill, quietFinding(nil)), pending), pending: true, pendingOn: quiet},
		{name: "held twice", trade: withRows(trade, pending, held, held), pending: true, held: true, pendingOn: quiet},
		{name: "held by each bound in turn", trade: withRows(trade, pending, held, heldAtDepth), pending: true, held: true, pendingOn: quiet},
		{name: "held at a lower price than the fill the ladder is pending from", trade: withRows(trade, pending, SaleHeldRow(fill-1, -3, FeatureQuietDecline)), pending: true, held: true, pendingOn: quiet},
		{name: "held at a higher price than the fill the ladder is pending from", trade: withRows(trade, pending, SaleHeldRow(fill+1, -3, FeatureQuietDecline)), pending: true, held: true, pendingOn: quiet},
		{name: "held with no features", trade: withEvents(withRows(trade, pending), eventOf(GateSlowDecline, `{"event":"saleHeld","price":184.45}`)), pending: true, pendingOn: quiet},
		{name: "held at no price", trade: withEvents(withRows(trade, pending), eventOf(GateSlowDecline, document(EventSaleHeld, 0))), pending: true, pendingOn: quiet},
		{name: "held under another gate", trade: withEvents(withRows(trade, pending), eventOf(GateCapitalProtection, document(EventSaleHeld, fill))), pending: true, pendingOn: quiet},
		{name: "held on the legacy quiet decline state", trade: withEvents(trade, eventOf(GateSlowDecline, legacyDocument(EventPending, fill)), eventOf(GateSlowDecline, document(EventSaleHeld, fill))), pending: true, held: true, pendingOn: quiet},
		{name: "pending, then activated", trade: withRows(trade, pending, activated), pending: true, activated: true, pendingOn: quiet},
		{name: "held, then activated", trade: withRows(trade, pending, held, activated), pending: true, held: true, activated: true, pendingOn: quiet},
		{name: "activated, then held", trade: withRows(trade, pending, activated, held), pending: true, held: true, activated: true, pendingOn: quiet},
		{name: "activated twice", trade: withRows(trade, pending, activated, activated), pending: true, activated: true, pendingOn: quiet},
		{name: "activated, then sold", trade: withRows(trade, pending, activated, sold), pending: true, pendingOn: quiet},
		{name: "held, activated, then sold", trade: withRows(trade, pending, held, activated, sold), pending: true, pendingOn: quiet},
		{name: "activated, then cancelled", trade: withRows(trade, pending, activated, CancelledRow(fill, quietFinding(nil)))},
		{name: "activated, then reset", trade: withRows(trade, pending, activated, ResetRow(fill, quietFinding(nil)))},
		{name: "activated, then pending again at the same fill", trade: withRows(trade, pending, activated, ConfirmedRow(fill, quietFinding(nil))), pending: true, pendingOn: quiet},
		{name: "held, activated, then pending again", trade: withRows(trade, pending, held, activated, ConfirmedRow(fill, quietFinding(nil))), pending: true, pendingOn: quiet},
		{name: "activated, cancelled, pending, activated", trade: withRows(trade, pending, activated, CancelledRow(fill, quietFinding(nil)), pending, activated), pending: true, activated: true, pendingOn: quiet},
		{name: "activated at a different price than the fill the ladder is pending from", trade: withRows(trade, pending, ActivatedRow(fill+1, -0.25, FeatureQuietDecline)), pending: true, activated: true, pendingOn: quiet},
		{name: "activated with no features", trade: withEvents(withRows(trade, pending), eventOf(GateSlowDecline, `{"event":"activated","price":184.45}`)), pending: true, pendingOn: quiet},
		{name: "activated at no price", trade: withEvents(withRows(trade, pending), eventOf(GateSlowDecline, document(EventActivated, 0))), pending: true, pendingOn: quiet},
		{name: "activated under another gate", trade: withEvents(withRows(trade, pending), eventOf(GateCapitalProtection, document(EventActivated, fill))), pending: true, pendingOn: quiet},
		{name: "activated by the flat market gate", trade: withRows(trade, pending, FlatMarketActivatedRow(fill, -0.25)), pending: true, pendingOn: quiet},
		{name: "activated on the legacy quiet decline state", trade: withEvents(trade, eventOf(GateSlowDecline, legacyDocument(EventPending, fill)), eventOf(GateSlowDecline, document(EventActivated, fill))), pending: true, activated: true, pendingOn: quiet},

		// The cases that switch the features off come last: the switches stay off for the rest of the test.
		{name: "held where nothing watches", trade: withRows(trade, pending, held), switchedOff: true},
		{name: "activated where nothing watches", trade: withRows(trade, pending, activated), switchedOff: true},
	}
	for _, c := range cases {
		if c.switchedOff {
			withQuietDecline(t, false)
			withSwingDecline(t, false)
			withEarlyTakeProfit(t, false)
		}
		st := rebuildState(c.trade)
		if st.slowDeclinePending != c.pending || st.saleHeld != c.held || st.slowDeclineActivated != c.activated || !reflect.DeepEqual(st.pendingFeatures, c.pendingOn) {
			t.Errorf("%s: pending %v on %v, action held %v, trailingBuy activated %v, want pending %v on %v, held %v, activated %v",
				c.name, st.slowDeclinePending, st.pendingFeatures, st.saleHeld, st.slowDeclineActivated, c.pending, c.pendingOn, c.held, c.activated)
		}
		if c.pending && st.saleHeldRowDue() == c.held {
			t.Errorf("%s: the held row is due %v with the action held %v", c.name, st.saleHeldRowDue(), st.saleHeld)
		}
	}
}

// The activated event moves its flag alone and opens no episode of the flat market gate.
func TestTheActivatedEventOfTheSlowDeclineGateMovesItsFlagAlone(t *testing.T) {
	withQuietDecline(t, true)
	withFlatMarket(t, true)
	trade := watchedTrade()
	fill := trade.PositionPrice
	pending := withRows(trade, quietPending(fill))
	activated := withRows(pending, ActivatedRow(fill, -0.25, FeatureQuietDecline))

	before, after := rebuildState(pending), rebuildState(activated)
	if before.slowDeclineActivated || !after.slowDeclineActivated || after.slowDeclineActivatedIn != "trailingBuy" {
		t.Fatalf("the event sets the flag and the state it moved to: before %v, after %v in %q", before.slowDeclineActivated, after.slowDeclineActivated, after.slowDeclineActivatedIn)
	}
	after.slowDeclineActivated, after.slowDeclineActivatedIn = false, ""
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("the activated event must move the flag alone:\nbefore %+v\n after %+v", before, after)
	}
	if st := rebuildState(activated); st.flatMarketActive || st.flatMarketActivated || st.flatMarketFrom != 0 {
		t.Fatalf("a slow decline activation opens no flat market episode, got %+v", st)
	}
}

// saleProfit tells reached (a profit at or over the trigger: a gain, break even or a loss the trigger
// takes) from short (a profit under it); the profit comes with it as a share of the used amount, a loss negative.
func TestSaleProfitTellsReachedFromShort(t *testing.T) {
	trade := flatMarketLadder(false)
	capPct := float64(shippedLoss)
	withAllowedLoss(t, capPct)
	atTheCap, oneStepLower := salePrices(t, trade)
	average := ladder.AverageEntryPrice(trade)
	within := (atTheCap + average) / 2
	farOver := average * lossFactor
	gain := average * gainFactor
	farOverTheCap(t, capPct)

	for _, tc := range []struct {
		name    string
		price   float64
		trigger float64
		profit  float64
		want    triggerStanding
	}{
		{"a gain", gain, -capPct, -lossPctOf(trade, gain), reached},
		{"the break even", average, -capPct, 0, reached},
		{"a loss the trigger takes", within, -capPct, -lossPctOf(trade, within), reached},
		{"a loss exactly at the trigger", atTheCap, -capPct, -capPct, reached},
		{"a loss one price step under the trigger", oneStepLower, -capPct, -lossPctOf(trade, oneStepLower), short},
		{"a loss far under the trigger", farOver, -capPct, -lossPctOf(trade, farOver), short},
		{"a gain, the trigger at zero", gain, 0, -lossPctOf(trade, gain), reached},
		{"the break even, the trigger at zero", average, 0, 0, reached},
		{"a loss, the trigger at zero", within, 0, -lossPctOf(trade, within), short},
		{"a gain over a trigger at a profit", gain, 1, -lossPctOf(trade, gain), reached},
		{"a gain under a trigger at a profit", gain, 3, -lossPctOf(trade, gain), short},
		{"the break even, the trigger at a profit", average, 1, 0, short},
	} {
		profitPct, standing := saleProfit(trade, tc.price, tc.trigger)
		if standing != tc.want || math.Abs(profitPct-tc.profit) > 1e-9 {
			t.Errorf("%s: saleProfit = %v, %v, want %v, %v", tc.name, profitPct, standing, tc.profit, tc.want)
		}
	}
}

// The profit is a share of the amount the close's net is counted in: quote for a long ladder
// (ladder.EntryNotional), base for an inverse one (ladder.EntryQuantity); a ladder that used nothing
// has nothing to measure, and the trigger is one for every gate that acts, each row naming the same profit.
func TestSaleProfitIsAShareOfTheAmountTheNetIsCountedIn(t *testing.T) {
	long, inverse := flatMarketLadder(false), flatMarketLadder(true)
	// The profit does not depend on the trigger.
	const anyCap = 1.0
	average := ladder.AverageEntryPrice(long)
	step := average / 50
	if ladder.EntryNotional(inverse) <= ladder.EntryQuantity(inverse) || ladder.AverageEntryPrice(inverse) != average {
		t.Fatal("fixture drifted: the round ladders must break even alike, on a quote far over the base the inverse one sold")
	}

	for name, tc := range map[string]struct {
		trade aggragates.Trades
		price float64
		want  float64
	}{
		"a long ladder sold under its break even": {long, average - step, -step / average * 100},
		"an inverse ladder bought back over it":   {inverse, average + step, -step / (average + step) * 100},
		"a long ladder in profit":                 {long, average + step, step / average * 100},
		"an inverse ladder in profit":             {inverse, average - step, step / (average - step) * 100},
	} {
		profitPct, _ := saleProfit(tc.trade, tc.price, -anyCap)
		if math.Abs(profitPct-tc.want) > 1e-9 {
			t.Errorf("%s: the profit is %v of the amount used, want %v", name, profitPct, tc.want)
		}
		if _, over := saleProfit(tc.trade, tc.price, profitPct+math.Abs(profitPct)/2); over != short {
			t.Errorf("%s: a trigger over the profit is one it falls short of, got %v", name, over)
		}
		if _, under := saleProfit(tc.trade, tc.price, profitPct-math.Abs(profitPct)); under != reached {
			t.Errorf("%s: a trigger under the profit is one it reaches, got %v", name, under)
		}
	}
	for name, trade := range map[string]aggragates.Trades{"a long ladder": testutil.LadderTrade(false), "an inverse ladder": testutil.LadderTrade(true)} {
		if profitPct, standing := saleProfit(trade, average, -anyCap); standing != nothingUsed || profitPct != 0 {
			t.Errorf("%s with no fill: the profit is %v and stands %v, want nothing used", name, profitPct, standing)
		}
	}

	triggers := []saleTrigger{
		{reason: reasonSlowDecline, gate: GateSlowDecline, features: []string{FeatureQuietDecline}, rowDue: true},
		{reason: reasonFlatMarket, gate: GateFlatMarket, rowDue: true},
	}
	for name, trade := range map[string]aggragates.Trades{"a long ladder": long, "an inverse ladder": inverse} {
		withAllowedLoss(t, anyCap)
		price := average - step
		if trade.Inverse {
			price = average + step
		}
		profitPct, standing := saleProfit(trade, price, -anyCap)
		if standing != short {
			t.Fatalf("fixture drifted: %s at %v must lose more than the trigger takes", name, price)
		}
		rows, did, reason, _ := sale(trade, rebuildState(trade), price, triggers, everyRowShared())
		want := []Row{SaleHeldRow(trade.PositionPrice, profitPct, FeatureQuietDecline), FlatMarketSaleHeldRow(trade.PositionPrice, profitPct)}
		if did != stepNone || reason != "" || !reflect.DeepEqual(rows, want) {
			t.Errorf("%s: the shared action holds both gates at one trigger and one profit, got %v, %q and %+v, want %+v", name, stepNames[did], reason, rows, want)
		}

		withAllowedLoss(t, -2*profitPct)
		rows, did, reason, _ = sale(trade, rebuildState(trade), price, triggers, everyRowShared())
		want = []Row{
			ActivatedRow(trade.PositionPrice, profitPct, FeatureQuietDecline),
			FlatMarketActivatedRow(trade.PositionPrice, profitPct),
		}
		if did != activationStep(profitPct) || reason != "" || !reflect.DeepEqual(rows, want) {
			t.Errorf("%s: a trigger the loss reaches activates both gates, each with its row, got %v, %q and %+v, want %+v", name, stepNames[did], reason, rows, want)
		}
		if rows, did, reason, _ := sale(restingIn(trade, "trailingBuy"), rebuildState(trade), price, triggers, everyRowShared()); did != stepSell || reason != reasonSlowDecline || len(rows) != 0 {
			t.Errorf("%s: a trigger the loss reaches sells a ladder in trailingBuy too, got %v, %q and %+v", name, stepNames[did], reason, rows)
		}
	}
}
