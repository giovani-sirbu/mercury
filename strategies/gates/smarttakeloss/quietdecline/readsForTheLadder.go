package quietdecline

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// ReadsForTheLadder is whether the quiet decline reads for a ladder on the last closed bar: sophos'
// verdict (QuietDeclineVerdict), or the leg on and quiet (QuietDeclineLegQuiet) with the newest fill
// stamped at or after QuietDeclineSmoothFrom and before QuietDeclineFillBefore; fails closed.
func ReadsForTheLadder(newestAt time.Time, block aggragates.SmartTakeLossIndicators) bool {
	if block.QuietDeclineVerdict {
		return true
	}
	if !block.QuietDeclineLegQuiet || block.QuietDeclineSmoothFrom <= 0 || block.QuietDeclineFillBefore <= 0 || newestAt.IsZero() {
		return false
	}
	stamp := newestAt.UnixMilli()
	return stamp >= block.QuietDeclineSmoothFrom && stamp < block.QuietDeclineFillBefore
}
