package lineardecline

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// MeetsTheNeed is the trend condition alone, at MinSmcTrendDown, which Trigger asks before any
// fill or bar; a caller asks it first to build nothing for a trend that misses.
func MeetsTheNeed(trend series.Trend) bool {
	return meets(trend, MinSmcTrendDown)
}

// meets is whether the trend reads down on at least need rows, or need is off (zero or below).
func meets(trend series.Trend, need int) bool {
	return trend.Meets(aggragates.SideLong, need)
}
