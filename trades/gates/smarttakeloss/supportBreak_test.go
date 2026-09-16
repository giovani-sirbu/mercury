package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// supportBlock is solBlock with the falling market's support — the line
// through two lower lows, L1 195.00 at 06:30, L2 190.00 at 11:45, falling
// ~0.95 an hour, ~184.3 at 17:45 — and the count of newest closed bars that
// closed under it. The line sits over the whole ladder, so a price that does
// NOT activate (over 181.20) can still sit under it.
func supportBlock(barsUnder int) aggragates.SmartTakeLossIndicators {
	block := solBlock()
	block.LowerLows = aggragates.TrendLine{From: anchor("06:30:00", 195.00), To: anchor("11:45:00", 190.00)}
	block.SupportBarsUnder = barsUnder
	return block
}

// withHigherLowsTarget is solBlock with the inverse ladder's bounce target
// (two higher lows) over the price and a count beside it: a long ladder
// reads neither.
func withHigherLowsTarget(barsUnder int) aggragates.SmartTakeLossIndicators {
	block := solBlock()
	block.Support = aggragates.TrendLine{From: anchor("06:30:00", 178.40), To: anchor("11:45:00", 183.00)}
	block.SupportBarsUnder = barsUnder
	return block
}

// resistanceBlock is risingBlock with the rising market's resistance — two
// higher highs, H1 96 at 06:30, H2 98 at 11:45, ~100.3 at 17:45 — the
// inverse ladder can break upwards without activating (under 106).
func resistanceBlock(barsOver int) aggragates.SmartTakeLossIndicators {
	block := risingBlock()
	block.HigherHighs = aggragates.TrendLine{From: anchor("06:30:00", 96), To: anchor("11:45:00", 98)}
	block.ResistanceBarsOver = barsOver
	return block
}

const w3sTolerance = 0.25

// Both conditions, on the tick: SupportBreakBars closed bars under the line,
// and the price plus one tolerance still under it. The count alone does not
// sell, the price alone does not sell.
func TestSupportBreakReached(t *testing.T) {
	trade := sizedLadder(false, fills(5, "17:38:00")...)
	st := rebuildState(trade)
	now := testutil.At("17:45:00").UnixMilli()
	block := supportBlock(SupportBreakBars)
	level, _ := projectLine(block.LowerLows, now)
	edge := level / (1 + w3sTolerance/100)
	if level <= block.LowBodyWithBarsLeft {
		t.Fatalf("the fixture wants the line over the activation level, got %v", level)
	}

	cases := []struct {
		name  string
		block aggragates.SmartTakeLossIndicators
		price float64
		want  bool
	}{
		{"three bars under and a tolerance under the line", block, 182, true},
		{"a cent under the tolerance edge", block, edge - 0.01, true},
		{"exactly a tolerance under the line", block, edge, false},
		{"within a tolerance of the line", block, edge + 0.01, false},
		{"over the line", block, level + 1, false},
		{"one bar short of the count", supportBlock(SupportBreakBars - 1), 182, false},
		{"no bar under yet", supportBlock(0), 182, false},
		{"no line", solBlock(), 182, false},
		{"the higher-lows bounce target is not the break line", withHigherLowsTarget(SupportBreakBars), 182, false},
		{"no verdict", aggragates.SmartTakeLossIndicators{}, 182, false},
		{"no price", block, 0, false},
	}
	for _, c := range cases {
		reason, hit := supportBreakReached(trade, st, c.price, now, c.block)
		if hit != c.want || (hit && reason != "support line break") {
			t.Errorf("%s (%.2f): %q %v, want hit %v", c.name, c.price, reason, hit, c.want)
		}
	}

	// A row without a tolerance cannot say what "a tolerance under" is.
	noTolerance := trade
	noTolerance.StrategyPair.StrategySettings = append([]aggragates.StrategySettings(nil), trade.StrategyPair.StrategySettings...)
	for index := range noTolerance.StrategyPair.StrategySettings {
		noTolerance.StrategyPair.StrategySettings[index].Tolerance = 0
	}
	if _, hit := supportBreakReached(noTolerance, rebuildState(noTolerance), 182, now, block); hit {
		t.Fatal("a row without a tolerance must not sell on the break")
	}
}

