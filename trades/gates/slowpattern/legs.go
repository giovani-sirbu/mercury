package slowpattern

// leg is one move of a window between two swings: a down leg from a swing
// high, an up leg from a swing low. fromPrice is the close it starts from and
// pct its move in percent, negative for a fall.
type leg struct {
	down      bool
	fromPrice float64
	pct       float64
}

// legsFrom are the legs of a window whose known, alternated pivots are given,
// ending at the bar end of closes: they start at the HIGHEST swing high — the
// first of equals — and alternate down and up between the pivots from there,
// then one provisional leg runs from the last pivot to the extreme close after
// it up to the end bar: the lowest close after a high, the highest after a
// low. The provisional leg is the decline still going on, the one a pivot
// cannot yet know; a pivot sits at least SlowPatternPivotBars before the end,
// so there is always a close after the last one. It answers false when the
// window holds no swing high to start from.
func legsFrom(closes []float64, pivots []pivot, end int) ([]leg, bool) {
	anchor := -1
	for index, candidate := range pivots {
		if candidate.high && (anchor < 0 || candidate.price > pivots[anchor].price) {
			anchor = index
		}
	}
	if anchor < 0 {
		return nil, false
	}
	swings := pivots[anchor:]
	legs := make([]leg, 0, len(swings))
	for index := 1; index < len(swings); index++ {
		from, to := swings[index-1], swings[index]
		legs = append(legs, leg{down: from.high, fromPrice: from.price, pct: (to.price/from.price - 1) * 100})
	}
	last := swings[len(swings)-1]
	extreme := closes[last.bar+1]
	for _, closed := range closes[last.bar+1 : end+1] {
		if (last.high && closed < extreme) || (!last.high && closed > extreme) {
			extreme = closed
		}
	}
	legs = append(legs, leg{down: last.high, fromPrice: last.price, pct: (extreme/last.price - 1) * 100})
	return legs, true
}
