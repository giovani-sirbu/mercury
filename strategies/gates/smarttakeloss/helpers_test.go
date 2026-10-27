package smarttakeloss

import (
	"encoding/json"
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"math"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/capitalprotection"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/flatmarket"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/freefallprevention"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/lineardecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/quietdecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// lastDepthFills is the w3s row's configured Depths: the w3s ladder holding
// that many fills is at its last depth, the one capital protection watches.
const lastDepthFills = 8

// lastDepthPrint is a print of lastDepthLadder's dead zone: over its newest fill, under its break
// even, a loss over capital protection's drawdown, so the gate triggers and waits there.
const lastDepthPrint = 172.40

// fills is the w3s ladder n deep, the first n−1 fills a quarter hour apart from 08:00 and the newest at
// lastClock; a ladder zero deep holds only the bookkeeping row, so it is unwatched and History[0] still exists.
func fills(n int, lastClock string) []testutil.LadderFill {
	if n <= 0 {
		return []testutil.LadderFill{{Price: ladder.AccountingPriceCeiling, At: testutil.At(lastClock)}}
	}
	clocks := make([]string, n)
	for index := range clocks {
		clocks[index] = fmt.Sprintf("%02d:%02d:00", 8+index/4, (index%4)*15)
	}
	clocks[n-1] = lastClock
	return testutil.W3sFills(clocks...)
}

// lastDepthLadder is the w3s ladder at its last configured depth, its next entry refused for funds.
func lastDepthLadder() aggragates.Trades {
	return fundsRefused(testutil.LadderTrade(false, fills(lastDepthFills, "21:30:00")...))
}

// fundsRefused is the trade with the insufficient funds error its next entry logs, stamped at its
// newest fill: from it capital protection watches a ladder at its last depth.
func fundsRefused(trade aggragates.Trades) aggragates.Trades {
	var at time.Time
	if fills := entryFills(trade); len(fills) > 0 {
		at = fills[len(fills)-1].At
	}
	trade.Logs = append(trade.Logs[:len(trade.Logs):len(trade.Logs)], aggragates.TradesLogs{
		Type:      aggragates.LOG_ERROR,
		Message:   "Insufficient funds (12.500000 USDT) for the requested action (stopLoss). You need at least 400.000000 USDT to resume this trade.",
		Price:     trade.PositionPrice,
		CreatedAt: at,
	})
	return trade
}

// solBlock is the capital protection reading sophos serves over the SOL ladder: the SMC trend
// bearish on every timeframe, no slow decline reading.
func solBlock() aggragates.SmartTakeLossIndicators {
	return aggragates.SmartTakeLossIndicators{CapitalProtectionSmcBearish: true}
}

// capitalProtectionTriggered is the triggered row capital protection opens the trade's episode
// with: at its newest fill, naming its depths and the trend on its side.
func capitalProtectionTriggered(trade aggragates.Trades) Row {
	return CapitalProtectionTriggeredRow(trade.PositionPrice, ladder.CountFilledEntries(trade), ladder.ConfiguredDepths(trade), trade.Inverse)
}

// capitalProtectionLadder is the round ladder (flatMarketLadder) at its last depth: its row's
// Depths lowered to its fills, long or inverse, resting in buy, its next entry refused for funds.
func capitalProtectionLadder(inverse bool) aggragates.Trades {
	trade := flatMarketLadder(inverse)
	trade.StrategyPair.StrategySettings[0].Depths = float64(len(roundPrices))
	return fundsRefused(trade)
}

// drawdownPrices are the prices at which the long round ladder loses exactly capital protection's
// drawdown and one cent more; it skips the test when the drawdown names no price on the cent grid.
func drawdownPrices(t *testing.T, trade aggragates.Trades) (atTheDrawdown, oneStepLower float64) {
	t.Helper()
	atTheDrawdown, oneStepLower, exact := capBoundary(ladder.EntryNotional(trade), ladder.EntryQuantity(trade), capitalProtectionMaxDrawdown)
	if !exact {
		t.Skip("capitalprotection.MaxDrawdownPercentage takes no loss or names no price above zero on the cent grid the round ladder loses exactly that much at")
	}
	return atTheDrawdown, oneStepLower
}

// firstPriceWithinTheDrawdown is the lowest grid price from the long ladder's newest fill up to its
// break even within capital protection's drawdown, and the cent under it; it skips the test when
// the newest fill's print is already within it.
func firstPriceWithinTheDrawdown(t *testing.T, trade aggragates.Trades) (within, oneStepLower float64) {
	t.Helper()
	newest := int(math.Round(trade.PositionPrice * 100))
	for cents := newest; float64(cents)/100 <= ladder.AverageEntryPrice(trade); cents++ {
		if handLossPct(trade, float64(cents)/100) <= capitalProtectionMaxDrawdown {
			if cents == newest {
				t.Skip("capitalprotection.MaxDrawdownPercentage is wide enough to let the newest fill's print be acted on")
			}
			return float64(cents) / 100, float64(cents-1) / 100
		}
	}
	t.Fatal("fixture drifted: no price up to break even is within the drawdown")
	return 0, 0
}

// gateEvents lists the trade's events under gate in the order they were written, each as the fold
// reads it (readEvent): its kind and its price.
func gateEvents(t *testing.T, trade aggragates.Trades, gate string) []string {
	t.Helper()
	var listed []string
	for _, event := range trade.StrategyEventsOf(aggragates.StrategyParamSmartTakeLoss, gate) {
		var data EventData
		if err := event.DecodeData(&data); err != nil {
			t.Fatalf("event %+v does not decode: %v", event, err)
		}
		listed = append(listed, eventAt(data.Event, data.Price))
	}
	return listed
}

func withBlock(block aggragates.SmartTakeLossIndicators) aggragates.AIIndicators {
	return aggragates.AIIndicators{SmartTakeLoss: block}
}

// quietFinding, swingFinding and indecisionFinding are the findings of one feature each, what the row builders take.
func quietFinding(reasons []string) Finding {
	return Finding{Feature: FeatureQuietDecline, Reasons: reasons}
}

func swingFinding(reasons []string) Finding {
	return Finding{Feature: FeatureSwingDecline, Reasons: reasons}
}

func indecisionFinding(reasons []string) Finding {
	return Finding{Feature: FeatureIndecision, Reasons: reasons}
}

// assertUntouched fails unless Apply handed the proposal back untouched: no sale, no activation
// (Result.Activated tells it from a proposed stopLoss) and no row.
func assertUntouched(t *testing.T, got Result, position string) {
	t.Helper()
	if got.Position != position || got.Reason != "" || got.Activated || len(got.Rows) != 0 {
		t.Fatalf("expected %q untouched with no row, got %+v", position, got)
	}
}

// assertRow fails unless Apply handed back exactly this one row, whole: the
// message, the price, the filing, the features and the reasons.
func assertRow(t *testing.T, got Result, want Row) {
	t.Helper()
	if len(got.Rows) != 1 || !reflect.DeepEqual(got.Rows[0], want) {
		t.Fatalf("rows = %+v, want exactly %+v", got.Rows, want)
	}
}

// assertNoSale is the answer of a ladder the gate neither sells nor activates:
// the proposal untouched, no reason, and the stopLoss not activated.
func assertNoSale(t *testing.T, got Result, position string) {
	t.Helper()
	if got.Position != position || got.Reason != "" || got.Activated {
		t.Fatalf("expected %q with no sale and no activation, got %+v", position, got)
	}
}

// assertForced fails unless Apply replaced the proposal with the sellLoss
// chain under reason.
func assertForced(t *testing.T, got Result, reason string) {
	t.Helper()
	if got.Position != "sellLoss" || got.Reason != reason || got.Activated {
		t.Fatalf("expected a forced sellLoss for %q, got %+v", reason, got)
	}
}

// assertSoldWith is assertForced with exactly these rows beside the sale, the gates' own rows of the tick.
func assertSoldWith(t *testing.T, got Result, reason string, rows ...Row) {
	t.Helper()
	assertForced(t, got, reason)
	assertRows(t, got, rows...)
}

// assertSold is assertSoldWith with no row at all.
func assertSold(t *testing.T, got Result, reason string) {
	t.Helper()
	assertSoldWith(t, got, reason)
}

// assertActivated fails unless Apply replaced the proposal with the activation: Position "stopLoss",
// Result.Activated, no Reason, and exactly these rows in this order.
func assertActivated(t *testing.T, got Result, rows ...Row) {
	t.Helper()
	if got.Position != "stopLoss" || !got.Activated || got.Reason != "" {
		t.Fatalf("expected the stopLoss activated with no sale, got %+v", got)
	}
	assertRows(t, got, rows...)
}

// assertWaits is the answer of a ladder whose action a bound refuses: the proposal
// untouched, nothing sold or activated, and exactly these rows, the held rows.
func assertWaits(t *testing.T, got Result, position string, rows ...Row) {
	t.Helper()
	assertNoSale(t, got, position)
	assertRows(t, got, rows...)
}

// indecisionReasons is what sophos names beside the indecision reading: the
// readings that do not hold, then the count one short of the need.
var indecisionReasons = []string{"NATR(14) 1.07x its median sampled every 24 bars, over 1.00x", "base volume of the last 24 bars 1.09x the median of the 720 before, over 0.90x", "2 of 4 readings hold with a smooth ladder, 3 needed"}

// indecisionReading is the indecision reading sophos serves: the window read beside what broke, the vote one short.
func indecisionReading() aggragates.AIIndicators {
	return withBlock(withEveryRowTrend(aggragates.SmartTakeLossIndicators{
		QuietDeclineFillFrom:     fillWindowFrom.UnixMilli(),
		QuietDeclineBreakReasons: indecisionReasons,
		Indecision:               true,
	}))
}

// latchRow is the row the engine wrote for a ladder the indecision latched at
// its newest fill on the sophos reading: the latch's own row.
func latchRow(trade aggragates.Trades) Row {
	return LatchedRow(rebuildState(trade).lastFill().Price, indecisionFinding(indecisionReasons))
}

// latchedBy is the trade carrying the latched pair the engine wrote for its
// newest fill, after the rows and events it already carries.
func latchedBy(trade aggragates.Trades) aggragates.Trades {
	return withRow(trade, latchRow(trade), testutil.At("21:45:00"))
}

// withRow is the trade with one more row, written the way the engines write
// the rows Apply hands back: the log row and its event, one stamp.
func withRow(trade aggragates.Trades, row Row, at time.Time) aggragates.Trades {
	return Append(trade, row, at)
}

// withRows is the trade with these rows written after the ones it carries, unstamped.
func withRows(trade aggragates.Trades, rows ...Row) aggragates.Trades {
	for _, row := range rows {
		trade = withRow(trade, row, time.Time{})
	}
	return trade
}

// restingIn is the trade resting in state (trade.PositionType), which the shared action decides by;
// the fixture ladders rest in buy.
func restingIn(trade aggragates.Trades, state string) aggragates.Trades {
	trade.PositionType = state
	return trade
}

// committedBy is the trade as the tick that activated the stopLoss leaves it: these activation rows
// written with their events, resting in stopLoss.
func committedBy(trade aggragates.Trades, rows ...Row) aggragates.Trades {
	return restingIn(withRows(trade, rows...), "stopLoss")
}

// engineStep is one engine tick that also leaves the state its chain leaves, so the next tick
// rebuilds from the trade alone: every row written with its event, and stopLoss at the tick price
// after an activation. Nothing is written for a sale (the sold row is the engines' ExitRow).
func engineStep(trade aggragates.Trades, position string, price float64, now time.Time, ai aggragates.AIIndicators) (aggragates.Trades, Result) {
	got := Apply(trade, position, price, ai)
	for _, row := range got.Rows {
		trade = withRow(trade, row, now)
	}
	if got.Activated {
		trade.PositionType, trade.PositionPrice = got.Position, price
	}
	return trade, got
}

// withEvents is the trade carrying these strategy events after the ones it carries, with no row beside them.
func withEvents(trade aggragates.Trades, events ...aggragates.TradesStrategyEvents) aggragates.Trades {
	trade.StrategyEvents = append(append([]aggragates.TradesStrategyEvents(nil), trade.StrategyEvents...), events...)
	return trade
}

// eventOf is an unstamped smartTakeLoss event of gate whose document is exactly this JSON.
func eventOf(gate, document string) aggragates.TradesStrategyEvents {
	return aggragates.TradesStrategyEvents{
		Param: aggragates.StrategyParamSmartTakeLoss,
		Gate:  gate,
		Data:  json.RawMessage(document),
	}
}

// legacyDocument is the document the gates before the merge wrote for a kind at
// a price: no features, no reasons.
func legacyDocument(kind string, price float64) string {
	return fmt.Sprintf(`{"event":%q,"price":%v}`, kind, price)
}

// carryingTheStateOf is the trade wearing the log rows and the strategy events of source, copied together.
func carryingTheStateOf(trade, source aggragates.Trades) aggragates.Trades {
	trade.Logs = source.Logs
	trade.StrategyEvents = source.StrategyEvents
	return trade
}

// withIndecision runs the test with the early take profit switched on or off and puts the switch back after.
func withEarlyTakeProfit(t *testing.T, on bool) {
	t.Helper()
	previous := earlyTakeProfit
	earlyTakeProfit = on
	t.Cleanup(func() { earlyTakeProfit = previous })
}

// withQuietDecline is withIndecision for the quiet decline.
func withQuietDecline(t *testing.T, on bool) {
	t.Helper()
	previous := quietDecline
	quietDecline = on
	t.Cleanup(func() { quietDecline = previous })
}

// withSwingDecline is withIndecision for the swing decline.
func withSwingDecline(t *testing.T, on bool) {
	t.Helper()
	previous := swingDecline
	swingDecline = on
	t.Cleanup(func() { swingDecline = previous })
}

// withCapitalProtectionExit is withIndecision for the capital protection gate.
func withCapitalProtectionExit(t *testing.T, on bool) {
	t.Helper()
	previous := capitalProtectionExit
	capitalProtectionExit = on
	t.Cleanup(func() { capitalProtectionExit = previous })
}

// withCapitalProtectionMaxDrawdown runs the test with capital protection's trigger at a loss of pct
// percent of the used amount and puts the package's value back after; package state, so never in parallel.
func withCapitalProtectionMaxDrawdown(t *testing.T, pct float64) {
	t.Helper()
	previous := capitalProtectionMaxDrawdown
	capitalProtectionMaxDrawdown = pct
	t.Cleanup(func() { capitalProtectionMaxDrawdown = previous })
}

// capitalProtectionAlone runs the test with capital protection on and every other gate off, the
// shared action's bounds as TestMain leaves them.
func capitalProtectionAlone(t *testing.T) {
	t.Helper()
	withCapitalProtectionExit(t, true)
	withQuietDecline(t, false)
	withSwingDecline(t, false)
	withEarlyTakeProfit(t, false)
	withFlatMarket(t, false)
}

// withRecentFillRule is withIndecision for the recent-fill rule.
func withRecentFillRule(t *testing.T, on bool) {
	t.Helper()
	previous := quietDeclineNeedsRecentFill
	quietDeclineNeedsRecentFill = on
	t.Cleanup(func() { quietDeclineNeedsRecentFill = previous })
}

// TestMain runs every test on the profile the fixtures were written against (pinTheProfile), whatever
// the constants say: they are hand-tuned. The tests of each bound and switch move it for themselves
// (withAllowedLoss, withSellLossIgnoreLastDepths, withSellLossStartAtDepth, withEarlyTakeProfitDepthPriority,
// withEarlyTakeProfitDynamicParams, withCapitalProtectionMaxDrawdown, withWaitAfterTrigger, ...).
func TestMain(m *testing.M) {
	pinTheProfile()
	os.Exit(m.Run())
}

// pin sets a leaf constant and the mirror the package reads it through (read at init) to one value.
func pin[T any](leaf, mirror *T, value T) {
	*leaf, *mirror = value, value
}

// pinTheProfile sets every knob the fixtures lean on, and the mirrors of the switches, to the profile
// they were written for. The shared action takes a loss of the whole used amount (anyLoss) with its
// depth bound, activation floor and wait off, so the ladders are acted on at the prints the tests
// read; a depth priority hold pauses the gates. The quiet decline watches from one fill, the early
// take profit from two and the flat market from one; the quiet and swing declines are on, the
// linear decline's own sale and the free fall prevention gate are off (their tests switch them on),
// and the early take profit latches on the swing and linear lines, the flat market and the free fall
// prevention, not on the quiet decline, a depth priority hold or a raised ladder. Capital protection
// is off, its drawdown at one percent: its tests switch it on (capitalProtectionAlone), and a ladder at
// its last depth would otherwise be watched through the funds refusal in its rows. The first-fill
// holds and the reset at a new depth are on. Values derived from a knob are set again as their
// definitions read them, and so are the test variables built on them at init (rederive).
func pinTheProfile() {
	sellLossTriggers = []SellLossTrigger{{MinTrendDirection: 1, Percentage: -anyLoss}}
	sellLossIgnoreLastDepths = 0
	sellLossStartAtDepth = 0
	waitAfterTrigger = 0
	activationConfirmation = 0
	activationConfirmationWindow = ActivationConfirmationWindow
	saleGapTolerances = 0
	saleGapWait = SaleGapWait
	pauseWhenActiveDepthPriority = true
	pin(&capitalprotection.Enabled, &capitalProtectionExit, false)
	pin(&capitalprotection.MaxDrawdownPercentage, &capitalProtectionMaxDrawdown, 1.0)
	pin(&capitalprotection.WaitAfterTrigger, &capitalProtectionWaitAfterTrigger, 0)

	pin(&quietdecline.Enabled, &quietDecline, true)
	pin(&quietdecline.NeedsRecentFill, &quietDeclineNeedsRecentFill, true)
	quietdecline.ArmDepth = 1

	pin(&swingdecline.Enabled, &swingDecline, true)
	pin(&swingdecline.ResetOnNewDepth, &swingDeclineResetOnNewDepth, true)
	swingdecline.MinDepthsBetween, swingdecline.LookbackBars, swingdecline.PivotBars, swingdecline.MinBarsBetween = 3, 24, 4, 6
	swingdecline.MinDropPct, swingdecline.MaxBouncePct = 5.0, 3.0
	swingdecline.MinLegsDown, swingdecline.MinLegsUp, swingdecline.MinSmcTrendDown = 2, 1, 2
	swingdecline.ArmDepth = 1 + swingdecline.MinDepthsBetween

	pin(&lineardecline.Enabled, &linearDecline, false)
	pin(&lineardecline.ResetOnNewDepth, &linearDeclineResetOnNewDepth, true)
	lineardecline.MinFills, lineardecline.MinSmcTrendDown = 5, 3
	lineardecline.MaxDeviationPct, lineardecline.MaxSlopeChange, lineardecline.MinFallPct = 5.0, 0.2, -5.0
	lineardecline.ArmDepth = lineardecline.MinFills
	lineardecline.Window = max(lineardecline.MinFills, 3) // 3 is the chord floor, previousChordFills

	pin(&earlytakeprofit.Enabled, &earlyTakeProfit, true)
	pin(&earlytakeprofit.QuietDecline, &earlyTakeProfitQuietDecline, false)
	pin(&earlytakeprofit.SwingDecline, &earlyTakeProfitSwingDecline, true)
	pin(&earlytakeprofit.LinearDecline, &earlyTakeProfitLinearDecline, true)
	pin(&earlytakeprofit.FlatMarket, &earlyTakeProfitFlatMarket, true)
	pin(&earlytakeprofit.FreeFallPrevention, &earlyTakeProfitFreeFallPrevention, true)
	pin(&earlytakeprofit.DepthPriority, &earlyTakeProfitDepthPriority, false)
	pin(&earlytakeprofit.DynamicParams, &earlyTakeProfitDynamicParams, false)
	earlytakeprofit.ArmDepth = 2

	pin(&flatmarket.Enabled, &flatMarket, true)
	pin(&flatmarket.HoldInitialDepth, &flatMarketHoldInitialDepth, true)
	pin(&flatmarket.ResetOnNewDepth, &flatMarketResetOnNewDepth, true)
	flatmarket.ArmDepth, flatmarket.MinSmcTrend = 1, 3
	flatmarket.Timeframe, flatmarket.WindowTimeframe, flatmarket.RangeBars = "1h", 36*time.Hour, 24
	flatmarket.BodyShare, flatmarket.QuietShare, flatmarket.MaxMovePct, flatmarket.MaxCloseRangePct = 0.75, 0.9, 2.0, 3.0
	flatmarket.MaxDeviation, flatmarket.MaxSlopePercentage, flatmarket.TolerancePercentage = 0.75, 0.15, 5
	flatmarket.Window = int(flatmarket.WindowTimeframe / (time.Duration(helpers.IntervalToMinutes(flatmarket.Timeframe)) * time.Minute))

	pin(&freefallprevention.Enabled, &freeFallPrevention, false)
	pin(&freefallprevention.HoldInitialDepth, &freeFallPreventionHoldInitialDepth, true)
	pin(&freefallprevention.ResetOnNewDepth, &freeFallPreventionResetOnNewDepth, true)
	freefallprevention.ArmDepth, freefallprevention.MinShare = 2, 0.8
	freefallprevention.MinFreeFallTimeframes, freefallprevention.FreeFallTimeframes = 1, []string{"1h"}
	freefallprevention.Timeframe, freefallprevention.WindowTimeframe = "1h", 10*24*time.Hour
	freefallprevention.Window = int(freefallprevention.WindowTimeframe / (time.Duration(helpers.IntervalToMinutes(freefallprevention.Timeframe)) * time.Minute))
	rederive()
}

// rederive builds the test variables defined on the knobs again from the pinned values, each as its
// own file defines it: they are built at init, before TestMain, from the constants as they ship.
func rederive() {
	latchDepth = max(earlytakeprofit.ArmDepth, 1)
	freeFallLatchDepth = max(earlytakeprofit.ArmDepth, freefallprevention.ArmDepth, 1)
	indecisionFills = max(earlytakeprofit.ArmDepth, 2)
	flatMarketLineBars = max(flatmarket.Window, flatmarket.MinBars)
	flatMarketHoldMove, flatMarketOverMove, flatMarketOverStray = 0.5*flatmarket.MaxMovePct, 1.5*flatmarket.MaxMovePct, 1.5*flatmarket.MaxMovePct
	linearFall = 1.5 * math.Max(-lineardecline.MinFallPct, 1)
	linearOver = 1.5 * math.Max(lineardecline.MaxDeviationPct, 1)
	linearSlope = -linearFall / float64((lineardecline.Window-1)*linearGap)
	linearDepth = max(lineardecline.ArmDepth, earlytakeprofit.ArmDepth, lineardecline.Window)
	linearTrendValid = max(3, lineardecline.MinSmcTrendDown)
	linearTrendMet = max(lineardecline.MinSmcTrendDown, 0)
	linearTrendShort = lineardecline.MinSmcTrendDown - 1
	lbLegBars = max(2*swingdecline.PivotBars, swingdecline.MinBarsBetween)
	patternTrendDown, shortTrendDown = swingdecline.MinSmcTrendDown, swingdecline.MinSmcTrendDown-1
	shortTrendReason = fmt.Sprintf("smc trend direction down %d/%d short of %d needed", shortTrendDown, patternTrendValid, swingdecline.MinSmcTrendDown)
}

// anyLoss is the loss every fixture's sale reaches the trigger within: the whole used amount.
const anyLoss = 100.0

// shippedLoss is the loss SellLossTriggers lets the shared action take on a dashboard reading the
// ladder's way on every read, in percent of the used amount (FullTrendTrigger negated, read at
// init off the shipped table); zero or below when it waits for break even or a profit.
var shippedLoss = -FullTrendTrigger()

// allowedLoss is shippedLoss for the table in force (FullTrendTrigger).
func allowedLoss() float64 {
	return 0 - FullTrendTrigger()
}

// withAllowedLoss runs the test with one rule, any row reading the ladder's way, at a loss of pct
// percent of the used amount (a profit for pct below zero) and puts the package's table back after;
// package state, so never in parallel. withTable runs it with the rules given.
func withAllowedLoss(t *testing.T, pct float64) {
	t.Helper()
	withTable(t, []SellLossTrigger{{MinTrendDirection: 1, Percentage: 0 - pct}})
}

func withTable(t *testing.T, rules []SellLossTrigger) {
	t.Helper()
	previous := sellLossTriggers
	sellLossTriggers = rules
	t.Cleanup(func() { sellLossTriggers = previous })
}

// everyRow is the rows of sophos' SMC trend dashboard (dashboardTimeframes: 1h, 4h, 1D) and everyRead
// the reads of each; withFullTrend is the block with every row reading the side's way on every read,
// under which the shared trigger is the table's strictest rule (FullTrendTrigger), withEveryRowTrend
// that block for a long ladder and everyRowShared its trigger. withTrendCounts is the block with the
// Trend Direction down on the first down rows and up on the next up rows, each on every read, the
// rest on none; withTrendReads the block with each row reading down and up on the reads given.
const (
	everyRow  = 3
	everyRead = 9
)

var dashboardTimeframes = []string{"1h", "4h", "1D"}

func withFullTrend(block aggragates.SmartTakeLossIndicators, side string) aggragates.SmartTakeLossIndicators {
	full := []int{everyRead, everyRead, everyRead}
	if side == aggragates.SideShort {
		return withTrendReads(block, nil, full)
	}
	return withTrendReads(block, full, nil)
}

func withEveryRowTrend(block aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators {
	return withFullTrend(block, aggragates.SideLong)
}

func everyRowShared() sharedTrigger {
	block := withEveryRowTrend(aggragates.SmartTakeLossIndicators{})
	return sharedTriggerOf(series.TrendOf(block), series.ReadsOf(block), aggragates.SideLong)
}

func withTrendCounts(block aggragates.SmartTakeLossIndicators, down, up int) aggragates.SmartTakeLossIndicators {
	downReads, upReads := make([]int, everyRow), make([]int, everyRow)
	for index := range everyRow {
		switch {
		case index < down:
			downReads[index] = everyRead
		case index < down+up:
			upReads[index] = everyRead
		}
	}
	return withTrendReads(block, downReads, upReads)
}

func withTrendReads(block aggragates.SmartTakeLossIndicators, down, up []int) aggragates.SmartTakeLossIndicators {
	rows := max(len(down), len(up))
	block.SmcTrendTimeframes = dashboardTimeframes[:rows]
	block.SmcTrendRowDown, block.SmcTrendRowUp, block.SmcTrendRowReads = make([]int64, rows), make([]int64, rows), make([]int64, rows)
	block.SmcTrendDown, block.SmcTrendUp, block.SmcTrendValid = 0, 0, rows
	for index := range rows {
		var d, u int
		if index < len(down) {
			d = down[index]
		}
		if index < len(up) {
			u = up[index]
		}
		block.SmcTrendRowDown[index], block.SmcTrendRowUp[index], block.SmcTrendRowReads[index] = int64(d), int64(u), everyRead
		if d > u {
			block.SmcTrendDown++
		} else if u > d {
			block.SmcTrendUp++
		}
	}
	return block
}

// trendHeldWords is the held row's reason the block's dashboard gives a long or an inverse ladder (heldByTrendOf).
func trendHeldWords(block aggragates.SmartTakeLossIndicators, inverse bool) string {
	return heldByTrendOf(block, inverse)
}

// withSellLossIgnoreLastDepths is withAllowedLoss for the depth bound: n depths unfilled, none at zero or below.
func withSellLossIgnoreLastDepths(t *testing.T, n int) {
	t.Helper()
	previous := sellLossIgnoreLastDepths
	sellLossIgnoreLastDepths = n
	t.Cleanup(func() { sellLossIgnoreLastDepths = previous })
}

// withSellLossStartAtDepth is withSellLossIgnoreLastDepths for the activation floor: n filled entries, none at zero or below.
func withSellLossStartAtDepth(t *testing.T, n int) {
	t.Helper()
	previous := sellLossStartAtDepth
	sellLossStartAtDepth = n
	t.Cleanup(func() { sellLossStartAtDepth = previous })
}

// doublingLadder is a long spot ladder on the LadderTrade row, one entry per price, quantities doubling from one, in buy.
func doublingLadder(prices ...float64) aggragates.Trades {
	trade := testutil.LadderTrade(false)
	quantity := 1.0
	for index, price := range prices {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: quantity, Price: price, OrderId: int64(index + 1), CreatedAt: testutil.At("12:00:00").Add(time.Duration(index) * time.Hour),
		})
		trade.PositionPrice = price
		quantity *= 2
	}
	return trade
}

