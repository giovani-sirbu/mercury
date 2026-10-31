package series

import "time"

// Fill is one executed entry order: the price it paid and its history row's stamp (the placement
// tick in backtesting, the fill time in production, the wall clock in live-testing).
type Fill struct {
	Price float64
	At    time.Time
}

// BarOpen is the open time in ms of the 1h bar a stamp falls in, floored to BarMs; zero for a zero
// time or one before the epoch, which no series holds.
func BarOpen(at time.Time) int64 {
	if at.IsZero() {
		return 0
	}
	ms := at.UnixMilli()
	if ms <= 0 {
		return 0
	}
	return ms - ms%BarMs
}
