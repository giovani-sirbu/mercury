package slowpattern

// Reading is what one window between two fills reads: the shape of the decline
// from the bar of the earlier fill to the bar of the later one.
//
// FromDepth and ToDepth are the depths of the two fills, StartAt and EndAt the
// open times in ms of the bars that hold them. Weight is the decline's weight,
// (ΣD − ΣU) / (ΣD + ΣU), ΣD the summed fall of the down legs as positive
// percentages and ΣU the summed rise of the up legs. DownLegs counts the down
// legs, the provisional last one included. LowerHighs counts the successive
// down legs that start strictly lower than the one before. MaxLegShare is the
// largest down leg over ΣD. NetPct is the end bar's close against the start
// bar's, in percent.
//
// Holds is whether every reading meets its constant: Weight at
// SlowPatternMinWeight or more, DownLegs at SlowPatternMinDownLegs or more,
// LowerHighs at SlowPatternMinLowerHighs or more, MaxLegShare at
// SlowPatternMaxLegShare or less and NetPct at SlowPatternMinFallPct or less.
// Reasons names the window and each reading while it Holds, and Breaks names
// the window and each reading that does not meet its constant while it does
// not — what the smart take loss writes on its pending and its cancelled row.
// A reading that was not read has none of the readings, and a Trigger that
// could not read any window names why in Breaks alone.
type Reading struct {
	FromDepth, ToDepth int
	StartAt, EndAt     int64
	Weight             float64
	DownLegs           int
	LowerHighs         int
	MaxLegShare        float64
	NetPct             float64
	Holds              bool
	Reasons, Breaks    []string
}

// Read reads the window of series from the bar opening at startAt to the bar
// opening at endAt, both inclusive, for the fills at fromDepth and toDepth. It
// is a pure function of those bars and the bars before the start a pivot
// looks at: pivots are swing highs and lows of the closes (swingPivots), only
// the ones known at the end bar and not before the start count, and the legs
// run from the highest of the swing highs (legsFrom).
//
// It answers false — unreadable, which never holds — when the start or the end
// bar is not in the series, the end is before the start, the window holds fewer
// bars than one pivot needs, a close in it is at or under zero, no swing high
// stands at or after the start, or the legs move nothing (ΣD + ΣU is zero).
func Read(series Series, startAt, endAt int64, fromDepth, toDepth int) (Reading, bool) {
	reading := Reading{FromDepth: fromDepth, ToDepth: toDepth, StartAt: startAt, EndAt: endAt}
	start, end, ok := series.window(startAt, endAt)
	if !ok {
		return reading, false
	}
	closes := series.Closes[:end+1]
	var known []pivot
	for _, candidate := range swingPivots(closes, SlowPatternPivotBars) {
		if candidate.bar >= start {
			known = append(known, candidate)
		}
	}
	legs, ok := legsFrom(closes, alternate(known), end)
	if !ok {
		return reading, false
	}

	var fall, rise, largest, previousHigh float64
	for _, move := range legs {
		if !move.down {
			rise += move.pct
			continue
		}
		fall -= move.pct
		largest = max(largest, -move.pct)
		if reading.DownLegs > 0 && move.fromPrice < previousHigh {
			reading.LowerHighs++
		}
		previousHigh = move.fromPrice
		reading.DownLegs++
	}
	if fall+rise == 0 {
		return reading, false
	}
	reading.Weight = (fall - rise) / (fall + rise)
	if fall > 0 {
		reading.MaxLegShare = largest / fall
	}
	reading.NetPct = (closes[end]/closes[start] - 1) * 100
	reading.Holds = holds(reading)
	if reading.Holds {
		reading.Reasons = holdReasons(reading)
	} else {
		reading.Breaks = breakReasons(reading)
	}
	return reading, true
}

// holds is whether every reading of a window meets its constant: the five
// comparisons Reading names, each with the bound included.
func holds(reading Reading) bool {
	return reading.Weight >= SlowPatternMinWeight &&
		reading.DownLegs >= SlowPatternMinDownLegs &&
		reading.LowerHighs >= SlowPatternMinLowerHighs &&
		reading.MaxLegShare <= SlowPatternMaxLegShare &&
		reading.NetPct <= SlowPatternMinFallPct
}
