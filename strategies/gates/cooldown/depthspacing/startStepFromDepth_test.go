package depthspacing

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// withStartStepFromDepth runs the test with the start step's two knobs at on and offset and puts
// them back after.
func withStartStepFromDepth(t *testing.T, on bool, offset int) {
	t.Helper()
	previousOn, previousOffset := startStepFromDepth, startStepOffset
	startStepFromDepth, startStepOffset = on, offset
	t.Cleanup(func() { startStepFromDepth, startStepOffset = previousOn, previousOffset })
}

// startStepProfile is one setting of the two knobs the tables run through.
type startStepProfile struct {
	name   string
	on     bool
	offset int
}

// startStepProfiles are the settings the start step is proved on: from the depth, one under it, off.
var startStepProfiles = []startStepProfile{
	{"on, offset 0", true, 0},
	{"on, offset 1", true, 1},
	{"off", false, 0},
}

// startStepLadder is a ladder filled after[i] past 09:00 at depth i+1, the gate activated at each of
// the activated depths a minute after its fill, as the engine writes it.
func startStepLadder(after []time.Duration, activated []int) ([]time.Time, []aggragates.TradesStrategyEvents) {
	start := testutil.At("09:00:00")
	placements := make([]time.Time, len(after))
	for index, gap := range after {
		placements[index] = start.Add(gap)
	}
	written := make([]aggragates.TradesStrategyEvents, len(activated))
	for index, depth := range activated {
		written[index] = spacingEvent(depth, placements[depth-1].Add(time.Minute))
	}

	return placements, written
}

// startStepsAtActivations are the steps the fold reads at each activation, one per written event, in
// the order they were written: the ladder as it stood at that depth, the tick on the activation.
func startStepsAtActivations(placements []time.Time, written []aggragates.TradesStrategyEvents, depths []int) []int {
	steps := make([]int, len(depths))
	for index, depth := range depths {
		steps[index] = spacingState(placements[:depth], written, written[index].CreatedAt).step
	}

	return steps
}

// heldEvent is the hold the gate answers on the trade at the tick and the event it carries.
func heldEvent(t *testing.T, trade aggragates.Trades, at time.Time) (gates.Hold, cooldown.DepthSpacingEvent) {
	t.Helper()

	hold := DepthSpacingHold(tickOf(trade, at), "stopLoss")
	if !hold.Held() {
		t.Fatalf("the gate let the depth through at %s, want it parked", at)
	}
	data, ok := hold.Data.(cooldown.DepthSpacingEvent)
	if !ok {
		t.Fatalf("hold carries %T, want the depth spacing event", hold.Data)
	}

	return hold, data
}

// A ladder five deep on its first activation starts at step 5 with the flag on: the capped hold, the
// row naming the depth and the step, and a release (percentage + tolerance + percentage*step) under
// the last fill, so a print the flag-off release would pay is still held. One under the depth with
// offset 1, step 1 with the flag off.
func TestDepthSpacingStartStepIsTheLadderDepthOnAFirstActivation(t *testing.T) {
	start := testutil.At("09:00:00")
	placements := make([]time.Time, 5)
	for index := range placements {
		placements[index] = start.Add(time.Duration(index) * 10 * time.Minute)
	}
	now := placements[4].Add(time.Minute)

	cases := []struct {
		profile  startStepProfile
		wantStep int
		wantHold time.Duration
	}{
		{startStepProfiles[0], 5, maxHold},
		{startStepProfiles[1], 4, maxHold},
		{startStepProfiles[2], 1, BaseHold},
	}
	for _, c := range cases {
		t.Run(c.profile.name, func(t *testing.T) {
			withStartStepFromDepth(t, c.profile.on, c.profile.offset)
			trade := testutil.DepthTrade(placements...)
			lastFill := trade.History[4].Price
			trade.PositionPrice = lastFill
			row := trade.StrategyPair.StrategySettings[0]
			releaseAt := func(step int) float64 {
				return lastFill * (1 - (row.Percentage+row.Tolerance+row.Percentage*float64(step))/100)
			}

			state := spacingState(placements, nil, now)
			if state.step != c.wantStep || state.hold != c.wantHold {
				t.Fatalf("state = step %d hold %s, want step %d hold %s", state.step, state.hold, c.wantStep, c.wantHold)
			}
			if want := placements[4].Add(c.wantHold); !state.eligibleFrom.Equal(want) {
				t.Errorf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
			}

			hold, data := heldEvent(t, trade, now)
			if want := fmt.Sprintf("(depth 5, step %d)", c.wantStep); !strings.Contains(hold.Reason, want) {
				t.Errorf("row = %q, want it to name %s", hold.Reason, want)
			}
			if data.Depth != 5 || data.Step != c.wantStep || data.Hold != c.wantHold {
				t.Errorf("event = %+v, want depth 5, step %d, hold %s", data, c.wantStep, c.wantHold)
			}
			if want := releaseAt(c.wantStep); math.Abs(data.Release-want) > 1e-9 {
				t.Errorf("release = %v, want %v on the trade's settings row", data.Release, want)
			}

			// The release is a bound on the print: a hair over it holds, at it the hold lifts.
			over, paid := trade, trade
			over.PositionPrice, paid.PositionPrice = releaseAt(c.wantStep)+0.01, releaseAt(c.wantStep)-1e-6
			if !DepthSpacingHold(tickOf(over, now), "stopLoss").Held() {
				t.Error("a print over the release must stay held")
			}
			if hold := DepthSpacingHold(tickOf(paid, now), "stopLoss"); hold.Held() {
				t.Errorf("a print at the release must lift the hold, got %q", hold.Reason)
			}

			// A print the flag-off release pays: lifted when the step is 1, held when the step starts deeper.
			shallow := trade
			shallow.PositionPrice = releaseAt(1) - 0.05
			if got := DepthSpacingHold(tickOf(shallow, now), "stopLoss").Held(); got != (c.wantStep > 1) {
				t.Errorf("a print under the step 1 release: held = %v, want %v", got, c.wantStep > 1)
			}
		})
	}
}

