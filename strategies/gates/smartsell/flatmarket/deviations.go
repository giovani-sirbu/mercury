package flatmarket

import "math"

// deviations is what the bars read against the line: the farthest close above and below it (signed
// percent, with the bars' open times), how many bars count against the tolerance — a close past the
// bound or a body over it (counted) — and how many of those on their body alone (wide), with the
// widest body and its bar.
type deviations struct {
	above, below     float64
	aboveAt, belowAt int64
	counted, wide    int
	widest           float64
	widestAt         int64
}

// read measures every bar of served bars against the line and the bound.
func (fitted line) read(bars Bars, bound float64) deviations {
	var found deviations
	for index, closed := range bars.Closes {
		deviation := deviationPct(closed, fitted.at(index))
		if deviation > found.above {
			found.above, found.aboveAt = deviation, bars.Opens[index]
		}
		if deviation < found.below {
			found.below, found.belowAt = deviation, bars.Opens[index]
		}
		body := bars.bodyPct(index)
		if math.Abs(body) > math.Abs(found.widest) {
			found.widest, found.widestAt = body, bars.Opens[index]
		}
		wide := !withinBound(body, bound)
		if wide {
			found.wide++
		}
		if wide || !withinBound(deviation, bound) {
			found.counted++
		}
	}
	return found
}
