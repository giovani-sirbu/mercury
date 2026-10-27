package flatmarket

import (
	"math"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
)

// RangeReading is what the range reads for a ladder of one side: the window (From, To, Of), the
// quiet bodies, the move, the stray from the mean of the first and last closes, the close range,
// the Side and Trend, Holds and the Reasons a holding reading names; a miss carries no text (RangeBreaks).
type RangeReading struct {
	From, To            int64
	Quiet, Of           int
	BodyMaxPct, MovePct float64
	Mean, StrayPct      float64
	StrayAt             int64
	CloseLow, CloseHigh float64
	CloseRangePct       float64
	Side                string
	Trend               series.Trend
	Holds               bool
	Reasons             []string
}

// TriggerRange is the range reading of the bars and the SMC trend for a ladder of that side.
func TriggerRange(bars Bars, percentage float64, side string, trend series.Trend) RangeReading {
	return rangeAt(bars, percentage, side, trend, MinSmcTrend)
}

// TriggerRangeOnBars is TriggerRange on the bars alone: no trend is read and no reason names it.
func TriggerRangeOnBars(bars Bars, percentage float64, side string) RangeReading {
	return rangeAt(bars, percentage, side, series.Trend{}, 0)
}

// rangeAt is TriggerRange against need, the least rows of the trend that must read the ladder's
// way; at zero or below no trend is read.
func rangeAt(bars Bars, percentage float64, side string, trend series.Trend, need int) RangeReading {
	reading := RangeReading{Side: side, Trend: trend}
	if !(percentage > 0) {
		return reading
	}
	reading.BodyMaxPct = BodyShare * percentage
	if !trend.Meets(side, need) {
		return reading
	}
	if !bars.Wicked() {
		return reading
	}
	last := bars.Len() - 1
	reading.From, reading.To, reading.Of = bars.Opens[0], bars.Opens[last], last+1
	reading.Quiet = bars.quietBodies(reading.BodyMaxPct)
	if !quietEnough(reading.Quiet, reading.Of) {
		return reading
	}
	reading.MovePct = bars.movePct(side)
	if !moveBounded(reading.MovePct) {
		return reading
	}
	reading.Mean, reading.StrayPct, reading.StrayAt = bars.stray()
	if !strayBounded(reading.StrayPct) {
		return reading
	}
	reading.CloseLow, reading.CloseHigh, reading.CloseRangePct = bars.closeRange()
	if !closeRangeBounded(reading.CloseRangePct) {
		return reading
	}
	reading.Holds = true
	reading.Reasons = rangeHoldReasons(reading, need)
	return reading
}

// quietEnough is whether quiet of of bars is strictly more than QuietShare, as a division so the
// bound is exact on a whole number of bars.
func quietEnough(quiet, of int) bool {
	return of > 0 && float64(quiet)/float64(of) > QuietShare
}

// moveBounded is whether a move is strictly under MaxMovePct (NaN misses, a negative move holds).
func moveBounded(movePct float64) bool {
	return movePct < MaxMovePct
}

// strayBounded is whether a stray is within MaxMovePct of the mean either way, the bound included.
func strayBounded(strayPct float64) bool {
	return math.Abs(strayPct) <= MaxMovePct
}

// closeRangeBounded is whether the close range is within MaxCloseRangePct, the bound included, or
// the condition is off (zero or below).
func closeRangeBounded(rangePct float64) bool {
	return MaxCloseRangePct <= 0 || rangePct <= float64(MaxCloseRangePct)
}

// RangeBreaks names what TriggerRange missed, none when it holds; it formats, so not for the hot path.
func RangeBreaks(bars Bars, percentage float64, side string, trend series.Trend) []string {
	return rangeBreakReasons(bars, percentage, side, trend, MinSmcTrend)
}

// RangeBreaksOnBars is RangeBreaks for TriggerRangeOnBars: it never names the trend.
func RangeBreaksOnBars(bars Bars, percentage float64, side string) []string {
	return rangeBreakReasons(bars, percentage, side, series.Trend{}, 0)
}

// rangeBreakReasons is RangeBreaks against need, in the hold's order: the percentage or the quiet
// bodies, the bars or the move, the stray, the close range, the side or the trend.
func rangeBreakReasons(bars Bars, percentage float64, side string, trend series.Trend, need int) []string {
	reading := RangeReading{Side: side, Trend: trend}
	served := bars.Wicked()
	if served {
		reading.From, reading.To, reading.Of = bars.Opens[0], bars.Opens[bars.Len()-1], bars.Len()
	}
	sideKnown := series.Sided(side)

	var misses []string
	switch {
	case !(percentage > 0):
		misses = append(misses, percentageBreak)
	case served:
		reading.BodyMaxPct = BodyShare * percentage
		reading.Quiet = bars.quietBodies(reading.BodyMaxPct)
		if !quietEnough(reading.Quiet, reading.Of) {
			misses = append(misses, quietBreak(reading))
		}
	}
	switch {
	case !served:
		misses = append(misses, barsBreak)
	case sideKnown:
		reading.MovePct = bars.movePct(side)
		if !moveBounded(reading.MovePct) {
			misses = append(misses, moveBreak(reading))
		}
	}
	if served {
		reading.Mean, reading.StrayPct, reading.StrayAt = bars.stray()
		if !strayBounded(reading.StrayPct) {
			misses = append(misses, strayBreak(reading))
		}
		reading.CloseLow, reading.CloseHigh, reading.CloseRangePct = bars.closeRange()
		if !closeRangeBounded(reading.CloseRangePct) {
			misses = append(misses, closeRangeBreak(reading))
		}
	}
	switch {
	case !sideKnown:
		misses = append(misses, sideBreak)
	case !trend.Meets(side, need):
		misses = append(misses, trendBreakReason(trend, side, need))
	}

	if len(misses) == 0 {
		return nil
	}
	if served {
		return append([]string{rangeFrame(reading)}, misses...)
	}
	return misses
}
