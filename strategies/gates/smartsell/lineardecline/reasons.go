package lineardecline

import (
	"fmt"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
)

// barLayout is how a bar's open time prints: the minute-accurate UTC clock.
const barLayout = "2006-01-02 15:04"

// frame opens every reason list with the line: the first and newest depths and their bars' open times.
func frame(reading Reading) string {
	return fmt.Sprintf(
		"depth %d to %d, 1h bars %s to %s UTC",
		reading.FromDepth, reading.ToDepth, barTime(reading.StartAt), barTime(reading.EndAt),
	)
}

// barTime is a bar's open time in ms on the UTC clock.
func barTime(open int64) string {
	return time.UnixMilli(open).UTC().Format(barLayout)
}

// holdReasons are the reasons a holding line names, the only text Trigger formats: the frame, the
// slopes, the larger deviation, the net fall and the trend (unless off).
func holdReasons(reading Reading, trend series.Trend, need int) []string {
	reasons := []string{
		frame(reading),
		fmt.Sprintf(
			"slope %.2f%%/h against %.2f%%/h at depth %d, within %.0f%%",
			reading.Slope, reading.PreviousSlope, reading.ToDepth-1, MaxSlopeChange*100,
		),
		fmt.Sprintf(
			"closes within %.1f%% of the line, at most %.1f%%",
			max(reading.Deviation, reading.PreviousDeviation), MaxDeviationPct,
		),
		fmt.Sprintf("down %.1f%% past %.1f%%", -reading.NetPct, -MinFallPct),
	}
	if need > 0 {
		reasons = append(reasons, trendHoldReason(trend, need))
	}
	return reasons
}

// breakReasons are the reasons a ladder that does not hold names: too few fills, the unread cause
// (unreadBreaks), or the frame and each missed reading (shapeBreaks); the trend last.
func breakReasons(reading Reading, missed misses, trend series.Trend, need int) []string {
	trendMissed := !meets(trend, need)
	if missed == 0 && !trendMissed {
		return nil
	}
	var reasons []string
	switch {
	case missed&missFills != 0:
		reasons = []string{fewFillsBreak(reading.ToDepth)}
	case missed&missUnread != 0:
		reasons = unreadBreaks(missed)
	default:
		reasons = append([]string{frame(reading)}, shapeBreaks(reading, missed)...)
	}
	if trendMissed {
		reasons = append(reasons, trendBreakReason(trend, need))
	}
	return reasons
}

// shapeBreaks names each reading that misses its constant, the chords told apart by their end depth.
func shapeBreaks(reading Reading, missed misses) []string {
	var reasons []string
	depth, before := reading.ToDepth, reading.ToDepth-1
	if missed&missDeviation != 0 {
		reasons = append(reasons, deviationBreak(reading.Deviation, depth))
	}
	if missed&missPreviousDeviation != 0 {
		reasons = append(reasons, deviationBreak(reading.PreviousDeviation, before))
	}
	if missed&missRising != 0 {
		reasons = append(reasons, risingBreak(reading.Slope, depth))
	}
	if missed&missPreviousRising != 0 {
		reasons = append(reasons, risingBreak(reading.PreviousSlope, before))
	}
	if missed&missSlopeChange != 0 {
		reasons = append(reasons, slopeBreak(reading))
	}
	if missed&missFall != 0 {
		reasons = append(reasons, fallBreak(reading.NetPct))
	}
	return reasons
}

// deviationBreak names a deviation over MaxDeviationPct on the chord ending at depth.
func deviationBreak(deviation float64, depth int) string {
	return fmt.Sprintf("closes %.1f%% from the line at depth %d over %.1f%%", deviation, depth, MaxDeviationPct)
}

// risingBreak names a chord that does not fall, at the depth it ends.
func risingBreak(slope float64, depth int) string {
	return fmt.Sprintf("slope %.2f%%/h rising at depth %d", slope, depth)
}

// slopeBreak names a change of slope over MaxSlopeChange.
func slopeBreak(reading Reading) string {
	return fmt.Sprintf(
		"slope %.2f%%/h against %.2f%%/h at depth %d, changed %.0f%% over %.0f%%",
		reading.Slope, reading.PreviousSlope, reading.ToDepth-1, slopeChange(reading)*100, MaxSlopeChange*100,
	)
}

// fallBreak names a net move short of MinFallPct, an upward one as up.
func fallBreak(netPct float64) string {
	if netPct > 0 {
		return fmt.Sprintf("up %.1f%% short of %.1f%% down", netPct, -MinFallPct)
	}
	return fmt.Sprintf("down %.1f%% short of %.1f%%", -netPct, -MinFallPct)
}

// fewFillsBreak names a ladder too shallow to be read.
func fewFillsBreak(fills int) string {
	return fmt.Sprintf("%d fills short of %d needed", fills, Window)
}

// The reasons of the causes of missUnread.
const (
	stampsBreak = "fill stamps not read: one is missing, or a later fill is not after the first"
	pricesBreak = "fill prices not read: one is at or under zero"
	barsBreak   = "1h bars not read: none served, or one that holds a fill is not among them"
	closesBreak = "closes not read: one is at or under zero"
)

// unreadBreaks names the causes in missed, in a fixed order.
func unreadBreaks(missed misses) []string {
	var reasons []string
	for _, cause := range []struct {
		bit  misses
		text string
	}{
		{missStamps, stampsBreak},
		{missPrices, pricesBreak},
		{missBars, barsBreak},
		{missCloses, closesBreak},
	} {
		if missed&cause.bit != 0 {
			reasons = append(reasons, cause.text)
		}
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
