package dynamicparams

import (
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// openedEvents remembers the decoded opened events: a ladder's raise is rebuilt on every print.
var openedEvents aggragates.EventMemo[OpenedEvent]

// OpenedRaise is the raise a ladder opened with, rebuilt from its opened events on every tick (the
// last one wins): the points and depths its event names and true, zeros and false without an
// event. Unreadable events are skipped and an amount not over zero adds nothing.
func OpenedRaise(trade aggragates.Trades) (points float64, depths int, ok bool) {
	for _, event := range trade.StrategyEventsOf(aggragates.StrategyParamDynamicParams, GateOpened) {
		data, err := openedEvents.Decode(event)
		if err != nil || data.Event != EventOpened {
			continue
		}
		points, depths, ok = openedPoints(data.Points), openedDepths(data.Depths), true
	}

	return points, depths, ok
}

// openedPoints is the points an opened event names, zero unless a finite amount over zero.
func openedPoints(points float64) float64 {
	if math.IsNaN(points) || math.IsInf(points, 0) || points <= 0 {
		return 0
	}

	return points
}

// openedDepths is the depths an opened event names, zero unless over zero.
func openedDepths(depths int) int {
	if depths <= 0 {
		return 0
	}

	return depths
}
