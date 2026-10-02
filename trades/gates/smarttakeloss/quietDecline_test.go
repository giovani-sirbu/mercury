package smarttakeloss

import (
	"math"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The fixtures are the w3s ladder watchedFills deep, on the plain w3s row:
// watched by the quiet decline, at QuietDeclineArmDepth filled entries or
// deeper, and short of that row's last depth, so capital protection does not
// watch it. None serves a series, so the swing decline reads nothing on them.
const (
	watchedFills        = 4
	slowDeclineBand     = 186.0
	underTheBand        = 185.0
	slowDeclineLastFill = 184.45
)

var slowDeclineReasons = []string{"leg down 7.0% from its high close", "leg 60 bars long on 1h"}

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

// slowDeclineBlock is the block sophos serves with the verdict on or off,
// the sell band and the fill window served either way.
func slowDeclineBlock(on bool) aggragates.AIIndicators {
	block := aggragates.SmartTakeLossIndicators{SlowDeclineExit: on, SlowDeclineSellBand: slowDeclineBand, SlowDeclineFillFrom: fillWindowFrom.UnixMilli()}
	if on {
		block.SlowDeclineExitReasons = slowDeclineReasons
	}
	return withBlock(block)
}

func assertNoSlowDeclineRow(t *testing.T, got Result, position string) {
	t.Helper()
	assertUntouched(t, got, position)
}

func TestQuietDeclineFixturesSitShortOfTheLastDepth(t *testing.T) {
	if watchedFills < QuietDeclineArmDepth || watchedFills >= lastDepthFills || capitalProtectionWatched(watchedTrade()) || !quietDeclineWatched(watchedTrade()) {
		t.Fatal("fixture drifted: the watched ladder must be watched and short of the w3s row's last depth")
	}
	if watchedTrade().PositionPrice != slowDeclineLastFill || !(underTheBand < slowDeclineBand) {
		t.Fatal("fixture drifted: the ladder's newest fill and the band")
	}
}

// A long parent ladder is watched from QuietDeclineArmDepth filled entries:
// the tick the verdict stands hands back one row with the newest fill's price
// and the reasons, and refuses nothing. A shallower ladder, a ladder with no
// fill, an inverse ladder, a child or a strategy without the flag writes
// nothing.
func TestApplyWatchesTheQuietDeclineFromQuietDeclineArmDepth(t *testing.T) {
	got := Apply(watchedTrade(), "", underTheBand, slowDeclineBlock(true))
	if got.Position != "" || got.Reason != "" {
		t.Fatalf("a watched ladder must go pending on the verdict without forcing, got %+v", got)
	}
	assertRow(t, got, quietPending(slowDeclineLastFill))
	want := "smartTakeLoss: quiet decline triggered (quiet decline: leg down 7.0% from its high close, leg 60 bars long on 1h)"
	if got.Rows[0].Message != want {
		t.Fatalf("row = %q, want %q", got.Rows[0].Message, want)
	}
	if got := Apply(watchedTrade(), "stopLoss", 179, slowDeclineBlock(true)); len(got.Rows) != 1 || got.Position != "stopLoss" {
		t.Fatalf("the add the ladder proposes goes on while the row is written, got %+v", got)
	}

	// QuietDeclineArmDepth filled entries are enough, and the row carries the
	// newest; one fewer is not watched, on the verdict or on the leg read
	// smooth from its newest fill with the closed bars after it.
	shallowest := testutil.LadderTrade(false, fills(QuietDeclineArmDepth, "17:38:00")...)
	got = Apply(shallowest, "", underTheBand, slowDeclineBlock(true))
	assertRow(t, got, quietPending(shallowest.PositionPrice))
	assertNoSale(t, got, "")
	shallower := testutil.LadderTrade(false, fills(QuietDeclineArmDepth-1, "17:38:00")...)
	for _, block := range []aggragates.AIIndicators{slowDeclineBlock(true), quietLegBlock(false, testutil.At("17:00:00"), fillBarClosed)} {
		assertNoSlowDeclineRow(t, Apply(shallower, "", underTheBand, block), "")
		assertNoSlowDeclineRow(t, Apply(shallower, "", slowDeclineBand+1, block), "")
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

	// Without the verdict a watched ladder is the plain ladder, the band
	// included.
	assertNoSlowDeclineRow(t, Apply(watchedTrade(), "", slowDeclineBand+1, slowDeclineBlock(false)), "")
}

// A futures ladder is watched by the quiet decline all the same, as it always
// was: it goes pending on the verdict, while the swing decline and the
// indecision direction, which watch spot ladders only, read nothing of it.
func TestApplyWatchesAFuturesLadderThroughTheQuietDeclineAlone(t *testing.T) {
	futures := watchedTrade()
	futures.Strategy.TradeType = aggragates.Futures
	st := rebuildState(futures)
	if !st.quietDeclineWatched || st.swingDeclineWatched || st.indecisionWatched || !st.slowDeclineWatched {
		t.Fatalf("a futures ladder is watched by the quiet decline alone, got %+v", st)
	}
	assertRow(t, Apply(futures, "", underTheBand, slowDeclineBlock(true)), quietPending(slowDeclineLastFill))
}

// The row is written once: a ladder that carries it is pending and proposes no
// second one, whatever the verdict says.
func TestApplyWritesTheQuietDeclineRowOnce(t *testing.T) {
	for _, on := range []bool{true, false} {
		assertNoSlowDeclineRow(t, Apply(pendingTrade(), "", underTheBand, slowDeclineBlock(on)), "")
	}
}

// A pending ladder sells on the first tick at or over the sell band — from
// the dead zone and from every add-side proposal — and stays pending after
// the verdict clears. Under the band, or with no band served, nothing sells:
// the band is the ladder's one sale
// (TestApplyPendingSellsAtTheBandAloneUnderBreakEven).
func TestApplyPendingSellsAtTheSellBand(t *testing.T) {
	for _, on := range []bool{true, false} {
		for _, position := range []string{"", "stopLoss", "update_stopLoss", "buy", "forceTrailingStopLoss"} {
			assertForced(t, Apply(pendingTrade(), position, slowDeclineBand, slowDeclineBlock(on)), "slow decline bollinger band")
			assertForced(t, Apply(pendingTrade(), position, slowDeclineBand+5, slowDeclineBlock(on)), "slow decline bollinger band")
		}
		assertNoSlowDeclineRow(t, Apply(pendingTrade(), "", underTheBand, slowDeclineBlock(on)), "")
		assertNoSlowDeclineRow(t, Apply(pendingTrade(), "stopLoss", underTheBand, slowDeclineBlock(on)), "stopLoss")
	}

	noBand := withBlock(aggragates.SmartTakeLossIndicators{})
	assertNoSlowDeclineRow(t, Apply(pendingTrade(), "", slowDeclineBand+1, noBand), "")

	got := Apply(pendingTrade(), "", slowDeclineBand, slowDeclineBlock(false))
	if msg := ExitMessage(pendingTrade(), got.Reason, slowDeclineBand); msg != "smartTakeLoss: sell at slow decline bollinger band 186.00 (quiet decline)" {
		t.Fatalf("unexpected exit row %q", msg)
	}
}

// pastTheNewestFill reaches the take profit measured from pendingTrade's
// newest fill — the held depth's Percentage plus Tolerance on the move from
// that fill — and sits under bandOverTheFill, which sits under the ladder's
// break even.
const (
	pastTheNewestFill = 189.20
	bandOverTheFill   = 191.0
)

// Under break even a pending ladder sells at the sell band and nowhere
// else. At a price past the take profit measured from its newest fill but
// under the band, the ladder's proposal passes — from the dead zone and from
// every add-side proposal, the band served, not served, or served with the
// verdict — and no row is written: nothing reads the newest fill there, the
// engines' take profit included (TakeProfitPercentage hands back the move
// against the average entry price). The band itself still sells.
func TestApplyPendingSellsAtTheBandAloneUnderBreakEven(t *testing.T) {
	trade := pendingTrade()
	breakEven := ladder.AverageEntryPrice(trade)
	underBandPastTheFill := math.Nextafter(bandOverTheFill, 0)
	if !(bandOverTheFill < breakEven) {
		t.Fatalf("fixture drifted: the band must sit under break even %v", breakEven)
	}
	band := withBlock(aggragates.SmartTakeLossIndicators{SlowDeclineSellBand: bandOverTheFill})
	verdict := slowDeclineBlock(true)
	verdict.SmartTakeLoss.SlowDeclineSellBand = bandOverTheFill
	quietLeg := quietLegBlock(false, testutil.At("17:00:00"), fillBarClosed)
	quietLeg.SmartTakeLoss.SlowDeclineSellBand = bandOverTheFill
	readings := map[string]aggragates.AIIndicators{
		"the band":             band,
		"no band":              withBlock(aggragates.SmartTakeLossIndicators{}),
		"the verdict and band": verdict,
		"a quiet leg and band": quietLeg,
	}
	for _, price := range []float64{pastTheNewestFill, underBandPastTheFill} {
		if !pastTheNewestFillUnderBreakEven(trade, price) || price >= bandOverTheFill {
			t.Fatalf("fixture drifted: %v must reach the take profit from the newest fill, under the band and break even", price)
		}
		fromAverage := moveAgainst(price, breakEven)
		if got := TakeProfitPercentage(trade, price, fromAverage); got != fromAverage {
			t.Fatalf("at %v, under break even, the take profit must read the average entry price alone, got %v want %v", price, got, fromAverage)
		}
		for name, reading := range readings {
			for _, position := range []string{"", "stopLoss", "update_stopLoss", "buy", "forceTrailingStopLoss"} {
				got := Apply(trade, position, price, reading)
				if got.Position != position || got.Reason != "" || len(got.Rows) != 0 {
					t.Fatalf("%s at %v: a pending ladder under the band must keep %q with no sale and no row, got %+v", name, price, position, got)
				}
			}
		}
	}
	for _, position := range []string{"", "stopLoss", "update_stopLoss", "buy", "forceTrailingStopLoss"} {
		assertForced(t, Apply(trade, position, bandOverTheFill, band), reasonSellBand)
	}
}

// The verdict can arrive while the price already sits over the band: the
// same tick hands back the row and sells.
func TestApplyGoesPendingAndSellsOnTheSameTick(t *testing.T) {
	got := Apply(watchedTrade(), "", slowDeclineBand+1, slowDeclineBlock(true))
	if got.Position != "sellLoss" || got.Reason != "slow decline bollinger band" {
		t.Fatalf("the pending row and the sell must come back together, got %+v", got)
	}
	assertRow(t, got, quietPending(slowDeclineLastFill))
}

// The take profit keeps working: a close the ladder decided — proposed on
// this tick or already the trade's own state — is never replaced by the
// sell band, and a trade resting in its take profit still goes pending.
func TestApplyPendingLeavesTheTakeProfitAlone(t *testing.T) {
	for _, position := range []string{"sell", "takeProfit", "update_takeProfit", "sellParent", "impasse", "sellLoss", "forceTrailingTakeProfit"} {
		assertNoSlowDeclineRow(t, Apply(pendingTrade(), position, slowDeclineBand+5, slowDeclineBlock(false)), position)
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

// Nothing waits after a fill once a ladder is pending: a pending ladder whose
// fills carry no stamp sells at the band like any other. An unstamped watched
// ladder does not go pending — the recent-fill rule fails closed on a newest
// fill without a stamp — and sells nothing; with that rule switched off it
// goes pending on the verdict and sells at the band on the same tick.
func TestApplyUnstampedFillsStillSellAtTheBand(t *testing.T) {
	unstamped := pendingTrade()
	for index := range unstamped.History {
		unstamped.History[index].CreatedAt = time.Time{}
	}
	assertForced(t, Apply(unstamped, "", slowDeclineBand+1, slowDeclineBlock(true)), reasonSellBand)

	fresh := watchedTrade()
	for index := range fresh.History {
		fresh.History[index].CreatedAt = time.Time{}
	}
	assertNoSlowDeclineRow(t, Apply(fresh, "", slowDeclineBand+1, slowDeclineBlock(true)), "")
	withRecentFillRule(t, false)
	got := Apply(fresh, "", slowDeclineBand+1, slowDeclineBlock(true))
	if len(got.Rows) != 1 {
		t.Fatalf("with the recent-fill rule off an unstamped ladder goes pending on the verdict, got %+v", got)
	}
	assertForced(t, got, reasonSellBand)
}

// The band is inclusive and exact: the band itself sells, one representable
// price under it does not — pending already, or going pending on this very
// tick — whatever the verdict says by then.
func TestApplySellBandHasNoToleranceUnderIt(t *testing.T) {
	hair := math.Nextafter(slowDeclineBand, 0)
	for _, on := range []bool{true, false} {
		assertForced(t, Apply(pendingTrade(), "", slowDeclineBand, slowDeclineBlock(on)), "slow decline bollinger band")
		assertNoSlowDeclineRow(t, Apply(pendingTrade(), "", hair, slowDeclineBlock(on)), "")
		assertNoSlowDeclineRow(t, Apply(pendingTrade(), "stopLoss", hair, slowDeclineBlock(on)), "stopLoss")
	}

	got := Apply(watchedTrade(), "", hair, slowDeclineBlock(true))
	if len(got.Rows) != 1 || got.Position != "" || got.Reason != "" {
		t.Fatalf("a hair under the band the ladder goes pending and sells nothing, got %+v", got)
	}
	got = Apply(watchedTrade(), "", slowDeclineBand, slowDeclineBlock(true))
	if len(got.Rows) != 1 || got.Position != "sellLoss" || got.Reason != "slow decline bollinger band" {
		t.Fatalf("at the band the ladder goes pending and sells on the same tick, got %+v", got)
	}
}

// A ladder resting in stopLoss whose add is about to fill — the ladder
// proposes the buy — sells instead once the price is at the band: the fill
// would commit more capital to the ladder the exit is taking out. Under the
// band the buy goes through.
func TestApplyPendingTurnsTheBuyOfAStopLossLadderIntoTheExit(t *testing.T) {
	for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
		trade := pendingTrade()
		trade.PositionType = state
		assertForced(t, Apply(trade, "buy", slowDeclineBand, slowDeclineBlock(false)), "slow decline bollinger band")
		assertForced(t, Apply(trade, "buy", slowDeclineBand+1, slowDeclineBlock(true)), "slow decline bollinger band")
		assertNoSlowDeclineRow(t, Apply(trade, "buy", underTheBand, slowDeclineBlock(false)), "buy")
	}
}

// The watch counts filled entries — distinct entry orders — the way the
// engines' tick gate (Armed, through ladder.CountFilledEntries) counts them,
// not history rows: an accounting row at the sentinel price, an empty row, an
// exit-side row and a partial fill of an order already counted add no depth,
// so rows of those beside QuietDeclineArmDepth−1 entries leave the ladder
// unwatched, neither ticked for the exit nor marked by it. One more entry —
// a legacy row without an order id is an entry of its own — makes both agree
// that it is watched.
func TestQuietDeclineWatchCountsFilledEntriesNotRows(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(QuietDeclineArmDepth-1, "17:38:00")...)
	trade.History = append(trade.History,
		aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: ladder.AccountingPriceCeiling, OrderId: 90},
		aggragates.TradesHistory{Type: "BUY", Quantity: 0, Price: 180, OrderId: 91},
		aggragates.TradesHistory{Type: "SELL", Quantity: 1, Price: 190, OrderId: 92},
		aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: trade.PositionPrice, OrderId: trade.History[0].OrderId},
	)
	if ladder.CountFilledEntries(trade) != QuietDeclineArmDepth-1 || len(trade.History) < QuietDeclineArmDepth {
		t.Fatal("fixture drifted: one entry short of the watch, with as many history rows as it needs")
	}
	if Armed(trade) || rebuildState(trade).quietDeclineWatched {
		t.Fatalf("rows that add no depth must not watch the ladder (Armed %v)", Armed(trade))
	}
	assertNoSlowDeclineRow(t, Apply(trade, "", slowDeclineBand, slowDeclineBlock(true)), "")

	legacy := aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: 179.78, CreatedAt: testutil.At("17:40:00")}
	trade.History = append(trade.History, legacy)
	if ladder.CountFilledEntries(trade) != QuietDeclineArmDepth || !Armed(trade) || !rebuildState(trade).quietDeclineWatched {
		t.Fatalf("the entry that reaches QuietDeclineArmDepth must watch the ladder for the tick gate and the exit alike (Armed %v)", Armed(trade))
	}
	assertRow(t, Apply(trade, "", underTheBand, slowDeclineBlock(true)), quietPending(legacy.Price))
}