// The inverse ladder mirrors it on the resistance line: bars that closed
// over it, the price minus one tolerance still over it.
func TestSupportBreakReachedInverseMirror(t *testing.T) {
	trade := sizedLadder(true, risingFills(5, "17:38:00")...)
	st := rebuildState(trade)
	now := testutil.At("17:45:00").UnixMilli()
	block := resistanceBlock(SupportBreakBars)
	level, _ := projectLine(block.HigherHighs, now)
	edge := level / (1 - w3sTolerance/100)
	if level >= block.HighBodyWithBarsLeft {
		t.Fatalf("the fixture wants the line under the activation level, got %v", level)
	}

	if reason, hit := supportBreakReached(trade, st, 103, now, block); !hit || reason != "resistance line break" {
		t.Fatalf("three bars over and a tolerance over the line sells, got %q %v", reason, hit)
	}
	if _, hit := supportBreakReached(trade, st, edge, now, block); hit {
		t.Fatal("exactly a tolerance over the line does not sell")
	}
	if _, hit := supportBreakReached(trade, st, 103, now, resistanceBlock(SupportBreakBars-1)); hit {
		t.Fatal("one bar short of the count does not sell")
	}
	// The long side's lower-lows line and count mean nothing to an inverse ladder.
	if _, hit := supportBreakReached(trade, st, 103, now, supportBlock(SupportBreakBars)); hit {
		t.Fatal("an inverse ladder reads the higher highs, not the lower lows")
	}
	// …and the bounce targets never break anything: a resistance through
	// lower highs is what an inverse ladder buys back into, not what it breaks.
	targets := risingBlock()
	targets.Resistance = aggragates.TrendLine{From: anchor("06:30:00", 100), To: anchor("11:45:00", 98)}
	targets.ResistanceBarsOver = SupportBreakBars
	if _, hit := supportBreakReached(trade, st, 103, now, targets); hit {
		t.Fatal("the resistance through lower highs is a bounce target, not a break line")
	}
}

// From arming on, activated or not: an armed ladder in its dead zone sells
// on the break from whatever the ladder proposed on the add side, a decided
// close is still the ladder's, and short of the count nothing changes.
func TestApplySupportBreakSellsFromArmingOn(t *testing.T) {
	block := supportBlock(SupportBreakBars)
	over := block.LowBodyWithBarsLeft + 0.01 // no activation at this price
	if over*(1+w3sTolerance/100) >= mustProject(t, block.LowerLows, activationTick) {
		t.Fatalf("the fixture wants a price under the line that does not activate, got %v", over)
	}

	for _, position := range []string{"", "stopLoss", "update_stopLoss", "buy", "forceTrailingStopLoss"} {
		got := Apply(armedTrade(), position, over, activationTick, withBlock(block))
		if got.Position != "sellLoss" || got.Reason != "support line break" || got.Activation != nil {
			t.Fatalf("an armed ladder sells on the break from %q without activating, got %+v", position, got)
		}
	}
	assertForced(t, Apply(activeTrade(), "", over, activeExitTick, withBlock(block)), "support line break")
	assertForced(t, Apply(lastDepthTrade(), "", over, depthExitTick, withBlock(block)), "support line break")

	for _, position := range []string{"sell", "takeProfit", "impasse", "sellLoss"} {
		assertUntouched(t, Apply(armedTrade(), position, over, activationTick, withBlock(block)), position)
	}
	short := supportBlock(SupportBreakBars - 1)
	assertUntouched(t, Apply(armedTrade(), "", over, activationTick, withBlock(short)), "")
	assertUntouched(t, Apply(armedTrade(), "stopLoss", over, activationTick, withBlock(short)), "stopLoss")
	assertUntouched(t, Apply(armedTrade(), "", over, activationTick, withBlock(solBlock())), "")

	// Below the arm depth the line is not read at all.
	shallow := testutil.LadderTrade(false, fills(ArmDepth(8)-1, "17:38:00")...)
	assertUntouched(t, Apply(shallow, "stopLoss", over, activationTick, withBlock(block)), "stopLoss")

	// The exit row names the leg and the level.
	got := Apply(armedTrade(), "", over, activationTick, withBlock(block))
	if msg := ExitMessage(armedTrade(), got.Reason, over); msg != "smartTakeLoss: sell at support line break 181.21" {
		t.Fatalf("unexpected exit row %q", msg)
	}
}

