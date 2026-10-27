package smarttakeloss

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/freefallprevention"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/quietdecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The package reads every switch as shipped, the swing decline's and the linear
// decline's included: every test that flips one has put it back. Which way each
// ships is the user's call, so no value is pinned here; TestMain pins the profile
// the tests run on, so the shipped leaf values and the mirrors read through them
// are the ones captured at init (shippedLeaf, shippedRead).
func TestSwitchesReadAsShipped(t *testing.T) {
	leaf, read := shippedLeaf, shippedRead
	for _, check := range []struct {
		name         string
		shipped, got any
	}{
		{"quietdecline.Enabled", leaf.quiet, read.quiet},
		{"swingdecline.Enabled", leaf.swing, read.swing},
		{"earlytakeprofit.Enabled", leaf.early, read.early},
		{"lineardecline.Enabled", leaf.linear, read.linear},
		{"flatmarket.Enabled", leaf.flat, read.flat},
		{"capitalprotection.Enabled", leaf.capital, read.capital},
		{"quietdecline.NeedsRecentFill", leaf.recentFill, read.recentFill},
		{"PauseWhenActiveDepthPriority", leaf.pause, read.pause},
		{"earlytakeprofit.QuietDecline", leaf.latchQuiet, read.latchQuiet},
		{"earlytakeprofit.SwingDecline", leaf.latchSwing, read.latchSwing},
		{"earlytakeprofit.LinearDecline", leaf.latchLinear, read.latchLinear},
		{"earlytakeprofit.FlatMarket", leaf.latchFlat, read.latchFlat},
		{"earlytakeprofit.FreeFallPrevention", leaf.latchFreeFall, read.latchFreeFall},
		{"earlytakeprofit.DepthPriority", leaf.latchDepthPriority, read.latchDepthPriority},
		{"earlytakeprofit.DynamicParams", leaf.latchDynamicParams, read.latchDynamicParams},
		{"flatmarket.HoldInitialDepth", leaf.flatHold, read.flatHold},
		{"flatmarket.ResetOnNewDepth", leaf.flatReset, read.flatReset},
		{"swingdecline.ResetOnNewDepth", leaf.swingReset, read.swingReset},
		{"lineardecline.ResetOnNewDepth", leaf.linearReset, read.linearReset},
		{"freefallprevention.Enabled", leaf.freeFall, read.freeFall},
		{"freefallprevention.HoldInitialDepth", leaf.freeFallHold, read.freeFallHold},
		{"freefallprevention.ResetOnNewDepth", leaf.freeFallReset, read.freeFallReset},
		{"capitalprotection.MaxDrawdownPercentage", leaf.drawdown, read.drawdown},
		{"WaitAfterTrigger", leaf.wait, read.wait},
		{"capitalprotection.WaitAfterTrigger", leaf.capitalWait, read.capitalWait},
		{"SellLossStartAtDepth", leaf.startAtDepth, read.startAtDepth},
		{"SellLossIgnoreLastDepths", leaf.ignoreLastDepths, read.ignoreLastDepths},
		{"SellLossTriggers", leaf.triggers, read.triggers},
	} {
		if !reflect.DeepEqual(check.shipped, check.got) {
			t.Fatalf("%s ships %v, read as %v", check.name, check.shipped, check.got)
		}
	}
}

