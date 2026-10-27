package smarttakeloss

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The fixtures are the w3s ladder watchedFills deep: watched by the quiet decline, short of the
// last depth, serving no series. slowDeclineBand and underTheBand are two prints of the dead zone
// (a loss, so the shared action acts), gainPrint books no loss, and armingPrice is where the buy
// row arms the next depth.
const (
	watchedFills        = 4
	slowDeclineBand     = 186.0
	underTheBand        = 185.0
	slowDeclineLastFill = 184.45
)

var slowDeclineReasons = []string{"leg down 7.0% from its high close", "leg 60 bars long on 1h"}

// ordinaryProposals are the proposals of a tick that arms no depth and decides no close; the action answers them by the state.
var ordinaryProposals = []string{"", "buy", "update_buy"}

// quietActivation is the activation row the quiet decline's trigger is owed at price: at the newest
// fill, naming the loss and the pending feature.
func quietActivation(trade aggragates.Trades, price float64) Row {
	return StopLossActivatedRow(rebuildState(trade).lastFill().Price, -handLossPct(trade, price), FeatureQuietDecline)
}

// printFor is the price a tick proposing position prints at: armingPrice when the proposal arms a
// depth, a print of the dead zone otherwise, both a loss.
func printFor(trade aggragates.Trades, position string) float64 {
	if slices.Contains(armingProposals, position) {
		return armingPrice(trade)
	}
	return underTheBand
}

func watchedTrade() aggragates.Trades {
	return testutil.LadderTrade(false, fills(watchedFills, "17:38:00")...)
}

// pendingTrade is watchedTrade carrying the pending pair the engine wrote for
// its newest fill: the row and its event, pending on the quiet decline.
func pendingTrade() aggragates.Trades {
	return withRows(watchedTrade(), PendingRow(slowDeclineLastFill, quietFinding(slowDeclineReasons)))
}

// quietPending is the row a quiet decline going pending at fill writes, naming
// the reasons the verdict's block serves.
func quietPending(fill float64) Row {
	return PendingRow(fill, quietFinding(slowDeclineReasons))
}

// slowDeclineBlock is the block sophos serves with the verdict on or off, the
// fill window served either way: sophos read its window.
func slowDeclineBlock(on bool) aggragates.AIIndicators {
	block := withEveryRowTrend(aggragates.SmartTakeLossIndicators{QuietDeclineVerdict: on, QuietDeclineFillFrom: fillWindowFrom.UnixMilli()})
	if on {
		block.QuietDeclineExitReasons = slowDeclineReasons
	}
	return withBlock(block)
}

func assertNoSlowDeclineRow(t *testing.T, got Result, position string) {
	t.Helper()
	assertUntouched(t, got, position)
}

func TestQuietDeclineFixturesSitShortOfTheLastDepth(t *testing.T) {
	withQuietDecline(t, true)
	trade := watchedTrade()
	if watchedFills < quietdecline.ArmDepth || watchedFills >= lastDepthFills || capitalProtectionWatched(trade) || !quietDeclineWatched(trade) {
		t.Fatal("fixture drifted: the watched ladder must be watched and short of the w3s row's last depth")
	}
	if trade.PositionPrice != slowDeclineLastFill || !(armingPrice(trade) < slowDeclineLastFill) {
		t.Fatal("fixture drifted: the ladder's newest fill, and the print that arms its next depth under it")
	}
	if !(slowDeclineLastFill < underTheBand && underTheBand < slowDeclineBand && slowDeclineBand < ladder.AverageEntryPrice(trade)) {
		t.Fatal("fixture drifted: the two prints of the dead zone sit over the newest fill, the lower first, under break even")
	}
}

