package smarttakeloss

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The package reads every switch as shipped, the slow pattern decline's
// included: every test that flips one has put it back. Which way each ships is
// the user's call, so no value is pinned here; a test that needs a rule off or
// on switches it for itself.
func TestSwitchesReadAsShipped(t *testing.T) {
	if quietSlowDeclineExit != QuietSlowDeclineExit {
		t.Fatalf("QuietSlowDeclineExit ships %v, read as %v", QuietSlowDeclineExit, quietSlowDeclineExit)
	}
	if capitalProtectionExit != CapitalProtectionExit {
		t.Fatalf("CapitalProtectionExit ships %v, read as %v", CapitalProtectionExit, capitalProtectionExit)
	}
	if indecisionDirection != IndecisionDirection {
		t.Fatalf("IndecisionDirection ships %v, read as %v", IndecisionDirection, indecisionDirection)
	}
	if slowDeclineNeedsRecentFill != SlowDeclineNeedsRecentFill {
		t.Fatalf("SlowDeclineNeedsRecentFill ships %v, read as %v", SlowDeclineNeedsRecentFill, slowDeclineNeedsRecentFill)
	}
	if depthPriorityHoldPauses != DepthPriorityHoldPausesSmartTakeLoss {
		t.Fatalf("DepthPriorityHoldPausesSmartTakeLoss ships %v, read as %v", DepthPriorityHoldPausesSmartTakeLoss, depthPriorityHoldPauses)
	}
	if slowPatternDeclineExit != SlowPatternDeclineExit {
		t.Fatalf("SlowPatternDeclineExit ships %v, read as %v", SlowPatternDeclineExit, slowPatternDeclineExit)
	}
}

