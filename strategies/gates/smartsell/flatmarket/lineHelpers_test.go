package flatmarket

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"math"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/helpers"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// lineFirstBar is the open time of the first bar of every synthetic window.
var lineFirstBar = time.Date(2026, time.June, 1, 3, 0, 0, 0, time.UTC)

const (
	// lineFixturePercentage is the percentage of the ladder the windows are read for; lineBasePrice the
	// line's start.
	lineFixturePercentage = 2.2
	lineBasePrice         = 100.0
	// lineDashboardRows is how many rows the SMC trend dashboard reads its Trend Direction row on.
	lineDashboardRows = 3
)

// fixtureBound is the bound the windows make with MaxDeviation at lineFixturePercentage.
var fixtureBound = MaxDeviation * lineFixturePercentage

// lineFixtureBars is how many bars a synthetic window holds: the look-back sophos serves, never
// fewer than the rule reads.
var lineFixtureBars = max(Window, MinBars)

// lineSideCase is one side of the gate with the trends and the words that belong to it: met a trend
// that just meets MinSmcTrend on the side, short one a row short of it and wrong one that meets it
// the other way.
type lineSideCase struct {
	name, side, trendWord string
	met, short, wrong     series.Trend
}

var (
	lineTrendRows = max(lineDashboardRows, MinSmcTrend) + 1
	lineSideCases = []lineSideCase{
		{
			name: "long", side: aggragates.SideLong, trendWord: "down",
			met:   series.Trend{Down: MinSmcTrend, Valid: lineTrendRows},
			short: series.Trend{Down: MinSmcTrend - 1, Valid: lineTrendRows},
			wrong: series.Trend{Up: MinSmcTrend, Valid: lineTrendRows},
		},
		{
			name: "inverse", side: aggragates.SideShort, trendWord: "up",
			met:   series.Trend{Up: MinSmcTrend, Valid: lineTrendRows},
			short: series.Trend{Up: MinSmcTrend - 1, Valid: lineTrendRows},
			wrong: series.Trend{Down: MinSmcTrend, Valid: lineTrendRows},
		},
	}
)

// ownWay is a move of that size the ladder's way: down for a long, up for an inverse.
func (sc lineSideCase) ownWay(size float64) float64 {
	if sc.side == aggragates.SideShort {
		return size
	}
	return -size
}

// lineBars is a window of n bars one hour apart whose opens and closes lie exactly on a line from
// lineBasePrice moving movePct over the window; closeOffsets and openOffsets, by index, push a close
// or an open off the line by that percent of it.
func lineBars(n int, movePct float64, closeOffsets, openOffsets map[int]float64) Bars {
	var bars Bars
	for index := range n {
		lineValue := lineBasePrice * (1 + movePct/100*float64(index)/float64(n-1))
		bars.Opens = append(bars.Opens, lineFirstBar.Add(time.Duration(index)*time.Hour).UnixMilli())
		bars.OpenPrices = append(bars.OpenPrices, lineValue*(1+openOffsets[index]/100))
		bars.Closes = append(bars.Closes, lineValue*(1+closeOffsets[index]/100))
	}
	return bars
}

// flatBars is a window of lineFixtureBars bars on a flat line.
func flatLineBars() Bars {
	return lineBars(lineFixtureBars, 0, nil, nil)
}

// slopedMove is the move over the window the diagonal tests read with: a tenth of the price, or
// under the slope cap when one is on (MaxSlopePercentage per hour over the window's hours).
func slopedMove() float64 {
	move := 10.0
	if MaxSlopePercentage > 0 {
		hours := float64(lineFixtureBars-1) * float64(helpers.IntervalToMinutes(Timeframe)) / 60
		move = math.Min(move, 0.9*MaxSlopePercentage*hours)
	}
	return move
}

// needsATolerance skips a test that reads bars past the bound within the tolerance when the
// tolerance in force lets none through.
func needsATolerance(t *testing.T) {
	t.Helper()
	if tolerance() <= 0 || allowedOver(lineFixtureBars) < 1 {
		t.Skipf("TolerancePercentage %v lets no bar past the bound over %d bars", TolerancePercentage, lineFixtureBars)
	}
}

// needsATrend skips a test that reads the trend condition when MinSmcTrend switches it off.
func needsATrend(t *testing.T) {
	t.Helper()
	if MinSmcTrend <= 0 {
		t.Skipf("MinSmcTrend %v reads no trend", MinSmcTrend)
	}
}

// strayOffsets is the offsets of count bars pushed past the bound by factor of it, placed around
// the middle of the window with alternating signs so the line itself hardly moves.
func strayOffsets(count int, factor float64) map[int]float64 {
	offsets := make(map[int]float64, count)
	middle := lineFixtureBars / 2
	for index := range count {
		sign := 1.0
		if index%2 == 1 {
			sign = -1
		}
		offsets[middle-count+2*index] = sign * factor * fixtureBound
	}
	return offsets
}