// A long parent ladder is watched from quietdecline.ArmDepth filled entries: the verdict's tick
// writes one row at the newest fill, at no loss that row alone, at a loss the activation beside it
// in buy and the sale in stopLoss; a shallower, unfilled, inverse or child ladder, or no flag, writes nothing.
func TestApplyWatchesTheQuietDeclineFromQuietDeclineArmDepth(t *testing.T) {
	withQuietDecline(t, true)
	trade := watchedTrade()
	got := applyAtAGain(trade, "", slowDeclineBlock(true))
	if got.Position != "" || got.Reason != "" {
		t.Fatalf("a watched ladder must go pending on the verdict without forcing, got %+v", got)
	}
	assertRow(t, got, quietPending(slowDeclineLastFill))
	assertNoSale(t, got, "")
	want := "smartTakeLoss: quiet decline triggered (quiet decline: leg down 7.0% from its high close, leg 60 bars long on 1h)"
	if got.Rows[0].Message != want {
		t.Fatalf("row = %q, want %q", got.Rows[0].Message, want)
	}
	for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
		price := printFor(trade, position)
		assertActivated(t, Apply(trade, position, price, slowDeclineBlock(true)), quietPending(slowDeclineLastFill), quietActivation(trade, price))
		assertSoldWith(t, Apply(restingIn(trade, "stopLoss"), position, price, slowDeclineBlock(true)), reasonSlowDecline, quietPending(slowDeclineLastFill))
	}

	// quietdecline.ArmDepth filled entries are enough and the row carries the newest; one fewer is
	// not watched on the verdict or on the smooth leg, and neither activates nor sells.
	shallowest := testutil.LadderTrade(false, fills(quietdecline.ArmDepth, "17:38:00")...)
	got = applyAtAGain(shallowest, "", slowDeclineBlock(true))
	assertRow(t, got, quietPending(shallowest.PositionPrice))
	assertNoSale(t, got, "")
	shallower := testutil.LadderTrade(false, fills(quietdecline.ArmDepth-1, "17:38:00")...)
	armed := armingPrice(watchedTrade())
	for _, block := range []aggragates.AIIndicators{slowDeclineBlock(true), quietLegBlock(false, testutil.At("17:00:00"), fillBarClosed)} {
		for _, state := range []string{"buy", "stopLoss"} {
			assertNoSlowDeclineRow(t, Apply(restingIn(shallower, state), "", underTheBand, block), "")
			assertNoSlowDeclineRow(t, Apply(restingIn(shallower, state), "stopLoss", armed, block), "stopLoss")
		}
	}
	noFill := testutil.LadderTrade(false)
	assertNoSlowDeclineRow(t, Apply(noFill, "", underTheBand, slowDeclineBlock(true)), "")

	inverse := testutil.LadderTrade(true, fills(watchedFills, "17:38:00")...)
	assertNoSlowDeclineRow(t, Apply(inverse, "", underTheBand, slowDeclineBlock(true)), "")

	child := watchedTrade()
	child.ParentID = 7
	assertNoSlowDeclineRow(t, Apply(child, "", slowDeclineBand, slowDeclineBlock(true)), "")

	off := watchedTrade()
	off.Strategy.Params.SmartTakeLoss = false
	assertNoSlowDeclineRow(t, Apply(off, "", slowDeclineBand, slowDeclineBlock(true)), "")

	// Without the verdict a watched ladder is the plain ladder, its arming
	// included.
	for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
		assertNoSlowDeclineRow(t, Apply(watchedTrade(), position, printFor(watchedTrade(), position), slowDeclineBlock(false)), position)
	}
}

// A futures ladder is not watched by the quiet decline (quietDeclineEligible, spot only): nothing is
// marked, activated or sold, no row written, ExitReached false and Armed false, since hermes' futures
// path never runs the overlay; the same ladder as spot is watched and acted on.
func TestApplyDoesNotWatchAFuturesLadderThroughTheQuietDecline(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	withEarlyTakeProfit(t, true)
	withCapitalProtectionExit(t, true)
	withFlatMarket(t, true)

	spot := watchedTrade()
	if st := rebuildState(spot); !st.quietDeclineWatched || !st.slowDeclineWatched || !quietDeclineEligible(spot) || !quietDeclineWatched(spot) || !Armed(spot) {
		t.Fatalf("control: a spot ladder is watched by the quiet decline, got %+v", st)
	}
	assertActivated(t, Apply(pendingTrade(), "", underTheBand, nothingServed()), quietActivation(pendingTrade(), underTheBand))
	assertSold(t, Apply(restingIn(pendingTrade(), "stopLoss"), "", underTheBand, nothingServed()), reasonSlowDecline)

	futures := watchedTrade()
	futures.Strategy.TradeType = aggragates.Futures
	atLastDepth := lastDepthLadder()
	atLastDepth.Strategy.TradeType = aggragates.Futures
	for name, trade := range map[string]aggragates.Trades{"at a watched depth": futures, "at its last depth": atLastDepth} {
		st := rebuildState(trade)
		if st.quietDeclineWatched || st.swingDeclineWatched || st.slowDeclineWatched || st.earlyTakeProfitWatched || st.capitalProtectionWatched || st.flatMarketWatched {
			t.Fatalf("a futures ladder %s is watched by no gate, got %+v", name, st)
		}
		if quietDeclineEligible(trade) || quietDeclineWatched(trade) || slowDeclineWatched(trade) || Armed(trade) {
			t.Fatalf("a futures ladder %s is neither eligible for the quiet decline nor armed", name)
		}
	}

	pending := withRows(futures, quietPending(slowDeclineLastFill))
	for _, trade := range []aggragates.Trades{futures, pending} {
		for _, state := range []string{"buy", "stopLoss", "forceTrailingStopLoss"} {
			resting := restingIn(trade, state)
			for _, price := range []float64{gainPrint(trade), slowDeclineBand, underTheBand, armingPrice(trade)} {
				for _, reading := range []aggragates.AIIndicators{slowDeclineBlock(true), slowDeclineBlock(false), nothingServed()} {
					for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
						assertUntouched(t, Apply(resting, position, price, reading), position)
					}
					if ExitReached(resting, price, reading.SmartTakeLoss) {
						t.Fatalf("state %q at %v: a futures ladder is never sold by the overlay, ExitReached must read false", state, price)
					}
				}
			}
		}
		if got := TakeProfitPercentage(trade, underTheBand, 1); got != 1 {
			t.Fatalf("a futures ladder's take profit reads the move it is handed, got %v", got)
		}
	}
	assertUntouched(t, Apply(atLastDepth, "", lastDepthPrint, withBlock(solBlock())), "")
}

