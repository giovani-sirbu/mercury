package slowpattern

import (
	"fmt"
	"time"
)

// barLayout is how a bar's open time is printed: the minute-accurate UTC
// clock, the zone named once after the window's second bar.
const barLayout = "2006-01-02 15:04"

// windowFrame is the frame every reason list opens with, the window the
// readings are of: "depth 1 to 4, 1h bars 2021-10-08 14:00 to 2021-10-10 00:00
// UTC". The bars are the ones the two fills fall in.
func windowFrame(reading Reading) string {
	return fmt.Sprintf("depth %d to %d, 1h bars %s to %s UTC", reading.FromDepth, reading.ToDepth, barTime(reading.StartAt), barTime(reading.EndAt))
}

// barTime is a bar's open time in ms as the UTC clock windowFrame prints.
func barTime(open int64) string {
	return time.UnixMilli(open).UTC().Format(barLayout)
}

// holdReasons are the reasons a window that holds names, in the order Reading
// names the readings: the frame, then each reading against its constant —
// "weight 0.50 over 0.33", "4 legs down of 3 needed", "3 lower highs of 2
// needed", "largest leg 51% of the fall under 60%", "down 7.7% past 4.0%".
func holdReasons(reading Reading) []string {
	return []string{
		windowFrame(reading),
		fmt.Sprintf("weight %.2f over %.2f", reading.Weight, SlowPatternMinWeight),
		fmt.Sprintf("%d legs down of %d needed", reading.DownLegs, SlowPatternMinDownLegs),
		fmt.Sprintf("%d lower highs of %d needed", reading.LowerHighs, SlowPatternMinLowerHighs),
		fmt.Sprintf("largest leg %.0f%% of the fall under %.0f%%", reading.MaxLegShare*100, SlowPatternMaxLegShare*100),
		fmt.Sprintf("down %.1f%% past %.1f%%", -reading.NetPct, -SlowPatternMinFallPct),
	}
}

// breakReasons are the reasons a window that does not hold names: the frame,
// then each reading that misses its constant in the frame the hold reasons
// use, with "under", "over" and "short of" for what holds names with "over",
// "under" and "of … needed" — "weight 0.21 under 0.33", "2 legs down short of
// 3 needed", "1 lower highs short of 2 needed", "largest leg 71% of the fall
// over 60%", "down 2.1% short of 4.0%". A rise is named a rise.
func breakReasons(reading Reading) []string {
	reasons := []string{windowFrame(reading)}
	if reading.Weight < SlowPatternMinWeight {
		reasons = append(reasons, fmt.Sprintf("weight %.2f under %.2f", reading.Weight, SlowPatternMinWeight))
	}
	if reading.DownLegs < SlowPatternMinDownLegs {
		reasons = append(reasons, fmt.Sprintf("%d legs down short of %d needed", reading.DownLegs, SlowPatternMinDownLegs))
	}
	if reading.LowerHighs < SlowPatternMinLowerHighs {
		reasons = append(reasons, fmt.Sprintf("%d lower highs short of %d needed", reading.LowerHighs, SlowPatternMinLowerHighs))
	}
	if reading.MaxLegShare > SlowPatternMaxLegShare {
		reasons = append(reasons, fmt.Sprintf("largest leg %.0f%% of the fall over %.0f%%", reading.MaxLegShare*100, SlowPatternMaxLegShare*100))
	}
	if reading.NetPct > SlowPatternMinFallPct {
		if reading.NetPct > 0 {
			reasons = append(reasons, fmt.Sprintf("up %.1f%% short of %.1f%% down", reading.NetPct, -SlowPatternMinFallPct))
		} else {
			reasons = append(reasons, fmt.Sprintf("down %.1f%% short of %.1f%%", -reading.NetPct, -SlowPatternMinFallPct))
		}
	}
	return reasons
}

// fewFillsBreak is the reason a ladder too shallow to have a window names.
func fewFillsBreak(fills int) string {
	return fmt.Sprintf("%d fills short of %d needed", fills, SlowPatternMinDepthsBetween+1)
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
