package smartsell

import (
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// withEarlyTakeProfitQuietDecline is withEarlyTakeProfit for the latch on the quiet decline.
func withEarlyTakeProfitQuietDecline(t *testing.T, on bool) {
	t.Helper()
	previous := earlyTakeProfitQuietDecline
	earlyTakeProfitQuietDecline = on
	t.Cleanup(func() { earlyTakeProfitQuietDecline = previous })
}

// The quiet decline latches on the row of the tick it makes the ladder pending with and, the gate
// off, on a tick with no row when sophos' verdict reads for the ladder or its look-back marks it;
// the switch off, nothing latches on either path.
func TestTheQuietDeclineLatchesOnItsRowAndOnAWatchedLadderWithTheGateOff(t *testing.T) {
	withTheFeaturesOff(t)
	withEarlyTakeProfitQuietDecline(t, true)
	trade := watchedTrade()
	verdict := slowDeclineBlock(true)
	lookBack := withRecent(slowDeclineBlock(false), testutil.At("20:00:00"), testutil.At("00:00:00"))

	withQuietDecline(t, true)
	got := Apply(withRestingOrder(trade), "", gainPrint(trade), verdict)
	assertRow(t, got, PendingRow(slowDeclineLastFill, latchMarker, quietFinding(slowDeclineReasons)))
	if !rebuildState(withRows(trade, got.Rows...)).earlyTakeProfit {
		t.Fatal("the quiet decline's row carries the latch")
	}

	withQuietDecline(t, false)
	got = Apply(withRestingOrder(trade), "", gainPrint(trade), verdict)
	assertRow(t, got, LatchedRow(slowDeclineLastFill, Finding{Feature: FeatureQuietDecline, Reasons: slowDeclineReasons}))
	got = Apply(withRestingOrder(trade), "", gainPrint(trade), lookBack)
	assertRow(t, got, LatchedRow(slowDeclineLastFill, Finding{Feature: FeatureQuietDecline, Reasons: lookBack.SmartSell.QuietDeclineRecentReasons}))
	if latchNamed(Apply(withRestingOrder(trade), "", gainPrint(trade), slowDeclineBlock(false))) {
		t.Fatal("neither the verdict nor the look-back, the quiet decline latches nothing")
	}

	withEarlyTakeProfitQuietDecline(t, false)
	assertUntouched(t, Apply(withRestingOrder(trade), "", gainPrint(trade), verdict), "")
	withQuietDecline(t, true)
	got = Apply(withRestingOrder(trade), "", gainPrint(trade), verdict)
	assertRow(t, got, quietPending(slowDeclineLastFill))
	if rebuildState(withRows(trade, got.Rows...)).earlyTakeProfit {
		t.Fatal("the switch off, the quiet decline's row latches nothing")
	}
}

// Events written before the latch was named carry "indecision" as the latch: a pending row's
// features and a latched row's both latch the ladder still; a pending row without it never did.
func TestThePreRenameIndecisionMarkerStillLatches(t *testing.T) {
	withEarlyTakeProfit(t, true)
	withQuietDecline(t, true)
	trade := watchedTrade()
	at := testutil.At("23:00:00")
	event := func(data EventData) aggragates.Trades {
		return withEvents(trade, aggragates.NewStrategyEvent(trade.ID, aggragates.StrategyParamSmartSell, GateSlowDecline, data, at))
	}
	for name, data := range map[string]EventData{
		"a pending row carrying the old marker": {Event: EventPending, Price: slowDeclineLastFill, Features: []string{"indecision", FeatureQuietDecline}},
		"a latched row carrying the old marker": {Event: EventLatched, Price: slowDeclineLastFill, Features: []string{"indecision"}},
		"a pending row carrying the latch":      {Event: EventPending, Price: slowDeclineLastFill, Features: []string{FeatureEarlyTakeProfit, FeatureQuietDecline}},
	} {
		if !EarlyTakeProfitLatched(event(data)) {
			t.Errorf("%s: latches still", name)
		}
	}
	if EarlyTakeProfitLatched(event(EventData{Event: EventPending, Price: slowDeclineLastFill, Features: []string{FeatureQuietDecline}})) {
		t.Error("a pending row without the marker latches nothing")
	}
}