// The row is written once: a pending ladder proposes no second one; at no loss the tick writes
// nothing, at a loss the activation alone, and stopLoss is sold with no row.
func TestApplyWritesTheQuietDeclineRowOnce(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingTrade()
	for _, on := range []bool{true, false} {
		assertNoSlowDeclineRow(t, applyAtAGain(trade, "", slowDeclineBlock(on)), "")
		assertActivated(t, Apply(trade, "", underTheBand, slowDeclineBlock(on)), quietActivation(trade, underTheBand))
		assertSold(t, Apply(restingIn(trade, "stopLoss"), "", underTheBand, slowDeclineBlock(on)), reasonSlowDecline)
	}
}

// A pending ladder whose newest fill is judged is acted on by its state and never its proposal,
// whatever is served: buy is activated at every print, a gain included (the activation row alone),
// a self-armed stopLoss is sold at any print, and an activated one trails (held at the
// activation print, re-anchored on a rise over the trail, sold under the tolerance).
func TestAPendingLadderIsActedOnByItsStateWhateverTheVerdictServes(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingTrade()
	armed := armingPrice(trade)
	committed := committedBy(trade, quietActivation(trade, armed))
	committed.PositionPrice = armed
	falls := priceAtMove(committed, armed, -(trailRow(committed).Tolerance + 1))
	for _, reading := range []aggragates.AIIndicators{slowDeclineBlock(true), slowDeclineBlock(false), nothingServed()} {
		for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
			for _, price := range []float64{armed, underTheBand, slowDeclineBand, slowDeclineBand + 5} {
				assertActivated(t, Apply(trade, position, price, reading), quietActivation(trade, price))
				assertSold(t, Apply(restingIn(trade, "stopLoss"), position, price, reading), reasonSlowDecline)
			}
			assertTrailHeld(t, Apply(committed, position, armed, reading))
			for _, price := range []float64{underTheBand, slowDeclineBand, slowDeclineBand + 5, gainPrint(trade)} {
				assertReanchored(t, Apply(committed, position, price, reading))
			}
			assertSold(t, Apply(committed, position, falls, reading), reasonSlowDecline)
			gain := gainPrint(trade)
			assertActivated(t, Apply(trade, position, gain, reading), quietActivation(trade, gain))
			assertSold(t, Apply(restingIn(trade, "stopLoss"), position, gain, reading), reasonSlowDecline)
		}
	}

	got := Apply(committed, "stopLoss", falls, slowDeclineBlock(false))
	want := fmt.Sprintf("smartTakeLoss: sell at slow decline %.2f (quiet decline)", falls)
	if msg := ExitMessage(trade, got.Reason, falls); msg != want {
		t.Fatalf("unexpected exit row %q, want %q", msg, want)
	}
}

// pastTheNewestFill reaches the take profit measured from pendingTrade's newest fill and sits under
// justUnderBreakEven, two prints of the dead zone.
const (
	pastTheNewestFill  = 189.20
	justUnderBreakEven = 191.0
)

