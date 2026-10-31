package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// recentFill is the recent-fill rule before the quiet decline makes a ladder pending: always holds
// with quietDeclineNeedsRecentFill off, else quietdecline.RecentFill on the newest fill's stamp.
func recentFill(newest entryFill, block aggragates.SmartSellIndicators) bool {
	return !quietDeclineNeedsRecentFill || quietdecline.RecentFill(newest.At, block)
}