// The start never goes under step 1: an offset at or over the depth leaves step 1, and the
// hold is the base hold the step earns.
func TestDepthSpacingStartStepFloorsAtOne(t *testing.T) {
	cases := []struct {
		name                string
		depth, offset, want int
	}{
		{"depth 1, offset 0", 1, 0, 1},
		{"depth 1, offset 3", 1, 3, 1},
		{"depth 2, offset 3", 2, 3, 1},
		{"offset the depth", 3, 3, 1},
		{"one over the offset", 4, 3, 1},
		{"two over the offset", 5, 3, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withStartStepFromDepth(t, true, c.offset)
			after := make([]time.Duration, c.depth)
			for index := range after {
				after[index] = time.Duration(index) * 10 * time.Minute
			}
			placements, _ := startStepLadder(after, nil)

			state := spacingState(placements, nil, placements[c.depth-1].Add(time.Minute))
			if state.step != c.want {
				t.Fatalf("step = %d, want %d", state.step, c.want)
			}
			if state.hold != depthSpacingHoldFor(c.want) {
				t.Errorf("hold = %s, want %s", state.hold, depthSpacingHoldFor(c.want))
			}
		})
	}
}

// Only the start of a cascade reads the depth: an activation that continues one keeps its count and
// adds one, even when the depths filled in between were never held. A depth that fills after the
// hold lifted starts over at its own depth.
func TestDepthSpacingStartStepOnlyStartsACascade(t *testing.T) {
	minutes := func(counts ...int) []time.Duration {
		after := make([]time.Duration, len(counts))
		for index, count := range counts {
			after[index] = time.Duration(count) * time.Minute
		}
		return after
	}
	// The fixtures are only these shapes while the holds are under six hours and over one.
	if BaseHold <= time.Hour || maxHold >= 6*time.Hour {
		t.Fatalf("fixture drifted: the base hold %s and the cap %s must straddle the fills", BaseHold, maxHold)
	}

	ladders := []struct {
		name      string
		after     []time.Duration
		activated []int
		want      map[string][]int
	}{
		{
			"the next depth fills inside the hold",
			minutes(0, 10, 20, 80), []int{3, 4},
			map[string][]int{"on, offset 0": {3, 4}, "on, offset 1": {2, 3}, "off": {1, 2}},
		},
		{
			"depth 6 fills inside depth 3's hold, depths 4 and 5 never held",
			minutes(0, 10, 20, 30, 40, 60), []int{3, 6},
			map[string][]int{"on, offset 0": {3, 4}, "on, offset 1": {2, 3}, "off": {1, 2}},
		},
		{
			"depth 6 fills after depth 3's hold lifted",
			minutes(0, 10, 20, 30, 40, 20+6*60), []int{3, 6},
			map[string][]int{"on, offset 0": {3, 6}, "on, offset 1": {2, 5}, "off": {1, 1}},
		},
	}
	for _, ladder := range ladders {
		placements, written := startStepLadder(ladder.after, ladder.activated)
		for _, profile := range startStepProfiles {
			t.Run(ladder.name+"/"+profile.name, func(t *testing.T) {
				withStartStepFromDepth(t, profile.on, profile.offset)
				got := startStepsAtActivations(placements, written, ladder.activated)
				if want := ladder.want[profile.name]; !slices.Equal(got, want) {
					t.Fatalf("steps at depths %v = %v, want %v", ladder.activated, got, want)
				}
			})
		}
	}
}