// Under break even a pending ladder is acted on by its state alone: past the take profit from its
// newest fill nothing reads that fill (TakeProfitPercentage measures from the average entry), so the
// shared action activates buy and sells stopLoss, whatever the proposal and whatever is served.
func TestAPendingLadderUnderBreakEvenIsActedOnByItsStateAlone(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingTrade()
	breakEven := ladder.AverageEntryPrice(trade)
	underBreakEvenPastTheFill := math.Nextafter(justUnderBreakEven, 0)
	if !(justUnderBreakEven < breakEven) {
		t.Fatalf("fixture drifted: the print must sit under break even %v", breakEven)
	}
	readings := map[string]aggragates.AIIndicators{
		"nothing served": nothingServed(),
		"the verdict":    slowDeclineBlock(true),
		"a quiet leg":    quietLegBlock(false, testutil.At("17:00:00"), fillBarClosed),
	}
	for _, price := range []float64{pastTheNewestFill, underBreakEvenPastTheFill, armingPrice(trade)} {
		if price != armingPrice(trade) && !pastTheNewestFillUnderBreakEven(trade, price) {
			t.Fatalf("fixture drifted: %v must reach the take profit from the newest fill, under break even", price)
		}
		fromAverage := moveAgainst(price, breakEven)
		if got := TakeProfitPercentage(trade, price, fromAverage); got != fromAverage {
			t.Fatalf("at %v, under break even, the take profit must read the average entry price alone, got %v want %v", price, got, fromAverage)
		}
		for name, reading := range readings {
			for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
				if got := Apply(trade, position, price, reading); !got.Activated || got.Position != "stopLoss" || got.Reason != "" || !reflect.DeepEqual(got.Rows, []Row{quietActivation(trade, price)}) {
					t.Fatalf("%s at %v: a pending ladder in buy has its stopLoss activated in place of %q with the activation row alone, got %+v", name, price, position, got)
				}
				if sold := Apply(restingIn(trade, "stopLoss"), position, price, reading); sold.Position != "sellLoss" || sold.Reason != reasonSlowDecline || sold.Activated || len(sold.Rows) != 0 {
					t.Fatalf("%s at %v: a pending ladder in stopLoss is sold in place of %q with no row, got %+v", name, price, position, sold)
				}
			}
		}
	}
}

// The verdict can arrive on the arming tick: the row and the action on the same tick, buy activated
// with the arming replaced, stopLoss sold with the gate's row beside the sale; at a gain alike.
func TestApplyGoesPendingAndActsOnTheSameTick(t *testing.T) {
	withQuietDecline(t, true)
	trade := watchedTrade()
	for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
		price := printFor(trade, position)
		assertActivated(t, Apply(trade, position, price, slowDeclineBlock(true)), quietPending(slowDeclineLastFill), quietActivation(trade, price))
		assertSoldWith(t, Apply(restingIn(trade, "stopLoss"), position, price, slowDeclineBlock(true)), reasonSlowDecline, quietPending(slowDeclineLastFill))

		gain := gainPrint(trade)
		assertActivated(t, Apply(trade, position, gain, slowDeclineBlock(true)), quietPending(slowDeclineLastFill), quietActivation(trade, gain))
		assertSoldWith(t, Apply(restingIn(trade, "stopLoss"), position, gain, slowDeclineBlock(true)), reasonSlowDecline, quietPending(slowDeclineLastFill))
	}
}

// The take profit keeps working: a close the ladder decided is never replaced but a take profit
// proposed in buy, which the pending ladder's activation replaces, and a trade in its take profit
// still goes pending.
func TestApplyPendingLeavesTheClosesAloneAndActivatesInPlaceOfATakeProfit(t *testing.T) {
	withQuietDecline(t, true)
	price := slowDeclineBand + 5
	for _, position := range closingProposals {
		assertNoSlowDeclineRow(t, Apply(pendingTrade(), position, price, slowDeclineBlock(false)), position)
	}
	for _, position := range takeProfitProposals {
		assertActivated(t, Apply(pendingTrade(), position, price, slowDeclineBlock(false)), quietActivation(pendingTrade(), price))
	}
	for _, state := range []string{"takeProfit", "sellLoss", "forceTrailingTakeProfit"} {
		trade := pendingTrade()
		trade.PositionType = state
		assertNoSlowDeclineRow(t, Apply(trade, "", slowDeclineBand+5, slowDeclineBlock(false)), "")
	}

	trailing := watchedTrade()
	trailing.PositionType = "takeProfit"
	got := Apply(trailing, "", slowDeclineBand+5, slowDeclineBlock(true))
	if got.Position != "" || got.Reason != "" {
		t.Fatalf("a trailing take profit goes pending and keeps trailing, got %+v", got)
	}
	assertRow(t, got, quietPending(slowDeclineLastFill))
}

