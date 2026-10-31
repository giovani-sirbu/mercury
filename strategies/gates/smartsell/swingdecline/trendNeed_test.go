package swingdecline

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"testing"
)

// MeetsTheNeed is the condition withTrend folds in, asked alone: at any need a
// trend meets it exactly when no condition is asked (zero or below, where every
// trend does, one not served too) or sophos read a row and the rows down reach
// the need, the bound included — and a shape that holds keeps its hold under the
// fold exactly then. The exported predicate is that at
// MinSmcTrendDown. A caller may therefore skip the windows for a trend
// that misses it and answer as the fold would. The shape is made by hand, so the
// test holds under any calibration of the rule.
func TestMeetsTheNeedIsExactlyWhenAHoldingShapeKeepsItsHold(t *testing.T) {
	shape := Reading{FromDepth: 1, ToDepth: 3, StartAt: barOpenAt(0), EndAt: barOpenAt(8), Holds: true, Reasons: []string{"a window that holds"}}
	for need := -1; need <= 4; need++ {
		for down := 0; down <= 3; down++ {
			for _, valid := range []int{0, down, 3} {
				trend, want := series.Trend{Down: down, Valid: valid}, need <= 0 || (valid > 0 && down >= need)
				if got := meets(trend, need); got != want {
					t.Errorf("need %d, trend %+v: meets %v, want %v", need, trend, got, want)
				}
				if got := shape.withTrend(trend, need).Holds; got != want {
					t.Errorf("need %d, trend %+v: the fold holds %v, want %v", need, trend, got, want)
				}
			}
		}
	}
	for _, trend := range []series.Trend{allDown, bearishTrend, shortTrend, {Valid: 3}, {Down: 3}, {}} {
		want := MinSmcTrendDown <= 0 || (trend.Valid > 0 && trend.Down >= MinSmcTrendDown)
		if got := MeetsTheNeed(trend); got != want {
			t.Errorf("trend %+v: MeetsTheNeed %v, want %v at MinSmcTrendDown", trend, got, want)
		}
		if got := shape.withTrend(trend, MinSmcTrendDown).Holds; got != want {
			t.Errorf("trend %+v: the fold at MinSmcTrendDown holds %v, want %v", trend, got, want)
		}
	}
}
