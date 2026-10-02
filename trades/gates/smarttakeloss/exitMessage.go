package smarttakeloss

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// The reasons a forced exit can name in Result.Reason: the slow decline gate's
// sale at the sell band sophos serves, whichever feature made the ladder
// pending, and capital protection's at the upper Bollinger band sophos serves
// beside its SMC trend reading. The rules watch long ladders only and have no
// inverse mirror.
const (
	reasonSellBand          = "slow decline bollinger band"
	reasonCapitalProtection = "upper bollinger band (capital protection)"
)

// ExitMessage is the text of the INFO row the engines write beside a forced
// sellLoss: the human-readable trace of WHY the trade sold, and what the
// backtest verification reads. ExitRow builds the row from it, with the sold
// event that records the rule beside it. level is the price the sellLoss chain
// places its limit at, printed with the pair's PriceFilter decimals. The slow
// decline gate's sale names, after the level, the decline features the ladder
// is pending on, read off the trade's events (rebuildState): the engines append
// the tick's gate row before they ask for the exit row, so the trade carries
// the row of the very tick that sold.
func ExitMessage(trade aggragates.Trades, reason string, level float64) string {
	return exitMessage(trade, reason, level, exitFeatures(trade, reason))
}

// exitFeatures are the decline features the sale at the sell band ends: the
// ones the ladder is pending on. Capital protection's sale has none.
func exitFeatures(trade aggragates.Trades, reason string) []string {
	if reason == reasonCapitalProtection {
		return nil
	}
	return rebuildState(trade).pendingFeatures
}

// exitMessage is ExitMessage for the features already read.
func exitMessage(trade aggragates.Trades, reason string, level float64, features []string) string {
	message := fmt.Sprintf("smartTakeLoss: sell at %s %s", reason, gates.FormatPriceLevel(trade, level))
	if len(features) == 0 {
		return message
	}
	return message + " (" + labelList(features) + ")"
}
