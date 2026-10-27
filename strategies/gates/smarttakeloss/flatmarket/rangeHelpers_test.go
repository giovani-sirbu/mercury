package flatmarket

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// firstBar is the open time of the first bar of every synthetic window.
var firstBar = time.Date(2022, time.June, 8, 4, 0, 0, 0, time.UTC)

const (
	// rangeFixtureBars is how many bars a synthetic window holds: as many as sophos
	// serves the gate, and never fewer than the rule reads, so that the share of quiet
	// bars has room to fall one bar short. fixturePercentage is the percentage of the
	// ladder the windows are scaled to, basePrice the open of every bar and wickPct how
	// far past its body a bar's high and low reach, in percent.
	rangeFixtureBars  = max(24, MinBars)
	fixturePercentage = 2.0
	basePrice         = 100.0
	wickPct           = 0.1
	// quietFraction is how far into the limit of a body a quiet bar's body lies,
	// and loudFactor how many limits a loud bar's body is.
	quietFraction = 0.5
	loudFactor    = 2.0
	// rangeDashboardRows is how many rows the SMC trend dashboard reads its Trend
	// Direction row on: the 1h chart row and the two rows above it.
	rangeDashboardRows = 3
)

// rangeSideCase is one side of the gate with the trends and the words that belong to it:
// met a trend that just meets MinSmcTrend on the side, short one a row
// short of it and wrong one that meets it the other way, so that no test leans on the
// constant's value.
type rangeSideCase struct {
	name, side, trendName, moveName string
	met, short, wrong               series.Trend
}

var (
	// trendRows is how many rows a synthetic trend has read: one more than the
	// dashboard has and the need asks, so that a reason's two counts tell apart.
	trendRows      = max(rangeDashboardRows, MinSmcTrend) + 1
	rangeSideCases = []rangeSideCase{
		{
			name: "long", side: aggragates.SideLong, trendName: "down", moveName: "fall",
			met:   series.Trend{Down: MinSmcTrend, Valid: trendRows},
			short: series.Trend{Down: MinSmcTrend - 1, Valid: trendRows},
			wrong: series.Trend{Up: MinSmcTrend, Valid: trendRows},
		},
		{
			name: "inverse", side: aggragates.SideShort, trendName: "up", moveName: "rise",
			met:   series.Trend{Up: MinSmcTrend, Valid: trendRows},
			short: series.Trend{Up: MinSmcTrend - 1, Valid: trendRows},
			wrong: series.Trend{Down: MinSmcTrend, Valid: trendRows},
		},
	}
)

// count is how many rows of a trend read the side's way, written out against the
// rule: down for a long ladder, up for an inverse one.
func (sc rangeSideCase) count(trend series.Trend) int {
	if sc.side == aggragates.SideShort {
		return trend.Up
	}
	return trend.Down
}

// window is a synthetic window of bars: of bars (rangeFixtureBars when zero), quiet of
// them with a quiet body and the rest a loud one, bodies alternating up and down, a
// move of exactly movePct on the side, a stray of exactly strayPct (recentred) and,
// when closeRangePct is asked, a span of the closes of exactly that (stretched).
type window struct {
	of, quiet                        int
	movePct, strayPct, closeRangePct float64
	side                             string
}

// build is the window scaled to the ladder's percentage. The bodies alternate from
// an up one for a long ladder and from a down one for an inverse ladder, the mirror,
// so that the oldest close lies where the move starts. The newest bar is brought to
// the move (moved), then stretched to the span of the closes if one is asked, and the
// closes between are last recentred.
func (w window) build(percentage float64) Bars {
	of := w.of
	if of == 0 {
		of = rangeFixtureBars
	}
	limit := BodyShare * percentage
	inverse := w.side == aggragates.SideShort
	bars := Bars{
		Opens: make([]int64, of), OpenPrices: make([]float64, of),
		Highs: make([]float64, of), Lows: make([]float64, of), Closes: make([]float64, of),
	}
	for index := range of {
		body := quietFraction * limit
		if index >= w.quiet {
			body = loudFactor * limit
		}
		if (index%2 == 1) != inverse {
			body = -body
		}
		closed := basePrice * (1 + body/100)
		bars.Opens[index] = firstBar.Add(time.Duration(index) * time.Hour).UnixMilli()
		bars.OpenPrices[index] = basePrice
		bars.Closes[index] = closed
		bars.Highs[index] = math.Max(basePrice, closed) * (1 + wickPct/100)
		bars.Lows[index] = math.Min(basePrice, closed) * (1 - wickPct/100)
	}
	return w.recentred(w.stretched(w.moved(bars)))
}