// Nothing waits after a fill once pending: an unstamped pending ladder is acted on like any other;
// an unstamped watched ladder does not go pending (the recent-fill rule fails closed) unless that rule is off.
func TestApplyUnstampedFillsAreStillActedOn(t *testing.T) {
	withQuietDecline(t, true)
	unstamped := pendingTrade()
	for index := range unstamped.History {
		unstamped.History[index].CreatedAt = time.Time{}
	}
	armedPending := armingPrice(unstamped)
	assertActivated(t, Apply(unstamped, "stopLoss", armedPending, slowDeclineBlock(true)), quietActivation(unstamped, armedPending))
	assertSold(t, Apply(restingIn(unstamped, "stopLoss"), "stopLoss", armedPending, slowDeclineBlock(true)), reasonSlowDecline)

	fresh := watchedTrade()
	for index := range fresh.History {
		fresh.History[index].CreatedAt = time.Time{}
	}
	armed := armingPrice(fresh)
	for _, state := range []string{"buy", "stopLoss"} {
		assertNoSlowDeclineRow(t, Apply(restingIn(fresh, state), "stopLoss", armed, slowDeclineBlock(true)), "stopLoss")
	}
	withRecentFillRule(t, false)
	pending := quietPending(slowDeclineLastFill)
	assertActivated(t, Apply(fresh, "stopLoss", armed, slowDeclineBlock(true)), pending, quietActivation(fresh, armed))
	assertSoldWith(t, Apply(restingIn(fresh, "stopLoss"), "stopLoss", armed, slowDeclineBlock(true)), reasonSlowDecline, pending)
}

// With the trigger taking any loss the action reads no price: buy is activated at the arming print,
// a hair under and far under alike, stopLoss sold at all of them.
func TestTheActionReadsNoPriceWhileTheTriggerTakesAnyLoss(t *testing.T) {
	withQuietDecline(t, true)
	withAllowedLoss(t, anyLoss)
	withSellLossIgnoreLastDepths(t, 0)
	armed := armingPrice(pendingTrade())
	hair := math.Nextafter(armed, 0)
	for _, on := range []bool{true, false} {
		for _, price := range []float64{armed, hair, armed / 2} {
			for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
				assertActivated(t, Apply(pendingTrade(), position, price, slowDeclineBlock(on)), quietActivation(pendingTrade(), price))
				assertSold(t, Apply(restingIn(pendingTrade(), "stopLoss"), position, price, slowDeclineBlock(on)), reasonSlowDecline)
			}
		}
	}

	pending := quietPending(slowDeclineLastFill)
	for _, position := range []string{"", "stopLoss"} {
		assertActivated(t, Apply(watchedTrade(), position, hair, slowDeclineBlock(true)), pending, quietActivation(watchedTrade(), hair))
		assertSoldWith(t, Apply(restingIn(watchedTrade(), "stopLoss"), position, hair, slowDeclineBlock(true)), reasonSlowDecline, pending)
	}
}

// A stopLoss whose armed depth is about to fill has the buy replaced by the sale (the action reads
// the state), the force-trailing state too; the one buy left to fill is a self-armed one the cap
// holds, with the held row written once and the new fill judged next tick. An activated ladder is
// sold in place of the buy at any loss.
func TestApplyTurnsTheBuyOfAStopLossLadderIntoTheSaleUnlessTheCapHoldsIt(t *testing.T) {
	withQuietDecline(t, true)
	for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
		trade := restingIn(pendingTrade(), state)
		armed := armingPrice(trade)
		for _, price := range []float64{armed, underTheBand, slowDeclineBand + 1, gainPrint(trade)} {
			for _, reading := range []aggragates.AIIndicators{slowDeclineBlock(false), slowDeclineBlock(true)} {
				for _, position := range []string{"buy", "update_buy", "update_stopLoss"} {
					assertSold(t, Apply(trade, position, price, reading), reasonSlowDecline)
				}
			}
		}
	}

	t.Run("over the cap", func(t *testing.T) {
		underTheShippedCap(t)
		for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
			trade := restingIn(pendingTrade(), state)
			armed, loss := armingOverTheCap(t, trade)
			within, _ := firstPriceWithinTheCap(t, trade, armed)
			held := SaleHeldRow(slowDeclineLastFill, -loss, FeatureQuietDecline)
			committed := committedBy(trade, quietActivation(trade, within))
			for _, reading := range []aggragates.AIIndicators{slowDeclineBlock(false), slowDeclineBlock(true)} {
				for _, position := range []string{"buy", "update_buy", "update_stopLoss"} {
					assertWaits(t, Apply(trade, position, armed, reading), position, held)
					assertUntouched(t, Apply(withRows(trade, held), position, armed, reading), position)
					assertSold(t, Apply(trade, position, within, reading), reasonSlowDecline)
					assertSold(t, Apply(committed, position, armed, reading), reasonSlowDecline)
				}
			}
		}
	})
}

