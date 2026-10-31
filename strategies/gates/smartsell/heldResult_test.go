package smartsell

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
)

// Result.Held is whether the overlay holds the sale on the tick, the engines' hold status: a depth
// bound or the trend refusing it, a profit short of the trigger, a reached trigger waiting for its
// confirmation and a gap of the committed sale hold it.
func TestTheResultIsHeldWhileTheOverlayHoldsTheSale(t *testing.T) {
	flatMarketAlone(t)
	withAllowedLoss(t, anyLoss)
	trade := flatMarketLadder(false)
	ai := flatMarketMarket(trade)
	at := testutil.At("13:00:00")

	t.Run("the floor", func(t *testing.T) {
		withSaleStartAtDepth(t, 10)
		assertHeldResult(t, Apply(trade, "", pathUnder, ai), true)
	})
	t.Run("the trend", func(t *testing.T) {
		withTable(t, []SaleTrigger{{MinTrendDirection: everyRow + 1, Percentage: -anyLoss}})
		assertHeldResult(t, Apply(trade, "", pathUnder, ai), true)
	})
	t.Run("a profit short of the trigger", func(t *testing.T) {
		withAllowedLoss(t, -5)
		assertHeldResult(t, Apply(trade, "", pathUnder, ai), true)
	})
	t.Run("a reached trigger waiting for its confirmation", func(t *testing.T) {
		withActivationConfirmation(t, 5*time.Second)
		got := ApplyAt(trade, "", pathUnder, ai, at)
		assertHeldResult(t, got, true)
		if got.Position != "" {
			t.Fatalf("the confirmation holds the ladder, got %+v", got)
		}
	})
	t.Run("a gap of the committed sale, with its row and after it", func(t *testing.T) {
		withSaleGapTolerances(t, 5)
		committed := pathGates()[1].committedAt(t, pathUnder)
		anchored := anchoredFor(committed, pathLoss, -2*5*trailRow(committed).Tolerance)
		gapped, got := engineStepAt(anchored, "", pathLoss, at, ai)
		assertHeldResult(t, got, true)
		assertHeldResult(t, ApplyAt(gapped, "", pathLoss, ai, at.Add(time.Second)), true)
	})
}

// The overlay's other answers hold nothing: the activation, the committed trail's own hold, its
// re-anchor and its sale, the wait after a trigger, a ladder no gate watches and a take profit the
// ladder armed itself.
func TestTheResultIsNotHeldOnAnyOtherAnswer(t *testing.T) {
	flatMarketAlone(t)
	withAllowedLoss(t, anyLoss)
	trade := flatMarketLadder(false)
	ai := flatMarketMarket(trade)
	committed := pathGates()[1].committedAt(t, pathUnder)

	assertHeldResult(t, Apply(trade, "", pathUnder, ai), false)
	assertHeldResult(t, Apply(anchoredToHold(committed, pathUnder), "", pathUnder, ai), false)
	assertHeldResult(t, Apply(anchoredToReanchor(committed, pathUnder), "", pathUnder, ai), false)
	assertHeldResult(t, Apply(anchoredToSell(committed, pathUnder), "", pathUnder, ai), false)
	assertHeldResult(t, Apply(restingIn(withRows(trade, triggeredBy(trade, ai)), "takeProfit"), "sell", pathGain, ai), false)
	unwatched := trade
	unwatched.Strategy.Params.SmartSell = false
	assertHeldResult(t, Apply(unwatched, "", pathUnder, ai), false)

	withWaitAfterTrigger(t, time.Hour)
	waiting := ApplyAt(trade, "", pathUnder, ai, testutil.At("13:00:00"))
	assertHeldResult(t, waiting, false)
	if len(waiting.Rows) != 1 || waiting.Rows[0].Event != EventTriggered {
		t.Fatalf("fixture drifted: the wait after the trigger must leave the triggered row alone, got %+v", waiting)
	}
}

// assertHeldResult fails unless the answer's Held is held.
func assertHeldResult(t *testing.T, got Result, held bool) {
	t.Helper()
	if got.Held != held {
		t.Fatalf("Held = %v, want %v, got %+v", got.Held, held, got)
	}
}