// Switched off, the quiet decline watches no ladder: the verdict marks
// nothing, a new fill on a pending ladder is judged by nothing, the band
// sells nothing — the rows written earlier are ignored — and a ladder short
// of its last depth is no longer armed by it; the swing decline and the
// indecision direction, which watch such a ladder too, are switched off beside
// it. Switched back on, the same ticks mark and sell again.
func TestApplyQuietDeclineSwitchedOff(t *testing.T) {
	withQuietDecline(t, false)
	withIndecision(t, false)
	withSwingDecline(t, false)
	for _, position := range []string{"", "stopLoss"} {
		assertNoSlowDeclineRow(t, Apply(watchedTrade(), position, underTheBand, slowDeclineBlock(true)), position)
		assertNoSlowDeclineRow(t, Apply(pendingTrade(), position, slowDeclineBand+1, slowDeclineBlock(true)), position)
		assertNoSlowDeclineRow(t, Apply(pendingAtFive(), position, overJudgeBand, brokenReading()), position)
	}
	if Armed(watchedTrade()) || Armed(pendingTrade()) {
		t.Fatal("switched off, a ladder short of its last depth is not armed")
	}

	withQuietDecline(t, true)
	assertRow(t, Apply(watchedTrade(), "", underTheBand, slowDeclineBlock(true)), quietPending(slowDeclineLastFill))
	assertForced(t, Apply(pendingTrade(), "", slowDeclineBand+1, slowDeclineBlock(true)), reasonSellBand)
}