// Armed is the predicate hermes and live-testing ask before their
// empty-position early return: the flag, a parent, and a ladder one of the
// four rules watches — the quiet slow decline a long ladder from
// SlowDeclineArmDepth filled entries, capital protection a long spot ladder
// outside an impasse strategy from its last depth, the indecision direction
// a long spot ladder from IndecisionArmDepth filled entries, the slow pattern
// decline a long spot ladder, an impasse strategy's included, from
// SlowPatternArmDepth filled entries. Each switch takes its own rule's watch
// away and nothing else; a child and a trade without the flag are never armed.
func TestArmed(t *testing.T) {
	futures := lastDepthLadder()
	futures.Strategy.TradeType = aggragates.Futures
	impasse := lastDepthLadder()
	impasse.Strategy.Params.Impasse = true
	child := lastDepthLadder()
	child.ParentID = 7
	off := lastDepthLadder()
	off.Strategy.Params.SmartTakeLoss = false

	type armedCase struct {
		name                                                    string
		trade                                                   aggragates.Trades
		slowDecline, capitalProtection, indecision, slowPattern bool
	}
	cases := []armedCase{
		{"a long ladder short of every watch", testutil.LadderTrade(false, fills(min(SlowDeclineArmDepth, IndecisionArmDepth)-1, "17:38:00")...), false, false, false, false},
		{"a long ladder at SlowDeclineArmDepth", testutil.LadderTrade(false, fills(SlowDeclineArmDepth, "17:38:00")...), true, false, SlowDeclineArmDepth >= IndecisionArmDepth, SlowDeclineArmDepth >= SlowPatternArmDepth},
		{"a long ladder one short of IndecisionArmDepth", testutil.LadderTrade(false, fills(IndecisionArmDepth-1, "17:38:00")...), IndecisionArmDepth-1 >= SlowDeclineArmDepth, false, false, IndecisionArmDepth-1 >= SlowPatternArmDepth},
		{"a long ladder at IndecisionArmDepth", testutil.LadderTrade(false, fills(IndecisionArmDepth, "17:38:00")...), IndecisionArmDepth >= SlowDeclineArmDepth, false, true, IndecisionArmDepth >= SlowPatternArmDepth},
		{"a long ladder one short of SlowPatternArmDepth", testutil.LadderTrade(false, fills(SlowPatternArmDepth-1, "17:38:00")...), true, false, true, false},
		{"a long ladder at SlowPatternArmDepth", testutil.LadderTrade(false, fills(SlowPatternArmDepth, "17:38:00")...), true, false, true, true},
		{"one depth short of the last", testutil.LadderTrade(false, fills(lastDepthFills-1, "21:30:00")...), true, false, true, true},
		{"a long ladder at its last depth", lastDepthLadder(), true, true, true, true},
		{"a futures ladder at its last depth", futures, true, false, false, false},
		{"an impasse strategy at its last depth", impasse, true, false, true, true},
		{"an inverse ladder at its last depth", testutil.LadderTrade(true, fills(lastDepthFills, "21:30:00")...), false, false, false, false},
		{"a ladder with no fill", testutil.LadderTrade(false), false, false, false, false},
	}
	// A row short of the slow decline's watch that is still at its last depth
	// needs a filled entry under SlowDeclineArmDepth: with the constant at one
	// the only ladder under it has no fill, so the case does not exist.
	if SlowDeclineArmDepth >= 2 {
		shortRow := testutil.LadderTrade(false, fills(SlowDeclineArmDepth-1, "17:38:00")...)
		shortRow.StrategyPair.StrategySettings[0].Depths = SlowDeclineArmDepth - 1
		cases = append(cases, armedCase{"a short row at its last depth", shortRow, false, true, false, false})
	}
	for _, switches := range gridSwitches() {
		withQuietSlowDeclineExit(t, switches[0])
		withCapitalProtectionExit(t, switches[1])
		withIndecisionDirection(t, switches[2])
		withSlowPatternDeclineExit(t, switches[3])
		for _, tc := range cases {
			want := (switches[0] && tc.slowDecline) || (switches[1] && tc.capitalProtection) || (switches[2] && tc.indecision) || (switches[3] && tc.slowPattern)
			if got := Armed(tc.trade); got != want {
				t.Errorf("switches %v, %s: armed %v, want %v", switches, tc.name, got, want)
			}
		}
		if Armed(child) || Armed(off) {
			t.Errorf("switches %v: a child and a trade without the flag are never armed", switches)
		}
	}
}

// A ladder the indecision direction alone watches — the other two rules
// switched off — is armed so its dead-zone ticks reach Apply, and the tick
// sophos serves the indecision on hands back the row from the empty
// proposal, sells nothing and keeps the proposal.
func TestArmedLadderGetsTheIndecisionRowFromTheDeadZone(t *testing.T) {
	withQuietSlowDeclineExit(t, false)
	withCapitalProtectionExit(t, false)
	withSlowPatternDeclineExit(t, false)
	trade := testutil.LadderTrade(false, fills(IndecisionArmDepth, "17:38:00")...)
	if !Armed(trade) {
		t.Fatal("the indecision direction alone must arm the ladder from IndecisionArmDepth")
	}
	got := Apply(trade, "", underTheBand, indecisionReading())
	if got.Position != "" || got.Reason != "" || got.SlowDecline != nil {
		t.Fatalf("the indecision row sells nothing and writes no slow-decline row, got %+v", got)
	}
	assertRow(t, got.Indecision, IndecisionMessage("buy", indecisionReasons), trade.PositionPrice)
}

// A ladder capital protection alone watches — capital protection switched on,
// the slow decline off — is armed so its dead-zone ticks reach Apply, and the
// tick at the band sells it from the empty proposal.
func TestArmedLadderSellsFromTheDeadZone(t *testing.T) {
	withCapitalProtectionExit(t, true)
	withQuietSlowDeclineExit(t, false)
	trade := lastDepthLadder()
	if !Armed(trade) {
		t.Fatal("capital protection alone must arm the ladder at its last depth")
	}
	assertForced(t, Apply(trade, "", capitalProtectionBand, withBlock(solBlock())), reasonCapitalProtection)
}

