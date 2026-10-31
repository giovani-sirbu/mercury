package swingdecline

import (
	"fmt"
	"strconv"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
)

// barLayout is how a bar's open time prints: the minute-accurate UTC clock.
const barLayout = "2006-01-02 15:04"

// windowFrame opens every reason list with the window: the two depths and their bars' open times.
func windowFrame(reading Reading) string {
	return fmt.Sprintf("depth %d to %d, 1h bars %s to %s UTC", reading.FromDepth, reading.ToDepth, barTime(reading.StartAt), barTime(reading.EndAt))
}

// barTime is a bar's open time in ms on the UTC clock.
func barTime(open int64) string {
	return time.UnixMilli(open).UTC().Format(barLayout)
}

// price prints a wick as the exchange quotes it, with no trailing zeros.
func price(wick float64) string {
	return strconv.FormatFloat(wick, 'f', -1, 64)
}

// swingFrame names the swing high the window fell from and the lowest low since, with the fall
// between them.
func swingFrame(reading Reading) string {
	return fmt.Sprintf("high %s at %s UTC, low %s at %s UTC, down %.1f%%", price(reading.High), barTime(reading.HighAt), price(reading.Low), barTime(reading.LowAt), (1-reading.Low/reading.High)*100)
}

// downLegsReason names the down legs that count against MinLegsDown, with what a leg needs.
func downLegsReason(reading Reading, short bool) string {
	return fmt.Sprintf("%d legs down %s %d needed, each %d bars and %.1f%% down at least", reading.DownLegs, ofOrShortOf(short), MinLegsDown, MinBarsBetween, MinDropPct)
}

// upLegsReason names the up legs that count against MinLegsUp, with what a leg needs, and the
// bounces over it when there are any.
func upLegsReason(reading Reading, short bool) string {
	text := fmt.Sprintf("%d legs up %s %d needed, each %.1f%% up at most", reading.UpLegs, ofOrShortOf(short), MinLegsUp, MaxBouncePct)
	if reading.Bounces > 0 {
		text += fmt.Sprintf(", %d over it", reading.Bounces)
	}
	return text
}

// ofOrShortOf is the grammar of a count that meets its need or falls short of it.
func ofOrShortOf(short bool) string {
	if short {
		return "short of"
	}
	return "of"
}

// holdReasons are the reasons a holding window names: the frame, the swings, then the legs.
func holdReasons(reading Reading) []string {
	return []string{windowFrame(reading), swingFrame(reading), downLegsReason(reading, false), upLegsReason(reading, false)}
}

// breakReasons are the reasons a missing window names: the frame, the swings, then each count that
// falls short.
func breakReasons(reading Reading) []string {
	reasons := []string{windowFrame(reading), swingFrame(reading)}
	if reading.DownLegs < MinLegsDown {
		reasons = append(reasons, downLegsReason(reading, true))
	}
	if reading.UpLegs < MinLegsUp {
		reasons = append(reasons, upLegsReason(reading, true))
	}
	return reasons
}

// trendHoldReason names a trend that meets its need: the rows reading down of those read.
func trendHoldReason(trend series.Trend, need int) string {
	return fmt.Sprintf("smc trend direction down %d/%d of %d needed", trend.Down, trend.Valid, need)
}

// trendUnreadBreak names a trend sophos did not serve.
const trendUnreadBreak = "smc trend direction not read"

// trendBreakReason names a trend that misses its need, or trendUnreadBreak when not served.
func trendBreakReason(trend series.Trend, need int) string {
	if !trend.Served() {
		return trendUnreadBreak
	}
	return fmt.Sprintf("smc trend direction down %d/%d short of %d needed", trend.Down, trend.Valid, need)
}

// fewFillsBreak names a ladder too shallow to have a window.
func fewFillsBreak(fills int) string {
	return fmt.Sprintf("%d fills short of %d needed", fills, MinDepthsBetween+1)
}

// unstampedBreak names a ladder whose newest fill carries no stamp.
func unstampedBreak(fills int) string {
	return fmt.Sprintf("depth %d, the newest fill carries no stamp", fills)
}

// unreadableBreak names a ladder none of whose windows could be read: no bars, no wicks, or no
// window between the fills in the bars.
func unreadableBreak(bars series.Series, fills int) string {
	switch {
	case !bars.Served():
		return fmt.Sprintf("depth %d, no 1h bars served", fills)
	case !bars.WicksServed():
		return fmt.Sprintf("depth %d, no 1h wicks served", fills)
	}
	return fmt.Sprintf("depth %d, no window between the fills is readable in the 1h bars served", fills)
}