// takeProfitLines are the prices the buy row arms the take profit at, over the position price and over the average entry.
func takeProfitLines(trade aggragates.Trades) (fromPosition, fromAverage float64) {
	step := takeProfitStep(trade) / 100
	return trade.PositionPrice / (1 - step), ladder.AverageEntryPrice(trade) / (1 - step)
}

// pastTheNewestFillUnderBreakEven reports whether price reaches the take profit measured from the
// newest fill while under the ladder's break even, where only the shared action acts.
func pastTheNewestFillUnderBreakEven(trade aggragates.Trades, price float64) bool {
	newest := rebuildState(trade).lastFill().Price
	settings := trade.StrategyPair.StrategySettings
	breakEven := ladder.AverageEntryPrice(trade)
	if newest <= 0 || price <= 0 || len(settings) == 0 || breakEven <= 0 || price >= breakEven {
		return false
	}
	row := settings[ladder.SettingsIndexOrBase(settings, ladder.CountFilledEntries(trade)-1)]
	return moveAgainst(price, newest) >= row.Percentage+row.Tolerance
}

// quietDeclineRecentReasons is what sophos names for the bar among its last
// closed bars on which the verdict stood: that bar, then its own reasons.
var quietDeclineRecentReasons = []string{"read on the 1h bar opening 2021-07-26 16:00 UTC, one of the last 24 closed bars", "leg down 7.0% from its high close"}

