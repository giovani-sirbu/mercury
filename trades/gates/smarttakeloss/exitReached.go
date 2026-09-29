package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// ExitReached reports whether Apply forces a sale at this price whatever the
// ladder proposes on the add side, the dead zone included: the same guards
// and the same watches, this tick's slow-decline judgement taken first, then
// a pending ladder at its sell band or a ladder capital protection watches at
// its upper band. A trade resting in a close reads false, because Apply
// replaces none.
//
// It is Apply's own answer on the empty proposal — never a close — so the two
// cannot drift apart, and it is pure: the row that answer may carry is
// dropped, and nothing is written. A close the ladder proposes passes Apply
// untouched whatever this answer says. sisyphus backtesting asks it before it
// drops a print of a blocked trade: at any price under a funds block, and
// under a profit block only at or under the block's price, past which the
// ladder proposes the close hasProfit refused.
func ExitReached(trade aggragates.Trades, price float64, block aggragates.SmartTakeLossIndicators) bool {
	return Apply(trade, "", price, aggragates.AIIndicators{SmartTakeLoss: block}).Reason != ""
}
