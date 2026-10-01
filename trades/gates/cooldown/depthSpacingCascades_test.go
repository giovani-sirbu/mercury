package cooldown

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The rule on its own, every boundary at the nanosecond. An activation
// continues the cascade only when its depth filled strictly before the previous
// hold's expiry (the price release bought it) and it landed less than
// DepthSpacingWindow past that expiry. Either failing starts the count over, and
// the first activation has nothing to continue however its times read.
func TestDepthSpacingCascadesDecidesOnTheFillAndTheWindow(t *testing.T) {
	expiry := testutil.At("13:00:00")
	bought := expiry.Add(-time.Minute)
	waited := expiry.Add(DepthSpacingWindow / 2)

	cases := []struct {
		name      string
		step      int
		activated time.Time
		filled    time.Time
		want      bool
	}{
		{"the first activation has nothing to continue", 0, expiry, bought, false},
		{"filled inside the hold", 1, expiry, bought, true},
		{"filled a nanosecond before the expiry", 1, expiry, expiry.Add(-time.Nanosecond), true},
		{"filled at the expiry", 1, expiry, expiry, false},
		{"filled a nanosecond after the expiry", 1, expiry, expiry.Add(time.Nanosecond), false},
		{"filled half a window after the expiry", 1, waited.Add(time.Minute), waited, false},
		{"activated before the expiry", 1, expiry.Add(-30 * time.Second), bought, true},
		{"activated a nanosecond short of the window", 1, expiry.Add(DepthSpacingWindow - time.Nanosecond), bought, true},
		{"activated exactly the window past the expiry", 1, expiry.Add(DepthSpacingWindow), bought, false},
		{"activated a nanosecond past the window", 1, expiry.Add(DepthSpacingWindow + time.Nanosecond), bought, false},
		{"filled at the expiry and activated a window past it", 1, expiry.Add(DepthSpacingWindow), expiry, false},
		{"a long cascade is never capped", 40, expiry, bought, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := depthSpacingCascades(c.step, expiry, c.activated, c.filled); got != c.want {
				t.Fatalf("depthSpacingCascades(step %d, filled %s, activated %s past the expiry) = %v, want %v",
					c.step, c.filled.Sub(expiry), c.activated.Sub(expiry), got, c.want)
			}
		})
	}
}

// The same boundaries through the fold, on the second depth of a ladder whose
// first depth was held: the step that depth reads, and the hold and expiry that
// step earns, move only across the nanosecond that separates a price release
// from a time release, and the one that separates a pause from a cascade.
func TestDepthSpacingCascadeBoundariesThroughTheFold(t *testing.T) {
	start := testutil.At("09:00:00")
	expiry := start.Add(depthSpacingHoldFor(1))
	written := []aggragates.TradesStrategyEvents{spacingEvent(1, start.Add(time.Minute))}

	cases := []struct {
		name      string
		fill      time.Duration // from the previous expiry
		activated time.Duration // from the previous expiry
		wantStep  int
	}{
		{"filled a nanosecond before the expiry", -time.Nanosecond, 0, 2},
		{"filled at the expiry", 0, 0, 1},
		{"activated a nanosecond short of the window", -time.Minute, DepthSpacingWindow - time.Nanosecond, 2},
		{"activated exactly the window past the expiry", -time.Minute, DepthSpacingWindow, 1},
		{"activated a nanosecond past the window", -time.Minute, DepthSpacingWindow + time.Nanosecond, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fill := expiry.Add(c.fill)
			activated := expiry.Add(c.activated)
			next := append(written[:1:1], spacingEvent(2, activated))

			state := spacingState([]time.Time{start, fill}, next, activated)
			if state.step != c.wantStep {
				t.Fatalf("step = %d, want %d", state.step, c.wantStep)
			}
			if wantHold := depthSpacingHoldFor(c.wantStep); state.hold != wantHold {
				t.Errorf("hold = %s, want %s", state.hold, wantHold)
			}
			if want := fill.Add(depthSpacingHoldFor(c.wantStep)); !state.eligibleFrom.Equal(want) {
				t.Errorf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
			}
		})
	}
}