// withRecent is the reading carrying a bar the verdict stood on (opening at stood, the oldest at from)
// and that bar's reasons; a zero time serves zero.
func withRecent(reading aggragates.AIIndicators, stood, from time.Time) aggragates.AIIndicators {
	if !stood.IsZero() {
		reading.SmartTakeLoss.QuietDeclineRecentAt = stood.UnixMilli()
	}
	if !from.IsZero() {
		reading.SmartTakeLoss.QuietDeclineRecentFrom = from.UnixMilli()
	}
	reading.SmartTakeLoss.QuietDeclineRecentReasons = quietDeclineRecentReasons
	return reading
}

// fillWindowFrom is the QuietDeclineFillFrom the fixture readings serve, early on the fixture day, so
// every fixture fill is recent unless a test moves it.
var fillWindowFrom = testutil.At("00:00:00")

// withFlatMarket is withIndecision for the flat market gate.
func withFlatMarket(t *testing.T, on bool) {
	t.Helper()
	previous := flatMarket
	flatMarket = on
	t.Cleanup(func() { flatMarket = previous })
}

// withFlatMarketHoldInitialDepth is withIndecision for the flat market gate's first-fill hold.
func withFlatMarketHoldInitialDepth(t *testing.T, on bool) {
	t.Helper()
	previous := flatMarketHoldInitialDepth
	flatMarketHoldInitialDepth = on
	t.Cleanup(func() { flatMarketHoldInitialDepth = previous })
}

