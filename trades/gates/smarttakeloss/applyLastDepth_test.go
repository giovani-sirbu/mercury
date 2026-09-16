package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The last permitted depth: one tolerance under the last fill sells from the
// dead zone once the window ALSO says there is no bar left under the price,
// and every add-side proposal becomes the exit whatever the window says.
func TestApplyLastPermittedDepthSellsUnderTheToleranceAndRefusesAdds(t *testing.T) {
	skipUnlessLastDepthRule(t)
	block := solBlock()
	line := toleranceLine(175.83, false)

	assertForced(t, Apply(lastDepthTrade(), "", line, depthExitTick, withBlock(block)), "tolerance under the last fill")
	assertForced(t, Apply(lastDepthTrade(), "", line-1, depthExitTick, withBlock(block)), "tolerance under the last fill")
	assertUntouched(t, Apply(lastDepthTrade(), "", line+0.01, depthExitTick, withBlock(block)), "")

	for _, position := range []string{"stopLoss", "update_stopLoss", "forceTrailingStopLoss"} {
		assertForced(t, Apply(lastDepthTrade(), position, 176, depthExitTick, withBlock(block)), "tolerance under the last fill")
	}
	buying := lastDepthTrade()
	buying.PositionType = "stopLoss"
	assertForced(t, Apply(buying, "buy", 176, depthExitTick, withBlock(block)), "tolerance under the last fill")
}

// The tolerance alone does not sell: the window has to have run out of bars
// to the left of the price first. With the window's lowest low under the
// tolerance line the trade is still inside two hundred days of prints, and it
// waits — while the add refusal, which commits nothing either way, stands.
func TestApplyLastPermittedDepthWaitsWhileBarsRemainToTheLeft(t *testing.T) {
	skipUnlessLastDepthRule(t)
	block := solBlock()
	line := toleranceLine(175.83, false)
	block.LowestBody = line - 1

	assertUntouched(t, Apply(lastDepthTrade(), "", line, depthExitTick, withBlock(block)), "")
	assertUntouched(t, Apply(lastDepthTrade(), "", line-0.99, depthExitTick, withBlock(block)), "")
	assertForced(t, Apply(lastDepthTrade(), "", block.LowestBody, depthExitTick, withBlock(block)), "tolerance under the last fill")
	assertForced(t, Apply(lastDepthTrade(), "stopLoss", line, depthExitTick, withBlock(block)), "tolerance under the last fill")
}

// Ported from the old protectiveTick tests: UseAI alone or CrashGuard alone
// never sells at a loss, whatever the payload says.
func TestApplyOtherFlagsAloneNeverForce(t *testing.T) {
	ai := withBlock(solBlock())
	ai.CrashActive = true
	for _, params := range []aggragates.StrategyParams{{UseAI: true}, {CrashGuard: true}, {UseAI: true, CrashGuard: true}} {
		trade := lastDepthTrade()
		trade.Strategy.Params = params
		assertUntouched(t, Apply(trade, "stopLoss", 175, depthTick, ai), "stopLoss")
		assertUntouched(t, Apply(trade, "", ai.SmartTakeLoss.UpperBB, depthTick, ai), "")
	}
}

// Sophos down (a zero block): nothing activates, no line or band sells, and
// the tolerance exit does not fire either — it is armed by the window, and a
// window that answers nothing cannot say the price is under everything it
// holds. The add refusal is the one rule that reads the trade alone, so an
// active trade on its last permitted depth still commits no further capital.
func TestApplyZeroBlock(t *testing.T) {
	assertUntouched(t, Apply(armedTrade(), "", 1e6, activationTick, aggragates.AIIndicators{}), "")
	assertUntouched(t, Apply(activeTrade(), "", 1e6, activationTick, aggragates.AIIndicators{}), "")

	line := toleranceLine(175.83, false)
	assertUntouched(t, Apply(lastDepthTrade(), "", line, depthExitTick, aggragates.AIIndicators{}), "")
	if LastPermittedDepthExit {
		assertForced(t, Apply(lastDepthTrade(), "stopLoss", 176, depthExitTick, aggragates.AIIndicators{}), "tolerance under the last fill")
	} else {
		assertUntouched(t, Apply(lastDepthTrade(), "stopLoss", 176, depthExitTick, aggragates.AIIndicators{}), "stopLoss")
	}
}

