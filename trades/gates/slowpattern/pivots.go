package slowpattern

// pivot is a swing high or a swing low of a series of closes: the position of
// its bar and the close there.
type pivot struct {
	bar   int
	price float64
	high  bool
}

// swingPivots are the swing highs and swing lows of closes, oldest first, at
// width bars each side. A bar j is a swing high when its close is the highest
// of the closes j−width … j+width, and a swing low when it is the lowest; TIES
// COUNT, so a bar equal to its neighbours is a swing of that kind — this is
// not the strict tie rule of sophos' SMC pivots, on purpose. A bar both
// highest and lowest, a flat stretch, is both, the high first. Only bars with
// width bars each side in closes are read, so a pivot exists only once width
// bars have closed after it: appending bars to closes never changes a pivot
// found before, which is what makes a window's reading derivable again on any
// later tick.
func swingPivots(closes []float64, width int) []pivot {
	var pivots []pivot
	for bar := width; bar < len(closes)-width; bar++ {
		high, low := true, true
		for neighbour := bar - width; neighbour <= bar+width; neighbour++ {
			high = high && closes[neighbour] <= closes[bar]
			low = low && closes[neighbour] >= closes[bar]
		}
		if high {
			pivots = append(pivots, pivot{bar: bar, price: closes[bar], high: true})
		}
		if low {
			pivots = append(pivots, pivot{bar: bar, price: closes[bar]})
		}
	}
	return pivots
}

// alternate folds consecutive pivots of one kind into the more extreme of
// them — the higher of two highs, the lower of two lows, the later one on a
// tie — so highs and lows alternate. It reads the pivots it is given in order
// and nothing else, so the same pivots fold the same way on every call.
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