// Activations that skip depths: the gate held depth 1 and depth 3 and let
// depth 2 through. The count follows the activations, and the fill that decides
// is the one of the depth that activated, not of the depth between: depth 3
// filled inside depth 1's hold, so the release bought it and every depth
// between; depth 3 filled after the expiry, however early depth 2 came, waited.
func TestDepthSpacingSkippedDepthsEscalateOnTheFillOfTheActivatedDepth(t *testing.T) {
	start := testutil.At("09:00:00")
	third := depthSpacingHoldFor(1) / 3
	expiry := start.Add(depthSpacingHoldFor(1))
	unheld := start.Add(third)

	cases := []struct {
		name     string
		fill3    time.Time
		wantStep int
	}{
		{"depth 3 filled inside depth 1's hold", start.Add(2 * third), 2},
		{"depth 3 filled a minute short of depth 1's expiry", expiry.Add(-time.Minute), 2},
		{"depth 3 filled the instant depth 1's hold lifts", expiry, 1},
		{"depth 3 filled after depth 1's hold, depth 2 inside it", expiry.Add(time.Minute), 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			activated := c.fill3.Add(time.Minute)
			if activated.Sub(expiry) >= DepthSpacingWindow {
				t.Fatal("fixture drifted: the activation must land inside the window, or the window is what resets")
			}
			written := []aggragates.TradesStrategyEvents{spacingEvent(1, start.Add(time.Minute)), spacingEvent(3, activated)}

			state := spacingState([]time.Time{start, unheld, c.fill3}, written, activated)
			if state.step != c.wantStep {
				t.Fatalf("step = %d, want %d", state.step, c.wantStep)
			}
			if wantHold := depthSpacingHoldFor(c.wantStep); state.hold != wantHold {
				t.Errorf("hold = %s, want %s", state.hold, wantHold)
			}
			if want := c.fill3.Add(depthSpacingHoldFor(c.wantStep)); !state.eligibleFrom.Equal(want) {
				t.Errorf("eligibleFrom = %s, want the hold of the activated depth %s", state.eligibleFrom, want)
			}
		})
	}
}

// The step counts activations however many depths fill between them. Depths 1,
// 3 and 6 were held, each filled a minute short of the previous activation's
// expiry, so they read steps 1, 2 and 3 and the hold escalates with them; depths
// 2, 4 and 5 filled with no hold and deepen nothing.
func TestDepthSpacingTheStepCountsActivationsAcrossSkippedDepths(t *testing.T) {
	start := testutil.At("09:00:00")
	fill3 := start.Add(depthSpacingHoldFor(1)).Add(-time.Minute)
	fill6 := fill3.Add(depthSpacingHoldFor(2)).Add(-time.Minute)
	between := func(from, to time.Time, parts, which int) time.Time {
		return from.Add(to.Sub(from) / time.Duration(parts) * time.Duration(which))
	}
	placements := []time.Time{
		start, between(start, fill3, 2, 1), fill3,
		between(fill3, fill6, 3, 1), between(fill3, fill6, 3, 2), fill6,
	}
	written := []aggragates.TradesStrategyEvents{
		spacingEvent(1, start.Add(time.Minute)),
		spacingEvent(3, fill3.Add(time.Minute)),
		spacingEvent(6, fill6.Add(time.Minute)),
	}

	for index, c := range []struct {
		depth    int
		wantStep int
	}{{1, 1}, {3, 2}, {6, 3}} {
		t.Run(fmt.Sprintf("depth %d", c.depth), func(t *testing.T) {
			state := spacingState(placements[:c.depth], written, written[index].CreatedAt)
			if state.step != c.wantStep {
				t.Fatalf("step = %d, want %d", state.step, c.wantStep)
			}
			if want := placements[c.depth-1].Add(depthSpacingHoldFor(c.wantStep)); !state.eligibleFrom.Equal(want) {
				t.Errorf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
			}
		})
	}
}

