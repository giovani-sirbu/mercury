package swingdecline

import (
	"os"
	"testing"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
)

// TestMain runs every test on the knobs the real-tape replays and the fixtures were measured at
// (the first settings, set A), whatever the constants say: they are hand-tuned. The derived
// values are set again after the pin, as their definitions read them, and the test
// variables built on the knobs at init (helpers_test.go, reading_test.go) are built again.
func TestMain(m *testing.M) {
	MinDepthsBetween, LookbackBars, PivotBars, MinBarsBetween = 3, 24, 4, 6
	MinDropPct, MaxBouncePct = 5.0, 3.0
	MinLegsDown, MinLegsUp = 2, 1
	MinSmcTrendDown = 2
	ArmDepth = 1 + MinDepthsBetween

	bearishTrend = series.Trend{Down: MinSmcTrendDown, Valid: dashboardRows}
	shortTrend = series.Trend{Down: MinSmcTrendDown - 1, Valid: dashboardRows}
	bearishTrendReason = trendHoldReason(bearishTrend, MinSmcTrendDown)
	shortTrendBreak = trendBreakReason(shortTrend, MinSmcTrendDown)
	legBars = max(2*PivotBars, MinBarsBetween)
	lastTurn = 8 * legBars
	os.Exit(m.Run())
}