// flatMarketAlone runs the test with the flat market gate on, the cap at SellLossTriggerPercentage and
// every other rule off, on a market built from the lowinterest constants (flatMarketBlockOf); it
// skips only for a constant that leaves no market (noFlatMarketMarket) and fails when the market does not hold.
func flatMarketAlone(t *testing.T) {
	t.Helper()
	probe := testutil.LadderTrade(false)
	if name := noFlatMarketMarket(probe.StrategyPair.StrategySettings[0].Percentage); name != "" {
		t.Skipf("%s leaves no market that holds for the fixture ladders to be read under", name)
	}
	market := flatMarketMarket(probe)
	if reading := flatMarketReading(probe, market.SmartTakeLoss, market); !reading.Holds {
		missed := flatmarket.RangeBreaks(flatmarket.BarsOf(market.SmartTakeLoss), basePercentage(probe), reading.Range.Side, reading.Range.Trend)
		t.Fatalf("the constants of gates/smarttakeloss/lowinterest leave a market that holds, built for a long ladder (side %q), and the reading does not hold on it: %+v; the market itself misses %q",
			aggragates.SideLong, reading, missed)
	}
	withFlatMarket(t, true)
	withAllowedLoss(t, shippedLoss)
	withQuietDecline(t, false)
	withSwingDecline(t, false)
	withEarlyTakeProfit(t, false)
	withCapitalProtectionExit(t, false)
}

