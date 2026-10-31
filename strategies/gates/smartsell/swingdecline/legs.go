package swingdecline

// leg is one move between two swings by their wicks: down from a swing high to a swing low, up
// from a swing low to a swing high; a window's last leg is provisional, to the extreme wick so far.
type leg struct {
	down               bool
	fromBar, toBar     int
	fromPrice, toPrice float64
}

// pct is the leg's move in percent, negative for a fall.
func (move leg) pct() float64 {
	return (move.toPrice/move.fromPrice - 1) * 100
}

// bars is how many bars the leg runs, from its first swing's bar to its last's.
func (move leg) bars() int {
	return move.toBar - move.fromBar
}

// legsFrom are the legs of a window from its alternated pivots: from the highest swing high (the
// first of equals) alternating between the pivots, then one provisional leg from the last pivot to
// the extreme wick up to end, the lowest low after a high or the highest high after a low; false
// without a swing high to start from.
func legsFrom(highs, lows []float64, pivots []pivot, end int) ([]leg, bool) {
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
		legs = append(legs, leg{down: from.high, fromBar: from.bar, toBar: to.bar, fromPrice: from.price, toPrice: to.price})
	}
	last := swings[len(swings)-1]
	extreme := last.bar + 1
	for bar := last.bar + 1; bar <= end; bar++ {
		if (last.high && lows[bar] < lows[extreme]) || (!last.high && highs[bar] > highs[extreme]) {
			extreme = bar
		}
	}
	price := highs[extreme]
	if last.high {
		price = lows[extreme]
	}
	legs = append(legs, leg{down: last.high, fromBar: last.bar, toBar: extreme, fromPrice: last.price, toPrice: price})
	return legs, true
}
