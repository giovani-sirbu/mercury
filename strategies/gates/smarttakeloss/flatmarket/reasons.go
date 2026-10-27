package flatmarket

import (
	"fmt"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// barLayout is how a bar's open time prints: the minute-accurate UTC clock.
const barLayout = "2006-01-02 15:04"

// frame opens a reading's reason list with its name and window: the oldest and newest bars' open times.
func frame(reading string, from, to int64) string {
	return fmt.Sprintf("%s on %s bars %s to %s UTC", reading, Timeframe, barTime(from), barTime(to))
}

// barTime is a bar's open time in ms on the UTC clock.
func barTime(open int64) string {
	return time.UnixMilli(open).UTC().Format(barLayout)
}

// trendWord is the trend direction a side needs: down for a long ladder, up for an inverse one.
func trendWord(side string) string {
	if side == aggragates.SideShort {
		return "up"
	}
	return "down"
}

// trendHoldReason names a trend that meets its need: the rows reading the ladder's way of those read.
func trendHoldReason(trend series.Trend, side string, need int) string {
	return fmt.Sprintf("smc trend direction %s %d/%d of %d needed", trendWord(side), trend.Rows(side), trend.Valid, need)
}

// The reasons of a reading that could not be made.
const (
	barsBreak        = "bars not served"
	percentageBreak  = "percentage not set"
	sideBreak        = "side not read"
	trendUnreadBreak = "smc trend direction not read"
)

// trendBreakReason names a trend that misses its need, or trendUnreadBreak when not served.
func trendBreakReason(trend series.Trend, side string, need int) string {
	if !trend.Served() {
		return trendUnreadBreak
	}
	return fmt.Sprintf("smc trend direction %s %d/%d short of %d needed", trendWord(side), trend.Rows(side), trend.Valid, need)
}
