package swingdecline

import (
	"slices"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// MeetsTheNeed is the trend condition alone, at MinSmcTrendDown; a caller asks it first to read no
// window for a trend that misses.
func MeetsTheNeed(trend series.Trend) bool {
	return meets(trend, MinSmcTrendDown)
}

// meets is whether the trend reads down on at least need rows, or need is off (zero or below).
func meets(trend series.Trend, need int) bool {
	return trend.Meets(aggragates.SideLong, need)
}

// withTrend folds the trend into a reading: met, a holding reading names it last in Reasons;
// missed or not read, the hold is taken away and the trend named in Breaks; need off, unchanged.
func (reading Reading) withTrend(trend series.Trend, need int) Reading {
	if need <= 0 {
		return reading
	}
	if meets(trend, need) {
		if reading.Holds {
			reading.Reasons = append(slices.Clip(reading.Reasons), trendHoldReason(trend, need))
		}
		return reading
	}
	miss := trendBreakReason(trend, need)
	if reading.Holds {
		reading.Holds = false
		reading.Reasons = nil
		reading.Breaks = []string{windowFrame(reading), miss}
		return reading
	}
	reading.Breaks = append(slices.Clip(reading.Breaks), miss)
	return reading
}
