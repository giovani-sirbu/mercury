package flatmarket

import "github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"

// Reading is what the gate reads for a ladder of one side: the Range and the Line readings, Holds
// when either does, and the Reasons of the one that holds, the range first.
type Reading struct {
	Range   RangeReading
	Line    LineReading
	Holds   bool
	Reasons []string
}

// Trigger is the gate's reading of the bars and the SMC trend for a ladder of that side: the range
// on the newest RangeBars of the bars, then the line on all of them; the first that holds names the reading.
func Trigger(bars Bars, percentage float64, side string, trend series.Trend) Reading {
	return readings(TriggerRange(bars.Tail(RangeBars), percentage, side, trend), TriggerLine(bars, percentage, side, trend))
}

// LatchTrigger is the latch's reading: the range on its bars alone, no trend asked, or the line
// under the trend.
func LatchTrigger(bars Bars, percentage float64, side string, trend series.Trend) Reading {
	return readings(TriggerRangeOnBars(bars.Tail(RangeBars), percentage, side), TriggerLine(bars, percentage, side, trend))
}

// TrendMeets is the trend condition alone, at MinSmcTrend, which both readings ask before any bar.
func TrendMeets(trend series.Trend, side string) bool {
	return trend.Meets(side, MinSmcTrend)
}

// readings folds the two readings into the gate's, the range first.
func readings(ranged RangeReading, lined LineReading) Reading {
	reading := Reading{Range: ranged, Line: lined}
	switch {
	case ranged.Holds:
		reading.Holds, reading.Reasons = true, ranged.Reasons
	case lined.Holds:
		reading.Holds, reading.Reasons = true, lined.Reasons
	}
	return reading
}

// Breaks names what Trigger missed, the range's misses then the line's, none when it holds; it
// formats, so not for the hot path.
func Breaks(bars Bars, percentage float64, side string, trend series.Trend) []string {
	if Trigger(bars, percentage, side, trend).Holds {
		return nil
	}
	return append(RangeBreaks(bars.Tail(RangeBars), percentage, side, trend), LineBreaks(bars, percentage, side, trend)...)
}
