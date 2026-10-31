package flatmarket

import (
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Bars is the last closed bars sophos serves, parallel arrays oldest first (open time in ms, open,
// high, low, close), ladder-agnostic; Served says whether the opens and closes can be read, Wicked
// whether the highs and lows can too.
type Bars struct {
	Opens                           []int64
	OpenPrices, Highs, Lows, Closes []float64
}

// MinBars is the floor of the bars, not a knob: a window needs an oldest bar apart from a newest;
// LineMinBars is the line's own, a close apart from its two ends.
const (
	MinBars     = 2
	LineMinBars = 3
)

// BarsOf is the block's bars as decoded, not copied: the window both readings read, the range on
// its newest RangeBars; a sophos that served none hands bars not served.
func BarsOf(block aggragates.SmartSellIndicators) Bars {
	return Bars{
		Opens:      block.FlatMarketBarOpens,
		OpenPrices: block.FlatMarketOpenPrices,
		Highs:      block.FlatMarketHighs,
		Lows:       block.FlatMarketLows,
		Closes:     block.FlatMarketCloses,
	}
}

// Len is the number of bars the arrays agree on, zero when they do not; highs and lows may be
// absent, but given at all they must agree too.
func (bars Bars) Len() int {
	count := len(bars.Opens)
	if len(bars.OpenPrices) != count || len(bars.Closes) != count {
		return 0
	}
	if (len(bars.Highs) != 0 && len(bars.Highs) != count) || (len(bars.Lows) != 0 && len(bars.Lows) != count) {
		return 0
	}
	return count
}

// Served is whether the bars can be read: the arrays agree, at least MinBars bars, every price
// priced; highs and lows given at all must agree and be priced too.
func (bars Bars) Served() bool {
	count := bars.Len()
	if count < MinBars || !wicksAgree(bars.Highs, count) || !wicksAgree(bars.Lows, count) {
		return false
	}
	for index := range count {
		if !priced(bars.OpenPrices[index]) || !priced(bars.Closes[index]) {
			return false
		}
	}
	return true
}

// wicksAgree is whether a highs or lows array is absent, or holds count priced values.
func wicksAgree(wicks []float64, count int) bool {
	if len(wicks) == 0 {
		return true
	}
	if len(wicks) != count {
		return false
	}
	for _, price := range wicks {
		if !priced(price) {
			return false
		}
	}
	return true
}

// Wicked is whether the highs and lows can be read as well: served, with both arrays given.
func (bars Bars) Wicked() bool {
	return bars.Served() && len(bars.Highs) == bars.Len() && len(bars.Lows) == bars.Len()
}

// Tail is the newest n bars, or all of them under n; not copied.
func (bars Bars) Tail(n int) Bars {
	count := bars.Len()
	if n <= 0 || n >= count {
		return bars
	}
	from := count - n
	tail := Bars{Opens: bars.Opens[from:], OpenPrices: bars.OpenPrices[from:], Closes: bars.Closes[from:]}
	if len(bars.Highs) == count && len(bars.Lows) == count {
		tail.Highs, tail.Lows = bars.Highs[from:], bars.Lows[from:]
	}
	return tail
}

// priced is a price above zero and not infinite (NaN is not above zero).
func priced(price float64) bool {
	return price > 0 && !math.IsInf(price, 1)
}

// LastBarOpen is the open time in ms of the newest bar served, zero when not served.
func (bars Bars) LastBarOpen() int64 {
	if !bars.Served() {
		return 0
	}
	return bars.Opens[len(bars.Opens)-1]
}

// bodyPct is a bar's body, close over open in signed percent of the open.
func (bars Bars) bodyPct(index int) float64 {
	return (bars.Closes[index]/bars.OpenPrices[index] - 1) * 100
}

// quietBodies is how many bars have a body (open to close, either way) under limit, in percent of
// the open; read of served bars only.
func (bars Bars) quietBodies(limit float64) int {
	quiet := 0
	for index, opened := range bars.OpenPrices {
		if math.Abs(bars.Closes[index]-opened)/opened*100 < limit {
			quiet++
		}
	}
	return quiet
}

// movePct is the move against a ladder of that side, in percent: the fall from the oldest high to
// the newest low for a long, the rise from the oldest low to the newest high for an inverse, a move
// the other way negative; NaN for any other side.
func (bars Bars) movePct(side string) float64 {
	last := len(bars.Opens) - 1
	switch side {
	case aggragates.SideLong:
		return (bars.Highs[0] - bars.Lows[last]) / bars.Highs[0] * 100
	case aggragates.SideShort:
		return (bars.Highs[last] - bars.Lows[0]) / bars.Lows[0] * 100
	}
	return math.NaN()
}

// stray is the mean of the oldest and newest closes, the inner close farthest from it in signed
// percent of the mean (the earlier of two as far) and that bar's open time; zero at no bar under
// three bars.
func (bars Bars) stray() (mean, pct float64, at int64) {
	last := len(bars.Closes) - 1
	mean = (bars.Closes[0] + bars.Closes[last]) / 2
	for index := 1; index < last; index++ {
		deviation := (bars.Closes[index] - mean) / mean * 100
		if math.Abs(deviation) > math.Abs(pct) {
			pct, at = deviation, bars.Opens[index]
		}
	}
	return mean, pct, at
}

// closeRange is the lowest and highest close of all the bars and the highest over the lowest in
// percent; the range is zero without bars or with a lowest close at or under zero.
func (bars Bars) closeRange() (low, high, pct float64) {
	if len(bars.Closes) < 1 {
		return 0, 0, 0
	}
	low, high = bars.Closes[0], bars.Closes[0]
	for _, closed := range bars.Closes[1:] {
		if closed < low {
			low = closed
		}
		if closed > high {
			high = closed
		}
	}
	if low <= 0 {
		return low, high, 0
	}
	return low, high, (high - low) / low * 100
}
