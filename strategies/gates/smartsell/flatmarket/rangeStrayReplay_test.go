package flatmarket

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// strayCause is what the reason that names a close straying from the mean opens with,
// as the replays' assertions name it (assertMissOn).
const strayCause = "close at"

// strayReplay is a replay of a window whose closes stray from the mean over the bound,
// and what that reading measured at the constants rangeMeasuredAt names beside the replay's
// own columns: the open time of the bar whose close strays (UTC, stampLayout), and the
// mean of the oldest and the newest close the stray is measured against, as the tape's
// prices make it.
type strayReplay struct {
	replay
	at   string
	mean float64
}

// strayReplays miss on the stray alone: enough quiet bodies and a fall under the bound,
// and one close between the oldest bar and the newest farther from the mean of those
// two closes than the bound, above it in the first window and below it in the second.
var strayReplays = []strayReplay{
	{replay{"solusdt-1h-2022-05.json", "2022-05-05 14:00", 2.4, 21, 0.13, 7.73}, "2022-05-05 03:00", 87.55},
	{replay{"ethusdt-1h-2022-06.json", "2022-06-07 18:00", 2.2, 21, 4.68, -5.16}, "2022-06-07 02:00", 1835.68},
}

// strayOf is the stray of the bars written out against the rule, so that a reason
// names the number the measurement made and not the one a replay was given: the close
// between the oldest bar and the newest farthest from the mean of those two, in percent
// of the mean and signed.
func strayOf(bars Bars) float64 {
	_, pct, _ := closesAgainstMean(bars)
	return pct
}

// strayNamed is how the reason that names the replay's stray says how far it lies and on
// which side of the mean, to the tenth of a percent.
func (c replay) strayNamed() string {
	where := "above"
	if c.strayPct < 0 {
		where = "below"
	}
	return fmt.Sprintf(" strays %.1f%% %s the mean ", math.Abs(c.strayPct), where)
}

// A window of a real tape whose bodies are quiet enough and whose fall is under the
// bound, but one of whose closes lies farther from the mean of the oldest close and the
// newest than the bound, misses on the stray alone: Trigger stops at it with no text and
// with the stray measured, how far, at which bar and against which mean, and Breaks names
// the frame and that one close, the bar, how far and on which side of the mean, over the
// bound.
func TestRealBarsWithACloseStrayingFromTheMeanMissOnTheStray(t *testing.T) {
	rangeNeedsTheMeasuredConstants(t)
	side := aggragates.SideLong
	for _, c := range strayReplays {
		t.Run(c.name(), func(t *testing.T) {
			reading := assertMissOn(t, c.replay, strayCause)
			assertNear(t, "fall", reading.MovePct, c.fallPct, 0.01)
			if reading.MovePct >= MaxMovePct {
				t.Errorf("fall %v is not under %v, a window that misses on its stray alone is", reading.MovePct, MaxMovePct)
			}
			assertNear(t, "stray", reading.StrayPct, c.strayPct, 0.01)
			assertNear(t, "mean", reading.Mean, c.mean, 1e-6)
			at, err := time.Parse(stampLayout, c.at)
			if err != nil {
				t.Fatalf("%q is not a stamp: %v", c.at, err)
			}
			if reading.StrayAt != at.UnixMilli() {
				t.Errorf("the stray is at %s, want %s", barTime(reading.StrayAt), c.at)
			}
			if math.Abs(reading.StrayPct) <= MaxMovePct {
				t.Errorf("stray %v is within %v, a window that misses on it is not", reading.StrayPct, MaxMovePct)
			}
			want := strayCause + " " + c.at + c.strayNamed() + meanString(c.mean) + fmt.Sprintf(" over %.1f%%", float64(MaxMovePct))
			if breaks := RangeBreaks(c.bars(t), c.percentage, side, rangeMetTrend(side)); len(breaks) != 2 || breaks[1] != want {
				t.Errorf("breaks %q, want the frame and %q", breaks, want)
			}
		})
	}
}
