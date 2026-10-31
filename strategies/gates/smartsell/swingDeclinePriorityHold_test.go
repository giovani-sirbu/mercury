package smartsell

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// A depth priority hold pauses the swing decline as it does the quiet decline:
// the first held tick of a ladder the swing decline made pending writes the one
// reset row — it resets whichever feature made the ladder pending — and a new
// fill on it is judged on nothing, whatever the series serves and whatever the
// proposal is; nothing is activated or sold, in buy or in trailingBuy, where the same
// ladder unheld would have its trailingBuy activated or be sold. From the next tick the
// ladder reads watched and not pending, still held, and nothing is written. The
// ladder's next fill ends the hold, and the swing decline goes pending on a tick of
// the ladder once unheld, as soon as its window and trend read, and the shared
// action acts on it from that tick.
func TestAHeldSwingLadderIsResetOnceAndNothingIsJudged(t *testing.T) {
	withBothDeclines(t)
	withDepthPriorityPause(t, true)
	held := heldBy(stairFifthFill(t), stairOpen(31))
	if st := rebuildState(held); !st.depthPriorityHeld || !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[3] || !slowDeclineFillUnjudged(held, st) {
		t.Fatalf("fixture drifted: held, pending from the fourth fill, the fifth unjudged, got %+v", st)
	}
	reset := ResetRow(stairPrices[4], swingFinding(nil))
	for name, series := range map[string][]float64{"a series that confirms": stairTurns, "a series that cancels": stairRally} {
		for _, state := range []string{"buy", "trailingBuy"} {
			for _, position := range everyProposal() {
				got := Apply(restingIn(held, state), position, armingPrice(held), withBlock(stairBlock(series, 30)))
				assertRow(t, got, reset)
				assertNoSale(t, got, position)
				if st := rebuildState(held); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[3] {
					t.Fatalf("%s resting in %s on %q: Apply writes nothing into the trade, the pending stays as it was, got %+v", name, state, position, st)
				}
			}
		}
	}

	ticked, _ := engineTick(held, "trailingBuy", armingPrice(held), stairOpen(32), withBlock(stairFifthBlock()))
	if st := rebuildState(ticked); !st.depthPriorityHeld || st.slowDeclinePending || !st.swingDeclineWatched || !st.earlyTakeProfit {
		t.Fatalf("the reset leaves the ladder watched, not pending, held and latched, got %+v", st)
	}
	assertSoldNever(t, ticked, withBlock(stairFifthBlock()))

	sixth := withFill(ticked, stairPrices[5], stairOpen(36).Add(17*time.Minute))
	if st := rebuildState(sixth); st.depthPriorityHeld || st.slowDeclinePending {
		t.Fatalf("the next fill ends the hold on a ladder that is not pending, got %+v", st)
	}
	decline := stairBlock(stairDecline, 36)
	pending := PendingRow(stairPrices[5], swingFinding(stairReading(t, sixth, decline, true).Reasons))
	assertActedOn(t, sixth, withBlock(decline), pending)
	assertSoldNever(t, sixth, withBlock(stairBlock(stairRally, 36)))
}

// A ladder pending on both declines is reset on its first held tick by one row,
// whichever of them made it pending: the row names no reasons and the event
// carries both features. The ladder reads not pending from then on, and nothing
// is written, activated or sold while the hold lasts, whatever the ladder proposes.
func TestAHeldLadderPendingOnBothDeclinesIsResetOnce(t *testing.T) {
	withBothDeclines(t)
	withDepthPriorityPause(t, true)
	both := withRow(stairPending(t), PendingRow(stairPrices[3], quietFinding(nil), swingFinding(nil)), stairOpen(24).Add(30*time.Minute))
	held := heldBy(both, stairOpen(25))
	st := rebuildState(held)
	if !st.depthPriorityHeld || !st.slowDeclinePending || len(st.pendingFeatures) != 2 {
		t.Fatalf("fixture drifted: held and pending on both declines, got %+v", st)
	}
	block := withBlock(stairFourthBlock())
	ticked, got := engineTick(held, "trailingBuy", armingPrice(held), stairOpen(26), block)
	want := ResetRow(stairPrices[3], quietFinding(nil), swingFinding(nil))
	assertRow(t, got, want)
	if want.Message != "smartSell: slow decline reset, depth priority holds this ladder" || want.Event != EventReset {
		t.Fatalf("the reset row names no feature but the gate, got %+v", want)
	}
	assertNoSale(t, got, "trailingBuy")
	if st := rebuildState(ticked); st.slowDeclinePending || st.slowDeclinePendingFrom != 0 || len(st.pendingFeatures) != 0 {
		t.Fatalf("the reset ends the pending on both declines, got %+v", st)
	}
	assertSoldNever(t, ticked, block)
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
	if st := rebuildState(pending); !st.slowDeclinePending || !st.earlyTakeProfit || !(input >= 0 && input < fromPosition && fromPosition < fromNewestFill) {
		t.Fatalf("fixture drifted: pending, latched, the moves %v < %v < %v, got %+v", input, fromPosition, fromNewestFill, st)
	}
	if got := TakeProfitPercentage(pending, price, input); got != fromNewestFill {
		t.Errorf("unheld, the take profit reads the newest fill %v, got %v", fromNewestFill, got)
	}
	if got := TakeProfitPercentage(heldBy(pending, stairOpen(25)), price, input); got != fromPosition {
		t.Errorf("held, the newest-fill reading is paused and the latched one goes on at %v, got %v", fromPosition, got)
	}
}
