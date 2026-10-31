package quietdecline

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Confirms is whether a pending ladder's new fill holds the quiet decline, and the reasons the row
// names: the leg on and quiet on the last closed bar (QuietDeclineLegQuiet, QuietDeclineExitReasons),
// else the decline read recently from that fill (ReadsRecently, QuietDeclineRecentReasons).
func Confirms(newestAt time.Time, block aggragates.SmartSellIndicators) ([]string, bool) {
	if block.QuietDeclineLegQuiet {
		return block.QuietDeclineExitReasons, true
	}
	if ReadsRecently(newestAt, block) {
		return block.QuietDeclineRecentReasons, true
	}
	return nil, false
}
