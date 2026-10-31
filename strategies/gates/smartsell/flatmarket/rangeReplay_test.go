package flatmarket

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// lookbackBars is how many closed 1h bars the replays read: the look-back sophos
// serves the gate, the LowInterestBars of its shared/smartsell/lowinterest (the flat market window after the wire rename)
// package. Mercury cannot import sophos, so the literal mirrors it.
const lookbackBars = 24

// rangeMeasuredAt are the constants the readings below were measured at: the replays run
// only while every one is what it was, and skip, naming the constant, as soon as one
// is retuned. The trend is not among them: the replays inject one that meets
// MinSmcTrend whatever it is, so that they read the bars alone.
var rangeMeasuredAt = []struct {
	name                string
	inForce, measuredAs float64
}{
	{"BodyShare", BodyShare, 0.5},
	{"QuietShare", QuietShare, 0.85},
	{"MaxMovePct", MaxMovePct, 5.0},
}

// rangeNeedsTheMeasuredConstants skips a replay whose measured readings a retune of a
// constant has made meaningless, naming it.
func rangeNeedsTheMeasuredConstants(t *testing.T) {
	t.Helper()
	for _, constant := range rangeMeasuredAt {
		if constant.inForce != constant.measuredAs {
			t.Skipf("%s is %v, the readings were measured at %v", constant.name, constant.inForce, constant.measuredAs)
		}
	}
}

// rangeMetTrend is the Trend Direction row the replays inject for a side: served among the
// rows the dashboard has and just meeting MinSmcTrend the side's way, down
// for a long ladder and up for an inverse one.
func rangeMetTrend(side string) series.Trend {
	if side == aggragates.SideShort {
		return series.Trend{Up: MinSmcTrend, Valid: rangeDashboardRows}
	}
	return series.Trend{Down: MinSmcTrend, Valid: rangeDashboardRows}
}

// replay is one window of a tape's own bars, as the gate reads it for a ladder, and
// what that reading measured at the constants rangeMeasuredAt names: the tape, the open
// time of the window's newest bar (UTC, stampLayout), the ladder's configured
// percentage, how many of the bars are quiet, for a long ladder the fall in percent,
// and the stray: the close between the oldest bar and the newest that lies farthest
// from the mean of those two closes, in percent of the mean and signed, positive above
// the mean and negative below it. A window that misses on its bodies has no fall: the
// reading stops there, and stops before the stray too, so the stray of such a window,
// and of one that misses on its move, is the one the bars measure against the rule
// (strayOf), the one Breaks names where it is over the bound.
type replay struct {
	tape, last string
	percentage float64
	quiet      int
	fallPct    float64
	strayPct   float64
}

var (
	// holdingReplays hold: enough quiet bodies, a fall under the bound, a negative one
	// where the market rose, and every close within the bound of the mean, on either
	// side of it. The last is the window the stray was asked for (userExample).
	holdingReplays = []replay{
		{"solusdt-1h-2025-09.json", "2025-09-19 03:00", 2.2, 24, 0.94, 2.34},
		{"ethusdt-1h-2022-06.json", "2022-06-09 03:00", 2.2, 21, 0.21, 1.65},
		{"solusdt-1h-2022-12.json", "2022-12-14 15:00", 2.4, 22, -2.82, -3.58},
		{"hbarusdt-1h-2025-01.json", "2025-01-30 03:00", 2.4, 21, -2.77, -3.44},
		{"hbarusdt-1h-2026-05.json", "2026-06-02 09:00", 2.4, 23, 4.92, 2.00},
		{"linkusdt-1h-2022-05.json", "2022-05-06 15:00", 2.4, 21, 3.26, -2.27},
		{"linkusdt-1h-2022-05.json", "2022-05-07 14:00", 2.0, 21, 2.38, -1.63},
		userExample,
	}
	// shortReplays miss on their bodies: too few quiet bars to be over the share. Both
	// also stray, and Breaks names the stray after the bodies.
	shortReplays = []replay{
		{"ethusdt-1h-2022-06.json", "2022-06-07 19:00", 2.2, 20, 0, -6.12},
		{"hbarusdt-1h-2026-05.json", "2026-05-29 23:00", 2.4, 15, 0, -5.62},
	}
	// overReplays miss on their move: every body quiet and the fall over the bound, the
	// closes within the bound of the mean.
	overReplays = []replay{
		{"solusdt-1h-2025-09.json", "2025-09-19 15:00", 2.2, 24, 5.76, 3.16},
	}
)