// Armed is the predicate hermes and live-testing ask before their
// empty-position early return: the flag, a parent, and a ladder one of the
// rules watches — the slow decline gate, through the quiet decline a long spot
// ladder from quietdecline.ArmDepth filled entries and through the
// swing decline a long spot ladder, an impasse strategy's included, from
// swingdecline.ArmDepth; capital protection a spot ladder, long or inverse,
// outside an impasse strategy from its last depth; the early take profit a long spot
// ladder from earlytakeprofit.ArmDepth filled entries; the flat market gate
// every spot ladder from its first fill, an inverse one included. Each switch
// takes its own feature's watch away and nothing else; the linear decline's takes
// none away, since it watches no ladder of its own — it reads the ones the
// early take profit watches — so no case moves with it, and the grid runs the
// five switches to say so, each under the flat market gate on and off. A ladder
// the older rules do not watch reads armed exactly when it is a spot one with a
// filled entry and flatmarket.Enabled is on. A futures ladder is watched by
// none of them, the quiet decline's included, whatever the switches say. A child
// and a trade without the flag are never armed.
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
		name                                          string
		trade                                         aggragates.Trades
		quiet, swing, capitalProtection, indecisionOn bool
	}
	cases := []armedCase{
		{"a long ladder short of every watch", testutil.LadderTrade(false, fills(min(quietdecline.ArmDepth, earlytakeprofit.ArmDepth)-1, "17:38:00")...), false, false, false, false},
		{"a long ladder at quietdecline.ArmDepth", testutil.LadderTrade(false, fills(quietdecline.ArmDepth, "17:38:00")...), true, quietdecline.ArmDepth >= swingdecline.ArmDepth, false, quietdecline.ArmDepth >= earlytakeprofit.ArmDepth},
		{"a long ladder one short of earlytakeprofit.ArmDepth", testutil.LadderTrade(false, fills(earlytakeprofit.ArmDepth-1, "17:38:00")...), earlytakeprofit.ArmDepth-1 >= quietdecline.ArmDepth, earlytakeprofit.ArmDepth-1 >= swingdecline.ArmDepth, false, false},
		{"a long ladder at earlytakeprofit.ArmDepth", testutil.LadderTrade(false, fills(earlytakeprofit.ArmDepth, "17:38:00")...), earlytakeprofit.ArmDepth >= quietdecline.ArmDepth, earlytakeprofit.ArmDepth >= swingdecline.ArmDepth, false, true},
		{"a long ladder one short of swingdecline.ArmDepth", testutil.LadderTrade(false, fills(swingdecline.ArmDepth-1, "17:38:00")...), swingdecline.ArmDepth-1 >= quietdecline.ArmDepth, false, false, swingdecline.ArmDepth-1 >= earlytakeprofit.ArmDepth},
		{"a long ladder at swingdecline.ArmDepth", testutil.LadderTrade(false, fills(swingdecline.ArmDepth, "17:38:00")...), swingdecline.ArmDepth >= quietdecline.ArmDepth, true, false, swingdecline.ArmDepth >= earlytakeprofit.ArmDepth},
		{"one depth short of the last", testutil.LadderTrade(false, fills(lastDepthFills-1, "21:30:00")...), true, true, false, true},
		{"a long ladder at its last depth", lastDepthLadder(), true, true, true, true},
		{"a futures ladder at its last depth", futures, false, false, false, false},
		{"an impasse strategy at its last depth", impasse, true, true, false, true},
		{"an inverse ladder at its last depth", fundsRefused(testutil.LadderTrade(true, fills(lastDepthFills, "21:30:00")...)), false, false, true, false},
		{"a long ladder at its last depth, funds not refused", testutil.LadderTrade(false, fills(lastDepthFills, "21:30:00")...), true, true, false, true},
		{"an inverse ladder with one fill", testutil.LadderTrade(true, fills(1, "17:38:00")...), false, false, false, false},
		{"a ladder with no fill", testutil.LadderTrade(false), false, false, false, false},
	}
	// A row short of the quiet decline's watch that is still at its last depth
	// needs a filled entry under quietdecline.ArmDepth: with the
	// constant at one the only ladder under it has no fill, so the case does
	// not exist.
	if quietdecline.ArmDepth >= 2 {
		shortRow := testutil.LadderTrade(false, fills(quietdecline.ArmDepth-1, "17:38:00")...)
		shortRow.StrategyPair.StrategySettings[0].Depths = float64(quietdecline.ArmDepth - 1)
		cases = append(cases, armedCase{"a short row at its last depth", shortRow, false, quietdecline.ArmDepth-1 >= swingdecline.ArmDepth, true, quietdecline.ArmDepth-1 >= earlytakeprofit.ArmDepth})
	}
	// The flat market gate's watch, stated apart from the code: a spot ladder
	// holding a filled entry, of either side and at any depth.
	flatMarketWatches := func(trade aggragates.Trades) bool {
		return trade.Strategy.TradeType != aggragates.Futures && ladder.CountFilledEntries(trade) > 0
	}
	for _, flatMarketOn := range []bool{true, false} {
		withFlatMarket(t, flatMarketOn)
		for _, switches := range gridSwitches() {
			setGridSwitches(t, switches)
			for _, tc := range cases {
				// switches[4], the linear decline's latch, is in no term: a latch arms nothing.
				want := (switches[0] && tc.quiet) || (switches[3] && tc.swing) || (switches[1] && tc.capitalProtection) || (switches[2] && tc.indecisionOn) || (flatMarketOn && flatMarketWatches(tc.trade))
				if got := Armed(tc.trade); got != want {
					t.Errorf("switches %v, flat market %v, %s: armed %v, want %v", switches, flatMarketOn, tc.name, got, want)
				}
			}
			if Armed(child) || Armed(off) {
				t.Errorf("switches %v, flat market %v: a child and a trade without the flag are never armed", switches, flatMarketOn)
			}
		}
	}
}