// The watch counts filled entries as Armed does (ladder.CountFilledEntries), not history rows: an
// accounting row, an empty row, an exit-side row and a partial fill add no depth; a legacy row
// without an order id is an entry of its own.
func TestQuietDeclineWatchCountsFilledEntriesNotRows(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, false)
	withEarlyTakeProfit(t, false)
	withFlatMarket(t, false)
	withCapitalProtectionExit(t, false)
	trade := testutil.LadderTrade(false, fills(quietdecline.ArmDepth-1, "17:38:00")...)
	trade.History = append(trade.History,
		aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: ladder.AccountingPriceCeiling, OrderId: 90},
		aggragates.TradesHistory{Type: "BUY", Quantity: 0, Price: 180, OrderId: 91},
		aggragates.TradesHistory{Type: "SELL", Quantity: 1, Price: 190, OrderId: 92},
		aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: trade.PositionPrice, OrderId: trade.History[0].OrderId},
	)
	if ladder.CountFilledEntries(trade) != quietdecline.ArmDepth-1 || len(trade.History) < quietdecline.ArmDepth {
		t.Fatal("fixture drifted: one entry short of the watch, with as many history rows as it needs")
	}
	if Armed(trade) || rebuildState(trade).quietDeclineWatched {
		t.Fatalf("rows that add no depth must not watch the ladder (Armed %v)", Armed(trade))
	}
	assertNoSlowDeclineRow(t, Apply(trade, "", slowDeclineBand, slowDeclineBlock(true)), "")

	legacy := aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: 179.78, CreatedAt: testutil.At("17:40:00")}
	trade.History = append(trade.History, legacy)
	if ladder.CountFilledEntries(trade) != quietdecline.ArmDepth || !Armed(trade) || !rebuildState(trade).quietDeclineWatched {
		t.Fatalf("the entry that reaches quietdecline.ArmDepth must watch the ladder for the tick gate and the exit alike (Armed %v)", Armed(trade))
	}
	assertRow(t, applyAtAGain(trade, "", slowDeclineBlock(true)), quietPending(legacy.Price))
}

// Switched off, the quiet decline watches no ladder: nothing marked, judged, activated, sold or
// armed, the sibling features off beside it; switched back on, the same ladders are marked and acted on.
func TestApplyQuietDeclineSwitchedOff(t *testing.T) {
	withQuietDecline(t, false)
	withEarlyTakeProfit(t, false)
	withSwingDecline(t, false)
	withFlatMarket(t, false)
	withFlatMarket(t, false)
	for _, state := range []string{"buy", "stopLoss"} {
		for _, position := range []string{"", "stopLoss"} {
			assertNoSlowDeclineRow(t, Apply(restingIn(watchedTrade(), state), position, printFor(watchedTrade(), position), slowDeclineBlock(true)), position)
			assertNoSlowDeclineRow(t, Apply(restingIn(pendingTrade(), state), position, printFor(pendingTrade(), position), slowDeclineBlock(true)), position)
			assertNoSlowDeclineRow(t, Apply(restingIn(pendingAtFive(), state), position, printFor(pendingAtFive(), position), brokenReading()), position)
		}
	}
	if Armed(watchedTrade()) || Armed(pendingTrade()) {
		t.Fatal("switched off, a ladder short of its last depth is not armed")
	}

	withQuietDecline(t, true)
	assertRow(t, applyAtAGain(watchedTrade(), "", slowDeclineBlock(true)), quietPending(slowDeclineLastFill))
	armed := armingPrice(pendingTrade())
	assertActivated(t, Apply(pendingTrade(), "stopLoss", armed, slowDeclineBlock(true)), quietActivation(pendingTrade(), armed))
	assertSold(t, Apply(restingIn(pendingTrade(), "stopLoss"), "stopLoss", armed, slowDeclineBlock(true)), reasonSlowDecline)
	assertNoSlowDeclineRow(t, applyAtAGain(pendingTrade(), "", slowDeclineBlock(true)), "")
}

// fillBarClosed is a FillBefore under which watchedTrade's newest fill counts: the open of the bar after the fill's.
var fillBarClosed = testutil.At("18:00:00")

// quietLegBlock is the block sophos serves for a leg on and quiet, smooth from the bar opening at
// smoothFrom, a fill counted when stamped before fillBefore, the verdict given apart; a zero time serves zero.
func quietLegBlock(verdict bool, smoothFrom, fillBefore time.Time) aggragates.AIIndicators {
	block := withEveryRowTrend(aggragates.SmartTakeLossIndicators{
		QuietDeclineVerdict:     verdict,
		QuietDeclineLegQuiet:    true,
		QuietDeclineExitReasons: slowDeclineReasons,
		QuietDeclineFillFrom:    fillWindowFrom.UnixMilli(),
	})
	if !smoothFrom.IsZero() {
		block.QuietDeclineSmoothFrom = smoothFrom.UnixMilli()
	}
	if !fillBefore.IsZero() {
		block.QuietDeclineFillBefore = fillBefore.UnixMilli()
	}
	return withBlock(block)
}