// The recorded ladder of trade 210913 (SOL, 10 Oct 2025, percentage 1.4, tolerance 0.1): eight fills
// over four days, the gate activated at depths 3, 5, 6, 7 and 8. Depth 5 filled long after depth 3's
// hold and read step 1 as recorded; depths 6, 7 and 8 filled inside the previous hold and cascade.
// Started at the depth the same ladder reads 3, 5, 6, 7, 8, and one under it 2, 4, 5, 6, 7; the row
// of the depth 5 activation parks it for the capped hold, its release 8.5% under the fill (2.9% as
// recorded).
func TestDepthSpacingStartStepOnTheRecordedTrade210913(t *testing.T) {
	at := func(stamp string) time.Time {
		parsed, err := time.Parse("2006-01-02 15:04:05.000", "2025-"+stamp)
		if err != nil {
			t.Fatal(err)
		}
		return parsed.UTC()
	}
	placements := []time.Time{
		at("10-06 21:24:20.296"), at("10-07 06:42:23.206"), at("10-07 13:43:49.095"), at("10-09 08:11:39.571"),
		at("10-10 14:58:19.910"), at("10-10 15:21:54.869"), at("10-10 19:41:43.435"), at("10-10 20:59:48.781"),
	}
	activated := []int{3, 5, 6, 7, 8}
	written := []aggragates.TradesStrategyEvents{
		spacingEvent(3, at("10-07 15:06:51.016")),
		spacingEvent(5, at("10-10 15:04:23.610")),
		spacingEvent(6, at("10-10 15:30:50.736")),
		spacingEvent(7, at("10-10 20:56:01.387")),
		spacingEvent(8, at("10-10 21:12:35.554")),
	}
	wantSteps := map[string][]int{
		"on, offset 0": {3, 5, 6, 7, 8},
		"on, offset 1": {2, 4, 5, 6, 7},
		"off":          {1, 1, 2, 3, 4},
	}
	const lastFill = 214.28

	for _, profile := range startStepProfiles {
		t.Run(profile.name, func(t *testing.T) {
			withStartStepFromDepth(t, profile.on, profile.offset)
			want := wantSteps[profile.name]

			if got := startStepsAtActivations(placements, written, activated); !slices.Equal(got, want) {
				t.Fatalf("steps at depths %v = %v, want %v", activated, got, want)
			}
			for index, depth := range activated {
				state := spacingState(placements[:depth], written, written[index].CreatedAt)
				if state.hold != depthSpacingHoldFor(want[index]) {
					t.Errorf("depth %d: hold = %s, want %s", depth, state.hold, depthSpacingHoldFor(want[index]))
				}
				if wantFrom := placements[depth-1].Add(depthSpacingHoldFor(want[index])); !state.eligibleFrom.Equal(wantFrom) {
					t.Errorf("depth %d: eligibleFrom = %s, want %s", depth, state.eligibleFrom, wantFrom)
				}
			}

			// The depth 5 activation through the gate: its row and the release under the 214.28 fill.
			trade := testutil.DepthTrade(placements[:5]...)
			trade.StrategyEvents = written
			trade.History[4].Price = lastFill
			trade.PositionPrice = lastFill
			trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Depths: 8, Percentage: 1.4, Tolerance: 0.1}}

			hold, data := heldEvent(t, trade, written[1].CreatedAt)
			if wantRow := fmt.Sprintf("(depth 5, step %d)", want[1]); !strings.Contains(hold.Reason, wantRow) {
				t.Errorf("row = %q, want it to name %s", hold.Reason, wantRow)
			}
			wantRelease := lastFill * (1 - (1.4+0.1+1.4*float64(want[1]))/100)
			if math.Abs(data.Release-wantRelease) > 1e-9 {
				t.Errorf("release = %v, want %v", data.Release, wantRelease)
			}
		})
	}
}
