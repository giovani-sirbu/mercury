package freefallprevention

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Reading is what the gate reads for a ladder of one side on the rows FreeFallTimeframes counts:
// LastBar when at least MinFreeFallTimeframes of them are free the ladder's way on the last closed
// bar, Count of the Of bars served on which at least as many were, Holds when either reads, and the
// Reasons a holding reading names.
type Reading struct {
	Holds, LastBar bool
	Count, Of      int
	Reasons        []string
}

// Trigger is the gate's reading of the block for a ladder of that side: the last closed bar's
// rows, then the share of the newest Window bars of the series; MinFreeFallTimeframes at or below
// zero reads nothing.
func Trigger(block aggragates.SmartSellIndicators, side string) Reading {
	if MinFreeFallTimeframes <= 0 {
		return Reading{}
	}
	rows := series.ReadsOf(block)
	counted := countedMask(rows)
	free := freeMask(rows, counted, side)
	reading := Reading{LastBar: rowCount(free) >= MinFreeFallTimeframes}
	reading.Count, reading.Of = freeBars(SeriesOf(block).Tail(Window), shareMask(counted), side)
	shared := shareOver(reading.Count, reading.Of)
	reading.Holds = reading.LastBar || shared
	if reading.Holds {
		reading.Reasons = holdReasons(reading, shared, rowNames(rows, free), shareRows(rows, counted), side)
	}
	return reading
}

// freeBars is how many bars have at least MinFreeFallTimeframes bits of counted set in the side's
// masks, of the bars served; none of none when the series is not served or the side reads no masks.
func freeBars(bars Series, counted int64, side string) (count, of int) {
	masks := bars.Masks(side)
	if !bars.Served() || masks == nil {
		return 0, 0
	}
	for _, mask := range masks {
		if rowCount(mask&counted) >= MinFreeFallTimeframes {
			count++
		}
	}
	return count, len(masks)
}

// shareOver is whether count of of bars is strictly more than MinShare, as a division so the bound
// is exact on a whole number of bars.
func shareOver(count, of int) bool {
	return of > 0 && float64(count)/float64(of) > MinShare
}

// holdReasons are the reasons a holding reading names: the last bar's free rows when it reads, the
// share's counted rows when it is over.
func holdReasons(reading Reading, shared bool, freeRows, sharedRows, side string) []string {
	reasons := make([]string, 0, 2)
	if reading.LastBar {
		reasons = append(reasons, fmt.Sprintf("%s in %s on the last closed bar", freeRows, freeWord(side)))
	}
	if shared {
		reasons = append(reasons, fmt.Sprintf(
			"%s in %s on %d of %d %s bars, over %.0f%%",
			sharedRows, freeWord(side), reading.Count, reading.Of, Timeframe, MinShare*100,
		))
	}
	return reasons
}

// freeWord is the free fall a side reads: free rise for an inverse ladder, free fall otherwise.
func freeWord(side string) string {
	if side == aggragates.SideShort {
		return "free rise"
	}
	return "free fall"
}
