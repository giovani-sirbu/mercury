package flatmarket

import (
	"fmt"
	"math"
	"strconv"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// rangeHoldReasonCap is the most reasons a holding range reading names.
const rangeHoldReasonCap = 6

// rangeFrame opens the range's reason list (frame).
func rangeFrame(reading RangeReading) string {
	return frame("range", reading.From, reading.To)
}

// moveWord is the move a side reads: a fall for a long ladder, a rise for an inverse one.
func moveWord(side string) string {
	if side == aggragates.SideShort {
		return "rise"
	}
	return "fall"
}

// strayWord is where a stray lies against the mean.
func strayWord(strayPct float64) string {
	if strayPct < 0 {
		return "below"
	}
	return "above"
}

// meanText is a price as a reason prints it: rounded to the eighth decimal, shortest form.
func meanText(mean float64) string {
	return strconv.FormatFloat(math.Round(mean*1e8)/1e8, 'f', -1, 64)
}

// rangeHoldReasons are the reasons a holding range names, the only text TriggerRange formats: the
// frame, the quiet bodies, the move, the stray, the close range (unless off) and the trend (unless off).
func rangeHoldReasons(reading RangeReading, need int) []string {
	reasons := make([]string, 0, rangeHoldReasonCap)
	reasons = append(
		reasons,
		rangeFrame(reading),
		fmt.Sprintf(
			"quiet bodies %d/%d under %.2f%%, over %.0f%%",
			reading.Quiet, reading.Of, reading.BodyMaxPct, QuietShare*100,
		),
		fmt.Sprintf("%s %.1f%% under %.1f%%", moveWord(reading.Side), reading.MovePct, MaxMovePct),
		fmt.Sprintf(
			"closes within %.1f%% of the mean %s, at most %.1f%%",
			math.Abs(reading.StrayPct), meanText(reading.Mean), MaxMovePct,
		),
	)
	if MaxCloseRangePct > 0 {
		reasons = append(reasons, closeRangeHoldReason(reading))
	}
	if need > 0 {
		reasons = append(reasons, trendHoldReason(reading.Trend, reading.Side, need))
	}
	return reasons
}

// closeRangeHoldReason names a close range within MaxCloseRangePct.
func closeRangeHoldReason(reading RangeReading) string {
	return fmt.Sprintf(
		"closes span %.1f%% from %s to %s, at most %.1f%%",
		reading.CloseRangePct, meanText(reading.CloseLow), meanText(reading.CloseHigh),
		MaxCloseRangePct,
	)
}

// quietBreak names bodies not quiet enough, "short of" the share.
func quietBreak(reading RangeReading) string {
	return fmt.Sprintf(
		"quiet bodies %d/%d under %.2f%%, short of %.0f%%",
		reading.Quiet, reading.Of, reading.BodyMaxPct, QuietShare*100,
	)
}

// moveBreak names a move not under MaxMovePct.
func moveBreak(reading RangeReading) string {
	return fmt.Sprintf("%s %.1f%% over %.1f%%", moveWord(reading.Side), reading.MovePct, MaxMovePct)
}

// strayBreak names a stray not within MaxMovePct: the bar, how far and on which side of the mean.
func strayBreak(reading RangeReading) string {
	return fmt.Sprintf(
		"close at %s strays %.1f%% %s the mean %s over %.1f%%",
		barTime(reading.StrayAt), math.Abs(reading.StrayPct), strayWord(reading.StrayPct),
		meanText(reading.Mean), MaxMovePct,
	)
}

// closeRangeBreak names a close range not within MaxCloseRangePct.
func closeRangeBreak(reading RangeReading) string {
	return fmt.Sprintf(
		"closes span %.1f%% from %s to %s over %.1f%%",
		reading.CloseRangePct, meanText(reading.CloseLow), meanText(reading.CloseHigh),
		MaxCloseRangePct,
	)
}
