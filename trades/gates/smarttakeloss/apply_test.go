package smarttakeloss

import (
	"fmt"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The fixtures replay SOL 45211 on the w3s row. armedTrade is the ladder at
// its fifth fill (179.78, stamped 17:38); activeTrade carries the activation
// row for that fill; lastDepthTrade took the one permitted depth after it
// (175.83).
// The exit ticks are the same ladders once MinAgeAfterLastFill has passed
// since their newest fill; the plain ticks sit minutes after it, where every
// exit still waits.
var (
	activationTick = testutil.At("17:45:00")
	depthTick      = testutil.At("18:45:00")
	activeExitTick = testutil.At("17:38:00").Add(MinAgeAfterLastFill)
	depthExitTick  = testutil.At("18:41:00").Add(MinAgeAfterLastFill)
)

func armedTrade() aggragates.Trades {
	return sizedLadder(false, fills(5, "17:38:00")...)
}

func activeTrade() aggragates.Trades {
	trade := armedTrade()
	trade.Logs = []aggragates.TradesLogs{activationRow(179.78)}
	return trade
}

func lastDepthTrade() aggragates.Trades {
	trade := sizedLadder(false, fills(6, "18:41:00")...)
	trade.Logs = []aggragates.TradesLogs{activationRow(179.78)}
	return trade
}

func withBlock(block aggragates.SmartTakeLossIndicators) aggragates.AIIndicators {
	return aggragates.AIIndicators{SmartTakeLoss: block}
}

func assertUntouched(t *testing.T, got Result, position string) {
	t.Helper()
	if got.Position != position || got.Reason != "" || got.Activation != nil {
		t.Fatalf("expected %q untouched with no row, got %+v", position, got)
	}
}

func assertForced(t *testing.T, got Result, reason string) {
	t.Helper()
	if got.Position != "sellLoss" || got.Reason != reason {
		t.Fatalf("expected a forced sellLoss for %q, got %+v", reason, got)
	}
}

// The flag owns the overlay; a child, a zero price or no settings is inert.
func TestApplyInertWithoutTheFlagOnAChildOrWithoutInputs(t *testing.T) {
	band := solBlock().UpperBB

	off := lastDepthTrade()
	off.Strategy.Params.SmartTakeLoss = false
	assertUntouched(t, Apply(off, "stopLoss", 176, depthTick, withBlock(solBlock())), "stopLoss")
	offArmed := armedTrade()
	offArmed.Strategy.Params.SmartTakeLoss = false
	assertUntouched(t, Apply(offArmed, "", band, activationTick, withBlock(solBlock())), "")

	child := lastDepthTrade()
	child.ParentID = 7
	assertUntouched(t, Apply(child, "stopLoss", 176, depthTick, withBlock(solBlock())), "stopLoss")
	childArmed := armedTrade()
	childArmed.ParentID = 7
	assertUntouched(t, Apply(childArmed, "", 179.90, activationTick, withBlock(solBlock())), "")

	assertUntouched(t, Apply(lastDepthTrade(), "stopLoss", 0, depthTick, withBlock(solBlock())), "stopLoss")
	noSettings := lastDepthTrade()
	noSettings.StrategyPair.StrategySettings = nil
	assertUntouched(t, Apply(noSettings, "stopLoss", 176, depthTick, withBlock(solBlock())), "stopLoss")
}

func TestApplyInertBelowArmDepth(t *testing.T) {
	shallow := testutil.LadderTrade(false, fills(ArmDepth(8)-1, "17:38:00")...)
	assertUntouched(t, Apply(shallow, "", 179.90, activationTick, withBlock(solBlock())), "")
	assertUntouched(t, Apply(shallow, "stopLoss", 179.90, activationTick, withBlock(solBlock())), "stopLoss")
}

// An armed trade whose price still has more than MaxBarsLeft bars under it is
// the plain ladder: a cent over the level is one bar too many.
func TestApplyArmedDeadZoneTickUnchanged(t *testing.T) {
	over := solBlock().LowBodyWithBarsLeft + 0.01
	assertUntouched(t, Apply(armedTrade(), "", over, activationTick, withBlock(solBlock())), "")
	assertUntouched(t, Apply(armedTrade(), "stopLoss", over, activationTick, withBlock(solBlock())), "stopLoss")
}

// The activation tick hands back the marker row with the FILL price — not
// the tick price on trade.PositionPrice — and refuses nothing.
func TestApplyActivationTickReturnsTheRowAndForcesNothing(t *testing.T) {
	trade := armedTrade()
	trade.PositionPrice = 179.90
	got := Apply(trade, "", 179.90, activationTick, withBlock(solBlock()))
	if got.Position != "" || got.Reason != "" {
		t.Fatalf("the activation tick forces nothing, got %+v", got)
	}
	if got.Activation == nil {
		t.Fatal("the activation tick must hand back the marker row")
	}
	if got.Activation.Message != "Hold buy: smartTakeLoss: Potential trend reversal" || got.Activation.Price != 179.78 {
		t.Fatalf("the row names the raw state and carries the fill, got %+v", *got.Activation)
	}

	// The ladder's own proposal on that tick: while a depth is permitted the
	// arming passes; with none permitted it is already the exit — or, inside
	// a wait, the dropped add.
	got = Apply(trade, "stopLoss", 175, activationTick, withBlock(solBlock()))
	if got.Activation == nil {
		t.Fatalf("the activation tick must hand back the marker row, got %+v", got)
	}
	waitOver := activationTick.Sub(testutil.At("17:38:00")) >= MinAgeAfterLastFill
	switch {
	case !LastPermittedDepthExit || PermittedDepths > 0:
		if got.Position != "stopLoss" {
			t.Fatalf("the activation tick keeps the ladder's arming while a depth is permitted, got %+v", got)
		}
	case waitOver:
		assertForced(t, got, "tolerance under the last fill")
	default:
		if got.Position != "" {
			t.Fatalf("with no depth permitted the arming is dropped while the exit waits, got %+v", got)
		}
	}
}

func TestApplySecondTickWritesNoSecondRow(t *testing.T) {
	assertUntouched(t, Apply(activeTrade(), "", 179.90, activationTick, withBlock(solBlock())), "")
}

// Active: the band (or the line) sells from whatever the ladder proposed on
// the add side, the dead zone included.
func TestApplyActiveSellsAtTheBandAndTheLine(t *testing.T) {
	block := solBlock()
	for _, position := range []string{"", "stopLoss", "update_stopLoss", "buy", "forceTrailingStopLoss"} {
		assertForced(t, Apply(activeTrade(), position, block.UpperBB, activeExitTick, withBlock(block)), "upper bollinger band")
	}
	assertUntouched(t, Apply(activeTrade(), "", block.UpperBB-0.01, activeExitTick, withBlock(block)), "")

	block.Resistance = aggragates.TrendLine{From: anchor("06:30:00", 193.25), To: anchor("11:45:00", 192.97)}
	level, _ := projectLine(block.Resistance, activeExitTick.UnixMilli())
	assertForced(t, Apply(activeTrade(), "", level, activeExitTick, withBlock(block)), "resistance line")
}

// Ported from the old protectedPosition tests: a close the ladder already
// decided is never replaced, whether it is the proposal or the trade's own
// state. A force-trailing take profit reads as the take profit it re-arms.
func TestApplyNeverReplacesADecidedClose(t *testing.T) {
	block := solBlock()
	protected := []string{"sell", "takeProfit", "update_takeProfit", "sellParent", "impasse", "sellLoss", "forceTrailingTakeProfit"}
	for _, position := range protected {
		assertUntouched(t, Apply(activeTrade(), position, block.UpperBB, activeExitTick, withBlock(block)), position)
		assertUntouched(t, Apply(lastDepthTrade(), position, 175, depthExitTick, withBlock(block)), position)
	}

	states := []string{"sell", "takeProfit", "sellParent", "impasse", "sellLoss", "forceTrailingTakeProfit"}
	for _, state := range states {
		trade := activeTrade()
		trade.PositionType = state
		assertUntouched(t, Apply(trade, "", block.UpperBB, activeExitTick, withBlock(block)), "")

		last := lastDepthTrade()
		last.PositionType = state
		assertUntouched(t, Apply(last, "", 175, depthExitTick, withBlock(block)), "")
	}
}

// A resting sellLoss limit is re-placed only by its own logic row: the
// overlay never re-forces it on the prints under the tolerance line.
func TestApplyNeverReforcesARestingSellLoss(t *testing.T) {
	trade := lastDepthTrade()
	trade.PositionType = "sellLoss"
	for _, price := range []float64{175.39, 175.0, 170.0} {
		assertUntouched(t, Apply(trade, "", price, depthExitTick, withBlock(solBlock())), "")
	}
	assertUntouched(t, Apply(trade, "sellLoss", 170, depthExitTick, withBlock(solBlock())), "sellLoss")
}

// The permitted depth, while one is permitted: its arming passes, and
// nothing sells before it fills — even far under the activating fill. With
// none permitted (PermittedDepths 0) the activating fill is the last depth:
// the arming is the exit once the wait is over, and under everything the
// window holds the tolerance sells from the dead zone.
func TestApplyPermittedDepthPassesTheArmingThrough(t *testing.T) {
	block := solBlock()
	if !LastPermittedDepthExit || PermittedDepths > 0 {
		for _, tick := range []time.Time{activationTick, activeExitTick} {
			assertUntouched(t, Apply(activeTrade(), "stopLoss", 175, tick, withBlock(block)), "stopLoss")
			assertUntouched(t, Apply(activeTrade(), "update_stopLoss", 174, tick, withBlock(block)), "update_stopLoss")
			buying := activeTrade()
			buying.PositionType = "stopLoss"
			assertUntouched(t, Apply(buying, "buy", 175.83, tick, withBlock(block)), "buy")
			assertUntouched(t, Apply(activeTrade(), "", 175, tick, withBlock(block)), "")
		}
		return
	}
	assertForced(t, Apply(activeTrade(), "stopLoss", 175, activeExitTick, withBlock(block)), "tolerance under the last fill")
	assertForced(t, Apply(activeTrade(), "update_stopLoss", 174, activeExitTick, withBlock(block)), "tolerance under the last fill")
	buying := activeTrade()
	buying.PositionType = "stopLoss"
	assertForced(t, Apply(buying, "buy", 175.83, activeExitTick, withBlock(block)), "tolerance under the last fill")
	assertForced(t, Apply(activeTrade(), "", 175, activeExitTick, withBlock(block)), "tolerance under the last fill")
	// In the dead zone over the tolerance line nothing sells and nothing is
	// proposed.
	assertUntouched(t, Apply(activeTrade(), "", 179.90, activeExitTick, withBlock(block)), "")
}

// Nothing sells inside MinAgeAfterLastFill — not the tolerance, not the band,
// not the line — because without that wait a ladder can buy and sell in the
// same instant. On the last permitted depth the ladder may not add either:
// the add-side proposal is dropped, so the wait cannot be reset by a fill it
// would otherwise take.
func TestApplyWaitsMinAgeAfterTheLastFillBeforeAnyExit(t *testing.T) {
	if MinAgeAfterLastFill <= 0 {
		t.Skip("MinAgeAfterLastFill is deactivated (0): nothing waits, no add is refused")
	}
	block := solBlock()
	block.Resistance = aggragates.TrendLine{From: anchor("06:30:00", 193.25), To: anchor("11:45:00", 192.97)}
	line, _ := projectLine(block.Resistance, depthTick.UnixMilli())
	tolerance := toleranceLine(175.83, false)

	// Minutes after the sixth fill: every leg waits.
	assertUntouched(t, Apply(lastDepthTrade(), "", tolerance, depthTick, withBlock(block)), "")
	assertUntouched(t, Apply(lastDepthTrade(), "", block.UpperBB, depthTick, withBlock(block)), "")
	assertUntouched(t, Apply(lastDepthTrade(), "", line, depthTick, withBlock(block)), "")

	// …and no further depth is committed while it waits.
	for _, position := range []string{"stopLoss", "update_stopLoss", "buy", "forceTrailingStopLoss"} {
		if got := Apply(lastDepthTrade(), position, 170, depthTick, withBlock(block)); got.Position != "" || got.Reason != "" {
			t.Fatalf("an add on the last permitted depth is dropped while the exit waits, %q gave %+v", position, got)
		}
	}

	// A decided close is still the ladder's, waiting or not.
	assertUntouched(t, Apply(lastDepthTrade(), "takeProfit", 200, depthTick, withBlock(block)), "takeProfit")

	// The refusal is the only thing an operator can see, so it is written —
	// once for the fill it stands for, whatever the ladder proposes next.
	got := Apply(lastDepthTrade(), "stopLoss", 170, depthTick, withBlock(block))
	if got.Wait == nil || got.Wait.Price != 175.83 {
		t.Fatalf("the refusal must name the fill it waits on, got %+v", got)
	}
	if want := fmt.Sprintf("Hold buy: smartTakeLoss: last fill too fresh to sell, no add (%s)", MinAgeAfterLastFill); got.Wait.Message != want {
		t.Fatalf("wait row %q, want %q", got.Wait.Message, want)
	}
	logged := lastDepthTrade()
	logged.Logs = append(logged.Logs, aggragates.TradesLogs{Message: got.Wait.Message, Price: got.Wait.Price})
	if again := Apply(logged, "stopLoss", 170, depthTick, withBlock(block)); again.Wait != nil || again.Position != "" {
		t.Fatalf("a fill whose refusal already stands writes no second row, got %+v", again)
	}
	// The row of an earlier fill does not cover the newest one.
	stale := lastDepthTrade()
	stale.Logs = append(stale.Logs, aggragates.TradesLogs{Message: got.Wait.Message, Price: 179.78})
	if again := Apply(stale, "stopLoss", 170, depthTick, withBlock(block)); again.Wait == nil {
		t.Fatalf("each fill gets its own refusal row, got %+v", again)
	}
	// Nothing is written when nothing was overridden: the exit simply waits.
	if quiet := Apply(lastDepthTrade(), "", tolerance, depthTick, withBlock(block)); quiet.Wait != nil {
		t.Fatalf("a waiting exit that overrides nothing writes no row, got %+v", quiet)
	}

	// One second before the window closes and one second after it.
	last := testutil.At("18:41:00")
	assertUntouched(t, Apply(lastDepthTrade(), "", tolerance, last.Add(MinAgeAfterLastFill-time.Second), withBlock(block)), "")
	assertForced(t, Apply(lastDepthTrade(), "", tolerance, last.Add(MinAgeAfterLastFill), withBlock(block)), "tolerance under the last fill")
}

// The guard measures the fill's own stamp against the tick clock, and an
// unknown clock on either side holds the exit: this gate acts by closing the
// trade, and the permitted depth it reads passes no age check of its own, so
// a row that lost its stamp must not sell on the tick it appears.
func TestApplyMinAgeHoldsOnAnUnknownClock(t *testing.T) {
	block := solBlock()
	tolerance := toleranceLine(175.83, false)

	unstamped := lastDepthTrade()
	for index := range unstamped.History {
		unstamped.History[index].CreatedAt = time.Time{}
	}
	assertUntouched(t, Apply(unstamped, "", tolerance, depthExitTick, withBlock(block)), "")
	assertUntouched(t, Apply(lastDepthTrade(), "", tolerance, time.Time{}, withBlock(block)), "")

	// …and, while the last-permitted-depth rule is on, the add stays
	// refused, so an unmeasurable trade commits nothing.
	if got := Apply(unstamped, "stopLoss", tolerance, depthExitTick, withBlock(block)); LastPermittedDepthExit && got.Position != "" {
		t.Fatalf("an unstamped last fill must not add either, got %+v", got)
	}
}