// noFlatMarketMarket names the lowinterest constant that leaves the fixture ladders no market that
// holds (the quiet share, the body bound or the move bound), and nothing when every one leaves
// room; percentage is the base row read off the fixture's settings.
func noFlatMarketMarket(percentage float64) string {
	quietBody := flatMarketQuietBody * flatmarket.BodyShare * percentage
	switch {
	case flatmarket.QuietShare >= 1:
		return "flatmarket.QuietShare"
	case quietBody <= 0 || quietBody >= 100:
		return "flatmarket.BodyShare"
	case flatMarketHoldMove <= 0 || flatMarketHoldMove >= 100:
		return "flatmarket.MaxMovePct"
	}
	return ""
}

const (
	// flatMarketBars is how many bars a fixture block serves, as many as sophos does and never fewer than the rule reads.
	flatMarketBars = max(24, flatmarket.MinBars)
	// flatMarketOpen is the open of every fixture bar, flatMarketWickPct the wick past the body; a
	// quiet body lies flatMarketQuietBody of the way into the limit, a loud one flatMarketLoudBody
	// limits.
	flatMarketOpen      = 100.0
	flatMarketWickPct   = 0.1
	flatMarketQuietBody = 0.5
	flatMarketLoudBody  = 2.0
)

// flatMarketBlockOf is the block sophos serves over flatMarketBars bars for a ladder at
// percentage on side: the first loud bars, the rest quiet, a move on the side of exactly move
// percent, every close on the mean of the first and last (no stray) and a trend on exactly
// flatmarket.MinSmcTrend rows.
func flatMarketBlockOf(percentage float64, side string, loud int, move float64) aggragates.SmartTakeLossIndicators {
	block := aggragates.SmartTakeLossIndicators{}
	limit := flatmarket.BodyShare * percentage
	for index := range flatMarketBars {
		body := flatMarketQuietBody * limit
		if index < loud {
			body = flatMarketLoudBody * limit
		}
		if index%2 == 1 {
			body = -body
		}
		closed := flatMarketOpen * (1 + body/100)
		opened := testutil.At("00:00:00").Add(time.Duration(index) * time.Hour)
		block.FlatMarketBarOpens = append(block.FlatMarketBarOpens, opened.UnixMilli())
		block.FlatMarketOpenPrices = append(block.FlatMarketOpenPrices, flatMarketOpen)
		block.FlatMarketHighs = append(block.FlatMarketHighs, math.Max(flatMarketOpen, closed)*(1+flatMarketWickPct/100))
		block.FlatMarketLows = append(block.FlatMarketLows, math.Min(flatMarketOpen, closed)*(1-flatMarketWickPct/100))
		block.FlatMarketCloses = append(block.FlatMarketCloses, closed)
	}

	last := flatMarketBars - 1
	factor := block.FlatMarketHighs[0] * (1 - move/100) / block.FlatMarketLows[last]
	if side == aggragates.SideShort {
		factor = block.FlatMarketLows[0] * (1 + move/100) / block.FlatMarketHighs[last]
	}
	scaleFlatMarketBar(&block, last, factor)

	mean := (block.FlatMarketCloses[0] + block.FlatMarketCloses[last]) / 2
	for index := 1; index < last; index++ {
		scaleFlatMarketBar(&block, index, mean/block.FlatMarketCloses[index])
	}

	block = withFullTrend(block, side)
	return block
}

