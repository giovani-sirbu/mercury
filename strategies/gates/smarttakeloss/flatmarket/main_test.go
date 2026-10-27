package flatmarket

import (
	"os"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestMain runs every test on the knobs the fixtures and the line replays were written at, whatever
// the constants say: they are hand-tuned. The line is read over 48 bars at a tolerance of 10 and a
// slope of at most 0.25 an hour (the replays' steepest is 0.21); the range at the first settings,
// under which the real-tape range replays (rangeMeasuredAt) skip: they were measured with the
// close range off, which the close range tests cannot run under. Window is set again after the pin,
// as its definition reads it, and so are the test variables built on the knobs at init.
func TestMain(m *testing.M) {
	ArmDepth, MinSmcTrend = 1, 3
	Timeframe, WindowTimeframe = "1h", 48*time.Hour
	RangeBars = 24
	BodyShare, QuietShare, MaxMovePct, MaxCloseRangePct = 0.75, 0.9, 2.0, 3.0
	MaxDeviation, MaxSlopePercentage, TolerancePercentage = 0.75, 0.25, 10
	Window = int(WindowTimeframe / (time.Duration(helpers.IntervalToMinutes(Timeframe)) * time.Minute))

	fixtureBound = MaxDeviation * lineFixturePercentage
	lineFixtureBars = max(Window, MinBars)
	for index, constant := range lineMeasuredAt {
		lineMeasuredAt[index].inForce = inForce(constant.name)
	}
	for index, constant := range rangeMeasuredAt {
		rangeMeasuredAt[index].inForce = inForce(constant.name)
	}
	holdMove, overMove, holdRange, overRange = 0.5*MaxMovePct, 1.5*MaxMovePct, 0.5*MaxCloseRangePct, 1.5*MaxCloseRangePct
	holdStray, overStray = 0.5*MaxMovePct, 1.5*MaxMovePct

	lineTrendRows = max(lineDashboardRows, MinSmcTrend) + 1
	lineSideCases[0].met, lineSideCases[0].short, lineSideCases[0].wrong = sidedTrends(aggragates.SideLong, lineTrendRows)
	lineSideCases[1].met, lineSideCases[1].short, lineSideCases[1].wrong = sidedTrends(aggragates.SideShort, lineTrendRows)
	trendRows = max(rangeDashboardRows, MinSmcTrend) + 1
	rangeSideCases[0].met, rangeSideCases[0].short, rangeSideCases[0].wrong = sidedTrends(aggragates.SideLong, trendRows)
	rangeSideCases[1].met, rangeSideCases[1].short, rangeSideCases[1].wrong = sidedTrends(aggragates.SideShort, trendRows)
	os.Exit(m.Run())
}

// inForce is the value of a constant the replays name in lineMeasuredAt and rangeMeasuredAt.
func inForce(name string) float64 {
	switch name {
	case "TolerancePercentage":
		return TolerancePercentage
	case "Window":
		return float64(Window)
	case "BodyShare":
		return BodyShare
	case "QuietShare":
		return QuietShare
	case "MaxMovePct":
		return MaxMovePct
	}
	panic("TestMain: the replays name " + name + ", which has no value here")
}

// sidedTrends are the trend that just meets MinSmcTrend on the side, one a row short of it and
// one that meets it the other way, over rows rows read.
func sidedTrends(side string, rows int) (met, short, wrong series.Trend) {
	if side == aggragates.SideShort {
		return series.Trend{Up: MinSmcTrend, Valid: rows}, series.Trend{Up: MinSmcTrend - 1, Valid: rows}, series.Trend{Down: MinSmcTrend, Valid: rows}
	}
	return series.Trend{Down: MinSmcTrend, Valid: rows}, series.Trend{Down: MinSmcTrend - 1, Valid: rows}, series.Trend{Up: MinSmcTrend, Valid: rows}
}