// heldBy is the trade carrying the pair the cooldown depth priority gate
// writes for a hold, stamped at, after the rows and events it already
// carries: the gate's text row, as cp shows it, and its event, the record the
// pause reads. The event is built with the constructor the gate's writers and
// every fixture share.
func heldBy(trade aggragates.Trades, at time.Time) aggragates.Trades {
	row := aggragates.TradesLogs{
		Message:   "Hold stopLoss: " + cooldown.DepthPriorityHoldMarker + ", ETH/USDT at depth 5 of 8 keeps the wallet for its remaining depths, this ladder waits at depth 4 of 8",
		Type:      aggragates.LOG_INFO,
		Price:     trade.PositionPrice,
		CreatedAt: at,
	}
	event := cooldown.NewDepthPriorityEvent(trade.ID, cooldown.DepthPriorityEvent{
		Event:            gates.EventHeld,
		PrioritySymbol:   "ETH/USDT",
		PriorityDepth:    5,
		PriorityMaxDepth: 8,
		Depth:            4,
		MaxDepth:         8,
	}, at)
	return aggragates.AppendStrategyRow(trade, row, event)
}

// A ladder the depth priority holds — the gate's event stamped after its
// newest fill — stays armed, and Apply pauses it: a pending exit is reset
// once, with a row at the newest fill, then nothing is marked or sold, at the
// sell band or at capital protection's band; the next fill ends the hold and
// the verdict marks the ladder pending again. An event stamped before the
// newest fill, or without a stamp, holds nothing. The reset marker neither
// holds nor is held by any other marker, the gate's included: the texts stay
// apart for the filters that find a row by them.
func TestAHeldLadderStaysArmedAndApplyPausesIt(t *testing.T) {
	withCapitalProtectionExit(t, true)
	held := heldBy(pendingTrade(), testutil.At("18:05:00"))
	if !Armed(held) || !rebuildState(held).depthPriorityHeld {
		t.Fatal("a held ladder stays armed and reads held")
	}
	reset, got := engineTick(held, "", slowDeclineBand+1, testutil.At("18:10:00"), slowDeclineBlock(true))
	assertRow(t, got.SlowDecline, SlowDeclineResetMessage("buy"), slowDeclineLastFill)
	assertNoSale(t, got, "")
	if st := rebuildState(reset); st.slowDeclinePending || !st.slowDeclineWatched {
		t.Fatalf("the reset row leaves the ladder watched and not pending, got %+v", st)
	}
	for _, position := range []string{"", "stopLoss"} {
		assertUntouched(t, Apply(reset, position, slowDeclineBand+1, slowDeclineBlock(true)), position)
	}
	assertUntouched(t, Apply(heldBy(lastDepthLadder(), testutil.At("21:45:00")), "", capitalProtectionBand, withBlock(solBlock())), "")

	refilled := withFill(reset, 179.78, testutil.At("18:30:00"))
	assertRow(t, Apply(refilled, "", underTheBand, slowDeclineBlock(true)).SlowDecline, SlowDeclineMessage("buy", slowDeclineReasons), 179.78)
	for _, at := range []time.Time{testutil.At("17:00:00"), {}} {
		if rebuildState(heldBy(pendingTrade(), at)).depthPriorityHeld {
			t.Fatalf("an event stamped %v holds nothing", at)
		}
	}
	markers := []string{SlowDeclineMarker, SlowDeclineCancelMarker, SlowDeclineResetMarker, IndecisionMarker, SlowPatternMarker, SlowPatternCancelMarker, cooldown.DepthPriorityHoldMarker}
	for _, one := range markers {
		for _, other := range markers {
			if one != other && strings.Contains(one, other) {
				t.Errorf("%q contains %q", one, other)
			}
		}
	}
}
