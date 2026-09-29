package actions

import (
	"errors"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// crashGuardAnchor is the `buy` anchor of testutil.DeepTrade: its last fill.
const crashGuardAnchor = 94.0

// crashGuardEvent is the deep fixture ladder under CrashGuard on a stopLoss
// transition from oldPosition, the tick at `tick`.
func crashGuardEvent(trade aggragates.Trades, oldPosition string, tick float64, ai aggragates.AIIndicators) events.Events {
	trade.PositionPrice = tick
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: oldPosition, OldPositionPrice: crashGuardAnchor, AIIndicators: ai},
	}
}

// slowDeclineVerdict is a slow decline, with or without free fall, on a
// payload whose regime block would allow the add by itself.
func slowDeclineVerdict(freeFall bool) aggragates.AIIndicators {
	return aggragates.AIIndicators{
		HasRegimeVerdict: true,
		AddAllowed:       true,
		SlowDecline:      true,
		FreeFall:         freeFall,
	}
}

// ladderArmTick is where the fixture ladder arms its next depth on its own:
// one step, −(p + t), under the anchor.
func ladderArmTick(trade aggragates.Trades) float64 {
	row := trade.StrategyPair.StrategySettings[0]
	return crashGuardAnchor / (1 + (row.Percentage+row.Tolerance)/100)
}

// parkedLevel is where the held depth arms: SlowDeclineDepthFactor steps.
func parkedLevel(trade aggragates.Trades) float64 {
	row := trade.StrategyPair.StrategySettings[0]
	return crashGuardAnchor / (1 + (crashguard.SlowDeclineDepthFactor*row.Percentage+row.Tolerance)/100)
}

func lastMessage(event events.Events) string {
	if len(event.Trade.Logs) == 0 {
		return ""
	}
	return event.Trade.Logs[len(event.Trade.Logs)-1].Message
}

// A slow decline alone holds the arming of a deep ladder's next depth where
// the ladder would take it, and lets it arm SlowDeclineDepthFactor steps down.
func TestShouldHoldCrashGuardParksTheNextDepthUntilItsLevel(t *testing.T) {
	trade := testutil.DeepTrade(true)

	held, err := ShouldHold(crashGuardEvent(trade, "buy", ladderArmTick(trade), slowDeclineVerdict(false)))
	if err == nil {
		t.Fatal("a slow decline must hold the next depth at the ladder's own arm level")
	}
	if !strings.Contains(lastMessage(held), crashguard.SlowDeclineHoldPrefix+", next depth parked until") {
		t.Fatalf("expected the parked-depth hold, got %q", lastMessage(held))
	}
	if held.Trade.PositionType != "buy" || held.Trade.PositionPrice != crashGuardAnchor {
		t.Fatalf("a held arming must leave the ladder on its old position and anchor, got %s @ %v",
			held.Trade.PositionType, held.Trade.PositionPrice)
	}

	if _, err := ShouldHold(crashGuardEvent(trade, "buy", parkedLevel(trade), slowDeclineVerdict(false))); err != nil {
		t.Fatalf("the depth must arm once the price reaches its level, got %v", err)
	}
}

// Without free fall only the arming is held: the trailing re-anchor of a
// depth already armed moves as it always does.
func TestShouldHoldCrashGuardSlowDeclineAloneLetsTheReAnchorThrough(t *testing.T) {
	trade := testutil.DeepTrade(true)
	if _, err := ShouldHold(crashGuardEvent(trade, "stopLoss", 90, slowDeclineVerdict(false))); err != nil {
		t.Fatalf("a slow decline without free fall must not hold the re-anchor, got %v", err)
	}
}

// Slow decline with free fall holds every stopLoss transition: the arming at
// any price, the trailing re-anchor and the force-trailing one.
func TestShouldHoldCrashGuardFreeFallHoldsEveryStopLossTransition(t *testing.T) {
	cases := []struct {
		name         string
		positionType string
		oldPosition  string
		tick         float64
	}{
		{"arming far under the parked level", "stopLoss", "buy", 70},
		{"trailing re-anchor", "stopLoss", "stopLoss", 90},
		{"force-trailing re-anchor", "forceTrailingStopLoss", "forceTrailingStopLoss", 90},
	}
	for _, c := range cases {
		trade := testutil.DeepTrade(true)
		trade.PositionType = c.positionType
		held, err := ShouldHold(crashGuardEvent(trade, c.oldPosition, c.tick, slowDeclineVerdict(true)))
		if err == nil {
			t.Fatalf("%s: free fall must hold", c.name)
		}
		if want := "Hold " + c.positionType + ": " + crashguard.FreeFallHoldReason; lastMessage(held) != want {
			t.Fatalf("%s: got %q, want %q", c.name, lastMessage(held), want)
		}
	}
}

// The hold starts at DeRiskMinDepth filled entries: the shallow, cheap fills
// still trade.
func TestShouldHoldCrashGuardNeedsTheDepth(t *testing.T) {
	shallow := testutil.DeepTrade(true)
	shallow.History = shallow.History[:crashguard.DeRiskMinDepth-1]
	if _, err := ShouldHold(crashGuardEvent(shallow, "stopLoss", 90, slowDeclineVerdict(true))); err != nil {
		t.Fatalf("under the depth nothing may hold, got %v", err)
	}

	deep := testutil.DeepTrade(true)
	if _, err := ShouldHold(crashGuardEvent(deep, "stopLoss", 90, slowDeclineVerdict(true))); err == nil {
		t.Fatal("at the depth free fall must hold")
	}
}

