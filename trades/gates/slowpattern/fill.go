package slowpattern

import "time"

// Fill is one executed entry order of a ladder, in the order the ladder placed
// them: the price it paid and the stamp of its history row. The stamp is the
// order's placement tick in backtesting, its reconciliation time in production
// and the wall clock in live-testing, so a fill's bar can differ from the bar
// it filled in by one — accepted.
type Fill struct {
	Price float64
	At    time.Time
}

// BarOpen is the open time, in ms, of the 1h bar a stamp falls in: its UTC
// milliseconds floored to SlowPatternBarMs. A zero time has no bar and answers
// zero, as does a stamp before the epoch; a zero open time is never in a
// series, so a fill without a stamp reads nothing.
func BarOpen(at time.Time) int64 {
	if at.IsZero() {
		return 0
	}
	ms := at.UnixMilli()
	if ms <= 0 {
		return 0
	}
	return ms - ms%SlowPatternBarMs
}