// moved is the bars with the newest bar brought to the move the window asks. All four
// prices of the bar are scaled alike, so that its body keeps its share of its open,
// until the window makes a move of holdMove: that is where the bar's close lies for
// any move at or past it, and a move past it (overMove) is made by a wick, the low of
// the newest bar pulled down for a long ladder and its high pulled up for an inverse
// one, so that a move over the bound does not take the span of the closes with it. A
// move short of holdMove, down to the other way, is the whole bar scaled to it.
func (w window) moved(bars Bars) Bars {
	last := len(bars.Closes) - 1
	inverse := w.side == aggragates.SideShort
	scale := func(movePct float64) float64 {
		if inverse {
			return bars.Lows[0] * (1 + movePct/100) / bars.Highs[last]
		}
		return bars.Highs[0] * (1 - movePct/100) / bars.Lows[last]
	}
	factor, resting := scale(w.movePct), scale(holdMove)
	if inverse {
		factor = min(factor, resting)
	} else {
		factor = max(factor, resting)
	}
	wick := bars.Highs[0] * (1 - w.movePct/100)
	if inverse {
		wick = bars.Lows[0] * (1 + w.movePct/100)
	}
	bars.OpenPrices[last] *= factor
	bars.Highs[last] *= factor
	bars.Lows[last] *= factor
	bars.Closes[last] *= factor
	if inverse {
		bars.Highs[last] = max(bars.Highs[last], wick)
	} else {
		bars.Lows[last] = min(bars.Lows[last], wick)
	}
	return bars
}

// stretched is the bars with the newest bar, all four prices scaled alike so that its
// body keeps its share, brought to where the closes span exactly closeRangePct from
// the lowest to the highest, and none when it is zero or below. The oldest close is one
// end of the span and the newest the other, the newest above it for a long ladder and
// below it for an inverse one: the market runs the ladder's way no longer, so that the
// move reads the other way and is under its bound, and the closes between, recentred
// onto the mean of the two ends, are inside the span and stray from nothing. The span
// is therefore the one condition the window misses, as the stray is in recentred, as
// long as no stray is asked of it.
func (w window) stretched(bars Bars) Bars {
	if !(w.closeRangePct > 0) {
		return bars
	}
	last := len(bars.Closes) - 1
	spread := 1 + w.closeRangePct/100
	target := bars.Closes[0] * spread
	if w.side == aggragates.SideShort {
		target = bars.Closes[0] / spread
	}
	scale := target / bars.Closes[last]
	bars.OpenPrices[last] *= scale
	bars.Highs[last] *= scale
	bars.Lows[last] *= scale
	bars.Closes[last] *= scale
	return bars
}

// holding is the window that holds on the side at the fixture percentage: every
// bar quiet and a move of holdMove.
func holding(side string) Bars {
	return window{quiet: rangeFixtureBars, movePct: holdMove, side: side}.build(fixturePercentage)
}

// quietNeeded is the fewest quiet bars, of that many, that are more than
// QuietShare of them: the bound itself falls short, even where it lands on
// a whole bar (the allowance is for the rounding of the product).
func quietNeeded(of int) int {
	return int(math.Floor(QuietShare*float64(of)+1e-9)) + 1
}

// exactBars is the fewest bars the rule reads, from two, at which QuietShare
// lands on a whole number of them, where a share that is not strict would let the
// bound hold.
func exactBars() (int, bool) {
	for of := max(2, MinBars); of <= 200; of++ {
		share := QuietShare * float64(of)
		if math.Abs(share-math.Round(share)) < 1e-9 {
			return of, true
		}
	}
	return 0, false
}

// needsTheTrend, needsAMove and needsAQuietBoundary skip a test whose measured
// assertion a retune of the constant makes meaningless.
func needsTheTrend(t *testing.T) {
	t.Helper()
	if MinSmcTrend <= 0 {
		t.Skip("MinSmcTrend is zero or below: the condition is off and no trend fails a reading")
	}
}

