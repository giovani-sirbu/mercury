package dynamicparams

import (
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// OpenedRaise is the raise a ladder opened with: the points its opened event
// adds to every row's percentage, the depths it adds to every row's depths,
// and true; zeros and false when the trade carries no opened event. It is
// rebuilt from trade.StrategyEvents on every tick, the way
// smarttakeloss.rebuildState and the cooldown first-fill state are: the events
// are the only state.
//
// The events are folded in slice order — agora loads them in id order,
// sisyphus appends them — and the last opened event wins. An event of another
// gate, an event whose document cannot be read and an event of another kind
// are skipped, as is an opened row without its event: the row is the text an
// operator reads, and the text alone opens nothing. A part the event does not
// name adds nothing, and so does one whose amount is not a finite amount over
// zero, the only amounts BearPercentagePoints and BearDepths can write.
//
// The amounts are read from the event itself, never from those constants, so
// a later retune reaches the next ladder that opens and never one already
// open.
func OpenedRaise(trade aggragates.Trades) (points float64, depths int, ok bool) {
	for _, event := range trade.StrategyEventsOf(aggragates.StrategyParamDynamicParams, GateOpened) {
		var data OpenedEvent
		if err := event.DecodeData(&data); err != nil || data.Event != EventOpened {
			continue
		}
		points, depths, ok = openedPoints(data.Points), openedDepths(data.Depths), true
	}

	return points, depths, ok
}

// openedPoints is the percentage points an opened event names, zero when the
// amount is not a finite amount over zero.
func openedPoints(points float64) float64 {
	if math.IsNaN(points) || math.IsInf(points, 0) || points <= 0 {
		return 0
	}

	return points
}

// openedDepths is the depths an opened event names, zero when the amount is
// not over zero.
func openedDepths(depths int) int {
	if depths <= 0 {
		return 0
	}

	return depths
}
