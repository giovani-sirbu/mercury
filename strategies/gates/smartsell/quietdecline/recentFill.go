package quietdecline

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// RecentFill is the recent-fill rule's stamp check, asked under NeedsRecentFill before a watched
// ladder goes pending: the newest fill stamped at or after QuietDeclineFillFrom; fails closed.
func RecentFill(newestAt time.Time, block aggragates.SmartSellIndicators) bool {
	if block.QuietDeclineFillFrom <= 0 || newestAt.IsZero() {
		return false
	}
	return newestAt.UnixMilli() >= block.QuietDeclineFillFrom
}
