package dynamicparams

import (
	"math"
	"strconv"
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// OpenedRaise is the raise a ladder opened with: the points its opened row
// adds to every row's percentage, the depths it adds to every row's depths,
// and true; zeros and false when the trade carries no opened row. It is
// rebuilt from trade.Logs on every tick, the way smarttakeloss.rebuildState
// and the cooldown first-fill state are: the row is the only state.
//
// The rows are folded in slice order — hermes loads them in id order,
// sisyphus appends them — and the last opened row wins. A row is matched by
// RowPrefix followed by OpenedMarker anywhere in its message, so a frame an
// engine writes around it (gates.SaveHoldLog's "Hold …: ") does not hide it,
// while every other row carrying RowPrefix is ignored. A part the row does not
// name adds nothing, and so does one whose amount is not a finite amount over
// zero, the only amounts BearPercentagePoints and BearDepths can write.
//
// The amounts are parsed back from the row itself, never read from those
// constants, so a later retune reaches the next ladder that opens and never
// one already open.
func OpenedRaise(trade aggragates.Trades) (points float64, depths int, ok bool) {
	for _, row := range trade.Logs {
		_, body, found := strings.Cut(row.Message, openedHead)
		if !found {
			continue
		}
		points, depths, ok = openedPoints(body), openedDepths(body), true
	}

	return points, depths, ok
}

// openedPoints is the percentage points an opened row's body names under
// percentageLabel, zero when it names none it can read.
func openedPoints(body string) float64 {
	amount, named := amountAfter(body, percentageLabel)
	if !named {
		return 0
	}

	points, err := strconv.ParseFloat(amount, 64)
	if err != nil || math.IsNaN(points) || math.IsInf(points, 0) || points <= 0 {
		return 0
	}

	return points
}

// openedDepths is the depths an opened row's body names under depthsLabel,
// zero when it names none it can read.
func openedDepths(body string) int {
	amount, named := amountAfter(body, depthsLabel)
	if !named {
		return 0
	}

	depths, err := strconv.Atoi(amount)
	if err != nil || depths <= 0 {
		return 0
	}

	return depths
}

// amountAfter is the word that follows label in text — the amount an opened
// row names under it — and whether label is there at all.
func amountAfter(text, label string) (string, bool) {
	_, rest, found := strings.Cut(text, label)
	if !found {
		return "", false
	}

	amount, _, _ := strings.Cut(rest, " ")

	return amount, true
}