// A tick that activates AND sits under a broken support hands back both:
// the marker row, and the forced exit.
func TestApplySupportBreakOnTheActivationTick(t *testing.T) {
	block := supportBlock(SupportBreakBars)
	got := Apply(armedTrade(), "", block.LowBodyWithBarsLeft, activationTick, withBlock(block))
	if got.Activation == nil || got.Activation.Price != 179.78 {
		t.Fatalf("the activation row still comes back, got %+v", got)
	}
	if got.Position != "sellLoss" || got.Reason != "support line break" {
		t.Fatalf("the break sells on the activation tick, got %+v", got)
	}
}

// The break is the same sell as every other leg: inside MinAgeAfterLastFill
// it waits too. Only meaningful while the wait is on.
func TestApplySupportBreakWaitsMinAge(t *testing.T) {
	if MinAgeAfterLastFill <= 0 {
		t.Skip("MinAgeAfterLastFill is deactivated (0): nothing waits")
	}
	block := supportBlock(SupportBreakBars)
	over := block.LowBodyWithBarsLeft + 0.01
	assertUntouched(t, Apply(armedTrade(), "", over, testutil.At("17:38:00").Add(MinAgeAfterLastFill-time.Second), withBlock(block)), "")
	assertForced(t, Apply(armedTrade(), "", over, testutil.At("17:38:00").Add(MinAgeAfterLastFill), withBlock(block)), "support line break")
}

func TestApplySupportBreakInverseMirror(t *testing.T) {
	block := resistanceBlock(SupportBreakBars)
	inverseArmed := sizedLadder(true, risingFills(5, "17:38:00")...)
	under := block.HighBodyWithBarsLeft - 0.01 // no activation at this price
	if under*(1-w3sTolerance/100) <= mustProject(t, block.HigherHighs, activationTick) {
		t.Fatalf("the fixture wants a price over the line that does not activate, got %v", under)
	}

	got := Apply(inverseArmed, "stopLoss", under, activationTick, withBlock(block))
	if got.Position != "sellLoss" || got.Reason != "resistance line break" || got.Activation != nil {
		t.Fatalf("an armed inverse ladder sells on the break without activating, got %+v", got)
	}
	assertUntouched(t, Apply(inverseArmed, "stopLoss", under, activationTick, withBlock(resistanceBlock(SupportBreakBars-1))), "stopLoss")
}

// bounceBlock is solBlock with the support the price bounced from — a level
// over the whole ladder, so a price that does not activate can sit under it
// — and the count of newest closed bars that closed under that level.
func bounceBlock(level float64, barsUnder int) aggragates.SmartTakeLossIndicators {
	block := supportBlock(0)
	block.SupportBounceLevel = level
	block.SupportBounceBarsUnder = barsUnder
	return block
}

