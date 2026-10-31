package swingdecline

import "github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"

// Reading is what one window reads: the depths and bars it runs between, the swing high it fell
// from (High on the bar HighAt) and the lowest low since (Low on LowAt, the provisional leg's
// included), the legs that count (DownLegs, UpLegs), the bounces over MaxBouncePct (Bounces,
// named and not counted), Holds, and the Reasons or Breaks it names; Trigger and TriggerAtLastBar
// fold the trend in last (withTrend).
type Reading struct {
	FromDepth, ToDepth int
	StartAt, EndAt     int64
	HighAt, LowAt      int64
	High, Low          float64
	DownLegs, UpLegs   int
	Bounces            int
	Holds              bool
	Reasons, Breaks    []string
}

// Read reads the window from the bar opening at startAt to the one at endAt, inclusive, on the
// wicks: the pivots known by the end bar (swingPivots), the legs from the highest swing high
// (legsFrom), then the legs that count; false, unreadable, on a missing bar or wick, a bad range,
// a price at or under zero or no swing high.
func Read(bars series.Series, startAt, endAt int64, fromDepth, toDepth int) (Reading, bool) {
	reading := Reading{FromDepth: fromDepth, ToDepth: toDepth, StartAt: startAt, EndAt: endAt}
	start, end, ok := windowOf(bars, startAt, endAt)
	if !ok {
		return reading, false
	}
	highs, lows := bars.Highs[:end+1], bars.Lows[:end+1]
	var known []pivot
	for _, candidate := range swingPivots(highs, lows, PivotBars) {
		if candidate.bar >= start {
			known = append(known, candidate)
		}
	}
	legs, ok := legsFrom(highs, lows, alternate(known), end)
	if !ok {
		return reading, false
	}

	reading.HighAt, reading.High = bars.Opens[legs[0].fromBar], legs[0].fromPrice
	for _, move := range legs {
		if !move.down {
			if move.pct() <= MaxBouncePct {
				reading.UpLegs++
			} else {
				reading.Bounces++
			}
			continue
		}
		if reading.LowAt == 0 || move.toPrice < reading.Low { // the first down leg is the anchor's own
			reading.LowAt, reading.Low = bars.Opens[move.toBar], move.toPrice
		}
		if move.bars() >= MinBarsBetween && -move.pct() >= MinDropPct {
			reading.DownLegs++
		}
	}
	reading.Holds = holds(reading)
	if reading.Holds {
		reading.Reasons = holdReasons(reading)
	} else {
		reading.Breaks = breakReasons(reading)
	}
	return reading, true
}

// windowOf is the positions of the bars opening at startAt and endAt when a window between them
// can be read: the wicks served, both bars held, the end not before the start, at least the bars
// one pivot needs, every close and low over zero and every high at or over its low.
func windowOf(bars series.Series, startAt, endAt int64) (start, end int, ok bool) {
	if !bars.WicksServed() {
		return 0, 0, false
	}
	start, startFound := bars.IndexOf(startAt)
	end, endFound := bars.IndexOf(endAt)
	if !startFound || !endFound || end < start || end-start+1 < 2*PivotBars+1 {
		return 0, 0, false
	}
	for bar := start; bar <= end; bar++ {
		if bars.Closes[bar] <= 0 || bars.Lows[bar] <= 0 || bars.Highs[bar] < bars.Lows[bar] {
			return 0, 0, false
		}
	}
	return start, end, true
}

// holds is whether the legs that count reach their constants, each bound included.
func holds(reading Reading) bool {
	return reading.DownLegs >= MinLegsDown && reading.UpLegs >= MinLegsUp
}