// scaleFlatMarketBar scales the open, the high, the low and the close of the block's
// bar at index alike, so that its body keeps its share of its own open.
func scaleFlatMarketBar(block *aggragates.SmartTakeLossIndicators, index int, factor float64) {
	for _, prices := range [][]float64{block.FlatMarketOpenPrices, block.FlatMarketHighs, block.FlatMarketLows, block.FlatMarketCloses} {
		prices[index] *= factor
	}
}

// flatMarketBlock is the block that holds for a ladder of that percentage and
// side: every body quiet and a move of flatMarketHoldMove.
func flatMarketBlock(percentage float64, side string) aggragates.SmartTakeLossIndicators {
	return flatMarketBlockOf(percentage, side, 0, flatMarketHoldMove)
}

// flatMarketStrayBlock is the holding block but for the middle bar's close, stray percent of the
// mean away from it (above when positive, below when negative).
func flatMarketStrayBlock(percentage float64, side string, stray float64) aggragates.SmartTakeLossIndicators {
	block := flatMarketBlock(percentage, side)
	scaleFlatMarketBar(&block, flatMarketBars/2, 1+stray/100)
	return block
}

// flatMarketWideMoveBlock is the holding block but for its move: the newest bar's extreme is
// stretched until the move on that side is exactly flatMarketOverMove, the closes untouched so the
// close range still holds and the move is the one miss.
func flatMarketWideMoveBlock(percentage float64, side string) aggragates.SmartTakeLossIndicators {
	block := flatMarketBlock(percentage, side)
	last := flatMarketBars - 1
	if side == aggragates.SideShort {
		block.FlatMarketHighs[last] = block.FlatMarketLows[0] * (1 + flatMarketOverMove/100)
		return block
	}
	block.FlatMarketLows[last] = block.FlatMarketHighs[0] * (1 - flatMarketOverMove/100)
	return block
}

// flatMarketMarket is the payload that holds for the trade: the block of its base
// percentage and its entry side.
func flatMarketMarket(trade aggragates.Trades) aggragates.AIIndicators {
	return withBlock(flatMarketBlock(basePercentage(trade), aggragates.EntrySide(trade, aggragates.AIIndicators{})))
}

// flatMarketMisses are the markets neither reading holds on: bars not served, all-loud bodies (the
// range's quiet share and the line's tolerance both miss), a line rising against the ladder (the
// range's stray and close range miss with it) and a trend a row short of the need (still a row,
// which the shared trigger reads); the trend is left out when a retune leaves it nothing to read.
// A wick stretched or one close astray miss the range alone and read as a flat line still.
func flatMarketMisses(percentage float64, side string) map[string]aggragates.AIIndicators {
	against := 10.0
	if side == aggragates.SideShort {
		against = -against
	}
	misses := map[string]aggragates.AIIndicators{
		"bars not served":           withBlock(withEveryRowTrend(aggragates.SmartTakeLossIndicators{})),
		"loud bodies":               withBlock(flatMarketBlockOf(percentage, side, flatMarketBars, flatMarketHoldMove)),
		"a line against the ladder": withBlock(flatMarketLineBlockOf(percentage, side, against)),
	}
	if flatmarket.MinSmcTrend > 1 {
		block := flatMarketBlock(percentage, side)
		short := flatmarket.MinSmcTrend - 1
		block.SmcTrendDown, block.SmcTrendUp = min(block.SmcTrendDown, short), min(block.SmcTrendUp, short)
		misses["trend short"] = withBlock(block)
	}
	return misses
}

// flatMarketOffTheWatch are the trades the gate does nothing for: futures, a child, no flag, no settings, no fill.
func flatMarketOffTheWatch(base aggragates.Trades) map[string]aggragates.Trades {
	futures, child, unflagged, unsettled := base, base, base, base
	futures.Strategy.TradeType = aggragates.Futures
	child.ParentID = 7
	unflagged.Strategy.Params.SmartTakeLoss = false
	unsettled.StrategyPair.StrategySettings = nil
	return map[string]aggragates.Trades{
		"futures": futures, "child": child, "no flag": unflagged, "no settings": unsettled,
		"no fill": testutil.LadderTrade(base.Inverse),
	}
}

// flatMarketLadder is a spot ladder of whole units at roundPrices, long or inverse, filled an hour
// apart, resting in buy, whose loss is exact in floating point.
func flatMarketLadder(inverse bool) aggragates.Trades {
	trade := testutil.LadderTrade(inverse)
	entry := "BUY"
	if inverse {
		entry = "SELL"
	}
	for index, price := range roundPrices {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: entry, Quantity: exactUnits, Price: price, OrderId: int64(index + 1),
			CreatedAt: testutil.At(fmt.Sprintf("%02d:00:00", 9+index)),
		})
		trade.PositionPrice = price
	}
	return trade
}

// flatMarketPrices are the prices at which the round ladder loses exactly the cap and one cent
// more; it skips the test when the trigger takes no loss or names no price above zero.
func flatMarketPrices(t *testing.T, trade aggragates.Trades) (atTheCap, oneStepLower float64) {
	t.Helper()
	used, units := ladder.EntryNotional(trade), ladder.EntryQuantity(trade)
	cents := math.Round((used - used*allowedLoss()/100) / units * 100)
	atTheCap, oneStepLower = cents/100, (cents-1)/100
	loss, lowerLoss := used-float64(units*atTheCap), used-float64(units*oneStepLower)
	if allowedLoss() <= 0 || oneStepLower <= 0 || loss*100 != allowedLoss()*used || lowerLoss*100 <= allowedLoss()*used {
		t.Skip("SellLossTriggerPercentage takes no loss or names no price above zero on the cent grid the round ladder loses exactly that much at")
	}
	return atTheCap, oneStepLower
}

// triggeredBy is the triggered row the market opens the trade's episode with.
func triggeredBy(trade aggragates.Trades, ai aggragates.AIIndicators) Row {
	return FlatMarketTriggeredRow(trade.PositionPrice, flatMarketReading(trade, ai.SmartTakeLoss, ai).Reasons)
}

// withCapitalProtectionWaitAfterTrigger runs the test with capital protection's wait after its
// trigger at wait and puts the package's value back after; package state, so never in parallel.
func withCapitalProtectionWaitAfterTrigger(t *testing.T, wait time.Duration) {
	t.Helper()
	previous := capitalProtectionWaitAfterTrigger
	capitalProtectionWaitAfterTrigger = wait
	t.Cleanup(func() { capitalProtectionWaitAfterTrigger = previous })
}

