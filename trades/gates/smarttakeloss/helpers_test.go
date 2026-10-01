package smarttakeloss

import (
	"fmt"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// lastDepthFills is the w3s row's configured Depths: the w3s ladder holding
// that many fills is at its last depth, the one capital protection watches.
const lastDepthFills = 8

// capitalProtectionBand is the upper band solBlock serves: over the newest
// fill of lastDepthLadder and under its break even, in the dead zone where the
// ladder proposes nothing of its own.
const capitalProtectionBand = 172.40

// fills is the w3s ladder n deep: the first n−1 fills a quarter hour apart
// from 08:00, the newest stamped at lastClock. A ladder zero deep — what "one
// short of the arm depth" is when the arm depth is one — holds no entry: its
// one history row is the bookkeeping row at ladder.AccountingPriceCeiling,
// which adds no depth, so the ladder is unwatched and a test that reads
// History[0] still finds a row.
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

// lastDepthLadder is the w3s ladder at its last configured depth.
func lastDepthLadder() aggragates.Trades {
	return testutil.LadderTrade(false, fills(lastDepthFills, "21:30:00")...)
}

// solBlock is the capital protection reading sophos serves over the SOL
// fixture ladder: the upper Bollinger band at capitalProtectionBand and the
// SMC trend bearish on every timeframe. It carries no slow-decline reading.
func solBlock() aggragates.SmartTakeLossIndicators {
	return aggragates.SmartTakeLossIndicators{
		CapitalProtectionUpperBB:    capitalProtectionBand,
		CapitalProtectionSmcBearish: true,
	}
}

func withBlock(block aggragates.SmartTakeLossIndicators) aggragates.AIIndicators {
	return aggragates.AIIndicators{SmartTakeLoss: block}
}

// assertUntouched fails unless Apply handed the proposal back untouched, with
// no sale and no row.
func assertUntouched(t *testing.T, got Result, position string) {
	t.Helper()
	if got.Position != position || got.Reason != "" || got.SlowDecline != nil || got.Indecision != nil {
		t.Fatalf("expected %q untouched with no row, got %+v", position, got)
	}
}

// indecisionReasons is what sophos names beside the indecision reading: the
// readings that do not hold, then the count one short of the need.
var indecisionReasons = []string{"NATR(14) 1.07x its median sampled every 24 bars, over 1.00x", "base volume of the last 24 bars 1.09x the median of the 720 before, over 0.90x", "2 of 4 readings hold with a smooth ladder, 3 needed"}

// indecisionReading is the indecision reading sophos serves: the band beside
// what broke, and the vote one short of the need.
func indecisionReading() aggragates.AIIndicators {
	return withBlock(aggragates.SmartTakeLossIndicators{
		SlowDeclineSellBand:     slowDeclineBand,
		SlowDeclineBreakReasons: indecisionReasons,
		SlowDeclineIndecision:   true,
	})
}

// latchedBy is the trade carrying the indecision row the engine wrote for its
// newest fill, after the rows it already carries.
func latchedBy(trade aggragates.Trades) aggragates.Trades {
	row := Row{Message: IndecisionMessage("buy", indecisionReasons), Price: rebuildState(trade).lastFill().Price}
	return withRow(trade, row, testutil.At("21:45:00"))
}

// withIndecisionDirection is withQuietSlowDeclineExit for the indecision
// direction.
func withIndecisionDirection(t *testing.T, on bool) {
	t.Helper()
	previous := indecisionDirection
	indecisionDirection = on
	t.Cleanup(func() { indecisionDirection = previous })
}

// doublingLadder is a long spot ladder on the LadderTrade row, one entry
// order per price given, its quantities doubling from one as a ladder's
// multiplier doubles them, in `buy` at its newest fill.
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

// takeProfitLines are the prices the `buy` row arms the take profit at,
// percentage + tolerance over the position price and over the average entry
// price.
func takeProfitLines(trade aggragates.Trades) (fromPosition, fromAverage float64) {
	step := takeProfitStep(trade) / 100
	return trade.PositionPrice / (1 - step), ladder.AverageEntryPrice(trade) / (1 - step)
}

// assertForced fails unless Apply replaced the proposal with the sellLoss
// chain under reason.
func assertForced(t *testing.T, got Result, reason string) {
	t.Helper()
	if got.Position != "sellLoss" || got.Reason != reason {
		t.Fatalf("expected a forced sellLoss for %q, got %+v", reason, got)
	}
}

// withQuietSlowDeclineExit runs the calling test with the quiet slow-decline
// exit switched on or off, and puts the package's switch back when the test
// ends. The switch is package state, so a test that calls it never runs in
// parallel.
func withQuietSlowDeclineExit(t *testing.T, on bool) {
	t.Helper()
	previous := quietSlowDeclineExit
	quietSlowDeclineExit = on
	t.Cleanup(func() { quietSlowDeclineExit = previous })
}

// withCapitalProtectionExit is withQuietSlowDeclineExit for the capital
// protection exit.
func withCapitalProtectionExit(t *testing.T, on bool) {
	t.Helper()
	previous := capitalProtectionExit
	capitalProtectionExit = on
	t.Cleanup(func() { capitalProtectionExit = previous })
}

// withRecentFillRule is withQuietSlowDeclineExit for the recent-fill rule.
func withRecentFillRule(t *testing.T, on bool) {
	t.Helper()
	previous := slowDeclineNeedsRecentFill
	slowDeclineNeedsRecentFill = on
	t.Cleanup(func() { slowDeclineNeedsRecentFill = previous })
}

// pastTheNewestFillUnderBreakEven reports whether price reaches the take
// profit measured from the trade's newest fill — the held depth's Percentage
// plus Tolerance on the move from that fill — while it sits under the
// ladder's break even, where no take profit reads that fill and the band
// alone sells a pending ladder.
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

// slowDeclineRecentReasons is what sophos names for the bar among its last
// closed bars on which the verdict stood: that bar, then its own reasons.
var slowDeclineRecentReasons = []string{"read on the 1h bar opening 2021-07-26 16:00 UTC, one of the last 24 closed bars", "leg down 7.0% from its high close"}

// withRecent is the reading carrying a bar the verdict stood on among the
// last closed bars sophos looks back over — the bar opening at stood, the
// oldest of those bars opening at from — and that bar's reasons. A zero time
// serves zero.
func withRecent(reading aggragates.AIIndicators, stood, from time.Time) aggragates.AIIndicators {
	if !stood.IsZero() {
		reading.SmartTakeLoss.SlowDeclineRecentAt = stood.UnixMilli()
	}
	if !from.IsZero() {
		reading.SmartTakeLoss.SlowDeclineRecentFrom = from.UnixMilli()
	}
	reading.SmartTakeLoss.SlowDeclineRecentReasons = slowDeclineRecentReasons
	return reading
}

// brokenAtTheBand is a reading whose last closed bar reads for no ladder —
// no verdict, no leg on and quiet — serving slowDeclineBand beside what
// broke, and the fill window.
func brokenAtTheBand() aggragates.AIIndicators {
	return withBlock(aggragates.SmartTakeLossIndicators{SlowDeclineSellBand: slowDeclineBand, SlowDeclineBreakReasons: slowDeclineBreakReasons, SlowDeclineFillFrom: fillWindowFrom.UnixMilli()})
}

// fillWindowFrom is the SlowDeclineFillFrom the fixture readings of a read
// window serve, as sophos serves it whenever it read the window: the open of
// the oldest bar a ladder's newest fill counts as recent from, early on the
// fixture day, so every fixture fill is recent and the recent-fill rule
// decides only the tests that move it.
var fillWindowFrom = testutil.At("00:00:00")