// A ladder's smoothness is counted from its newest fill: a leg smooth from the fill's bar or earlier
// (or from the fill's very stamp) marks the ladder pending once the bars after the fill's bar stand
// (fillBarClosed); smooth only from a later bar, or a millisecond after the fill, marks nothing.
func TestApplyCountsTheSmoothLegFromTheNewestFill(t *testing.T) {
	withQuietDecline(t, true)
	newest := testutil.At("17:38:00")
	if watchedTrade().History[watchedFills-1].CreatedAt != newest {
		t.Fatal("fixture drifted: the watched ladder's newest fill")
	}
	for name, smoothFrom := range map[string]time.Time{
		"the bar holding the fill": testutil.At("17:00:00"),
		"an earlier bar":           testutil.At("09:00:00"),
		"the fill's own stamp":     newest,
	} {
		got := applyAtAGain(watchedTrade(), "", quietLegBlock(false, smoothFrom, fillBarClosed))
		if got.Position != "" || got.Reason != "" {
			t.Fatalf("smooth from %s: the ladder must go pending without forcing, got %+v", name, got)
		}
		assertRow(t, got, quietPending(slowDeclineLastFill))
	}

	for name, smoothFrom := range map[string]time.Time{
		"the bar after the fill":         testutil.At("18:00:00"),
		"a millisecond after the fill":   newest.Add(time.Millisecond),
		"no bar: the leg has not fallen": {},
	} {
		assertNoSlowDeclineRow(t, Apply(watchedTrade(), "", underTheBand, quietLegBlock(false, smoothFrom, fillBarClosed)), "")
		if name == "no bar: the leg has not fallen" {
			continue
		}
		assertRow(t, applyAtAGain(watchedTrade(), "", quietLegBlock(true, smoothFrom, fillBarClosed)), quietPending(slowDeclineLastFill))
	}
}

// The count needs the leg on and quiet and a stamp to count from: a smooth bar without the quiet leg,
// or without FillBefore, marks nothing, an unstamped newest fill is marked on nothing, and a pending
// ladder writes no second row.
func TestApplySmoothFromTheNewestFillFailsClosed(t *testing.T) {
	withQuietDecline(t, true)
	early := testutil.At("09:00:00")
	notQuiet := withBlock(aggragates.SmartTakeLossIndicators{
		QuietDeclineSmoothFrom:  early.UnixMilli(),
		QuietDeclineFillBefore:  fillBarClosed.UnixMilli(),
		QuietDeclineExitReasons: slowDeclineReasons,
		QuietDeclineFillFrom:    fillWindowFrom.UnixMilli(),
	})
	assertNoSlowDeclineRow(t, Apply(watchedTrade(), "", underTheBand, notQuiet), "")
	assertNoSlowDeclineRow(t, Apply(watchedTrade(), "", underTheBand, quietLegBlock(false, early, time.Time{})), "")

	unstamped := watchedTrade()
	for index := range unstamped.History {
		unstamped.History[index].CreatedAt = time.Time{}
	}
	assertNoSlowDeclineRow(t, Apply(unstamped, "", underTheBand, quietLegBlock(false, early, fillBarClosed)), "")
	assertNoSlowDeclineRow(t, Apply(unstamped, "", underTheBand, quietLegBlock(true, early, fillBarClosed)), "")

	assertNoSlowDeclineRow(t, applyAtAGain(pendingTrade(), "", quietLegBlock(false, early, fillBarClosed)), "")
	assertActivated(t, Apply(pendingTrade(), "", underTheBand, quietLegBlock(false, early, fillBarClosed)), quietActivation(pendingTrade(), underTheBand))
}

// Only the newest fill counts: earlier fills before the smooth bar and the newest after it go
// pending, the same ladder one depth shorter does not.
func TestApplySmoothFromReadsOnlyTheNewestFill(t *testing.T) {
	withQuietDecline(t, true)
	smoothFrom := testutil.At("12:00:00")
	deep := testutil.LadderTrade(false, fills(watchedFills, "17:38:00")...)
	shallow := testutil.LadderTrade(false, fills(watchedFills-1, "08:30:00")...)
	if deep.History[watchedFills-2].CreatedAt.After(smoothFrom) || shallow.History[watchedFills-2].CreatedAt.After(smoothFrom) {
		t.Fatal("fixture drifted: every fill but the deep ladder's newest must come before the smooth bar")
	}
	if !quietDeclineWatched(shallow) {
		t.Fatal("fixture drifted: the shorter ladder must still be watched")
	}
	assertRow(t, applyAtAGain(deep, "", quietLegBlock(false, smoothFrom, fillBarClosed)), quietPending(deep.PositionPrice))
	assertNoSlowDeclineRow(t, Apply(shallow, "", underTheBand, quietLegBlock(false, smoothFrom, fillBarClosed)), "")
}

