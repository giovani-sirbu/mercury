package smarttakeloss

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// A depth priority hold pauses the slow pattern decline's judgement too: a new
// fill on a pending ladder is not judged while the hold stands, whatever the
// series serves and whether the band is reached — nothing is written and
// nothing is sold. The ladder's next fill ends the hold, and that fill is the
// one judged: confirmed, or cancelled, on the series that has closed its bar.
func TestAHeldPatternLadderJudgesNothingAndItsNextFillIsJudged(t *testing.T) {
	held := heldBy(stairFifthFill(t), stairOpen(31))
	if st := rebuildState(held); !st.depthPriorityHeld || !st.slowPatternPending || st.slowPatternPendingFrom != stairPrices[3] || !slowPatternFillUnjudged(held, st) {
		t.Fatalf("fixture drifted: held, pending from the fourth fill, the fifth unjudged, got %+v", st)
	}
	for name, series := range map[string][]float64{"a series that confirms": stairTurns, "a series that cancels": stairRally} {
		for _, price := range []float64{underTheBand, slowDeclineBand + 1} {
			assertUntouched(t, Apply(held, "", price, withBlock(stairBlock(series, 30))), "")
			if st := rebuildState(held); !st.slowPatternPending || st.slowPatternPendingFrom != stairPrices[3] {
				t.Fatalf("%s at %v: the hold judges nothing, the pending stays as it was, got %+v", name, price, st)
			}
		}
	}

	sixth := withFill(held, stairPrices[5], stairOpen(36).Add(17*time.Minute))
	if st := rebuildState(sixth); st.depthPriorityHeld || !st.slowPatternPending || !slowPatternFillUnjudged(sixth, st) {
		t.Fatalf("the next fill ends the hold on a ladder still pending and unjudged, got %+v", st)
	}
	confirmed := Apply(sixth, "", underTheBand, withBlock(stairBlock(stairDecline, 36)))
	if row := confirmed.SlowDecline; row == nil || row.Gate != GateSlowPattern || row.Event != EventPending || row.Price != stairPrices[5] || !strings.HasPrefix(row.Reasons[0], "depth 1 to 6, 1h bars ") {
		t.Errorf("want the pending row at the sixth fill, got %+v", confirmed.SlowDecline)
	}
	cancelled := Apply(sixth, "", underTheBand, withBlock(stairBlock(stairRally, 36)))
	if row := cancelled.SlowDecline; row == nil || row.Gate != GateSlowPattern || row.Event != EventCancelled || row.Price != stairPrices[5] {
		t.Errorf("want the cancelled row at the sixth fill, got %+v", cancelled.SlowDecline)
	}
}

// The hold never resets the slow pattern: the first held tick writes the quiet
// slow decline's reset row for a ladder pending on both rules, folding the quiet
// exit away, and the pattern stays pending from the fill it was, with no row of
// its own on that tick or on any later held one, and no second latch.
func TestAHeldLadderPendingOnBothRulesIsResetOnTheQuietRuleAlone(t *testing.T) {
	both := withRow(stairPending(t), PendingRow("buy", stairPrices[3], nil), stairOpen(24).Add(30*time.Minute))
	held := heldBy(both, stairOpen(25))
	if st := rebuildState(held); !st.depthPriorityHeld || !st.slowDeclinePending || !st.slowPatternPending {
		t.Fatalf("fixture drifted: held and pending on both rules, got %+v", st)
	}
	block := withBlock(stairBlock(stairTurns, 24))
	ticked, got := engineTick(held, "", slowDeclineBand+1, stairOpen(26), block)
	assertRow(t, got.SlowDecline, SlowDeclineResetMessage("buy"), stairPrices[3])
	if got.Position != "" || got.Indecision != nil || got.Reason != "" {
		t.Fatalf("the reset row is the only answer and nothing is sold, got %+v", got)
	}
	if st := rebuildState(ticked); st.slowDeclinePending || !st.slowPatternPending || st.slowPatternPendingFrom != stairPrices[3] {
		t.Fatalf("the quiet exit is reset and the pattern stays pending from its fill, got %+v", st)
	}
	assertUntouched(t, Apply(ticked, "", slowDeclineBand+1, block), "")
}

// A held, pattern-pending and latched ladder has its newest-fill take profit
// paused while its latched take profit goes on: unheld the take profit reads
// the larger of the move from the newest fill and the move from the position
// price, held the move from the position price alone, never the input's under
// it, and a hold that ends with the next fill gives the newest fill back.
func TestAHeldPatternLadderKeepsTheLatchedTakeProfitOnly(t *testing.T) {
	pending := stairPending(t)
	pending.PositionPrice = 190
	price := 195.0
	input, fromPosition, fromNewestFill := moveAgainst(price, ladder.AverageEntryPrice(pending)), moveAgainst(price, 190), moveAgainst(price, stairPrices[3])
	if st := rebuildState(pending); !st.slowPatternPending || !st.indecision || !(input >= 0 && input < fromPosition && fromPosition < fromNewestFill) {
		t.Fatalf("fixture drifted: pending, latched, the moves %v < %v < %v, got %+v", input, fromPosition, fromNewestFill, st)
	}
	if got := TakeProfitPercentage(pending, price, input); got != fromNewestFill {
		t.Errorf("unheld, the take profit reads the newest fill %v, got %v", fromNewestFill, got)
	}
	if got := TakeProfitPercentage(heldBy(pending, stairOpen(25)), price, input); got != fromPosition {
		t.Errorf("held, the newest-fill reading is paused and the latched one goes on at %v, got %v", fromPosition, got)
	}
}