func needsAMove(t *testing.T) {
	t.Helper()
	if MaxMovePct <= 0 || overMove >= 100 {
		t.Skip("MaxMovePct is zero or below, or so wide that no fall is over it: no window can be built")
	}
}

// needsACloseRange skips a test whose measured assertion a retune of the bound that
// turns the condition off makes meaningless: no span of the closes is over it.
func needsACloseRange(t *testing.T) {
	t.Helper()
	if MaxCloseRangePct <= 0 {
		t.Skip("MaxCloseRangePct is zero or below: the condition is off and no span of the closes fails a reading")
	}
}

func needsAQuietBoundary(t *testing.T) {
	t.Helper()
	if need := quietNeeded(rangeFixtureBars); need < 1 || need > rangeFixtureBars {
		t.Skip("QuietShare leaves no count of bars short of it or over it in the fixture window")
	}
}

// stamp is the open time of the synthetic window's bar at index as the frame prints it.
func stamp(index int) string {
	return firstBar.Add(time.Duration(index) * time.Hour).Format("2006-01-02 15:04")
}

// The reasons, written out against the grammar. joint is "over" or "short of" where
// the hold says "over" and the break says "short of", and "under" or "over" for the
// move.
func frameText(of int) string {
	return fmt.Sprintf("range on 1h bars %s to %s UTC", stamp(0), stamp(of-1))
}

func quietText(quiet, of int, joint string) string {
	return fmt.Sprintf(
		"quiet bodies %d/%d under %.2f%%, %s %.0f%%",
		quiet, of, BodyShare*fixturePercentage, joint, float64(QuietShare)*100,
	)
}

func moveText(name string, move float64, joint string) string {
	return fmt.Sprintf("%s %.1f%% %s %.1f%%", name, move, joint, float64(MaxMovePct))
}

func trendText(name string, count int, trend series.Trend, joint string) string {
	return fmt.Sprintf("smc trend direction %s %d/%d %s %d needed", name, count, trend.Valid, joint, MinSmcTrend)
}

// rangeOf is the span of the closes of the bars written out against the rule, so that a
// reason names the number the measurement made and not the one the window was asked,
// which a rounding step could tell apart: the lowest close, the highest and the highest
// over the lowest in percent of the lowest. The bars are served.
func rangeOf(bars Bars) (low, high, pct float64) {
	low, high = slices.Min(bars.Closes), slices.Max(bars.Closes)
	return low, high, (high - low) / low * 100
}

func closeRangeHoldText(bars Bars) string {
	low, high, pct := rangeOf(bars)
	return fmt.Sprintf(
		"closes span %.1f%% from %s to %s, at most %.1f%%",
		pct, meanString(low), meanString(high), float64(MaxCloseRangePct),
	)
}

func closeRangeBreakText(bars Bars) string {
	low, high, pct := rangeOf(bars)
	return fmt.Sprintf(
		"closes span %.1f%% from %s to %s over %.1f%%",
		pct, meanString(low), meanString(high), float64(MaxCloseRangePct),
	)
}

// spanBreaks is what Breaks names of the span of the bars' closes, as the rule reads it
// at whatever the bound is: nothing where the condition is off or the span is within the
// bound, the one reason where it is over. A window that strays, or any real tape, spans
// what its closes span, so its expected list takes this slot from the bars.
func spanBreaks(bars Bars) []string {
	if _, _, pct := rangeOf(bars); MaxCloseRangePct > 0 && pct > float64(MaxCloseRangePct) {
		return []string{closeRangeBreakText(bars)}
	}
	return nil
}

// holdMove is a move well under MaxMovePct and overMove one well over it, in percent: a retune
// moves them with the bound. holdRange is a span of the closes well within MaxCloseRangePct and
// overRange one well over it, in percent of the lowest close, moved with the bound as holdMove
// and overMove are. At a bound of zero or below the condition is off and so are they: a window
// asked for either is not stretched. They are variables, as the bounds are.
var (
	holdMove  = 0.5 * MaxMovePct
	overMove  = 1.5 * MaxMovePct
	holdRange = 0.5 * MaxCloseRangePct
	overRange = 1.5 * MaxCloseRangePct
)