// The flat market gate watches what no other rule does: with the five older
// switches off, a spot ladder holding a fill — a long one and an inverse one, at
// its first depth and at its last — is armed while flatmarket.Enabled is on,
// and is not armed with it off. A ladder with no fill has nothing to carry its
// rows, a futures ladder is never read, and a child and a trade without the flag
// are never armed either way.
func TestArmedByTheFlatMarketGateAlone(t *testing.T) {
	setGridSwitches(t, [5]bool{})
	watched := map[string]aggragates.Trades{
		"a long ladder with one fill":         testutil.LadderTrade(false, fills(1, "17:38:00")...),
		"an inverse ladder with one fill":     testutil.LadderTrade(true, fills(1, "17:38:00")...),
		"a long ladder at its last depth":     lastDepthLadder(),
		"an inverse ladder at its last depth": testutil.LadderTrade(true, fills(lastDepthFills, "21:30:00")...),
		"the round inverse ladder":            flatMarketLadder(true),
	}
	futures, child, unflagged := lastDepthLadder(), lastDepthLadder(), lastDepthLadder()
	futures.Strategy.TradeType = aggragates.Futures
	child.ParentID = 7
	unflagged.Strategy.Params.SmartTakeLoss = false
	never := map[string]aggragates.Trades{
		"a ladder with no fill":    testutil.LadderTrade(false),
		"a futures ladder":         futures,
		"a child":                  child,
		"a trade without the flag": unflagged,
	}

	withFlatMarket(t, true)
	for name, trade := range watched {
		if !Armed(trade) || !rebuildState(trade).flatMarketWatched {
			t.Errorf("%s: the flat market gate alone must arm it", name)
		}
	}
	for name, trade := range never {
		if Armed(trade) {
			t.Errorf("%s: is never armed by the flat market gate", name)
		}
	}
	withFlatMarket(t, false)
	for name, trade := range watched {
		if Armed(trade) || rebuildState(trade).flatMarketWatched {
			t.Errorf("%s: with flatmarket.Enabled off nothing watches it", name)
		}
	}
}

// The free fall prevention gate watches what no other rule does and from its own depth: with the
// five older switches and the flat market off, a spot ladder holding freefallprevention.ArmDepth
// filled entries — a long one and an inverse one, at that depth and at its last — is armed while
// freefallprevention.Enabled is on, and is not armed with it off. A ladder one fill short of the
// depth or with none, a futures ladder, a child and a trade without the flag are never armed.
func TestArmedByTheFreeFallPreventionGateAlone(t *testing.T) {
	setGridSwitches(t, [5]bool{})
	withFlatMarket(t, false)
	watched := map[string]aggragates.Trades{
		"a long ladder at the arm depth":      testutil.LadderTrade(false, fills(freefallprevention.ArmDepth, "17:38:00")...),
		"an inverse ladder at the arm depth":  testutil.LadderTrade(true, fills(freefallprevention.ArmDepth, "17:38:00")...),
		"a long ladder at its last depth":     lastDepthLadder(),
		"an inverse ladder at its last depth": testutil.LadderTrade(true, fills(lastDepthFills, "21:30:00")...),
		"the round inverse ladder":            flatMarketLadder(true),
	}
	futures, child, unflagged := lastDepthLadder(), lastDepthLadder(), lastDepthLadder()
	futures.Strategy.TradeType = aggragates.Futures
	child.ParentID = 7
	unflagged.Strategy.Params.SmartTakeLoss = false
	never := map[string]aggragates.Trades{
		"a ladder one fill short of the arm depth": testutil.LadderTrade(false, fills(freefallprevention.ArmDepth-1, "17:38:00")...),
		"a ladder with no fill":                    testutil.LadderTrade(false),
		"a futures ladder":                         futures,
		"a child":                                  child,
		"a trade without the flag":                 unflagged,
	}

	withFreeFallPrevention(t, true)
	for name, trade := range watched {
		if !Armed(trade) || !rebuildState(trade).freeFallPreventionWatched {
			t.Errorf("%s: the free fall prevention gate alone must arm it", name)
		}
	}
	for name, trade := range never {
		if Armed(trade) {
			t.Errorf("%s: is never armed by the free fall prevention gate", name)
		}
	}
	withFreeFallPrevention(t, false)
	for name, trade := range watched {
		if Armed(trade) || rebuildState(trade).freeFallPreventionWatched {
			t.Errorf("%s: with freefallprevention.Enabled off nothing watches it", name)
		}
	}
}

