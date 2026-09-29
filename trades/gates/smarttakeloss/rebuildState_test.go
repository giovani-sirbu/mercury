package smarttakeloss

import (
	"reflect"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// retiredRows are the rows older releases wrote for the retired
// trend-reversal rule and that stay on trades in the database: its activation
// row and the row its wait after a fill refused an add with, each carrying a
// fill's price. The texts are copied as those releases wrote them.
func retiredRows(price float64) []aggragates.TradesLogs {
	return []aggragates.TradesLogs{
		{Message: "Hold buy: smartTakeLoss: Potential trend reversal", Price: price, Type: aggragates.LOG_INFO},
		{Message: "Hold stopLoss: smartTakeLoss: last fill too fresh to sell, no add (0s)", Price: price, Type: aggragates.LOG_INFO},
	}
}

// Without rows a watched ladder is not pending, and its newest fill is the
// last in slice order even when its stamp is older than the one before it
// (live-testing stamps by hand).
func TestRebuildStateWithoutRows(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	trade.History[4].CreatedAt = testutil.At("07:00:00")

	st := rebuildState(trade)
	if !st.slowDeclineWatched || st.slowDeclinePending || st.slowDeclinePendingFrom != 0 || st.capitalProtectionWatched {
		t.Fatalf("no row must mean nothing pending, got %+v", st)
	}
	if !st.indecisionWatched || st.indecision {
		t.Fatalf("no row must mean nothing latched on a watched ladder, got %+v", st)
	}
	if len(st.fills) != 5 || st.lastFill().Price != 179.78 {
		t.Fatalf("5 fills and the last fill is 179.78, got %+v", st)
	}
}

// A slow-decline marker makes a watched ladder pending, found anywhere in the
// message and only with a price. It never makes pending a ladder the exit
// does not watch — one short of SlowDeclineArmDepth included — and the
// first-fill hold row is not a marker.
func TestRebuildStateFindsTheSlowDeclineMarker(t *testing.T) {
	trade := watchedTrade()
	if st := rebuildState(trade); !st.slowDeclineWatched || st.slowDeclinePending {
		t.Fatalf("a watched ladder without the row is not pending, got %+v", st)
	}

	trade.Logs = []aggragates.TradesLogs{
		{Message: "Hold stopLoss: cooldown: depth 5 held for 30m0s", Price: 180},
		{Message: SlowDeclineMessage("stopLoss", nil), Price: slowDeclineLastFill},
	}
	if st := rebuildState(trade); !st.slowDeclinePending {
		t.Fatalf("the framed marker must make the ladder pending, got %+v", st)
	}

	trade.Logs = []aggragates.TradesLogs{{Message: SlowDeclineMessage("buy", slowDeclineReasons), Price: 0}}
	if st := rebuildState(trade); st.slowDeclinePending {
		t.Fatalf("a marker without a price carries no fill, got %+v", st)
	}

	trade.Logs = []aggragates.TradesLogs{{Message: "Hold entry: " + SlowDeclineEntryHoldReason, Price: 201.46}}
	if st := rebuildState(trade); st.slowDeclinePending {
		t.Fatalf("the first-fill hold row is not the marker, got %+v", st)
	}

	marker := []aggragates.TradesLogs{{Message: SlowDeclineMessage("buy", nil), Price: slowDeclineLastFill}}
	inverse := testutil.LadderTrade(true, fills(SlowDeclineArmDepth, "17:38:00")...)
	inverse.Logs = marker
	noFill := testutil.LadderTrade(false)
	noFill.Logs = marker
	shallow := testutil.LadderTrade(false, fills(SlowDeclineArmDepth-1, "17:38:00")...)
	shallow.Logs = marker
	for _, unwatched := range []aggragates.Trades{inverse, noFill, shallow} {
		if st := rebuildState(unwatched); st.slowDeclineWatched || st.slowDeclinePending {
			t.Fatalf("a ladder the exit does not watch is never pending, got %+v", st)
		}
	}

	// SlowDeclineArmDepth filled entries are enough to be watched, and to be
	// pending on the marker, exactly as slowDeclineWatched reads the trade.
	shallowest := testutil.LadderTrade(false, fills(SlowDeclineArmDepth, "17:38:00")...)
	shallowest.Logs = marker
	if st := rebuildState(shallowest); !st.slowDeclineWatched || !st.slowDeclinePending || !slowDeclineWatched(shallowest) {
		t.Fatalf("a long ladder is watched from SlowDeclineArmDepth, got %+v", st)
	}
	if slowDeclineWatched(shallow) {
		t.Fatal("slowDeclineWatched must agree with rebuildState on the shallower ladder")
	}
}

// The slow-decline rows fold in slice order and the last one wins: a marker
// makes a watched ladder pending from the fill its price names, a cancel row
// makes it not pending. Rows of other kinds between them change nothing, a
// cancel row without a price is no row, and a ladder the exit does not watch
// is pending on none of them.
func TestRebuildStateFoldsTheSlowDeclineRowsInOrder(t *testing.T) {
	marker := func(price float64) aggragates.TradesLogs {
		return aggragates.TradesLogs{Message: SlowDeclineMessage("buy", slowDeclineReasons), Price: price}
	}
	cancel := func(price float64) aggragates.TradesLogs {
		return aggragates.TradesLogs{Message: SlowDeclineCancelMessage("buy", slowDeclineBreakReasons), Price: price}
	}
	other := aggragates.TradesLogs{Message: "Hold stopLoss: cooldown: depth held", Price: sixthFill}
	for name, tc := range map[string]struct {
		logs    []aggragates.TradesLogs
		pending bool
		from    float64
	}{
		"a marker":                                   {[]aggragates.TradesLogs{marker(pendingFromFifth)}, true, pendingFromFifth},
		"marker, cancel":                             {[]aggragates.TradesLogs{marker(pendingFromFifth), cancel(sixthFill)}, false, 0},
		"marker, cancel, marker":                     {[]aggragates.TradesLogs{marker(pendingFromFifth), cancel(sixthFill), marker(sixthFill)}, true, sixthFill},
		"marker, marker":                             {[]aggragates.TradesLogs{marker(pendingFromFifth), marker(sixthFill)}, true, sixthFill},
		"cancel, marker":                             {[]aggragates.TradesLogs{cancel(sixthFill), marker(pendingFromFifth)}, true, pendingFromFifth},
		"a cancel alone":                             {[]aggragates.TradesLogs{cancel(sixthFill)}, false, 0},
		"marker, other, cancel, other":               {[]aggragates.TradesLogs{marker(pendingFromFifth), other, cancel(sixthFill), other}, false, 0},
		"marker, a cancel without a price":           {[]aggragates.TradesLogs{marker(pendingFromFifth), cancel(0)}, true, pendingFromFifth},
		"marker, cancel, a marker without a price":   {[]aggragates.TradesLogs{marker(pendingFromFifth), cancel(sixthFill), marker(0)}, false, 0},
		"marker, cancel, marker, cancel, and marker": {[]aggragates.TradesLogs{marker(pendingFromFifth), cancel(sixthFill), marker(sixthFill), cancel(sixthFill), marker(sixthFill)}, true, sixthFill},
	} {
		trade := testutil.LadderTrade(false, fills(6, "23:09:00")...)
		trade.Logs = tc.logs
		st := rebuildState(trade)
		if st.slowDeclinePending != tc.pending || st.slowDeclinePendingFrom != tc.from {
			t.Errorf("%s: pending %v from %v, want %v from %v", name, st.slowDeclinePending, st.slowDeclinePendingFrom, tc.pending, tc.from)
		}

		shallow := testutil.LadderTrade(false, fills(SlowDeclineArmDepth-1, "17:38:00")...)
		shallow.Logs = tc.logs
		if st := rebuildState(shallow); st.slowDeclinePending || st.slowDeclinePendingFrom != 0 {
			t.Errorf("%s: a ladder the exit does not watch is never pending, got %+v", name, st)
		}
	}
}

// A trade carrying the retired rule's rows reads nothing from them: the state
// is the one the same trade has without them, at its last depth as short of
// it, and a slow-decline marker after them still folds.
func TestRebuildStateIgnoresTheRetiredRows(t *testing.T) {
	for _, trade := range []aggragates.Trades{watchedTrade(), lastDepthLadder()} {
		carrying := trade
		carrying.Logs = retiredRows(trade.PositionPrice)
		if got, want := rebuildState(carrying), rebuildState(trade); !reflect.DeepEqual(got, want) {
			t.Fatalf("the retired rows must read as nothing, got %+v want %+v", got, want)
		}
		carrying.Logs = append(carrying.Logs, aggragates.TradesLogs{Message: SlowDeclineMessage("buy", nil), Price: trade.PositionPrice})
		if st := rebuildState(carrying); !st.slowDeclinePending || st.slowDeclinePendingFrom != trade.PositionPrice {
			t.Fatalf("a marker after the retired rows still makes the ladder pending, got %+v", st)
		}
	}
}

// While QuietSlowDeclineExit is off the exit watches no ladder, so the rows it
// wrote earlier are ignored: a marker makes nothing pending.
func TestRebuildStateIgnoresTheSlowDeclineRowsWhileSwitchedOff(t *testing.T) {
	withQuietSlowDeclineExit(t, false)
	if st := rebuildState(pendingTrade()); st.slowDeclineWatched || st.slowDeclinePending || st.slowDeclinePendingFrom != 0 {
		t.Fatalf("switched off, the marker must be ignored, got %+v", st)
	}
}

// An indecision row latches a ladder the indecision direction watches, found
// anywhere in the message and only with a price, and nothing takes the latch
// away: no cancel row, no later marker, no row of another kind. It latches no
// ladder the rule does not watch — one short of IndecisionArmDepth, inverse,
// futures, with no fill — and while IndecisionDirection is off every such
// row is ignored. The slow-decline fold reads the same rows as it does
// without the latch: each rule folds its own rows alone.
func TestRebuildStateLatchesOnTheIndecisionRow(t *testing.T) {
	latch := func(price float64) aggragates.TradesLogs {
		return aggragates.TradesLogs{Message: IndecisionMessage("stopLoss", indecisionReasons), Price: price}
	}
	marker := aggragates.TradesLogs{Message: SlowDeclineMessage("buy", slowDeclineReasons), Price: pendingFromFifth}
	cancel := aggragates.TradesLogs{Message: SlowDeclineCancelMessage("buy", slowDeclineBreakReasons), Price: sixthFill}
	other := aggragates.TradesLogs{Message: "Hold stopLoss: cooldown: depth held", Price: sixthFill}
	for name, tc := range map[string]struct {
		logs             []aggragates.TradesLogs
		latched, pending bool
	}{
		"no row":                                  {nil, false, false},
		"an indecision row":                       {[]aggragates.TradesLogs{latch(sixthFill)}, true, false},
		"an indecision row without a price":       {[]aggragates.TradesLogs{latch(0)}, false, false},
		"the latch, then a marker and its cancel": {[]aggragates.TradesLogs{latch(sixthFill), marker, cancel}, true, false},
		"a marker, then the latch":                {[]aggragates.TradesLogs{marker, latch(sixthFill)}, true, true},
		"the latch among other rows":              {[]aggragates.TradesLogs{other, latch(sixthFill), other}, true, false},
		"a cancel, then the latch, then a marker": {[]aggragates.TradesLogs{cancel, latch(sixthFill), marker}, true, true},
	} {
		trade := testutil.LadderTrade(false, fills(6, "23:09:00")...)
		trade.Logs = tc.logs
		st := rebuildState(trade)
		if !st.indecisionWatched || st.indecision != tc.latched || st.slowDeclinePending != tc.pending {
			t.Errorf("%s: latched %v pending %v, want %v and %v (%+v)", name, st.indecision, st.slowDeclinePending, tc.latched, tc.pending, st)
		}
		without := trade
		without.Logs = nil
		for _, row := range tc.logs {
			if !strings.Contains(row.Message, IndecisionMarker) {
				without.Logs = append(without.Logs, row)
			}
		}
		if got, want := rebuildState(without), rebuildState(trade); got.slowDeclinePending != want.slowDeclinePending || got.slowDeclinePendingFrom != want.slowDeclinePendingFrom {
			t.Errorf("%s: the latch moved the slow-decline fold: %+v, want %+v", name, want, got)
		}
	}

	rows := []aggragates.TradesLogs{latch(slowDeclineLastFill)}
	shallow := testutil.LadderTrade(false, fills(IndecisionArmDepth-1, "17:38:00")...)
	inverse := testutil.LadderTrade(true, fills(watchedFills, "17:38:00")...)
	futures := watchedTrade()
	futures.Strategy.TradeType = aggragates.Futures
	noFill := testutil.LadderTrade(false)
	for name, trade := range map[string]aggragates.Trades{"one short of IndecisionArmDepth": shallow, "an inverse ladder": inverse, "a futures ladder": futures, "a ladder with no fill": noFill} {
		trade.Logs = rows
		if st := rebuildState(trade); st.indecisionWatched || st.indecision || indecisionWatched(trade) {
			t.Errorf("%s: a ladder the rule does not watch is never latched, got %+v", name, st)
		}
	}
	atDepth := testutil.LadderTrade(false, fills(IndecisionArmDepth, "17:38:00")...)
	atDepth.Logs = rows
	if st := rebuildState(atDepth); !st.indecisionWatched || !st.indecision || !indecisionWatched(atDepth) {
		t.Fatalf("a long spot ladder is watched and latched from IndecisionArmDepth, exactly as indecisionWatched reads it, got %+v", st)
	}

	withIndecisionDirection(t, false)
	if st := rebuildState(atDepth); st.indecisionWatched || st.indecision {
		t.Fatalf("switched off, the indecision row must be ignored, got %+v", st)
	}
}

// The two folds are independent: a ladder the indecision direction watches
// and the slow decline does not — that exit switched off — is latched on its
// row all the same, and a futures ladder the slow decline watches and the
// indecision direction does not goes pending on its marker all the same.
func TestRebuildStateFoldsEachRuleApart(t *testing.T) {
	rows := []aggragates.TradesLogs{
		{Message: SlowDeclineMessage("buy", nil), Price: slowDeclineLastFill},
		{Message: IndecisionMessage("buy", nil), Price: slowDeclineLastFill},
	}
	futures := watchedTrade()
	futures.Strategy.TradeType = aggragates.Futures
	futures.Logs = rows
	if st := rebuildState(futures); !st.slowDeclinePending || st.indecisionWatched || st.indecision {
		t.Fatalf("a futures ladder goes pending and is not latched, got %+v", st)
	}

	withQuietSlowDeclineExit(t, false)
	spot := watchedTrade()
	spot.Logs = rows
	if st := rebuildState(spot); st.slowDeclineWatched || st.slowDeclinePending || !st.indecision {
		t.Fatalf("with the slow decline switched off the ladder is latched and not pending, got %+v", st)
	}
}
