package series

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// TrendRow is one row of sophos' SMC trend dashboard as of the series' last bar: its timeframe,
// how many of its Reads read Down and Up (a row not read has no reads), and whether it is in
// FreeFall (a resistance, no support left below) or FreeRise (a support, no resistance left above).
type TrendRow struct {
	Timeframe          string
	Down, Up, Reads    int
	FreeFall, FreeRise bool
}

// Way is how many of the row's reads read the side's way: Down for aggragates.SideLong, Up for
// aggragates.SideShort, none for another side.
func (row TrendRow) Way(side string) int {
	switch side {
	case aggragates.SideLong:
		return row.Down
	case aggragates.SideShort:
		return row.Up
	}
	return 0
}

// Free is whether the row is in free fall the side's way: FreeFall for aggragates.SideLong,
// FreeRise for aggragates.SideShort, false for another side.
func (row TrendRow) Free(side string) bool {
	switch side {
	case aggragates.SideLong:
		return row.FreeFall
	case aggragates.SideShort:
		return row.FreeRise
	}
	return false
}

// ReadsOf is the block's dashboard rows as decoded, in dashboard order (1h, 4h, 1D); nil or unequal
// arrays are no rows. A free fall or free rise array not one per row leaves every row out of it.
func ReadsOf(block aggragates.SmartTakeLossIndicators) []TrendRow {
	count := len(block.SmcTrendTimeframes)
	if count == 0 || len(block.SmcTrendRowDown) != count || len(block.SmcTrendRowUp) != count || len(block.SmcTrendRowReads) != count {
		return nil
	}
	freeFall := len(block.SmcTrendRowFreeFall) == count
	freeRise := len(block.SmcTrendRowFreeRise) == count
	rows := make([]TrendRow, count)
	for index := range rows {
		rows[index] = TrendRow{
			Timeframe: block.SmcTrendTimeframes[index],
			Down:      int(block.SmcTrendRowDown[index]),
			Up:        int(block.SmcTrendRowUp[index]),
			Reads:     int(block.SmcTrendRowReads[index]),
			FreeFall:  freeFall && block.SmcTrendRowFreeFall[index],
			FreeRise:  freeRise && block.SmcTrendRowFreeRise[index],
		}
	}
	return rows
}
