package swingdecline

// pivot is a swing high or a swing low: the bar's position and its wick, the high of a swing high
// and the low of a swing low.
type pivot struct {
	bar   int
	price float64
	high  bool
}

// swingPivots are the swing highs of the highs and the swing lows of the lows, oldest first, at
// width bars each side, ties counting (a bar may be both, the high first); a pivot exists only once
// width bars closed after it, so appending bars never changes one found before.
func swingPivots(highs, lows []float64, width int) []pivot {
	var pivots []pivot
	for bar := width; bar < len(highs)-width; bar++ {
		high, low := true, true
		for neighbour := bar - width; neighbour <= bar+width; neighbour++ {
			high = high && highs[neighbour] <= highs[bar]
			low = low && lows[neighbour] >= lows[bar]
		}
		if high {
			pivots = append(pivots, pivot{bar: bar, price: highs[bar], high: true})
		}
		if low {
			pivots = append(pivots, pivot{bar: bar, price: lows[bar]})
		}
	}
	return pivots
}

// alternate folds consecutive pivots of one kind into the more extreme (the later on a tie), so
// highs and lows alternate.
func alternate(pivots []pivot) []pivot {
	folded := make([]pivot, 0, len(pivots))
	for _, next := range pivots {
		last := len(folded) - 1
		if last < 0 || folded[last].high != next.high {
			folded = append(folded, next)
			continue
		}
		if (next.high && next.price >= folded[last].price) || (!next.high && next.price <= folded[last].price) {
			folded[last] = next
		}
	}
	return folded
}
