package freefallprevention

import (
	"fmt"
	"math/bits"
	"strings"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
)

// countedMask is the served rows that count as a mask, bit r for dashboard row r (countsFreeFall);
// a row counts once, however many entries name it.
func countedMask(rows []series.TrendRow) int64 {
	var mask int64
	for row, read := range rows {
		if countsFreeFall(read.Timeframe) {
			mask |= 1 << row
		}
	}
	return mask
}

// freeMask is the rows of counted free the side's way on the last closed bar, as a mask.
func freeMask(rows []series.TrendRow, counted int64, side string) int64 {
	var mask int64
	for row, read := range rows {
		if counted&(1<<row) != 0 && read.Free(side) {
			mask |= 1 << row
		}
	}
	return mask
}

// shareMask is the rows a bar of the share counts: every bit when FreeFallTimeframes names none,
// the rows served or not, the counted rows served otherwise.
func shareMask(counted int64) int64 {
	if len(FreeFallTimeframes) == 0 {
		return ^int64(0)
	}
	return counted
}

// countsFreeFall is whether a row of this timeframe counts: FreeFallTimeframes names it, whatever
// its case, or names none.
func countsFreeFall(timeframe string) bool {
	if len(FreeFallTimeframes) == 0 {
		return true
	}
	for _, listed := range FreeFallTimeframes {
		if strings.EqualFold(listed, timeframe) {
			return true
		}
	}
	return false
}

// rowCount is how many rows a mask sets.
func rowCount(mask int64) int {
	return bits.OnesCount64(uint64(mask))
}

// shareRows names the counted rows served for the share's reason, after "at least N of " when
// MinFreeFallTimeframes is fewer than them; "at least N of the rows" when none is served.
func shareRows(rows []series.TrendRow, counted int64) string {
	served := rowCount(counted)
	if served == 0 {
		return fmt.Sprintf("at least %d of the rows", MinFreeFallTimeframes)
	}
	names := rowNames(rows, counted)
	if MinFreeFallTimeframes < served {
		return fmt.Sprintf("at least %d of %s", MinFreeFallTimeframes, names)
	}
	return names
}

// rowNames names the rows of mask by their timeframes, "row N" (from 1) for one served without its
// timeframe, as "1h", "1h and 4h" or "1h, 4h and 1D".
func rowNames(rows []series.TrendRow, mask int64) string {
	var names []string
	for row, read := range rows {
		if mask&(1<<row) == 0 {
			continue
		}
		if read.Timeframe != "" {
			names = append(names, read.Timeframe)
			continue
		}
		names = append(names, fmt.Sprintf("row %d", row+1))
	}
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	last := len(names) - 1
	return strings.Join(names[:last], ", ") + " and " + names[last]
}