// withWaitAfterTrigger runs the test with the sale gates' wait after their trigger at wait and
// puts the package's value back after; package state, so never in parallel.
func withWaitAfterTrigger(t *testing.T, wait time.Duration) {
	t.Helper()
	previous := waitAfterTrigger
	waitAfterTrigger = wait
	t.Cleanup(func() { waitAfterTrigger = previous })
}

// withActivationConfirmation runs the test with the reached trigger's confirmation at d and puts
// the package's value back after; package state, so never in parallel.
func withActivationConfirmation(t *testing.T, d time.Duration) {
	t.Helper()
	previous := activationConfirmation
	activationConfirmation = d
	t.Cleanup(func() { activationConfirmation = previous })
}

// withActivationConfirmationWindow runs the test with the reached row's confirmation window at d and
// puts the package's value back after; package state, so never in parallel.
func withActivationConfirmationWindow(t *testing.T, d time.Duration) {
	t.Helper()
	previous := activationConfirmationWindow
	activationConfirmationWindow = d
	t.Cleanup(func() { activationConfirmationWindow = previous })
}

// withSaleGapTolerances runs the test with the committed sale's gap floor at n tolerances and puts
// the package's value back after; package state, so never in parallel.
func withSaleGapTolerances(t *testing.T, n float64) {
	t.Helper()
	previous := saleGapTolerances
	saleGapTolerances = n
	t.Cleanup(func() { saleGapTolerances = previous })
}

// withSaleGapWait runs the test with the gap hold's wait at d and puts the package's value back
// after; package state, so never in parallel.
func withSaleGapWait(t *testing.T, d time.Duration) {
	t.Helper()
	previous := saleGapWait
	saleGapWait = d
	t.Cleanup(func() { saleGapWait = previous })
}

// engineStepAt is engineStep on a clock: ApplyAt at now, every row written with its event stamped
// at now, and stopLoss at the tick price after an activation or a re-anchor.
func engineStepAt(trade aggragates.Trades, position string, price float64, now time.Time, ai aggragates.AIIndicators) (aggragates.Trades, Result) {
	got := ApplyAt(trade, position, price, ai, now)
	for _, row := range got.Rows {
		trade = withRow(trade, row, now)
	}
	if got.Activated {
		trade.PositionType, trade.PositionPrice = got.Position, price
	}
	return trade, got
}

// gateMemory is what the state remembers of one sale gate: its open episode (pending for the slow
// decline), the held sale and the trigger it was held under, the reached stamp, the activation, the
// stamp of its open gap (zero when none) and whether one is open.
type gateMemory struct {
	open      bool
	saleHeld  bool
	heldUnder heldTrigger
	reachedAt time.Time
	activated bool
	gapHeldAt time.Time
	gapHeld   bool
}

// memoryOf is the state's memory of the gate.
func memoryOf(st state, gate string) gateMemory {
	memory := func(open, saleHeld bool, heldUnder heldTrigger, reachedAt time.Time, activated bool, gapHeldAt time.Time) gateMemory {
		return gateMemory{open, saleHeld, heldUnder, reachedAt, activated, gapHeldAt, !gapHeldAt.IsZero()}
	}
	switch gate {
	case GateFlatMarket:
		return memory(st.flatMarketActive, st.flatMarketSaleHeld, st.flatMarketHeldUnder, st.flatMarketReachedAt, st.flatMarketActivated, st.flatMarketGapHeldAt)
	case GateFreeFallPrevention:
		return memory(st.freeFallPreventionActive, st.freeFallPreventionSaleHeld, st.freeFallPreventionHeldUnder, st.freeFallPreventionReachedAt, st.freeFallPreventionActivated, st.freeFallPreventionGapHeldAt)
	case GateCapitalProtection:
		return memory(st.capitalProtectionActive, st.capitalProtectionSaleHeld, st.capitalProtectionHeldUnder, st.capitalProtectionReachedAt, st.capitalProtectionActivated, st.capitalProtectionGapHeldAt)
	}
	return memory(st.slowDeclinePending, st.saleHeld, st.saleHeldUnder, st.saleReachedAt, st.slowDeclineActivated, st.slowDeclineGapHeldAt)
}

// withFreeFallPrevention is withIndecision for the free fall prevention gate.
func withFreeFallPrevention(t *testing.T, on bool) {
	t.Helper()
	previous := freeFallPrevention
	freeFallPrevention = on
	t.Cleanup(func() { freeFallPrevention = previous })
}

// withFreeFallPreventionHoldInitialDepth is withIndecision for the free fall prevention gate's first-fill hold.
func withFreeFallPreventionHoldInitialDepth(t *testing.T, on bool) {
	t.Helper()
	previous := freeFallPreventionHoldInitialDepth
	freeFallPreventionHoldInitialDepth = on
	t.Cleanup(func() { freeFallPreventionHoldInitialDepth = previous })
}

// withFreeFallPreventionResetOnNewDepth is withIndecision for the free fall prevention gate's reset at a new depth.
func withFreeFallPreventionResetOnNewDepth(t *testing.T, on bool) {
	t.Helper()
	previous := freeFallPreventionResetOnNewDepth
	freeFallPreventionResetOnNewDepth = on
	t.Cleanup(func() { freeFallPreventionResetOnNewDepth = previous })
}

// withEarlyTakeProfitFreeFallPrevention is withIndecision for the latch on the free fall prevention reading.
func withEarlyTakeProfitFreeFallPrevention(t *testing.T, on bool) {
	t.Helper()
	previous := earlyTakeProfitFreeFallPrevention
	earlyTakeProfitFreeFallPrevention = on
	t.Cleanup(func() { earlyTakeProfitFreeFallPrevention = previous })
}

// switches are the switches, bounds and waits TestSwitchesReadAsShipped compares.
type switches struct {
	quiet, swing, early, linear, flat, capital, recentFill, pause bool
	latchQuiet, latchSwing, latchLinear, latchFlat                bool
	latchFreeFall, latchDepthPriority, latchDynamicParams         bool
	flatHold, flatReset, swingReset, linearReset                  bool
	freeFall, freeFallHold, freeFallReset                         bool
	drawdown                                                      float64
	startAtDepth, ignoreLastDepths                                int
	triggers                                                      []SellLossTrigger
	wait, capitalWait                                             time.Duration
}