// fillBarClosed is a FillBefore under which watchedTrade's newest fill
// counts: the open of the bar after the one that holds that fill, served
// once sophos' minimum of closed bars stands after the fill's bar.
var fillBarClosed = testutil.At("18:00:00")

// quietLegBlock is the block sophos serves for a leg on and quiet — its vote
// passing with a ladder's own smoothness — read smooth from the bar opening
// at smoothFrom, a fill counted only when stamped before fillBefore — its
// verdict given apart — the band, the reasons and the fill window served. A
// zero time serves zero.
func quietLegBlock(verdict bool, smoothFrom, fillBefore time.Time) aggragates.AIIndicators {
	block := aggragates.SmartTakeLossIndicators{
		SlowDeclineExit:        verdict,
		SlowDeclineLegQuiet:    true,
		SlowDeclineSellBand:    slowDeclineBand,
		SlowDeclineExitReasons: slowDeclineReasons,
		SlowDeclineFillFrom:    fillWindowFrom.UnixMilli(),
	}
	if !smoothFrom.IsZero() {
		block.SlowDeclineSmoothFrom = smoothFrom.UnixMilli()
	}
	if !fillBefore.IsZero() {
		block.SlowDeclineFillBefore = fillBefore.UnixMilli()
	}
	return withBlock(block)
}

