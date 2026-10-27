package freefallprevention

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Series is the free fall series sophos serves, parallel arrays oldest first over the last closed
// bars of Timeframe: each bar's open time in ms and its masks, bit r set when dashboard row r reads
// free fall (FreeFall) or free rise (FreeRise) as of the bar's close.
type Series struct {
	Opens, FreeFall, FreeRise []int64
}

// SeriesOf is the block's series as decoded, not copied; a sophos that served none hands a series
// not served.
func SeriesOf(block aggragates.SmartTakeLossIndicators) Series {
	return Series{
		Opens:    block.FreeFallOpens,
		FreeFall: block.FreeFallMasks,
		FreeRise: block.FreeRiseMasks,
	}
}

// Served is whether the series can be read: at least one bar, the three arrays of one length.
func (bars Series) Served() bool {
	count := len(bars.Opens)
	return count > 0 && len(bars.FreeFall) == count && len(bars.FreeRise) == count
}

// Tail is the newest n bars, or all of them under n or when not served; not copied.
func (bars Series) Tail(n int) Series {
	count := len(bars.Opens)
	if !bars.Served() || n <= 0 || n >= count {
		return bars
	}
	from := count - n
	return Series{
		Opens:    bars.Opens[from:],
		FreeFall: bars.FreeFall[from:],
		FreeRise: bars.FreeRise[from:],
	}
}

// Masks is the masks a ladder of that side reads: FreeFall for aggragates.SideLong, FreeRise for
// aggragates.SideShort, nil for another side.
func (bars Series) Masks(side string) []int64 {
	switch side {
	case aggragates.SideLong:
		return bars.FreeFall
	case aggragates.SideShort:
		return bars.FreeRise
	}
	return nil
}