// name is what the replay reads: the symbol of its tape and the open of its newest bar.
func (c replay) name() string {
	symbol, _, _ := strings.Cut(c.tape, "-")
	return strings.ToUpper(symbol) + " " + c.last
}

// lastOpen is the open time, in ms, of the window's newest bar. It fails the test on a
// stamp that is not one.
func (c replay) lastOpen(t *testing.T) int64 {
	t.Helper()
	at, err := time.Parse(stampLayout, c.last)
	if err != nil {
		t.Fatalf("%q is not a stamp: %v", c.last, err)
	}
	return at.UnixMilli()
}

// bars are the lookbackBars of the tape that end at the replay's newest bar.
func (c replay) bars(t *testing.T) Bars {
	t.Helper()
	return windowEnding(t, loadTape(t, c.tape), c.lastOpen(t), lookbackBars)
}

// frame is the reason a reading of the replay's window opens with, written out from
// its stamps: the open times of the oldest and of the newest bar.
func (c replay) frame(t *testing.T) string {
	t.Helper()
	oldest := time.UnixMilli(c.lastOpen(t) - (lookbackBars-1)*series.BarMs).UTC()
	return fmt.Sprintf("range on 1h bars %s to %s UTC", oldest.Format(stampLayout), c.last)
}

// A window of a real tape holds where it was measured to. Read for a long ladder under
// a trend injected as met, it is the lookbackBars bars ending at the replay's newest
// bar, with the quiet count, the fall and the stray measured (the fall and the stray to
// the hundredth of a percent, the stray named in the reasons as the bars measure it),
// and there is nothing left for Breaks to name.
func TestRealBarsHoldWhereTheyWereMeasured(t *testing.T) {
	rangeNeedsTheMeasuredConstants(t)
	side := aggragates.SideLong
	for _, c := range holdingReplays {
		t.Run(c.name(), func(t *testing.T) {
			bars := c.bars(t)
			reading := TriggerRange(bars, c.percentage, side, rangeMetTrend(side))
			if !reading.Holds {
				t.Fatalf("does not hold, breaks %q", RangeBreaks(bars, c.percentage, side, rangeMetTrend(side)))
			}
			if reading.Of != lookbackBars || reading.Quiet != c.quiet {
				t.Errorf("%d of %d bars quiet, want %d of %d", reading.Quiet, reading.Of, c.quiet, lookbackBars)
			}
			assertNear(t, "fall", reading.MovePct, c.fallPct, 0.01)
			assertNear(t, "stray", reading.StrayPct, c.strayPct, 0.01)
			assertNear(t, "limit of a body", reading.BodyMaxPct, BodyShare*c.percentage, 1e-9)
			if reading.To != c.lastOpen(t) || reading.To-reading.From != (lookbackBars-1)*series.BarMs {
				t.Errorf("window from %d to %d, want %d bars one bar apart ending at %s", reading.From, reading.To, lookbackBars, c.last)
			}
			if len(reading.Reasons) == 0 || reading.Reasons[0] != c.frame(t) {
				t.Errorf("reasons %q, want them opened by %q", reading.Reasons, c.frame(t))
			}
			if len(reading.Reasons) < 4 || reading.Reasons[3] != strayHoldText(bars) {
				t.Errorf("reasons %q, want the fourth to name the closes against their mean as %q", reading.Reasons, strayHoldText(bars))
			}
			if got := RangeBreaks(bars, c.percentage, side, rangeMetTrend(side)); got != nil {
				t.Errorf("a window that holds breaks %q", got)
			}
		})
	}
}

// assertMissOn fails unless the replay's window, read for a long ladder under a trend
// injected as met, misses with no text and the quiet count measured, the stray of its
// bars is the one the replay says (strayOf), and Breaks names the frame and then
// exactly the conditions it missed, one reason each in the order of causes, each
// opening with its cause. A reason that names the stray names it as the replay says
// (strayNamed). It answers the reading for the assertions that are the caller's own.
func assertMissOn(t *testing.T, c replay, causes ...string) RangeReading {
	t.Helper()
	bars, side := c.bars(t), aggragates.SideLong
	reading := TriggerRange(bars, c.percentage, side, rangeMetTrend(side))
	if reading.Holds || reading.Reasons != nil {
		t.Fatalf("want a miss with no text, got %+v", reading)
	}
	if reading.Of != lookbackBars || reading.Quiet != c.quiet {
		t.Errorf("%d of %d bars quiet, want %d of %d", reading.Quiet, reading.Of, c.quiet, lookbackBars)
	}
	assertNear(t, "stray of the bars", strayOf(bars), c.strayPct, 0.01)
	breaks := RangeBreaks(bars, c.percentage, side, rangeMetTrend(side))
	if len(breaks) != 1+len(causes) || breaks[0] != c.frame(t) {
		t.Fatalf("breaks %q, want the frame and %d reasons opening %q", breaks, len(causes), causes)
	}
	for slot, cause := range causes {
		if text := breaks[1+slot]; !strings.HasPrefix(text, cause) {
			t.Errorf("break %d is %q, want it opening %q", 1+slot, text, cause)
		} else if cause == strayCause && !strings.Contains(text, c.strayNamed()) {
			t.Errorf("break %d is %q, want it naming the stray as %q", 1+slot, text, c.strayNamed())
		}
	}
	return reading
}