// A ladder's smoothness is counted from its newest fill: without the
// verdict, a leg on and quiet that sophos reads smooth from the bar
// holding the newest fill — or from an earlier bar, or from the fill's very
// stamp — marks the ladder pending once the closed bars after the fill's bar
// stand (fillBarClosed), with the same row at the newest fill's price. Read
// smooth only from a later bar, or a millisecond after the fill, the fast drop
// behind that fill still counts and nothing is written.
func TestApplyCountsTheSmoothLegFromTheNewestFill(t *testing.T) {
	newest := testutil.At("17:38:00")
	if watchedTrade().History[watchedFills-1].CreatedAt != newest {
		t.Fatal("fixture drifted: the watched ladder's newest fill")
	}
	for name, smoothFrom := range map[string]time.Time{
		"the bar holding the fill": testutil.At("17:00:00"),
		"an earlier bar":           testutil.At("09:00:00"),
		"the fill's own stamp":     newest,
	} {
		got := Apply(watchedTrade(), "", underTheBand, quietLegBlock(false, smoothFrom, fillBarClosed))
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
		assertRow(t, Apply(watchedTrade(), "", underTheBand, quietLegBlock(true, smoothFrom, fillBarClosed)), quietPending(slowDeclineLastFill))
	}
}

// The count needs the leg on and quiet, and a stamp to count from. A smooth
// bar served without the quiet leg marks nothing, and neither does one served
// without the bar a fill has to precede. A newest fill without a stamp is
// marked on nothing, the verdict included: the recent-fill rule fails closed
// on it. A ladder already pending writes no second row.
func TestApplySmoothFromTheNewestFillFailsClosed(t *testing.T) {
	early := testutil.At("09:00:00")
	notQuiet := withBlock(aggragates.SmartTakeLossIndicators{
		SlowDeclineSmoothFrom:  early.UnixMilli(),
		SlowDeclineFillBefore:  fillBarClosed.UnixMilli(),
		SlowDeclineSellBand:    slowDeclineBand,
		SlowDeclineExitReasons: slowDeclineReasons,
		SlowDeclineFillFrom:    fillWindowFrom.UnixMilli(),
	})
	assertNoSlowDeclineRow(t, Apply(watchedTrade(), "", underTheBand, notQuiet), "")
	assertNoSlowDeclineRow(t, Apply(watchedTrade(), "", underTheBand, quietLegBlock(false, early, time.Time{})), "")

	unstamped := watchedTrade()
	for index := range unstamped.History {
		unstamped.History[index].CreatedAt = time.Time{}
	}
	assertNoSlowDeclineRow(t, Apply(unstamped, "", underTheBand, quietLegBlock(false, early, fillBarClosed)), "")
	assertNoSlowDeclineRow(t, Apply(unstamped, "", underTheBand, quietLegBlock(true, early, fillBarClosed)), "")

	assertNoSlowDeclineRow(t, Apply(pendingTrade(), "", underTheBand, quietLegBlock(false, early, fillBarClosed)), "")
}