// Every rule mirrored for an inverse ladder: fresh high, lower band, one
// tolerance over the last fill.
func TestApplyInverseMirror(t *testing.T) {
	block := risingBlock()

	inverseArmed := sizedLadder(true, risingFills(5, "17:38:00")...)
	got := Apply(inverseArmed, "", 107, activationTick, withBlock(block))
	if got.Activation == nil || got.Activation.Price != 108 || got.Position != "" {
		t.Fatalf("an inverse ladder activates on the fresh high with the fill price, got %+v", got)
	}

	active := inverseArmed
	active.Logs = []aggragates.TradesLogs{activationRow(108)}
	assertForced(t, Apply(active, "", block.LowerBB, activeExitTick, withBlock(block)), "lower bollinger band")
	if !LastPermittedDepthExit || PermittedDepths > 0 {
		assertUntouched(t, Apply(active, "stopLoss", 111, activationTick, withBlock(block)), "stopLoss")
	} else {
		assertForced(t, Apply(active, "stopLoss", 111, activeExitTick, withBlock(block)), "tolerance above the last fill")
	}

	last := sizedLadder(true, risingFills(6, "18:41:00")...) // …, 108, 110
	last.Logs = []aggragates.TradesLogs{activationRow(108)}
	line := toleranceLine(110, true)
	if !LastPermittedDepthExit {
		// The rule is off: the inverse ladder keeps adding and the
		// tolerance over the last fill sells nothing.
		assertUntouched(t, Apply(last, "", line, depthExitTick, withBlock(block)), "")
		assertUntouched(t, Apply(last, "stopLoss", 109, depthExitTick, withBlock(block)), "stopLoss")
		return
	}
	assertForced(t, Apply(last, "", line, depthExitTick, withBlock(block)), "tolerance above the last fill")
	assertUntouched(t, Apply(last, "", line-0.01, depthExitTick, withBlock(block)), "")
	assertForced(t, Apply(last, "stopLoss", 109, depthExitTick, withBlock(block)), "tolerance above the last fill")
}

// skipUnlessLastDepthRule skips a test of the last-permitted-depth rule
// while LastPermittedDepthExit is switched off; the rule's tests stay, for
// the day it is switched back on.
func skipUnlessLastDepthRule(t *testing.T) {
	t.Helper()
	if !LastPermittedDepthExit {
		t.Skip("LastPermittedDepthExit is switched off: the last permitted depth sells nothing")
	}
}

// With the last-permitted-depth rule switched off an activated ladder keeps
// trading: the next depth arms, a print under the tolerance line and under
// everything the window holds sells nothing, and no add is ever refused.
// The bounce targets and the support line are the only exits left.
func TestApplyLastPermittedDepthRuleSwitchedOff(t *testing.T) {
	if LastPermittedDepthExit {
		t.Skip("LastPermittedDepthExit is on")
	}
	block := solBlock()
	for _, trade := range []aggragates.Trades{activeTrade(), lastDepthTrade()} {
		for _, position := range []string{"stopLoss", "update_stopLoss", "forceTrailingStopLoss"} {
			assertUntouched(t, Apply(trade, position, 170, depthExitTick, withBlock(block)), position)
		}
		buying := trade
		buying.PositionType = "stopLoss"
		assertUntouched(t, Apply(buying, "buy", 170, depthExitTick, withBlock(block)), "buy")
		assertUntouched(t, Apply(trade, "", toleranceLine(175.83, false), depthExitTick, withBlock(block)), "")
		assertUntouched(t, Apply(trade, "", block.LowestBody-5, depthExitTick, withBlock(block)), "")
		assertForced(t, Apply(trade, "", block.UpperBB, depthExitTick, withBlock(block)), "upper bollinger band")
	}
	// The activation tick itself keeps the ladder's arming.
	got := Apply(armedTrade(), "stopLoss", 175, activationTick, withBlock(block))
	if got.Position != "stopLoss" || got.Activation == nil {
		t.Fatalf("the activation tick keeps the arming with the rule off, got %+v", got)
	}
	// An unstamped fill refuses nothing either: the refusal belongs to the rule.
	unstamped := lastDepthTrade()
	for index := range unstamped.History {
		unstamped.History[index].CreatedAt = time.Time{}
	}
	assertUntouched(t, Apply(unstamped, "stopLoss", 170, depthExitTick, withBlock(block)), "stopLoss")
}

// The engines stamp the exit row with the tick clock; the message names the
// leg and the level at the pair's precision.
func TestApplyReasonFeedsTheExitMessage(t *testing.T) {
	block := solBlock()
	got := Apply(activeTrade(), "", block.UpperBB, activeExitTick, withBlock(block))
	if msg := ExitMessage(activeTrade(), got.Reason, block.UpperBB); msg != "smartTakeLoss: sell at upper bollinger band 193.83" {
		t.Fatalf("unexpected exit row %q", msg)
	}
}
