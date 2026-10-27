package lineardecline

import "math"

// Reading is what the two chords from the window's first fill read at the newest: the depths and
// bars they run between, Slope and PreviousSlope (percent per hour), Deviation and
// PreviousDeviation, NetPct, Holds and the Reasons a holding line names; a miss carries no text (Breaks).
type Reading struct {
	FromDepth, ToDepth           int
	StartAt, EndAt               int64
	Slope, PreviousSlope         float64
	Deviation, PreviousDeviation float64
	NetPct                       float64
	Holds                        bool
	Reasons                      []string
}

// misses is the set of conditions a line misses, one bit each; the trend is not among them.
type misses uint16

const (
	// missFills is a ladder with fewer fills than Window.
	missFills misses = 1 << iota
	// missStamps is a chord whose stamps cannot draw a line.
	missStamps
	// missPrices is a chord with a fill price not above zero.
	missPrices
	// missBars is a series not served or missing the bar of a fill.
	missBars
	// missCloses is a close at or under zero among a chord's bars.
	missCloses
	// missDeviation and missPreviousDeviation are a deviation over MaxDeviationPct on each chord.
	missDeviation
	missPreviousDeviation
	// missRising and missPreviousRising are a slope not under zero on each chord.
	missRising
	missPreviousRising
	// missSlopeChange is a change of slope over MaxSlopeChange, asked of two falling slopes.
	missSlopeChange
	// missFall is a net move short of MinFallPct.
	missFall
)

// missUnread are the misses that leave a line unread, nothing measured.
const missUnread = missStamps | missPrices | missBars | missCloses

// missesOf is every condition the measured numbers miss, the trend apart (NaN misses).
func missesOf(reading Reading) misses {
	var missed misses
	if !(reading.Deviation <= MaxDeviationPct) {
		missed |= missDeviation
	}
	if !(reading.PreviousDeviation <= MaxDeviationPct) {
		missed |= missPreviousDeviation
	}
	if !(reading.Slope < 0) {
		missed |= missRising
	}
	if !(reading.PreviousSlope < 0) {
		missed |= missPreviousRising
	}
	if reading.Slope < 0 && reading.PreviousSlope < 0 && !(slopeChange(reading) <= MaxSlopeChange) {
		missed |= missSlopeChange
	}
	if !(reading.NetPct <= MinFallPct) {
		missed |= missFall
	}
	return missed
}

// slopeChange is the change of slope between the chords as a share of the previous one, read of
// two falling slopes.
func slopeChange(reading Reading) float64 {
	return math.Abs(reading.Slope/reading.PreviousSlope - 1)
}