// Only the newest fill counts: a ladder whose earlier fills came before the
// bar sophos reads the leg smooth from and whose newest fill came after it —
// the fast drop filled the depths before it — goes pending. The same ladder
// one depth shorter, still watched, its newest fill before that bar, does
// not.
func TestApplySmoothFromReadsOnlyTheNewestFill(t *testing.T) {
	smoothFrom := testutil.At("12:00:00")
	deep := testutil.LadderTrade(false, fills(watchedFills, "17:38:00")...)
	shallow := testutil.LadderTrade(false, fills(watchedFills-1, "08:30:00")...)
	if deep.History[watchedFills-2].CreatedAt.After(smoothFrom) || shallow.History[watchedFills-2].CreatedAt.After(smoothFrom) {
		t.Fatal("fixture drifted: every fill but the deep ladder's newest must come before the smooth bar")
	}
	if !quietDeclineWatched(shallow) {
		t.Fatal("fixture drifted: the shorter ladder must still be watched")
	}
	assertRow(t, Apply(deep, "", underTheBand, quietLegBlock(false, smoothFrom, fillBarClosed)), quietPending(deep.PositionPrice))
	assertNoSlowDeclineRow(t, Apply(shallow, "", underTheBand, quietLegBlock(false, smoothFrom, fillBarClosed)), "")
}

