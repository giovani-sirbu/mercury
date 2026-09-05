package cooldown

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// FirstFillVerdictNeeded tells an engine whether this tick has to fetch the
// sophos /markers verdict for the trade. It stands where the eight-hour
// Expired cap stood in the five fetch conditions (backtesting, live-testing
// and hermes, spot and futures), and it is a fetch decision only — the gate
// is FirstFillHold.
//
// On spot the verdict is a one-shot: it activates the hold, and from then on
// price alone decides, so fetching it again would cost a bar of sophos work
// per tick that nothing reads. Once the entry was released above the
// reference the gate is finished with the trade and the verdict is not
// needed either — the entry goes to market as soon as funds allow. Futures
// keep the verdict-only gate and fetch on every tick of a new trade.
//
// `position` is the trade position the engines resolve before the fetch;
// only "new" has a first fill to judge.
func FirstFillVerdictNeeded(trade aggragates.Trades, position string) bool {
	if position != "new" {
		return false
	}
	if trade.Strategy.TradeType == aggragates.Futures {
		return true
	}
	state := firstFillState(trade)
	return !state.activated && !state.enteredAbove
}
