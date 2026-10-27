package smarttakeloss

import (
	"slices"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
)

// swingDeclineEntry is one remembered answer of the swing decline's reading with its own copies of
// the inputs it was worked out for; never changed once remembered.
type swingDeclineEntry struct {
	bars    []int64
	opens   []int64
	closes  []float64
	highs   []float64
	lows    []float64
	trend   series.Trend
	reading swingdecline.Reading
	holds   bool
}

func newSwingDeclineEntry(fills []entryFill, served series.Series, trend series.Trend, reading swingdecline.Reading, holds bool) *swingDeclineEntry {
	bars := make([]int64, len(fills))
	for index, fill := range fills {
		bars[index] = series.BarOpen(fill.At)
	}
	return &swingDeclineEntry{
		bars:    bars,
		opens:   slices.Clone(served.Opens),
		closes:  slices.Clone(served.Closes),
		highs:   slices.Clone(served.Highs),
		lows:    slices.Clone(served.Lows),
		trend:   trend,
		reading: cloneReading(reading),
		holds:   holds,
	}
}

// answers is whether the entry was worked out for exactly these inputs: the same trend, the same
// fills' bars and the same series, wicks included, cheapest comparison first.
func (entry *swingDeclineEntry) answers(fills []entryFill, served series.Series, trend series.Trend) bool {
	if entry.trend != trend || len(entry.bars) != len(fills) {
		return false
	}
	for index, fill := range fills {
		if entry.bars[index] != series.BarOpen(fill.At) {
			return false
		}
	}
	return slices.Equal(entry.opens, served.Opens) && slices.Equal(entry.closes, served.Closes) &&
		slices.Equal(entry.highs, served.Highs) && slices.Equal(entry.lows, served.Lows)
}

// cloneReading is the reading with its Reasons and Breaks copied, so the memo shares no slice.
func cloneReading(reading swingdecline.Reading) swingdecline.Reading {
	reading.Reasons = slices.Clone(reading.Reasons)
	reading.Breaks = slices.Clone(reading.Breaks)
	return reading
}