// A ladder the early take profit alone watches — the other features
// switched off — is armed so its dead-zone ticks reach Apply, and the tick
// sophos serves the indecision on hands back the latch row from the empty
// proposal, sells nothing and keeps the proposal.
func TestArmedLadderGetsTheEarlyTakeProfitRowFromTheDeadZone(t *testing.T) {
	withEarlyTakeProfit(t, true)
	withQuietDecline(t, false)
	withCapitalProtectionExit(t, false)
	withSwingDecline(t, false)
	trade := testutil.LadderTrade(false, fills(earlytakeprofit.ArmDepth, "17:38:00")...)
	if !Armed(trade) {
		t.Fatal("the early take profit alone must arm the ladder from earlytakeprofit.ArmDepth")
	}
	got := Apply(trade, "", underTheBand, indecisionReading())
	assertNoSale(t, got, "")
	assertRow(t, got, LatchedRow(trade.PositionPrice, indecisionFinding(indecisionReasons)))
}

// A ladder capital protection alone watches — capital protection switched on,
// the slow decline gate's features off — is armed so its dead-zone ticks reach
// Apply, and the tick the trend reads against it on opens its episode from the
// empty proposal: the triggered row, then the held row of a loss over the drawdown.
func TestArmedLadderIsTriggeredFromTheDeadZone(t *testing.T) {
	capitalProtectionAlone(t)
	trade := lastDepthLadder()
	if !Armed(trade) {
		t.Fatal("capital protection alone must arm the ladder at its last depth")
	}
	got := Apply(trade, "", lastDepthPrint, withBlock(solBlock()))
	assertWaits(t, got, "", capitalProtectionTriggered(trade), CapitalProtectionSaleHeldRow(trade.PositionPrice, -handLossPct(trade, lastDepthPrint)))
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
// newest fill — stays armed, and Apply pauses it: a pending gate is reset
// once, with a row at the newest fill, then nothing is marked, activated or
// sold, in buy or in stopLoss, on an arming; capital protection, immune to the
// pause, still opens its episode on a held ladder at its last depth. The next
// fill ends the hold and the verdict marks the ladder pending again, the
// shared action activating its stopLoss on the same tick at a loss. An event
// stamped before the newest fill, or without a stamp, holds nothing. The reset
// row carries no text of the depth priority gate, nor the gate's row any of the
// reset's: the texts stay apart for the filters that find a row by them.
func TestAHeldLadderStaysArmedAndApplyPausesIt(t *testing.T) {
	withQuietDecline(t, true)
	withCapitalProtectionExit(t, true)
	held := heldBy(pendingTrade(), testutil.At("18:05:00"))
	if !Armed(held) || !rebuildState(held).depthPriorityHeld {
		t.Fatal("a held ladder stays armed and reads held")
	}
	resetRow := ResetRow(slowDeclineLastFill, quietFinding(nil))
	reset, got := engineTick(held, "", slowDeclineBand+1, testutil.At("18:10:00"), slowDeclineBlock(true))
	assertRow(t, got, resetRow)
	assertNoSale(t, got, "")
	if st := rebuildState(reset); st.slowDeclinePending || !st.slowDeclineWatched {
		t.Fatalf("the reset row leaves the ladder watched and not pending, got %+v", st)
	}
	for _, state := range []string{"buy", "stopLoss"} {
		for _, position := range []string{"", "stopLoss"} {
			assertUntouched(t, Apply(restingIn(reset, state), position, slowDeclineBand+1, slowDeclineBlock(true)), position)
		}
	}
	last := lastDepthLadder()
	assertWaits(t, Apply(heldBy(last, testutil.At("21:45:00")), "", lastDepthPrint, withBlock(solBlock())), "",
		capitalProtectionTriggered(last), CapitalProtectionSaleHeldRow(last.PositionPrice, -handLossPct(last, lastDepthPrint)))

	refilled := withFill(reset, 179.78, testutil.At("18:30:00"))
	assertRow(t, applyAtAGain(refilled, "", slowDeclineBlock(true)), quietPending(179.78))
	assertActivated(t, Apply(refilled, "", underTheBand, slowDeclineBlock(true)), quietPending(179.78), quietActivation(refilled, underTheBand))
	for _, at := range []time.Time{testutil.At("17:00:00"), {}} {
		if rebuildState(heldBy(pendingTrade(), at)).depthPriorityHeld {
			t.Fatalf("an event stamped %v holds nothing", at)
		}
	}
	gate := heldBy(watchedTrade(), testutil.At("18:05:00")).Logs[0].Message
	if strings.Contains(resetRow.Message, cooldown.DepthPriorityHoldMarker) || strings.Contains(gate, resetRow.Message) {
		t.Errorf("the reset row %q and the gate's row %q name each other", resetRow.Message, gate)
	}
}