// An inverse ladder enters on SELL and its price release is a rise, but the
// cascade is read off the same clocks: a SELL depth that filled inside the
// previous hold escalates, one that waited the hold out starts over, and the
// BUY row that is the ladder's exit leg is never one of its depths. Each case
// is folded beside the same ladder as a long, and the two must agree.
func TestDepthSpacingReadsAnInverseLadderTheWayItReadsALongOne(t *testing.T) {
	start := testutil.At("09:00:00")
	expiry := start.Add(depthSpacingHoldFor(1))
	written := []aggragates.TradesStrategyEvents{spacingEvent(1, start.Add(time.Minute))}

	cases := []struct {
		name     string
		fill     time.Time
		wantStep int
	}{
		{"bought out of the hold by the price release", expiry.Add(-time.Minute), 2},
		{"a nanosecond short of the expiry", expiry.Add(-time.Nanosecond), 2},
		{"waited the hold out", expiry, 1},
		{"filled after the hold", expiry.Add(time.Minute), 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			now := c.fill.Add(time.Minute)
			long := testutil.DepthTrade(start, c.fill)
			short := testutil.LadderTrade(true,
				testutil.LadderFill{Price: 100, At: start},
				testutil.LadderFill{Price: 102, At: c.fill},
			)
			// The exit leg of a short is a BUY: bookkeeping inside the hold, never a depth.
			short.History = append(short.History, aggragates.TradesHistory{
				Type: "BUY", Quantity: 1, Price: 98, OrderId: 99, CreatedAt: start.Add(depthSpacingHoldFor(1) / 2),
			})

			asShort := depthSpacingEligibleFrom(written, depthFills(short), now)
			if asShort.step != c.wantStep {
				t.Fatalf("inverse: step = %d, want %d", asShort.step, c.wantStep)
			}
			if asLong := depthSpacingEligibleFrom(written, depthFills(long), now); !sameSpacingState(asShort, asLong) {
				t.Errorf("inverse state = %+v, the same ladder as a long reads %+v", asShort, asLong)
			}
			if want := c.fill.Add(depthSpacingHoldFor(c.wantStep)); !asShort.eligibleFrom.Equal(want) {
				t.Errorf("inverse: eligibleFrom = %s, want %s", asShort.eligibleFrom, want)
			}
		})
	}
}

// The inverse ladder end to end on what gates.SaveHoldLog wrote: the SELL depth
// that filled inside the first hold is held at step 2, the one that waited the
// hold out at step 1.
func TestDepthSpacingHoldsAnInverseLadderAtTheStepItsFillsEarn(t *testing.T) {
	start := testutil.At("09:00:00")
	expiry := start.Add(depthSpacingHoldFor(1))

	cases := []struct {
		name     string
		fill     time.Time
		wantStep int
	}{
		{"bought out of the hold by the price release", expiry.Add(-time.Minute), 2},
		{"waited the hold out", expiry, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			trade := testutil.LadderTrade(true, testutil.LadderFill{Price: 100, At: start})
			trade.Strategy.Params.Cooldown = true

			held, one := heldAt(t, trade, start.Add(time.Minute))
			if !strings.Contains(one.Reason, "(depth 1, step 1)") {
				t.Fatalf("depth 1 held as %q, want step 1", one.Reason)
			}

			held.History = append(held.History, aggragates.TradesHistory{
				Type: "SELL", Quantity: 1, Price: 102, OrderId: 2, CreatedAt: c.fill,
			})
			held.PositionPrice = 102
			_, two := heldAt(t, held, c.fill.Add(time.Minute))
			if want := fmt.Sprintf("(depth 2, step %d)", c.wantStep); !strings.Contains(two.Reason, want) {
				t.Fatalf("depth 2 held as %q, want %s", two.Reason, want)
			}
			if data, ok := two.Data.(DepthSpacingEvent); !ok || data.Hold != depthSpacingHoldFor(c.wantStep) {
				t.Fatalf("hold data = %+v, want the hold of step %d, %s", two.Data, c.wantStep, depthSpacingHoldFor(c.wantStep))
			}
		})
	}
}
