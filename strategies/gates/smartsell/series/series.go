package series

import (
	"sort"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Series is the closed 1h bars sophos serves, parallel arrays oldest first: the open time in ms and
// the close (Served), and the high and the low, the wicks the swing decline reads (WicksServed);
// empty or unequal arrays are not served.
type Series struct {
	Opens  []int64
	Closes []float64
	Highs  []float64
	Lows   []float64
}

// SeriesOf is the block's series as decoded; a sophos that served none hands a series not served.
func SeriesOf(block aggragates.SmartSellIndicators) Series {
	return Series{Opens: block.SeriesOpens, Closes: block.SeriesCloses, Highs: block.SeriesHighs, Lows: block.SeriesLows}
}

// Served is whether the series holds any bar, the opens and closes agreeing; the wicks are not asked.
func (bars Series) Served() bool {
	return len(bars.Opens) > 0 && len(bars.Opens) == len(bars.Closes)
}

// WicksServed is whether a served series carries a high and a low for every bar.
func (bars Series) WicksServed() bool {
	return bars.Served() && len(bars.Highs) == len(bars.Opens) && len(bars.Lows) == len(bars.Opens)
}

// LastBarOpen is the open time in ms of the newest bar, zero when not served.
func (bars Series) LastBarOpen() int64 {
	if !bars.Served() {
		return 0
	}
	return bars.Opens[len(bars.Opens)-1]
}

// IndexOf is the position of the bar opening at open, by binary search (an exchange leaves gaps);
// false when not held.
func (bars Series) IndexOf(open int64) (int, bool) {
	if !bars.Served() {
		return 0, false
	}
	index := sort.Search(len(bars.Opens), func(at int) bool { return bars.Opens[at] >= open })
	if index == len(bars.Opens) || bars.Opens[index] != open {
		return 0, false
	}
	return index, true
}

// BarAtOrAfter is the open time of the first bar held at or after open (the first bar for an open
// before the series); false past the last bar or when not served.
func (bars Series) BarAtOrAfter(open int64) (int64, bool) {
	if !bars.Served() {
		return 0, false
	}
	index := sort.Search(len(bars.Opens), func(at int) bool { return bars.Opens[at] >= open })
	if index == len(bars.Opens) {
		return 0, false
	}
	return bars.Opens[index], true
}
