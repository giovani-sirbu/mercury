package flatmarket

import (
	"math"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// LineReading is what the line reads for a ladder of one side: the window (From, To, Of), the line
// (LineStart, LineEnd, MovePct, its SlopePct per hour against the MaxSlope in force) and the Bound,
// the farthest closes above and below the line with their bars, the bars Counted against the
// tolerance (Wide of them on their body) against the Allowed, the Side and Trend, Holds and the
// Reasons a holding reading names; a miss carries no text (LineBreaks).
type LineReading struct {
	From, To           int64
	Of                 int
	LineStart, LineEnd float64
	MovePct, Bound     float64
	SlopePct, MaxSlope float64
	MaxAbove, MaxBelow float64
	AboveAt, BelowAt   int64
	Counted, Wide      int
	Allowed            int
	Side               string
	Trend              series.Trend
	Holds              bool
	Reasons            []string
}

// TriggerLine is the line reading of the bars and the SMC trend for a ladder of that side.
func TriggerLine(bars Bars, percentage float64, side string, trend series.Trend) LineReading {
	return lineAt(bars, percentage, side, trend, MinSmcTrend, MaxSlopePercentage)
}

// TriggerLineOnBars is TriggerLine on the bars alone: no trend is read and no reason names it.
func TriggerLineOnBars(bars Bars, percentage float64, side string) LineReading {
	return lineAt(bars, percentage, side, series.Trend{}, 0, MaxSlopePercentage)
}

// lineAt is TriggerLine against need, the least rows of the trend that must read the ladder's way,
// and maxSlope, the steepest the line may slope per hour; at zero or below no trend is read and no
// slope is capped.
func lineAt(bars Bars, percentage float64, side string, trend series.Trend, need int, maxSlope float64) LineReading {
	reading := LineReading{Side: side, Trend: trend, MaxSlope: maxSlope}
	if !(percentage > 0) {
		return reading
	}
	reading.Bound = MaxDeviation * percentage
	if !trend.Meets(side, need) {
		return reading
	}
	if !lineServed(bars) {
		return reading
	}
	reading = reading.measured(bars)
	if !flat(reading.MovePct, reading.Bound) && !declines(reading.MovePct, side) {
		return reading
	}
	if !withinSlope(reading.SlopePct, maxSlope) {
		return reading
	}
	reading = reading.deviated(fit(bars.Closes).read(bars, reading.Bound))
	if reading.Counted > reading.Allowed {
		return reading
	}
	reading.Holds = true
	reading.Reasons = lineHoldReasons(reading, need)
	return reading
}

// measured is the reading with the window and the line of served bars.
func (reading LineReading) measured(bars Bars) LineReading {
	last := bars.Len() - 1
	fitted := fit(bars.Closes)
	reading.From, reading.To, reading.Of = bars.Opens[0], bars.Opens[last], last+1
	reading.LineStart, reading.LineEnd, reading.MovePct = fitted.at(0), fitted.at(last), fitted.movePct(last)
	reading.SlopePct = slopePerHour(reading.MovePct, reading.Of)
	return reading
}

// deviated is the reading with what the bars read against the line and the bound.
func (reading LineReading) deviated(found deviations) LineReading {
	reading.MaxAbove, reading.AboveAt = found.above, found.aboveAt
	reading.MaxBelow, reading.BelowAt = found.below, found.belowAt
	reading.Counted, reading.Wide, reading.Allowed = found.counted, found.wide, allowedOver(reading.Of)
	return reading
}

// lineServed is whether the line can be read: the bars served, at least LineMinBars of them.
func lineServed(bars Bars) bool {
	return bars.Served() && bars.Len() >= LineMinBars
}

// tolerance is TolerancePercentage, never under zero.
func tolerance() float64 {
	return math.Max(float64(TolerancePercentage), 0)
}

// flat is whether the line's move stays within the bound either way (NaN misses).
func flat(movePct, bound float64) bool {
	return math.Abs(movePct) <= bound
}

// declines is whether the line moves the ladder's way: down for a long, up for an inverse; never
// for any other side (NaN misses).
func declines(movePct float64, side string) bool {
	switch side {
	case aggragates.SideLong:
		return movePct < 0
	case aggragates.SideShort:
		return movePct > 0
	}
	return false
}

// slopePerHour is the line's move per hour in percent of its start: movePct over the of bars'
// of-1 steps, each Timeframe long; NaN for a start at or under zero or a Timeframe that is no interval.
func slopePerHour(movePct float64, of int) float64 {
	minutes := helpers.IntervalToMinutes(Timeframe)
	if minutes <= 0 || of < 2 {
		return math.NaN()
	}
	return movePct / float64(of-1) * 60 / float64(minutes)
}

// withinSlope is whether the slope stays within the cap either way, the cap included; a cap at or
// under zero is off, and a NaN slope misses a cap that is on.
func withinSlope(slopePct, maxSlope float64) bool {
	return maxSlope <= 0 || math.Abs(slopePct) <= maxSlope
}

// withinBound is whether a deviation or a body stays within the bound either way, the bound included.
func withinBound(pct, bound float64) bool {
	return math.Abs(pct) <= bound
}

// allowedOver is how many of of bars may count against the tolerance: its share, rounded down.
func allowedOver(of int) int {
	return int(math.Floor(float64(of) * tolerance() / 100))
}

// LineBreaks names what TriggerLine missed, none when it holds; it formats, so not for the hot path.
func LineBreaks(bars Bars, percentage float64, side string, trend series.Trend) []string {
	return lineBreakReasons(bars, percentage, side, trend, MinSmcTrend, MaxSlopePercentage)
}

// LineBreaksOnBars is LineBreaks for TriggerLineOnBars: it never names the trend.
func LineBreaksOnBars(bars Bars, percentage float64, side string) []string {
	return lineBreakReasons(bars, percentage, side, series.Trend{}, 0, MaxSlopePercentage)
}

// lineBreakReasons is LineBreaks against need and maxSlope, nil when nothing missed, else the frame
// of served bars then the misses in the hold's order: the percentage, the bars, the line's way, the
// line's slope, the bars counted over the tolerance, the side or the trend.
func lineBreakReasons(bars Bars, percentage float64, side string, trend series.Trend, need int, maxSlope float64) []string {
	reading := LineReading{Side: side, Trend: trend, MaxSlope: maxSlope}
	var misses []string
	if !(percentage > 0) {
		misses = append(misses, percentageBreak)
	}
	reading.Bound = MaxDeviation * percentage
	if !lineServed(bars) {
		return append(misses, barsBreak)
	}
	reading = reading.measured(bars)
	found := fit(bars.Closes).read(bars, reading.Bound)
	reading = reading.deviated(found)
	if !flat(reading.MovePct, reading.Bound) && !declines(reading.MovePct, side) {
		misses = append(misses, directionBreak(reading))
	}
	if !withinSlope(reading.SlopePct, maxSlope) {
		misses = append(misses, slopeBreak(reading))
	}
	if reading.Counted > reading.Allowed {
		misses = append(misses, countedBreak(reading, found))
	}
	if !series.Sided(side) {
		misses = append(misses, sideBreak)
	} else if !trend.Meets(side, need) {
		misses = append(misses, trendBreakReason(trend, side, need))
	}
	if len(misses) == 0 {
		return nil
	}
	return append([]string{lineFrame(reading)}, misses...)
}