// The failed retest: SupportBounceBreakBars closed bars under the support
// the price bounced from, and the tick price under it — no tolerance.
func TestSupportBreakReachedAfterABounce(t *testing.T) {
	trade := sizedLadder(false, fills(5, "17:38:00")...)
	st := rebuildState(trade)
	now := testutil.At("17:45:00").UnixMilli()
	level := 185.0

	cases := []struct {
		name  string
		block aggragates.SmartTakeLossIndicators
		price float64
		want  bool
	}{
		{"two bars under the bounced level and the price under it", bounceBlock(level, SupportBounceBreakBars), 184.99, true},
		{"far under it", bounceBlock(level, SupportBounceBreakBars), 170, true},
		{"on the level", bounceBlock(level, SupportBounceBreakBars), level, false},
		{"over the level", bounceBlock(level, SupportBounceBreakBars), 185.5, false},
		{"one bar short", bounceBlock(level, SupportBounceBreakBars-1), 184, false},
		{"no bounce served", bounceBlock(0, SupportBounceBreakBars), 184, false},
		{"no verdict", aggragates.SmartTakeLossIndicators{}, 184, false},
	}
	for _, c := range cases {
		reason, hit := supportBreakReached(trade, st, c.price, now, c.block)
		if hit != c.want || (hit && reason != "support break after bounce") {
			t.Errorf("%s (%.2f): %q %v, want hit %v", c.name, c.price, reason, hit, c.want)
		}
	}

	// The line break has the first word when both stand.
	both := supportBlock(SupportBreakBars)
	both.SupportBounceLevel, both.SupportBounceBarsUnder = level, SupportBounceBreakBars
	if reason, hit := supportBreakReached(trade, st, 182, now, both); !hit || reason != "support line break" {
		t.Fatalf("with both conditions the line break names the exit, got %q %v", reason, hit)
	}

	// The inverse mirror: the resistance the price bounced from, bars that
	// closed over it, the price over it.
	inverse := sizedLadder(true, risingFills(5, "17:38:00")...)
	mirror := resistanceBlock(0)
	mirror.ResistanceBounceLevel, mirror.ResistanceBounceBarsOver = 100, SupportBounceBreakBars
	if reason, hit := supportBreakReached(inverse, rebuildState(inverse), 100.01, now, mirror); !hit || reason != "resistance break after bounce" {
		t.Fatalf("an inverse ladder sells over the bounced resistance, got %q %v", reason, hit)
	}
	if _, hit := supportBreakReached(inverse, rebuildState(inverse), 100, now, mirror); hit {
		t.Fatal("on the bounced resistance nothing sells")
	}
	if _, hit := supportBreakReached(inverse, rebuildState(inverse), 100.01, now, bounceBlock(level, SupportBounceBreakBars)); hit {
		t.Fatal("an inverse ladder reads the resistance bounce, not the support's")
	}
}

// Like the line break, the failed retest sells from arming on, activated or
// not, and never over a decided close.
func TestApplyBounceBreakSellsFromArmingOn(t *testing.T) {
	block := bounceBlock(185, SupportBounceBreakBars)
	over := block.LowBodyWithBarsLeft + 0.01 // no activation at this price
	for _, position := range []string{"", "stopLoss", "buy"} {
		got := Apply(armedTrade(), position, over, activationTick, withBlock(block))
		if got.Position != "sellLoss" || got.Reason != "support break after bounce" || got.Activation != nil {
			t.Fatalf("an armed ladder sells on the failed retest from %q, got %+v", position, got)
		}
	}
	assertForced(t, Apply(activeTrade(), "", over, activeExitTick, withBlock(block)), "support break after bounce")
	assertUntouched(t, Apply(armedTrade(), "takeProfit", over, activationTick, withBlock(block)), "takeProfit")
	assertUntouched(t, Apply(armedTrade(), "", over, activationTick, withBlock(bounceBlock(185, SupportBounceBreakBars-1))), "")
	assertUntouched(t, Apply(armedTrade(), "", 185.5, activationTick, withBlock(block)), "")
	got := Apply(armedTrade(), "", over, activationTick, withBlock(block))
	if msg := ExitMessage(armedTrade(), got.Reason, over); msg != "smartTakeLoss: sell at support break after bounce 181.21" {
		t.Fatalf("unexpected exit row %q", msg)
	}
}

func mustProject(t *testing.T, line aggragates.TrendLine, at time.Time) float64 {
	t.Helper()
	level, ok := projectLine(line, at.UnixMilli())
	if !ok {
		t.Fatalf("the fixture line %+v does not project", line)
	}
	return level
}
