package swingdecline

import (
	"fmt"
	"time"
)

// barLayout is how a bar's open time is printed: the minute-accurate UTC
// clock, the zone named once after the window's second bar.
const barLayout = "2006-01-02 15:04"

// windowFrame is the frame every reason list opens with, the window the
// readings are of: the depths of the two fills it runs between, then the open
// times of the two bars that hold them (barTime) on the UTC clock, the bars
// being SwingDeclineBarMs long.
func windowFrame(reading Reading) string {
	return fmt.Sprintf("depth %d to %d, 1h bars %s to %s UTC", reading.FromDepth, reading.ToDepth, barTime(reading.StartAt), barTime(reading.EndAt))
}

// barTime is a bar's open time in ms as the UTC clock windowFrame prints.
func barTime(open int64) string {
	return time.UnixMilli(open).UTC().Format(barLayout)
}

// holdReasons are the reasons a window that holds names, in the order Reading
// names the readings: the frame, then each reading against the constant it is
// held to — the weight over SwingDeclineMinWeight, the down legs and the lower
// highs in the "of … needed" frame against SwingDeclineMinDownLegs and
// SwingDeclineMinLowerHighs, the largest leg's share of the fall under
// SwingDeclineMaxLegShare, and the net fall past SwingDeclineMinFallPct.
func holdReasons(reading Reading) []string {
	return []string{
		windowFrame(reading),
		fmt.Sprintf("weight %.2f over %.2f", reading.Weight, SwingDeclineMinWeight),
		fmt.Sprintf("%d legs down of %d needed", reading.DownLegs, SwingDeclineMinDownLegs),
		fmt.Sprintf("%d lower highs of %d needed", reading.LowerHighs, SwingDeclineMinLowerHighs),
		fmt.Sprintf("largest leg %.0f%% of the fall under %.0f%%", reading.MaxLegShare*100, SwingDeclineMaxLegShare*100),
		fmt.Sprintf("down %.1f%% past %.1f%%", -reading.NetPct, -SwingDeclineMinFallPct),
	}
}

// breakReasons are the reasons a window that does not hold names: the frame,
// then each reading that misses the constant it is held to, in the frame the
// hold reasons use, with "under", "over" and "short of" for what holds names
// with "over", "under" and "of … needed" — and "short of" for the net fall's
// "past". A net move that went up is named a rise.
func breakReasons(reading Reading) []string {
	reasons := []string{windowFrame(reading)}
	if reading.Weight < SwingDeclineMinWeight {
		reasons = append(reasons, fmt.Sprintf("weight %.2f under %.2f", reading.Weight, SwingDeclineMinWeight))
	}
	if reading.DownLegs < SwingDeclineMinDownLegs {
		reasons = append(reasons, fmt.Sprintf("%d legs down short of %d needed", reading.DownLegs, SwingDeclineMinDownLegs))
	}
	if reading.LowerHighs < SwingDeclineMinLowerHighs {
		reasons = append(reasons, fmt.Sprintf("%d lower highs short of %d needed", reading.LowerHighs, SwingDeclineMinLowerHighs))
	}
	if reading.MaxLegShare > SwingDeclineMaxLegShare {
		reasons = append(reasons, fmt.Sprintf("largest leg %.0f%% of the fall over %.0f%%", reading.MaxLegShare*100, SwingDeclineMaxLegShare*100))
	}
	if reading.NetPct > SwingDeclineMinFallPct {
		if reading.NetPct > 0 {
			reasons = append(reasons, fmt.Sprintf("up %.1f%% short of %.1f%% down", reading.NetPct, -SwingDeclineMinFallPct))
		} else {
			reasons = append(reasons, fmt.Sprintf("down %.1f%% short of %.1f%%", -reading.NetPct, -SwingDeclineMinFallPct))
		}
	}
	return reasons
}

// trendHoldReason is the reason a trend that meets its need names, the last of a
// holding reading's reasons: the rows whose trend reads down out of the rows
// read, against the least the need asks for (SwingDeclineMinSmcTrendDown, at
// Trigger), in the "of … needed" frame of the hold reasons.
func trendHoldReason(trend Trend, need int) string {
	return fmt.Sprintf("smc trend direction down %d/%d of %d needed", trend.Down, trend.Valid, need)
}

// trendUnreadBreak is the reason a trend sophos did not serve names: no row was
// read, so there is no count to hold against the need.
const trendUnreadBreak = "smc trend direction not read"

// trendBreakReason is the reason a trend that misses its need names, in the
// frame trendHoldReason uses with "short of" for its "of … needed", or
// trendUnreadBreak for a trend that was not read.
func trendBreakReason(trend Trend, need int) string {
	if !trend.Served() {
		return trendUnreadBreak
	}
	return fmt.Sprintf("smc trend direction down %d/%d short of %d needed", trend.Down, trend.Valid, need)
}

// fewFillsBreak is the reason a ladder too shallow to have a window names.
func fewFillsBreak(fills int) string {
	return fmt.Sprintf("%d fills short of %d needed", fills, SwingDeclineMinDepthsBetween+1)
}

// unstampedBreak is the reason a ladder whose newest fill carries no stamp
// names: with no bar to end at, no window is read.
func unstampedBreak(fills int) string {
	return fmt.Sprintf("depth %d, the newest fill carries no stamp", fills)
}

// unreadableBreak is the reason a ladder none of whose windows could be read
// names: no bar served at all, or the bars its fills fall in are not among
// the ones served.
func unreadableBreak(series Series, fills int) string {
	if !series.Served() {
		return fmt.Sprintf("depth %d, no 1h bars served", fills)
	}
	return fmt.Sprintf("depth %d, no window between the fills is readable in the 1h bars served", fills)
}
