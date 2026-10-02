package swingdecline

import (
	"slices"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Trend is the Trend Direction row of sophos' SMC trend dashboard as the block
// serves it: Down the rows whose trend reads down, Valid the rows sophos read —
// a row that is not valid counts for nothing. It is read at the series' interval
// as of the close of the series' last bar, for every ladder alike, and it is the
// one input of Trigger that is not a function of the fills' bars.
type Trend struct {
	Down, Valid int
}

// TrendOf is the trend of a smart take loss block: the two counts sophos
// serves, as they decoded.
func TrendOf(block aggragates.SmartTakeLossIndicators) Trend {
	return Trend{Down: block.SwingDeclineSmcTrendDown, Valid: block.SwingDeclineSmcTrendValid}
}

// Served is whether sophos read the dashboard: at least one row was valid. A
// sophos without the keys, or one whose dashboard could not be read, serves
// none, and the trend is not read — which meets no need above zero.
func (trend Trend) Served() bool {
	return trend.Valid > 0
}

// withTrend folds the trend into the reading Trigger's window search answered,
// against the least number of rows that must read down (need).
//
// A trend that meets the need — served, with Down at least need, the bound
// included — leaves the reading as it was and, when it Holds, names the trend
// last of its Reasons. One that does not, or that was not read, takes the hold
// away: a reading that held on its shape loses its Reasons and names the window
// and the trend in its Breaks, one that did not hold keeps the Breaks that say
// why and names the trend after them.
//
// At need zero or below the condition is off and the reading comes back as it
// was, no reason added. Trigger passes SwingDeclineMinSmcTrendDown; the need is a
// parameter only so the off branch can be read without turning the constant into
// a variable.
func (reading Reading) withTrend(trend Trend, need int) Reading {
	if need <= 0 {
		return reading
	}
	if trend.Served() && trend.Down >= need {
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
