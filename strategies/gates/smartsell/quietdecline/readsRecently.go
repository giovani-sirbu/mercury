package quietdecline

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// ReadsRecently is whether the quiet decline reads recently for a ladder: sophos served a bar among
// its last SlowDeclineRecentBars closed bars on which the verdict stood (QuietDeclineRecentAt) and
// the newest fill is stamped at or after QuietDeclineRecentFrom; fails closed.
func ReadsRecently(newestAt time.Time, block aggragates.SmartSellIndicators) bool {
	if block.QuietDeclineRecentAt <= 0 || block.QuietDeclineRecentFrom <= 0 || newestAt.IsZero() {
		return false
	}
	return newestAt.UnixMilli() >= block.QuietDeclineRecentFrom
}
