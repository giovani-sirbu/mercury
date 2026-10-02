package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// A depth priority hold pauses the swing decline as it does the quiet decline:
// the first held tick of a ladder the swing decline made pending writes the one
// reset row — it resets whichever feature made the ladder pending — and a new
// fill on it is judged on nothing, whatever the series serves and whether the
// band is reached; nothing is sold. From the next tick the ladder reads watched
// and not pending, still held, and nothing is written. The ladder's next fill
// ends the hold, and the swing decline goes pending on it afresh when its
// window and trend read inside that fill's read bars.
func TestAHeldSwingLadderIsResetOnceAndNothingIsJudged(t *testing.T) {
	held := heldBy(stairFifthFill(t), stairOpen(31))
	if st := rebuildState(held); !st.depthPriorityHeld || !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[3] || !slowDeclineFillUnjudged(held, st) {
		t.Fatalf("fixture drifted: held, pending from the fourth fill, the fifth unjudged, got %+v", st)
	}
	reset := ResetRow(stairPrices[4], swingFinding(nil))
	for name, series := range map[string][]float64{"a series that confirms": stairTurns, "a series that cancels": stairRally} {
		for _, price := range []float64{underTheBand, slowDeclineBand + 1} {
			got := Apply(held, "", price, withBlock(stairBlock(series, 30)))
			assertRow(t, got, reset)
			assertNoSale(t, got, "")
			if st := rebuildState(held); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[3] {
				t.Fatalf("%s at %v: Apply writes nothing into the trade, the pending stays as it was, got %+v", name, price, st)
			}
		}
	}

	ticked, _ := engineTick(held, "", slowDeclineBand+1, stairOpen(32), withBlock(stairFifthBlock()))
	if st := rebuildState(ticked); !st.depthPriorityHeld || st.slowDeclinePending || !st.swingDeclineWatched || !st.indecision {
		t.Fatalf("the reset leaves the ladder watched, not pending, held and latched, got %+v", st)
	}
	assertUntouched(t, Apply(ticked, "", slowDeclineBand+1, withBlock(stairFifthBlock())), "")

	sixth := withFill(ticked, stairPrices[5], stairOpen(36).Add(17*time.Minute))
	if st := rebuildState(sixth); st.depthPriorityHeld || st.slowDeclinePending {
		t.Fatalf("the next fill ends the hold on a ladder that is not pending, got %+v", st)
	}
	decline := stairBlock(stairDecline, 36)
	assertRow(t, Apply(sixth, "", underTheBand, withBlock(decline)), PendingRow(stairPrices[5], swingFinding(stairReading(t, sixth, decline, true).Reasons)))
	assertUntouched(t, Apply(sixth, "", underTheBand, withBlock(stairBlock(stairRally, 36))), "")
}

// A ladder pending on both declines is reset on its first held tick by one row,
// whichever of them made it pending: the row names no reasons and the event
// carries both features. The ladder reads not pending from then on, and nothing
// is written or sold while the hold lasts.
func TestAHeldLadderPendingOnBothDeclinesIsResetOnce(t *testing.T) {
	both := withRow(stairPending(t), PendingRow(stairPrices[3], quietFinding(nil), swingFinding(nil)), stairOpen(24).Add(30*time.Minute))
	held := heldBy(both, stairOpen(25))
	st := rebuildState(held)
	if !st.depthPriorityHeld || !st.slowDeclinePending || len(st.pendingFeatures) != 2 {
		t.Fatalf("fixture drifted: held and pending on both declines, got %+v", st)
	}
	block := withBlock(stairFourthBlock())
	ticked, got := engineTick(held, "", slowDeclineBand+1, stairOpen(26), block)
	want := ResetRow(stairPrices[3], quietFinding(nil), swingFinding(nil))
	assertRow(t, got, want)
	if want.Message != "smartTakeLoss: slow decline reset, depth priority holds this ladder" || want.Event != EventReset {
		t.Fatalf("the reset row names no feature but the gate, got %+v", want)
	}
	assertNoSale(t, got, "")
	if st := rebuildState(ticked); st.slowDeclinePending || st.slowDeclinePendingFrom != 0 || len(st.pendingFeatures) != 0 {
		t.Fatalf("the reset ends the pending on both declines, got %+v", st)
	}
	assertUntouched(t, Apply(ticked, "", slowDeclineBand+1, block), "")
}

// A held, swing-pending and latched ladder has its newest-fill take profit
// paused while its latched take profit goes on: unheld the take profit reads
// the larger of the move from the newest fill and the move from the position
// price, held the move from the position price alone, never the input's under
// it, and a hold that ends with the next fill gives the newest fill back.
func TestAHeldSwingLadderKeepsTheLatchedTakeProfitOnly(t *testing.T) {
	pending := stairPending(t)
	pending.PositionPrice = 190
	price := 195.0
	input, fromPosition, fromNewestFill := moveAgainst(price, ladder.AverageEntryPrice(pending)), moveAgainst(price, 190), moveAgainst(price, stairPrices[3])
	if st := rebuildState(pending); !st.slowDeclinePending || !st.indecision || !(input >= 0 && input < fromPosition && fromPosition < fromNewestFill) {
		t.Fatalf("fixture drifted: pending, latched, the moves %v < %v < %v, got %+v", input, fromPosition, fromNewestFill, st)
	}
	if got := TakeProfitPercentage(pending, price, input); got != fromNewestFill {
		t.Errorf("unheld, the take profit reads the newest fill %v, got %v", fromNewestFill, got)
	}
	if got := TakeProfitPercentage(heldBy(pending, stairOpen(25)), price, input); got != fromPosition {
		t.Errorf("held, the newest-fill reading is paused and the latched one goes on at %v, got %v", fromPosition, got)
	}
}