// shippedLeaf and shippedRead are the switches as the packages initialise them, before TestMain
// pins the profile: the leaf constants, and the mirrors the composer reads them through, which
// the wiring of each to its constant makes equal.
var (
	shippedLeaf = switches{
		quiet: quietdecline.Enabled, swing: swingdecline.Enabled, early: earlytakeprofit.Enabled, linear: lineardecline.Enabled,
		flat: flatmarket.Enabled, capital: capitalprotection.Enabled, recentFill: quietdecline.NeedsRecentFill, pause: PauseWhenActiveDepthPriority,
		latchQuiet: earlytakeprofit.QuietDecline, latchSwing: earlytakeprofit.SwingDecline, latchLinear: earlytakeprofit.LinearDecline,
		latchFlat: earlytakeprofit.FlatMarket, latchFreeFall: earlytakeprofit.FreeFallPrevention,
		latchDepthPriority: earlytakeprofit.DepthPriority, latchDynamicParams: earlytakeprofit.DynamicParams,
		flatHold: flatmarket.HoldInitialDepth, flatReset: flatmarket.ResetOnNewDepth,
		swingReset: swingdecline.ResetOnNewDepth, linearReset: lineardecline.ResetOnNewDepth,
		freeFall: freefallprevention.Enabled, freeFallHold: freefallprevention.HoldInitialDepth, freeFallReset: freefallprevention.ResetOnNewDepth,
		drawdown: capitalprotection.MaxDrawdownPercentage, wait: WaitAfterTrigger, capitalWait: capitalprotection.WaitAfterTrigger,
		startAtDepth: SellLossStartAtDepth, ignoreLastDepths: SellLossIgnoreLastDepths, triggers: SellLossTriggers,
	}
	shippedRead = switches{
		quiet: quietDecline, swing: swingDecline, early: earlyTakeProfit, linear: linearDecline,
		flat: flatMarket, capital: capitalProtectionExit, recentFill: quietDeclineNeedsRecentFill, pause: pauseWhenActiveDepthPriority,
		latchQuiet: earlyTakeProfitQuietDecline, latchSwing: earlyTakeProfitSwingDecline, latchLinear: earlyTakeProfitLinearDecline,
		latchFlat: earlyTakeProfitFlatMarket, latchFreeFall: earlyTakeProfitFreeFallPrevention,
		latchDepthPriority: earlyTakeProfitDepthPriority, latchDynamicParams: earlyTakeProfitDynamicParams,
		flatHold: flatMarketHoldInitialDepth, flatReset: flatMarketResetOnNewDepth,
		swingReset: swingDeclineResetOnNewDepth, linearReset: linearDeclineResetOnNewDepth,
		freeFall: freeFallPrevention, freeFallHold: freeFallPreventionHoldInitialDepth, freeFallReset: freeFallPreventionResetOnNewDepth,
		drawdown: capitalProtectionMaxDrawdown, wait: waitAfterTrigger, capitalWait: capitalProtectionWaitAfterTrigger,
		startAtDepth: sellLossStartAtDepth, ignoreLastDepths: sellLossIgnoreLastDepths, triggers: sellLossTriggers,
	}
)

// freeFallPreventionAlone runs the test with the free fall prevention gate on and every other gate
// off, the cap at the shipped loss (withAllowedLoss) and the rest of the shared action's bounds as
// TestMain leaves them; the dashboards of freeFallMarket read the shared trigger on every row.
func freeFallPreventionAlone(t *testing.T) {
	t.Helper()
	withFreeFallPrevention(t, true)
	withAllowedLoss(t, shippedLoss)
	withQuietDecline(t, false)
	withSwingDecline(t, false)
	withLinearDecline(t, false)
	withEarlyTakeProfit(t, false)
	withCapitalProtectionExit(t, false)
	withFlatMarket(t, false)
}

// freeFallLastBar are the reasons the 1h row in free fall on the last closed bar gives a long ladder.
var freeFallLastBar = []string{"1h in free fall on the last closed bar"}

// freeFallWay is the free fall and free rise arrays that read on rows for a ladder of side: rows as
// the free fall of a long ladder, as the free rise of an inverse one, the other array not served.
func freeFallWay(side string, rows []bool) (fall, rise []bool) {
	if side == aggragates.SideShort {
		return nil, rows
	}
	return rows, nil
}

// freeFallBlockOf is the block with the dashboard served, every row reading the side's way, and the
// 1h, 4h and 1D rows in free fall (or free rise for an inverse ladder) as given.
func freeFallBlockOf(side string, rows ...bool) aggragates.SmartTakeLossIndicators {
	fall, rise := freeFallWay(side, rows)
	return inFreeFall(withFullTrend(aggragates.SmartTakeLossIndicators{}, side), fall, rise)
}

// freeFallMarket is the payload on which the free fall prevention reading holds for the trade, on
// the last closed bar alone: the 1h row in free fall for a long ladder, in free rise for an inverse
// one, the rows after it not.
func freeFallMarket(trade aggragates.Trades) aggragates.AIIndicators {
	return withBlock(freeFallBlockOf(aggragates.EntrySide(trade, aggragates.AIIndicators{}), true, false, false))
}

// freeFallSeries is the block sophos serves with the free fall series over the window, the newest
// free of its freefallprevention.Window bars carrying the 1h bit (free fall of a long ladder, free
// rise of an inverse one) and the rest none; the rows are served in free fall nowhere, so the share
// alone holds the reading when free is over the share.
func freeFallSeries(side string, free int) aggragates.SmartTakeLossIndicators {
	block := freeFallBlockOf(side, false, false, false)
	bars := freefallprevention.Window
	masks := make([]int64, bars)
	for index := bars - free; index < bars; index++ {
		masks[index] = 1
	}
	for index := range bars {
		block.FreeFallOpens = append(block.FreeFallOpens, testutil.At("00:00:00").Add(time.Duration(index)*time.Hour).UnixMilli())
	}
	block.FreeFallMasks, block.FreeRiseMasks = make([]int64, bars), make([]int64, bars)
	if side == aggragates.SideShort {
		block.FreeRiseMasks = masks
	} else {
		block.FreeFallMasks = masks
	}
	return block
}

// freeFallMisses are the payloads the reading does not hold on for a ladder of side, each with the
// dashboard read the ladder's way so that the shared trigger stands as it does under freeFallMarket:
// no row in free fall, the 4h and the 1D alone, the other way's free fall (free rise for a long
// ladder), and free fall not served.
func freeFallMisses(side string) map[string]aggragates.AIIndicators {
	trend := withFullTrend(aggragates.SmartTakeLossIndicators{}, side)
	wrongFall, wrongRise := freeFallWay(aggragates.SideShort, []bool{true, true, true})
	if side == aggragates.SideShort {
		wrongFall, wrongRise = freeFallWay(aggragates.SideLong, []bool{true, true, true})
	}
	return map[string]aggragates.AIIndicators{
		"no row in free fall":     withBlock(freeFallBlockOf(side, false, false, false)),
		"the 4h and the 1D alone": withBlock(freeFallBlockOf(side, false, true, true)),
		"the other way's rows":    withBlock(inFreeFall(trend, wrongFall, wrongRise)),
		"free fall not served":    withBlock(trend),
	}
}

// freeFallTriggeredBy is the triggered row the 1h row in free fall opens the trade's episode with:
// at its newest fill, naming the last closed bar.
func freeFallTriggeredBy(trade aggragates.Trades) Row {
	reasons := freeFallLastBar
	if trade.Inverse {
		reasons = []string{"1h in free rise on the last closed bar"}
	}
	return FreeFallPreventionTriggeredRow(trade.PositionPrice, reasons)
}

// flatMarketHoldMove and flatMarketOverMove sit either side of flatmarket.MaxMovePct,
// flatMarketOverStray over the same bound from the mean.
var (
	flatMarketHoldMove  = 0.5 * flatmarket.MaxMovePct
	flatMarketOverMove  = 1.5 * flatmarket.MaxMovePct
	flatMarketOverStray = 1.5 * flatmarket.MaxMovePct
)
