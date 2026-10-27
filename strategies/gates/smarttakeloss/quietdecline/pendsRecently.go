package quietdecline

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// PendsRecently is whether the look-back marks a watched ladder pending: it reads recently for the
// ladder (ReadsRecently) and the first fill is stamped strictly before the bar the verdict stood on
// (QuietDeclineRecentAt), so a ladder opened after that bar goes pending on the last closed bar alone.
func PendsRecently(firstAt, newestAt time.Time, block aggragates.SmartTakeLossIndicators) bool {
	if !ReadsRecently(newestAt, block) {
		return false
	}
	return !firstAt.IsZero() && firstAt.UnixMilli() < block.QuietDeclineRecentAt
}