// The verdict reads a falling market; an inverse ladder is not the side it
// traps, so it is never held.
func TestShouldHoldCrashGuardNeverHoldsAnInverseLadder(t *testing.T) {
	trade := testutil.DeepTrade(true)
	trade.Inverse = true
	for i := range trade.History {
		trade.History[i].Type = "SELL"
	}
	for _, oldPosition := range []string{"buy", "stopLoss"} {
		if _, err := ShouldHold(crashGuardEvent(trade, oldPosition, 110, slowDeclineVerdict(true))); err != nil {
			t.Fatalf("an inverse ladder was held from %s: %v", oldPosition, err)
		}
	}
}

// No slow-decline verdict, no hold: neither free fall alone nor the crash
// score holds anything any more.
func TestShouldHoldCrashGuardWithoutASlowDeclineHoldsNothing(t *testing.T) {
	trade := testutil.DeepTrade(true)
	verdicts := []aggragates.AIIndicators{
		{},
		{HasRegimeVerdict: true, AddAllowed: true, FreeFall: true},
		{HasRegimeVerdict: true, AddAllowed: true, CrashActive: true, CrashScore: 90},
	}
	for i, ai := range verdicts {
		for _, oldPosition := range []string{"buy", "stopLoss"} {
			if _, err := ShouldHold(crashGuardEvent(trade, oldPosition, ladderArmTick(trade), ai)); err != nil {
				t.Fatalf("verdict %d from %s held: %v", i, oldPosition, err)
			}
		}
	}
}

// The guard holds capital, never a profitable close.
func TestShouldHoldCrashGuardNeverHoldsAProfitExit(t *testing.T) {
	trade := testutil.DeepTrade(true)
	trade.PositionType = "takeProfit"
	if _, err := ShouldHold(crashGuardEvent(trade, "buy", 105, slowDeclineVerdict(true))); err != nil {
		t.Fatalf("crash guard must not hold takeProfit, got %v", err)
	}
}

// The ARMED row a trade carries is history, not state: the hold reads only
// the live verdict, so a trade that once logged an ARM is free once the
// verdict clears.
func TestShouldHoldCrashGuardIsNotStickyOnTheArmedRow(t *testing.T) {
	trade := testutil.DeepTrade(true)
	trade.Logs = []aggragates.TradesLogs{{
		Message: crashguard.TransitionMessage(slowDeclineVerdict(true)),
	}}
	if _, err := ShouldHold(crashGuardEvent(trade, "stopLoss", 90, aggragates.AIIndicators{HasRegimeVerdict: true, AddAllowed: true})); err != nil {
		t.Fatalf("a cleared verdict must not hold on an old ARMED row, got %v", err)
	}
}

// While the hold stands the ladder stays on `buy` at its old anchor, so the
// parked level — and the reason naming it — is the same on every tick: the
// first tick writes the row, every later one collapses onto it and tells the
// engine nothing was persisted.
func TestShouldHoldCrashGuardParkedDepthLogsOnceWhileItStands(t *testing.T) {
	trade := testutil.DeepTrade(true)
	held, err := ShouldHold(crashGuardEvent(trade, "buy", ladderArmTick(trade), slowDeclineVerdict(false)))
	if err == nil || len(held.Trade.Logs) != 1 {
		t.Fatalf("the first held tick must write one row, got %v with %d rows", err, len(held.Trade.Logs))
	}

	next := held.Trade
	next.PositionType = "stopLoss" // the ladder proposes the arming again
	again, err := ShouldHold(crashGuardEvent(next, "buy", (ladderArmTick(trade)+parkedLevel(trade))/2, slowDeclineVerdict(false)))
	if !errors.Is(err, events.ErrHoldNotPersisted) {
		t.Fatalf("a later tick of the same hold must collapse onto its row, got %v", err)
	}
	if len(again.Trade.Logs) != 1 || lastMessage(again) != lastMessage(held) {
		t.Fatalf("the hold wrote a second row: %d rows, last %q", len(again.Trade.Logs), lastMessage(again))
	}
}

// Under RegimeHold and CrashGuard together the crash guard's reason replaces
// the regime add veto while the depth is parked, and once the price pays the
// widened level the regime veto still stands: the release is the crash guard
// stepping aside, and capitulation never bypasses either on a deep ladder.
func TestShouldHoldCrashGuardParkedDepthOverRegimeAddVeto(t *testing.T) {
	trade := testutil.DeepTrade(true)
	trade.Strategy.Params.RegimeHold = true
	verdict := slowDeclineVerdict(false)
	verdict.AddAllowed = false

	held, err := ShouldHold(crashGuardEvent(trade, "buy", ladderArmTick(trade), verdict))
	if err == nil || !strings.Contains(lastMessage(held), crashguard.SlowDeclineHoldPrefix+", next depth parked until") {
		t.Fatalf("above the level the crash guard's reason must stand, got %v %q", err, lastMessage(held))
	}

	held, err = ShouldHold(crashGuardEvent(trade, "buy", parkedLevel(trade), verdict))
	if err == nil || !strings.Contains(lastMessage(held), regime.AddVetoPrefix) {
		t.Fatalf("at the level the regime veto must still hold, got %v %q", err, lastMessage(held))
	}
}