// Windows of a real tape with too few quiet bars miss on their bodies, whatever the
// market did over them: the quiet count measured is short of the share, and the move
// is not what missed. Both stray too, which Breaks names after the bodies, though
// Trigger has stopped before it.
func TestRealBarsWithTooFewQuietBodiesMissOnTheBodies(t *testing.T) {
	rangeNeedsTheMeasuredConstants(t)
	for _, c := range shortReplays {
		t.Run(c.name(), func(t *testing.T) {
			assertMissOn(t, c, "quiet bodies", strayCause)
		})
	}
}

// A window of a real tape whose bodies are all quiet but whose fall is over the bound
// misses on its move: the fall measured, to the hundredth of a percent, is not under
// MaxMovePct, and neither the bodies nor the stray are what missed.
func TestRealBarsWithAFallOverTheBoundMissOnTheMove(t *testing.T) {
	rangeNeedsTheMeasuredConstants(t)
	for _, c := range overReplays {
		t.Run(c.name(), func(t *testing.T) {
			reading := assertMissOn(t, c, "fall")
			assertNear(t, "fall", reading.MovePct, c.fallPct, 0.01)
			if reading.MovePct < MaxMovePct {
				t.Errorf("fall %v is under %v, a window that misses on its move is not", reading.MovePct, MaxMovePct)
			}
		})
	}
}

// An inverse ladder reads the mirror of the same bars. The window and the bodies are
// the same, and the move is the rise, from the oldest bar's low to the newest bar's
// high, under a trend read up. A market that rose reads a negative fall and a positive
// rise, one that fell the other way round, and no window reads negative both ways. The
// closes against their mean are read alike by the two sides, so the mirror holds where
// the rise is under the bound and the closes are within it.
func TestAnInverseLadderReadsTheRiseOfTheSameRealBars(t *testing.T) {
	rangeNeedsTheMeasuredConstants(t)
	long, inverse := aggragates.SideLong, aggragates.SideShort
	windows := slices.Concat(holdingReplays, overReplays)
	for _, c := range strayReplays {
		windows = append(windows, c.replay)
	}
	var risen, fallen int
	for _, c := range windows {
		t.Run(c.name(), func(t *testing.T) {
			bars := c.bars(t)
			fall := TriggerRange(bars, c.percentage, long, rangeMetTrend(long))
			rise := TriggerRange(bars, c.percentage, inverse, rangeMetTrend(inverse))
			if rise.Quiet != fall.Quiet || rise.Of != fall.Of || rise.From != fall.From || rise.To != fall.To {
				t.Errorf("the bodies and the window are the side's alike, got %+v against %+v", rise, fall)
			}
			if rise.Mean != 0 && fall.Mean != 0 && (rise.Mean != fall.Mean || rise.StrayPct != fall.StrayPct || rise.StrayAt != fall.StrayAt) {
				t.Errorf("the closes against their mean are the side's alike, got %+v against %+v", rise, fall)
			}
			want := moveOf(bars, inverse)
			assertNear(t, "rise", rise.MovePct, want, 1e-9)
			if rise.Holds != (want < MaxMovePct && math.Abs(strayOf(bars)) <= MaxMovePct) {
				t.Errorf("holds %v at a rise of %v and a stray of %v against the bound %v", rise.Holds, want, strayOf(bars), MaxMovePct)
			}
			if fall.MovePct < 0 {
				risen++
				if rise.MovePct <= 0 {
					t.Errorf("the market rose (fall %v) and reads a rise of %v", fall.MovePct, rise.MovePct)
				}
			}
			if rise.MovePct < 0 {
				fallen++
				if fall.MovePct <= 0 {
					t.Errorf("the market fell (rise %v) and reads a fall of %v", rise.MovePct, fall.MovePct)
				}
			}
		})
	}
	if risen == 0 || fallen == 0 {
		t.Errorf("the windows hold %d where the market rose and %d where it fell, want both read", risen, fallen)
	}
}
