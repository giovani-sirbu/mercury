package smarttakeloss

import (
	"testing"
)

// toleranceLine is the w3s row's tolerance (0.25) applied to a last fill,
// with the same float64 arithmetic the rule uses so an "at the line" print
// compares equal.
func toleranceLine(lastFill float64, inverse bool) float64 {
	tolerance := 0.25
	if inverse {
		return lastFill * (1 + tolerance/100)
	}
	return lastFill * (1 - tolerance/100)
}

// One tolerance (0.25) under the last fill of 175.83: at or below sells,
// above does not.
func TestToleranceExitReached(t *testing.T) {
	trade := sizedLadder(false, fills(6, "18:41:00")...)
	st := rebuildState(trade)
	line := toleranceLine(175.83, false)

	if !toleranceExitReached(trade, st, line) {
		t.Fatal("touching the tolerance line sells")
	}
	if !toleranceExitReached(trade, st, line*0.999) {
		t.Fatal("under the tolerance line sells")
	}
	if toleranceExitReached(trade, st, line+0.01) {
		t.Fatal("over the tolerance line holds")
	}
}

func TestToleranceExitNeedsATolerance(t *testing.T) {
	trade := sizedLadder(false, fills(6, "18:41:00")...)
	trade.StrategyPair.StrategySettings[0].Tolerance = 0
	if toleranceExitReached(trade, rebuildState(trade), 1) {
		t.Fatal("a row without a tolerance never exits here")
	}
	trade.StrategyPair.StrategySettings = nil
	if toleranceExitReached(trade, rebuildState(trade), 1) {
		t.Fatal("no settings, no exit")
	}
}

func TestToleranceExitInverseMirror(t *testing.T) {
	trade := sizedLadder(true, risingFills(5, "17:38:00")...) // last fill 108
	st := rebuildState(trade)
	line := toleranceLine(108, true)

	if !toleranceExitReached(trade, st, line) {
		t.Fatal("touching the tolerance line over the last fill sells an inverse ladder")
	}
	if toleranceExitReached(trade, st, line-0.01) {
		t.Fatal("under the tolerance line an inverse ladder holds")
	}
	if toleranceReason(true) != "tolerance above the last fill" || toleranceReason(false) != "tolerance under the last fill" {
		t.Fatal("the tolerance reason names the side")
	}
}