// QuietDeclineSmoothFrom is a bar's open in ms and the fill's stamp is compared in ms: a fill at the
// open or inside its first millisecond counts, one a nanosecond before the open does not.
func TestApplySmoothFromAtTheBarOpen(t *testing.T) {
	withQuietDecline(t, true)
	open := testutil.At("17:00:00")
	for name, tc := range map[string]struct {
		stamp   time.Time
		pending bool
	}{
		"at the open":                      {open, true},
		"inside the open's millisecond":    {open.Add(time.Millisecond - time.Nanosecond), true},
		"a millisecond before the open":    {open.Add(-time.Millisecond), false},
		"a nanosecond before the open":     {open.Add(-time.Nanosecond), false},
		"the last bar before, at its open": {open.Add(-time.Hour), false},
	} {
		trade := watchedTrade()
		trade.History[watchedFills-1].CreatedAt = tc.stamp
		if trade.History[watchedFills-2].CreatedAt.After(tc.stamp) {
			t.Fatalf("%s: fixture drifted: the stamp must stay the newest", name)
		}
		got := applyAtAGain(trade, "", quietLegBlock(false, open, fillBarClosed))
		if !tc.pending {
			assertNoSlowDeclineRow(t, got, "")
			continue
		}
		if got.Position != "" || got.Reason != "" {
			t.Fatalf("%s: the ladder must go pending at its newest fill without forcing, got %+v", name, got)
		}
		assertRow(t, got, quietPending(slowDeclineLastFill))
	}
}

// The count waits for the closed bars after the fill's bar: the fill must be stamped strictly before
// QuietDeclineFillBefore (counts at the next bar's open and a millisecond after the stamp, not at its
// own bar's open or its stamp); the verdict does not wait.
func TestApplyCountsTheNewestFillOnlyWithTheClosedBarsAfterIt(t *testing.T) {
	withQuietDecline(t, true)
	newest := testutil.At("17:38:00")
	smoothFrom := testutil.At("17:00:00")
	for name, tc := range map[string]struct {
		fillBefore time.Time
		pending    bool
	}{
		"the bar after the fill's bar":  {fillBarClosed, true},
		"a millisecond after the stamp": {newest.Add(time.Millisecond), true},
		"the fill's own stamp":          {newest, false},
		"the open of the fill's bar":    {smoothFrom, false},
		"a day before the fill":         {smoothFrom.Add(-24 * time.Hour), false},
	} {
		got := applyAtAGain(watchedTrade(), "", quietLegBlock(false, smoothFrom, tc.fillBefore))
		if !tc.pending {
			assertNoSlowDeclineRow(t, got, "")
			assertRow(t, applyAtAGain(watchedTrade(), "", quietLegBlock(true, smoothFrom, tc.fillBefore)), quietPending(slowDeclineLastFill))
			continue
		}
		if got.Position != "" || got.Reason != "" {
			t.Fatalf("fill before %s: the ladder must go pending at its newest fill without forcing, got %+v", name, got)
		}
		assertRow(t, got, quietPending(slowDeclineLastFill))
	}
}

// A fill in the forming bar does not count yet: with no closed bar after it (SlowDeclineMinBarsAfterFill
// at none) a fresh fill on a quiet leg without the verdict goes pending on no tick, activates and sells nothing.
func TestApplyAFreshFillOnAQuietLegNeitherGoesPendingNorSells(t *testing.T) {
	withQuietDecline(t, true)
	fresh := testutil.At("17:38:00")
	// The fill prints in the forming bar: the last closed bar is the one before it, and FillBefore is the forming bar's open.
	lastClosed := testutil.At("16:00:00")
	block := quietLegBlock(false, lastClosed, testutil.At("17:00:00"))
	for _, depth := range []int{1, 2, quietdecline.ArmDepth, watchedFills} {
		trade := testutil.LadderTrade(false, fills(depth, "17:38:00")...)
		if trade.History[depth-1].CreatedAt != fresh {
			t.Fatalf("%d fills: fixture drifted: the newest fill must be the fresh one", depth)
		}
		for _, state := range []string{"buy", "stopLoss"} {
			resting := restingIn(trade, state)
			for _, price := range []float64{slowDeclineBand, slowDeclineBand + 5} {
				assertNoSlowDeclineRow(t, Apply(resting, "", price, block), "")
			}
			for _, position := range armingProposals {
				assertNoSlowDeclineRow(t, Apply(resting, position, armingPrice(watchedTrade()), block), position)
			}
		}
	}
}