// SlowDeclineSmoothFrom is a bar's open time in milliseconds, and the newest
// fill's stamp is compared with it in milliseconds too, its sub-millisecond
// part dropped. A fill stamped at the open, or anywhere inside the open's
// first millisecond, lies in that bar and counts the leg smooth from it; a
// fill in the bar before, even a nanosecond before the open, does not.
func TestApplySmoothFromAtTheBarOpen(t *testing.T) {
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
		got := Apply(trade, "", underTheBand, quietLegBlock(false, open, fillBarClosed))
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

// The count from the newest fill waits for the closed bars sophos requires
// after the bar that holds it: the fill is stamped strictly before
// SlowDeclineFillBefore. Served at the open of the bar after the fill's bar,
// the fill counts; at the open of the fill's own bar, or at the fill's very
// stamp, it does not; a millisecond after the stamp it does. The verdict
// does not wait for it.
func TestApplyCountsTheNewestFillOnlyWithTheClosedBarsAfterIt(t *testing.T) {
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
		got := Apply(watchedTrade(), "", underTheBand, quietLegBlock(false, smoothFrom, tc.fillBefore))
		if !tc.pending {
			assertNoSlowDeclineRow(t, got, "")
			assertRow(t, Apply(watchedTrade(), "", underTheBand, quietLegBlock(true, smoothFrom, tc.fillBefore)), quietPending(slowDeclineLastFill))
			continue
		}
		if got.Position != "" || got.Reason != "" {
			t.Fatalf("fill before %s: the ladder must go pending at its newest fill without forcing, got %+v", name, got)
		}
		assertRow(t, got, quietPending(slowDeclineLastFill))
	}
}

