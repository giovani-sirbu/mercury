package series

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Trend is the Trend Direction row of sophos' SMC trend dashboard as of the series' last bar: the
// rows reading Up, reading Down, and Valid (read at all).
type Trend struct {
	Up, Down, Valid int
}

// TrendOf is the block's trend as decoded.
func TrendOf(block aggragates.SmartTakeLossIndicators) Trend {
	return Trend{
		Up:    block.SmcTrendUp,
		Down:  block.SmcTrendDown,
		Valid: block.SmcTrendValid,
	}
}

// Served is whether sophos read the dashboard: at least one valid row.
func (trend Trend) Served() bool {
	return trend.Valid > 0
}

// Rows is how many rows read the side's way: Down for aggragates.SideLong, Up for
// aggragates.SideShort, none for another side.
func (trend Trend) Rows(side string) int {
	switch side {
	case aggragates.SideLong:
		return trend.Down
	case aggragates.SideShort:
		return trend.Up
	}
	return 0
}

// Meets is whether a served trend reads the side's way on at least need rows, or need is off (zero
// or below); a side with no way never meets.
func (trend Trend) Meets(side string, need int) bool {
	if !Sided(side) {
		return false
	}
	return need <= 0 || (trend.Served() && trend.Rows(side) >= need)
}

// Sided is whether a side has a way on the dashboard: aggragates.SideLong or aggragates.SideShort.
func Sided(side string) bool {
	return side == aggragates.SideLong || side == aggragates.SideShort
}
