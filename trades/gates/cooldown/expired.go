package cooldown

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// MaxHold caps how long the first-fill gate may keep a new trade waiting.
// Measured on run 97: every gain the gate produced sat in waits under eight
// hours (24 of 24 entries cheaper, median −1.3%); past that the median gain
// collapsed and the two longest waits entered 4% higher, because
// "expensive" stayed true all the way up a rally. After the horizon the
// engines skip the /markers fetch and the entry proceeds.
const MaxHold = 8 * time.Hour

// Expired reports whether the trade has waited past MaxHold since it was
// created. Unknown clocks (zero on either side) never expire.
func Expired(trade aggragates.Trades, now time.Time) bool {
	if trade.CreatedAt.IsZero() || now.IsZero() {
		return false
	}
	return now.UTC().Sub(trade.CreatedAt.UTC()) >= MaxHold
}