// A fill in the bar still forming does not count yet: a ladder opened on a
// quiet leg the SMC trend confirms but without the verdict — the first-fill
// hold reads the verdict alone — has a fill no closed bar holds, and the bar
// a fill has to precede is at the latest the forming bar's own open, where
// sophos waits for no closed bar after the fill's (SlowDeclineMinBarsAfterFill
// at none). With the price already over the band a ladder pending on its
// fill's tick would sell there and open the pair to a new ladder on the next.
// A fresh first or second fill is not watched, and a fresh fill on a watched
// ladder precedes no closed bar, so none of them goes pending or sells.
func TestApplyAFreshFillOnAQuietLegNeitherGoesPendingNorSells(t *testing.T) {
	fresh := testutil.At("17:38:00")
	// The fill prints in the forming bar: the last closed bar is the one
	// before it, which the leg reads smooth from whenever it has fallen, and
	// the bar a fill has to precede is the forming bar itself.
	lastClosed := testutil.At("16:00:00")
	block := quietLegBlock(false, lastClosed, testutil.At("17:00:00"))
	for _, depth := range []int{1, 2, QuietDeclineArmDepth, watchedFills} {
		trade := testutil.LadderTrade(false, fills(depth, "17:38:00")...)
		if trade.History[depth-1].CreatedAt != fresh {
			t.Fatalf("%d fills: fixture drifted: the newest fill must be the fresh one", depth)
		}
		for _, price := range []float64{slowDeclineBand, slowDeclineBand + 5} {
			assertNoSlowDeclineRow(t, Apply(trade, "", price, block), "")
			assertNoSlowDeclineRow(t, Apply(trade, "stopLoss", price, block), "stopLoss")
		}
	}
}
